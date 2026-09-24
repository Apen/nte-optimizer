package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type pktmonRunner func(args ...string) error
type captureWaiter func(context.Context, time.Duration) error

const (
	loginReadyMessage      = "Capture ready. Click Login in NTE now."
	captureAnalysisMessage = "\nCapture window finished. Analyzing data..."
)

func runPktmon(args ...string) error {
	cmd := exec.Command("pktmon", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
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

	// A forcibly closed scanner cannot run its deferred cleanup and may leave the
	// pktmon driver capture active. Stop any stale session before starting a new
	// one; no active session is a harmless condition here.
	_ = runPktmon("stop")
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

func captureLoginAuto(ctx context.Context, name string, maxDuration time.Duration) (string, error) {
	return captureLoginAutoWithRunner(ctx, name, maxDuration, runPktmon)
}

func captureLoginAutoWithProgress(ctx context.Context, name string, maxDuration time.Duration, progress func(string, string)) (string, error) {
	return captureLoginAutoWithDependenciesAndProgress(ctx, name, maxDuration, runPktmon, waitForLoginCapture, progress)
}

func captureLoginAutoWithRunner(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner) (string, error) {
	return captureLoginAutoWithDependencies(ctx, name, maxDuration, run, waitForLoginCapture)
}

func captureLoginAutoWithDependencies(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, wait captureWaiter) (string, error) {
	return captureLoginAutoWithDependenciesAndProgress(ctx, name, maxDuration, run, wait, nil)
}

func captureLoginAutoWithDependenciesAndProgress(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, wait captureWaiter, progress func(string, string)) (string, error) {
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
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "input")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create input directory: %w", err)
	}
	if err := cleanInputDirectory(dir); err != nil {
		return "", err
	}
	etl, pcap := filepath.Join(dir, name+".etl"), filepath.Join(dir, name+".pcapng")
	if _, err := os.Stat(etl); err == nil {
		return "", fmt.Errorf("%s already exists", etl)
	}
	if _, err := os.Stat(pcap); err == nil {
		return "", fmt.Errorf("%s already exists", pcap)
	}
	// Recover from a previous helper that was closed before its deferred stop ran.
	// The command is intentionally best-effort because no active capture is the
	// normal state on a clean launch.
	_ = run("stop")
	if err := run("start", "--capture", "--pkt-size", "0", "--file-name", etl); err != nil {
		report("pktmon_start", "failed")
		return "", fmt.Errorf("pktmon failed to start (approve the administrator prompt): %w", err)
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
		return "", fmt.Errorf("capture interrupted: %w", err)
	}
	report("capture", "window_complete")
	fmt.Println(captureAnalysisMessage)
	if err := stopCapture(); err != nil {
		report("pktmon_stop", "failed")
		return "", fmt.Errorf("failed to stop pktmon: %w", err)
	}
	report("conversion", "started")
	if err := run("etl2pcap", etl, "--out", pcap); err != nil {
		report("conversion", "failed")
		return "", fmt.Errorf("conversion failed: %w", err)
	}
	report("conversion", "complete")
	return pcap, nil
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
