package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

func newTestNotificationService() (*NotificationService, *mockNotificationRepo, *mockQueuePublisher, *mockTemplateRepo, *mockIdempotencyStore) {
	repo := newMockNotificationRepo()
	queue := newMockQueuePublisher()
	tmplRepo := newMockTemplateRepo()
	idempotent := newMockIdempotencyStore()
	logger := zap.NewNop()
	svc := NewNotificationService(repo, queue, tmplRepo, idempotent, logger)
	return svc, repo, queue, tmplRepo, idempotent
}

func TestNotificationService_Create_Success(t *testing.T) {
	svc, repo, queue, _, _ := newTestNotificationService()

	n, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:   domain.ChannelSMS,
		Recipient: "+905530050594",
		Content:   "hello",
		Priority:  domain.PriorityNormal,
	})

	require.NoError(t, err)
	assert.NotNil(t, n)
	assert.Equal(t, domain.ChannelSMS, n.Channel)
	assert.Equal(t, domain.StatusPending, n.Status)
	assert.Len(t, queue.enqueued, 1)
	assert.Equal(t, n.ID, queue.enqueued[0].ID)

	stored, err := repo.GetByID(context.Background(), n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.ID, stored.ID)
}

func TestNotificationService_Create_Scheduled(t *testing.T) {
	svc, _, queue, _, _ := newTestNotificationService()

	scheduledAt := time.Now().Add(1 * time.Hour).UTC()
	n, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:     domain.ChannelEmail,
		Recipient:   "test@example.com",
		Content:     "scheduled msg",
		Priority:    domain.PriorityHigh,
		ScheduledAt: &scheduledAt,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusScheduled, n.Status)
	assert.Len(t, queue.enqueued, 0)
	assert.Equal(t, 1, queue.scheduledCount)
}

func TestNotificationService_Create_IdempotencyHit(t *testing.T) {
	svc, repo, queue, _, idempotent := newTestNotificationService()

	existing, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "first", domain.PriorityNormal, nil)
	_ = repo.Create(context.Background(), existing)
	idempotent.keys["idem-key-1"] = existing.ID.String()

	key := "idem-key-1"
	n, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:        domain.ChannelSMS,
		Recipient:      "+905530050594",
		Content:        "duplicate",
		Priority:       domain.PriorityNormal,
		IdempotencyKey: &key,
	})

	require.NoError(t, err)
	assert.Equal(t, existing.ID, n.ID)
	assert.Len(t, queue.enqueued, 0)
}

func TestNotificationService_Create_IdempotencyMiss(t *testing.T) {
	svc, _, queue, _, idempotent := newTestNotificationService()

	key := "new-idem-key"
	n, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:        domain.ChannelSMS,
		Recipient:      "+905530050594",
		Content:        "unique msg",
		Priority:       domain.PriorityNormal,
		IdempotencyKey: &key,
	})

	require.NoError(t, err)
	assert.NotNil(t, n)
	assert.Len(t, queue.enqueued, 1)

	storedID, ok := idempotent.keys[key]
	assert.True(t, ok)
	assert.Equal(t, n.ID.String(), storedID)
}

func TestNotificationService_Create_WithTemplate(t *testing.T) {
	svc, _, queue, tmplRepo, _ := newTestNotificationService()

	tmpl, _ := domain.NewTemplate("welcome", domain.ChannelSMS, "Hello {{.name}}")
	_ = tmplRepo.Create(context.Background(), tmpl)

	vars := map[string]string{"name": "John"}
	n, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:           domain.ChannelSMS,
		Recipient:         "+905530050594",
		Content:           "",
		Priority:          domain.PriorityNormal,
		TemplateID:        &tmpl.ID,
		TemplateVariables: vars,
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello John", n.Content)
	assert.Len(t, queue.enqueued, 1)
}

func TestNotificationService_Create_TemplateNotFound(t *testing.T) {
	svc, _, _, _, _ := newTestNotificationService()

	missingID := uuid.Must(uuid.NewV7())
	_, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:    domain.ChannelSMS,
		Recipient:  "+905530050594",
		Content:    "ignored",
		Priority:   domain.PriorityNormal,
		TemplateID: &missingID,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrTemplateNotFound)
}

func TestNotificationService_Create_ValidationError(t *testing.T) {
	svc, _, _, _, _ := newTestNotificationService()

	_, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:   domain.ChannelSMS,
		Recipient: "invalid-phone",
		Content:   "hello",
		Priority:  domain.PriorityNormal,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidRecipient)
}

func TestNotificationService_CreateBatch_Success(t *testing.T) {
	svc, repo, queue, _, _ := newTestNotificationService()

	batch, notifications, err := svc.CreateBatch(context.Background(), CreateBatchInput{
		Notifications: []CreateNotificationInput{
			{Channel: domain.ChannelSMS, Recipient: "+905530050594", Content: "msg1", Priority: domain.PriorityHigh},
			{Channel: domain.ChannelEmail, Recipient: "a@b.com", Content: "msg2", Priority: domain.PriorityNormal},
			{Channel: domain.ChannelPush, Recipient: "device-token", Content: "msg3", Priority: domain.PriorityLow},
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, 3, batch.TotalCount)
	assert.Equal(t, 3, batch.PendingCount)
	assert.Len(t, notifications, 3)
	assert.Len(t, queue.enqueued, 3)

	for _, n := range notifications {
		assert.NotNil(t, n.BatchID)
		assert.Equal(t, batch.ID, *n.BatchID)

		_, err := repo.GetByID(context.Background(), n.ID)
		require.NoError(t, err)
	}
}

func TestNotificationService_CreateBatch_Empty(t *testing.T) {
	svc, _, _, _, _ := newTestNotificationService()

	_, _, err := svc.CreateBatch(context.Background(), CreateBatchInput{
		Notifications: []CreateNotificationInput{},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrBatchEmpty)
}

func TestNotificationService_CreateBatch_TooLarge(t *testing.T) {
	svc, _, _, _, _ := newTestNotificationService()

	inputs := make([]CreateNotificationInput, 1001)
	for i := range inputs {
		inputs[i] = CreateNotificationInput{
			Channel:   domain.ChannelPush,
			Recipient: "device-token",
			Content:   "msg",
			Priority:  domain.PriorityLow,
		}
	}

	_, _, err := svc.CreateBatch(context.Background(), CreateBatchInput{Notifications: inputs})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrBatchTooLarge)
}

func TestNotificationService_Cancel_Success(t *testing.T) {
	svc, repo, _, _, _ := newTestNotificationService()

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	_ = repo.Create(context.Background(), n)

	err := svc.Cancel(context.Background(), n.ID)

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, domain.StatusCancelled, updated.Status)
}

func TestNotificationService_Cancel_AlreadyDelivered(t *testing.T) {
	svc, repo, _, _, _ := newTestNotificationService()

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	n.MarkDelivered("msg-123")
	_ = repo.Create(context.Background(), n)

	err := svc.Cancel(context.Background(), n.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
}

func TestNotificationService_Cancel_NotFound(t *testing.T) {
	svc, _, _, _, _ := newTestNotificationService()

	err := svc.Cancel(context.Background(), uuid.Must(uuid.NewV7()))

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotificationNotFound)
}

func TestNotificationService_GetByID(t *testing.T) {
	svc, repo, _, _, _ := newTestNotificationService()

	n, _ := domain.NewNotification(domain.ChannelEmail, "test@example.com", "hello", domain.PriorityHigh, nil)
	_ = repo.Create(context.Background(), n)

	result, err := svc.GetByID(context.Background(), n.ID)

	require.NoError(t, err)
	assert.Equal(t, n.ID, result.ID)
	assert.Equal(t, domain.ChannelEmail, result.Channel)
}

func TestNotificationService_GetBatch(t *testing.T) {
	svc, repo, _, _, _ := newTestNotificationService()

	batch := &domain.NotificationBatch{
		ID:         uuid.Must(uuid.NewV7()),
		TotalCount: 5,
		CreatedAt:  time.Now().UTC(),
	}
	repo.batches[batch.ID] = batch

	result, err := svc.GetBatch(context.Background(), batch.ID)

	require.NoError(t, err)
	assert.Equal(t, batch.ID, result.ID)
	assert.Equal(t, 5, result.TotalCount)
}

func TestNotificationService_Create_EnqueueError(t *testing.T) {
	svc, _, queue, _, _ := newTestNotificationService()

	queue.enqueueErr = assert.AnError

	_, err := svc.Create(context.Background(), CreateNotificationInput{
		Channel:   domain.ChannelSMS,
		Recipient: "+905530050594",
		Content:   "hello",
		Priority:  domain.PriorityNormal,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}
