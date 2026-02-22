package domain

import "errors"

var (
	ErrInvalidChannel          = errors.New("invalid channel")
	ErrInvalidRecipient        = errors.New("invalid recipient")
	ErrEmptyRecipient          = errors.New("recipient is required")
	ErrEmptyContent            = errors.New("content is required")
	ErrContentTooLong          = errors.New("content exceeds character limit")
	ErrInvalidPriority         = errors.New("invalid priority")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrNotificationNotFound    = errors.New("notification not found")
	ErrBatchNotFound           = errors.New("batch not found")
	ErrBatchTooLarge           = errors.New("batch exceeds maximum size of 1000")
	ErrBatchEmpty              = errors.New("batch must contain at least one notification")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
	ErrEmptyTemplateName       = errors.New("template name is required")
	ErrEmptyTemplateBody       = errors.New("template body is required")
	ErrInvalidTemplateBody     = errors.New("invalid template body syntax")
	ErrTemplateNotFound        = errors.New("template not found")
	ErrDuplicateTemplateName   = errors.New("template name already exists")
	ErrTemplateRenderFailed    = errors.New("template render failed")
	ErrProviderUnavailable     = errors.New("delivery provider unavailable")
	ErrCircuitOpen             = errors.New("circuit breaker is open")
)
