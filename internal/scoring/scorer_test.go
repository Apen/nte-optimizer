package scoring

import (
	"math"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestScoreDetailedSeparatesMainAndSubWeights(t *testing.T) {
	m := nte.Module{
		MainStats: []nte.Stat{{PropertyID: "CritBase", Value: 0.064, Percent: true}},
		SubStats:  []nte.Stat{{PropertyID: "CritBase", Value: 0.064, Percent: true}},
	}
	c := Character{MainWeights: map[string]float64{"CritBase": 0.5}, SubWeights: map[string]float64{"CritBase": 1}}
	got := ScoreDetailed(m, c, References{"CritBase": 0.064})
	if math.Abs(got.Total-1.5) > 1e-9 {
		t.Fatalf("score = %v, want 1.5", got.Total)
	}
	if len(got.Contributions) != 2 || got.Contributions[0].Source != "main" || got.Contributions[1].Source != "sub" {
		t.Fatalf("unexpected contributions: %#v", got.Contributions)
	}
}

func TestGeneralWeightAppliesToMainAndSubStats(t *testing.T) {
	module := nte.Module{
		MainStats: []nte.Stat{{PropertyID: "MagBase", Value: 100}},
		SubStats:  []nte.Stat{{PropertyID: "MagBase", Value: 50}},
	}
	result := ScoreDetailed(module, Character{Weights: map[string]float64{"MagBase": .8}}, References{"MagBase": 100})
	if math.Abs(result.Total-1.2) > 1e-12 || len(result.Contributions) != 2 || result.Contributions[0].Weight != .8 || result.Contributions[1].Weight != .8 {
		t.Fatalf("general weight not applied to both stat sources: %#v", result)
	}
}

func TestDeterministicModuleSubstatsKeepEqualPerCellRelevance(t *testing.T) {
	character := Character{SubWeights: map[string]float64{"CritBase": 1}}
	references := References{"CritBase": 0.02}
	tests := []struct {
		area  int
		value float64
	}{
		{area: 2, value: 0.02},
		{area: 3, value: 0.03},
		{area: 4, value: 0.04},
	}
	for _, test := range tests {
		module := nte.Module{Area: test.area, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: test.value, Percent: true}}}
		density := Score(module, character, references) / float64(test.area)
		if math.Abs(density-0.5) > 1e-12 {
			t.Fatalf("area %d relevance density = %v, want 0.5", test.area, density)
		}
	}
}

func TestScoreDetailedAppliesSoftAndHardCaps(t *testing.T) {
	m := nte.Module{SubStats: []nte.Stat{{PropertyID: "CritBase", Value: 0.2, Percent: true}}}
	c := Character{
		SubWeights:   map[string]float64{"CritBase": 1},
		CurrentStats: map[string]float64{"CritBase": 0.85},
		Caps: map[string]StatCap{"CritBase": {
			SoftCap: 0.9, HardCap: 1, AfterSoftScale: 0.25,
		}},
	}
	got := ScoreDetailed(m, c, References{"CritBase": 0.1})
	// 0.05 before soft cap + 0.10 up to hard cap valued at 25% = 0.075.
	if math.Abs(got.Total-0.75) > 1e-9 {
		t.Fatalf("score = %v, want 0.75", got.Total)
	}
	if !got.Contributions[0].Capped {
		t.Fatal("expected capped contribution")
	}
}

func TestValidateCharacterRejectsUnknownReference(t *testing.T) {
	err := ValidateCharacter("test", Character{Weights: map[string]float64{"Unknown": 1}}, References{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
