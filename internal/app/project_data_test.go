package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCharactersDefaultsGridWithoutConsoleTraits(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "recommendations", "targets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dataDir, "recommendations", "targets", "hero.json"), `{"schema_version":2,"id":"hero","name":"Hero","goals":[{"property_id":"AtkFinal","label":"ATK","minimum":1}],"character":{"character_id":42},"builds":[{"id":"hero","name":"DPS","variants":[{"id":"hero","name":"Hero"}]}]}`)

	characters, err := (OptimizerService{DataDir: dataDir}).loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	if got := characters["hero"].GridID; got != "character_42" {
		t.Fatalf("grid ID = %q, want character_42", got)
	}
}

func TestCharacterBaseStatsAreCachedAndReturnedAsCopies(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "game", "characters", "base_stats.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, `{"schema_version":1,"stats":{"42":{"80:6":{"AtkBase":1234}}}}`)
	service := NewOptimizerService(dataDir)

	first, ok, err := service.characterBaseStats(42, 80, 6)
	if err != nil || !ok {
		t.Fatalf("first characterBaseStats() = %#v, %t, %v", first, ok, err)
	}
	first["AtkBase"] = 9999
	writeTestFile(t, path, `{invalid`)

	second, ok, err := service.characterBaseStats(42, 80, 6)
	if err != nil || !ok {
		t.Fatalf("cached characterBaseStats() = %#v, %t, %v", second, ok, err)
	}
	if got := second["AtkBase"]; got != 1234 {
		t.Fatalf("cached AtkBase = %v, want 1234", got)
	}
}
