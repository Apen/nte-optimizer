package nte

import "testing"

func TestInventoryNormalizeUsesProtocolSetIDs(t *testing.T) {
	inventory := Inventory{Cartridges: []Cartridge{
		{GameItemID: "Incantation_orange", SetID: "ecarlate_:_papillons_jumeaux"},
		{GameItemID: "GetEfficiency_purple", SetID: "legacy-localized-name"},
		{GameItemID: "Unknown_orange", SetID: "preserved"},
	}}
	inventory.Normalize()
	if got := inventory.Cartridges[0].SetID; got != "Suit4" {
		t.Fatalf("Incantation set = %q, want Suit4", got)
	}
	if got := inventory.Cartridges[1].SetID; got != "Suit11" {
		t.Fatalf("GetEfficiency set = %q, want Suit11", got)
	}
	if got := inventory.Cartridges[2].SetID; got != "preserved" {
		t.Fatalf("unknown protocol prefix should preserve set id, got %q", got)
	}
}

func TestEveryProtocolCartridgeSetHasStableID(t *testing.T) {
	want := map[string]string{
		"Chaos": "Suit1", "Nature": "Suit2", "Psyche": "Suit3", "Incantation": "Suit4",
		"Lakshana": "Suit5", "Cosmos": "Suit6", "Shield": "Suit7", "Attack": "Suit8",
		"Heal": "Suit9", "Mag": "Suit10", "GetEfficiency": "Suit11", "Psychically": "Suit12",
	}
	for prefix, expected := range want {
		got, ok := CartridgeSetID(prefix + "_orange")
		if !ok || got != expected {
			t.Errorf("CartridgeSetID(%q) = %q, %v; want %q, true", prefix, got, ok, expected)
		}
	}
}
