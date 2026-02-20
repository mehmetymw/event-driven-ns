package domain

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/google/uuid"
)

type Template struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Channel   Channel   `db:"channel"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func NewTemplate(name string, channel Channel, body string) (*Template, error) {
	if name == "" {
		return nil, ErrEmptyTemplateName
	}
	if err := validateChannel(channel); err != nil {
		return nil, err
	}
	if body == "" {
		return nil, ErrEmptyTemplateBody
	}

	if _, err := template.New("validate").Parse(body); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTemplateBody, err)
	}

	now := time.Now().UTC()
	return &Template{
		ID:        uuid.Must(uuid.NewV7()),
		Name:      name,
		Channel:   channel,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (t *Template) Render(variables map[string]string) (string, error) {
	tmpl, err := template.New(t.Name).Parse(t.Body)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateRenderFailed, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateRenderFailed, err)
	}

	return buf.String(), nil
}
