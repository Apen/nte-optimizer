package nte

import (
	"encoding/json"
	"testing"
)

func TestModuleActivationPreservesRawStatsAndJSONRoundTrip(t *testing.T) {
	for _, level := range []int{0, 1, 20} {
		module := Module{Level: level, SubStats: []Stat{{PropertyID: "CritBase", Value: .03}}}
		if got := len(module.ActiveSubStats()); (got == 1) != (level != 0) {
			t.Fatalf("level %d active substats = %d", level, got)
		}
		data, err := json.Marshal(module)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			SubStatsActive bool `json:"sub_stats_active"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.SubStatsActive != (level != 0) {
			t.Fatalf("level %d activation flag = %v", level, wire.SubStatsActive)
		}
		var restored Module
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if restored.Level != level || len(restored.SubStats) != 1 || restored.SubStats[0].Value != .03 {
			t.Fatalf("lost raw stats: %+v", restored)
		}
	}
}
