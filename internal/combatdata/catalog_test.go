package combatdata

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/datafiles"
	ntelocale "nte-optimizer/internal/locale"
)

func production(t *testing.T) Catalog {
	t.Helper()
	c, err := Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestPublishedNumericalContractsAndLocaleReferences(t *testing.T) {
	c := production(t)
	dataDir := filepath.Join("..", "..", "data")
	if err := datafiles.New(dataDir).ValidateGameManifest(); err != nil {
		t.Fatal(err)
	}
	cards, lines, resolved, literal, scenario := 0, 0, 0, 0, 0
	for _, character := range c.Skills.Characters {
		for _, card := range character.Abilities {
			cards++
			for _, line := range card.Lines {
				lines++
				switch line.Status {
				case "resolved":
					resolved++
				case "literal":
					literal++
				case "requires_scenario_input":
					scenario++
				default:
					t.Errorf("unknown line status %s", line.Status)
				}
			}
		}
	}
	if len(c.Skills.Characters) != 27 || cards != 150 || lines != 704 || resolved != 681 || literal != 21 || scenario != 2 {
		t.Fatalf("published skill coverage: %d characters, %d cards, %d lines (%d/%d/%d)", len(c.Skills.Characters), cards, lines, resolved, literal, scenario)
	}
	if len(c.Buffs.Effects) != 139 || len(c.Buffs.AttributeExtractions) != 2 || len(c.Modifiers.Entries) != 401 || len(c.Reactions.DamageScenarios) != 6 || len(c.Reactions.SupportScenarios) != 2 || len(c.Reactions.Summons.Profiles) != 2 || len(c.Reactions.Summons.SelectedVariants) != 3 {
		t.Fatal("incomplete numerical catalogs")
	}
	for _, id := range []string{"1056", "1091"} {
		character := c.Skills.Characters[id]
		if character.Status != "no_imported_combat_skills" || len(character.Abilities) != 0 {
			t.Errorf("fabricated skills for %s", id)
		}
	}
	for _, language := range []string{"en", "fr"} {
		labels, err := ntelocale.Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		for _, character := range c.Skills.Characters {
			for _, card := range character.Abilities {
				if _, err := labels.GameLabel(card.Name.Table, card.Name.Key); err != nil {
					t.Error(err)
				}
				for _, line := range card.Lines {
					for _, ref := range []Label{line.Label, line.Format} {
						if _, err := labels.GameLabel(ref.Table, ref.Key); err != nil {
							t.Error(err)
						}
					}
				}
			}
		}
	}
	ids := map[string]bool{}
	for _, b := range c.Buffs.Effects {
		if ids[b.ID] {
			t.Errorf("duplicate buff %s", b.ID)
		}
		ids[b.ID] = true
	}
	for _, modifier := range c.Modifiers.Entries {
		for _, id := range modifier.PublishedIDs {
			if !ids[id] {
				t.Errorf("unknown published buff %s", id)
			}
		}
	}
}

func TestExactAxesAndManualCurve(t *testing.T) {
	for _, level := range []int{-1, 0, 3, 14} {
		if _, err := Exact([]float64{.1, .25}, level); err == nil {
			t.Errorf("clamped invalid level %d", level)
		}
	}
	p := Parameter{Values: []float64{.1, .25}, CharacterValues: []float64{705, 761}, Status: "resolved"}
	if value, err := p.Resolve(1, 2, nil); err != nil || value != 761 {
		t.Fatalf("character axis: %v %v", value, err)
	}
	c := production(t)
	for _, ability := range c.Skills.Characters["1073"].Abilities {
		for _, line := range ability.Lines {
			if line.Status != "requires_scenario_input" {
				continue
			}
			p := line.Parameters[0]
			if _, err := p.Resolve(13, 80, nil); err == nil {
				t.Fatal("guessed economic curve from level")
			}
			for input, want := range map[float64]float64{0: 1500, 1: 1500, 5.9: 1500, 6: 1250, 11: 1000, 16: 800, 21: 600, 26: 500, 100: 500} {
				value, err := p.Resolve(1, 1, map[string]float64{p.Input.ID: input})
				if err != nil || value != want {
					t.Errorf("curve %g: %v %v", input, value, err)
				}
			}
		}
	}
}

func buff(t *testing.T, c Catalog, id string) Buff {
	t.Helper()
	for _, b := range c.Buffs.Effects {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("missing buff %s", id)
	return Buff{}
}
func scenarioByID(t *testing.T, c Catalog, id string) Scenario {
	t.Helper()
	for _, s := range append(c.Reactions.DamageScenarios, c.Reactions.SupportScenarios...) {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("missing scenario %s", id)
	return Scenario{}
}

func TestSelectedBuffUnitsRecipientsUnlocksAndExclusion(t *testing.T) {
	c := production(t)
	actor := Actor{CharacterID: 1003, Awakening: 4, Levels: map[string]int{"GA_Sagiri_UltraSkill": 13}, Stats: map[string]float64{"AtkBase": 1000}}
	choice := BuffSelection{ID: "sagiri_ultimate_attack", Stacks: 1, ConditionsConfirmed: true}
	b := buff(t, c, choice.ID)
	value, err := b.Evaluate(choice, actor, 1010, 1010)
	if err != nil || value.Value != 300 || value.Target.Percent || value.Target.PropertyID != "AtkAdd" || value.Duration == nil || *value.Duration != 20 {
		t.Fatalf("flat attack: %+v %v", value, err)
	}
	if _, err := b.Evaluate(choice, actor, 1003, 1003); err == nil {
		t.Fatal("recipient exclusion ignored")
	}
	choice.ConditionsConfirmed = false
	if _, err := b.Evaluate(choice, actor, 1010, 1010); err == nil {
		t.Fatal("unselected native condition activated")
	}
	b = buff(t, c, "mint_on_field_defense")
	choice = BuffSelection{ID: b.ID, Stacks: 1, ConditionsConfirmed: true}
	actor = Actor{CharacterID: 1019, Breakthrough: 4}
	value, err = b.Evaluate(choice, actor, 1019, 1019)
	if err != nil || value.Value != .2 || !value.Target.Percent {
		t.Fatalf("percent defense: %+v %v", value, err)
	}
	actor.Breakthrough = 3
	if _, err := b.Evaluate(choice, actor, 1019, 1019); err == nil {
		t.Fatal("passive unlock ignored")
	}
	actor = Actor{CharacterID: 1042, Awakening: 3}
	b = buff(t, c, "blackbird_ultimate_attack_awakening4")
	if _, err := b.Evaluate(choice, actor, 1010, 1010); err == nil {
		t.Fatal("awakening filter ignored")
	}
	if err := ValidateBuffSelection(c.Buffs.Effects, []BuffSelection{{ID: "blackbird_ultimate_attack"}, {ID: "blackbird_ultimate_attack_awakening4"}}); err == nil {
		t.Fatal("summed replacement variants")
	}
	if err := ValidateBuffSelection(c.Buffs.Effects, []BuffSelection{{ID: "sagiri_ultimate_attack"}, {ID: "sagiri_ultimate_attack"}}); err == nil {
		t.Fatal("summed duplicate selection")
	}
	for _, pair := range [][2]string{{"akane_sound_field_lakshana_resistance", "akane_lakshana_resistance"}, {"adler_scorch_attack_reduction", "adler_scorch_resistance_reduction_damage_resist_normal_base"}} {
		if err := ValidateBuffSelection(c.Buffs.Effects, []BuffSelection{{ID: pair[0]}, {ID: pair[1]}}); err == nil {
			t.Fatal("summed conditional alternatives")
		}
	}
	if err := ValidateBuffSelection(c.Buffs.Effects, []BuffSelection{{ID: "adler_scorch_resistance_reduction_damage_resist_normal_base"}, {ID: "adler_scorch_resistance_reduction_damage_resist_cosmos_add"}}); err != nil {
		t.Fatalf("split properties of one selected random outcome rejected: %v", err)
	}
}

func TestSelectedStacksCountersCapsAndDamageFilter(t *testing.T) {
	c := production(t)
	b := buff(t, c, "nanally_followup_attack_stacks")
	actor := Actor{CharacterID: b.CharacterID}
	value, err := b.Evaluate(BuffSelection{Stacks: 3, ConditionsConfirmed: true}, actor, b.CharacterID, b.CharacterID)
	if err != nil || value.Value != b.Calculation.Values[0]*3 {
		t.Fatalf("per-stack value: %+v %v", value, err)
	}
	if _, err := b.Evaluate(BuffSelection{Stacks: 11, ConditionsConfirmed: true}, actor, b.CharacterID, b.CharacterID); err == nil {
		t.Fatal("stack limit ignored")
	}
	b = buff(t, c, "baicang_silence_next_skill_damage")
	actor = Actor{CharacterID: b.CharacterID, Awakening: 1}
	count := 10.0
	value, err = b.Evaluate(BuffSelection{Stacks: 1, Counter: &count, ConditionsConfirmed: true, DamageEffectIDs: []string{"selected_skill"}}, actor, b.CharacterID, b.CharacterID)
	if err != nil || value.Value != 2 || value.ModifierScope != "damage_evaluation" {
		t.Fatalf("counter cap: %+v %v", value, err)
	}
	b = buff(t, c, "skia_passive_fang_thrust_shadowatk_damage")
	actor = Actor{CharacterID: b.CharacterID, Breakthrough: 10}
	if _, err := b.Evaluate(BuffSelection{Stacks: 1, ConditionsConfirmed: true, DamageEffectIDs: []string{"other"}}, actor, b.CharacterID, b.CharacterID); err == nil {
		t.Fatal("damage filter ignored")
	}
}

func TestManualScenarioStagesInputsAndZeroActorPriority(t *testing.T) {
	c := production(t)
	actor := Actor{CharacterID: 1042, Stats: map[string]float64{"AtkBase": 100}}
	s := scenarioByID(t, c, "blackbird_nova_settlement")
	value, err := s.Evaluate(actor, nil, map[string]float64{"eligible_actual_damage": 1000000})
	if err != nil || value != 100000 {
		t.Fatalf("settlement cap: %v %v", value, err)
	}
	if _, err := s.Evaluate(actor, nil, nil); err == nil {
		t.Fatal("missing actual damage became zero")
	}
	s = scenarioByID(t, c, "zero_1051_charge_heal")
	team := map[int]Actor{1046: {CharacterID: 1046, Stats: map[string]float64{"AtkBase": 100}}, 1051: {CharacterID: 1051, Stats: map[string]float64{"AtkBase": 400}}}
	value, err = s.Evaluate(actor, team, nil)
	if err != nil || value != 200 {
		t.Fatalf("Zero priority/base attack: %v %v", value, err)
	}
	delete(team, 1051)
	value, err = s.Evaluate(actor, team, nil)
	if err != nil || value != 50 {
		t.Fatalf("Zero fallback: %v %v", value, err)
	}
	if _, err := s.Evaluate(actor, nil, nil); err == nil {
		t.Fatal("arbitrary caster replaced absent Zero")
	}
	s = scenarioByID(t, c, "nanally_passive_follow_up_damage")
	if s.DamageEffectID != nil || s.CandidateDamageEffectID == "" {
		t.Fatal("candidate became confirmed effect")
	}
	if _, err := s.Evaluate(Actor{Stats: map[string]float64{"AtkFinal": 100}}, nil, nil); err == nil {
		t.Fatal("missing assumed Melee level invented")
	}
	value, err = s.Evaluate(Actor{Levels: map[string]int{"GA_Nanally_Melee": 2}, Stats: map[string]float64{"AtkFinal": 100}}, nil, nil)
	if err != nil || value != 64.8 {
		t.Fatalf("assumed Melee curve: %v %v", value, err)
	}
	s = scenarioByID(t, c, "akane_remora_reapplication_damage")
	if s.Inputs["selected_effect_total_duration"].SourceParameter != "CooldownDuration" {
		t.Fatal("wrong native duration output")
	}
	if _, err := s.Evaluate(Actor{Stats: map[string]float64{"AtkFinal": 100}}, nil, map[string]float64{"TimeRemaining": 5}); err == nil {
		t.Fatal("accepted timer as total duration")
	}
}

func TestDeclarativeArithmeticAndWrappersRemainExplicit(t *testing.T) {
	for _, expression := range []string{`{"op":"unknown"}`, `{"op":"input","id":"missing"}`, `{"op":"divide","args":[1,0]}`, `{"op":"skill_level_value","ability_id":"skill","actor":"hit_actor","values":[1]}`} {
		if _, err := Evaluate(json.RawMessage(expression), Context{}); err == nil {
			t.Errorf("silently evaluated %s", expression)
		}
	}
	m := Modifier{ExpressionProjection: json.RawMessage(`{"cases":[{"when":true,"value":50}]}`), Transform: json.RawMessage(`{"coefficient_by_effect_level":[0,2],"pre_multiply_additive_by_effect_level":[3,3],"post_multiply_additive_by_effect_level":[4,4],"final_lookup":{"CurveTable":null,"RowName":"None"}}`)}
	if value, err := m.Evaluate(Context{}, 1); err != nil || value != 4 {
		t.Fatalf("zero wrapper: %v %v", value, err)
	}
	if value, err := m.Evaluate(Context{}, 2); err != nil || value != 110 {
		t.Fatalf("wrapper: %v %v", value, err)
	}
	if _, err := m.Evaluate(Context{}, 0); err == nil {
		t.Fatal("inferred effect-spec level")
	}
	m.PublishedIDs = []string{"existing_buff"}
	if _, err := m.Evaluate(Context{}, 1); err == nil {
		t.Fatal("published modifier counted again")
	}
	query := json.RawMessage(`{"op":"input","id":"duration.out_2","actor":"hit_actor","parameter_name":"CooldownDuration"}`)
	var decoded any
	_ = json.Unmarshal(query, &decoded)
	key, _ := json.Marshal(decoded)
	if _, err := Evaluate(query, Context{Inputs: map[string]float64{"duration.out_2": 3}}); err == nil {
		t.Fatal("lost actor query binding")
	}
	if value, err := Evaluate(query, Context{Bindings: map[string]any{string(key): 3.0}}); err != nil || value != 3 {
		t.Fatalf("explicit named actor query: %v %v", value, err)
	}
}

func TestProvisionalAttributeTransfersUseBaseAttributeAndCaps(t *testing.T) {
	c := production(t)
	for _, effect := range c.Buffs.AttributeExtractions {
		actor := Actor{CharacterID: effect.CharacterID, Stats: map[string]float64{effect.BuilderScenario.SourcePropertyID: 1000}}
		value, err := effect.Evaluate(actor, 1, 100)
		if err != nil || value.Value != 1000*effect.BuilderScenario.Calculation.Caps[0] || value.Target.Percent {
			t.Fatalf("transfer cap: %+v %v", value, err)
		}
		if _, err := effect.Evaluate(actor, 1, 1.5); err == nil {
			t.Fatal("fractional accepted drain count")
		}
	}
}

func TestChaosWarrantBranchesAreAlternativesWithExplicitActorBindings(t *testing.T) {
	c := production(t)
	var modifier Modifier
	for _, entry := range c.Modifiers.Entries {
		if entry.ID == "GE_Reaction_3_new_1071_Damage__CustomApplicationModifierInfos_1_GameplayModifierInfos_0" {
			modifier = entry
		}
	}
	if modifier.ID == "" {
		t.Fatal("missing Chaos modifier")
	}
	key := func(actor, effect string) string {
		raw, _ := json.Marshal(map[string]string{"op": "buff_active", "actor": actor, "effect_id": effect})
		return string(raw)
	}
	for _, test := range []struct {
		marked, passive bool
		want            float64
	}{{true, true, .30}, {true, false, .20}, {false, true, 0}, {false, false, 0}} {
		context := Context{Bindings: map[string]any{key("hit_actor", "Buff_Chaos_skill_Huntmark_C"): test.marked, key("owner_actor", "Buff_Chaos_Passvie2_C"): test.passive}}
		value, err := modifier.Evaluate(context, 1)
		if err != nil || value != test.want {
			t.Errorf("branches %v/%v: %v %v", test.marked, test.passive, value, err)
		}
	}
}
