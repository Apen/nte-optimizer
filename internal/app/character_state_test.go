package app

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func TestShinkuAwakeningResonancesInCharacterViews(t *testing.T) {
	projectDir := t.TempDir()
	if err := ensureWorkspace(projectDir); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join("..", "..", "data")
	observedCritRate := 1.004
	character := decoded.Character{
		CharacterID: 1076, Level: 80, BreakthroughLevel: 6,
		Stats: decoded.CharacterStats{CritRate: &observedCritRate},
		Skills: []decoded.CharacterSkill{
			{AbilityID: "GA_Shinku_Melee", Level: 10},
			{AbilityID: "GA_Shinku_Skill", Level: 10},
			{AbilityID: "GA_Shinku_UltraSkill", Level: 10},
			{AbilityID: "GA_Shinku_QTE", Level: 10},
		},
	}
	var critWithoutResonance float64
	for _, rank := range []int{2, 3, 5, 6} {
		character.AwakenLevel = rank
		state := decoded.State{Characters: []decoded.Character{character}}
		if err := writeAccountData(projectDir, nte.Inventory{}, state); err != nil {
			t.Fatal(err)
		}
		got, err := LoadCharacterGameState(projectDir, dataDir, character.CharacterID, "en")
		if err != nil {
			t.Fatal(err)
		}
		want := slices.Clone(character.Skills)
		if rank >= 3 {
			for i := range want[:3] {
				want[i].Level++
			}
		}
		if !slices.Equal(got.Character.Skills, want) {
			t.Fatalf("rank %d skills = %v, want %v", rank, got.Character.Skills, want)
		}
		if got.ObservedPanelStats == nil || *got.ObservedPanelStats.CritRate != observedCritRate {
			t.Fatalf("rank %d changed the observed panel", rank)
		}
		workspace, err := LoadBuildWorkspace(projectDir, dataDir, "en")
		if err != nil || len(workspace.Characters) != 1 || !slices.Equal(workspace.Characters[0].Skills, want) {
			t.Fatalf("rank %d workspace skills = %+v, error = %v", rank, workspace.Characters, err)
		}
		if rank == 2 {
			critWithoutResonance = got.Stats.Derived["CritBase"]
		}
		critBonus, defenseIgnore := 0.0, 0.0
		if rank == 6 {
			critBonus, defenseIgnore = .1, .15
		}
		if math.Abs(got.Stats.Derived["CritBase"]-critWithoutResonance-critBonus) > 1e-9 || got.Stats.Derived["DefenseIgnoreBase"] != defenseIgnore {
			t.Fatalf("rank %d has unexpected resonance stats: %v", rank, got.Stats.Derived)
		}
	}
	_, persisted, _, err := loadAccountData(projectDir)
	if err != nil || !slices.Equal(persisted.Characters[0].Skills, character.Skills) {
		t.Fatalf("raw skill levels changed: %v, %v", persisted.Characters, err)
	}
}

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

func TestCharacterStateDoesNotCalculateUnrelatedCharacters(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	state := scannedBuffFixture()
	attack := 1000.0
	state.Characters[0].Stats.Attack = &attack
	state.Characters = append(state.Characters, decoded.Character{CharacterID: 1004, Level: 80, BreakthroughLevel: 6})
	if err := writeAccountData(dir, nte.Inventory{}, state); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "account_overrides.json"), map[string]any{"characters": map[int]map[string]float64{1036: {"AtkFinal": 4}}}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCharacterGameState(dir, filepath.Join("..", "..", "data"), 1004, "en")
	if err != nil || got.Stats == nil {
		t.Fatalf("unrelated calibration blocked character view: %+v %v", got, err)
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
	if got.Stats == nil || got.Stats.Derived["AtkFinal"] != 636 || got.Stats.Derived["CritBase"] != .09 {
		t.Fatalf("calculated optimizer stats were replaced or altered: %#v", got.Stats)
	}
	if got.Stats.Sources["account_bond"]["CritBase"] != .04 {
		t.Fatalf("scanned bond bonus was not applied exactly once: %#v", got.Stats.Sources["account_bond"])
	}
}
