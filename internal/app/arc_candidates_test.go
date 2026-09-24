package app

import (
	"path/filepath"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func TestPrepareArcSelectionUsesStrongestOwnedCopy(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	state := &decoded.State{Weapons: []decoded.Weapon{
		{ID: decoded.NetID{Slot: 4, Serial: 1}, ForkID: "fork_DemonBlade", Level: 50, Breakthrough: 1, Star: 1},
		{ID: decoded.NetID{Slot: 4, Serial: 2}, ForkID: "fork_DemonBlade", Level: 80, Breakthrough: 6, Star: 5},
	}}

	got, err := service.prepareArcSelection(state, scoring.Character{CharacterID: 1036}, nil, nil, "fork_DemonBlade", "en", 1, scoring.References{})
	if err != nil {
		t.Fatal(err)
	}
	if got.option == nil || got.option.ID != "4:2" {
		t.Fatalf("selected Arc copy = %#v, want highest-level owned copy 4:2", got.option)
	}
	if got.weapons[got.option.ID].Level != 80 {
		t.Fatalf("selected Arc level = %d, want 80", got.weapons[got.option.ID].Level)
	}
	if got.additional[got.option.ID]["weapon"][0].PropertyID != "AtkBase" || got.additional[got.option.ID]["weapon"][0].Value != 570 {
		t.Fatalf("selected Arc panel stats = %#v", got.additional[got.option.ID]["weapon"])
	}
}

func TestNoArcSelectionRemovesCurrentlyEquippedArcStats(t *testing.T) {
	service := OptimizerService{}
	current := decoded.Weapon{ID: decoded.NetID{Slot: 1, Serial: 2}, ForkID: "fork_DemonBlade"}
	additional := map[string][]nte.Stat{
		"panel_calibration":  {{PropertyID: "AtkBase", Value: 25}},
		"weapon":             {{PropertyID: "AtkBase", Value: 570}},
		"weapon_permanent":   {{PropertyID: "CritDamageBase", Value: .8}},
		"weapon_conditional": {{PropertyID: "CritRate", Value: .1}},
	}

	got, err := service.prepareArcSelection(nil, scoring.Character{CharacterID: 1036}, &current, additional, NoArcForkSelection, "en", 1, scoring.References{})
	if err != nil {
		t.Fatal(err)
	}
	if got.option != nil || len(got.additional[""]["weapon"]) != 0 || len(got.additional[""]["weapon_permanent"]) != 0 || len(got.additional[""]["weapon_conditional"]) != 0 {
		t.Fatalf("no-Arc selection retained weapon stats: %#v", got)
	}
	if stats := got.additional[""]["panel_calibration"]; len(stats) != 1 || stats[0].Value != 25 {
		t.Fatalf("no-Arc selection lost unrelated profile stats: %#v", stats)
	}
}
