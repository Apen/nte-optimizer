package app

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareSavedOptimizationUsesDefaultsAndOverrides(t *testing.T) {
	service := NewOptimizerService("../../data")
	stateDir := t.TempDir()
	defaultRequest, err := PrepareSavedOptimization(service, stateDir, "1036", "")
	if err != nil {
		t.Fatal(err)
	}
	if defaultRequest.Profile.CharacterID != 1036 || len(defaultRequest.Goals) == 0 || len(defaultRequest.Weights.MainStats) == 0 {
		t.Fatalf("unexpected default request: %#v", defaultRequest)
	}
	goal, ok := defaultRequest.Goals["CritDamageBase"]
	if !ok {
		t.Fatal("missing default CRIT DMG goal")
	}
	if goal.Tolerance != .05 {
		t.Fatalf("default tolerance = %v, want .05", goal.Tolerance)
	}
	strict := 2.2
	settings := defaultRequest.Weights
	settings.Goals = map[string]SavedGoalSettings{
		"CritDamageBase": {Target: 2.3, Minimum: &strict, Tolerance: .1, StrictMinimum: true},
		"AtkFinal":       {Disabled: true},
	}
	settings.Weights["CritDamageBase"] = .42
	if err := os.MkdirAll(filepath.Join(stateDir, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveProfileSettings(stateDir, defaultRequest.Profile.ID, settings); err != nil {
		t.Fatal(err)
	}
	request, err := PrepareSavedOptimization(service, stateDir, "1036", defaultRequest.Profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := request.Goals["AtkFinal"]; exists {
		t.Fatal("disabled goal was included")
	}
	goal = request.Goals["CritDamageBase"]
	if !goal.StrictMinimum || goal.Minimum != strict || goal.Target != 2.3 || goal.Importance != .42 || math.Abs(goal.Tolerance-.1) > 1e-9 {
		t.Fatalf("saved goal not applied: %#v", goal)
	}
}

func TestPrepareSavedOptimizationRejectsUnknownProfile(t *testing.T) {
	_, err := PrepareSavedOptimization(NewOptimizerService("../../data"), t.TempDir(), "", "not-a-profile")
	if err == nil {
		t.Fatal("expected unknown profile error")
	}
}
