package datafiles

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRejectsChangedCatalogsDuplicateAndEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	game := filepath.Join(dir, "game")
	if err := os.MkdirAll(game, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"schema_version":1}`)
	hash := sha256.Sum256(content)
	entry := fmt.Sprintf(`{"path":"catalog.json","sha256":"%s"}`, hex.EncodeToString(hash[:]))
	if err := os.WriteFile(filepath.Join(game, "catalog.json"), content, 0600); err != nil {
		t.Fatal(err)
	}
	write := func(entries string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(game, "manifest.json"), []byte(`{"schema_version":1,"files":[`+entries+`]}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(entry)
	if err := New(dir).ValidateGameManifest(); err != nil {
		t.Fatal(err)
	}
	write(entry + "," + entry)
	if err := New(dir).ValidateGameManifest(); err == nil {
		t.Fatal("duplicate manifest entries accepted")
	}
	write(`{"path":"../catalog.json","sha256":""}`)
	if err := New(dir).ValidateGameManifest(); err == nil {
		t.Fatal("escaping manifest path accepted")
	}
	write(entry)
	if err := os.WriteFile(filepath.Join(game, "catalog.json"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := New(dir).ValidateGameManifest(); err == nil {
		t.Fatal("stale checksum accepted")
	}
}
