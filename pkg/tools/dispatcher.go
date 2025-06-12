package tools

import "context"

// ToolFunc defines a function executed asynchronously.
type ToolFunc func(ctx context.Context) error

// Dispatch runs the provided tool in a separate goroutine. fire-and-forget solution
func Dispatch(ctx context.Context, _ string, fn ToolFunc) {
	go func() {
		_ = fn(ctx)
	}()
}
