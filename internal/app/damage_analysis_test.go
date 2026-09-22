package app

import (
	"os"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/optimizer"
)

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
	groups := groupDamageResults(build, current)
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
