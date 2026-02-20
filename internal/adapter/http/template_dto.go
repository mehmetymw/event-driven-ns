package http

import (
	"time"

	"github.com/mehmetymw/event-driven-ns/internal/app"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type CreateTemplateRequest struct {
	Name    string `json:"name" binding:"required"`
	Channel string `json:"channel" binding:"required,oneof=sms email push"`
	Body    string `json:"body" binding:"required"`
}

func (r *CreateTemplateRequest) ToInput() app.CreateTemplateInput {
	return app.CreateTemplateInput{
		Name:    r.Name,
		Channel: domain.Channel(r.Channel),
		Body:    r.Body,
	}
}

type TemplateResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Channel   string    `json:"channel"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewTemplateResponse(t *domain.Template) TemplateResponse {
	return TemplateResponse{
		ID:        t.ID.String(),
		Name:      t.Name,
		Channel:   string(t.Channel),
		Body:      t.Body,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}
