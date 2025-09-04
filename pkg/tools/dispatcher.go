package tools

import (
	"context"
	"log"
)

// ToolFunc defines a function executed asynchronously.
type ToolFunc func(ctx context.Context) error

// Dispatch runs the provided tool in a separate goroutine. fire-and-forget solution
func Dispatch(ctx context.Context, name string, fn ToolFunc) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[tool:%s] panic: %v", name, r)
			}
		}()
		log.Printf("[tool:%s] start", name)
		if err := fn(ctx); err != nil {
			log.Printf("[tool:%s] error: %v", name, err)
		} else {
			log.Printf("[tool:%s] done", name)
		}
	}()
}
