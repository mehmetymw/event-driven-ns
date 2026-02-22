package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/mehmetymw/event-driven-ns/internal/app"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type CreateNotificationRequest struct {
	Channel           string            `json:"channel" binding:"required,oneof=sms email push"`
	Recipient         string            `json:"recipient" binding:"required"`
	Content           string            `json:"content" binding:"required"`
	Priority          string            `json:"priority" binding:"required,oneof=high normal low"`
	ScheduledAt       *time.Time        `json:"scheduled_at,omitempty"`
	IdempotencyKey    *string           `json:"idempotency_key,omitempty"`
	TemplateID        *string           `json:"template_id,omitempty"`
	TemplateVariables map[string]string `json:"template_variables,omitempty"`
}

func (r *CreateNotificationRequest) ToInput() app.CreateNotificationInput {
	input := app.CreateNotificationInput{
		Channel:           domain.Channel(r.Channel),
		Recipient:         r.Recipient,
		Content:           r.Content,
		Priority:          domain.Priority(r.Priority),
		ScheduledAt:       r.ScheduledAt,
		IdempotencyKey:    r.IdempotencyKey,
		TemplateVariables: r.TemplateVariables,
	}

	if r.TemplateID != nil {
		id, err := uuid.Parse(*r.TemplateID)
		if err == nil {
			input.TemplateID = &id
		}
	}

	return input
}

type CreateBatchRequest struct {
	Notifications []CreateNotificationRequest `json:"notifications" binding:"required,min=1,max=1000,dive"`
}

type ListNotificationsRequest struct {
	Status   *string `form:"status"`
	Channel  *string `form:"channel"`
	DateFrom *string `form:"date_from"`
	DateTo   *string `form:"date_to"`
	Cursor   *string `form:"cursor"`
	PageSize int     `form:"page_size"`
}

func (r *ListNotificationsRequest) ToFilter() domain.NotificationFilter {
	filter := domain.NotificationFilter{
		PageSize: r.PageSize,
	}

	if r.Status != nil {
		s := domain.Status(*r.Status)
		filter.Status = &s
	}
	if r.Channel != nil {
		c := domain.Channel(*r.Channel)
		filter.Channel = &c
	}
	if r.DateFrom != nil {
		if t, err := time.Parse(time.RFC3339, *r.DateFrom); err == nil {
			filter.DateFrom = &t
		}
	}
	if r.DateTo != nil {
		if t, err := time.Parse(time.RFC3339, *r.DateTo); err == nil {
			filter.DateTo = &t
		}
	}
	if r.Cursor != nil {
		if id, err := uuid.Parse(*r.Cursor); err == nil {
			filter.Cursor = &id
		}
	}

	return filter
}

type NotificationResponse struct {
	ID                string            `json:"id"`
	BatchID           *string           `json:"batch_id,omitempty"`
	Channel           string            `json:"channel"`
	Recipient         string            `json:"recipient"`
	Content           string            `json:"content"`
	Priority          string            `json:"priority"`
	Status            string            `json:"status"`
	ScheduledAt       *time.Time        `json:"scheduled_at,omitempty"`
	SentAt            *time.Time        `json:"sent_at,omitempty"`
	FailedAt          *time.Time        `json:"failed_at,omitempty"`
	ErrorMessage      *string           `json:"error_message,omitempty"`
	RetryCount        int               `json:"retry_count"`
	MaxRetries        int               `json:"max_retries"`
	ProviderMessageID *string           `json:"provider_message_id,omitempty"`
	TemplateID        *string           `json:"template_id,omitempty"`
	TemplateVariables map[string]string `json:"template_variables,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

func NewNotificationResponse(n *domain.Notification) NotificationResponse {
	resp := NotificationResponse{
		ID:                n.ID.String(),
		Channel:           string(n.Channel),
		Recipient:         n.Recipient,
		Content:           n.Content,
		Priority:          string(n.Priority),
		Status:            string(n.Status),
		ScheduledAt:       n.ScheduledAt,
		SentAt:            n.SentAt,
		FailedAt:          n.FailedAt,
		ErrorMessage:      n.ErrorMessage,
		RetryCount:        n.RetryCount,
		MaxRetries:        n.MaxRetries,
		ProviderMessageID: n.ProviderMessageID,
		TemplateVariables: n.TemplateVariables,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
	}

	if n.BatchID != nil {
		s := n.BatchID.String()
		resp.BatchID = &s
	}
	if n.TemplateID != nil {
		s := n.TemplateID.String()
		resp.TemplateID = &s
	}

	return resp
}

func NewNotificationListResponse(notifications []*domain.Notification, pageSize int) ListResponse[NotificationResponse] {
	data := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		data[i] = NewNotificationResponse(n)
	}

	var nextCursor *string
	if len(notifications) == pageSize {
		last := notifications[len(notifications)-1].ID.String()
		nextCursor = &last
	}

	return ListResponse[NotificationResponse]{
		Data:       data,
		NextCursor: nextCursor,
	}
}

type BatchResponse struct {
	ID             string    `json:"id"`
	TotalCount     int       `json:"total_count"`
	PendingCount   int       `json:"pending_count"`
	DeliveredCount int       `json:"delivered_count"`
	FailedCount    int       `json:"failed_count"`
	CancelledCount int       `json:"cancelled_count"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewBatchResponse(b *domain.NotificationBatch) BatchResponse {
	return BatchResponse{
		ID:             b.ID.String(),
		TotalCount:     b.TotalCount,
		PendingCount:   b.PendingCount,
		DeliveredCount: b.DeliveredCount,
		FailedCount:    b.FailedCount,
		CancelledCount: b.CancelledCount,
		CreatedAt:      b.CreatedAt,
	}
}

type CreateBatchResponse struct {
	Batch         BatchResponse          `json:"batch"`
	Notifications []NotificationResponse `json:"notifications"`
}

func NewCreateBatchResponse(b *domain.NotificationBatch, notifications []*domain.Notification) CreateBatchResponse {
	notifs := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		notifs[i] = NewNotificationResponse(n)
	}
	return CreateBatchResponse{
		Batch:         NewBatchResponse(b),
		Notifications: notifs,
	}
}
