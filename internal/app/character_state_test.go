package app

import (
	"path/filepath"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
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

func TestLoadCharacterGameStateReportsPanelMatchedHousingCandidatesWithoutApplyingThem(t *testing.T) {
	projectDir := t.TempDir()
	if err := ensureWorkspace(projectDir); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join("..", "..", "data")
	state := decoded.State{Characters: []decoded.Character{
		{CharacterID: 1004, Level: 80, BreakthroughLevel: 6, ObservedAtLogin: decoded.ObservedLoginState{Loaded: true}},
		{CharacterID: 1036, Level: 80, BreakthroughLevel: 6, ObservedAtLogin: decoded.ObservedLoginState{Loaded: true}},
	}}
	for index := range state.Characters {
		character := state.Characters[index]
		stats, err := loadCurrentCharacterStats(dataDir, &state, character, nil, nil, "en")
		if err != nil {
			t.Fatal(err)
		}
		attack := stats.Derived["AtkFinal"] + 4
		defense := stats.Derived["DefFinal"] + 9
		critDamage := stats.Derived["CritDamageBase"] + .024
		state.Characters[index].Stats = decoded.CharacterStats{Attack: &attack, Defense: &defense, CritDamage: &critDamage, Source: "synthetic_panel"}
	}
	if err := writeAccountData(projectDir, nte.Inventory{}, state); err != nil {
		t.Fatal(err)
	}

	got, err := LoadCharacterGameState(projectDir, dataDir, 1004, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PotentialHousingEffects) != 3 {
		t.Fatalf("potential housing effects = %#v, want 3 panel-matched candidates", got.PotentialHousingEffects)
	}
	if len(got.PossiblePanelBonuses) != 3 {
		t.Fatalf("possible panel bonuses = %#v, want three source-agnostic residuals", got.PossiblePanelBonuses)
	}
	wantBonuses := map[string]float64{"AtkFinal": 4, "DefFinal": 9, "CritDamageBase": .024}
	for _, bonus := range got.PossiblePanelBonuses {
		if want, ok := wantBonuses[bonus.PropertyID]; !ok || bonus.Difference != want {
			t.Errorf("unexpected panel bonus %#v, want %s +%v", bonus, bonus.PropertyID, want)
			continue
		}
		delete(wantBonuses, bonus.PropertyID)
	}
	if len(wantBonuses) != 0 {
		t.Errorf("missing panel bonus residuals: %#v", wantBonuses)
	}
	wantIDs := map[string]bool{"yaodao_1": true, "kaijia_2": true, "quantao_5": true}
	for _, effect := range got.PotentialHousingEffects {
		if !wantIDs[effect.ModifierID] || effect.MatchingCharacters != 2 || effect.ObservedCharacters != 2 {
			t.Errorf("unexpected housing candidate: %#v", effect)
		}
		delete(wantIDs, effect.ModifierID)
	}
	if len(wantIDs) != 0 {
		t.Errorf("missing expected housing candidates: %#v", wantIDs)
	}
	if got.Stats == nil || got.Stats.Derived["AtkFinal"] == *state.Characters[0].Stats.Attack {
		t.Fatalf("diagnostic candidate changed calculated stats: %#v", got.Stats)
	}
	if _, applied := got.Stats.Sources["housing"]; applied {
		t.Fatalf("inferred furniture effects were applied to optimizer sources: %#v", got.Stats.Sources["housing"])
	}
}

func TestInferPossiblePanelBonusesReportsPositiveResidualsWithoutRoundingNoise(t *testing.T) {
	attack, defense, critRate, critDamage := 1748.125, 1005.0, .73, 2.304
	charge, cycle, universal, elemental := 1.12, 172.0, .2, .1
	panel := &decoded.CharacterStats{
		MaxHP: 23471.475, Attack: &attack, Defense: &defense, CritRate: &critRate,
		CritDamage: &critDamage, ChargeEfficiency: &charge, CycleIntensity: &cycle,
		UniversalDMGBonus: &universal, ElementalDMGBonus: &elemental,
	}
	calculated := &optimizer.StatSummary{Derived: map[string]float64{
		"HPFinal": 23471, "AtkFinal": 1744, "DefFinal": 996,
		"CritBase": .73, "CritDamageBase": 2.28, "ChargeGetEfficiencyBase": 1.12,
		"MagBase": 72, "DamageUpGeneralBase": .2, "ElementalDMGBonus": .1,
	}}
	got := inferPossiblePanelBonuses(panel, calculated)
	want := map[string]float64{"AtkFinal": 4, "CritDamageBase": .024, "DefFinal": 9, "MagBase": 100}
	if len(got) != len(want) {
		t.Fatalf("panel bonuses = %#v, want %d residuals", got, len(want))
	}
	for _, bonus := range got {
		if expected, ok := want[bonus.PropertyID]; !ok || bonus.Difference != expected {
			t.Errorf("panel bonus = %#v, want %s +%v", bonus, bonus.PropertyID, expected)
			continue
		}
		delete(want, bonus.PropertyID)
	}
	if len(want) != 0 {
		t.Errorf("missing panel bonus differences: %#v", want)
	}
}
