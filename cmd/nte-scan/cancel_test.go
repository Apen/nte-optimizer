package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContextWithCancelFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancel")
	ctx, cancel := contextWithCancelFile(context.Background(), path)
	defer cancel()
	if err := os.WriteFile(path, []byte("cancel"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancel file did not cancel context")
	}
}

func TestContextWithCancelFileStopsWithParent(t *testing.T) {
	parent, stopParent := context.WithCancel(context.Background())
	ctx, cancel := contextWithCancelFile(parent, filepath.Join(t.TempDir(), "missing"))
	defer cancel()
	stopParent()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("parent cancellation was not propagated")
	}
}
