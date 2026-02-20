package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

func newTestScheduler() (*Scheduler, *mockNotificationRepo, *mockQueuePublisher) {
	repo := newMockNotificationRepo()
	publisher := newMockQueuePublisher()
	logger := zap.NewNop()
	s := NewScheduler(repo, publisher, logger)
	s.interval = 100 * time.Millisecond
	return s, repo, publisher
}

func TestScheduler_ProcessScheduled(t *testing.T) {
	s, repo, publisher := newTestScheduler()

	past := time.Now().Add(-1 * time.Minute)
	n1, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "scheduled1", domain.PriorityNormal, &past)
	n2, _ := domain.NewNotification(domain.ChannelEmail, "test@example.com", "scheduled2", domain.PriorityHigh, &past)
	_ = repo.Create(context.Background(), n1)
	_ = repo.Create(context.Background(), n2)

	repo.dueScheduled = []*domain.Notification{n1, n2}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	assert.GreaterOrEqual(t, len(publisher.enqueued), 2)
	assert.Equal(t, domain.StatusPending, publisher.enqueued[0].Status)
	assert.Equal(t, domain.StatusPending, publisher.enqueued[1].Status)
}

func TestScheduler_ProcessScheduled_Empty(t *testing.T) {
	s, repo, publisher := newTestScheduler()

	repo.dueScheduled = []*domain.Notification{}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	assert.Empty(t, publisher.enqueued)
}

func TestScheduler_RecoverStuck(t *testing.T) {
	s, repo, publisher := newTestScheduler()

	n, _ := domain.NewNotification(domain.ChannelPush, "device-token", "stuck", domain.PriorityNormal, nil)
	n.MarkProcessing()
	n.UpdatedAt = time.Now().Add(-10 * time.Minute)
	_ = repo.Create(context.Background(), n)

	repo.stuckItems = []*domain.Notification{n}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	assert.GreaterOrEqual(t, len(publisher.enqueued), 1)
	assert.Equal(t, domain.StatusPending, publisher.enqueued[0].Status)
}

func TestScheduler_EnqueueError(t *testing.T) {
	s, repo, publisher := newTestScheduler()

	publisher.enqueueErr = assert.AnError

	past := time.Now().Add(-1 * time.Minute)
	n, _ := domain.NewNotification(domain.ChannelSMS, "+905530050594", "will fail", domain.PriorityLow, &past)
	_ = repo.Create(context.Background(), n)
	repo.dueScheduled = []*domain.Notification{n}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	require.NotPanics(t, func() {
		s.Run(ctx)
	})
}

func TestScheduler_ContextCancellation(t *testing.T) {
	s, _, _ := newTestScheduler()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not exit on context cancellation")
	}
}
