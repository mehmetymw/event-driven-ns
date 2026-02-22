package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type TemplateRepository interface {
	Create(ctx context.Context, template *domain.Template) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	List(ctx context.Context) ([]*domain.Template, error)
}
