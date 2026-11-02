package app

import (
	"context"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicOptimizerRejectsMissingImportedTier(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	catalog, err := s.loadCharacterBaseStats()
	if err != nil {
		t.Fatal(err)
	}
	delete(catalog.Stats["1036"], "80:6")
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeAccountData(dir, nte.Inventory{}, decoded.State{Characters: []decoded.Character{{CharacterID: 1036, Level: 80, BreakthroughLevel: 6}}}); err != nil {
		t.Fatal(err)
	}
	_, err = s.OptimizeFlexibleProjectWithArc(context.Background(), dir, "zankou", true, "en", "fast", nil, NoArcForkSelection)
	if err == nil || !strings.Contains(err.Error(), `stats["1036"]["80:6"]`) {
		t.Fatalf("missing imported tier accepted: %v", err)
	}
}

func TestMissingImportedTierDoesNotUseRecommendationDefaults(t *testing.T) {
	c := decoded.Character{CharacterID: 1036, Level: 999, BreakthroughLevel: 6}
	_, err := loadCurrentCharacterStats(filepath.Join("..", "..", "data"), &decoded.State{Characters: []decoded.Character{c}}, c, nil, nil, "en")
	if err == nil || !strings.Contains(err.Error(), `stats["1036"]["999:6"]`) {
		t.Fatalf("missing tier not identified: %v", err)
	}
}

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
