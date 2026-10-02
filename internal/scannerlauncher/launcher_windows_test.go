//go:build windows

package scannerlauncher

import (
	"errors"
	"strings"
	"testing"
)

func TestRunElevatedBuildsEscapedPowerShellCommand(t *testing.T) {
	var script string
	err := runElevatedWithRunner(`C:\Program Files\NTE's\nte-scan.exe`, `C:\NTE's`, `C:\Users\Test User\AppData\Local\NTE Optimizer\workspace\scan-output`, `C:\Users\Test User\AppData\Local\NTE Optimizer\cancel`, 35, func(got string) ([]byte, error) {
		script = got
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Start-Process", "-Verb RunAs", "-Wait", "-login-seconds 35", `-output-dir "C:\Users\Test User\AppData\Local\NTE Optimizer\workspace\scan-output"`, `-cancel-file "C:\Users\Test User\AppData\Local\NTE Optimizer\cancel"`, `-scan-log "C:\Users\Test User\AppData\Local\NTE Optimizer\scan-diagnostic.jsonl"`, "NTE''s"} {
		if !strings.Contains(script, want) {
			t.Fatalf("PowerShell script does not contain %q: %s", want, script)
		}
	}
}

func TestRunElevatedReportsPowerShellOutput(t *testing.T) {
	err := runElevatedWithRunner("scanner", "project", "output", "cancel", 30, func(string) ([]byte, error) {
		return []byte("operation cancelled by the user"), errors.New("exit status 1")
	})
	if err == nil || !strings.Contains(err.Error(), "operation cancelled by the user") {
		t.Fatalf("runElevatedWithRunner() error = %v", err)
	}
}

func TestRunElevatedFallsBackToProcessError(t *testing.T) {
	err := runElevatedWithRunner("scanner", "project", "output", "cancel", 30, func(string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("runElevatedWithRunner() error = %v", err)
	}
}

func TestRunElevatedWithRecoveryAddsExplicitStopFlag(t *testing.T) {
	var script string
	err := runElevatedWithRecoveryWithRunner(`C:\Program Files\NTE's\nte-scan.exe`, `C:\NTE's`, `C:\output`, `C:\cancel`, 35, true, func(got string) ([]byte, error) {
		script = got
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Start-Process", "-Verb RunAs", "-Wait", "-login-capture", "-stop-pktmon-first", "NTE''s"} {
		if !strings.Contains(script, want) {
			t.Fatalf("PowerShell script does not contain %q: %s", want, script)
		}
	}
}

func TestRunElevatedWithRecoveryOmitsStopFlagByDefault(t *testing.T) {
	var script string
	err := runElevatedWithRunner("scanner", "project", "output", "cancel", 30, func(got string) ([]byte, error) {
		script = got
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, "-stop-pktmon-first") {
		t.Fatalf("normal scan unexpectedly stops Pktmon: %s", script)
	}
}
