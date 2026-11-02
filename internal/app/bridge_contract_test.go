package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestBridgeContractMatchesSharedFixture(t *testing.T) {
	minimum := .4
	values := map[string]any{
		"module":   nte.Module{LocalID: "synthetic", GameItemID: "synthetic", Quality: "blue", Geometry: "SYNTHETIC", Area: 2, MainStats: []nte.Stat{}, SubStats: []nte.Stat{}},
		"settings": WeightOverrides{MainStats: []string{"CritBase"}, Weights: map[string]float64{"CritBase": 1}, Goals: map[string]SavedGoalSettings{"CritBase": {Target: .5, Minimum: &minimum, Tolerance: .05, StrictMinimum: true, Custom: true, Percent: true, Label: "Critical rate"}}},
		"bonuses":  AccountBuffPage{DetectedFurnitureIDs: []string{}, Furniture: []BuffOption{}, Characters: []BuffCharacter{{CharacterID: 42, Name: "Synthetic"}}},
	}
	got, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "testdata", "contracts", "bridge.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bridge contract drifted: %s", got)
	}
}
