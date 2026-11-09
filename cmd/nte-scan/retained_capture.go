package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Retention copies are streamed to disk and need not fit the in-memory decode
// budget. Keep a separate finite bound so oversized captures remain available
// for offline filtering without allowing unbounded copies.
const maxRetainedCaptureBytes = 1 << 30

var errCaptureRetentionBudget = errors.New("capture exceeds private retention budget")

func captureRetentionFailureCode(err error) string {
	if errors.Is(err, errCaptureRetentionBudget) {
		return "capture_retention_budget_exceeded"
	}
	return "capture_retention_failed"
}

// Keep retained captures outside input: that directory is cleared on each scan.
// Only the converted PCAPNG is retained, before decoding can fail. The ETL
// remains subject to the normal temporary-capture lifecycle.
func retainDiagnosticCapture(source, outputDir string) (path string, size int64, err error) {
	input, err := os.Open(source)
	if err != nil {
		return "", 0, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("capture is not a regular file")
	}
	if info.Size() > maxRetainedCaptureBytes {
		return "", info.Size(), errCaptureRetentionBudget
	}
	dir := filepath.Join(filepath.Dir(outputDir), "diagnostic-captures")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", 0, err
	}
	output, err := os.CreateTemp(dir, "login-*.pcapng")
	if err != nil {
		return "", 0, err
	}
	path = output.Name()
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(path))
			path = ""
		}
	}()
	size, copyErr := io.Copy(output, io.LimitReader(input, maxRetainedCaptureBytes+1))
	err = errors.Join(copyErr, output.Close())
	if err != nil {
		return path, size, err
	}
	if size > maxRetainedCaptureBytes {
		return path, size, errCaptureRetentionBudget
	}
	return path, size, nil
}
