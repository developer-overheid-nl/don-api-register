package logging

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// NewGinMiddleware records one structured access event per completed request.
// Query strings are intentionally excluded from the logged path.
func NewGinMiddleware(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		level := slog.LevelInfo
		if c.Writer.Status() >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		logger.Log(
			c.Request.Context(),
			level,
			"HTTP request completed",
			"component", "http_server",
			"operation", "request",
			"method", c.Request.Method,
			"route", c.FullPath(),
			"path", c.Request.URL.Path,
			"status_code", c.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"response_bytes", c.Writer.Size(),
		)
	}
}
