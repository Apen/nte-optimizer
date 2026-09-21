package damage

import (
	"math"
	"testing"
)

func ptr(value float64) *float64 { return &value }

func closeTo(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResistanceMultiplierBranches(t *testing.T) {
	closeTo(t, ResistanceMultiplier(.2, 0, 0), .8)
	closeTo(t, ResistanceMultiplier(.2, 0, .24), 1.0363636363636364)
}

func TestDefenseMultiplierUsesOfficialPanel(t *testing.T) {
	base := 1050.0
	got, exact := DefenseMultiplier(80, Enemy{DefenseBase: &base}, 0, 0)
	if !exact {
		t.Fatal("expected an exact defense profile")
	}
	closeTo(t, got, 180.0/(175.0+180.0))
}

func TestCalculateRoundsDamageOutletsIndependently(t *testing.T) {
	zeroDefense := 0.0
	result := Calculate(80, Stats{Attack: 1000, CritRate: .5, CritDamage: 2}, Enemy{DefenseBase: &zeroDefense}, Instance{
		ID: "hit", Name: "Hit", Category: CategoryDirect, Scaling: ScalingAttack, Coefficient: .333,
		Provenance: Provenance{Confidence: ConfidenceGameData},
	})
	closeTo(t, result.NonCrit, 333)
	closeTo(t, result.Crit, 999)
	closeTo(t, result.Expected, 666)
}

func TestDOTUsesFixedCritAndTiming(t *testing.T) {
	zeroDefense := 0.0
	result := CalculateDOT(80, Stats{Attack: 1000, CritRate: .9, CritDamage: 2}, Enemy{DefenseBase: &zeroDefense}, Instance{
		ID: "dot", Name: "DOT", Category: CategoryDOT, Scaling: ScalingAttack, Coefficient: .2,
		FixedCritRate: ptr(.5), Timing: TimingRule{IntervalSeconds: 1, DurationSeconds: 5, MaxStacks: 3},
		Provenance: Provenance{Confidence: ConfidenceDocumented},
	}, 2)
	closeTo(t, result.Expected, 400)
	closeTo(t, result.Total, 4000)
	if result.Ticks != 5 || result.Stacks != 2 || result.Factors.CritRate != .5 {
		t.Fatalf("unexpected DOT expansion: %+v", result)
	}
}

func TestSimulateAggregatesCategories(t *testing.T) {
	zeroDefense := 0.0
	stats := Stats{Attack: 1000}
	enemy := Enemy{DefenseBase: &zeroDefense}
	events := []Event{
		{At: 1, Instance: Instance{ID: "a", SkillID: "skill", Name: "A", Category: CategoryDirect, Scaling: ScalingAttack, Coefficient: 1, Provenance: Provenance{Confidence: ConfidenceMeasured}}},
		{At: 2, Instance: Instance{ID: "b", SkillID: "dot", Name: "B", Category: CategoryDOT, Scaling: ScalingAttack, Coefficient: .1, Timing: TimingRule{Ticks: 2}, Provenance: Provenance{Confidence: ConfidenceMeasured}}},
	}
	result := Simulate(80, stats, enemy, 10, events)
	closeTo(t, result.TotalExpected, 1200)
	closeTo(t, result.DPS, 120)
	closeTo(t, result.ByCategory[CategoryDirect], 1000)
	closeTo(t, result.ByCategory[CategoryDOT], 200)
}
