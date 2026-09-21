package main

import (
	"context"
	"os"
	"time"
)

func contextWithCancelFile(parent context.Context, path string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if path == "" {
		return ctx, cancel
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Stat(path); err == nil {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return ctx, cancel
}
