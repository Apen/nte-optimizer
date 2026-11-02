package app

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func TestCurrentStatsDoNotRequireACharacterStrategy(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	if _, err := s.loadCharacters(); err != nil {
		t.Fatal(err)
	}
	for id, profile := range s.static.profiles {
		if profile.CharacterID == 1004 {
			delete(s.static.profiles, id)
		}
	}
	c := decoded.Character{CharacterID: 1004, Level: 80, BreakthroughLevel: 6}
	stats, err := s.currentCharacterStats(&decoded.State{Characters: []decoded.Character{c}}, c, nil, nil, "en")
	if err != nil || stats == nil || stats.Derived["AtkFinal"] != 636 {
		t.Fatalf("strategy-independent stats: %+v %v", stats, err)
	}
}

func TestLegacyNormalizationReportsMissingCoreDefinition(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	inv := nte.Inventory{Cartridges: []nte.Cartridge{{GameItemID: "missing_core", SetID: "unverified"}}}
	if err := s.normalizeInventory(&inv); err == nil || !strings.Contains(err.Error(), `items["missing_core"]`) {
		t.Fatalf("unknown core accepted: %v", err)
	}
}

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
