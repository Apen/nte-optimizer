package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func scannedBuffFixture() decoded.State {
	level := 10
	return decoded.State{
		FurnitureLevels: []nte.FurnitureLevel{{ProgressionID: "SF_0011", Level: 2}, {ProgressionID: "SF_0012", Level: 6}, {ProgressionID: "SF_0013", Level: 3}},
		Characters:      []decoded.Character{{CharacterID: 1036, Level: 80, BreakthroughLevel: 6, BondLevel: &level, BondStageFlags: []bool{true}}},
	}
}

func TestInformationalBonusesIgnoreCalculationCalibration(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	state := scannedBuffFixture()
	attack := 1000.0
	state.Characters[0].ObservedAtLogin.Loaded = true
	state.Characters[0].Stats.Attack = &attack
	if err := writeAccountData(dir, nte.Inventory{}, state); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "account_overrides.json"), map[string]any{"characters": map[int]map[string]float64{1036: {"AtkFinal": 4}}}); err != nil {
		t.Fatal(err)
	}
	page, err := LoadAccountBuffPage(dir, filepath.Join("..", "..", "data"), "en")
	if err != nil || len(page.Characters) != 1 || !page.Characters[0].BondBonusActive {
		t.Fatalf("informational page: %+v, %v", page, err)
	}
	if _, err := scannedAccountBuffSources(&state, filepath.Join("..", "..", "data"), 1036, map[string]float64{"AtkFinal": 4}); !errors.Is(err, errBuffCalibrationConflict) {
		t.Fatal("calculation must still reject the conflict")
	}
}

func BenchmarkAccountBuffPage(b *testing.B) {
	profiles, err := NewOptimizerService(filepath.Join("..", "..", "data")).loadCharacters()
	if err != nil {
		b.Fatal(err)
	}
	known := map[int]bool{}
	for _, profile := range profiles {
		known[profile.CharacterID] = true
	}
	ids := make([]int, 0, len(known))
	for id := range known {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) < 12 {
		b.Fatal("benchmark requires twelve cataloged characters")
	}
	for _, count := range []int{1, 12} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			dir := b.TempDir()
			if err := ensureWorkspace(dir); err != nil {
				b.Fatal(err)
			}
			state := decoded.State{}
			for i := 0; i < count; i++ {
				state.Characters = append(state.Characters, decoded.Character{CharacterID: ids[i], Level: 80, BreakthroughLevel: 6})
			}
			if err := writeAccountData(dir, nte.Inventory{}, state); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := LoadAccountBuffPage(dir, filepath.Join("..", "..", "data"), "en"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestScannedAccountBuffsApplyOnceAndBondIsCharacterSpecific(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	state := scannedBuffFixture()
	state.FurnitureLevels = append(state.FurnitureLevels, state.FurnitureLevels[0])
	state.Characters = append(state.Characters, state.Characters[0])
	for _, id := range []int{1036, 1004} {
		sources, err := scannedAccountBuffSources(&state, dataDir, id, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]float64{}
		for _, stats := range sources {
			for _, stat := range stats {
				got[stat.PropertyID] += stat.Value
			}
		}
		if got["AtkAdd"] != 4 || got["DefAdd"] != 9 || got["CritDamageBase"] != .024 {
			t.Fatalf("wrong shared bonuses: %v", got)
		}
		wantCrit := 0.
		if id == 1036 {
			wantCrit = .04
		}
		if got["CritBase"] != wantCrit {
			t.Fatalf("bond duplicated or leaked to another character: %v", got)
		}
	}
}

func TestScannedAccountBuffsRejectConflictingLevelsAndMissingCatalogTier(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	for _, levels := range [][]nte.FurnitureLevel{
		{{ProgressionID: "SF_0011", Level: 2}, {ProgressionID: "SF_0011", Level: 3}},
		{{ProgressionID: "SF_0011", Level: 999}},
	} {
		if _, err := scannedAccountBuffSources(&decoded.State{FurnitureLevels: levels}, dataDir, 1036, nil); err == nil {
			t.Fatalf("accepted inconsistent observation: %v", levels)
		}
	}
	_, err := scannedAccountBuffSources(&decoded.State{FurnitureLevels: []nte.FurnitureLevel{{ProgressionID: "SF_0011", Level: 999}}}, dataDir, 1036, nil)
	if !strings.Contains(err.Error(), "housing/special_furniture_modifiers.json") {
		t.Fatalf("missing catalog contract not identified: %v", err)
	}
}

func TestScannedAccountBuffsRejectOverlappingCalibration(t *testing.T) {
	state := scannedBuffFixture()
	dataDir := filepath.Join("..", "..", "data")
	for _, calibration := range []map[string]float64{{"AtkFinal": 4}, {"AtkUp": .1}, {"CritDamageBase": .024}, {"CritBase": .04}} {
		if _, err := scannedAccountBuffSources(&state, dataDir, 1036, calibration); !errors.Is(err, errBuffCalibrationConflict) {
			t.Fatalf("overlap accepted: %v", err)
		}
	}
	if _, err := scannedAccountBuffSources(&state, dataDir, 1036, map[string]float64{"MagBase": 10}); err != nil {
		t.Fatal(err)
	}
}

func TestAccountBuffPageAndCalculationUseSameScannedSelection(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	state := scannedBuffFixture()
	below := 9
	state.Characters = append(state.Characters, decoded.Character{CharacterID: 1004, Level: 80, BreakthroughLevel: 6, BondLevel: &below})
	if err := writeAccountData(dir, nte.Inventory{}, state); err != nil {
		t.Fatal(err)
	}
	// Old manual settings are obsolete and must not gate or override scan inputs.
	if err := writeJSON(workspaceFile(dir, "account_buffs.json"), map[string]any{"schema_version": 999, "furniture_ids": []string{"invalid"}}); err != nil {
		t.Fatal(err)
	}
	page, err := LoadAccountBuffPage(dir, filepath.Join("..", "..", "data"), "en")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(page.DetectedFurnitureIDs, []string{"kaijia_2", "quantao_5", "yaodao_1"}) {
		t.Fatalf("wrong active furniture: %v", page.DetectedFurnitureIDs)
	}
	for _, c := range page.Characters {
		if c.BondBonusActive != (c.CharacterID == 1036) {
			t.Fatalf("wrong bond activation for %d", c.CharacterID)
		}
		sources, err := scannedAccountBuffSources(&state, filepath.Join("..", "..", "data"), c.CharacterID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(sources["account_furniture"]) != 3 || (len(sources["account_bond"]) > 0) != c.BondBonusActive {
			t.Fatal("display and calculation selection differ")
		}

	}
	// A newer import replaces the old selection without persisting any settings.
	if err := writeAccountData(dir, nte.Inventory{}, decoded.State{Characters: state.Characters[1:]}); err != nil {
		t.Fatal(err)
	}
	page, err = LoadAccountBuffPage(dir, filepath.Join("..", "..", "data"), "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.DetectedFurnitureIDs) != 0 || page.Characters[0].BondBonusActive {
		t.Fatal("old scan or manual settings leaked into latest import")
	}
}
