package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type TemplateRepo struct {
	db *sqlx.DB
}

func NewTemplateRepo(db *sqlx.DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

func (r *TemplateRepo) Create(ctx context.Context, t *domain.Template) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO templates (id, name, channel, body, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.Name, t.Channel, t.Body, t.CreatedAt, t.UpdatedAt,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "name") {
		return domain.ErrDuplicateTemplateName
	}
	return err
}

func (r *TemplateRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	var t domain.Template
	err := r.db.GetContext(ctx, &t,
		`SELECT id, name, channel, body, created_at, updated_at FROM templates WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateRepo) List(ctx context.Context) ([]*domain.Template, error) {
	var templates []*domain.Template
	err := r.db.SelectContext(ctx, &templates,
		`SELECT id, name, channel, body, created_at, updated_at FROM templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return templates, nil
}
