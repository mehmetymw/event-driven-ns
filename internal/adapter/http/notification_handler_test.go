package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestCreateNotification_InvalidJSON(t *testing.T) {
	r := setupTestRouter()
	r.POST("/api/v1/notifications", func(c *gin.Context) {
		var req CreateNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
	})

	body := []byte(`{"invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Error)
}

func TestCreateNotification_MissingRequired(t *testing.T) {
	r := setupTestRouter()
	r.POST("/api/v1/notifications", func(c *gin.Context) {
		var req CreateNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	body := []byte(`{"channel":"sms"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateNotification_InvalidChannel(t *testing.T) {
	r := setupTestRouter()
	r.POST("/api/v1/notifications", func(c *gin.Context) {
		var req CreateNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	body := []byte(`{"channel":"fax","recipient":"+905530050594","content":"Hello","priority":"normal"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateNotification_ValidBinding(t *testing.T) {
	r := setupTestRouter()
	r.POST("/api/v1/notifications", func(c *gin.Context) {
		var req CreateNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"channel":   req.Channel,
			"recipient": req.Recipient,
			"content":   req.Content,
			"priority":  req.Priority,
		})
	})

	payload := CreateNotificationRequest{
		Channel:   "sms",
		Recipient: "+905530050594",
		Content:   "Hello World",
		Priority:  "high",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "sms", resp["channel"])
	assert.Equal(t, "+905530050594", resp["recipient"])
}

func TestCreateBatch_TooMany(t *testing.T) {
	r := setupTestRouter()
	r.POST("/api/v1/notifications/batch", func(c *gin.Context) {
		var req CreateBatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"count": len(req.Notifications)})
	})

	items := make([]CreateNotificationRequest, 1001)
	for i := range items {
		items[i] = CreateNotificationRequest{
			Channel:   "sms",
			Recipient: "+905530050594",
			Content:   "Hello",
			Priority:  "normal",
		}
	}
	payload := CreateBatchRequest{Notifications: items}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListNotifications_QueryParsing(t *testing.T) {
	r := setupTestRouter()
	r.GET("/api/v1/notifications", func(c *gin.Context) {
		var req ListNotificationsRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		filter := req.ToFilter()
		c.JSON(http.StatusOK, gin.H{"page_size": filter.PageSize})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?status=pending&page_size=50", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
