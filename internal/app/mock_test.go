package app

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
)

type mockNotificationRepo struct {
	mu            sync.Mutex
	notifications map[uuid.UUID]*domain.Notification
	batches       map[uuid.UUID]*domain.NotificationBatch
	createErr     error
	getByIDErr    error
	updateErr     error
	cancelErr     error
	listResult    []*domain.Notification
	listErr       error
	dueScheduled  []*domain.Notification
	stuckItems    []*domain.Notification
}

func newMockNotificationRepo() *mockNotificationRepo {
	return &mockNotificationRepo{
		notifications: make(map[uuid.UUID]*domain.Notification),
		batches:       make(map[uuid.UUID]*domain.NotificationBatch),
	}
}

func (m *mockNotificationRepo) Create(_ context.Context, n *domain.Notification) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications[n.ID] = n
	return nil
}

func (m *mockNotificationRepo) CreateBatch(_ context.Context, batch *domain.NotificationBatch, notifications []*domain.Notification) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches[batch.ID] = batch
	for _, n := range notifications {
		m.notifications[n.ID] = n
	}
	return nil
}

func (m *mockNotificationRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Notification, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.notifications[id]
	if !ok {
		return nil, domain.ErrNotificationNotFound
	}
	return n, nil
}

func (m *mockNotificationRepo) GetBatchByID(_ context.Context, batchID uuid.UUID) (*domain.NotificationBatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.batches[batchID]
	if !ok {
		return nil, domain.ErrBatchNotFound
	}
	return b, nil
}

func (m *mockNotificationRepo) List(_ context.Context, _ domain.NotificationFilter) ([]*domain.Notification, error) {
	return m.listResult, m.listErr
}

func (m *mockNotificationRepo) UpdateStatus(_ context.Context, n *domain.Notification) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications[n.ID] = n
	return nil
}

func (m *mockNotificationRepo) Cancel(_ context.Context, id uuid.UUID) error {
	if m.cancelErr != nil {
		return m.cancelErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.notifications[id]; ok {
		n.Status = domain.StatusCancelled
	}
	return nil
}

func (m *mockNotificationRepo) IncrementBatchCounter(_ context.Context, batchID uuid.UUID, status domain.Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.batches[batchID]
	if !ok {
		return nil
	}
	switch status {
	case domain.StatusDelivered:
		b.DeliveredCount++
		b.PendingCount--
	case domain.StatusFailed:
		b.FailedCount++
		b.PendingCount--
	case domain.StatusCancelled:
		b.CancelledCount++
		b.PendingCount--
	}
	return nil
}

func (m *mockNotificationRepo) ListDueScheduled(_ context.Context, _ int) ([]*domain.Notification, error) {
	return m.dueScheduled, nil
}

func (m *mockNotificationRepo) ListStuckProcessing(_ context.Context, _ time.Duration, _ int) ([]*domain.Notification, error) {
	return m.stuckItems, nil
}

func (m *mockNotificationRepo) GetChannelMetrics(_ context.Context) ([]domain.ChannelStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	channels := map[string]*domain.ChannelStats{}
	for _, n := range m.notifications {
		ch := string(n.Channel)
		if _, ok := channels[ch]; !ok {
			channels[ch] = &domain.ChannelStats{Channel: ch}
		}
		switch n.Status {
		case domain.StatusDelivered:
			channels[ch].Sent++
		case domain.StatusFailed:
			channels[ch].Failed++
		}
	}
	result := make([]domain.ChannelStats, 0, len(channels))
	for _, s := range channels {
		result = append(result, *s)
	}
	return result, nil
}

type mockQueuePublisher struct {
	mu             sync.Mutex
	enqueued       []*domain.Notification
	scheduledCount int
	enqueueErr     error
	scheduleErr    error
}

func newMockQueuePublisher() *mockQueuePublisher {
	return &mockQueuePublisher{}
}

func (m *mockQueuePublisher) Enqueue(_ context.Context, n *domain.Notification) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, n)
	return nil
}

func (m *mockQueuePublisher) EnqueueScheduled(_ context.Context, _ *domain.Notification) error {
	if m.scheduleErr != nil {
		return m.scheduleErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scheduledCount++
	return nil
}

func (m *mockQueuePublisher) Close() error { return nil }

type mockTemplateRepo struct {
	templates map[uuid.UUID]*domain.Template
	createErr error
}

func newMockTemplateRepo() *mockTemplateRepo {
	return &mockTemplateRepo{
		templates: make(map[uuid.UUID]*domain.Template),
	}
}

func (m *mockTemplateRepo) Create(_ context.Context, t *domain.Template) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.templates[t.ID] = t
	return nil
}

func (m *mockTemplateRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Template, error) {
	t, ok := m.templates[id]
	if !ok {
		return nil, domain.ErrTemplateNotFound
	}
	return t, nil
}

func (m *mockTemplateRepo) List(_ context.Context) ([]*domain.Template, error) {
	result := make([]*domain.Template, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result, nil
}

type mockIdempotencyStore struct {
	keys   map[string]string
	setErr error
}

func newMockIdempotencyStore() *mockIdempotencyStore {
	return &mockIdempotencyStore{keys: make(map[string]string)}
}

func (m *mockIdempotencyStore) Check(_ context.Context, key string) (bool, string, error) {
	val, ok := m.keys[key]
	return ok, val, nil
}

func (m *mockIdempotencyStore) SetNX(_ context.Context, key, notificationID string) (bool, error) {
	if m.setErr != nil {
		return false, m.setErr
	}
	if _, exists := m.keys[key]; exists {
		return false, nil
	}
	m.keys[key] = notificationID
	return true, nil
}

type mockDeliveryProvider struct {
	response *port.ProviderResponse
	err      error
}

func (m *mockDeliveryProvider) Send(_ context.Context, _ *domain.Notification) (*port.ProviderResponse, error) {
	return m.response, m.err
}

type mockBroadcaster struct {
	mu         sync.Mutex
	broadcasts []broadcastEvent
}

type broadcastEvent struct {
	NotificationID string
	Status         string
	Timestamp      string
}

func (m *mockBroadcaster) Broadcast(notificationID, status, timestamp string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcasts = append(m.broadcasts, broadcastEvent{
		NotificationID: notificationID,
		Status:         status,
		Timestamp:      timestamp,
	})
}
