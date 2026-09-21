package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type File struct {
	Name  string
	Value any
}

type Writer func(path string, value any) error

func Publish(directory string, files []File) error {
	return PublishWithWriter(directory, files, WriteJSON)
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func PublishWithWriter(directory string, files []File, writer Writer) error {
	directory = filepath.Clean(directory)
	parent, base := filepath.Dir(directory), filepath.Base(directory)
	if base == "." || base == string(filepath.Separator) || parent == directory {
		return fmt.Errorf("unsafe output directory %q", directory)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create output parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+base+"-staging-")
	if err != nil {
		return fmt.Errorf("create staged output: %w", err)
	}
	defer os.RemoveAll(staging)
	for _, file := range files {
		if filepath.Base(file.Name) != file.Name || file.Name == "." {
			return fmt.Errorf("unsafe output file name %q", file.Name)
		}
		if err := writer(filepath.Join(staging, file.Name), file.Value); err != nil {
			return fmt.Errorf("stage %s: %w", file.Name, err)
		}
	}
	if err := validateStagedOutput(staging, files); err != nil {
		return err
	}

	backup := ""
	if _, err := os.Stat(directory); err == nil {
		backup, err = reserveSiblingPath(parent, "."+base+"-previous-")
		if err != nil {
			return err
		}
		if err := os.Rename(directory, backup); err != nil {
			return fmt.Errorf("preserve previous output: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect previous output: %w", err)
	}

	if err := os.Rename(staging, directory); err != nil {
		if backup != "" {
			if restoreErr := os.Rename(backup, directory); restoreErr != nil {
				return fmt.Errorf("publish output: %w (restore previous output: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("publish output: %w", err)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous output backup: %w", err)
		}
	}
	return nil
}

func validateStagedOutput(staging string, files []File) error {
	for _, file := range files {
		path := filepath.Join(staging, file.Name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("validate staged %s: %w", file.Name, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("validate staged %s: invalid JSON", file.Name)
		}
	}
	return nil
}

func reserveSiblingPath(parent, pattern string) (string, error) {
	path, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return "", fmt.Errorf("reserve previous output path: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("prepare previous output path: %w", err)
	}
	return path, nil
}
