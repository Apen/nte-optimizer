package app

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func TestNewAndSavedBuildUsesScannedAccountBuffsAcrossArcSelection(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	bondLevel := 10
	weaponID := decoded.NetID{Slot: 1, Serial: 1}
	state := decoded.State{

		Characters: []decoded.Character{{CharacterID: 1036, Level: 80, BreakthroughLevel: 6, ForkNetID: &weaponID}},
		Weapons:    []decoded.Weapon{{ID: weaponID, ForkID: "fork_DemonBlade", Level: 80, Breakthrough: 6, Star: 1}},
	}
	inv := nte.Inventory{Modules: []nte.Module{{LocalID: "synthetic-module", Geometry: "H_2", Area: 2, Level: 20, EquippedCharacterID: 1036, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .1}}}}}
	if err := writeAccountData(dir, inv, state); err != nil {
		t.Fatal(err)
	}
	service := NewOptimizerService(dataDir)
	baseline := map[string]OptimizationResult{}
	for _, arc := range []string{NoArcForkSelection, "fork_DemonBlade"} {
		result, err := service.OptimizeFlexibleProjectWithArc(context.Background(), dir, "zankou", true, "en", "score", nil, arc)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Stats.Sources["account_furniture"]) != 0 || len(result.Stats.Sources["account_bond"]) != 0 {
			t.Fatal("bonuses applied without observations")
		}
		baseline[arc] = result
	}
	state.FurnitureLevels = scannedBuffFixture().FurnitureLevels
	state.Characters[0].BondLevel = &bondLevel
	if err := writeAccountData(dir, inv, state); err != nil {
		t.Fatal(err)
	}
	for _, arc := range []string{NoArcForkSelection, "fork_DemonBlade"} {
		result, err := service.OptimizeFlexibleProjectWithArc(context.Background(), dir, "zankou", true, "en", "score", nil, arc)
		if err != nil {
			t.Fatal(err)
		}
		if result.CurrentStats == nil {
			t.Fatal("missing equipped comparison")
		}
		for property, bonus := range map[string]float64{"AtkFinal": 4, "DefFinal": 9, "CritDamageBase": .024, "CritBase": .04} {
			if got := result.Stats.Derived[property] - baseline[arc].Stats.Derived[property]; math.Abs(got-bonus) > 1e-9 {
				t.Fatalf("Arc %q build %s bonus=%g, want %g", arc, property, got, bonus)
			}
			if got := result.CurrentStats.Derived[property] - baseline[arc].CurrentStats.Derived[property]; math.Abs(got-bonus) > 1e-9 {
				t.Fatalf("Arc %q equipped %s bonus=%g, want %g", arc, property, got, bonus)
			}
		}
		if result.Stats.Sources["account_furniture"]["AtkAdd"] != 4 || result.Stats.Sources["account_bond"]["CritBase"] != .04 {
			t.Fatal("missing separately attributed account sources")
		}
		if _, err := SaveOptimizationResult(dir, result); err != nil {
			t.Fatal(err)
		}
		saved, err := SavedOptimizationResult(dir, 1036)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Stats.Sources["account_furniture"]["AtkAdd"] != 4 || saved.Stats.Sources["account_bond"]["CritBase"] != .04 || saved.Stats.Derived["AtkFinal"] != result.Stats.Derived["AtkFinal"] {
			t.Fatal("saved build lost or counted account bonuses twice")
		}
	}
}
