package port

import (
	"context"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type ProviderResponse struct {
	MessageID string
	Status    string
	Timestamp string
}

type DeliveryProvider interface {
	Send(ctx context.Context, notification *domain.Notification) (*ProviderResponse, error)
}
