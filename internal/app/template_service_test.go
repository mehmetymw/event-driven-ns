package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

func newTestTemplateService() (*TemplateService, *mockTemplateRepo) {
	repo := newMockTemplateRepo()
	logger := zap.NewNop()
	svc := NewTemplateService(repo, logger)
	return svc, repo
}

func TestTemplateService_Create_Success(t *testing.T) {
	svc, repo := newTestTemplateService()

	tmpl, err := svc.Create(context.Background(), CreateTemplateInput{
		Name:    "welcome",
		Channel: domain.ChannelSMS,
		Body:    "Hello {{.name}}, welcome!",
	})

	require.NoError(t, err)
	assert.NotNil(t, tmpl)
	assert.Equal(t, "welcome", tmpl.Name)
	assert.Equal(t, domain.ChannelSMS, tmpl.Channel)

	stored, err := repo.GetByID(context.Background(), tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.ID, stored.ID)
}

func TestTemplateService_Create_EmptyName(t *testing.T) {
	svc, _ := newTestTemplateService()

	_, err := svc.Create(context.Background(), CreateTemplateInput{
		Name:    "",
		Channel: domain.ChannelSMS,
		Body:    "Hello",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrEmptyTemplateName)
}

func TestTemplateService_Create_EmptyBody(t *testing.T) {
	svc, _ := newTestTemplateService()

	_, err := svc.Create(context.Background(), CreateTemplateInput{
		Name:    "test",
		Channel: domain.ChannelEmail,
		Body:    "",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrEmptyTemplateBody)
}

func TestTemplateService_Create_InvalidChannel(t *testing.T) {
	svc, _ := newTestTemplateService()

	_, err := svc.Create(context.Background(), CreateTemplateInput{
		Name:    "test",
		Channel: "fax",
		Body:    "Hello",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidChannel)
}

func TestTemplateService_Create_InvalidBody(t *testing.T) {
	svc, _ := newTestTemplateService()

	_, err := svc.Create(context.Background(), CreateTemplateInput{
		Name:    "broken",
		Channel: domain.ChannelSMS,
		Body:    "Hello {{.name",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidTemplateBody)
}

func TestTemplateService_Create_RepoError(t *testing.T) {
	svc, repo := newTestTemplateService()
	repo.createErr = assert.AnError

	_, err := svc.Create(context.Background(), CreateTemplateInput{
		Name:    "test",
		Channel: domain.ChannelPush,
		Body:    "Notification: {{.msg}}",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestTemplateService_GetByID_Found(t *testing.T) {
	svc, repo := newTestTemplateService()

	tmpl, _ := domain.NewTemplate("promo", domain.ChannelEmail, "Sale: {{.discount}}%")
	_ = repo.Create(context.Background(), tmpl)

	result, err := svc.GetByID(context.Background(), tmpl.ID)

	require.NoError(t, err)
	assert.Equal(t, tmpl.ID, result.ID)
	assert.Equal(t, "promo", result.Name)
}

func TestTemplateService_GetByID_NotFound(t *testing.T) {
	svc, _ := newTestTemplateService()

	_, err := svc.GetByID(context.Background(), uuid.Must(uuid.NewV7()))

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrTemplateNotFound)
}

func TestTemplateService_List(t *testing.T) {
	svc, repo := newTestTemplateService()

	t1, _ := domain.NewTemplate("a", domain.ChannelSMS, "Hello")
	t2, _ := domain.NewTemplate("b", domain.ChannelEmail, "World")
	_ = repo.Create(context.Background(), t1)
	_ = repo.Create(context.Background(), t2)

	result, err := svc.List(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 2)
}
