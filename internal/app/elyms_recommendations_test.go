package app

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func TestElymsRecommendationsResolveBothSetsAndAllPublishedArcs(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	labels, err := service.loadLocale("en")
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := service.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	forks := []string{"fork_elyms", "fork_BitGame", "fork_Time", "fork_BlastCandy", "fork_worldrain", "fork_wuhuakuang"}
	state := decoded.State{Characters: []decoded.Character{{CharacterID: 1090, Level: 80, BreakthroughLevel: 6, Skills: []decoded.CharacterSkill{
		{AbilityID: "GA_Elyms_Melee", Level: 8}, {AbilityID: "GA_Elyms_Skill", Level: 8}, {AbilityID: "GA_Elyms_UltraSkill", Level: 8}, {AbilityID: "GA_Elyms_QTE", Level: 6},
	}}}}
	for i, id := range forks {
		star := 1
		if i == 5 {
			star = 5
		}
		state.Weapons = append(state.Weapons, decoded.Weapon{ID: decoded.NetID{Slot: 1, Serial: i + 1}, ForkID: id, Level: 80, Breakthrough: 6, Star: star})
	}
	for id, set := range map[string]string{"elyms": "Suit6", "elyms_speedy_hedgehog": "Suit11"} {
		t.Run(id, func(t *testing.T) {
			if !slices.ContainsFunc(profiles, func(p ProfileSummary) bool {
				return p.ID == id && p.CharacterID == 1090 && p.GridID == "character_1090" && len(p.PreferredSets) == 1 && p.PreferredSets[0].SetID == set
			}) {
				t.Fatal("Elyms variant not exposed")
			}
			target, err := service.Target(id)
			if err != nil || target.ID != "elyms" || len(target.Arcs) != 6 {
				t.Fatalf("Elyms recommendations: %+v %v", target, err)
			}
			for i, arc := range target.Arcs {
				if !strings.EqualFold(arc.Name, cleanArcName(labels.ItemName(forks[i], ""))) || arc.Rank != i+1 {
					t.Fatalf("Arc recommendation does not resolve: %+v", arc)
				}
			}
			request, err := PrepareSavedOptimization(service, t.TempDir(), "1090", id)
			if err != nil || request.Profile.ID != id || request.Goals["CritBase"].Target != .9 {
				t.Fatalf("Elyms defaults: %+v %v", request, err)
			}
			inventory := loadSanitizedInventoryFixture(t)
			for i := range inventory.Modules {
				inventory.Modules[i].EquippedCharacterID = 0
			}
			inventory.Cartridges = []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: set, Level: 20, MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .6, Percent: true}}}}
			// This smoke test validates profile/set/Arc plumbing, not whether a
			// small synthetic inventory meets the published endgame stat targets.
			goals := map[string]float64{}
			for _, goal := range target.Goals {
				goals[goal.PropertyID] = 0
			}
			for _, fork := range forks {
				result, err := service.optimizeTunedWithArc(context.Background(), inventory, id, &state, false, "en", "fast", goals, nil, fork)
				if err != nil || result.Character == nil || result.Character.CharacterID != 1090 || result.Weapon == nil || result.Weapon.ForkID != fork || result.Cartridge == nil || result.Cartridge.SetID != set || result.Damage == nil || len(result.Damage.Build) != 35 {
					t.Fatalf("search %s / %s: %v", id, fork, err)
				}
			}
		})
	}
}
