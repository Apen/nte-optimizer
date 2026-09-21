package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONAtomicallyReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"value":"old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	want := struct {
		Value string `json:"value"`
	}{Value: "new"}
	if err := writeJSON(path, want); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Value string `json:"value"`
	}
	if err := readJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("read %#v, want %#v", got, want)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".state.json-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", matches)
	}
}

func TestReadOptionalJSONDistinguishesMissingAndInvalidFiles(t *testing.T) {
	directory := t.TempDir()
	var value map[string]any
	loaded, err := readOptionalJSON(filepath.Join(directory, "missing.json"), &value)
	if err != nil || loaded {
		t.Fatalf("missing file: loaded=%v err=%v", loaded, err)
	}
	path := filepath.Join(directory, "invalid.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err = readOptionalJSON(path, &value)
	if err == nil || loaded {
		t.Fatalf("invalid file: loaded=%v err=%v", loaded, err)
	}
}
