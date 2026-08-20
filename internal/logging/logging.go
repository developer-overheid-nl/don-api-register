package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const appName = "api-register"

// NewJSONLogger creates a structured logger that Loki can assign a detected
// level to. An empty configuredLevel defaults to info.
func NewJSONLogger(output io.Writer, configuredLevel string) (*slog.Logger, error) {
	level, err := parseLevel(configuredLevel)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level}))
	return logger.With("app", appName), nil
}

func parseLevel(configuredLevel string) (slog.Level, error) {
	level := strings.ToLower(strings.TrimSpace(configuredLevel))
	switch level {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf(
			"unsupported LOG_LEVEL %q; use debug, info, warn or error",
			configuredLevel,
		)
	}
}

// CronLogger adapts robfig/cron logging to the application's structured
// logger, including stable component and operation fields.
type CronLogger struct {
	logger    *slog.Logger
	component string
}

func NewCronLogger(logger *slog.Logger, component string) CronLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return CronLogger{logger: logger, component: component}
}

func (l CronLogger) Info(msg string, keysAndValues ...any) {
	attrs := []any{"component", l.component, "operation", "scheduler"}
	l.logger.Info(msg, append(attrs, keysAndValues...)...)
}

func (l CronLogger) Error(err error, msg string, keysAndValues ...any) {
	attrs := []any{
		"component", l.component,
		"operation", "scheduler",
		"error", err,
	}
	l.logger.Error(msg, append(attrs, keysAndValues...)...)
}

// SlogWriter converts line-oriented framework output into structured log
// events. It is intended for libraries that only accept an io.Writer.
type SlogWriter struct {
	logger    *slog.Logger
	level     slog.Level
	component string
	operation string
}

func NewSlogWriter(logger *slog.Logger, level slog.Level, component, operation string) SlogWriter {
	if logger == nil {
		logger = slog.Default()
	}
	return SlogWriter{
		logger:    logger,
		level:     level,
		component: component,
		operation: operation,
	}
}

func (w SlogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.logger.Log(
			context.Background(),
			w.level,
			msg,
			"component", w.component,
			"operation", w.operation,
		)
	}
	return len(p), nil
}
