package app

import (
	"context"
	"time"

	"github.com/mehmetymw/event-driven-ns/internal/port"
)

type MetricsCollector struct {
	repo port.NotificationRepository
}

func NewMetricsCollector(repo port.NotificationRepository) *MetricsCollector {
	return &MetricsCollector{repo: repo}
}

func (m *MetricsCollector) RecordSuccess(channel string, latency time.Duration) {}

func (m *MetricsCollector) RecordFailure(channel string) {}

type MetricsSnapshot struct {
	Channels map[string]ChannelSnapshot `json:"channels"`
}

type ChannelSnapshot struct {
	Sent         int64   `json:"sent"`
	Failed       int64   `json:"failed"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
}

func (m *MetricsCollector) Snapshot(ctx context.Context) MetricsSnapshot {
	snapshot := MetricsSnapshot{
		Channels: map[string]ChannelSnapshot{
			"sms":   {},
			"email": {},
			"push":  {},
		},
	}

	stats, err := m.repo.GetChannelMetrics(ctx)
	if err != nil {
		return snapshot
	}

	for _, s := range stats {
		total := s.Sent + s.Failed
		var successRate float64
		if total > 0 {
			successRate = float64(s.Sent) / float64(total) * 100
		}
		snapshot.Channels[s.Channel] = ChannelSnapshot{
			Sent:         s.Sent,
			Failed:       s.Failed,
			AvgLatencyMs: s.AvgLatencyMs,
			SuccessRate:  successRate,
		}
	}

	return snapshot
}
