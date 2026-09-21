package scannerlauncher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type elevatedRunner func(helper, projectDir, outputDir, cancelFile string, seconds int) error

// Run locates the separately built scanner helper and starts it with elevated
// privileges. The desktop optimizer itself deliberately stays non-elevated.
func Run(installDir, stateDir string, seconds int) error {
	return RunContext(context.Background(), installDir, stateDir, seconds)
}

func RunContext(ctx context.Context, installDir, stateDir string, seconds int) error {
	return run(ctx, installDir, stateDir, seconds, runElevated)
}

func run(ctx context.Context, installDir, stateDir string, seconds int, elevate elevatedRunner) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("la capture automatique est disponible uniquement sous Windows")
	}
	if seconds < 10 {
		seconds = 10
	}
	helper, err := findHelper(installDir)
	if err != nil {
		return err
	}
	outputDir := OutputDir(stateDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("création du dossier de sortie du scanner: %w", err)
	}
	cancelFile, err := reserveCancelFile(stateDir)
	if err != nil {
		return err
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
	return elevate(helper, installDir, outputDir, cancelFile, seconds)
}

func reserveCancelFile(stateDir string) (string, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", fmt.Errorf("création du dossier utilisateur: %w", err)
	}
	file, err := os.CreateTemp(stateDir, ".scan-cancel-*")
	if err != nil {
		return "", fmt.Errorf("préparation du signal d'annulation: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("préparation du signal d'annulation: %w", err)
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
	return "", fmt.Errorf("nte-scan.exe est introuvable; relance scripts\\build-app.ps1 pour construire l'application et son scanner")
}
