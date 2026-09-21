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
		return fmt.Errorf("création du dossier input: %w", err)
	}
	if err := cleanInputDirectory(dir); err != nil {
		return err
	}
	etl := filepath.Join(dir, name+".etl")
	pcap := filepath.Join(dir, name+".pcapng")
	if _, err := os.Stat(etl); err == nil {
		return fmt.Errorf("%s existe déjà", etl)
	}
	if _, err := os.Stat(pcap); err == nil {
		return fmt.Errorf("%s existe déjà", pcap)
	}

	fmt.Printf("Démarrage de la capture %q...\n", name)
	if err := runPktmon("start", "--capture", "--pkt-size", "0", "--file-name", etl); err != nil {
		return fmt.Errorf("pktmon n'a pas démarré (ouvre PowerShell en administrateur): %w", err)
	}
	stopCapture := pktmonStopper(runPktmon)
	defer stopCapture()

	fmt.Println("Capture active. Clique maintenant sur UN objet dans NTE.")
	fmt.Println("Quand c'est fait, reviens ici et appuie sur Entrée (ou Ctrl+C).")
	done := make(chan struct{})
	go func() { _, _ = bufio.NewReader(os.Stdin).ReadString('\n'); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		fmt.Println("\nArrêt demandé...")
	}

	if err := stopCapture(); err != nil {
		return fmt.Errorf("impossible d'arrêter pktmon: %w", err)
	}

	fmt.Println("Conversion en PCAPNG...")
	if err := runPktmon("etl2pcap", etl, "--out", pcap); err != nil {
		return fmt.Errorf("conversion impossible: %w", err)
	}
	fmt.Printf("Capture prête : %s\n", pcap)
	return nil
}

func captureLoginAuto(ctx context.Context, name string, maxDuration time.Duration) (string, error) {
	return captureLoginAutoWithRunner(ctx, name, maxDuration, runPktmon)
}

func captureLoginAutoWithRunner(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner) (string, error) {
	return captureLoginAutoWithDependencies(ctx, name, maxDuration, run, waitForLoginCapture)
}

func captureLoginAutoWithDependencies(ctx context.Context, name string, maxDuration time.Duration, run pktmonRunner, wait captureWaiter) (string, error) {
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
		return "", fmt.Errorf("création du dossier input: %w", err)
	}
	if err := cleanInputDirectory(dir); err != nil {
		return "", err
	}
	etl, pcap := filepath.Join(dir, name+".etl"), filepath.Join(dir, name+".pcapng")
	if _, err := os.Stat(etl); err == nil {
		return "", fmt.Errorf("%s existe déjà", etl)
	}
	if _, err := os.Stat(pcap); err == nil {
		return "", fmt.Errorf("%s existe déjà", pcap)
	}
	if err := run("start", "--capture", "--pkt-size", "0", "--file-name", etl); err != nil {
		return "", fmt.Errorf("pktmon n'a pas démarré (ouvre PowerShell en administrateur): %w", err)
	}
	stopCapture := pktmonStopper(run)
	defer stopCapture()
	fmt.Println("Capture prête. Clique sur Connexion dans NTE maintenant.")
	if maxDuration < 10*time.Second {
		maxDuration = 10 * time.Second
	}
	if err := wait(ctx, maxDuration); err != nil {
		return "", fmt.Errorf("capture interrompue: %w", err)
	}
	fmt.Println("\nFenêtre de capture terminée, analyse en cours…")
	if err := stopCapture(); err != nil {
		return "", fmt.Errorf("impossible d'arrêter pktmon: %w", err)
	}
	if err := run("etl2pcap", etl, "--out", pcap); err != nil {
		return "", fmt.Errorf("conversion impossible: %w", err)
	}
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
			fmt.Printf("\rÉcoute du login… %ds/%ds", elapsed, totalSeconds)
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
		return fmt.Errorf("résolution du dossier input: %w", err)
	}
	if !strings.EqualFold(filepath.Base(abs), "input") || filepath.Dir(abs) == abs {
		return fmt.Errorf("refus de nettoyer un dossier non sécurisé: %s", abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Errorf("lecture du dossier input: %w", err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(abs, entry.Name())); err != nil {
			return fmt.Errorf("nettoyage de %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func removeCaptureFiles(pcap string) error {
	paths := []string{pcap, strings.TrimSuffix(pcap, filepath.Ext(pcap)) + ".etl"}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("suppression de la capture %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
