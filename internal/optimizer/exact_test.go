package optimizer

import (
	"context"
	"math"
	"math/rand"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
	"testing"
)

func TestExactSearchAgainstSubsetOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(71))
	for trial := 0; trial < 80; trial++ {
		grid, _ := NewGrid(3, 1, nil)
		catalog := ShapeCatalog{Shapes: map[string]Shape{"S": {ID: "S", Cells: []Point{{0, 0}}}, "D": {ID: "D", Cells: []Point{{0, 0}, {1, 0}}}}}
		candidates := make([]Candidate, 7)
		for i := range candidates {
			geometry := "S"
			area := 1
			if i%3 == 0 {
				geometry = "D"
				area = 2
			}
			candidates[i] = Candidate{Module: nte.Module{LocalID: string(rune('a' + i)), Geometry: geometry, Area: area, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: float64(rng.Intn(8)) / 10}}}, Score: float64(rng.Intn(9) - 2), Priority: rng.Float64()}
		}
		sets := SetCatalog{Definitions: map[string]SetDefinition{"set": {ID: "set", InventorySetID: "set", RequiredGeometries: []string{"S"}, Bonuses: []SetBonus{{Count: 1, Score: 3}}}}}
		cartridges := []nte.Cartridge{{LocalID: "c1", SetID: "set"}, {LocalID: "c2", SetID: "set", SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .1}}}}
		goals := []ObjectiveGoal{{PropertyID: "CritBase", Minimum: 1, Maximum: 1.2, Importance: float64(trial + 1)}}
		evaluator := NewExactObjectiveEvaluator(sets, cartridges, nil, candidates, scoring.Character{}, nil, goals, nil)
		required := map[string]bool{}
		if trial%2 == 0 {
			required["a"] = true
		}
		oracle := []Solution{}
		indexes := map[string]int{}
		// Independent subset enumeration; on this one-dimensional board every
		// subset of total area three tiles exactly, without geometric blueprints.
		for mask := 0; mask < 1<<len(candidates); mask++ {
			area, score := 0, 0.0
			placements := []Placement{}
			for i, c := range candidates {
				if mask&(1<<i) != 0 {
					area += c.Module.Area
					score += c.Score
					placements = append(placements, Placement{ModuleID: c.Module.LocalID})
				}
			}
			if area != 3 || !placementsContainRequired(placements, required) {
				continue
			}
			for _, set := range evaluator.sets {
				single := evaluator
				single.sets = []weightedSet{set}
				bonus := single.Evaluate(placements)
				if math.IsInf(bonus.Score, -1) {
					continue
				}
				solution := blueprintSolution(score+bonus.Score, score, bonus, placements)
				oracle, indexes = retainTopSolution(oracle, indexes, solution, 5)
			}
		}
		for _, disable := range []bool{true, false} {
			solver := SearchSolver{Catalog: catalog, Exact: true, KeepBest: 5, RequiredIDs: required, DisableBound: disable, DisableGroups: disable}
			got, err := solver.SolveWithBonus(context.Background(), grid, candidates, evaluator)
			if err != nil {
				t.Fatal(err)
			}
			actual := []Solution{}
			if len(got.Placements) > 0 {
				actual = append(actual, got)
				actual = append(actual, got.Alternatives...)
			}
			if len(actual) != len(oracle) {
				t.Fatalf("trial %d lengths %d != %d", trial, len(actual), len(oracle))
			}
			for i := range oracle {
				if math.Abs(actual[i].Score-oracle[i].Score) > 1e-8 || solutionSignature(actual[i]) != solutionSignature(oracle[i]) {
					t.Fatalf("trial %d rank %d got %s %.12f want %s %.12f", trial, i, solutionSignature(actual[i]), actual[i].Score, solutionSignature(oracle[i]), oracle[i].Score)
				}
			}
		}
	}
}

func TestExactRejectsImpossibleLockAndPartialGrid(t *testing.T) {
	solver, grid, candidates, _, _ := benchmarkFixture()
	solver.Exact = true
	solver.RequiredIDs = map[string]bool{"missing": true}
	got, err := solver.Solve(context.Background(), grid, candidates)
	if err != nil || len(got.Placements) != 0 || !got.Complete {
		t.Fatalf("invalid result %+v %v", got, err)
	}
	solver.RequiredIDs = nil
	got, err = solver.Solve(context.Background(), grid, candidates[:1])
	if err != nil || len(got.Placements) != 0 || !got.Complete {
		t.Fatalf("partial grid %+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, _ = solver.Solve(ctx, grid, candidates)
	if got.Complete {
		t.Fatal("cancelled search reported complete")
	}
}

func TestExactMaximumWithoutMinimum(t *testing.T) {
	if !exceedsMaximum(map[string]float64{"crit": .8}, []ObjectiveGoal{{PropertyID: "crit", Maximum: .7}}) {
		t.Fatal("maximum-only goal ignored")
	}
}

func TestBinomialDoesNotSaturateIntermediateProduct(t *testing.T) {
	if got := binomial(67, 33); got != 14226520737620288370 {
		t.Fatalf("got %d", got)
	}
}

func TestPrefixWorkersMatchEnumeration(t *testing.T) {
	solver, grid, candidates, _, _ := benchmarkFixture()
	for i := 24; i < 40; i++ {
		candidate := candidates[i%24]
		candidate.Module.LocalID = string(rune(100 + i))
		candidate.Score = float64(i)
		candidates = append(candidates, candidate)
	}
	solver.DisableBound = true
	reference, err := solver.Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if reference.Visited != binomial(40, 4) {
		t.Fatalf("prefixes overlap or miss assignments: %d", reference.Visited)
	}
	solver.DisableBound = false
	got, err := solver.Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if got.Score != reference.Score || len(got.Alternatives) != 4 {
		t.Fatalf("got %+v reference %+v", got, reference)
	}
	for i := range got.Alternatives {
		if got.Alternatives[i].Score != reference.Alternatives[i].Score {
			t.Fatal("top-k differs")
		}
	}
}

func TestStatIntervalBoundIncludesNegativeMultiplicativeStats(t *testing.T) {
	rng := rand.New(rand.NewSource(19))
	for trial := 0; trial < 100; trial++ {
		_, _, candidates, e, _ := benchmarkFixture()
		candidates = candidates[:6]
		e.modules = map[string]nte.Module{}
		for i := range candidates {
			candidates[i].Module.MainStats = []nte.Stat{{PropertyID: "AtkBase", Value: float64(rng.Intn(100) - 50)}, {PropertyID: "AtkUp", Value: float64(rng.Intn(20)-10) / 10}, {PropertyID: "AtkAdd", Value: float64(rng.Intn(50) - 25)}}
			e.modules[candidates[i].Module.LocalID] = candidates[i].Module
		}
		e.objectives = []ObjectiveGoal{{PropertyID: "AtkFinal", Minimum: 1000, Maximum: 2000}, {PropertyID: "CritBase", Minimum: .5}}
		e.exact = true
		bound := e.prepareStatBound([]blueprintGroup{{geometry: "S", count: 3}}, map[string][]Candidate{"S": candidates})(nil, 0)
		for a := 0; a < 6; a++ {
			for b := a + 1; b < 6; b++ {
				for c := b + 1; c < 6; c++ {
					placements := []Placement{{ModuleID: candidates[a].Module.LocalID}, {ModuleID: candidates[b].Module.LocalID}, {ModuleID: candidates[c].Module.LocalID}}
					if got := e.Evaluate(placements).Score; got > bound+1e-8 {
						t.Fatalf("inadmissible bound %g < %g", bound, got)
					}
				}
			}
		}
	}
}
