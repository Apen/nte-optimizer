package app

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
)

func TestAllyBuffSummaryUsesSelectedTeammatesLevelsAndUnlocks(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	labels, err := ntelocale.Load(service.DataDir, "fr")
	if err != nil {
		t.Fatal(err)
	}
	state := decoded.State{Characters: []decoded.Character{
		{CharacterID: 1036, Level: 80, AwakenLevel: 6, BreakthroughLevel: 6},
		{CharacterID: 1020, Level: 70, BreakthroughLevel: 5, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Haniel_Skill", Level: 9}}},
		{CharacterID: 1042, Level: 70, BreakthroughLevel: 5, AwakenLevel: 3},
		{CharacterID: 1039, Level: 70, BreakthroughLevel: 5},
	}}
	scenario := &teamScenario{allies: []TeamAlly{{CharacterID: 1020}, {CharacterID: 1042}}, gear: map[int]teamGear{}}
	summaries, err := service.allyBuffSummaries(&state, scenario, labels)
	if err != nil {
		t.Fatal(err)
	}
	foundSkill, foundMissingUltimate := false, false
	base, err := service.requiredCharacterBaseStats(1020, 70, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range summaries {
		if b.CharacterID != 1020 && b.CharacterID != 1042 {
			t.Fatalf("unselected source leaked into summary: %+v", b)
		}
		if b.CharacterName == "" || b.Condition == "" || b.Included {
			t.Fatalf("missing labels or activated potential effect: %+v", b)
		}
		switch b.ID {
		case "haniel_skill_attack":
			foundSkill = true
			if b.Value == nil || math.Abs(*b.Value-base["AtkBase"]*.18) > 1e-8 || b.Percent || b.PropertyID != "AtkAdd" {
				t.Fatalf("wrong flat attack or imported skill level: %+v", b)
			}
		case "haniel_ultimate_attack":
			foundMissingUltimate = true
			if b.Value != nil {
				t.Fatal("invented missing ultimate level")
			}
		case "blackbird_ultimate_attack_awakening4", "blackbird_ultimate_damage_awakening4":
			t.Fatal("locked awakening buff shown")
		}
	}
	if !foundSkill || !foundMissingUltimate {
		t.Fatal("missing supported or unresolved team buff")
	}
	state.Characters[2].AwakenLevel = 4
	summaries, err = service.allyBuffSummaries(&state, scenario, labels)
	if err != nil {
		t.Fatal(err)
	}
	upgraded := false
	for _, b := range summaries {
		if b.ID == "blackbird_ultimate_attack_awakening4" {
			upgraded = true
			if b.Value == nil || !b.Percent || b.Included {
				t.Fatalf("awakening variant projected automatically: %+v", b)
			}
		}
	}
	if !upgraded {
		t.Fatal("unlocked awakening variant absent")
	}
}

func TestAllyBuffSummaryUsesResolvedArcBaseAttack(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	character := decoded.Character{CharacterID: 1075, Level: 70, BreakthroughLevel: 5}
	bare, err := service.allyBuffActor(character, nil)
	if err != nil {
		t.Fatal(err)
	}
	arc := decoded.Weapon{ForkID: "fork_Door", Level: 70, Breakthrough: 5, Star: 1}
	withArc, err := service.allyBuffActor(character, &arc)
	if err != nil {
		t.Fatal(err)
	}
	if withArc.Stats["AtkBase"] <= bare.Stats["AtkBase"] {
		t.Fatal("selected ally Arc base attack was ignored")
	}
	again, err := service.allyBuffActor(character, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bare, again) {
		t.Fatal("cached character stats mutated by Arc preview")
	}
}

func TestAllyBuffProjectionMatchingRequiresExactSourcePropertyUnitsAndValue(t *testing.T) {
	value := 300.0
	rows := []AllyBuffSummary{{ID: "buff", CharacterID: 1003, PropertyID: "AtkAdd", Value: &value, abilityID: "ultimate"}}
	effects := []TeamPreviewEffect{{ID: "ultimate", Kind: "skill", CharacterID: 1003, Included: true, Bonus: &nte.Stat{PropertyID: "AtkAdd", Value: 300}}}
	matched := matchAllyBuffProjections(rows, effects)
	if !matched[0].Included || matched[0].LegacyEffectID != "ultimate" || rows[0].Included {
		t.Fatal("projection match absent or mutated source")
	}
	for _, wrong := range []TeamPreviewEffect{
		{ID: "ultimate", Kind: "skill", CharacterID: 1020, Included: true, Bonus: effects[0].Bonus},
		{ID: "ultimate", Kind: "skill", CharacterID: 1003, Included: true, Bonus: &nte.Stat{PropertyID: "AtkAdd", Value: 400}},
		{ID: "ultimate", Kind: "skill", CharacterID: 1003, Included: true, Bonus: &nte.Stat{PropertyID: "AtkAdd", Value: 300, Percent: true}},
		{ID: "ultimate", Kind: "skill", CharacterID: 1003, Included: true, Bonus: &nte.Stat{PropertyID: "HPMaxAdd", Value: 300}},
	} {
		if got := matchAllyBuffProjections(rows, []TeamPreviewEffect{wrong}); got[0].Included || got[0].LegacyEffectID != "" {
			t.Fatal("different contribution marked as already included")
		}
	}
}

func TestAllRosterAllySummariesLoadWithoutCasterOnlyEffects(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	labels, err := ntelocale.Load(service.DataDir, "en")
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := service.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, profile := range profiles {
		if seen[profile.CharacterID] {
			continue
		}
		seen[profile.CharacterID] = true
		character := decoded.Character{CharacterID: profile.CharacterID, Level: 80, BreakthroughLevel: 6, AwakenLevel: 6}
		state := decoded.State{Characters: []decoded.Character{character}}
		scenario := &teamScenario{allies: []TeamAlly{{CharacterID: character.CharacterID}}, gear: map[int]teamGear{}}
		rows, err := service.allyBuffSummaries(&state, scenario, labels)
		if err != nil {
			t.Fatalf("%d: %v", character.CharacterID, err)
		}
		for _, row := range rows {
			if row.Condition == "" || labels.Stats[row.PropertyID] == "" {
				t.Fatalf("missing user-facing label: %+v", row)
			}
		}
	}
}
