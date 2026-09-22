package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanInputDirectoryKeepsDirectoryAndRemovesContents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "input")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.pcapng"), []byte("capture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "old.etl"), []byte("capture"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cleanInputDirectory(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("input directory should still exist: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("input directory contains %d entries after cleanup", len(entries))
	}
}

func TestCaptureLoginCancellationAlwaysStopsPktmon(t *testing.T) {
	useWorkingDirectory(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	commands := [][]string{}
	_, err := captureLoginAutoWithRunner(ctx, "test", 10*time.Second, func(args ...string) error {
		commands = append(commands, append([]string(nil), args...))
		return nil
	})
	if err == nil {
		t.Fatal("cancelled capture returned no error")
	}
	if len(commands) != 3 || commands[0][0] != "stop" || commands[1][0] != "start" || commands[2][0] != "stop" {
		t.Fatalf("unexpected pktmon lifecycle: %#v", commands)
	}
}

func TestCaptureLoginNominalLifecycle(t *testing.T) {
	useWorkingDirectory(t)
	commands := [][]string{}
	pcap, err := captureLoginAutoWithDependencies(context.Background(), "test", time.Second, func(args ...string) error {
		commands = append(commands, append([]string(nil), args...))
		return nil
	}, func(context.Context, time.Duration) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(pcap) != "test.pcapng" {
		t.Fatalf("pcap = %q", pcap)
	}
	assertCommandNames(t, commands, "stop", "start", "stop", "etl2pcap")
}

func TestGuidedCaptureInstructionsAreEnglish(t *testing.T) {
	if loginReadyMessage != "Capture ready. Click Login in NTE now." {
		t.Fatalf("unexpected login instruction: %q", loginReadyMessage)
	}
	if captureAnalysisMessage != "\nCapture window finished. Analyzing data..." {
		t.Fatalf("unexpected analysis instruction: %q", captureAnalysisMessage)
	}
}

func TestCaptureLoginFailureLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		failedCommand string
		waitErr       error
		wantCommands  []string
	}{
		{name: "start", failedCommand: "start", wantCommands: []string{"stop", "start"}},
		{name: "wait", waitErr: errors.New("wait failed"), wantCommands: []string{"stop", "start", "stop"}},
		{name: "stop", failedCommand: "stop", wantCommands: []string{"stop", "start", "stop"}},
		{name: "conversion", failedCommand: "etl2pcap", wantCommands: []string{"stop", "start", "stop", "etl2pcap"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useWorkingDirectory(t)
			commands := [][]string{}
			_, err := captureLoginAutoWithDependencies(context.Background(), "test", time.Second, func(args ...string) error {
				commands = append(commands, append([]string(nil), args...))
				if args[0] == test.failedCommand {
					return errors.New("command failed")
				}
				return nil
			}, func(context.Context, time.Duration) error { return test.waitErr })
			if err == nil {
				t.Fatal("capture returned no error")
			}
			assertCommandNames(t, commands, test.wantCommands...)
		})
	}
}

func TestPktmonStopperRunsOnlyOnce(t *testing.T) {
	calls := 0
	stop := pktmonStopper(func(args ...string) error {
		calls++
		return nil
	})
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("stop calls = %d, want 1", calls)
	}
}

func TestCleanInputDirectoryRejectsOtherDirectory(t *testing.T) {
	if err := cleanInputDirectory(t.TempDir()); err == nil {
		t.Fatal("expected a non-input directory to be rejected")
	}
}

func TestRemoveCaptureFiles(t *testing.T) {
	dir := t.TempDir()
	pcap := filepath.Join(dir, "nte-login.pcapng")
	etl := filepath.Join(dir, "nte-login.etl")
	for _, path := range []string{pcap, etl} {
		if err := os.WriteFile(path, []byte("capture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeCaptureFiles(pcap); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{pcap, etl} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("capture still exists: %s", path)
		}
	}
}

func useWorkingDirectory(t *testing.T) {
	t.Helper()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })
}

func assertCommandNames(t *testing.T, commands [][]string, want ...string) {
	t.Helper()
	if len(commands) != len(want) {
		t.Fatalf("commands = %#v, want %v", commands, want)
	}
	for index, command := range commands {
		if len(command) == 0 || command[0] != want[index] {
			t.Fatalf("commands = %#v, want %v", commands, want)
		}
	}
}
