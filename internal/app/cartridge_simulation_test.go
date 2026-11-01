package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestCartridgeSimulationUsesQualityCurvesWithoutMutatingInventory(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	for _, test := range []struct {
		quality string
		want    float64
	}{{"blue", .36}, {"purple", .48}, {"orange", .6}} {
		t.Run(test.quality, func(t *testing.T) {
			input := []nte.Cartridge{{LocalID: "synthetic", GameItemID: "Lakshana_" + test.quality, Level: 0,
				MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .12, Percent: true}},
				SubStats:  []nte.Stat{{PropertyID: "CritBase", Value: .02, Percent: true}}}}
			before, _ := json.Marshal(input)
			got, err := service.simulateCartridgeMainStats(input)
			if err != nil {
				t.Fatal(err)
			}
			if got[0].MainStats[0].Value != test.want || got[0].SimulatedMainStatLevel != 20 || got[0].Level != 0 {
				t.Fatalf("unexpected projection: %#v", got[0])
			}
			if !reflect.DeepEqual(input[0].SubStats, got[0].SubStats) {
				t.Fatal("substats changed")
			}
			after, _ := json.Marshal(input)
			if string(before) != string(after) {
				t.Fatal("input inventory was mutated")
			}
		})
	}
	maxed := []nte.Cartridge{{GameItemID: "Lakshana_orange", Level: 20, MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .6}}}}
	got, err := service.simulateCartridgeMainStats(maxed)
	if err != nil || !reflect.DeepEqual(got, maxed) {
		t.Fatalf("maxed cartridge changed: %#v, %v", got, err)
	}
}

func TestCartridgeSimulationRejectsMissingGameData(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	for _, test := range []struct {
		cartridge nte.Cartridge
		want      string
	}{
		{nte.Cartridge{GameItemID: "missing"}, "missing core max_level"},
		{nte.Cartridge{GameItemID: "Lakshana_orange"}, "without a main stat"},
		{nte.Cartridge{GameItemID: "Lakshana_orange", MainStats: []nte.Stat{{PropertyID: "missing"}}}, "missing_Core_ITEM_QUALITY_ORANGE"},
	} {
		if _, err := service.simulateCartridgeMainStats([]nte.Cartridge{test.cartridge}); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("expected %q error, got %v", test.want, err)
		}
	}
}

func TestCartridgeSimulationChangesSelectionAndPreservesCurrentStats(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	profiles, err := service.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := service.loadOptimizerCatalog()
	if err != nil {
		t.Fatal(err)
	}
	profile := profiles["zankou"]
	setID := catalog.sets.Definitions[profile.PreferredSets[0].SetID].InventorySetID
	input := nte.Inventory{
		Modules: []nte.Module{{LocalID: "synthetic-module", Geometry: "H_2", Area: 2}},
		Cartridges: []nte.Cartridge{
			{LocalID: "upgrade", GameItemID: "Lakshana_orange", SetID: setID, Level: 0, EquippedCharacterID: profile.CharacterID,
				MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .12, Percent: true}}},
			{LocalID: "maxed", GameItemID: "Lakshana_purple", SetID: setID, Level: 20,
				MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .48, Percent: true}}},
		},
	}
	service.WeightOverrides = &WeightOverrides{MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"CritDamageBase": 1}}
	actual, err := service.Optimize(context.Background(), input, "zankou")
	if err != nil {
		t.Fatal(err)
	}
	if actual.Cartridge == nil || actual.Cartridge.LocalID != "maxed" || actual.SimulateCartridgeMainStats {
		t.Fatalf("unexpected actual selection: %#v", actual.Cartridge)
	}
	service.WeightOverrides.SimulateCartridgeMainStats = true
	simulated, err := service.Optimize(context.Background(), input, "zankou")
	if err != nil {
		t.Fatal(err)
	}
	if simulated.Cartridge == nil || simulated.Cartridge.LocalID != "upgrade" || !simulated.SimulateCartridgeMainStats || simulated.Cartridge.SimulatedMainStatLevel != 20 {
		t.Fatalf("unexpected simulated selection: %#v", simulated.Cartridge)
	}
	if simulated.Stats.Sources["cartridge"]["CritDamageBase"] != .6 || simulated.CurrentStats == nil || simulated.CurrentStats.Sources["cartridge"]["CritDamageBase"] != .12 {
		t.Fatalf("simulated and actual stats mixed: build=%#v current=%#v", simulated.Stats, simulated.CurrentStats)
	}
	if simulated.Solution.CartridgeScore <= actual.Solution.CartridgeScore {
		t.Fatal("ranking did not use projected main stat")
	}
	if input.Cartridges[0].MainStats[0].Value != .12 || input.Cartridges[0].SimulatedMainStatLevel != 0 {
		t.Fatal("inventory mutated")
	}
}
