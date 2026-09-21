package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"nte-optimizer/internal/datafiles"
)

func resolveProjectDir() (string, error) {
	candidates := make([]string, 0, 8)
	if cwd, err := os.Getwd(); err == nil {
		candidates = appendAncestors(candidates, cwd, 4)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = appendAncestors(candidates, filepath.Dir(executable), 4)
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if isProjectDir(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot locate nte-optimizer project directory from current directory or executable path")
}

func resolveUserDataDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate user data directory: %w", err)
		}
	}
	return filepath.Join(base, "NTE Optimizer"), nil
}

func prepareUserDataDir(installDir, userDataDir string) error {
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return fmt.Errorf("create user data directory: %w", err)
	}
	legacy := filepath.Join(installDir, "workspace")
	destination := filepath.Join(userDataDir, "workspace")
	if filepath.Clean(legacy) == filepath.Clean(destination) {
		return nil
	}
	if _, err := os.Stat(destination); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect user workspace: %w", err)
	}
	if info, err := os.Stat(legacy); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect legacy workspace: %w", err)
	} else if !info.IsDir() {
		return nil
	}
	temporary, err := os.MkdirTemp(userDataDir, ".workspace-migration-")
	if err != nil {
		return fmt.Errorf("prepare workspace migration: %w", err)
	}
	defer os.RemoveAll(temporary)
	if err := copyDirectory(legacy, temporary); err != nil {
		return fmt.Errorf("copy legacy workspace: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("publish migrated workspace: %w", err)
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return closeErr
	})
}

func appendAncestors(paths []string, start string, levels int) []string {
	current := filepath.Clean(start)
	for i := 0; i <= levels; i++ {
		paths = append(paths, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return paths
}

func isProjectDir(path string) bool {
	layout := datafiles.New(filepath.Join(path, "data"))
	for _, required := range []string{layout.Config()} {
		if info, err := os.Stat(required); err != nil || info.IsDir() {
			return false
		}
	}
	if info, err := os.Stat(layout.TargetsDir()); err != nil || !info.IsDir() {
		return false
	}
	return true
}
