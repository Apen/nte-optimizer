package optimizer

import (
	"context"
	"math/rand"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
	"testing"
)

func TestBoundPreservesTopK(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 100; trial++ {
		solver, grid, candidates, e, _ := benchmarkFixture()
		candidates = candidates[:8]
		for i := range candidates {
			candidates[i].Score = rng.Float64() * 100
			candidates[i].Priority = rng.Float64() * 100
		}
		e.objectives[0].Importance = float64(trial + 1)
		solver.DisableBound = true
		reference, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
		if err != nil {
			t.Fatal(err)
		}
		solver.DisableBound = false
		got, err := solver.SolveWithBonus(context.Background(), grid, candidates, e)
		if err != nil {
			t.Fatal(err)
		}
		wantAll := append([]Solution{reference}, reference.Alternatives...)
		gotAll := append([]Solution{got}, got.Alternatives...)
		if len(wantAll) != len(gotAll) {
			t.Fatal("top-k length")
		}
		for i := range wantAll {
			if wantAll[i].Score != gotAll[i].Score || solutionSignature(wantAll[i]) != solutionSignature(gotAll[i]) {
				t.Fatalf("trial %d rank %d: got %+v want %+v", trial, i, gotAll[i], wantAll[i])
			}
		}
	}
}

func TestPreparedEvaluatorMatchesSummary(t *testing.T) {
	_, _, candidates, e, placements := benchmarkFixture()
	e.character.ConsoleTrait = &scoring.ConsoleTrait{Area: 1, PropertyID: "AtkUp", ValuePerModule: .025}
	e.objectives = append(e.objectives, ObjectiveGoal{PropertyID: "AtkFinal", Minimum: 700, Maximum: 850})
	e.additional = map[string][]nte.Stat{"modules": {{PropertyID: "AtkAdd", Value: 13}}, "console_trait": {{PropertyID: "AtkUp", Value: .1}}, "weapon": {{PropertyID: "AtkBase", Value: 100}}, "weapon_conditional": {{PropertyID: "AtkUp", Value: 100}}}
	prepared := e.prepareBlueprintEvaluator([]string{"S", "S", "S", "S"})
	for i := 0; i < len(candidates); i++ {
		for j := 0; j < i; j++ {
			placements[0].ModuleID = candidates[i].Module.LocalID
			placements[1].ModuleID = candidates[j].Module.LocalID
			got, want := prepared(placements), e.Evaluate(placements)
			if got != want {
				t.Fatalf("got %+v want %+v", got, want)
			}
		}
	}
}
