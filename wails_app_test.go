package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/scanlog"
)

func TestStopOptimizationCancelsActiveSearch(t *testing.T) {
	app := NewDesktopApp(t.TempDir())
	if app.StopOptimization() {
		t.Fatal("reported an active search before one was registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.searchStop = cancel
	if !app.StopOptimization() {
		t.Fatal("active search was not stopped")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("search context was not cancelled")
	}
}

func TestStopScanCancelsActiveCapture(t *testing.T) {
	app := NewDesktopApp(t.TempDir())
	if app.StopScan() {
		t.Fatal("reported an active scan before one was registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.scanStop = cancel
	if !app.StopScan() {
		t.Fatal("active scan was not stopped")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("scan context was not cancelled")
	}
}

func TestLastScanLogReadsLocalDiagnostic(t *testing.T) {
	app := NewDesktopAppWithStateDir(t.TempDir(), t.TempDir())
	if err := scanlog.Start(scanlog.Path(app.stateDir), 35); err != nil {
		t.Fatal(err)
	}
	events, err := app.LastScanLog()
	if err != nil || len(events) != 1 || events[0].Stage != "scan" {
		t.Fatalf("scan log=%#v err=%v", events, err)
	}
}

func TestShutdownCancelsActiveWork(t *testing.T) {
	app := NewDesktopApp(t.TempDir())
	searchContext, stopSearch := context.WithCancel(context.Background())
	scanContext, stopScan := context.WithCancel(context.Background())
	app.searchStop = stopSearch
	app.scanStop = stopScan
	app.shutdown(context.Background())
	for name, ctx := range map[string]context.Context{"search": searchContext, "scan": scanContext} {
		select {
		case <-ctx.Done():
		default:
			t.Fatalf("%s context was not cancelled", name)
		}
	}
}

func TestAppendAncestorsFindsInstallationWithoutImportedAccount(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "optimizer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data", "recommendations", "targets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "data", "optimizer", "config.json")} {
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := filepath.Join(root, "build", "bin")
	paths := appendAncestors(nil, start, 4)
	found := false
	for _, path := range paths {
		if isProjectDir(path) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("project root not found in %#v", paths)
	}
}

func TestIsProjectDirRejectsMissingStaticData(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "recommendations", "targets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if isProjectDir(root) {
		t.Fatal("directory without optimizer config was accepted")
	}
}

func TestResolveUserDataDirUsesLocalAppData(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	got, err := resolveUserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "NTE Optimizer")
	if got != want {
		t.Fatalf("resolveUserDataDir() = %q, want %q", got, want)
	}
}

func TestDesktopAppSeparatesInstallationAndUserState(t *testing.T) {
	installDir := t.TempDir()
	stateDir := t.TempDir()
	app := NewDesktopAppWithStateDir(installDir, stateDir)
	if app.projectDir != installDir || app.stateDir != stateDir {
		t.Fatalf("unexpected paths: install=%q state=%q", app.projectDir, app.stateDir)
	}
	if app.service.DataDir != filepath.Join(installDir, "data") {
		t.Fatalf("static data directory = %q", app.service.DataDir)
	}
}

func TestPrepareUserDataDirMigratesLegacyWorkspaceOnce(t *testing.T) {
	installDir := t.TempDir()
	userDataDir := t.TempDir()
	legacyFile := filepath.Join(installDir, "workspace", "account_snapshot.json")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareUserDataDir(installDir, userDataDir); err != nil {
		t.Fatal(err)
	}
	migrated := filepath.Join(userDataDir, "workspace", "account_snapshot.json")
	if data, err := os.ReadFile(migrated); err != nil || string(data) != "legacy" {
		t.Fatalf("migrated workspace = %q, %v", data, err)
	}
	if err := os.WriteFile(legacyFile, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareUserDataDir(installDir, userDataDir); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(migrated); err != nil || string(data) != "legacy" {
		t.Fatalf("existing user workspace was replaced: %q, %v", data, err)
	}
}
