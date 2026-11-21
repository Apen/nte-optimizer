package app

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"nte-optimizer/internal/combatdata"
	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

func TestAwakeningResonancesCoverPublishedCharacterIDs(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	resonances, err := resolveAwakeningResonances(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(datafiles.New(dataDir).DecodeCatalog("characters.json"))
	if err != nil {
		t.Fatal(err)
	}
	var roster struct {
		Characters map[int]json.RawMessage `json:"characters"`
	}
	if err := json.Unmarshal(raw, &roster); err != nil {
		t.Fatal(err)
	}
	for id := range roster.Characters {
		if _, ok := resonances[id]; !ok {
			t.Errorf("character %d has no awakening resonance coverage", id)
		}
	}
}

func TestAwakeningResonanceSkillsBelongToPublishedCharacter(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	resonances, err := readAwakeningResonances(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := combatdata.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for id, ranks := range resonances {
		abilities := combat.Skills.Characters[strconv.Itoa(id)].Abilities
		for _, rank := range ranks {
			for abilityID := range rank.SkillLevels {
				if !slices.ContainsFunc(abilities, func(ability combatdata.Ability) bool {
					return ability.ID == abilityID
				}) {
					t.Errorf("character %d R%d references unknown skill %q", id, rank.RequiredAwakening, abilityID)
				}
			}
		}
	}
}

func TestReadAwakeningResonancesRejectsInvalidBonuses(t *testing.T) {
	for _, test := range []struct {
		name, resonances string
	}{
		{"missing rank", `[{"skill_levels":{"ability":1}}]`},
		{"duplicate rank", `[{"required_awakening":3},{"required_awakening":3}]`},
		{"negative skill increment", `[{"required_awakening":3,"skill_levels":{"ability":-1}}]`},
		{"duplicate effect", `[{"required_awakening":3,"stat_effects":[{"id":"effect"}]},{"required_awakening":6,"stat_effects":[{"id":"effect"}]}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			path := datafiles.New(dataDir).Game("characters", "awakening_resonances.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, path, `{"schema_version":1,"characters":{"1003":`+test.resonances+`}}`)
			if _, err := readAwakeningResonances(dataDir); err == nil {
				t.Fatal("invalid resonance catalog was accepted")
			}
		})
	}
}

func TestResolveAwakeningResonancesRejectsConditionalHanielEffect(t *testing.T) {
	dataDir := t.TempDir()
	layout := datafiles.New(dataDir)
	published := datafiles.New(filepath.Join("..", "..", "data"))
	resonancesPath := layout.Game("characters", "awakening_resonances.json")
	for _, dir := range []string{filepath.Dir(resonancesPath), layout.Game("combat")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"skill_buffs.json", "modifier_values.json"} {
		raw, err := os.ReadFile(published.Game("combat", name))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, layout.Game("combat", name), string(raw))
	}
	for _, test := range []struct {
		name, reference, reason string
	}{
		{"reviewed buff", `{"id":"haniel_awakening3_critical_rate"}`, "invalid permanent awakening resonance effect"},
		{"raw modifier", `{"id":"Buff_Haniel020_Level3_CritDamageUp__Modifiers_0","source":"modifier_values"}`, "use its published buff"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writeTestFile(t, resonancesPath, `{"schema_version":1,"characters":{"1020":[{"required_awakening":3,"stat_effects":[`+test.reference+`]}]}}`)
			if _, err := resolveAwakeningResonances(dataDir); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("expected conditional effect to be rejected, got %v", err)
			}
		})
	}
}

func TestBuilderUsesPersonalAndSelectedTeamResonanceStats(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	state := decoded.State{Characters: []decoded.Character{
		{CharacterID: 1076, Level: 80, BreakthroughLevel: 6, AwakenLevel: 6,
			Skills: []decoded.CharacterSkill{{AbilityID: "GA_Shinku_Melee", Level: 10}}},
		{CharacterID: 1039, Level: 80, BreakthroughLevel: 6, AwakenLevel: 6},
		{CharacterID: 1042, Level: 80, BreakthroughLevel: 6, AwakenLevel: 5},
		{CharacterID: 1052, Level: 80, BreakthroughLevel: 6, AwakenLevel: 6},
	}}
	inv := nte.Inventory{
		Modules: []nte.Module{{LocalID: "synthetic-module", Geometry: "H_2", Area: 2, Level: 20, EquippedCharacterID: 1076}},
		Cartridges: []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: "Suit6", Level: 20,
			MainStats: []nte.Stat{{PropertyID: "CritBase", Value: .3, Percent: true}}}},
	}
	service := NewOptimizerService(dataDir)
	service.WeightOverrides = &WeightOverrides{
		MainStats:  []string{"CritBase"},
		Weights:    map[string]float64{"CritBase": 1},
		TeamAllies: []TeamAlly{{CharacterID: 1039}, {CharacterID: 1042}},
	}
	result, err := service.optimizeTunedWithArc(context.Background(), inv, "shinku", &state, true, "en", "score", nil, nil, NoArcForkSelection)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentStats == nil || result.Character == nil || result.Character.Skills[0].Level != 11 {
		t.Fatalf("missing equipped comparison or projected skill level: %+v", result.Character)
	}
	for _, source := range []map[string]float64{result.Stats.Sources["awakening_resonance"], result.CurrentStats.Sources["awakening_resonance"]} {
		if len(source) != 2 || source["CritBase"] != .1 || source["DefenseIgnoreBase"] != .15 {
			t.Fatalf("unexpected equipped or optimized resonance stats: %v", source)
		}
	}
	for _, source := range []map[string]float64{result.Stats.Sources["team"], result.CurrentStats.Sources["team"]} {
		if len(source) != 1 || source["HPMaxUp"] != .1 {
			t.Fatalf("unexpected equipped or optimized team stats: %v", source)
		}
	}
}

func TestBuilderAppliesNanallyDamageBonusAtR6(t *testing.T) {
	character := decoded.Character{CharacterID: 1010, Level: 80, BreakthroughLevel: 6, AwakenLevel: 5}
	before := optimizeResonanceTestCharacter(t, character, "nanally", "Suit2", "CritBase")
	character.AwakenLevel = 6
	after := optimizeResonanceTestCharacter(t, character, "nanally", "Suit2", "CritBase")
	for _, check := range []struct {
		name          string
		before, after *optimizer.StatSummary
	}{
		{"equipped", before.CurrentStats, after.CurrentStats},
		{"optimized", &before.Stats, &after.Stats},
	} {
		if check.before == nil || check.after == nil {
			t.Fatalf("%s stats are missing", check.name)
		}
		const property = "DamageUpGeneralBase"
		if len(check.before.Sources["awakening_resonance"]) != 0 || check.after.Sources["awakening_resonance"][property] != .15 {
			t.Errorf("%s resonance source at R5/R6 = %v/%v", check.name, check.before.Sources["awakening_resonance"], check.after.Sources["awakening_resonance"])
		}
		if delta := check.after.Derived[property] - check.before.Derived[property]; math.Abs(delta-.15) > 1e-9 {
			t.Errorf("%s damage bonus increased by %g at R6, want .15", check.name, delta)
		}
	}
}

func TestShinkuR6DefenseIgnoreAffectsEquippedAndOptimizedDamage(t *testing.T) {
	character := decoded.Character{CharacterID: 1076, Level: 80, BreakthroughLevel: 6,
		Skills: []decoded.CharacterSkill{{AbilityID: "GA_Shinku_Melee", Level: 10}}}
	for _, check := range []struct {
		rank   int
		ignore float64
	}{{5, 0}, {6, .15}} {
		character.AwakenLevel = check.rank
		result := optimizeResonanceTestCharacter(t, character, "shinku", "Suit6", "CritBase")
		if result.Damage == nil || result.Damage.Status != "atomic_preview" {
			t.Fatalf("R%d damage preview is unavailable: %+v", check.rank, result.Damage)
		}
		want, _ := damage.DefenseMultiplier(character.Level, result.Damage.Enemy, check.ignore, 0)
		for _, preview := range []struct {
			name      string
			instances []damage.Result
		}{{"equipped", result.Damage.Current}, {"optimized", result.Damage.Build}} {
			if len(preview.instances) == 0 {
				t.Fatalf("R%d %s damage preview is empty", check.rank, preview.name)
			}
			for _, hit := range preview.instances {
				if math.Abs(hit.Factors.DefenseZone-want) > 1e-9 {
					t.Errorf("R%d %s %s defense multiplier = %g, want %g", check.rank, preview.name, hit.InstanceID, hit.Factors.DefenseZone, want)
				}
			}
		}
	}
}

func TestBuilderUsesEdgarR3SkillLevelsForDamage(t *testing.T) {
	character := decoded.Character{
		CharacterID: 1021, Level: 80, BreakthroughLevel: 6, AwakenLevel: 2,
		Skills: []decoded.CharacterSkill{
			{AbilityID: "GA_Edgar_Melee", Level: 10},
			{AbilityID: "GA_Edgar_QTE", Level: 10},
			{AbilityID: "GA_Edgar_Skill", Level: 10},
			{AbilityID: "GA_Edgar_UltraSkill", Level: 10},
		},
	}
	for _, test := range []struct {
		rank                         int
		qte, redirectSkill, ultimate float64
	}{
		{2, 3.998, 1.599, 7.996},
		{3, 4.318, 1.727, 8.636},
	} {
		character.AwakenLevel = test.rank
		result := optimizeResonanceTestCharacter(t, character, "edgar", "Suit9", "HPMaxUp")
		if result.Damage == nil || result.Damage.Status != "atomic_preview" {
			t.Fatalf("R%d damage preview is unavailable: %+v", test.rank, result.Damage)
		}
		want := map[string]float64{
			"GE_Player_Edgar_Melee1_Damage":     .618,
			"GE_Player_Edgar_QTE_Damage":        test.qte,
			"GE_Player_Edgar_Skill_Damage":      test.redirectSkill,
			"GE_Player_Edgar_UltraSkill_Damage": test.ultimate,
		}
		for _, preview := range []struct {
			name      string
			instances []damage.Result
		}{{"equipped", result.Damage.Current}, {"optimized", result.Damage.Build}} {
			coefficients := make(map[string]float64, len(preview.instances))
			for _, instance := range preview.instances {
				coefficients[instance.InstanceID] = instance.Factors.Coefficient
			}
			for id, expected := range want {
				if got := coefficients[id]; got != expected {
					t.Errorf("R%d %s %s coefficient = %g, want %g", test.rank, preview.name, id, got, expected)
				}
			}
		}
	}
}

func optimizeResonanceTestCharacter(t *testing.T, character decoded.Character, profile, set, mainStat string) OptimizationResult {
	t.Helper()
	inv := nte.Inventory{
		Modules: []nte.Module{{LocalID: "synthetic-module", Geometry: "H_2", Area: 2, Level: 20, EquippedCharacterID: character.CharacterID}},
		Cartridges: []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: set, Level: 20, EquippedCharacterID: character.CharacterID,
			MainStats: []nte.Stat{{PropertyID: mainStat, Value: .3, Percent: true}}}},
	}
	state := decoded.State{Characters: []decoded.Character{character}}
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	result, err := service.optimizeTunedWithArc(context.Background(), inv, profile, &state, true, "en", "score", nil, nil, NoArcForkSelection)
	if err != nil {
		t.Fatalf("optimize %s at R%d: %v", profile, character.AwakenLevel, err)
	}
	return result
}
