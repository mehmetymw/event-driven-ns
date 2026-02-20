package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
)

func newTestDeliveryService() (*DeliveryService, *mockNotificationRepo, *mockDeliveryProvider, *mockBroadcaster, *MetricsCollector) {
	repo := newMockNotificationRepo()
	provider := &mockDeliveryProvider{
		response: &port.ProviderResponse{
			MessageID: "provider-msg-001",
			Status:    "accepted",
			Timestamp: "2026-01-01T00:00:00Z",
		},
	}
	broadcaster := &mockBroadcaster{}
	metrics := NewMetricsCollector(repo)
	logger := zap.NewNop()
	svc := NewDeliveryService(repo, provider, broadcaster, metrics, logger)
	return svc, repo, provider, broadcaster, metrics
}

func TestDeliveryService_ProcessDelivery_Success(t *testing.T) {
	svc, repo, _, broadcaster, metrics := newTestDeliveryService()

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, domain.StatusDelivered, updated.Status)
	assert.NotNil(t, updated.ProviderMessageID)
	assert.Equal(t, "provider-msg-001", *updated.ProviderMessageID)

	assert.Len(t, broadcaster.broadcasts, 1)
	assert.Equal(t, n.ID.String(), broadcaster.broadcasts[0].NotificationID)
	assert.Equal(t, string(domain.StatusDelivered), broadcaster.broadcasts[0].Status)

	snapshot := metrics.Snapshot(context.Background())
	assert.Equal(t, int64(1), snapshot.Channels["sms"].Sent)
}

func TestDeliveryService_ProcessDelivery_TransientError_WithRetry(t *testing.T) {
	svc, repo, provider, _, metrics := newTestDeliveryService()

	provider.response = nil
	provider.err = fmt.Errorf("%w: connection reset", domain.ErrProviderUnavailable)

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrProviderUnavailable)

	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, 1, updated.RetryCount)
	assert.NotEqual(t, domain.StatusFailed, updated.Status)

	snapshot := metrics.Snapshot(context.Background())
	assert.Equal(t, int64(0), snapshot.Channels["sms"].Failed)
}

func TestDeliveryService_ProcessDelivery_TransientError_RetriesExhausted(t *testing.T) {
	svc, repo, provider, broadcaster, _ := newTestDeliveryService()

	provider.response = nil
	provider.err = fmt.Errorf("%w: connection reset", domain.ErrProviderUnavailable)

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	n.RetryCount = n.MaxRetries
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, domain.StatusFailed, updated.Status)
	assert.NotNil(t, updated.ErrorMessage)

	assert.Len(t, broadcaster.broadcasts, 1)
	assert.Equal(t, string(domain.StatusFailed), broadcaster.broadcasts[0].Status)
}

func TestDeliveryService_ProcessDelivery_PermanentError(t *testing.T) {
	svc, repo, provider, broadcaster, _ := newTestDeliveryService()

	provider.response = nil
	provider.err = fmt.Errorf("permanent provider error: status 400")

	n, _ := domain.NewNotification(domain.ChannelEmail, "test@example.com", "hello", domain.PriorityHigh, nil)
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, domain.StatusFailed, updated.Status)

	assert.Len(t, broadcaster.broadcasts, 1)
}

func TestDeliveryService_ProcessDelivery_SkipCancelled(t *testing.T) {
	svc, repo, _, broadcaster, _ := newTestDeliveryService()

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	n.Status = domain.StatusCancelled
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.NoError(t, err)
	assert.Len(t, broadcaster.broadcasts, 0)
}

func TestDeliveryService_ProcessDelivery_SkipDelivered(t *testing.T) {
	svc, repo, _, broadcaster, _ := newTestDeliveryService()

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	n.MarkDelivered("already-delivered")
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.NoError(t, err)
	assert.Len(t, broadcaster.broadcasts, 0)
}

func TestDeliveryService_ProcessDelivery_NotFound(t *testing.T) {
	svc, _, _, _, _ := newTestDeliveryService()

	err := svc.ProcessDelivery(context.Background(), "019476cb-f13a-7000-8000-000000000001")

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotificationNotFound)
}

func TestDeliveryService_ProcessDelivery_InvalidID(t *testing.T) {
	svc, _, _, _, _ := newTestDeliveryService()

	err := svc.ProcessDelivery(context.Background(), "not-a-uuid")

	require.Error(t, err)
}

func TestDeliveryService_ProcessDelivery_BatchCounterOnSuccess(t *testing.T) {
	svc, repo, _, _, _ := newTestDeliveryService()

	batchID := uuid.Must(uuid.NewV7())
	batch := &domain.NotificationBatch{
		ID:           batchID,
		TotalCount:   2,
		PendingCount: 2,
	}
	repo.batches[batchID] = batch

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityNormal, nil)
	n.BatchID = &batchID
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())
	require.NoError(t, err)

	assert.Equal(t, 1, batch.DeliveredCount)
	assert.Equal(t, 1, batch.PendingCount)
}

func TestDeliveryService_ProcessDelivery_CircuitOpenRetry(t *testing.T) {
	svc, repo, provider, _, _ := newTestDeliveryService()

	provider.response = nil
	provider.err = domain.ErrCircuitOpen

	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "hello", domain.PriorityHigh, nil)
	_ = repo.Create(context.Background(), n)

	err := svc.ProcessDelivery(context.Background(), n.ID.String())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrCircuitOpen)

	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, 1, updated.RetryCount)
}
