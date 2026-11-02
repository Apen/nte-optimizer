package main

import "testing"

func TestPanelBreakIntensityExcludesInactiveModuleSubstats(t *testing.T) {
	id := itemNetID{Solt: 1, Serial: 2}
	characters := []characterItem{{NetID: id, PanelStats: &characterPanelStats{}}}
	items := []inventoryItem{
		{Kind: "module", Level: 0, CharacterNetID: &id, MainStats: []inventoryStat{{Property: "UnbalIntensityBase", Value: 2}}, SubStats: []inventoryStat{{Property: "UnbalIntensityBase", Value: 999}}},
		{Kind: "module", Level: 20, CharacterNetID: &id, SubStats: []inventoryStat{{Property: "UnbalIntensityBase", Value: 12}}},
		{Kind: "core", Level: 0, CharacterNetID: &id, SubStats: []inventoryStat{{Property: "UnbalIntensityBase", Value: 60}}},
	}
	attachEquipmentPanelStats(characters, items)
	if got := characters[0].PanelStats.BreakIntensity; got != 74 {
		t.Fatalf("break intensity = %g, want 74", got)
	}
	if items[0].SubStats[0].Value != 999 {
		t.Fatal("raw inactive substat was changed")
	}
}
