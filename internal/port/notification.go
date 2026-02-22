package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type NotificationRepository interface {
	Create(ctx context.Context, notification *domain.Notification) error
	CreateBatch(ctx context.Context, batch *domain.NotificationBatch, notifications []*domain.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	GetBatchByID(ctx context.Context, batchID uuid.UUID) (*domain.NotificationBatch, error)
	List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, error)
	UpdateStatus(ctx context.Context, notification *domain.Notification) error
	Cancel(ctx context.Context, id uuid.UUID) error
	IncrementBatchCounter(ctx context.Context, batchID uuid.UUID, status domain.Status) error
	ListDueScheduled(ctx context.Context, limit int) ([]*domain.Notification, error)
	ListStuckProcessing(ctx context.Context, olderThan time.Duration, limit int) ([]*domain.Notification, error)
	GetChannelMetrics(ctx context.Context) ([]domain.ChannelStats, error)
}
