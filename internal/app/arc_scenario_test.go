package app

import (
	"context"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

func TestBuilderAppliesConfirmedElymsArcConditionsAtTheEquippedRank(t *testing.T) {
	const activeID, elymsID, iroiID = 1036, 1090, 1075
	echoes, gate := decoded.NetID{Slot: 1, Serial: 1}, decoded.NetID{Slot: 2, Serial: 1}
	state := decoded.State{
		Characters: []decoded.Character{
			{CharacterID: activeID, Level: 80, BreakthroughLevel: 6,
				Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_Melee", Level: 10}}},
			{CharacterID: elymsID, Level: 70, BreakthroughLevel: 5, ForkNetID: &echoes},
			{CharacterID: iroiID, Level: 70, BreakthroughLevel: 5, ForkNetID: &gate},
		},
		Weapons: []decoded.Weapon{
			{ID: echoes, ForkID: "fork_elyms", Level: 70, Breakthrough: 5, Star: 3},
			{ID: gate, ForkID: "fork_Door", Level: 70, Breakthrough: 5, Star: 1},
		},
	}
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	service.WeightOverrides = &WeightOverrides{
		MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"DamageUpGeneralBase": 1},
		TeamAllies: []TeamAlly{{CharacterID: elymsID}, {CharacterID: iroiID}},
	}
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
		inv.Modules[index].EquippedCharacterID = activeID
	}
	inv.Cartridges = []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: setID, Level: 20, EquippedCharacterID: activeID,
		MainStats: []nte.Stat{{PropertyID: "CritDamageBase", Value: .6, Percent: true}}}}
	goals := map[string]GoalTuning{
		"DamageUpGeneralBase": {Target: .2, Custom: true, Label: "General DMG", Percent: true},
	}
	defense := ArcConditionSelection{ArcConditionKey: ArcConditionKey{CharacterID: elymsID, ForkID: "fork_elyms", Trigger: "three_followup_hits_after_ultimate_or_enter_combat"}, Stacks: 1}
	penetration := ArcConditionSelection{ArcConditionKey: ArcConditionKey{CharacterID: elymsID, ForkID: "fork_elyms", Trigger: "redirect_skill_or_enter_combat"}, Stacks: 1}
	healing := ArcConditionSelection{ArcConditionKey: ArcConditionKey{CharacterID: iroiID, ForkID: "fork_Door", Trigger: "healing"}, Stacks: 1}
	var baselineDamage float64
	for _, tc := range []struct {
		name                                     string
		conditions                               []ArcConditionSelection
		wantDefense, wantPenetration, wantDamage float64
	}{
		{name: "inactive"},
		{name: "follow-ups", conditions: []ArcConditionSelection{defense}, wantDefense: .16},
		{name: "redirect", conditions: []ArcConditionSelection{penetration}, wantPenetration: .14},
		{name: "both and heal", conditions: []ArcConditionSelection{defense, penetration, healing}, wantDefense: .16, wantPenetration: .14, wantDamage: .15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service.WeightOverrides.ArcConditions = tc.conditions
			result, err := service.optimizeTunedWithArc(context.Background(), inv, "zankou", &state, true, "en", "fast", nil, goals, NoArcForkSelection)
			if err != nil {
				t.Fatal(err)
			}
			if result.CurrentStats == nil {
				t.Fatal("equipped build is missing")
			}
			for _, stats := range []optimizer.StatSummary{result.Stats, *result.CurrentStats} {
				for label, values := range map[string]map[string]float64{
					"ranking": stats.RankingStats(), "buffed": stats.DerivedWithAllBuffs,
				} {
					if values["DefenseIgnoreBase"] != tc.wantDefense || values["ResistanceIgnoreCosmosBase"] != tc.wantPenetration || values["DamageUpGeneralBase"] != tc.wantDamage {
						t.Fatalf("%s: expected exact confirmed Arc bonuses, got %v", label, values)
					}
				}
				if stats.Derived["DefenseIgnoreBase"] != 0 || stats.Derived["ResistanceIgnoreCosmosBase"] != 0 || stats.Derived["DamageUpGeneralBase"] != 0 {
					t.Fatal("conditional bonus entered the unbuffed panel")
				}
			}
			if result.Solution.Ranking == nil || !slices.ContainsFunc(result.Solution.Ranking.Contributions, func(contribution optimizer.RankingContribution) bool {
				return contribution.PropertyID == "DamageUpGeneralBase" && contribution.Value == tc.wantDamage
			}) {
				t.Fatalf("reported ranking did not use the confirmed Arc damage bonus: %+v", result.Solution.Ranking)
			}
			if result.Damage == nil || len(result.Damage.Groups) == 0 {
				t.Fatal("missing damage preview")
			}
			for _, group := range result.Damage.Groups {
				if group.Buffed == nil || group.BuildDamage == 0 {
					continue
				}
				if len(tc.conditions) == 0 {
					baselineDamage = group.Buffed.Build
					return
				}
				changed := group.Buffed.Build > baselineDamage
				if changed != (tc.wantDefense > 0 || tc.wantDamage > 0) {
					t.Fatalf("unexpected change to non-Cosmos hit %s: %g to %g", group.ID, baselineDamage, group.Buffed.Build)
				}
				return
			}
			t.Fatal("no usable damage group")
		})
	}
}

func TestElymsArcWearerBonusRequiresConfirmationAndIsCountedOnce(t *testing.T) {
	const elymsID = 1090
	echoes := decoded.NetID{Slot: 1, Serial: 1}
	weapon := decoded.Weapon{ID: echoes, ForkID: "fork_elyms", Level: 70, Breakthrough: 5, Star: 1}
	state := decoded.State{Characters: []decoded.Character{{CharacterID: elymsID, Level: 80, BreakthroughLevel: 6, ForkNetID: &echoes}},
		Weapons: []decoded.Weapon{weapon}}
	inv := nte.Inventory{
		Modules: []nte.Module{{LocalID: "synthetic-module", Geometry: "H_2", Area: 2, Level: 20, EquippedCharacterID: elymsID}},
		Cartridges: []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: "Suit2", Level: 20, EquippedCharacterID: elymsID,
			MainStats: []nte.Stat{{PropertyID: "CritBase", Value: .3, Percent: true}}}},
	}
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	service.WeightOverrides = &WeightOverrides{MainStats: []string{"CritBase"}, Weights: map[string]float64{"CritBase": 1}}
	workspace := t.TempDir()
	if err := ensureWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if err := writeAccountData(workspace, inv, state); err != nil {
		t.Fatal(err)
	}
	key := ArcConditionKey{CharacterID: elymsID, ForkID: "fork_elyms", Trigger: "three_followup_hits_after_ultimate_or_enter_combat"}
	for _, proposed := range []string{"", NoArcForkSelection} {
		preview, err := service.PreviewTeam(workspace, "elyms", "en", proposed, nil)
		if err != nil || !slices.ContainsFunc(preview.ArcConditions, func(option ArcConditionOption) bool {
			return option.ArcConditionKey == key
		}) {
			t.Fatalf("proposed Arc %q: equipped Arc option missing: %+v (%v)", proposed, preview.ArcConditions, err)
		}
	}
	for _, want := range []float64{0, .22} {
		if want > 0 {
			service.WeightOverrides.ArcConditions = []ArcConditionSelection{{ArcConditionKey: key, Stacks: 1}}
		}
		for _, proposed := range []string{"", NoArcForkSelection} {
			result, err := service.optimizeTunedWithArc(context.Background(), inv, "elyms", &state, true, "en", "score", nil, nil, proposed)
			if err != nil || result.CurrentStats == nil {
				t.Fatalf("proposed Arc %q: equipped build is missing (%v)", proposed, err)
			}
			wantProposed := want
			if proposed == NoArcForkSelection {
				wantProposed = 0
			}
			for _, check := range []struct {
				stats optimizer.StatSummary
				want  float64
			}{{result.Stats, wantProposed}, {*result.CurrentStats, want}} {
				if check.stats.WithConditions["DefIgnore"] != 0 || math.Abs(check.stats.WithConditions["DefenseIgnoreBase"]-check.want) > 1e-9 {
					t.Fatalf("proposed Arc %q: expected %g DEF Ignore exactly once, got %v", proposed, check.want, check.stats.WithConditions)
				}
			}
		}
	}
}

func TestArcDamageTagsRestrictDamageBonusToMatchingHits(t *testing.T) {
	const summon, melee, branch = "State.Damage.Summon", "State.Damage.Melee", "State.Damage.Branch"
	base := damage.TieredInstance{ID: "untagged", AbilityID: "test", Element: "DAMAGE_TYPE_COSMOS", Category: damage.CategoryDirect, Scaling: damage.ScalingAttack, Coefficients: []float64{1}}
	instances := []damage.TieredInstance{base}
	for id, tags := range map[string][]string{
		"summon": {summon}, "melee": {melee}, "branch": {branch},
		"overlap": {melee, branch}, "child_overlap": {melee, branch + ".AirAttack"},
	} {
		hit := base
		hit.ID, hit.DamageTags = id, tags
		instances = append(instances, hit)
	}
	catalog := damage.Catalog{Characters: map[string]damage.CharacterCatalog{"1": {Instances: instances}}}
	character := &decoded.Character{CharacterID: 1, Level: 80, Skills: []decoded.CharacterSkill{{AbilityID: "test", Level: 1}}}
	stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkBase": 100, "AtkFinal": 100}}
	result := damageAnalysisWithArcBuffs(character, stats, nil, catalog, nil, []taggedArcBuff{
		{filter: decoded.ForkDamageFilter{RequiredDamageTags: []string{summon}}, stat: nte.Stat{PropertyID: "DamageUpGeneralBase", Value: .2, Percent: true}},
		{filter: decoded.ForkDamageFilter{AnyDamageTags: []string{melee, branch}}, stat: nte.Stat{PropertyID: "DamageUpGeneralBase", Value: .5, Percent: true}},
	})
	want := map[string]float64{"untagged": 1, "summon": 1.2, "melee": 1.5, "branch": 1.5, "overlap": 1.5, "child_overlap": 1.5}
	if len(result.Build) != len(want) {
		t.Fatalf("unexpected per-hit buffs: %+v", result.Build)
	}
	for _, hit := range result.Build {
		if hit.Factors.DamageZone != want[hit.InstanceID] {
			t.Fatalf("unexpected damage multiplier for %s: %g", hit.InstanceID, hit.Factors.DamageZone)
		}
	}
}

func TestEquippedHitScopedArcBonusesApplyInBothDamagePreviews(t *testing.T) {
	const skill = "State.Damage.Skill"
	eligible := damage.TieredInstance{ID: "eligible", AbilityID: "test", Category: damage.CategoryDirect, Scaling: damage.ScalingAttack, Coefficients: []float64{1}, DamageTags: []string{skill}}
	other := eligible
	other.ID, other.DamageTags = "other", nil
	catalog := damage.Catalog{Characters: map[string]damage.CharacterCatalog{"1": {Instances: []damage.TieredInstance{eligible, other}}}}
	character := &decoded.Character{CharacterID: 1, Level: 80, Skills: []decoded.CharacterSkill{{AbilityID: "test", Level: 1}}}
	stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkBase": 100, "AtkFinal": 100}}
	buff := func(value float64, alwaysActive bool) taggedArcBuff {
		return taggedArcBuff{
			filter:       decoded.ForkDamageFilter{AnyDamageTags: []string{skill}},
			stat:         nte.Stat{PropertyID: "DamageUpGeneralBase", Value: value, Percent: true},
			alwaysActive: alwaysActive,
		}
	}
	batch := damageBatch{
		character: character, catalog: catalog,
		buildArcBuffs: []taggedArcBuff{buff(.2, true), buff(.5, false)},
	}
	batch.setCurrent(&stats, []taggedArcBuff{buff(.1, true), buff(.4, false)})
	report := batch.evaluate(stats)
	for _, tc := range []struct {
		name string
		hits []damage.Result
		want float64
	}{
		{"planned", report.Build, 1.2},
		{"equipped", report.Current, 1.1},
		{"planned with buffs", batch.calculate(stats, true, batch.buildArcBuffs).Build, 1.7},
		{"equipped with buffs", batch.currentBuffed, 1.5},
	} {
		if len(tc.hits) != 2 || tc.hits[0].InstanceID != "eligible" ||
			math.Abs(tc.hits[0].Factors.DamageZone-tc.want) > 1e-9 || tc.hits[1].Factors.DamageZone != 1 {
			t.Errorf("%s damage projection = %+v, want %g on eligible hits only", tc.name, tc.hits, tc.want)
		}
	}
}
