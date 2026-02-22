package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

type DeliveryService struct {
	repo        port.NotificationRepository
	provider    port.DeliveryProvider
	broadcaster port.StatusBroadcaster
	metrics     *MetricsCollector
	logger      *zap.Logger
}

func NewDeliveryService(
	repo port.NotificationRepository,
	provider port.DeliveryProvider,
	broadcaster port.StatusBroadcaster,
	metrics *MetricsCollector,
	logger *zap.Logger,
) *DeliveryService {
	return &DeliveryService{
		repo:        repo,
		provider:    provider,
		broadcaster: broadcaster,
		metrics:     metrics,
		logger:      logger,
	}
}

func (s *DeliveryService) ProcessDelivery(ctx context.Context, notificationID string) error {
	ctx, span := tracing.Tracer().Start(ctx, "delivery.process")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", notificationID))

	start := time.Now()

	id, err := uuid.Parse(notificationID)
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}

	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}

	span.SetAttributes(
		attribute.String("notification.channel", string(notification.Channel)),
		attribute.String("notification.priority", string(notification.Priority)),
		attribute.String("notification.status", string(notification.Status)),
		attribute.Int("notification.retry_count", notification.RetryCount),
	)

	if notification.Status == domain.StatusCancelled || notification.Status == domain.StatusDelivered {
		span.SetAttributes(attribute.Bool("delivery.skipped", true))
		return nil
	}

	notification.MarkProcessing()
	if err := s.repo.UpdateStatus(ctx, notification); err != nil {
		tracing.RecordError(span, err)
		return err
	}

	resp, sendErr := s.provider.Send(ctx, notification)

	latency := time.Since(start)
	span.SetAttributes(attribute.Int64("delivery.latency_ms", latency.Milliseconds()))

	if sendErr != nil {
		notification.IncrementRetry()

		if isTransient(sendErr) && notification.HasRetriesLeft() {
			span.SetAttributes(
				attribute.Bool("delivery.will_retry", true),
				attribute.Int("delivery.retry_count", notification.RetryCount),
			)
			if err := s.repo.UpdateStatus(ctx, notification); err != nil {
				s.logger.Error("failed to update retry status", zap.Error(err))
			}
			s.metrics.RecordFailure(string(notification.Channel))
			s.logger.Warn("delivery failed, will retry",
				zap.String("id", notificationID),
				zap.Int("retry", notification.RetryCount),
				zap.Error(sendErr),
				zap.String("trace_id", tracing.TraceIDFromContext(ctx)),
			)
			tracing.RecordError(span, sendErr)
			return sendErr
		}

		notification.MarkFailed(sendErr.Error())
		if err := s.repo.UpdateStatus(ctx, notification); err != nil {
			s.logger.Error("failed to update failed status", zap.Error(err))
		}

		if notification.BatchID != nil {
			_ = s.repo.IncrementBatchCounter(ctx, *notification.BatchID, domain.StatusFailed)
		}

		s.metrics.RecordFailure(string(notification.Channel))
		s.broadcastStatus(notification)

		span.SetAttributes(attribute.Bool("delivery.permanently_failed", true))
		tracing.RecordError(span, sendErr)

		s.logger.Error("delivery permanently failed",
			zap.String("id", notificationID),
			zap.Error(sendErr),
			zap.String("trace_id", tracing.TraceIDFromContext(ctx)),
		)
		return nil
	}

	notification.MarkDelivered(resp.MessageID)
	if err := s.repo.UpdateStatus(ctx, notification); err != nil {
		s.logger.Error("failed to update delivered status", zap.Error(err))
	}

	if notification.BatchID != nil {
		_ = s.repo.IncrementBatchCounter(ctx, *notification.BatchID, domain.StatusDelivered)
	}

	s.metrics.RecordSuccess(string(notification.Channel), latency)
	s.broadcastStatus(notification)

	span.SetAttributes(
		attribute.Bool("delivery.success", true),
		attribute.String("delivery.provider_message_id", resp.MessageID),
	)

	s.logger.Info("notification delivered",
		zap.String("id", notificationID),
		zap.String("provider_message_id", resp.MessageID),
		zap.Duration("latency", latency),
		zap.String("trace_id", tracing.TraceIDFromContext(ctx)),
	)

	return nil
}

func (s *DeliveryService) broadcastStatus(n *domain.Notification) {
	s.broadcaster.Broadcast(n.ID.String(), string(n.Status), time.Now().UTC().Format(time.RFC3339))
}

func isTransient(err error) bool {
	return errors.Is(err, domain.ErrProviderUnavailable) || errors.Is(err, domain.ErrCircuitOpen)
}
