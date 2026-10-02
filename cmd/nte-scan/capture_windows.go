package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type pktmonRunner func(args ...string) error
type pktmonStatusRunner func() (string, error)
type captureWaiter func(context.Context, time.Duration) error

type pktmonCaptureState uint8

const (
	pktmonCaptureUnknown pktmonCaptureState = iota
	pktmonCaptureStopped
	pktmonCaptureActive
)

type pktmonCommandError struct {
	output string
	err    error
}

func (e *pktmonCommandError) Error() string {
	output := strings.TrimSpace(e.output)
	if output == "" {
		return e.err.Error()
	}
	return fmt.Sprintf("%s: %s", e.err, output)
}

func (e *pktmonCommandError) Unwrap() error { return e.err }

type captureDiagnosticError struct {
	code string
	err  error
}

func (e *captureDiagnosticError) Error() string { return e.err.Error() }
func (e *captureDiagnosticError) Unwrap() error { return e.err }

func captureFailureCode(err error) string {
	var diagnostic *captureDiagnosticError
	if errors.As(err, &diagnostic) {
		return diagnostic.code
	}
	return "capture_failed"
}

const (
	loginReadyMessage      = "Capture ready. Click Login in NTE now."
	captureAnalysisMessage = "\nCapture window finished. Analyzing data..."
)

func runPktmon(args ...string) error {
	cmd := exec.Command("pktmon", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	if err != nil {
		return &pktmonCommandError{output: stdout.String() + stderr.String(), err: err}
	}
	return nil
}

func queryPktmonStatus() (string, error) {
	cmd := exec.Command("pktmon", "status")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	output := stdout.String() + stderr.String()
	if err != nil {
		return output, &pktmonCommandError{output: output, err: err}
	}
	return output, nil
}

func parsePktmonCaptureState(output string) pktmonCaptureState {
	text := strings.ToLower(strings.Join(strings.Fields(output), " "))
	for _, marker := range []string{"not running", "not active", "not capturing", "no active capture", "running: false", "active: false", "capturing: false", "capture stopped", "capture is stopped", "not started", "stopped", "idle", "pas en cours", "en cours : non", "arrêté", "arrêtée", "arrete", "inactif", "inaktiv"} {
		if strings.Contains(text, marker) {
			return pktmonCaptureStopped
		}
	}
	for _, marker := range []string{"capture running", "capture is running", "capture active", "capture is active", "pktmon is running", "status: running", "status: active", "monitoring is running", "capture in progress", "capture is in progress", "capture started", "en cours d’exécution", "en cours d'exécution", "capture en cours", "capture active", "wird ausgeführt", "en ejecución"} {
		if strings.Contains(text, marker) {
			return pktmonCaptureActive
		}
	}
	return pktmonCaptureUnknown
}

func pktmonStartFailureCode(err error) string {
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"already running", "already started", "already active", "already in progress", "déjà démarr", "déjà en cours", "deja demarr", "deja en cours"} {
		if strings.Contains(text, marker) {
			return "pktmon_already_running"
		}
	}
	for _, marker := range []string{"access is denied", "access denied", "permission denied", "accès refusé", "acces refuse", "access refusé"} {
		if strings.Contains(text, marker) {
			return "pktmon_permission_denied"
		}
	}
	return "pktmon_start_failed"
}

func captureMode(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "nte-capture"
	}
	name = regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(name, "-")
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "input")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create input directory: %w", err)
	}
	if err := probeCaptureDirectoryWrite(dir); err != nil {
		return fmt.Errorf("input directory is not writable: %w", err)
	}
	status, err := queryPktmonStatus()
	if err != nil {
		return fmt.Errorf("could not query pktmon status: %w", err)
	}
	switch parsePktmonCaptureState(status) {
	case pktmonCaptureActive:
		return fmt.Errorf("pktmon is already capturing; stop the existing capture before starting NTE Optimizer")
	case pktmonCaptureUnknown:
		return fmt.Errorf("could not determine pktmon status; no capture was started")
	}
	if err := cleanInputDirectory(dir); err != nil {
		return err
	}
	etl := filepath.Join(dir, name+".etl")
	pcap := filepath.Join(dir, name+".pcapng")
	if _, err := os.Stat(etl); err == nil {
		return fmt.Errorf("%s already exists", etl)
	}
	if _, err := os.Stat(pcap); err == nil {
		return fmt.Errorf("%s already exists", pcap)
	}

	fmt.Printf("Starting capture %q...\n", name)
	if err := runPktmon("start", "--capture", "--pkt-size", "0", "--file-name", etl); err != nil {
		return fmt.Errorf("pktmon failed to start (open PowerShell as administrator): %w", err)
	}
	stopCapture := pktmonStopper(runPktmon)
	defer stopCapture()

	fmt.Println("Capture is active. Click ONE item in NTE now.")
	fmt.Println("When finished, return here and press Enter (or Ctrl+C).")
	done := make(chan struct{})
	go func() { _, _ = bufio.NewReader(os.Stdin).ReadString('\n'); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		fmt.Println("\nStop requested...")
	}

	if err := stopCapture(); err != nil {
		return fmt.Errorf("failed to stop pktmon: %w", err)
	}

	fmt.Println("Converting to PCAPNG...")
	if err := runPktmon("etl2pcap", etl, "--out", pcap); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}
	fmt.Printf("Capture ready: %s\n", pcap)
	return nil
}

func captureLoginAutoWithProgressAndRecovery(ctx context.Context, name string, maxDuration time.Duration, stopPktmonFirst bool, progress func(string, string)) (string, error) {
	return captureLoginAutoWithStatusAndRecovery(ctx, name, maxDuration, runPktmon, queryPktmonStatus, waitForLoginCapture, stopPktmonFirst, progress)
}

func captureLoginAutoWithRunner(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner) (string, error) {
	return captureLoginAutoWithStatus(ctx, name, maxDuration, run, func() (string, error) { return "Pktmon is not running", nil }, waitForLoginCapture, nil)
}

func captureLoginAutoWithDependencies(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, wait captureWaiter) (string, error) {
	return captureLoginAutoWithStatus(ctx, name, maxDuration, run, func() (string, error) { return "Pktmon is not running", nil }, wait, nil)
}

func captureLoginAutoWithDependenciesAndProgress(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, wait captureWaiter, progress func(string, string)) (string, error) {
	return captureLoginAutoWithStatus(ctx, name, maxDuration, run, func() (string, error) { return "Pktmon is not running", nil }, wait, progress)
}

func captureLoginAutoWithStatus(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, statusRunner pktmonStatusRunner, wait captureWaiter, progress func(string, string)) (string, error) {
	return captureLoginAutoWithStatusAndRecovery(ctx, name, maxDuration, run, statusRunner, wait, false, progress)
}

func captureLoginAutoWithStatusAndRecovery(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, statusRunner pktmonStatusRunner, wait captureWaiter, stopPktmonFirst bool, progress func(string, string)) (string, error) {
	report := func(stage, status string) {
		if progress != nil {
			progress(stage, status)
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "nte-login-" + time.Now().Format("20060102-150405")
	}
	name = regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(name, "-")
	report("preflight", "started")
	dir, err := os.Getwd()
	if err != nil {
		return "", &captureDiagnosticError{code: "capture_directory_failed", err: err}
	}
	dir = filepath.Join(dir, "input")
	if err := os.MkdirAll(dir, 0755); err != nil {
		report("preflight", "failed")
		return "", &captureDiagnosticError{code: "input_not_writable", err: fmt.Errorf("create input directory: %w", err)}
	}
	if err := probeCaptureDirectoryWrite(dir); err != nil {
		report("preflight", "failed")
		return "", &captureDiagnosticError{code: "input_not_writable", err: fmt.Errorf("input directory is not writable: %w", err)}
	}
	if stopPktmonFirst {
		report("pktmon_status", "skipped")
		report("pktmon_recovery", "started")
		if err := run("stop"); err != nil {
			report("pktmon_recovery", "failed")
			return "", &captureDiagnosticError{code: "pktmon_stop_failed", err: fmt.Errorf("could not stop Pktmon before recovery scan: %w", err)}
		}
		report("pktmon_recovery", "complete")
	} else {
		report("pktmon_status", "started")
		status, err := statusRunner()
		if err != nil {
			report("pktmon_status", "failed")
			code := pktmonStartFailureCode(err)
			if code == "pktmon_start_failed" {
				code = "pktmon_status_failed"
			}
			return "", &captureDiagnosticError{code: code, err: fmt.Errorf("query pktmon status: %w", err)}
		}
		switch parsePktmonCaptureState(status) {
		case pktmonCaptureActive:
			report("pktmon_status", "already_running")
			return "", &captureDiagnosticError{code: "pktmon_already_running", err: fmt.Errorf("pktmon is already capturing; NTE Optimizer left the existing capture untouched")}
		case pktmonCaptureUnknown:
			report("pktmon_status", "failed")
			return "", &captureDiagnosticError{code: "pktmon_status_unknown", err: fmt.Errorf("could not determine pktmon status; no capture was started and existing data was left untouched")}
		}
		report("pktmon_status", "complete")
	}
	if err := cleanInputDirectory(dir); err != nil {
		report("preflight", "failed")
		return "", &captureDiagnosticError{code: "input_prepare_failed", err: err}
	}
	etl, pcap := filepath.Join(dir, name+".etl"), filepath.Join(dir, name+".pcapng")
	if _, err := os.Stat(etl); err == nil {
		report("preflight", "failed")
		return "", &captureDiagnosticError{code: "capture_file_exists", err: fmt.Errorf("capture file already exists: %s", filepath.Base(etl))}
	}
	if _, err := os.Stat(pcap); err == nil {
		report("preflight", "failed")
		return "", &captureDiagnosticError{code: "capture_file_exists", err: fmt.Errorf("capture file already exists: %s", filepath.Base(pcap))}
	}
	report("preflight", "complete")
	if err := run("start", "--capture", "--pkt-size", "0", "--file-name", etl); err != nil {
		report("pktmon_start", "failed")
		return "", &captureDiagnosticError{code: pktmonStartFailureCode(err), err: fmt.Errorf("pktmon failed to start: %w", err)}
	}
	stopCapture := pktmonStopper(run)
	defer stopCapture()
	report("capture", "ready")
	fmt.Println(loginReadyMessage)
	if maxDuration < 10*time.Second {
		maxDuration = 10 * time.Second
	}
	if err := wait(ctx, maxDuration); err != nil {
		report("capture", "interrupted")
		if stopErr := stopCapture(); stopErr != nil {
			report("pktmon_stop", "failed")
			return "", &captureDiagnosticError{code: "pktmon_stop_failed", err: fmt.Errorf("capture interrupted and pktmon could not be stopped: %w", stopErr)}
		}
		return "", &captureDiagnosticError{code: "capture_interrupted", err: fmt.Errorf("capture interrupted: %w", err)}
	}
	report("capture", "window_complete")
	fmt.Println(captureAnalysisMessage)
	if err := stopCapture(); err != nil {
		report("pktmon_stop", "failed")
		return "", &captureDiagnosticError{code: "pktmon_stop_failed", err: fmt.Errorf("failed to stop pktmon: %w", err)}
	}
	report("conversion", "started")
	if err := run("etl2pcap", etl, "--out", pcap); err != nil {
		report("conversion", "failed")
		return "", &captureDiagnosticError{code: "etl_conversion_failed", err: fmt.Errorf("conversion failed: %w", err)}
	}
	report("conversion", "complete")
	return pcap, nil
}

func probeCaptureDirectoryWrite(dir string) error {
	file, err := os.CreateTemp(dir, ".nte-capture-write-check-*")
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

func waitForLoginCapture(ctx context.Context, maxDuration time.Duration) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	totalSeconds := int(maxDuration / time.Second)
	for elapsed := 1; elapsed <= totalSeconds; elapsed++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			fmt.Printf("\rWaiting for login traffic... %ds/%ds", elapsed, totalSeconds)
		}
	}
	return nil
}

func pktmonStopper(run pktmonRunner) func() error {
	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() { stopErr = run("stop") })
		return stopErr
	}
}

func cleanInputDirectory(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve input directory: %w", err)
	}
	if !strings.EqualFold(filepath.Base(abs), "input") || filepath.Dir(abs) == abs {
		return fmt.Errorf("refusing to clean an unsafe directory: %s", abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Errorf("read input directory: %w", err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(abs, entry.Name())); err != nil {
			return fmt.Errorf("clean %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func removeCaptureFiles(pcap string) error {
	paths := []string{pcap, strings.TrimSuffix(pcap, filepath.Ext(pcap)) + ".etl"}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove capture %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
