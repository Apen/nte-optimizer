package app

import (
	"path/filepath"
	"testing"

	"nte-optimizer/internal/decoded"
)

func TestLoadCurrentCharacterStatsIncludesCharacterPanel(t *testing.T) {
	character := decoded.Character{CharacterID: 1004, Level: 80, BreakthroughLevel: 6}
	stats, err := loadCurrentCharacterStats(filepath.Join("..", "..", "data"), &decoded.State{Characters: []decoded.Character{character}}, character, nil, nil, "en")
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil {
		t.Fatal("current stats were not generated")
	}
	if stats.Derived["HPFinal"] != 15998 || stats.Derived["AtkFinal"] != 636 || stats.Derived["CritBase"] != .05 {
		t.Fatalf("unexpected current character stats: %#v", stats.Derived)
	}
}
