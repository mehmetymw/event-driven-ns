package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mehmetymw/event-driven-ns/internal/app"
)

type MetricsHandler struct {
	collector *app.MetricsCollector
}

func NewMetricsHandler(collector *app.MetricsCollector) *MetricsHandler {
	return &MetricsHandler{collector: collector}
}

func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, h.collector.Snapshot(c.Request.Context()))
}
