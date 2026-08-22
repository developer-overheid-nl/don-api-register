package tools

import (
	"context"
	"log/slog"
)

// ToolFunc defines a function executed asynchronously.
type ToolFunc func(ctx context.Context) error

// Dispatch runs the provided tool in a separate goroutine. fire-and-forget solution
func Dispatch(ctx context.Context, name string, fn ToolFunc) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(
					ctx,
					"asynchronous tool panicked",
					"component", "tools",
					"operation", name,
					"panic", r,
				)
			}
		}()
		slog.DebugContext(ctx, "asynchronous tool started", "component", "tools", "operation", name)
		if err := fn(ctx); err != nil {
			slog.ErrorContext(
				ctx,
				"asynchronous tool failed",
				"component", "tools",
				"operation", name,
				"error", err,
			)
		} else {
			slog.DebugContext(ctx, "asynchronous tool completed", "component", "tools", "operation", name)
		}
	}()
}
