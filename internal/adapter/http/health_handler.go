package http

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type HealthHandler struct {
	db           *sqlx.DB
	kafkaBrokers []string
}

func NewHealthHandler(db *sqlx.DB, kafkaBrokers []string) *HealthHandler {
	return &HealthHandler{db: db, kafkaBrokers: kafkaBrokers}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	checks := make(map[string]string)

	if err := h.db.PingContext(c.Request.Context()); err != nil {
		checks["database"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
		return
	}
	checks["database"] = "healthy"

	broker := h.kafkaBrokers[0]
	if !strings.Contains(broker, ":") {
		broker = broker + ":9092"
	}
	conn, err := net.DialTimeout("tcp", broker, 3e9)
	if err != nil {
		checks["kafka"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
		return
	}
	_ = conn.Close()
	checks["kafka"] = "healthy"

	c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
}
