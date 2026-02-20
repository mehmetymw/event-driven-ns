package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotification_ValidSMS(t *testing.T) {
	n, err := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)

	require.NoError(t, err)
	assert.Equal(t, ChannelSMS, n.Channel)
	assert.Equal(t, "+905530050594", n.Recipient)
	assert.Equal(t, "Hello", n.Content)
	assert.Equal(t, PriorityNormal, n.Priority)
	assert.Equal(t, StatusPending, n.Status)
	assert.Equal(t, 3, n.MaxRetries)
}

func TestNewNotification_ValidEmail(t *testing.T) {
	n, err := NewNotification(ChannelEmail, "test@example.com", "Hello Email", PriorityHigh, nil)

	require.NoError(t, err)
	assert.Equal(t, ChannelEmail, n.Channel)
	assert.Equal(t, 5, n.MaxRetries)
}

func TestNewNotification_ValidPush(t *testing.T) {
	n, err := NewNotification(ChannelPush, "device-token-abc", "Push content", PriorityLow, nil)

	require.NoError(t, err)
	assert.Equal(t, ChannelPush, n.Channel)
	assert.Equal(t, 2, n.MaxRetries)
}

func TestNewNotification_ScheduledStatus(t *testing.T) {
	scheduledTime := time.Now().Add(1 * time.Hour)
	n, err := NewNotification(ChannelSMS, "+905530050594", "Scheduled msg", PriorityNormal, &scheduledTime)

	require.NoError(t, err)
	assert.Equal(t, StatusScheduled, n.Status)
}

func TestNewNotification_InvalidChannel(t *testing.T) {
	_, err := NewNotification(Channel("fax"), "+905530050594", "Hello", PriorityNormal, nil)

	assert.ErrorIs(t, err, ErrInvalidChannel)
}

func TestNewNotification_EmptyRecipient(t *testing.T) {
	_, err := NewNotification(ChannelSMS, "", "Hello", PriorityNormal, nil)

	assert.ErrorIs(t, err, ErrEmptyRecipient)
}

func TestNewNotification_InvalidSMSRecipient(t *testing.T) {
	_, err := NewNotification(ChannelSMS, "not-a-phone", "Hello", PriorityNormal, nil)

	assert.ErrorIs(t, err, ErrInvalidRecipient)
}

func TestNewNotification_InvalidEmailRecipient(t *testing.T) {
	_, err := NewNotification(ChannelEmail, "not-an-email", "Hello", PriorityNormal, nil)

	assert.ErrorIs(t, err, ErrInvalidRecipient)
}

func TestNewNotification_EmptyContent(t *testing.T) {
	_, err := NewNotification(ChannelSMS, "+905530050594", "", PriorityNormal, nil)

	assert.ErrorIs(t, err, ErrEmptyContent)
}

func TestNewNotification_SMSContentTooLong(t *testing.T) {
	longContent := make([]byte, 161)
	for i := range longContent {
		longContent[i] = 'a'
	}
	_, err := NewNotification(ChannelSMS, "+905530050594", string(longContent), PriorityNormal, nil)

	assert.ErrorIs(t, err, ErrContentTooLong)
}

func TestNewNotification_InvalidPriority(t *testing.T) {
	_, err := NewNotification(ChannelSMS, "+905530050594", "Hello", Priority("urgent"), nil)

	assert.ErrorIs(t, err, ErrInvalidPriority)
}

func TestNotification_Cancel(t *testing.T) {
	n, _ := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)

	err := n.Cancel()

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, n.Status)
}

func TestNotification_CancelProcessing(t *testing.T) {
	n, _ := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)
	n.MarkProcessing()

	err := n.Cancel()

	assert.ErrorIs(t, err, ErrInvalidStatusTransition)
}

func TestNotification_CanCancel(t *testing.T) {
	n, _ := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)
	assert.True(t, n.CanCancel())

	n.MarkProcessing()
	assert.False(t, n.CanCancel())
}

func TestNotification_MarkDelivered(t *testing.T) {
	n, _ := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)
	n.MarkDelivered("provider-msg-123")

	assert.Equal(t, StatusDelivered, n.Status)
	assert.NotNil(t, n.SentAt)
	assert.Equal(t, "provider-msg-123", *n.ProviderMessageID)
}

func TestNotification_MarkFailed(t *testing.T) {
	n, _ := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)
	n.MarkFailed("provider timeout")

	assert.Equal(t, StatusFailed, n.Status)
	assert.NotNil(t, n.FailedAt)
	assert.Equal(t, "provider timeout", *n.ErrorMessage)
}

func TestNotification_RetryLogic(t *testing.T) {
	n, _ := NewNotification(ChannelSMS, "+905530050594", "Hello", PriorityNormal, nil)

	assert.True(t, n.HasRetriesLeft())

	n.IncrementRetry()
	n.IncrementRetry()
	n.IncrementRetry()

	assert.False(t, n.HasRetriesLeft())
	assert.Equal(t, 3, n.RetryCount)
}
