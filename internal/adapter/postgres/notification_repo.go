package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

func wrapIDempotencyError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "idempotency_key") {
		return domain.ErrDuplicateIdempotencyKey
	}
	return err
}

type NotificationRepo struct {
	db *sqlx.DB
}

func NewNotificationRepo(db *sqlx.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

type notificationRow struct {
	ID                uuid.UUID       `db:"id"`
	BatchID           *uuid.UUID      `db:"batch_id"`
	IdempotencyKey    *string         `db:"idempotency_key"`
	Channel           string          `db:"channel"`
	Recipient         string          `db:"recipient"`
	Content           string          `db:"content"`
	Priority          string          `db:"priority"`
	Status            string          `db:"status"`
	ScheduledAt       *time.Time      `db:"scheduled_at"`
	SentAt            *time.Time      `db:"sent_at"`
	FailedAt          *time.Time      `db:"failed_at"`
	ErrorMessage      *string         `db:"error_message"`
	RetryCount        int             `db:"retry_count"`
	MaxRetries        int             `db:"max_retries"`
	ProviderMessageID *string         `db:"provider_message_id"`
	TemplateID        *uuid.UUID      `db:"template_id"`
	TemplateVariables json.RawMessage `db:"template_variables"`
	CreatedAt         time.Time       `db:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"`
}

func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) error {
	vars, _ := json.Marshal(n.TemplateVariables)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notifications 
		(id, batch_id, idempotency_key, channel, recipient, content, priority, status, 
		 scheduled_at, max_retries, template_id, template_variables, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		n.ID, n.BatchID, n.IdempotencyKey, n.Channel, n.Recipient, n.Content, n.Priority,
		n.Status, n.ScheduledAt, n.MaxRetries, n.TemplateID, vars, n.CreatedAt, n.UpdatedAt,
	)
	return wrapIDempotencyError(err)
}

func (r *NotificationRepo) CreateBatch(ctx context.Context, batch *domain.NotificationBatch, notifications []*domain.Notification) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO notification_batches (id, total_count, pending_count, created_at)
		VALUES ($1, $2, $3, $4)`,
		batch.ID, batch.TotalCount, batch.PendingCount, batch.CreatedAt,
	)
	if err != nil {
		return err
	}

	for _, n := range notifications {
		vars, _ := json.Marshal(n.TemplateVariables)
		_, err = tx.ExecContext(ctx,
			`INSERT INTO notifications 
			(id, batch_id, idempotency_key, channel, recipient, content, priority, status,
			 scheduled_at, max_retries, template_id, template_variables, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			n.ID, n.BatchID, n.IdempotencyKey, n.Channel, n.Recipient, n.Content, n.Priority,
			n.Status, n.ScheduledAt, n.MaxRetries, n.TemplateID, vars, n.CreatedAt, n.UpdatedAt,
		)
		if err != nil {
			return wrapIDempotencyError(err)
		}
	}

	return tx.Commit()
}

func (r *NotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var row notificationRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM notifications WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotificationNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToNotification(row), nil
}

func (r *NotificationRepo) GetBatchByID(ctx context.Context, batchID uuid.UUID) (*domain.NotificationBatch, error) {
	var batch domain.NotificationBatch
	err := r.db.GetContext(ctx, &batch,
		`SELECT id, total_count, pending_count, delivered_count, failed_count, cancelled_count, created_at
		FROM notification_batches WHERE id = $1`, batchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrBatchNotFound
	}
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

func (r *NotificationRepo) List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, error) {
	query := `SELECT * FROM notifications WHERE 1=1`
	args := []any{}
	argIdx := 1

	if filter.Status != nil {
		query += ` AND status = $` + itoa(argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Channel != nil {
		query += ` AND channel = $` + itoa(argIdx)
		args = append(args, *filter.Channel)
		argIdx++
	}
	if filter.BatchID != nil {
		query += ` AND batch_id = $` + itoa(argIdx)
		args = append(args, *filter.BatchID)
		argIdx++
	}
	if filter.DateFrom != nil {
		query += ` AND created_at >= $` + itoa(argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		query += ` AND created_at <= $` + itoa(argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if filter.Cursor != nil {
		query += ` AND id < $` + itoa(argIdx)
		args = append(args, *filter.Cursor)
		argIdx++
	}

	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	query += ` ORDER BY created_at DESC, id DESC LIMIT $` + itoa(argIdx)
	args = append(args, pageSize)

	var rows []notificationRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	result := make([]*domain.Notification, len(rows))
	for i, row := range rows {
		result[i] = rowToNotification(row)
	}
	return result, nil
}

func (r *NotificationRepo) UpdateStatus(ctx context.Context, n *domain.Notification) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications 
		SET status=$1, sent_at=$2, failed_at=$3, error_message=$4, retry_count=$5, 
		    provider_message_id=$6, updated_at=$7
		WHERE id=$8`,
		n.Status, n.SentAt, n.FailedAt, n.ErrorMessage, n.RetryCount,
		n.ProviderMessageID, n.UpdatedAt, n.ID,
	)
	return err
}

func (r *NotificationRepo) Cancel(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET status='cancelled', updated_at=NOW() 
		WHERE id=$1 AND status IN ('pending','scheduled')`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrInvalidStatusTransition
	}
	return nil
}

func (r *NotificationRepo) IncrementBatchCounter(ctx context.Context, batchID uuid.UUID, status domain.Status) error {
	var column string
	switch status {
	case domain.StatusDelivered:
		column = "delivered_count"
	case domain.StatusFailed:
		column = "failed_count"
	case domain.StatusCancelled:
		column = "cancelled_count"
	default:
		return nil
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE notification_batches 
		SET `+column+` = `+column+` + 1, pending_count = pending_count - 1
		WHERE id = $1`, batchID)
	return err
}

func rowToNotification(row notificationRow) *domain.Notification {
	n := &domain.Notification{
		ID:                row.ID,
		BatchID:           row.BatchID,
		IdempotencyKey:    row.IdempotencyKey,
		Channel:           domain.Channel(row.Channel),
		Recipient:         row.Recipient,
		Content:           row.Content,
		Priority:          domain.Priority(row.Priority),
		Status:            domain.Status(row.Status),
		ScheduledAt:       row.ScheduledAt,
		SentAt:            row.SentAt,
		FailedAt:          row.FailedAt,
		ErrorMessage:      row.ErrorMessage,
		RetryCount:        row.RetryCount,
		MaxRetries:        row.MaxRetries,
		ProviderMessageID: row.ProviderMessageID,
		TemplateID:        row.TemplateID,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}

	if row.TemplateVariables != nil {
		_ = json.Unmarshal(row.TemplateVariables, &n.TemplateVariables)
	}

	return n
}

func (r *NotificationRepo) ListDueScheduled(ctx context.Context, limit int) ([]*domain.Notification, error) {
	var rows []notificationRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM notifications WHERE status = 'scheduled' AND scheduled_at <= NOW() ORDER BY scheduled_at LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Notification, len(rows))
	for i, row := range rows {
		result[i] = rowToNotification(row)
	}
	return result, nil
}

func (r *NotificationRepo) ListStuckProcessing(ctx context.Context, olderThan time.Duration, limit int) ([]*domain.Notification, error) {
	var rows []notificationRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM notifications WHERE status = 'processing' AND updated_at < NOW() - $1::interval ORDER BY updated_at LIMIT $2`,
		olderThan.String(), limit,
	)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Notification, len(rows))
	for i, row := range rows {
		result[i] = rowToNotification(row)
	}
	return result, nil
}

func (r *NotificationRepo) GetChannelMetrics(ctx context.Context) ([]domain.ChannelStats, error) {
	var stats []domain.ChannelStats
	err := r.db.SelectContext(ctx, &stats,
		`SELECT channel,
			COUNT(*) FILTER (WHERE status = 'delivered') AS sent,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COALESCE(AVG(EXTRACT(EPOCH FROM (sent_at - created_at)) * 1000) FILTER (WHERE status = 'delivered' AND sent_at IS NOT NULL), 0) AS avg_latency_ms
		FROM notifications
		GROUP BY channel`)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
