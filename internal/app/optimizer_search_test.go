package app

import (
	"context"
	"fmt"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func TestCurrentBuildSeedReconstructsEquippedLayout(t *testing.T) {
	grid, err := optimizer.NewGrid(4, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	shapes := optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{
		"H_2":    {ID: "H_2", Cells: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}}},
		"SINGLE": {ID: "SINGLE", Cells: []optimizer.Point{{X: 0, Y: 0}}},
	}}
	modules := []nte.Module{
		{LocalID: "wide", Geometry: "H_2", EquippedCharacterID: 1036, EquippedPlacement: &nte.Placement{Row: 1, Column: 1}},
		{LocalID: "small", Geometry: "SINGLE", EquippedCharacterID: 1036, EquippedPlacement: &nte.Placement{Row: 2, Column: 4}},
		{LocalID: "other-character", Geometry: "H_2", EquippedCharacterID: 1003, EquippedPlacement: &nte.Placement{Row: 2, Column: 1}},
	}
	candidates := []optimizer.Candidate{
		{Module: modules[0]}, {Module: modules[1]}, {Module: modules[2]},
	}

	got := currentBuildSeed(grid, shapes, modules, 1036, candidates)
	if len(got) != 2 {
		t.Fatalf("current build seed = %#v", got)
	}
	ids := map[string]bool{}
	occupied := grid.Clone()
	for _, placement := range got {
		if ids[placement.ModuleID] || !occupied.CanPlace(placement.Cells) {
			t.Fatalf("invalid or duplicate seeded placement: %#v", placement)
		}
		ids[placement.ModuleID] = true
		if err := occupied.Place(placement.Cells); err != nil {
			t.Fatal(err)
		}
	}
	if !ids["wide"] || !ids["small"] {
		t.Fatalf("seeded module IDs = %#v", ids)
	}
}

func TestCurrentBuildSeedSkipsUnreconstructableLoadout(t *testing.T) {
	grid, err := optimizer.NewGrid(2, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	shapes := optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{
		"H_2": {ID: "H_2", Cells: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}}},
	}}
	modules := []nte.Module{
		{LocalID: "first", Geometry: "H_2", EquippedCharacterID: 1036, EquippedPlacement: &nte.Placement{Row: 0, Column: 0}},
		{LocalID: "missing-placement", Geometry: "H_2", EquippedCharacterID: 1036},
	}
	candidates := []optimizer.Candidate{{Module: modules[0]}, {Module: modules[1]}}

	if got := currentBuildSeed(grid, shapes, modules, 1036, candidates); len(got) != 0 {
		t.Fatalf("seed for incomplete equipped layout = %#v, want none", got)
	}
}

func TestCurrentBuildSeedFitsOffOriginGeometry(t *testing.T) {
	grid, err := optimizer.NewGrid(4, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	shapes := optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{
		"L_3_TL": {ID: "L_3_TL", Cells: []optimizer.Point{{X: 1, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}}},
	}}
	modules := []nte.Module{{LocalID: "off-origin-shape", Geometry: "L_3_TL", EquippedCharacterID: 1036}}
	candidates := []optimizer.Candidate{{Module: modules[0]}}

	got := currentBuildSeed(grid, shapes, modules, 1036, candidates)
	if len(got) != 1 || len(got[0].Cells) != 3 {
		t.Fatalf("current build seed = %#v, want the L shape", got)
	}
}

func TestExecuteSearchRetainsSeedWhenApproximateSearchIsInterrupted(t *testing.T) {
	grid, err := optimizer.NewGrid(2, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	shapes := optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{
		"SINGLE": {ID: "SINGLE", Cells: []optimizer.Point{{X: 0, Y: 0}}},
	}}
	candidates := make([]optimizer.Candidate, 1200)
	for index := range candidates {
		candidates[index] = optimizer.Candidate{Module: nte.Module{LocalID: fmt.Sprintf("module-%04d", index), Geometry: "SINGLE"}, Score: 1}
	}
	for _, id := range []string{"seed-a", "seed-b"} {
		candidates = append(candidates, optimizer.Candidate{Module: nte.Module{LocalID: id, Geometry: "SINGLE"}, Score: 1})
	}
	seed := []optimizer.Placement{
		{ModuleID: "seed-a", X: 0, Y: 0, Cells: []optimizer.Point{{X: 0, Y: 0}}},
		{ModuleID: "seed-b", X: 1, Y: 0, Cells: []optimizer.Point{{X: 1, Y: 0}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	setup := searchSetup{evaluator: optimizer.NoBonusEvaluator{}, solver: optimizer.SearchSolver{Catalog: shapes, DisableBound: true}}

	got, err := (OptimizerService{}).executeSearch(ctx, searchRequest{grid: grid, setup: setup, selected: candidates, seed: seed})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.solution.Placements) != len(seed) || got.solution.Placements[0].ModuleID != "seed-a" || got.solution.Placements[1].ModuleID != "seed-b" {
		t.Fatalf("interrupted search lost its seed: %#v", got.solution.Placements)
	}
}

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
