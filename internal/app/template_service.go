package app

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
)

type TemplateService struct {
	repo   port.TemplateRepository
	logger *zap.Logger
}

func NewTemplateService(repo port.TemplateRepository, logger *zap.Logger) *TemplateService {
	return &TemplateService{repo: repo, logger: logger}
}

type CreateTemplateInput struct {
	Name    string
	Channel domain.Channel
	Body    string
}

func (s *TemplateService) Create(ctx context.Context, input CreateTemplateInput) (*domain.Template, error) {
	tmpl, err := domain.NewTemplate(input.Name, input.Channel, input.Body)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, tmpl); err != nil {
		return nil, err
	}

	s.logger.Info("template created",
		zap.String("id", tmpl.ID.String()),
		zap.String("name", tmpl.Name),
	)

	return tmpl, nil
}

func (s *TemplateService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TemplateService) List(ctx context.Context) ([]*domain.Template, error) {
	return s.repo.List(ctx)
}
