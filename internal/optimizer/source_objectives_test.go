package optimizer

import (
	"math"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
	"testing"
)

func TestSourceObjectivePreservesMainAndSubWeights(t *testing.T) {
	character := scoring.Character{MainWeights: map[string]float64{"MagBase": 1}, SubWeights: map[string]float64{"MagBase": .85}}
	goals := []ObjectiveGoal{{PropertyID: "MagBase", Minimum: 200, Importance: 1, SourceWeights: true}}
	modules := []nte.Module{{MainStats: []nte.Stat{{PropertyID: "MagBase", Value: 100}}, SubStats: []nte.Stat{{PropertyID: "MagBase", Value: 100}}}}
	score, rows := SourceObjectiveScore(map[string]float64{"MagBase": 200}, modules, nil, character, goals)
	if math.Abs(score-9.25) > 1e-10 || len(rows) != 2 || rows[0].Points != 5 || rows[1].Points != 4.25 {
		t.Fatalf("score=%g rows=%+v", score, rows)
	}
	character.MainWeights["MagBase"] = 0
	score, rows = SourceObjectiveScore(map[string]float64{"MagBase": 200}, modules, nil, character, goals)
	if score != 4.25 || rows[0].Points != 0 {
		t.Fatalf("zero main weight ignored: %g %+v", score, rows)
	}
}

func TestSourceWeightedPreparedAndCachedEvaluatorMatchReference(t *testing.T) {
	_, _, candidates, e := fixedFixture(14)
	e.character.MainWeights = map[string]float64{"CritBase": 1}
	e.character.SubWeights = map[string]float64{"CritBase": .85}
	e.objectives = []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .5, Importance: 1, SourceWeights: true, ScoreScale: .64}}
	// Equal totals and raw equipment scores, different source attribution.
	a, b := candidates[0].Module, candidates[1].Module
	a.MainStats = []nte.Stat{{PropertyID: "CritBase", Value: .5}}
	a.SubStats = nil
	b.MainStats = nil
	b.SubStats = []nte.Stat{{PropertyID: "CritBase", Value: .5}}
	e.modules[a.LocalID] = a
	e.modules[b.LocalID] = b
	e.exact = false
	counts := statCacheCounts{}
	cached := e.cachedBlueprintEvaluations([]string{a.Geometry}, 1, e.statProjection(), &counts)
	scores := []float64{}
	for _, id := range []string{a.LocalID, b.LocalID, a.LocalID} {
		placements := []Placement{{ModuleID: id}}
		reference := e.Evaluate(placements)
		got := math.Inf(-1)
		cached(placements, func(result BonusResult) { got = math.Max(got, result.Score) })
		if math.Abs(got-reference.Score) > 1e-8 {
			t.Fatalf("prepared/cache=%g reference=%g", got, reference.Score)
		}
		scores = append(scores, got)
	}
	if scores[0] <= scores[1] || scores[0] != scores[2] {
		t.Fatalf("source distinction lost: %v", scores)
	}
	if counts.hits == 0 {
		t.Fatal("source-aware cache failed to reuse identical state")
	}
}
