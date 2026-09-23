package app

import (
	"math"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

func TestParseSearchPlan(t *testing.T) {
	tests := []struct {
		mode        string
		solver      string
		approximate bool
	}{
		{mode: "fast", solver: "objective", approximate: true},
		{mode: "beta", solver: "objective", approximate: true},
		{mode: "exact-objective", solver: "objective"},
		{mode: "score", solver: "score", approximate: true},
		{mode: "exact-score", solver: "score"},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			got, err := parseSearchPlan(test.mode)
			if err != nil {
				t.Fatal(err)
			}
			if got.requestedMode != test.mode || got.solverMode != test.solver || got.approximate != test.approximate {
				t.Fatalf("parseSearchPlan(%q) = %#v", test.mode, got)
			}
		})
	}
	for _, mode := range []string{"deep", "balanced", "fast-balanced", "unknown"} {
		if _, err := parseSearchPlan(mode); err == nil {
			t.Fatalf("removed or unknown search mode %q was accepted", mode)
		}
	}
}

func TestExactObjectiveDiagnosticUsesFullPoolAndNoTimeout(t *testing.T) {
	plan, err := parseSearchPlan("exact-objective")
	if err != nil {
		t.Fatal(err)
	}
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 1
	config.Optimize.TopPerSet = 1
	config.Optimize.TimeoutSeconds = 5
	pool := moduleCandidatePool{eligible: []optimizer.Candidate{{Module: nte.Module{LocalID: "first", Geometry: "H_2"}}, {Module: nte.Module{LocalID: "second", Geometry: "H_2"}}}}
	selection, err := (OptimizerService{}).selectModuleCandidates(pool, config, plan)
	if err != nil || len(selection.selected) != 2 {
		t.Fatalf("exact objective diagnostic lost candidates: %#v, %v", selection, err)
	}
	setup := (OptimizerService{}).prepareSearch(scoring.Character{}, scoring.References{}, optimizer.ShapeCatalog{}, optimizer.SetCatalog{}, config, selection.selected, nil, []optimizer.ObjectiveGoal{{PropertyID: "CritBase", Minimum: .6}}, nil, plan)
	if setup.timeoutSeconds != 0 || !setup.solver.Exact || publicOptimizationMode(plan) != "exact-objective" {
		t.Fatalf("exact objective diagnostic was not configured: %+v", setup)
	}
}

func TestApplyWeightOverridesClonesWeights(t *testing.T) {
	original := scoring.Character{MainWeights: map[string]float64{"Atk": 1}, SubWeights: map[string]float64{"Crit": 1}}
	overrides := &WeightOverrides{Weights: map[string]float64{"Crit": .8}}
	got, err := applyWeightOverrides(original, overrides)
	if err != nil {
		t.Fatal(err)
	}
	if got.MainWeights != nil || got.SubWeights != nil || got.Weights["Crit"] != .8 {
		t.Fatalf("unexpected overridden profile: %#v", got)
	}
	overrides.Weights["Crit"] = .2
	if got.Weights["Crit"] != .8 {
		t.Fatal("profile weights alias caller-owned overrides")
	}
	if original.MainWeights["Atk"] != 1 || original.SubWeights["Crit"] != 1 {
		t.Fatal("original profile was mutated")
	}
}

func TestApplyWeightOverridesRejectsInvalidValues(t *testing.T) {
	for _, weight := range []float64{-1, 11, math.NaN(), math.Inf(1)} {
		if _, err := applyWeightOverrides(scoring.Character{}, &WeightOverrides{Weights: map[string]float64{"Crit": weight}}); err == nil {
			t.Fatalf("weight %v was accepted", weight)
		}
	}
}

func TestPrepareObjectivesUsesTuningsAndSetGeometry(t *testing.T) {
	buildTarget := &target.BuildTarget{Goals: []target.Goal{{PropertyID: "Crit", Minimum: .75}, {PropertyID: "Atk", Minimum: 2000}}}
	profile := scoring.Character{PreferredSets: []scoring.SetPreference{{SetID: "set"}}}
	sets := optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{"set": {RequiredGeometries: []string{"H_2", "L_3"}}}}
	objectives, required := prepareObjectives("objective", buildTarget, map[string]GoalTuning{"Crit": {Importance: .8, Tolerance: .1}}, nil, profile, sets)
	if len(objectives) != 1 || objectives[0].PropertyID != "Crit" || objectives[0].Importance != .8 || objectives[0].Tolerance != .1 {
		t.Fatalf("unexpected objectives: %#v", objectives)
	}
	if !required["H_2"] || !required["L_3"] {
		t.Fatalf("missing required geometries: %#v", required)
	}
}

func TestPrepareObjectivesPreservesUnweightedHardBounds(t *testing.T) {
	buildTarget := &target.BuildTarget{Goals: []target.Goal{{PropertyID: "Crit", Minimum: .75}, {PropertyID: "Atk", Minimum: 2000}}}
	profile := scoring.Character{Weights: map[string]float64{"Crit": 1}}
	tunings := map[string]GoalTuning{
		"Crit": {Importance: .2},
		"Atk":  {Maximum: 2500, StrictMinimum: true},
	}
	objectives, _ := prepareObjectives("objective", buildTarget, tunings, &WeightOverrides{}, profile, optimizer.SetCatalog{})
	if len(objectives) != 2 || objectives[0].Importance != 1 || !objectives[1].StrictMinimum || objectives[1].Maximum != 2500 || objectives[1].Importance != 0 {
		t.Fatalf("unexpected overridden objectives: %#v", objectives)
	}
}

func TestPrepareObjectivesCarriesAbsoluteMinimum(t *testing.T) {
	buildTarget := &target.BuildTarget{Goals: []target.Goal{{PropertyID: "UnbalIntensityBase", Minimum: 360}}}
	profile := scoring.Character{Weights: map[string]float64{"UnbalIntensityBase": 1}}
	tunings := map[string]GoalTuning{"UnbalIntensityBase": {Importance: 1, Minimum: 300, StrictMinimum: true}}
	objectives, _ := prepareObjectives("objective", buildTarget, tunings, &WeightOverrides{}, profile, optimizer.SetCatalog{})
	if len(objectives) != 1 || objectives[0].StrictFloor != 300 {
		t.Fatalf("absolute minimum was lost: %#v", objectives)
	}
}

func TestPrepareObjectivesSkipsLegacyScoreModes(t *testing.T) {
	objectives, required := prepareObjectives("score", &target.BuildTarget{Goals: []target.Goal{{PropertyID: "Crit", Minimum: .75}}}, nil, nil, scoring.Character{}, optimizer.SetCatalog{})
	if len(objectives) != 0 || len(required) != 0 {
		t.Fatalf("score mode prepared objective goals: %#v, %#v", objectives, required)
	}
}
