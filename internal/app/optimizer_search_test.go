package app

import (
	"testing"

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
		{name: "fast", plan: searchPlan{requestedMode: "fast", solverMode: "objective", approximate: true}, configured: 7, wantTimeout: 7, wantExact: true},
		{name: "exact score", plan: searchPlan{requestedMode: "exact-score", solverMode: "score"}, configured: 20, wantTimeout: 0, wantExact: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := optimizerDataConfig{}
			config.Optimize.TimeoutSeconds = test.configured
			setup := (OptimizerService{}).prepareSearch(scoring.Character{}, scoring.References{}, optimizer.ShapeCatalog{}, optimizer.SetCatalog{}, config, nil, nil, nil, nil, test.plan, nil)
			if setup.timeoutSeconds != test.wantTimeout || setup.solver.Exact != test.wantExact || setup.solver.KeepBest != 50 || setup.evaluator == nil {
				t.Fatalf("unexpected search setup: timeout=%d exact=%v keep=%d evaluator=%T", setup.timeoutSeconds, setup.solver.Exact, setup.solver.KeepBest, setup.evaluator)
			}
		})
	}
}
