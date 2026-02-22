package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mehmetymw/event-driven-ns/internal/adapter/ws"
)

type WebSocketHandler struct {
	hub *ws.Hub
}

func NewWebSocketHandler(hub *ws.Hub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

func (h *WebSocketHandler) Handle(c *gin.Context) {
	if err := h.hub.Accept(c.Writer, c.Request); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "websocket upgrade failed"})
	}
}
