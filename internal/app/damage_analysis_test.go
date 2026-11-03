package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/optimizer"
)

func TestDamageBatchMatchesIndependentCalculation(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	c := &decoded.Character{CharacterID: 1036, Level: 80, AwakenLevel: 6, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_Skill", Level: 8}, {AbilityID: "GA_Zankou_UltraSkill", Level: 8}}}
	current := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkFinal": 1200, "CritBase": .5, "CritDamageBase": 1, "MagBase": 100}}
	batch := s.prepareDamageBatch(c, &current, "en")
	for _, attack := range []float64{800, 1600, 2400} {
		build := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkFinal": attack, "CritBase": .6, "CritDamageBase": 1.2, "MagBase": 200}}
		got := batch.evaluate(build)
		want := s.damageAnalysis(c, build, &current)
		if !reflect.DeepEqual(got.Build, want.Build) || !reflect.DeepEqual(got.Current, want.Current) {
			t.Fatal("batch changed damage or equipped baseline")
		}
	}
}

func TestDamageBatchProjectsBuffsForBothBuilds(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	character := &decoded.Character{CharacterID: 1036, Level: 80, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_Skill", Level: 8}}}
	current := optimizer.StatSummary{
		Derived:             map[string]float64{"AtkFinal": 1000},
		DerivedConditional:  map[string]float64{"AtkFinal": 1200},
		DerivedWithAllBuffs: map[string]float64{"AtkFinal": 1600},
	}
	build := optimizer.StatSummary{
		Derived:             map[string]float64{"AtkFinal": 1000},
		DerivedConditional:  map[string]float64{"AtkFinal": 1300},
		DerivedWithAllBuffs: map[string]float64{"AtkFinal": 2000},
	}
	result := s.prepareDamageBatch(character, &current, "en").evaluate(build)
	for _, group := range result.Groups {
		if group.Category == "dot" || group.Category == "reaction" || group.BuildDamage <= 0 {
			continue
		}
		if group.BuildDamage != group.CurrentDamage || group.Buffed == nil ||
			group.Buffed.Build <= group.Buffed.Current || group.Buffed.Current <= group.CurrentDamage {
			t.Fatalf("unexpected normal and buffed damage: %+v, buffs %+v", group, group.Buffed)
		}
		return
	}
	t.Fatal("missing direct damage comparison")
}

func BenchmarkDamageReportingBatch(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s := NewOptimizerService(filepath.Join("..", "..", "data"))
			c := &decoded.Character{CharacterID: 1036, Level: 80, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_Skill", Level: 8}}}
			stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkFinal": 1200}}
			b.ReportAllocs()
			for b.Loop() {
				batch := s.prepareDamageBatch(c, &stats, "en")
				for i := 0; i < count; i++ {
					batch.evaluate(stats)
				}
			}
		})
	}
}

func TestZankouActionsUseSharedImportedSkillLevels(t *testing.T) {
	s := NewOptimizerService(filepath.Join("..", "..", "data"))
	character := &decoded.Character{CharacterID: 1036, Level: 80, AwakenLevel: 1, Skills: []decoded.CharacterSkill{
		{AbilityID: "GA_Zankou_Melee", Level: 8},
		{AbilityID: "GA_Zankou_UltraSkill", Level: 10},
	}}
	current := optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1500}}
	build := optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1600}}
	analysis := s.prepareDamageBatch(character, &current, "en").evaluate(build)
	for _, id := range []string{
		"nightmare-waltz", "moonfall", "bloodfeast", "heartwrench", "vile-ash",
		"inferno-enhanced", "broken-twigs", "voidstep",
	} {
		var comparable bool
		for _, group := range analysis.Groups {
			if group.ID == id {
				comparable = group.BuildDamage > group.CurrentDamage && group.CurrentDamage > 0
			}
		}
		if !comparable {
			t.Errorf("expected comparable build and equipped damage for %s", id)
		}
	}
	factors := map[string]damage.Result{}
	for _, result := range analysis.Build {
		factors[result.InstanceID] = result
	}
	for _, id := range []string{
		"GE_Player_Zankou_Melee1_Damage", "GE_Player_Zankou_Melee1_1_Damage",
		"GE_Player_Zankou_ForceUltraSkill1_Damage", "GE_Player_Zankou_ForceUltraSkill2_Damage",
	} {
		if _, ok := factors[id]; !ok {
			t.Errorf("expected damage impact %s", id)
		}
	}
	if got := factors["GE_Player_Zankou_MagicMelee2_1_Damage"].Factors.Coefficient; got != .406 {
		t.Errorf("expected level 8 Illusion basic coefficient, got %g", got)
	}
	if got := factors["GE_Player_Zankou_MagicUltraSkill1_Damage"].Factors.Coefficient; got != 1.205 {
		t.Errorf("expected level 10 Illusion ultimate coefficient, got %g", got)
	}
	if got := factors["GE_Player_Zankou_DotDamage"]; got.Category != damage.CategoryDOT || got.Factors.CritDamage != .5 {
		t.Errorf("expected Delusion bonus on the Illusion DoT, got %+v", got)
	}
}

func TestShinkuAndBlackbirdActionHitCounts(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	for _, tc := range []struct {
		id     int
		prefix string
		hits   map[string]int
	}{
		{1076, "GA_Shinku_", map[string]int{
			"Melee": 10, "Melee_Rage": 12, "Skill_Rage": 2,
			"UltraSkill": 9, "UltraSkill_Judgment": 4, "UltraSkill_Verdict": 11,
		}},
		{1042, "GA_BlackBird_", map[string]int{
			"Melee": 17, "Evolve_Melee": 20, "Skill": 4, "SkillAssist": 1,
			"Evolve_Skill": 4, "Evolve_Branch1": 5, "Evolve_Branch2": 5,
			"UltraSkill": 9, "Evolve_UltraSkill": 7,
		}},
	} {
		t.Run(tc.prefix, func(t *testing.T) {
			character := &decoded.Character{CharacterID: tc.id, Level: 80}
			for _, skill := range []string{"Melee", "Skill", "UltraSkill", "QTE"} {
				character.Skills = append(character.Skills, decoded.CharacterSkill{AbilityID: tc.prefix + skill, Level: 9})
			}
			current := optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1000}}
			build := optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1500}}
			analysis := service.prepareDamageBatch(character, &current, "en").evaluate(build)
			groups := map[string]DamageGroup{}
			for _, group := range analysis.Groups {
				groups[group.ID] = group
			}
			for skill, hits := range tc.hits {
				group := groups[tc.prefix+skill]
				if group.Instances != hits || group.CurrentDamage <= 0 || group.BuildDamage <= group.CurrentDamage {
					t.Errorf("%s: expected %d comparable hits, got %+v", skill, hits, group)
				}
			}
			for id, name := range map[string]string{
				"GA_Shinku_UltraSkill_Judgment": "Crimson Judgment",
				"GA_Shinku_UltraSkill_Verdict":  "Dragonflame Verdict",
				"GA_Shinku_Melee_Rage":          "Tidal Assault",
				"GA_BlackBird_Evolve_Branch1":   "Bestowed Grace",
			} {
				if strings.HasPrefix(id, tc.prefix) && groups[id].Name != name {
					t.Errorf("%s: expected action name %q, got %q", id, name, groups[id].Name)
				}
			}
			for _, impact := range analysis.Build {
				if impact.Hits > 1 && impact.Total != float64(impact.Hits)*impact.Expected {
					t.Errorf("%s: repeated hits were not included in the action total", impact.InstanceID)
				}
			}
		})
	}
}

func TestZeroStandardSkillOmitsConditionalKillTiers(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	for _, tc := range []struct {
		id      int
		variant string
	}{
		{1046, "Female046"},
		{1051, "Female051"},
	} {
		t.Run(tc.variant, func(t *testing.T) {
			ability := "GA_" + tc.variant + "_Skill"
			character := &decoded.Character{CharacterID: tc.id, Level: 80, Skills: []decoded.CharacterSkill{
				{AbilityID: ability, Level: 6},
			}}
			stats := optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1000}}
			analysis := service.prepareDamageBatch(character, &stats, "en").evaluate(stats)
			var expected float64
			var standard, conditional int
			for _, impact := range analysis.Build {
				if impact.SkillID != ability {
					continue
				}
				if strings.Contains(impact.InstanceID, "_Skill_Kill_Damage_lv") {
					conditional++
					if !impact.OmitFromActionPreview {
						t.Errorf("conditional effect %s entered the standard preview", impact.InstanceID)
					}
					continue
				}
				expected += impact.Total
				standard += impact.Hits
			}
			if conditional != 2 || standard == 0 {
				t.Fatalf("expected two diagnostic effects and a standard skill, got %d and %d", conditional, standard)
			}
			for _, group := range analysis.Groups {
				if group.ID == ability && group.Instances == standard && group.BuildDamage == expected && group.CurrentDamage == expected {
					return
				}
			}
			t.Errorf("standard skill should include only %d hits totaling %g for each build", standard, expected)
		})
	}
}

func TestLinkoSubUltimateBuffScenarios(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	character := &decoded.Character{CharacterID: 1072, Level: 80, Skills: []decoded.CharacterSkill{
		{AbilityID: "GA_Radio072_UltraSkill", Level: 6},
	}}
	stats := optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1000}}
	analysis := service.prepareDamageBatch(character, &stats, "en").evaluate(stats)
	const buff = "linko_sub_ultimate_level2"
	for _, tc := range []struct {
		name  string
		buffs map[string]bool
		want  []string
	}{
		{"unknown", nil, []string{"linko_sub_ultimate_base", buff}},
		{"inactive", map[string]bool{buff: false}, []string{"linko_sub_ultimate_base"}},
		{"active", map[string]bool{buff: true}, []string{buff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]bool{}
			for _, group := range groupDamageResults(character.CharacterID, analysis.Build, analysis.Current, tc.buffs) {
				if group.ActionCondition == nil {
					continue
				}
				if group.ActionCondition.ExclusiveGroup != buff || group.Instances != 1 || group.BuildDamage <= 0 || group.BuildDamage != group.CurrentDamage {
					t.Fatalf("unexpected scenario: %+v", group)
				}
				got[group.ID] = true
			}
			if len(got) != len(tc.want) {
				t.Fatalf("scenarios = %v, want %v", got, tc.want)
			}
			for _, id := range tc.want {
				if !got[id] {
					t.Errorf("missing scenario %s", id)
				}
			}
		})
	}
}

func TestDamageAnalysisUsesImportedSkillLevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "game", "combat"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := `{"schema_version":1,"source":"test","characters":{"1036":{"character_id":1036,"name":"Zankou","instances":[{"id":"dot","ability_id":"GA_Zankou_Melee","name":"DOT","category":"dot","element":"Incantation","scaling":"attack","coefficients":[0.1,0.2],"fixed_crit_rate":0.5,"classification_confidence":"documented","provenance":{"confidence":"game_data","source":"test"}}],"rules":{"instance_modifiers":[{"min_awaken_level":1,"stat_additions":{"general_damage":0.4},"category_stat_additions":{"dot":{"crit_damage":0.5}}}],"derived_instances":[{"id":"reaction_scorch","kind":"scorch","element":"Incantation","magic_property":"MagBase","crit_damage_property":"CritDamageBase","awakening_crit_damage_bonus":{"min_awaken_level":1,"value":0.5}}]}}}}`
	if err := os.WriteFile(filepath.Join(dir, "game", "combat", "damage.json"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	character := &decoded.Character{CharacterID: 1036, Level: 80, AwakenLevel: 1, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_Melee", Level: 2}}}
	stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkFinal": 1000, "CritBase": .9, "CritDamageBase": 2}}
	got := (OptimizerService{DataDir: dir}).damageAnalysis(character, stats, nil)
	if got.Status != "atomic_preview" || len(got.Build) != 2 {
		t.Fatalf("unexpected analysis: %+v", got)
	}
	skill := resultByInstance(t, got.Build, "dot")
	if skill.Factors.Coefficient != .2 || skill.Factors.CritRate != .5 {
		t.Fatalf("skill level or fixed crit was not applied: %+v", skill)
	}
	if skill.Provenance.Confidence != damage.ConfidenceGameData {
		t.Fatalf("missing provenance: %+v", skill.Provenance)
	}
	if skill.Factors.DamageZone != 1.4 || skill.Factors.CritDamage != 2.5 {
		t.Fatalf("Zankou A1 combat bonuses not applied: %+v", skill.Factors)
	}
}

func TestGroupDamageResultsAddsMultiHitActionsAndDOTStacks(t *testing.T) {
	build := []damage.Result{
		{InstanceID: "GE_Player_Zankou_UltraSkill1_Damage", Total: 100, NonCrit: 50, Crit: 150, Hits: 1},
		{InstanceID: "GE_Player_Zankou_UltraSkill2_Damage", Total: 250, NonCrit: 100, Crit: 300, Hits: 1},
		{InstanceID: "GE_Player_Zankou_DotDamage", Total: 500, NonCrit: 200, Crit: 800, Hits: 1},
	}
	current := []damage.Result{
		{InstanceID: "GE_Player_Zankou_UltraSkill1_Damage", Total: 80},
		{InstanceID: "GE_Player_Zankou_UltraSkill2_Damage", Total: 200},
		{InstanceID: "GE_Player_Zankou_DotDamage", Total: 400},
	}
	groups := groupDamageResults(1036, build, current, nil)
	byID := map[string]DamageGroup{}
	for _, group := range groups {
		byID[group.ID] = group
	}
	if got := byID["inferno"]; got.BuildDamage != 350 || got.CurrentDamage != 280 || got.Instances != 2 {
		t.Fatalf("unexpected ultimate group: %+v", got)
	}
	if byID["inferno"].ActionType != "ultimate" || byID["heartwrench"].ActionType != "dot" {
		t.Fatalf("unexpected action types: ultimate=%q dot=%q", byID["inferno"].ActionType, byID["heartwrench"].ActionType)
	}
	if got := byID["heartwrench"]; got.BuildDamage != 500 || got.BuildMaxTick != 5000 || got.MaxStacks != 10 {
		t.Fatalf("unexpected DOT group: %+v", got)
	}
	if got := byID["inferno"]; got.BuildNonCrit != 150 || got.BuildCrit != 450 {
		t.Fatalf("unexpected crit totals: %+v", got)
	}
}

func TestGroupDamageResultsDoesNotApplyZankouLabelsToOtherCharacters(t *testing.T) {
	groups := groupDamageResults(1076, []damage.Result{
		{
			InstanceID: "GE_Player_Shinku_Skill1_1_Damage",
			SkillID:    "GA_Shinku_Skill1",
			Name:       "Shinku skill hit",
			Category:   damage.CategoryDirect,
			Total:      100,
		},
	}, nil, nil)
	if len(groups) != 1 {
		t.Fatalf("expected one Shinku group, got %+v", groups)
	}
	if groups[0].ID != "GA_Shinku_Skill1" || groups[0].Name != "Shinku skill hit" {
		t.Fatalf("unexpected Shinku group: %+v", groups[0])
	}
	if groups[0].Name == "Sanguine Dash" {
		t.Fatal("Shinku skill was mislabeled with a Zankou skill name")
	}
}

func TestLocalizedAbilityLabelHandlesRuntimeAliases(t *testing.T) {
	labels := map[string]string{
		"GA_Chaos_UltraSkill": "Retribution",
		"GA_Mint_Skill":       "Ultimate: Super Claws",
		"GA_Radio072_QTE":     "Spectral Cross",
		"GA_Shinku_Skill1":    "High-Speed Breach",
	}
	for abilityID, want := range map[string]string{
		"GA_Chaos071_UltraSkill":    "Retribution",
		"GA_MintActorWind_Skill":    "Ultimate: Super Claws",
		"GA_Radio072_QTE_BackToLTE": "Spectral Cross",
		"GA_Shinku_Skill_Rage":      "High-Speed Breach",
	} {
		if got := localizedAbilityLabel(labels, abilityID); got != want {
			t.Fatalf("localizedAbilityLabel(%q) = %q, want %q", abilityID, got, want)
		}
	}
}

func TestFallbackActionNameNeverExposesTechnicalInstanceID(t *testing.T) {
	labels := map[string]string{"skill_ultimate": "Ultimate"}
	if got := fallbackActionName(labels, "ultimate"); got != "Ultimate" {
		t.Fatalf("unexpected fallback label %q", got)
	}
}

func TestProductionDamageAbilitiesHaveLocalizedActionLabels(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	catalog, err := damage.ReadCatalog(filepath.Join(dataDir, "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"en", "fr"} {
		labels, err := ntelocale.LoadPresentation(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		for characterID, character := range catalog.Characters {
			seen := map[string]bool{}
			for _, instance := range character.Instances {
				abilityID := instance.AbilityID
				if abilityID == "" || seen[abilityID] || !isNamedCombatAction(abilityID) {
					continue
				}
				seen[abilityID] = true
				if localizedAbilityLabel(labels.Abilities, abilityID) == "" {
					id, _ := strconv.Atoi(characterID)
					t.Errorf("%s character %d ability %s has no localized action label", language, id, abilityID)
				}
			}
		}
	}
}

func isNamedCombatAction(abilityID string) bool {
	for _, marker := range []string{"Melee", "Appear", "Skill", "QTE"} {
		if strings.Contains(abilityID, marker) {
			return true
		}
	}
	return false
}

func TestDamageAnalysisAppliesAwakeningCoefficientOnceToBothBuilds(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "game", "combat"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := `{"schema_version":1,"source":"test","characters":{"1036":{"character_id":1036,"name":"Zankou","instances":[{"id":"GE_Player_Zankou_UltraSkill1_Damage","ability_id":"GA_Zankou_UltraSkill","name":"Ult","category":"direct","element":"Incantation","scaling":"attack","coefficients":[1],"classification_confidence":"documented","provenance":{"confidence":"game_data","source":"test"}}],"rules":{"instance_modifiers":[{"min_awaken_level":3,"instance_id_contains":"UltraSkill","coefficient_multiplier":1.2}],"derived_instances":[{"id":"reaction_scorch","kind":"scorch","element":"Incantation","magic_property":"MagBase","crit_damage_property":"CritDamageBase","awakening_crit_damage_bonus":{"min_awaken_level":1,"value":0.5}}]}}}}`
	if err := os.WriteFile(filepath.Join(dir, "game", "combat", "damage.json"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	character := &decoded.Character{CharacterID: 1036, Level: 80, AwakenLevel: 5, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_UltraSkill", Level: 1}}}
	stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkFinal": 1000, "CritDamageBase": 2}}
	got := (OptimizerService{DataDir: dir}).damageAnalysis(character, stats, &stats)
	if len(got.Build) != 2 || len(got.Current) != 2 {
		t.Fatalf("unexpected analysis sizes: build=%d current=%d", len(got.Build), len(got.Current))
	}
	buildUlt := resultByInstance(t, got.Build, "GE_Player_Zankou_UltraSkill1_Damage")
	currentUlt := resultByInstance(t, got.Current, "GE_Player_Zankou_UltraSkill1_Damage")
	if buildUlt.Factors.Coefficient != 1.2 || currentUlt.Factors.Coefficient != 1.2 {
		t.Fatalf("awakening coefficient was compounded: build=%v current=%v", buildUlt.Factors.Coefficient, currentUlt.Factors.Coefficient)
	}
	if buildUlt.Total != currentUlt.Total {
		t.Fatalf("identical stats must produce identical damage: build=%v current=%v", buildUlt.Total, currentUlt.Total)
	}
}

func resultByInstance(t *testing.T, results []damage.Result, instanceID string) damage.Result {
	t.Helper()
	for _, result := range results {
		if result.InstanceID == instanceID {
			return result
		}
	}
	t.Fatalf("missing damage result %s", instanceID)
	return damage.Result{}
}
