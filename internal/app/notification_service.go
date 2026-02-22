package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

type NotificationService struct {
	repo       port.NotificationRepository
	queue      port.QueuePublisher
	tmplRepo   port.TemplateRepository
	idempotent port.IdempotencyStore
	logger     *zap.Logger
}

func NewNotificationService(
	repo port.NotificationRepository,
	queue port.QueuePublisher,
	tmplRepo port.TemplateRepository,
	idempotent port.IdempotencyStore,
	logger *zap.Logger,
) *NotificationService {
	return &NotificationService{
		repo:       repo,
		queue:      queue,
		tmplRepo:   tmplRepo,
		idempotent: idempotent,
		logger:     logger,
	}
}

type CreateNotificationInput struct {
	Channel           domain.Channel
	Recipient         string
	Content           string
	Priority          domain.Priority
	ScheduledAt       *time.Time
	IdempotencyKey    *string
	TemplateID        *uuid.UUID
	TemplateVariables map[string]string
}

func (s *NotificationService) Create(ctx context.Context, input CreateNotificationInput) (*domain.Notification, error) {
	ctx, span := tracing.Tracer().Start(ctx, "notification.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.channel", string(input.Channel)),
		attribute.String("notification.priority", string(input.Priority)),
		attribute.String("notification.recipient", input.Recipient),
	)

	if input.IdempotencyKey != nil {
		span.SetAttributes(attribute.String("notification.idempotency_key", *input.IdempotencyKey))
		exists, existingID, err := s.idempotent.Check(ctx, *input.IdempotencyKey)
		if err != nil {
			s.logger.Error("idempotency check failed", zap.Error(err))
		}
		if exists {
			span.SetAttributes(attribute.Bool("notification.idempotent_hit", true))
			id, _ := uuid.Parse(existingID)
			return s.repo.GetByID(ctx, id)
		}
	}

	content := input.Content
	if input.TemplateID != nil {
		span.SetAttributes(attribute.String("notification.template_id", input.TemplateID.String()))
		tmpl, err := s.tmplRepo.GetByID(ctx, *input.TemplateID)
		if err != nil {
			tracing.RecordError(span, err)
			return nil, err
		}
		rendered, err := tmpl.Render(input.TemplateVariables)
		if err != nil {
			tracing.RecordError(span, err)
			return nil, err
		}
		content = rendered
	}

	notification, err := domain.NewNotification(input.Channel, input.Recipient, content, input.Priority, input.ScheduledAt)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	span.SetAttributes(attribute.String("notification.id", notification.ID.String()))

	notification.IdempotencyKey = input.IdempotencyKey
	notification.TemplateID = input.TemplateID
	notification.TemplateVariables = input.TemplateVariables

	if err := s.repo.Create(ctx, notification); err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	if input.IdempotencyKey != nil {
		if _, err := s.idempotent.SetNX(ctx, *input.IdempotencyKey, notification.ID.String()); err != nil {
			s.logger.Error("idempotency set failed", zap.Error(err))
		}
	}

	if notification.ScheduledAt != nil {
		if err := s.queue.EnqueueScheduled(ctx, notification); err != nil {
			tracing.RecordError(span, err)
			return nil, err
		}
	} else {
		if err := s.queue.Enqueue(ctx, notification); err != nil {
			tracing.RecordError(span, err)
			return nil, err
		}
	}

	s.logger.Info("notification created",
		zap.String("id", notification.ID.String()),
		zap.String("channel", string(notification.Channel)),
		zap.String("priority", string(notification.Priority)),
		zap.String("trace_id", tracing.TraceIDFromContext(ctx)),
	)

	return notification, nil
}

type CreateBatchInput struct {
	Notifications []CreateNotificationInput
}

func (s *NotificationService) CreateBatch(ctx context.Context, input CreateBatchInput) (*domain.NotificationBatch, []*domain.Notification, error) {
	ctx, span := tracing.Tracer().Start(ctx, "notification.create_batch")
	defer span.End()

	span.SetAttributes(attribute.Int("batch.size", len(input.Notifications)))

	if len(input.Notifications) == 0 {
		tracing.RecordError(span, domain.ErrBatchEmpty)
		return nil, nil, domain.ErrBatchEmpty
	}
	if len(input.Notifications) > 1000 {
		tracing.RecordError(span, domain.ErrBatchTooLarge)
		return nil, nil, domain.ErrBatchTooLarge
	}

	batch := &domain.NotificationBatch{
		ID:           uuid.Must(uuid.NewV7()),
		TotalCount:   len(input.Notifications),
		PendingCount: len(input.Notifications),
		CreatedAt:    time.Now().UTC(),
	}

	span.SetAttributes(attribute.String("batch.id", batch.ID.String()))

	notifications := make([]*domain.Notification, 0, len(input.Notifications))
	for _, in := range input.Notifications {
		content := in.Content
		if in.TemplateID != nil {
			tmpl, err := s.tmplRepo.GetByID(ctx, *in.TemplateID)
			if err != nil {
				tracing.RecordError(span, err)
				return nil, nil, err
			}
			rendered, err := tmpl.Render(in.TemplateVariables)
			if err != nil {
				tracing.RecordError(span, err)
				return nil, nil, err
			}
			content = rendered
		}

		n, err := domain.NewNotification(in.Channel, in.Recipient, content, in.Priority, in.ScheduledAt)
		if err != nil {
			tracing.RecordError(span, err)
			return nil, nil, err
		}
		n.BatchID = &batch.ID
		n.IdempotencyKey = in.IdempotencyKey
		n.TemplateID = in.TemplateID
		n.TemplateVariables = in.TemplateVariables
		notifications = append(notifications, n)
	}

	if err := s.repo.CreateBatch(ctx, batch, notifications); err != nil {
		tracing.RecordError(span, err)
		return nil, nil, err
	}

	for _, n := range notifications {
		if n.ScheduledAt != nil {
			_ = s.queue.EnqueueScheduled(ctx, n)
		} else {
			_ = s.queue.Enqueue(ctx, n)
		}
	}

	s.logger.Info("batch created",
		zap.String("batch_id", batch.ID.String()),
		zap.Int("count", batch.TotalCount),
		zap.String("trace_id", tracing.TraceIDFromContext(ctx)),
	)

	return batch, notifications, nil
}

func (s *NotificationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *NotificationService) GetBatch(ctx context.Context, batchID uuid.UUID) (*domain.NotificationBatch, error) {
	return s.repo.GetBatchByID(ctx, batchID)
}

func (s *NotificationService) List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, error) {
	return s.repo.List(ctx, filter)
}

func (s *NotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.Tracer().Start(ctx, "notification.cancel")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", id.String()))

	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}

	if err := n.Cancel(); err != nil {
		tracing.RecordError(span, err)
		return err
	}

	if err := s.repo.Cancel(ctx, id); err != nil {
		tracing.RecordError(span, err)
		return err
	}

	if n.BatchID != nil {
		_ = s.repo.IncrementBatchCounter(ctx, *n.BatchID, domain.StatusCancelled)
	}

	s.logger.Info("notification cancelled",
		zap.String("id", id.String()),
		zap.String("trace_id", tracing.TraceIDFromContext(ctx)),
	)
	return nil
}
