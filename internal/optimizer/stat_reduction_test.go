package optimizer

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
	"slices"
	"testing"
)

func fixedFixture(n int) (SearchSolver, Grid, []Candidate, CartridgeSetEvaluator) {
	grid, _ := NewGrid(3, 1, nil)
	solver := SearchSolver{Exact: true, KeepBest: 5, DisableBound: true, Catalog: ShapeCatalog{Shapes: map[string]Shape{"S": {ID: "S", Cells: []Point{{0, 0}}}}}}
	candidates := make([]Candidate, n)
	for i := range candidates {
		candidates[i] = Candidate{Module: nte.Module{LocalID: fmt.Sprintf("item%03d", i), Geometry: "S", Area: 1, MainStats: []nte.Stat{{PropertyID: "AtkAdd", Value: 8}}, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .02 * float64(1+i%2)}, {PropertyID: "CritDamageBase", Value: .04}}}, Score: float64(i % 4), Priority: float64(n - i)}
	}
	sets := SetCatalog{Definitions: map[string]SetDefinition{"set": {ID: "set", InventorySetID: "set", RequiredGeometries: []string{"S"}, Bonuses: []SetBonus{{Count: 1, Score: 3, ConditionalStats: []nte.Stat{{PropertyID: "DamageUpGeneralBase", Value: .1}}}}}}}
	cartridges := []nte.Cartridge{{LocalID: "c1", SetID: "set"}, {LocalID: "c2", SetID: "set", MainStats: []nte.Stat{{PropertyID: "AtkUp", Value: .05}}}}
	evaluator := NewExactObjectiveEvaluator(sets, cartridges, nil, candidates, scoring.Character{BaseStats: map[string]float64{"AtkBase": 100}}, nil, []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .08, Maximum: .11}, {PropertyID: "AtkFinal", Minimum: 120}}, nil)
	return solver, grid, candidates, evaluator
}

func assertSameLeaders(t *testing.T, a, b Solution) {
	t.Helper()
	flatten := func(s Solution) []Solution {
		if len(s.Placements) == 0 {
			return nil
		}
		return append([]Solution{s}, s.Alternatives...)
	}
	left, right := flatten(a), flatten(b)
	if len(left) != len(right) {
		t.Fatalf("lengths %d != %d", len(left), len(right))
	}
	for i := range left {
		if math.Abs(left[i].Score-right[i].Score) > 1e-8 || solutionSignature(left[i]) != solutionSignature(right[i]) {
			t.Fatalf("rank %d: %s %.12f != %s %.12f", i, solutionSignature(left[i]), left[i].Score, solutionSignature(right[i]), right[i].Score)
		}
	}
}

func TestStatCacheKeepsIdentityAlternatives(t *testing.T) {
	solver, grid, candidates, e := fixedFixture(14)
	solver.RequiredIDs = map[string]bool{"item000": true}
	solver.DisableStatReduction = true
	reference, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
	if err != nil {
		t.Fatal(err)
	}
	solver.DisableStatReduction = false

	got, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
	if err != nil {
		t.Fatal(err)
	}
	assertSameLeaders(t, reference, got)
	if got.Metrics.StatCacheHits == 0 {
		t.Fatal("equivalent states were not cached")
	}
	for _, s := range append([]Solution{got}, got.Alternatives...) {
		if !placementsContainRequired(s.Placements, solver.RequiredIDs) {
			t.Fatal("lost locked identity")
		}
	}
}

func TestTopKDominanceRequiresEnoughReplacements(t *testing.T) {
	solver, grid, candidates, e := fixedFixture(8)
	for i := range candidates {
		candidates[i].Module.SubStats = nil
		candidates[i].Score = 2
	}
	candidates[0].Score = 0
	for i := range candidates {
		e.modules[candidates[i].Module.LocalID] = candidates[i].Module
	}
	// M=3 and K=5: seven dominators are necessary. Six are insufficient.
	keep := e.reduceStatCandidates(candidates[:7], solver.Catalog, grid.FreeCells(), 5, nil)
	if len(keep) != 7 {
		t.Fatal("single-optimum filter incorrectly used for top K")
	}
	keep = e.reduceStatCandidates(candidates, solver.Catalog, grid.FreeCells(), 5, nil)
	if len(keep) != 7 {
		t.Fatal("provably dominated candidate was not removed")
	}
	keep = e.reduceStatCandidates(candidates, solver.Catalog, grid.FreeCells(), 5, map[string]bool{"item000": true})
	if len(keep) != 8 {
		t.Fatal("removed required item")
	}
	// A stronger capped stat is not a dominance certificate.
	candidates[0].Module.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: .02}}
	for i := 1; i < len(candidates); i++ {
		candidates[i].Module.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: .04}}
	}
	for i := range candidates {
		e.modules[candidates[i].Module.LocalID] = candidates[i].Module
	}
	if got := e.reduceStatCandidates(candidates, solver.Catalog, grid.FreeCells(), 5, nil); len(got) != 8 {
		t.Fatal("cap ignored")
	}
}

func TestReduceBlueprintCandidatesKeepsGeometryPoolsConsistent(t *testing.T) {
	solver, grid, candidates, evaluator := fixedFixture(8)
	for i := range candidates {
		candidates[i].Module.SubStats = nil
		candidates[i].Score = 2
		evaluator.modules[candidates[i].Module.LocalID] = candidates[i].Module
	}
	candidates[0].Score = 0
	usable, pools, err := solver.prepareCandidatePools(candidates)
	if err != nil {
		t.Fatal(err)
	}

	reduction := solver.reduceBlueprintCandidates(grid, usable, pools, []geometryBlueprint{{geometries: []string{"S", "S", "S"}}}, evaluator)
	if len(reduction.usable) != 7 || len(pools["S"]) != 7 {
		t.Fatalf("reduced candidates = %d, geometry pool = %d; want 7 and 7", len(reduction.usable), len(pools["S"]))
	}
	retained := make(map[string]bool, len(reduction.usable))
	for _, candidate := range reduction.usable {
		retained[candidate.Module.LocalID] = true
	}
	for _, candidate := range pools["S"] {
		if !retained[candidate.Module.LocalID] {
			t.Fatalf("geometry pool retained removed candidate %q", candidate.Module.LocalID)
		}
	}
	if reduction.statValues == 0 || reduction.profiles == 0 {
		t.Fatalf("missing reduction metadata: %#v", reduction)
	}
}

func TestProjectionPreservesFractionOrderAndUnknownValues(t *testing.T) {
	_, _, candidates, e := fixedFixture(3)
	candidates[0].Module.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: .02}, {PropertyID: "Unrelated", Value: 999}}
	candidates[1].Module.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: .03}}
	candidates[2].Module.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: .031}}
	for _, c := range candidates {
		e.modules[c.Module.LocalID] = c.Module
	}
	p := e.statProjection()
	scratch := p.scratch()
	key, _ := p.key([]Placement{{ModuleID: "item000"}, {ModuleID: "item001"}}, &scratch)
	first := slices.Clone(key)
	key, _ = p.key([]Placement{{ModuleID: "item001"}, {ModuleID: "item000"}}, &scratch)
	if slices.Equal(first, key) {
		t.Fatal("fractional additions were silently reordered")
	}
	key, _ = p.key([]Placement{{ModuleID: "item000"}, {ModuleID: "item002"}}, &scratch)
	if slices.Equal(first, key) {
		t.Fatal("unobserved value rounded to census value")
	}
	for _, property := range p.properties {
		if property == "Unrelated" {
			t.Fatal("irrelevant property in projection")
		}
	}
	if !slices.Contains(p.properties, "AtkBase") || !slices.Contains(p.properties, "AtkUp") || !slices.Contains(p.properties, "AtkAdd") {
		t.Fatal("derived dependencies missing")
	}
}

func TestStatCacheCollisionEvictsWithoutChangingScores(t *testing.T) {
	_, _, candidates, e := fixedFixture(1100)
	for i := range candidates {
		module := candidates[i].Module
		module.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: float64(i) * .00001}}
		e.modules[module.LocalID] = module
	}
	p := e.statProjection()
	scratch := p.scratch()
	slots := map[uint64]string{}
	first, second := "", ""
	for _, candidate := range candidates {
		_, hash := p.key([]Placement{{ModuleID: candidate.Module.LocalID}}, &scratch)
		slot := hash % 1024
		if previous, ok := slots[slot]; ok {
			first, second = previous, candidate.Module.LocalID
			break
		}
		slots[slot] = candidate.Module.LocalID
	}
	if first == "" {
		t.Fatal("expected a direct-mapped cache collision")
	}
	counts := &statCacheCounts{}
	cached := e.cachedBlueprintEvaluations([]string{"S"}, 5, p, counts)
	reference := e.prepareBlueprintEvaluator([]string{"S"})
	for _, id := range []string{first, second, first, first} {
		placements := []Placement{{ModuleID: id}}
		want := reference(placements)
		got := BonusResult{Score: math.Inf(-1)}
		cached(placements, func(result BonusResult) {
			if result.Score > got.Score {
				got = result
			}
		})
		if got != want {
			t.Fatalf("cache collision changed result: %+v != %+v", got, want)
		}
	}
	if counts.hits != 1 || counts.misses != 3 {
		t.Fatalf("unexpected counters %+v", counts)
	}
}

func TestStatReductionAgainstUnreducedEnumeration(t *testing.T) {
	rng := rand.New(rand.NewSource(8806))
	for trial := 0; trial < 250; trial++ {
		solver, grid, candidates, e := fixedFixture(10)
		for i := range candidates {
			candidates[i].Score = float64(rng.Intn(9))
			candidates[i].Priority = rng.Float64()
			candidates[i].Module.MainStats[0].Value = float64(8 * (1 + rng.Intn(3)))
			candidates[i].Module.SubStats[0].Value = .02 * float64(rng.Intn(3))
			e.modules[candidates[i].Module.LocalID] = candidates[i].Module
		}
		if trial%2 == 0 {
			e.objectives[0].Maximum = 0
		}
		if trial%3 == 0 {
			solver.RequiredIDs = map[string]bool{"item000": true}
		}
		if trial%7 == 0 {
			candidates[0].Module.SubStats[0].Value = -.01
			e.modules[candidates[0].Module.LocalID] = candidates[0].Module
		}
		solver.DisableStatReduction = true
		reference, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
		if err != nil {
			t.Fatal(err)
		}
		solver.DisableStatReduction = false
		solver.DisableBound = trial%2 == 0
		got, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
		if err != nil {
			t.Fatal(err)
		}
		assertSameLeaders(t, reference, got)
	}
}

func BenchmarkFixedStatSearch(b *testing.B) {
	for _, reduction := range []bool{false, true} {
		b.Run(fmt.Sprintf("reduction=%v", reduction), func(b *testing.B) {
			solver, grid, candidates, e := fixedFixture(32)
			solver.DisableStatReduction = !reduction
			e.objectives[0].Maximum = 0
			template := e.sets[0]
			e.sets = nil
			for i := 0; i < 12; i++ {
				set := template
				set.cartridge.LocalID = fmt.Sprint("cart", i)
				e.sets = append(e.sets, set)
			}
			b.ReportAllocs()
			for b.Loop() {
				got, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(got.Metrics.StatCacheHits), "cache_hits/op")
			}
		})
	}
}
