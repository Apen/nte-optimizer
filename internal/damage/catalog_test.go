package damage

import "testing"

func TestTieredInstanceClampsSkillLevel(t *testing.T) {
	entry := TieredInstance{ID: "x", Coefficients: []float64{.1, .2}, Scaling: ScalingAttack}
	if got := entry.Resolve("1036", 10).Coefficient; got != .2 {
		t.Fatalf("got %v", got)
	}
	if got := entry.Resolve("1036", 0).Coefficient; got != .1 {
		t.Fatalf("got %v", got)
	}
}

func TestStatsFromOptimizer(t *testing.T) {
	got := StatsFromOptimizer(map[string]float64{
		"AtkFinal": 2000, "CritBase": .6, "CritDamageBase": 2,
		"DamageUpGeneralBase": .1, "DamageUpIncantationBase": .2,
	}, "Incantation", false)
	if got.Attack != 2000 || got.GeneralDamage != .1 || got.ElementDamage != .2 {
		t.Fatalf("unexpected stats: %+v", got)
	}
}
