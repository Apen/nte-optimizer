package scannerlauncher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOutputDir(t *testing.T) {
	projectDir := filepath.Join("test", "project")
	want := filepath.Join(projectDir, "workspace", "scan-output")
	if got := OutputDir(projectDir); got != want {
		t.Fatalf("OutputDir() = %q, want %q", got, want)
	}
}

func TestFindHelperPrefersBuildOutput(t *testing.T) {
	projectDir := t.TempDir()
	buildHelper := filepath.Join(projectDir, "build", "bin", "nte-scan.exe")
	rootHelper := filepath.Join(projectDir, "nte-scan.exe")
	writeTestHelper(t, buildHelper)
	writeTestHelper(t, rootHelper)

	got, err := findHelper(projectDir)
	if err != nil {
		t.Fatalf("findHelper() error = %v", err)
	}
	if got != buildHelper {
		t.Fatalf("findHelper() = %q, want %q", got, buildHelper)
	}
}

func TestFindHelperFallsBackToProjectRoot(t *testing.T) {
	projectDir := t.TempDir()
	rootHelper := filepath.Join(projectDir, "nte-scan.exe")
	writeTestHelper(t, rootHelper)

	got, err := findHelper(projectDir)
	if err != nil {
		t.Fatalf("findHelper() error = %v", err)
	}
	if got != rootHelper {
		t.Fatalf("findHelper() = %q, want %q", got, rootHelper)
	}
}

func TestFindHelperIgnoresDirectories(t *testing.T) {
	projectDir := t.TempDir()
	invalidHelper := filepath.Join(projectDir, "build", "bin", "nte-scan.exe")
	if err := os.MkdirAll(invalidHelper, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	rootHelper := filepath.Join(projectDir, "nte-scan.exe")
	writeTestHelper(t, rootHelper)

	got, err := findHelper(projectDir)
	if err != nil {
		t.Fatalf("findHelper() error = %v", err)
	}
	if got != rootHelper {
		t.Fatalf("findHelper() = %q, want %q", got, rootHelper)
	}
}

func TestRunPreparesOutputAndClampsDuration(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scanner elevation is Windows-only")
	}
	projectDir := t.TempDir()
	stateDir := t.TempDir()
	helper := filepath.Join(projectDir, "nte-scan.exe")
	writeTestHelper(t, helper)

	called := false
	err := run(context.Background(), projectDir, stateDir, 2, func(gotHelper, gotProjectDir, gotOutputDir, gotCancelFile string, gotSeconds int) error {
		called = true
		if gotHelper != helper || gotProjectDir != projectDir || gotOutputDir != OutputDir(stateDir) || gotSeconds != 10 {
			t.Fatalf("unexpected elevated arguments: %q, %q, %q, %d", gotHelper, gotProjectDir, gotOutputDir, gotSeconds)
		}
		if filepath.Dir(gotCancelFile) != stateDir {
			t.Fatalf("cancel file = %q, want parent %q", gotCancelFile, stateDir)
		}
		if info, statErr := os.Stat(gotOutputDir); statErr != nil || !info.IsDir() {
			t.Fatalf("output directory was not prepared: %v", statErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("elevated runner was not called")
	}
}

func TestRunPropagatesElevatedFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scanner elevation is Windows-only")
	}
	projectDir := t.TempDir()
	stateDir := t.TempDir()
	writeTestHelper(t, filepath.Join(projectDir, "nte-scan.exe"))
	want := errors.New("UAC refused")
	err := run(context.Background(), projectDir, stateDir, 30, func(_, _, _, _ string, _ int) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("run() error = %v, want %v", err, want)
	}
}

func TestRunFailsBeforeElevationWhenHelperIsMissing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scanner elevation is Windows-only")
	}
	called := false
	err := run(context.Background(), t.TempDir(), t.TempDir(), 30, func(_, _, _, _ string, _ int) error {
		called = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "introuvable") {
		t.Fatalf("run() error = %v", err)
	}
	if called {
		t.Fatal("elevation was attempted without a helper")
	}
}

func TestRunContextSignalsElevatedScannerCancellation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scanner elevation is Windows-only")
	}
	installDir := t.TempDir()
	stateDir := t.TempDir()
	writeTestHelper(t, filepath.Join(installDir, "nte-scan.exe"))
	ctx, cancel := context.WithCancel(context.Background())
	err := run(ctx, installDir, stateDir, 30, func(_, _, _, cancelFile string, _ int) error {
		cancel()
		for deadline := time.Now().Add(2 * time.Second); ; {
			if _, statErr := os.Stat(cancelFile); statErr == nil {
				return context.Canceled
			}
			if time.Now().After(deadline) {
				t.Fatal("cancel file was not created")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want cancellation", err)
	}
}

func writeTestHelper(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("test helper"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
