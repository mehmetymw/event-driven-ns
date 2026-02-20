package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/pkg/logger"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

func Logging(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		correlationID := logger.CorrelationIDFromContext(c.Request.Context())

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("correlation_id", correlationID),
			zap.String("trace_id", tracing.TraceIDFromContext(c.Request.Context())),
			zap.String("span_id", tracing.SpanIDFromContext(c.Request.Context())),
			zap.String("client_ip", c.ClientIP()),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
			log.Error("http request", fields...)
		} else {
			log.Info("http request", fields...)
		}
	}
}
