package optimizer

import (
	"context"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestFractionalBoundUsesBestRemainingScoreDensity(t *testing.T) {
	candidates := []preparedCandidate{
		{Candidate: Candidate{Score: 12}, area: 2},
		{Candidate: Candidate{Score: 15}, area: 5},
		{Candidate: Candidate{Score: 100}, area: 0},
	}

	if got, want := fractionalBound(candidates, 1, 6), 18.0; got != want {
		t.Fatalf("fractionalBound() = %g, want %g", got, want)
	}
	if got := fractionalBound(candidates, len(candidates), 6); got != 0 {
		t.Fatalf("fractionalBound() with no candidates = %g, want 0", got)
	}
}

func TestPrepareGroupOptionsEnumeratesSharedSmallGroups(t *testing.T) {
	blueprints := []geometryBlueprint{
		{geometries: []string{"A", "A", "B"}},
		{geometries: []string{"A", "A", "C"}},
	}
	pools := map[string][]Candidate{
		"A": {
			{Module: nte.Module{LocalID: "a"}, Score: 1},
			{Module: nte.Module{LocalID: "b"}, Score: 2},
			{Module: nte.Module{LocalID: "c"}, Score: 4},
		},
		"B": {{Module: nte.Module{LocalID: "only-used-once"}, Score: 10}},
		"C": {{Module: nte.Module{LocalID: "also-used-once"}, Score: 20}},
	}

	options := prepareGroupOptions(context.Background(), blueprints, pools)
	group := blueprintGroup{geometry: "A", count: 2}
	got := options[group]
	if len(got) != 3 {
		t.Fatalf("cached options = %d, want C(3, 2) = 3", len(got))
	}
	wantIndices := [][]int{{0, 1}, {0, 2}, {1, 2}}
	wantScores := []float64{3, 5, 6}
	for index, option := range got {
		if len(option.indices) != 2 || option.indices[0] != wantIndices[index][0] || option.indices[1] != wantIndices[index][1] || option.score != wantScores[index] {
			t.Errorf("option %d = %#v, want indices %v and score %g", index, option, wantIndices[index], wantScores[index])
		}
	}
	if _, ok := options[blueprintGroup{geometry: "B", count: 1}]; ok {
		t.Fatal("group used by only one blueprint was cached")
	}
}

func TestPrepareGroupOptionsSkipsOversizedOrCanceledGroups(t *testing.T) {
	largePool := make([]Candidate, 16)
	for index := range largePool {
		largePool[index].Module.LocalID = "large"
	}
	largeBlueprint := geometryBlueprint{geometries: []string{"A", "A", "A", "A", "A", "A", "A", "A"}}
	large := prepareGroupOptions(context.Background(), []geometryBlueprint{largeBlueprint, largeBlueprint}, map[string][]Candidate{"A": largePool})
	if _, ok := large[blueprintGroup{geometry: "A", count: 8}]; ok {
		t.Fatal("group with more than 4096 combinations was cached")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	smallBlueprint := geometryBlueprint{geometries: []string{"B", "B"}}
	canceled := prepareGroupOptions(ctx, []geometryBlueprint{smallBlueprint, smallBlueprint}, map[string][]Candidate{"B": {{Score: 1}, {Score: 2}, {Score: 3}}})
	if _, ok := canceled[blueprintGroup{geometry: "B", count: 2}]; ok {
		t.Fatal("group options were published after cancellation")
	}
}
