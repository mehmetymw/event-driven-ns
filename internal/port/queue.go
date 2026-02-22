package port

import (
	"context"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type QueuePublisher interface {
	Enqueue(ctx context.Context, notification *domain.Notification) error
	EnqueueScheduled(ctx context.Context, notification *domain.Notification) error
	Close() error
}

type MessageHandler func(ctx context.Context, notificationID string) error

type QueueConsumer interface {
	Start(ctx context.Context, handler MessageHandler) error
	Stop(ctx context.Context) error
}
