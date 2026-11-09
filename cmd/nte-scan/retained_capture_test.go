package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	scannerpcap "nte-optimizer/internal/scanner/pcap"
)

func TestRetainedCaptureSurvivesTemporaryCleanupAndLaterRetention(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	if err := os.MkdirAll(input, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(input, "synthetic.pcapng")
	data := []byte("synthetic capture bytes, not a real account")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(root, "workspace", "scan-output")
	first, size, err := retainDiagnosticCapture(source, outputDir)
	if err != nil || size != int64(len(data)) {
		t.Fatalf("retain: size=%d err=%v", size, err)
	}
	second, _, err := retainDiagnosticCapture(source, outputDir)
	if err != nil || first == second {
		t.Fatalf("retention overwrote previous capture: %v", err)
	}
	if filepath.Dir(first) != filepath.Join(root, "workspace", "diagnostic-captures") {
		t.Fatal("capture retained under temporary input")
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("retained bytes lost after input cleanup: %v", err)
		}
	}
}

func TestRetainedCaptureRejectsOversizedInputWithoutPublishing(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "oversized.pcapng")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxRetainedCaptureBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	path, _, err := retainDiagnosticCapture(source, filepath.Join(root, "workspace", "scan-output"))
	if !errors.Is(err, errCaptureRetentionBudget) || path != "" || captureRetentionFailureCode(err) != "capture_retention_budget_exceeded" {
		t.Fatal("oversized capture published")
	}
}

func TestRetainedCapturePreservesInputAboveDecodeBudget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "synthetic-large.pcapng")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce a file that cannot be decoded in memory but still needs to be
	// retained for bounded offline filtering. No real packet data is used.
	const size = (256 << 20) + 1
	if err := f.Truncate(size); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{0x5a}, size-1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := scannerpcap.ReadFile(source); !errors.Is(err, scannerpcap.ErrCaptureBudgetExceeded) {
		t.Fatalf("fixture does not exceed decoder budget: %v", err)
	}
	retained, copied, err := retainDiagnosticCapture(source, filepath.Join(root, "workspace", "scan-output"))
	if err != nil || copied != size {
		t.Fatalf("private retention incorrectly tied to decode budget: bytes=%d err=%v", copied, err)
	}
	r, err := os.Open(retained)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var tail [1]byte
	if _, err := r.ReadAt(tail[:], size-1); err != nil || tail[0] != 0x5a {
		t.Fatalf("retained capture truncated: %v", err)
	}
	if info, err := os.Stat(source); err != nil || info.Size() != size {
		t.Fatalf("source was altered: %v", err)
	}
}
