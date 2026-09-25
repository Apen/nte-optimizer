package app

import (
	"path/filepath"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
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
	if _, ok := stats.Sources["bond"]; ok {
		t.Fatalf("missing bond level unexpectedly added a bond source: %#v", stats.Sources["bond"])
	}
}

func TestLoadCharacterGameStateExposesObservedPanelSeparatelyFromCalculatedStats(t *testing.T) {
	projectDir := t.TempDir()
	if err := ensureWorkspace(projectDir); err != nil {
		t.Fatal(err)
	}
	attack, defense, critRate, critDamage := 1748.125, 1005.0, .73, 2.304
	bondLevel := 10
	character := decoded.Character{
		CharacterID: 1004, Level: 80, BreakthroughLevel: 6, BondLevel: &bondLevel,
		Stats: decoded.CharacterStats{
			MaxHP: 23471.475, Attack: &attack, Defense: &defense, CritRate: &critRate, CritDamage: &critDamage,
			PanelBase: &decoded.CharacterPanelBase{MaxHP: 15514, Attack: 1230, Defense: 909},
			Source:    "unreal_attribute_set_and_equipment",
		},
	}
	if err := writeAccountData(projectDir, nte.Inventory{}, decoded.State{Characters: []decoded.Character{character}}); err != nil {
		t.Fatal(err)
	}

	got, err := LoadCharacterGameState(projectDir, filepath.Join("..", "..", "data"), character.CharacterID, "en")
	if err != nil {
		t.Fatal(err)
	}
	if got.ObservedPanelStats == nil || got.ObservedPanelStats.Attack == nil || *got.ObservedPanelStats.Attack != attack || got.ObservedPanelStats.PanelBase == nil || got.ObservedPanelStats.PanelBase.Attack != 1230 {
		t.Fatalf("observed panel stats were not exposed: %#v", got.ObservedPanelStats)
	}
	if got.ObservedPanelStats.CritRate == nil || *got.ObservedPanelStats.CritRate != critRate {
		t.Fatalf("observed panel crit rate was not preserved: %#v", got.ObservedPanelStats)
	}
	if got.Stats == nil || got.Stats.Derived["AtkFinal"] != 636 || got.Stats.Derived["CritBase"] != .05 {
		t.Fatalf("calculated optimizer stats were replaced or altered: %#v", got.Stats)
	}
	if _, ok := got.Stats.Sources["bond"]; ok {
		t.Fatalf("bond level was incorrectly applied as a calculated stat source: %#v", got.Stats.Sources["bond"])
	}
}
