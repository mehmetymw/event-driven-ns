package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTemplate_Valid(t *testing.T) {
	tmpl, err := NewTemplate("welcome", ChannelSMS, "Hello {{.Name}}")

	require.NoError(t, err)
	assert.Equal(t, "welcome", tmpl.Name)
	assert.Equal(t, ChannelSMS, tmpl.Channel)
	assert.Equal(t, "Hello {{.Name}}", tmpl.Body)
}

func TestNewTemplate_EmptyName(t *testing.T) {
	_, err := NewTemplate("", ChannelSMS, "Hello")

	assert.ErrorIs(t, err, ErrEmptyTemplateName)
}

func TestNewTemplate_EmptyBody(t *testing.T) {
	_, err := NewTemplate("welcome", ChannelSMS, "")

	assert.ErrorIs(t, err, ErrEmptyTemplateBody)
}

func TestNewTemplate_InvalidChannel(t *testing.T) {
	_, err := NewTemplate("welcome", Channel("fax"), "Hello")

	assert.ErrorIs(t, err, ErrInvalidChannel)
}

func TestNewTemplate_InvalidBodySyntax(t *testing.T) {
	_, err := NewTemplate("broken", ChannelSMS, "Hello {{.Name")

	assert.ErrorIs(t, err, ErrInvalidTemplateBody)
}

func TestTemplate_Render(t *testing.T) {
	tmpl, _ := NewTemplate("welcome", ChannelSMS, "Hello {{.Name}}, code: {{.Code}}")

	result, err := tmpl.Render(map[string]string{
		"Name": "Mehmet",
		"Code": "1234",
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello Mehmet, code: 1234", result)
}

func TestTemplate_RenderNoVariables(t *testing.T) {
	tmpl, _ := NewTemplate("static", ChannelSMS, "No variables here")

	result, err := tmpl.Render(nil)

	require.NoError(t, err)
	assert.Equal(t, "No variables here", result)
}
