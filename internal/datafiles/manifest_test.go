package datafiles

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProductionGameManifestChecksums(t *testing.T) {
	gameDir := filepath.Join("..", "..", "data", "game")
	content, err := os.ReadFile(filepath.Join(gameDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) == 0 {
		t.Fatal("game manifest contains no files")
	}
	for _, entry := range manifest.Files {
		data, err := os.ReadFile(filepath.Join(gameDir, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Errorf("%s: %v", entry.Path, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != entry.SHA256 {
			t.Errorf("%s checksum = %s, manifest has %s", entry.Path, got, entry.SHA256)
		}
	}
}
