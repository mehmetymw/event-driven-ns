package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

const idempotencyTTL = 24 * time.Hour

type IdempotencyRepo struct {
	db *sqlx.DB
}

func NewIdempotencyRepo(db *sqlx.DB) *IdempotencyRepo {
	return &IdempotencyRepo{db: db}
}

func (r *IdempotencyRepo) Check(ctx context.Context, key string) (bool, string, error) {
	var notificationID string
	err := r.db.GetContext(ctx, &notificationID,
		`SELECT notification_id FROM idempotency_keys WHERE key = $1 AND expires_at > NOW()`,
		key,
	)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, notificationID, nil
}

func (r *IdempotencyRepo) SetNX(ctx context.Context, key string, notificationID string) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO idempotency_keys (key, notification_id, expires_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO NOTHING`,
		key, notificationID, time.Now().UTC().Add(idempotencyTTL),
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
