package app

import (
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func TestRefinementKeepsRequiredModule(t *testing.T) {
	placements := []optimizer.Placement{{ModuleID: "best"}, {ModuleID: "locked"}}
	seed := []nte.Module{{LocalID: "best", Geometry: "SINGLE"}, {LocalID: "locked", Geometry: "SINGLE"}}
	candidates := []optimizer.Candidate{
		{Module: nte.Module{LocalID: "best", Geometry: "SINGLE"}, Score: 100, Priority: 100},
		{Module: nte.Module{LocalID: "second", Geometry: "SINGLE"}, Score: 90, Priority: 90},
		{Module: nte.Module{LocalID: "locked", Geometry: "SINGLE"}, Score: 1, Priority: 1},
	}
	got := refineFixedGeometry(placements, seed, candidates, scoring.Character{}, nil, optimizer.SetDefinition{}, 0, nil, nil, map[string]bool{"locked": true})
	if !containsModule(got, "locked") {
		t.Fatalf("required module was replaced during refinement: %#v", got)
	}
}
