package damage

import "testing"

func TestCalculateScorchUsesCycleIntensityAndFixedCrit(t *testing.T) {
	zeroDefense := 0.0
	enemy := Enemy{DefenseBase: &zeroDefense}
	low := CalculateScorch(80, 100, 2, enemy, Stats{})
	high := CalculateScorch(80, 280, 2, enemy, Stats{})
	if low.Factors.CritRate != .5 || low.Factors.DamageZone != 1+100.0/600 {
		t.Fatalf("unexpected Scorch factors: %+v", low.Factors)
	}
	if high.Expected <= low.Expected {
		t.Fatalf("cycle intensity should increase Scorch: low=%v high=%v", low.Expected, high.Expected)
	}
	if low.Factors.ScalingStat != ScorchLevel80Base || low.Factors.DefenseZone != 1 {
		t.Fatalf("unexpected base curve or mitigation: %+v", low.Factors)
	}
}
