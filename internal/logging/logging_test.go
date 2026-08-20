package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJSONLoggerDefaultsToInfoForLoki(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewJSONLogger(&output, "")
	require.NoError(t, err)

	logger.Debug("hidden diagnostic", "component", "openapi")
	logger.Info(
		"OAS refresh completed",
		"component", "oas_refresh",
		"operation", "refresh",
		"api_id", "api-123",
	)

	var event map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &event))
	assert.Equal(t, "INFO", event["level"])
	assert.Equal(t, "OAS refresh completed", event["msg"])
	assert.Equal(t, "oas_refresh", event["component"])
	assert.Equal(t, "refresh", event["operation"])
	assert.Equal(t, "api-123", event["api_id"])
}

func TestNewJSONLoggerAddsApplicationIdentity(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewJSONLogger(&output, "info")
	require.NoError(t, err)

	logger.Info("application event")

	var event map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &event))
	assert.Equal(t, "api-register", event["app"])
}

func TestNewJSONLoggerHonoursConfiguredMinimumLevel(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewJSONLogger(&output, "warn")
	require.NoError(t, err)

	logger.Info("hidden informational event")
	logger.Warn("actionable fallback", "component", "openapi")

	var event map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &event))
	assert.Equal(t, "WARN", event["level"])
	assert.Equal(t, "actionable fallback", event["msg"])
	assert.Equal(t, "openapi", event["component"])
}

func TestNewJSONLoggerSupportsDocumentedLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			logger, err := NewJSONLogger(&bytes.Buffer{}, level)
			require.NoError(t, err)
			assert.True(t, logger.Enabled(t.Context(), configuredSlogLevel(level)))
		})
	}
}

func TestNewJSONLoggerRejectsUnknownLevel(t *testing.T) {
	logger, err := NewJSONLogger(&bytes.Buffer{}, "verbose")

	assert.Nil(t, logger)
	assert.EqualError(t, err, `unsupported LOG_LEVEL "verbose"; use debug, info, warn or error`)
}

func TestCronLoggerUsesStructuredSeverityAndContext(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewJSONLogger(&output, "debug")
	require.NoError(t, err)
	cronLogger := NewCronLogger(logger, "harvest")

	cronLogger.Info("job delayed", "duration", "1s")
	cronLogger.Error(errors.New("panic value"), "job panicked", "stack", "trace")

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	require.Len(t, lines, 2)

	var infoEvent map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &infoEvent))
	assert.Equal(t, "INFO", infoEvent["level"])
	assert.Equal(t, "harvest", infoEvent["component"])
	assert.Equal(t, "scheduler", infoEvent["operation"])
	assert.Equal(t, "1s", infoEvent["duration"])

	var errorEvent map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &errorEvent))
	assert.Equal(t, "ERROR", errorEvent["level"])
	assert.Equal(t, "panic value", errorEvent["error"])
	assert.Equal(t, "trace", errorEvent["stack"])
}

func TestSlogWriterConvertsFrameworkOutputToStructuredEvent(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewJSONLogger(&output, "debug")
	require.NoError(t, err)
	writer := NewSlogWriter(logger, slog.LevelInfo, "http_server", "access")

	written, err := writer.Write([]byte("[GIN] GET /v1/apis 200\n"))
	require.NoError(t, err)
	assert.Equal(t, len("[GIN] GET /v1/apis 200\n"), written)

	var event map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &event))
	assert.Equal(t, "INFO", event["level"])
	assert.Equal(t, "[GIN] GET /v1/apis 200", event["msg"])
	assert.Equal(t, "http_server", event["component"])
	assert.Equal(t, "access", event["operation"])
}

func TestGinMiddlewareClassifiesServerResponsesAndAddsRequestFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger, err := NewJSONLogger(&output, "info")
	require.NoError(t, err)

	router := gin.New()
	router.Use(NewGinMiddleware(logger))
	router.GET("/ok/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/failed", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	for _, path := range []string{"/ok/api-123?token=must-not-be-logged", "/failed"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	require.Len(t, lines, 2)

	var okEvent map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &okEvent))
	assert.Equal(t, "INFO", okEvent["level"])
	assert.Equal(t, "http_server", okEvent["component"])
	assert.Equal(t, "request", okEvent["operation"])
	assert.Equal(t, "GET", okEvent["method"])
	assert.Equal(t, "/ok/:id", okEvent["route"])
	assert.Equal(t, "/ok/api-123", okEvent["path"])
	assert.Equal(t, float64(http.StatusNoContent), okEvent["status_code"])
	assert.NotContains(t, output.String(), "must-not-be-logged")

	var failedEvent map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &failedEvent))
	assert.Equal(t, "ERROR", failedEvent["level"])
	assert.Equal(t, float64(http.StatusInternalServerError), failedEvent["status_code"])
}

func configuredSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
