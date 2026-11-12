package combatdata

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestGame15PassiveProjectionsKeepUnlocksRecipientsFiltersAndSourceZero(t *testing.T) {
	c := production(t)
	for _, tc := range []struct {
		id                                      string
		caster, breakthrough, recipient, stacks int
		want                                    float64
		property                                string
	}{
		{"elyms_passive_critical_damage", 1090, 4, 1090, 1, .1, "CritDamageBase"},
		{"elyms_charge_team_cosmos_damage", 1090, 2, 1036, 8, .3, "DamageUpCosmosBase"},
		{"elyms_charge_team_cosmos_damage", 1090, 2, 1090, 1, .0375, "DamageUpCosmosBase"},
		{"exe_passive_summon_triggered_damage", 1074, 4, 1074, 20, 1, "DamageUpGeneralBase"},
		{"exe_passive_clone_defense_ignore", 1074, 4, 1074, 1, 0, "DefIgnore"},
	} {
		b := buff(t, c, tc.id)
		actor := Actor{CharacterID: tc.caster, Breakthrough: tc.breakthrough}
		selection := BuffSelection{ID: b.ID, Stacks: tc.stacks, ConditionsConfirmed: true, DamageEffectIDs: []string{"GE_Player_Exe_CloneAtk_Damage"}}
		value, err := b.Evaluate(selection, actor, tc.recipient, tc.recipient)
		if err != nil || math.Abs(value.Value-tc.want) > 1e-10 || value.Target.PropertyID != tc.property || !value.Target.Percent {
			t.Fatalf("%s: %+v %v", tc.id, value, err)
		}
		if tc.caster == 1074 && (value.Duration != nil || value.ActiveWindow != nil || b.DurationPolicy != "infinite") {
			t.Fatal("serialized magnitude became a timed lifetime")
		}
		actor.Breakthrough--
		if _, err := b.Evaluate(selection, actor, tc.recipient, tc.recipient); err == nil {
			t.Fatal("passive unlock ignored")
		}
		actor.Breakthrough++
		selection.ConditionsConfirmed = false
		if _, err := b.Evaluate(selection, actor, tc.recipient, tc.recipient); err == nil {
			t.Fatal("passive automatically activated")
		}
		selection.ConditionsConfirmed = true
		selection.Stacks = b.Stacking.Limit + 1
		if _, err := b.Evaluate(selection, actor, tc.recipient, tc.recipient); err == nil {
			t.Fatal("passive stack limit ignored")
		}
	}
	clone := buff(t, c, "exe_passive_clone_defense_ignore")
	if _, err := clone.Evaluate(BuffSelection{Stacks: 1, ConditionsConfirmed: true, DamageEffectIDs: []string{"GE_Player_Exe_Melee2_Damage"}}, Actor{CharacterID: 1074, Breakthrough: 4}, 1074, 1074); err == nil {
		t.Fatal("clone-only zero modifier accepted unrelated damage")
	}
	bonus := buff(t, c, "exe_passive_summon_triggered_damage")
	value, err := bonus.Evaluate(BuffSelection{Stacks: 1, ConditionsConfirmed: true, DamageEffectIDs: []string{"GE_Player_Exe_Melee2_Damage"}}, Actor{CharacterID: 1074, Breakthrough: 4}, 1074, 1074)
	if err != nil || value.Value != .05 || bonus.Conditions.SourceCounterLimit != 5 || !bonus.Conditions.RemoveOnLeaveBattle {
		t.Fatal("summon trigger became summon-only damage or lost contributor evidence")
	}
	if _, err := bonus.Evaluate(BuffSelection{Stacks: 1, ConditionsConfirmed: true, DamageEffectIDs: []string{"selected_damage"}}, Actor{CharacterID: 1074, Breakthrough: 4}, 1090, 1090); err == nil {
		t.Fatal("caster bonus applied to ally")
	}
}

func TestLinko15UsesSelectedEligibleQTEQueryAndReactionExtension(t *testing.T) {
	c := production(t)
	checked := 0
	for _, m := range c.Modifiers.Entries {
		if !strings.Contains(string(m.ExpressionProjection), "linko_eligible_other_qte_count") {
			continue
		}
		checked++
		var projection struct {
			Cases []struct {
				Value json.RawMessage `json:"value"`
			} `json:"cases"`
		}
		if err := json.Unmarshal(m.ExpressionProjection, &projection); err != nil {
			t.Fatal(err)
		}
		var root any
		if err := json.Unmarshal(projection.Cases[0].Value, &root); err != nil {
			t.Fatal(err)
		}
		queries := map[string]string{}
		var walk func(any)
		walk = func(v any) {
			switch node := v.(type) {
			case map[string]any:
				if node["op"] == "input" || node["op"] == "team_members" {
					key, _ := json.Marshal(node)
					id, _ := node["id"].(string)
					if node["op"] == "team_members" {
						id = "team"
					}
					queries[id] = string(key)
				}
				for _, value := range node {
					walk(value)
				}
			case []any:
				for _, value := range node {
					walk(value)
				}
			}
		}
		walk(root)
		if len(queries) != 3 || !strings.Contains(queries["linko_eligible_other_qte_count"], "GetCanTriggerOtherPlayerQTECount") {
			t.Fatal("eligible-QTE query contract changed")
		}
		for count := 0; count <= 3; count++ {
			context := Context{Bindings: map[string]any{queries["GetLinkoSyncFrequencyValue"]: float64(25), queries["team"]: []any{1, 2, 3, 4}, queries["linko_eligible_other_qte_count"]: float64(count)}}
			got, err := m.Evaluate(context, 1)
			want := 1.5 + .004*25 + .25*float64(3-count)
			if err != nil || math.Abs(got-want) > 1e-10 {
				t.Fatalf("eligible QTE count %d: %v want %v (%v)", count, got, want, err)
			}
			delete(context.Bindings, queries["linko_eligible_other_qte_count"])
			context.Inputs = map[string]float64{"linko_equipped_alive_count": 4}
			if _, err := m.Evaluate(context, 1); err == nil {
				t.Fatal("alive character count substituted for native QTE query")
			}
		}
	}
	if checked != 2 {
		t.Fatalf("Linko modifier coverage %d", checked)
	}
	extensions := 0
	for _, e := range c.Reactions.CharacterExtensions {
		if e.CharacterID == 1072 {
			extensions++
			if e.NumericParameters["/ExtraDamageRatio"] != .10 || e.AutomaticActivationSupported {
				t.Fatal("Linko extension changed or activated automatically")
			}
		}
	}
	if extensions != 1 {
		t.Fatal("missing Linko extension")
	}
}
