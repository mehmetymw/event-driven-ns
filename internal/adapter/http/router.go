package http

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/docs"
	"github.com/mehmetymw/event-driven-ns/internal/adapter/http/middleware"
)

type RouterDeps struct {
	NotificationHandler *NotificationHandler
	TemplateHandler     *TemplateHandler
	HealthHandler       *HealthHandler
	MetricsHandler      *MetricsHandler
	WebSocketHandler    *WebSocketHandler
	Logger              *zap.Logger
}

func NewRouter(deps RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.CorrelationID())
	r.Use(middleware.Tracing())
	r.Use(middleware.Logging(deps.Logger))

	r.GET("/health", deps.HealthHandler.Liveness)
	r.GET("/health/ready", deps.HealthHandler.Readiness)

	r.GET("/ws", deps.WebSocketHandler.Handle)

	staticFS, _ := fs.Sub(docs.Static, ".")
	r.StaticFileFS("/swagger/openapi.yaml", "openapi.yaml", http.FS(staticFS))
	r.GET("/swagger/", func(c *gin.Context) {
		data, _ := docs.Static.ReadFile("swagger.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RateLimit(200))
	{
		notifications := v1.Group("/notifications")
		{
			notifications.POST("", deps.NotificationHandler.Create)
			notifications.POST("/batch", deps.NotificationHandler.CreateBatch)
			notifications.GET("", deps.NotificationHandler.List)
			notifications.GET("/:id", deps.NotificationHandler.GetByID)
			notifications.PATCH("/:id/cancel", deps.NotificationHandler.Cancel)
		}

		batches := v1.Group("/batches")
		{
			batches.GET("/:id", deps.NotificationHandler.GetBatch)
		}

		templates := v1.Group("/templates")
		{
			templates.POST("", deps.TemplateHandler.Create)
			templates.GET("", deps.TemplateHandler.List)
			templates.GET("/:id", deps.TemplateHandler.GetByID)
		}

		v1.GET("/metrics", deps.MetricsHandler.GetMetrics)
	}

	return r
}
