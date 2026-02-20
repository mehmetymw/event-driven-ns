package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

var topicForPriority = map[domain.Priority]string{
	domain.PriorityHigh:   "notifications.high",
	domain.PriorityNormal: "notifications.normal",
	domain.PriorityLow:    "notifications.low",
}

type NotificationPayload struct {
	NotificationID string            `json:"notification_id"`
	Channel        string            `json:"channel"`
	Carrier        map[string]string `json:"carrier,omitempty"`
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		},
	}
}

func (p *Producer) Enqueue(ctx context.Context, n *domain.Notification) error {
	ctx, span := tracing.Tracer().Start(ctx, "kafka.produce")
	defer span.End()

	topic, ok := topicForPriority[n.Priority]
	if !ok {
		err := fmt.Errorf("unknown priority: %s", n.Priority)
		tracing.RecordError(span, err)
		return err
	}

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", topic),
		attribute.String("messaging.operation.type", "publish"),
		attribute.String("notification.id", n.ID.String()),
		attribute.String("notification.channel", string(n.Channel)),
		attribute.String("notification.priority", string(n.Priority)),
	)

	payload := NotificationPayload{
		NotificationID: n.ID.String(),
		Channel:        string(n.Channel),
		Carrier:        propagateTraceContext(ctx),
	}

	value, err := json.Marshal(payload)
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}

	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(n.ID.String()),
		Value: value,
	}); err != nil {
		tracing.RecordError(span, err)
		return err
	}

	return nil
}

func (p *Producer) EnqueueScheduled(_ context.Context, _ *domain.Notification) error {
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func propagateTraceContext(ctx context.Context) map[string]string {
	carrier := make(map[string]string)
	propagation.TraceContext{}.Inject(ctx, propagation.MapCarrier(carrier))
	return carrier
}
