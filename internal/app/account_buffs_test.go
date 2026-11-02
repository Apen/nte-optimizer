package app

import (
	"errors"
	"path/filepath"
	"reflect"
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
		if c.Sources["account_furniture"]["AtkAdd"] != 4 {
			t.Fatal("displayed furniture absent from calculation")
		}
		if c.CharacterID == 1036 && c.Sources["account_bond"]["CritBase"] != .04 {
			t.Fatal("displayed bond absent from calculation")
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
	if len(page.DetectedFurnitureIDs) != 0 || page.Characters[0].BondBonusActive || len(page.Characters[0].Sources["account_furniture"]) != 0 {
		t.Fatal("old scan or manual settings leaked into latest import")
	}
}
