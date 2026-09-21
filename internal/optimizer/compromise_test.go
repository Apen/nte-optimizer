package optimizer

import (
	"context"
	"fmt"
	"math"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestCompromiseSelectsOverallScoreAndEnoughCopies(t *testing.T) {
	input := make([]Candidate, 30)
	for i := range input {
		input[i] = Candidate{Module: nte.Module{LocalID: fmt.Sprint(i), Geometry: "S", Area: 1}, Score: float64(i), Priority: float64(30 - i)}
	}
	got := SelectCompromiseCandidates(input, 20)
	if len(got) != 22 || got[0].Score != 29 || got[len(got)-1].Score != 8 {
		t.Fatalf("unexpected score selection: %+v", got)
	}
}

func TestCompromiseRefinementMatchesSmallExactSearchWithLockAndCap(t *testing.T) {
	solver, grid, candidates, evaluator := fixedFixture(10)
	solver.RequiredIDs = map[string]bool{"item000": true}
	seed, err := solver.SolveWithBonus(context.Background(), grid, candidates[:3], evaluator)
	if err != nil || len(seed.Placements) != 3 {
		t.Fatalf("seed: %+v %v", seed, err)
	}
	full, err := solver.SolveWithBonus(context.Background(), grid, candidates, evaluator)
	if err != nil {
		t.Fatal(err)
	}
	got := RefineCompromise(context.Background(), seed, candidates, evaluator, solver.RequiredIDs, 5)
	assertSameLeaders(t, full, got)
	if got.Score <= seed.Score {
		t.Fatal("discarded candidates did not improve the seed")
	}
	for _, solution := range append([]Solution{got}, got.Alternatives...) {
		if solution.Complete || !placementsContainRequired(solution.Placements, solver.RequiredIDs) {
			t.Fatal("lost lock or claimed optimality")
		}
		if math.IsInf(evaluator.Evaluate(solution.Placements).Score, -1) {
			t.Fatal("hard cap violated")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopped := RefineCompromise(ctx, seed, candidates, evaluator, solver.RequiredIDs, 5)
	if stopped.Score != seed.Score || stopped.Complete {
		t.Fatal("cancellation lost incumbent")
	}
}
