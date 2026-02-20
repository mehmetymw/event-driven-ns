package queue

import (
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

type ConsumerConfig struct {
	Brokers        []string
	Group          string
	RatePerChannel int
	Logger         *zap.Logger
}

var priorityTopics = []string{
	"notifications.high",
	"notifications.normal",
	"notifications.low",
}

type Consumer struct {
	cfg      ConsumerConfig
	readers  []*kafka.Reader
	writer   *kafka.Writer
	limiters map[string]*rate.Limiter
	logger   *zap.Logger
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewConsumer(cfg ConsumerConfig) *Consumer {
	limiters := map[string]*rate.Limiter{
		string(domain.ChannelSMS):   rate.NewLimiter(rate.Limit(cfg.RatePerChannel), cfg.RatePerChannel),
		string(domain.ChannelEmail): rate.NewLimiter(rate.Limit(cfg.RatePerChannel), cfg.RatePerChannel),
		string(domain.ChannelPush):  rate.NewLimiter(rate.Limit(cfg.RatePerChannel), cfg.RatePerChannel),
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
	}

	return &Consumer{
		cfg:      cfg,
		writer:   writer,
		limiters: limiters,
		logger:   cfg.Logger,
	}
}

func (c *Consumer) Start(ctx context.Context, handler port.MessageHandler) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	for _, topic := range priorityTopics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        c.cfg.Brokers,
			Topic:          topic,
			GroupID:        c.cfg.Group,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
			StartOffset:    kafka.FirstOffset,
		})
		c.readers = append(c.readers, reader)

		c.wg.Add(1)
		go c.consume(ctx, reader, handler)
	}

	c.logger.Info("kafka consumer started",
		zap.Strings("brokers", c.cfg.Brokers),
		zap.String("group", c.cfg.Group),
		zap.Int("topic_count", len(priorityTopics)),
	)

	<-ctx.Done()
	return ctx.Err()
}

func (c *Consumer) Stop(_ context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	var firstErr error
	for _, r := range c.readers {
		if err := r.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := c.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (c *Consumer) consume(ctx context.Context, reader *kafka.Reader, handler port.MessageHandler) {
	defer c.wg.Done()

	topic := reader.Config().Topic
	c.logger.Info("consumer goroutine started", zap.String("topic", topic))

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("fetch message failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			time.Sleep(time.Second)
			continue
		}

		var payload NotificationPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			c.logger.Error("unmarshal payload failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		msgCtx := ctx
		if len(payload.Carrier) > 0 {
			msgCtx = propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier(payload.Carrier))
		}

		msgCtx, span := tracing.Tracer().Start(msgCtx, "kafka.consume")
		span.SetAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.source.name", msg.Topic),
			attribute.String("messaging.operation.type", "receive"),
			attribute.String("messaging.consumer.group.id", c.cfg.Group),
			attribute.String("notification.id", payload.NotificationID),
			attribute.String("notification.channel", payload.Channel),
			attribute.Int64("messaging.kafka.message.offset", msg.Offset),
			attribute.Int("messaging.kafka.destination.partition", msg.Partition),
		)

		if limiter, ok := c.limiters[payload.Channel]; ok {
			_ = limiter.Wait(msgCtx)
		}

		c.logger.Info("processing notification",
			zap.String("notification_id", payload.NotificationID),
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
		)

		if err := handler(msgCtx, payload.NotificationID); err != nil {
			span.SetAttributes(attribute.Bool("delivery.will_retry", true))
			tracing.RecordError(span, err)
			span.End()
			c.retry(ctx, msg, payload)
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		span.End()
		_ = reader.CommitMessages(ctx, msg)
	}
}

func (c *Consumer) retry(ctx context.Context, original kafka.Message, payload NotificationPayload) {
	delay := retryDelay(payload.NotificationID)
	time.Sleep(delay)

	if err := c.writer.WriteMessages(ctx, kafka.Message{
		Topic: original.Topic,
		Key:   original.Key,
		Value: original.Value,
	}); err != nil {
		c.logger.Error("retry re-enqueue failed",
			zap.String("notification_id", payload.NotificationID),
			zap.Error(err),
		)
	}
}

func retryDelay(id string) time.Duration {
	baseDelay := 2 * time.Second
	maxDelay := 30 * time.Second
	jitter := time.Duration(rand.Int64N(1000)) * time.Millisecond

	delay := baseDelay + jitter
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func RetryDelayForAttempt(attempt int) time.Duration {
	baseDelay := time.Second
	maxDelay := 5 * time.Minute
	jitter := time.Duration(rand.Int64N(500)) * time.Millisecond

	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
	delay += jitter

	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}
