package scanlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Event contains only fixed diagnostic codes and aggregate counts. Never add
// paths, packet contents, account identifiers, or raw errors to this format.
type Event struct {
	Time   string `json:"time"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	// Code is a fixed, privacy-safe diagnostic identifier. Never put raw errors,
	// paths, packet contents, or account identifiers here.
	Code   string         `json:"code,omitempty"`
	Counts map[string]int `json:"counts,omitempty"`
}

func Path(stateDir string) string { return filepath.Join(stateDir, "scan-diagnostic.jsonl") }

func Start(path string, seconds int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return err
	}
	return Append(path, "scan", "started", map[string]int{"duration_seconds": seconds})
}

func Append(path, stage, status string, counts map[string]int) error {
	return AppendWithCode(path, stage, status, "", counts)
}

func AppendWithCode(path, stage, status, code string, counts map[string]int) error {
	if path == "" {
		return nil
	}
	event := Event{Time: time.Now().UTC().Format(time.RFC3339), Stage: stage, Status: status, Code: code, Counts: counts}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func Read(path string) ([]Event, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // A terminated helper may leave an incomplete final line.
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}
