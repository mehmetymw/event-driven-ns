package domain

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Channel string

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusScheduled  Status = "scheduled"
	StatusProcessing Status = "processing"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

var (
	e164Regex  = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

var channelContentLimits = map[Channel]int{
	ChannelSMS:   160,
	ChannelEmail: 10000,
	ChannelPush:  4096,
}

var priorityMaxRetries = map[Priority]int{
	PriorityHigh:   5,
	PriorityNormal: 3,
	PriorityLow:    2,
}

type Notification struct {
	ID                uuid.UUID
	BatchID           *uuid.UUID
	IdempotencyKey    *string
	Channel           Channel
	Recipient         string
	Content           string
	Priority          Priority
	Status            Status
	ScheduledAt       *time.Time
	SentAt            *time.Time
	FailedAt          *time.Time
	ErrorMessage      *string
	RetryCount        int
	MaxRetries        int
	ProviderMessageID *string
	TemplateID        *uuid.UUID
	TemplateVariables map[string]string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type NotificationBatch struct {
	ID             uuid.UUID `db:"id"`
	TotalCount     int       `db:"total_count"`
	PendingCount   int       `db:"pending_count"`
	DeliveredCount int       `db:"delivered_count"`
	FailedCount    int       `db:"failed_count"`
	CancelledCount int       `db:"cancelled_count"`
	CreatedAt      time.Time `db:"created_at"`
}

type ChannelStats struct {
	Channel      string  `db:"channel"`
	Sent         int64   `db:"sent"`
	Failed       int64   `db:"failed"`
	AvgLatencyMs float64 `db:"avg_latency_ms"`
}

type NotificationFilter struct {
	Status   *Status
	Channel  *Channel
	DateFrom *time.Time
	DateTo   *time.Time
	BatchID  *uuid.UUID
	Cursor   *uuid.UUID
	PageSize int
}

func NewNotification(channel Channel, recipient, content string, priority Priority, scheduledAt *time.Time) (*Notification, error) {
	if err := validateChannel(channel); err != nil {
		return nil, err
	}
	if err := validateRecipient(channel, recipient); err != nil {
		return nil, err
	}
	if err := validateContent(channel, content); err != nil {
		return nil, err
	}
	if err := validatePriority(priority); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	status := StatusPending
	if scheduledAt != nil {
		status = StatusScheduled
	}

	return &Notification{
		ID:          uuid.Must(uuid.NewV7()),
		Channel:     channel,
		Recipient:   recipient,
		Content:     content,
		Priority:    priority,
		Status:      status,
		MaxRetries:  priorityMaxRetries[priority],
		ScheduledAt: scheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (n *Notification) CanCancel() bool {
	return n.Status == StatusPending || n.Status == StatusScheduled
}

func (n *Notification) Cancel() error {
	if !n.CanCancel() {
		return fmt.Errorf("%w: current status is %s", ErrInvalidStatusTransition, n.Status)
	}
	n.Status = StatusCancelled
	n.UpdatedAt = time.Now().UTC()
	return nil
}

func (n *Notification) MarkProcessing() {
	n.Status = StatusProcessing
	n.UpdatedAt = time.Now().UTC()
}

func (n *Notification) MarkDelivered(providerMessageID string) {
	now := time.Now().UTC()
	n.Status = StatusDelivered
	n.ProviderMessageID = &providerMessageID
	n.SentAt = &now
	n.UpdatedAt = now
}

func (n *Notification) MarkFailed(errMsg string) {
	now := time.Now().UTC()
	n.Status = StatusFailed
	n.ErrorMessage = &errMsg
	n.FailedAt = &now
	n.UpdatedAt = now
}

func (n *Notification) IncrementRetry() {
	n.RetryCount++
	n.UpdatedAt = time.Now().UTC()
}

func (n *Notification) HasRetriesLeft() bool {
	return n.RetryCount < n.MaxRetries
}

func validateChannel(ch Channel) error {
	switch ch {
	case ChannelSMS, ChannelEmail, ChannelPush:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidChannel, ch)
	}
}

func validateRecipient(ch Channel, recipient string) error {
	if recipient == "" {
		return ErrEmptyRecipient
	}

	switch ch {
	case ChannelSMS:
		if !e164Regex.MatchString(recipient) {
			return fmt.Errorf("%w: must be E.164 format", ErrInvalidRecipient)
		}
	case ChannelEmail:
		if !emailRegex.MatchString(recipient) {
			return fmt.Errorf("%w: must be valid email", ErrInvalidRecipient)
		}
	case ChannelPush:
		if len(recipient) < 1 {
			return fmt.Errorf("%w: device token required", ErrInvalidRecipient)
		}
	}

	return nil
}

func validateContent(ch Channel, content string) error {
	if content == "" {
		return ErrEmptyContent
	}

	limit, ok := channelContentLimits[ch]
	if !ok {
		return fmt.Errorf("%w: %s", ErrInvalidChannel, ch)
	}

	if len(content) > limit {
		return fmt.Errorf("%w: max %d characters for %s", ErrContentTooLong, limit, ch)
	}

	return nil
}

func validatePriority(p Priority) error {
	switch p {
	case PriorityHigh, PriorityNormal, PriorityLow:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidPriority, p)
	}
}
