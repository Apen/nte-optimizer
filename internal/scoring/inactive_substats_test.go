package scoring

import (
	"nte-optimizer/internal/nte"
	"testing"
)

func TestInactiveModuleSubstatsDoNotInflateRelevance(t *testing.T) {
	character := Character{Weights: map[string]float64{"CritBase": 1}}
	refs := References{"CritBase": .1}
	module := nte.Module{MainStats: []nte.Stat{{PropertyID: "CritBase", Value: .1}}, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: 1}}}
	if got := ScoreDetailed(module, character, refs); got.Total != 1 || len(got.Contributions) != 1 {
		t.Fatalf("inactive stats scored: %+v", got)
	}
	module.Level = 20
	if got := Score(module, character, refs); got != 11 {
		t.Fatalf("active score = %g", got)
	}
}
