package datafiles

import (
	"path/filepath"
	"testing"
)

func TestLayout(t *testing.T) {
	layout := New("data")
	tests := map[string]string{
		layout.CharacterBaseStats():        filepath.Join("data", "game", "characters", "base_stats.json"),
		layout.Sets():                      filepath.Join("data", "game", "equipment", "sets.json"),
		layout.TargetsDir():                filepath.Join("data", "recommendations", "targets"),
		layout.Target("zankou"):            filepath.Join("data", "recommendations", "targets", "zankou.json"),
		layout.DecodeCatalog("forks.json"): filepath.Join("data", "game", "forks", "decode.json"),
		layout.Locale("fr"):                filepath.Join("data", "presentation", "fr.json"),
		layout.GameLocale("fr"):            filepath.Join("data", "game", "locales", "fr.json"),
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	}
}
