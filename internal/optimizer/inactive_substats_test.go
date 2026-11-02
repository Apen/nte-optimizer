package optimizer

import (
	"math"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
	"testing"
)

func TestInactiveSubstatsCannotSatisfyStrictFloorAcrossEvaluators(t *testing.T) {
	character := scoring.Character{BaseStats: map[string]float64{"AtkBase": 100}, ConsoleTrait: &scoring.ConsoleTrait{Area: 2, PropertyID: "CritBase", ValuePerModule: .1}}
	module := nte.Module{LocalID: "inactive", Geometry: "H_2", Area: 2, MainStats: []nte.Stat{{PropertyID: "AtkAdd", Value: 12}}, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .5}, {PropertyID: "AtkAdd", Value: 999}}}
	stats := BuildStatSummary(character, []nte.Module{module}, nil, SetDefinition{}, 0)
	if stats.Derived["AtkFinal"] != 112 || stats.Derived["CritBase"] != .1 {
		t.Fatalf("inactive stats or missing console trait: %+v", stats.Derived)
	}
	placements := []Placement{{ModuleID: module.LocalID}}
	for _, active := range []bool{false, true} {
		if active {
			module.Level = 20
		}
		sets := SetCatalog{Definitions: map[string]SetDefinition{"test": {ID: "test", InventorySetID: "test", RequiredGeometries: []string{module.Geometry}}}}
		e := NewExactObjectiveEvaluator(sets, []nte.Cartridge{{LocalID: "core", SetID: "test"}}, nil, []Candidate{{Module: module}}, character, nil, []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .5, StrictMinimum: true}}, nil)
		result := e.Evaluate(placements)
		if math.IsInf(result.Score, -1) == active {
			t.Fatalf("active=%v feasibility score=%g", active, result.Score)
		}
		cached := e.cachedBlueprintEvaluations([]string{module.Geometry}, 1, e.statProjection(), &statCacheCounts{})
		for repeat := 0; repeat < 2; repeat++ {
			got := math.Inf(-1)
			cached(placements, func(r BonusResult) { got = r.Score })
			if got != result.Score {
				t.Fatalf("active=%v cached=%g reference=%g", active, got, result.Score)
			}
		}
	}
}
