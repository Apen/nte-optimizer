package target

import "testing"

func TestCompare(t *testing.T) {
	target := BuildTarget{Goals: []Goal{{PropertyID: "AtkFinal", Minimum: 2000}, {PropertyID: "CritBase", Minimum: .6, Percent: true}}}
	got := Compare(target, map[string]float64{"AtkFinal": 1500, "CritBase": .7})
	if got[0].Missing != 500 || got[0].Progress != .75 || got[0].Reached || !got[1].Reached || got[1].Missing != 0 {
		t.Fatalf("unexpected progress: %#v", got)
	}
}
