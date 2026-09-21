package app

import (
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func TestPrepareSearchConfiguresExactnessAndTimeout(t *testing.T) {
	tests := []struct {
		name        string
		plan        searchPlan
		configured  int
		wantTimeout int
		wantExact   bool
	}{
		{name: "score", plan: searchPlan{requestedMode: "score", solverMode: "score", approximate: true}, configured: 7, wantTimeout: 7},
		{name: "fast balanced", plan: searchPlan{requestedMode: "fast-balanced", solverMode: "balanced", approximate: true}, configured: 7, wantTimeout: 7, wantExact: true},
		{name: "balanced minimum", plan: searchPlan{requestedMode: "balanced", solverMode: "balanced"}, configured: 7, wantTimeout: 15, wantExact: true},
		{name: "balanced configured", plan: searchPlan{requestedMode: "balanced", solverMode: "balanced"}, configured: 20, wantTimeout: 20, wantExact: true},
		{name: "exact score", plan: searchPlan{requestedMode: "exact-score", solverMode: "score"}, configured: 20, wantTimeout: 0, wantExact: true},
		{name: "compromise default", plan: searchPlan{requestedMode: "compromise", solverMode: "balanced", approximate: true, compromise: true}, wantTimeout: 5, wantExact: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := optimizerDataConfig{}
			config.Optimize.TimeoutSeconds = test.configured
			setup := (OptimizerService{}).prepareSearch(scoring.Character{}, scoring.References{}, optimizer.ShapeCatalog{}, optimizer.SetCatalog{}, config, nil, nil, nil, nil, test.plan)
			if setup.timeoutSeconds != test.wantTimeout || setup.solver.Exact != test.wantExact || setup.solver.KeepBest != 50 || setup.evaluator == nil {
				t.Fatalf("unexpected search setup: timeout=%d exact=%v keep=%d evaluator=%T", setup.timeoutSeconds, setup.solver.Exact, setup.solver.KeepBest, setup.evaluator)
			}
		})
	}
}

func TestQuickSearchCandidatesIncludesBaseAndPinnedModules(t *testing.T) {
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 1
	config.Optimize.TopPerSet = 1
	candidates := []optimizer.Candidate{
		{Module: nte.Module{LocalID: "objective", Geometry: "H_2"}, Priority: 10},
		{Module: nte.Module{LocalID: "pinned", Geometry: "H_2"}, Priority: 0},
	}
	raw := []optimizer.Candidate{
		{Module: nte.Module{LocalID: "base", Geometry: "V_2"}, Score: 10},
	}
	got := quickSearchCandidates(candidates, raw, []optimizer.ObjectiveGoal{{PropertyID: "Crit", Minimum: 1}}, config, 1, map[string]bool{"pinned": true})
	ids := candidateIDs(got)
	for _, id := range []string{"base", "pinned"} {
		if !ids[id] {
			t.Fatalf("quick candidates do not contain %q: %#v", id, got)
		}
	}
}
