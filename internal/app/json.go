package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"nte-optimizer/internal/atomicfile"
	"nte-optimizer/internal/nte"
)

const workspaceDirectory = "workspace"

func workspaceFile(projectDir, name string) string {
	return filepath.Join(projectDir, workspaceDirectory, name)
}

func ensureWorkspace(projectDir string) error {
	return os.MkdirAll(filepath.Join(projectDir, workspaceDirectory), 0o755)
}

func readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func readOptionalJSON(path string, destination any) (bool, error) {
	err := readJSON(path, destination)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func readInventory(path string) (nte.Inventory, error) {
	var inventory nte.Inventory
	if err := readJSON(path, &inventory); err != nil {
		return nte.Inventory{}, err
	}
	inventory.Normalize()
	return inventory, nil
}

func readOptionalInventory(path string) (nte.Inventory, bool, error) {
	var inventory nte.Inventory
	loaded, err := readOptionalJSON(path, &inventory)
	if err != nil || !loaded {
		return nte.Inventory{}, loaded, err
	}
	inventory.Normalize()
	return inventory, true, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return atomicfile.Write(path, data, 0o644)
}
