package app

import (
	"path/filepath"
	"reflect"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/scoring"
)

func TestCalculationContextDoesNotMutateAccountOrCachedProfiles(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	profiles, err := s.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	profile := profiles["zankou"]
	profiles["zankou"].BaseStats["AtkBase"] = 99999
	again, err := s.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	if again["zankou"].BaseStats["AtkBase"] == 99999 {
		t.Fatal("cached profile mutated")
	}
	id := decoded.NetID{Slot: 4, Serial: 1}
	state := decoded.State{Characters: []decoded.Character{{CharacterID: 1036, Name: "raw", ForkNetID: &id}}, Weapons: []decoded.Weapon{{ID: id, ForkID: "fork_DemonBlade", Name: "raw", Level: 80, Breakthrough: 6}}}
	before := state.Characters[0]
	weaponBefore := state.Weapons[0]
	_, weapon, _, sources, _, err := s.resolveAccountContext(&state, profile, "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.prepareArcSelection(&state, scoring.Character{CharacterID: 1036}, weapon, sources, "fork_DemonBlade", "fr", 1, scoring.References{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, state.Characters[0]) || !reflect.DeepEqual(weaponBefore, state.Weapons[0]) {
		t.Fatal("localization mutated imported facts")
	}
}
