package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mehmetymw/event-driven-ns/internal/app"
	"github.com/mehmetymw/event-driven-ns/internal/domain"
)

type NotificationHandler struct {
	service *app.NotificationService
}

func NewNotificationHandler(service *app.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) Create(c *gin.Context) {
	var req CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	notification, err := h.service.Create(c.Request.Context(), req.ToInput())
	if err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, NewNotificationResponse(notification))
}

func (h *NotificationHandler) CreateBatch(c *gin.Context) {
	var req CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	inputs := make([]app.CreateNotificationInput, len(req.Notifications))
	for i, n := range req.Notifications {
		inputs[i] = n.ToInput()
	}

	batch, notifications, err := h.service.CreateBatch(c.Request.Context(), app.CreateBatchInput{
		Notifications: inputs,
	})
	if err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, NewCreateBatchResponse(batch, notifications))
}

func (h *NotificationHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid notification id"})
		return
	}

	notification, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, NewNotificationResponse(notification))
}

func (h *NotificationHandler) List(c *gin.Context) {
	var req ListNotificationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	filter := req.ToFilter()
	notifications, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, NewNotificationListResponse(notifications, filter.PageSize))
}

func (h *NotificationHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid notification id"})
		return
	}

	if err := h.service.Cancel(c.Request.Context(), id); err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func (h *NotificationHandler) GetBatch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid batch id"})
		return
	}

	batch, err := h.service.GetBatch(c.Request.Context(), id)
	if err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, NewBatchResponse(batch))
}

func handleDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotificationNotFound),
		errors.Is(err, domain.ErrBatchNotFound),
		errors.Is(err, domain.ErrTemplateNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidChannel),
		errors.Is(err, domain.ErrInvalidRecipient),
		errors.Is(err, domain.ErrEmptyRecipient),
		errors.Is(err, domain.ErrEmptyContent),
		errors.Is(err, domain.ErrContentTooLong),
		errors.Is(err, domain.ErrInvalidPriority),
		errors.Is(err, domain.ErrBatchTooLarge),
		errors.Is(err, domain.ErrBatchEmpty),
		errors.Is(err, domain.ErrEmptyTemplateName),
		errors.Is(err, domain.ErrEmptyTemplateBody),
		errors.Is(err, domain.ErrInvalidTemplateBody):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidStatusTransition):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey),
		errors.Is(err, domain.ErrDuplicateTemplateName):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}
