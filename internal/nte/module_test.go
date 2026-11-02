package nte

import "testing"

func TestInventoryNormalizeUsesProtocolSetIDs(t *testing.T) {
	inventory := Inventory{Cartridges: []Cartridge{
		{GameItemID: "Incantation_orange", SetID: "ecarlate_:_papillons_jumeaux"},
		{GameItemID: "GetEfficiency_purple", SetID: "legacy-localized-name"},
		{GameItemID: "Unknown_orange", SetID: "preserved"},
	}}
	inventory.Normalize(EquipmentDefinitions{"Incantation_orange": {Kind: "core", SetID: "Suit4"}, "GetEfficiency_purple": {Kind: "core", SetID: "Suit11"}})
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
