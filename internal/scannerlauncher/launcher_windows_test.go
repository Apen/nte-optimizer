//go:build windows

package scannerlauncher

import (
	"errors"
	"strings"
	"testing"
)

func TestRunElevatedBuildsNativeProcessRequest(t *testing.T) {
	const helper = `C:\Program Files\NTE's\nte-scan.exe`
	const workingDirectory = `C:\NTE's`
	var gotHelper, gotWorkingDirectory, arguments string
	err := runElevatedWithRunner(helper, workingDirectory, `C:\Users\Test User\AppData\Local\NTE Optimizer\workspace\scan-output`, `C:\Users\Test User\AppData\Local\NTE Optimizer\cancel`, 35, func(gotFile, gotDirectory, gotArguments string) ([]byte, error) {
		gotHelper, gotWorkingDirectory, arguments = gotFile, gotDirectory, gotArguments
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotHelper != helper || gotWorkingDirectory != workingDirectory {
		t.Fatalf("runner received helper %q and directory %q", gotHelper, gotWorkingDirectory)
	}
	for _, want := range []string{"-login-capture", "-login-seconds 35", `-output-dir "C:\Users\Test User\AppData\Local\NTE Optimizer\workspace\scan-output"`, `-cancel-file "C:\Users\Test User\AppData\Local\NTE Optimizer\cancel"`, `-scan-log "C:\Users\Test User\AppData\Local\NTE Optimizer\scan-diagnostic.jsonl"`} {
		if !strings.Contains(arguments, want) {
			t.Fatalf("scanner arguments do not contain %q: %s", want, arguments)
		}
	}
}

func TestRunElevatedReportsRunnerOutput(t *testing.T) {
	err := runElevatedWithRunner("scanner", "project", "output", "cancel", 30, func(string, string, string) ([]byte, error) {
		return []byte("operation cancelled by the user"), errors.New("exit status 1")
	})
	if err == nil || !strings.Contains(err.Error(), "operation cancelled by the user") {
		t.Fatalf("runElevatedWithRunner() error = %v", err)
	}
}

func TestRunElevatedFallsBackToProcessError(t *testing.T) {
	err := runElevatedWithRunner("scanner", "project", "output", "cancel", 30, func(string, string, string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("runElevatedWithRunner() error = %v", err)
	}
}

func TestRunElevatedWithRecoveryAddsExplicitStopFlag(t *testing.T) {
	var arguments string
	err := runElevatedWithRecoveryWithRunner(`C:\Program Files\NTE's\nte-scan.exe`, `C:\NTE's`, `C:\output`, `C:\cancel`, 35, true, func(_, _, gotArguments string) ([]byte, error) {
		arguments = gotArguments
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-login-capture", "-stop-pktmon-first"} {
		if !strings.Contains(arguments, want) {
			t.Fatalf("scanner arguments do not contain %q: %s", want, arguments)
		}
	}
}

func TestRunElevatedWithRecoveryOmitsStopFlagByDefault(t *testing.T) {
	var arguments string
	err := runElevatedWithRunner("scanner", "project", "output", "cancel", 30, func(_, _, gotArguments string) ([]byte, error) {
		arguments = gotArguments
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(arguments, "-stop-pktmon-first") {
		t.Fatalf("normal scan unexpectedly stops Pktmon: %s", arguments)
	}
}

func TestCaptureRetentionRequiresExplicitOptionAndSupportsRecovery(t *testing.T) {
	for _, retain := range []bool{false, true} {
		for _, recovery := range []bool{false, true} {
			var arguments string
			err := runElevatedOptionsWithRunner("scanner", "project", "output", "cancel", Options{Seconds: 45, RetainCapture: retain, StopPktmonFirst: recovery}, func(_, _, gotArguments string) ([]byte, error) { arguments = gotArguments; return nil, nil })
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(arguments, "-retain-capture") != retain || strings.Contains(arguments, "-stop-pktmon-first") != recovery || !strings.Contains(arguments, "-login-seconds 45") {
				t.Fatalf("options lost: retain=%v recovery=%v arguments=%s", retain, recovery, arguments)
			}
		}
	}
}
