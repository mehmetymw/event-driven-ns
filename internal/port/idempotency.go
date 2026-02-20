package port

import "context"

type IdempotencyStore interface {
	Check(ctx context.Context, key string) (bool, string, error)
	SetNX(ctx context.Context, key string, notificationID string) (bool, error)
}
