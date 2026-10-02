package scannerlauncher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var ErrScanAlreadyRunning = errors.New("scan already running")

// DiagnosticError carries a fixed reason code separately from the underlying
// error so logs can remain useful without copying command output or paths.
type DiagnosticError struct {
	Code string
	Err  error
}

func (e *DiagnosticError) Error() string { return e.Err.Error() }
func (e *DiagnosticError) Unwrap() error { return e.Err }

func ErrorCode(err error) string {
	var diagnostic *DiagnosticError
	if errors.As(err, &diagnostic) {
		return diagnostic.Code
	}
	return "scanner_helper_failed"
}

type elevatedRunner func(helper, projectDir, outputDir, cancelFile string, seconds int) error
type elevatedRecoveryRunner func(helper, projectDir, outputDir, cancelFile string, seconds int, stopPktmonFirst bool) error

// Run locates the separately built scanner helper and starts it with elevated
// privileges. The desktop optimizer itself deliberately stays non-elevated.
func Run(installDir, stateDir string, seconds int) error {
	return RunContext(context.Background(), installDir, stateDir, seconds)
}

func RunContext(ctx context.Context, installDir, stateDir string, seconds int) error {
	return run(ctx, installDir, stateDir, seconds, runElevated)
}

// RunContextAfterPktmonRecovery stops any existing system-wide Pktmon capture
// before starting a new guided scan, after an explicit user recovery action.
func RunContextAfterPktmonRecovery(ctx context.Context, installDir, stateDir string, seconds int) error {
	return runWithRecovery(ctx, installDir, stateDir, seconds, true, runElevatedWithRecovery)
}

func run(ctx context.Context, installDir, stateDir string, seconds int, elevate elevatedRunner) error {
	return runWithRecovery(ctx, installDir, stateDir, seconds, false, func(helper, projectDir, outputDir, cancelFile string, seconds int, _ bool) error {
		return elevate(helper, projectDir, outputDir, cancelFile, seconds)
	})
}

func runWithRecovery(ctx context.Context, installDir, stateDir string, seconds int, stopPktmonFirst bool, elevate elevatedRecoveryRunner) error {
	if runtime.GOOS != "windows" {
		return &DiagnosticError{Code: "scanner_unavailable", Err: fmt.Errorf("guided capture is available only on Windows")}
	}
	if seconds < 10 {
		seconds = 10
	}
	helper, err := findHelper(installDir)
	if err != nil {
		return &DiagnosticError{Code: "helper_missing", Err: err}
	}
	outputDir := OutputDir(stateDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return &DiagnosticError{Code: "output_not_writable", Err: fmt.Errorf("create scanner output directory: %w", err)}
	}
	if err := probeDirectoryWrite(outputDir); err != nil {
		return &DiagnosticError{Code: "output_not_writable", Err: fmt.Errorf("scanner output directory is not writable: %w", err)}
	}
	cancelFile, err := reserveCancelFile(stateDir)
	if err != nil {
		return &DiagnosticError{Code: "state_not_writable", Err: err}
	}
	defer os.Remove(cancelFile)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = os.WriteFile(cancelFile, []byte("cancel\n"), 0o600)
		case <-done:
		}
	}()
	if err := elevate(helper, installDir, outputDir, cancelFile, seconds, stopPktmonFirst); err != nil {
		var diagnostic *DiagnosticError
		if errors.As(err, &diagnostic) {
			return err
		}
		return &DiagnosticError{Code: "scanner_helper_failed", Err: err}
	}
	return nil
}

func probeDirectoryWrite(dir string) error {
	file, err := os.CreateTemp(dir, ".nte-scan-write-check-*")
	if err != nil {
		return err
	}
	path := file.Name()
	if _, err := file.Write([]byte{0}); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

func reserveCancelFile(stateDir string) (string, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", fmt.Errorf("create user directory: %w", err)
	}
	file, err := os.CreateTemp(stateDir, ".scan-cancel-*")
	if err != nil {
		return "", fmt.Errorf("prepare cancellation signal: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("prepare cancellation signal: %w", err)
	}
	return path, nil
}

func OutputDir(stateDir string) string {
	return filepath.Join(stateDir, "workspace", "scan-output")
}

func findHelper(projectDir string) (string, error) {
	candidates := []string{
		filepath.Join(projectDir, "build", "bin", "nte-scan.exe"),
		filepath.Join(projectDir, "nte-scan.exe"),
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(executable), "nte-scan.exe")}, candidates...)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("nte-scan.exe was not found; run scripts\\build-app.ps1 to build the application and scanner")
}
