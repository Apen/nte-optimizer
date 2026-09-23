package app

import (
	"math"
	"testing"

	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func TestApplyObjectiveScalesUsesSharedReferences(t *testing.T) {
	goals := []optimizer.ObjectiveGoal{
		{PropertyID: "UnbalIntensityBase", Minimum: 360, Importance: 1},
		{PropertyID: "CritDamageBase", Minimum: 2.3, Importance: 1},
		{PropertyID: "AtkFinal", Minimum: 2300, Importance: 1},
	}
	refs := scoring.References{"UnbalIntensityBase": 12, "CritDamageBase": .128, "AtkAdd": 80, "AtkUp": .12}
	if err := applyObjectiveScales(goals, refs, map[string]float64{"AtkBase": 1000}); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{120, 1.28, 1200} {
		if math.Abs(goals[i].ScoreScale-want) > 1e-9 {
			t.Fatalf("goal %d scale = %g, want %g", i, goals[i].ScoreScale, want)
		}
	}
	if got := optimizer.ObjectiveScore(map[string]float64{"UnbalIntensityBase": 140}, goals[:1]); math.Abs(got-10*140.0/260) > 1e-9 {
		t.Fatalf("Break contribution = %g", got)
	}
	goals[0].Minimum = 150
	if got := optimizer.ObjectiveScore(map[string]float64{"UnbalIntensityBase": 140}, goals[:1]); math.Abs(got-10*140.0/260) > 1e-9 {
		t.Fatalf("changing the target changed subtarget contribution: %g", got)
	}
}

func TestApplyObjectiveScalesRejectsMissingReference(t *testing.T) {
	goals := []optimizer.ObjectiveGoal{{PropertyID: "Unknown", Minimum: 10}}
	if err := applyObjectiveScales(goals, scoring.References{}, nil); err == nil {
		t.Fatal("missing scoring reference was accepted")
	}
}

func TestProductionProfilesHaveScalesForEveryGoal(t *testing.T) {
	service := NewOptimizerService("../../data")
	profiles, err := service.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := service.loadOptimizerCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for id, profile := range profiles {
		buildTarget, err := service.Target(id)
		if err != nil {
			t.Fatalf("%s target: %v", id, err)
		}
		goals := make([]optimizer.ObjectiveGoal, len(buildTarget.Goals))
		for i, goal := range buildTarget.Goals {
			goals[i] = optimizer.ObjectiveGoal{PropertyID: goal.PropertyID, Minimum: goal.Minimum}
		}
		if err := applyObjectiveScales(goals, catalog.references, profile.BaseStats); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
}
