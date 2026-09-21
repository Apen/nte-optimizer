package optimizer

import (
	"context"
	"fmt"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
	"testing"
)

func benchmarkFixture() (SearchSolver, Grid, []Candidate, CartridgeSetEvaluator, []Placement) {
	grid, _ := NewGrid(4, 1, nil)
	solver := SearchSolver{Catalog: ShapeCatalog{Shapes: map[string]Shape{"S": {ID: "S", Cells: []Point{{0, 0}}}}}, KeepBest: 5, BlueprintCache: NewBlueprintCache()}
	candidates := make([]Candidate, 24)
	for i := range candidates {
		candidates[i] = Candidate{Module: nte.Module{LocalID: fmt.Sprint(i), Geometry: "S", Area: 1, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: float64(i+1) * .001}}}, Score: float64(i + 1)}
	}
	catalog := SetCatalog{Definitions: map[string]SetDefinition{"set": {ID: "set", InventorySetID: "set", RequiredGeometries: []string{"S"}, Bonuses: []SetBonus{{Count: 1, Score: 3}}}}}
	evaluator := NewObjectiveEvaluator(catalog, []nte.Cartridge{{LocalID: "c", SetID: "set"}}, nil, candidates, scoring.Character{BaseStats: map[string]float64{"AtkBase": 600}}, nil, []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .6}}, nil)
	placements := []Placement{{ModuleID: "0"}, {ModuleID: "1"}, {ModuleID: "2"}, {ModuleID: "3"}}
	return solver, grid, candidates, evaluator, placements
}

func BenchmarkBlueprintSearch(b *testing.B) {
	solver, grid, candidates, evaluator, _ := benchmarkFixture()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := solver.SolveWithBonus(context.Background(), grid, candidates, evaluator); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkSetEvaluation(b *testing.B) {
	_, _, _, evaluator, placements := benchmarkFixture()
	b.ReportAllocs()
	for b.Loop() {
		evaluator.Evaluate(placements)
	}
}
func BenchmarkStatSummary(b *testing.B) {
	_, _, candidates, evaluator, _ := benchmarkFixture()
	modules := []nte.Module{candidates[0].Module, candidates[1].Module, candidates[2].Module, candidates[3].Module}
	b.ReportAllocs()
	for b.Loop() {
		BuildStatSummary(evaluator.character, modules, &evaluator.sets[0].cartridge, evaluator.sets[0].definition, 1)
	}
}

type referenceEvaluator struct{ GlobalBonusEvaluator }

func BenchmarkBlueprintComparison(b *testing.B) {
	for _, mode := range []string{"reference", "compiled", "bounded"} {
		b.Run(mode, func(b *testing.B) {
			solver, grid, candidates, e, _ := benchmarkFixture()
			solver.DisableBound = mode != "bounded"
			var evaluator GlobalBonusEvaluator = e
			if mode == "reference" {
				evaluator = referenceEvaluator{e}
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := solver.SolveWithBonus(context.Background(), grid, candidates, evaluator); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
