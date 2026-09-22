package optimizer

import (
	"math"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func TestBuildStatSummarySeparatesConditionalSetEffect(t *testing.T) {
	character := scoring.Character{
		BaseStats:    map[string]float64{"AtkBase": 660, "CritDamageBase": .5},
		ConsoleTrait: &scoring.ConsoleTrait{Area: 3, PropertyID: "CritDamageBase", ValuePerModule: .16},
	}
	modules := []nte.Module{{Area: 3, MainStats: []nte.Stat{{PropertyID: "AtkUp", Value: .10}}}}
	cartridge := &nte.Cartridge{MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .60}}}
	set := SetDefinition{Bonuses: []SetBonus{
		{Count: 2, Stats: []nte.Stat{{PropertyID: "DamageUpIncantationBase", Value: .10}}},
		{Count: 4, ConditionalStats: []nte.Stat{{PropertyID: "AtkUp", Value: .36}}},
	}}
	got := BuildStatSummary(character, modules, cartridge, set, 4)
	if !near(got.Derived["AtkFinal"], 726) || !near(got.DerivedConditional["AtkFinal"], 963) {
		t.Fatalf("ATK totals = %.2f / %.2f", got.Derived["AtkFinal"], got.DerivedConditional["AtkFinal"])
	}
	if !near(got.Derived["CritDamageBase"], 1.26) || !near(got.Derived["DamageUpIncantationBase"], .10) {
		t.Fatalf("unexpected derived stats: %#v", got.Derived)
	}
}

func TestBasicDamageIndexUsesExpectedCriticalDamage(t *testing.T) {
	stats := map[string]float64{
		"AtkFinal":            1000,
		"CritBase":            .5,
		"CritDamageBase":      2,
		"DamageUpGeneralBase": .2,
	}
	if got := BasicDamageIndex(stats); !near(got, 1800) {
		t.Fatalf("basic damage index = %v, want 1800", got)
	}
	stats["CritBase"] = 1.5
	if got := BasicDamageIndex(stats); !near(got, 2400) {
		t.Fatalf("crit rate should be capped at 100%%: got %v", got)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.000001 }

func TestDeriveUsesHPMaxUpProperty(t *testing.T) {
	got := derive(map[string]float64{"HPMaxBase": 15514, "HPMaxUp": .1875, "HPMaxAdd": 6800})
	if !near(got["HPFinal"], 25222) {
		t.Fatalf("HPFinal = %f", got["HPFinal"])
	}
}
