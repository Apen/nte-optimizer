package exporter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishPreservesPreviousGenerationWhenStagingFails(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "scan-output")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(directory, "previous.json")
	if err := os.WriteFile(marker, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := PublishWithWriter(directory, []File{{Name: "next.json", Value: map[string]bool{"ok": true}}}, func(string, any) error { return os.ErrPermission })
	if err == nil {
		t.Fatal("staging failure was ignored")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("previous generation was not preserved: %v", err)
	}
}

func TestPublishRejectsInvalidJSON(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "scan-output")
	err := PublishWithWriter(directory, []File{{Name: "invalid.json"}}, func(path string, _ any) error {
		return os.WriteFile(path, []byte("invalid"), 0o644)
	})
	if err == nil {
		t.Fatal("invalid JSON was published")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("output exists after invalid staging: %v", err)
	}
}
