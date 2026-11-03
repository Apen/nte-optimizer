package app

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

func TestBuilderAppliesSelectedTeamToSearchAndComparison(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	bitgame := decoded.NetID{Slot: 1, Serial: 1}
	thief := decoded.NetID{Slot: 2, Serial: 2}
	state := decoded.State{
		Characters: []decoded.Character{
			{CharacterID: 1036, Level: 80, BreakthroughLevel: 6},
			{CharacterID: 1003}, {CharacterID: 1023}, {CharacterID: 1020, ForkNetID: &bitgame},
		},
		Weapons: []decoded.Weapon{{ID: bitgame, ForkID: "fork_BitGame", Star: 1}, {ID: thief, ForkID: "fork_tuansanlang", Star: 1}},
	}
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	profiles, err := service.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := service.loadOptimizerCatalog()
	if err != nil {
		t.Fatal(err)
	}
	setID := catalog.sets.Definitions[profiles["zankou"].PreferredSets[0].SetID].InventorySetID
	inv := loadSanitizedInventoryFixture(t)
	for index := range inv.Modules {
		inv.Modules[index].SetID = setID
		inv.Modules[index].EquippedCharacterID = 1036
	}
	inv.Cartridges = []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: setID, Level: 20, EquippedCharacterID: 1036,
		MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .6, Percent: true}}}}
	if err := writeAccountData(dir, inv, state); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveLocalBuild(dir, "synthetic-ally", 1023, nil, "", nil, thief.Key()); err != nil {
		t.Fatal(err)
	}
	workspace, err := LoadBuildWorkspace(dir, service.DataDir, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, character := range workspace.Characters {
		if character.CharacterID == 1023 && (!character.TeamGear.Saved || character.TeamGear.ArcID != thief.Key()) {
			t.Fatal("saved ally gear missing")
		}
		if character.CharacterID == 1020 && (character.TeamGear.Saved || character.TeamGear.ArcID != bitgame.Key()) {
			t.Fatal("scanned ally gear missing")
		}
	}
	service.WeightOverrides = &WeightOverrides{MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"AtkUp": 5}, TeamAllies: []TeamAlly{{CharacterID: 1003}, {CharacterID: 1023}, {CharacterID: 1020}}}
	preview, err := service.PreviewTeam(dir, "zankou", "en", NoArcForkSelection, service.WeightOverrides.TeamAllies)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Applied) != 2 || len(preview.Conditional) != 3 || preview.Conditional[0].Detail != "sakiri_skill" {
		t.Fatalf("unexpected team preview: %+v", preview)
	}
	for _, mode := range []string{"fast", "beta"} {
		result, err := service.OptimizeFlexibleProjectWithArc(context.Background(), dir, "zankou", true, "en", mode, nil, NoArcForkSelection)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.TeamAllies) != 3 || result.CurrentStats == nil || len(result.TeamWarnings) != 0 {
			t.Fatalf("missing team comparison in %s", mode)
		}
		if _, err := SaveOptimizationResult(dir, result); err != nil {
			t.Fatal(err)
		}
		saved, err := SavedOptimizationResult(dir, 1036)
		if err != nil {
			t.Fatal(err)
		}
		if len(saved.TeamAllies) != 3 || saved.Stats.Sources["team"]["AtkUp"] != .1 {
			t.Fatal("saved result lost the team scenario")
		}
		if len(saved.BuffEffects) != len(preview.Conditional) || len(saved.BuffEffects[0].SourceNames) != 1 {
			t.Fatal("saved result lost its buff details")
		}
		for _, sources := range []map[string]map[string]float64{result.Stats.Sources, result.CurrentStats.Sources} {
			if sources["team"]["AtkUp"] != .1 || sources["team"]["UnbalIntensityBase"] != 70 {
				t.Fatalf("unexpected team sources in %s: %v", mode, sources["team"])
			}
		}
	}
	service.WeightOverrides.TeamAllies[0].CharacterID = 1023
	if _, err := service.OptimizeFlexibleProjectWithArc(context.Background(), dir, "zankou", true, "en", "fast", nil, NoArcForkSelection); err == nil {
		t.Fatal("accepted duplicate ally")
	}
}

func TestTeamPreviewCalculatesBaseAttackAndGroupsNonStackingSets(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	catalog, err := service.loadOptimizerCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var geometries []string
	for _, set := range catalog.sets.Definitions {
		if set.InventorySetID == "Suit11" {
			geometries = set.RequiredGeometries
			break
		}
	}
	if len(geometries) < 4 {
		t.Fatal("missing set geometry requirements")
	}
	arcID := decoded.NetID{Slot: 3, Serial: 1}
	state := decoded.State{
		Characters: []decoded.Character{
			{CharacterID: 1036, Level: 80, BreakthroughLevel: 6},
			{CharacterID: 1075, Level: 70, BreakthroughLevel: 5, ForkNetID: &arcID}, {CharacterID: 1042},
			{CharacterID: 1003, Level: 70, BreakthroughLevel: 5},
			{CharacterID: 1020, Level: 70, BreakthroughLevel: 5, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Haniel_Skill", Level: 9}}},
		},
		Weapons: []decoded.Weapon{{ID: arcID, ForkID: "fork_Door", Level: 70, Breakthrough: 5, Star: 1}},
	}
	var inventory nte.Inventory
	for _, id := range []int{1075, 1003} {
		inventory.Cartridges = append(inventory.Cartridges, nte.Cartridge{LocalID: fmt.Sprintf("cartridge-%d", id), SetID: "Suit11", EquippedCharacterID: id})
		for index, geometry := range geometries[:4] {
			inventory.Modules = append(inventory.Modules, nte.Module{LocalID: fmt.Sprintf("module-%d-%d", id, index), SetID: "Suit11", Geometry: geometry, EquippedCharacterID: id, SubStats: []nte.Stat{{PropertyID: "AtkAdd", Value: 100}}})
		}
	}
	if err := writeAccountData(dir, inventory, state); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewTeam(dir, "zankou", "en", NoArcForkSelection, []TeamAlly{{CharacterID: 1075}, {CharacterID: 1003}, {CharacterID: 1020}})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, effect := range preview.Conditional {
		counts[effect.ID]++
		switch effect.ID {
		case "GA_Oneiroi_Skill":
			if effect.Bonus == nil || math.Abs(effect.Bonus.Value-203.6) > 1e-9 || effect.Duration != 20 || effect.Trigger != "redirect_press" {
				t.Fatalf("unexpected Iroi effect: %+v", effect)
			}
		case "Suit11":
			if !effect.NonStacking || !slices.Equal(effect.SourceIDs, []int{1075, 1003}) {
				t.Fatalf("unexpected set sources: %+v", effect)
			}
		case "GA_Sagiri_UltraSkill", "GA_Haniel_Skill":
			id := effect.CharacterID
			ratio := .3
			if id == 1020 {
				ratio = .18
			}
			base, err := service.requiredCharacterBaseStats(id, 70, 5)
			if err != nil {
				t.Fatal(err)
			}
			if effect.Bonus == nil || math.Abs(effect.Bonus.Value-base["AtkBase"]*ratio) > 1e-9 {
				t.Fatalf("unexpected scaled bonus: %+v", effect)
			}
		}
	}
	for _, id := range []string{"GA_Oneiroi_Skill", "Suit11", "GA_Sagiri_UltraSkill", "GA_Haniel_Skill"} {
		if counts[id] != 1 {
			t.Fatalf("expected one row for %s, got %d", id, counts[id])
		}
	}
	withoutSet := optimizer.StatSummary{
		WithConditions: map[string]float64{"AtkBase": 1000},
		Derived:        map[string]float64{"AtkFinal": 1000},
	}
	projectTeamBuffs(&withoutSet, preview.Conditional, optimizer.SetDefinition{}, 0)
	withSet := optimizer.StatSummary{
		WithConditions: map[string]float64{"AtkBase": 1000, "AtkUp": .15},
		Derived:        map[string]float64{"AtkFinal": 1000},
	}
	projectTeamBuffs(&withSet, preview.Conditional, optimizer.SetDefinition{InventorySetID: "Suit11"}, 4)
	if withoutSet.DerivedWithAllBuffs["AtkFinal"] <= 1000 ||
		withoutSet.DerivedWithAllBuffs["AtkFinal"] != withSet.DerivedWithAllBuffs["AtkFinal"] ||
		withoutSet.DerivedWithAllBuffs["DamageUpGeneralBase"] != .15 {
		t.Fatalf("unexpected team projections without set %+v and with set %+v", withoutSet, withSet)
	}
	if withSet.Derived["AtkFinal"] != 1000 || withSet.WithConditions["AtkUp"] != .15 {
		t.Fatal("projection modified the stats used for the normal build")
	}
	blackbird, err := service.PreviewTeam(dir, "zankou", "en", NoArcForkSelection, []TeamAlly{{CharacterID: 1042}})
	if err != nil {
		t.Fatal(err)
	}
	if len(blackbird.Conditional) != 1 {
		t.Fatalf("expected Blackbird's ultimate, got %+v", blackbird.Conditional)
	}
	effect := blackbird.Conditional[0]
	if effect.Bonus == nil || *effect.Bonus != (nte.Stat{PropertyID: "AtkUp", Value: .2, Percent: true}) || effect.Duration != 15 || effect.Trigger != "ultimate" {
		t.Fatalf("unexpected Blackbird effect: %+v", effect)
	}
}
