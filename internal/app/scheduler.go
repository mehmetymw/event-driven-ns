package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
)

type Scheduler struct {
	repo      port.NotificationRepository
	publisher port.QueuePublisher
	logger    *zap.Logger
	interval  time.Duration
}

func NewScheduler(repo port.NotificationRepository, publisher port.QueuePublisher, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		interval:  5 * time.Second,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processScheduled(ctx)
			s.recoverStuck(ctx)
		}
	}
}

func (s *Scheduler) processScheduled(ctx context.Context) {
	notifications, err := s.repo.ListDueScheduled(ctx, 100)
	if err != nil {
		s.logger.Error("failed to list due scheduled notifications", zap.Error(err))
		return
	}

	for _, n := range notifications {
		n.Status = domain.StatusPending
		n.UpdatedAt = time.Now().UTC()

		if err := s.repo.UpdateStatus(ctx, n); err != nil {
			s.logger.Error("failed to update scheduled notification status",
				zap.String("id", n.ID.String()),
				zap.Error(err),
			)
			continue
		}

		if err := s.publisher.Enqueue(ctx, n); err != nil {
			s.logger.Error("failed to enqueue scheduled notification",
				zap.String("id", n.ID.String()),
				zap.Error(err),
			)
		}
	}

	if len(notifications) > 0 {
		s.logger.Info("processed scheduled notifications", zap.Int("count", len(notifications)))
	}
}

func (s *Scheduler) recoverStuck(ctx context.Context) {
	notifications, err := s.repo.ListStuckProcessing(ctx, 5*time.Minute, 50)
	if err != nil {
		s.logger.Error("failed to list stuck notifications", zap.Error(err))
		return
	}

	for _, n := range notifications {
		n.Status = domain.StatusPending
		n.UpdatedAt = time.Now().UTC()

		if err := s.repo.UpdateStatus(ctx, n); err != nil {
			s.logger.Error("failed to reset stuck notification",
				zap.String("id", n.ID.String()),
				zap.Error(err),
			)
			continue
		}

		if err := s.publisher.Enqueue(ctx, n); err != nil {
			s.logger.Error("failed to re-enqueue stuck notification",
				zap.String("id", n.ID.String()),
				zap.Error(err),
			)
		}
	}

	if len(notifications) > 0 {
		s.logger.Warn("recovered stuck notifications", zap.Int("count", len(notifications)))
	}
}
