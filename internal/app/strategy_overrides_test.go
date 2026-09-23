package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileStrategyRoundTripAndDelete(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := WeightOverrides{MainStats: []string{"MagBase", "CritDamageBase", "MagBase"}, Weights: map[string]float64{"MagBase": .85}}
	saved, err := SaveProfileStrategy(dir, "zero", want)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.MainStats) != 2 || saved.MainStats[0] != "CritDamageBase" {
		t.Fatalf("settings not normalized: %#v", saved)
	}
	state, err := LoadStrategyOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profiles["zero"].Weights["MagBase"] != .85 {
		t.Fatalf("settings not persisted: %#v", state)
	}
	if err := DeleteProfileStrategy(dir, "zero"); err != nil {
		t.Fatal(err)
	}
	state, err = LoadStrategyOverrides(dir)
	if err != nil || len(state.Profiles) != 0 {
		t.Fatalf("settings not deleted: %#v, %v", state, err)
	}
}

func TestProfileSettingsStayIsolatedAndWeightSavePreservesGoals(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	zankou := WeightOverrides{
		MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"CritDamageBase": 1},
		Goals: map[string]SavedGoalSettings{"CritDamageBase": {Target: 2.1, Maximum: 2.5, Tolerance: .05, Disabled: true}},
	}
	if _, err := SaveProfileSettings(dir, "zankou", zankou); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveProfileSettings(dir, "zero", WeightOverrides{MainStats: []string{"MagBase"}, Weights: map[string]float64{"MagBase": .8}}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveProfileStrategy(dir, "zankou", WeightOverrides{MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"CritDamageBase": .9}}); err != nil {
		t.Fatal(err)
	}
	state, err := LoadStrategyOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profiles["zankou"].Weights["CritDamageBase"] != .9 || state.Profiles["zankou"].Goals["CritDamageBase"].Target != 2.1 || !state.Profiles["zankou"].Goals["CritDamageBase"].Disabled {
		t.Fatalf("Zankou settings were not preserved: %#v", state.Profiles["zankou"])
	}
	if state.Profiles["zero"].Weights["MagBase"] != .8 || len(state.Profiles["zero"].Goals) != 0 {
		t.Fatalf("Zero settings were modified by Zankou save: %#v", state.Profiles["zero"])
	}
}

func TestProfileSettingsPersistStrictMinimumAboveSoftTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	minimum := 300.0
	settings := WeightOverrides{MainStats: []string{"UnbalIntensityBase"}, Goals: map[string]SavedGoalSettings{
		"UnbalIntensityBase": {Target: 150, Minimum: &minimum, StrictMinimum: true},
	}}
	if _, err := SaveProfileSettings(dir, "daffodill", settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadStrategyOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	goal := loaded.Profiles["daffodill"].Goals["UnbalIntensityBase"]
	if goal.Minimum == nil || *goal.Minimum != 300 || goal.Target != 150 {
		t.Fatalf("absolute minimum was not persisted independently: %+v", goal)
	}
}
