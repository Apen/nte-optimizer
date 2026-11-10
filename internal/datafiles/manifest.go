package datafiles

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ValidateGameManifest checks the complete published snapshot before consumption.
func (l Layout) ValidateGameManifest() error {
	raw, err := os.ReadFile(l.Game("manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Version int `json:"schema_version"`
		Files   []struct {
			Path string `json:"path"`
			Hash string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("decode game manifest: %w", err)
	}
	if manifest.Version != 1 || len(manifest.Files) == 0 {
		return fmt.Errorf("unsupported or empty game manifest")
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Files {
		path := filepath.FromSlash(entry.Path)
		if !filepath.IsLocal(path) || seen[path] {
			return fmt.Errorf("invalid or duplicate manifest path %q", entry.Path)
		}
		seen[path] = true
		content, err := os.ReadFile(l.Game(path))
		if err != nil {
			return fmt.Errorf("manifest %s: %w", entry.Path, err)
		}
		hash := sha256.Sum256(content)
		if hex.EncodeToString(hash[:]) != entry.Hash {
			return fmt.Errorf("manifest checksum mismatch: %s", entry.Path)
		}
	}
	return nil
}
