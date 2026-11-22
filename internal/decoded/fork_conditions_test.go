package decoded

import (
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestPublishedArcConditionCatalogResolvesEveryDeclaredRank(t *testing.T) {
	catalog, err := LoadForkCatalog(filepath.Join("..", "..", "data", "game", "equipment", "arcs.json"))
	if err != nil {
		t.Fatal(err)
	}
	for forkID, arc := range catalog.Forks {
		for _, effect := range arc.ConditionalEffects {
			t.Run(forkID+"/"+effect.ID, func(t *testing.T) {
				if len(arc.EffectsByStar) == 0 {
					t.Fatal("published condition has no rank parameters")
				}
				for rank := range arc.EffectsByStar {
					star, err := strconv.Atoi(rank)
					if err != nil || star < 1 {
						t.Errorf("invalid published Arc rank %q", rank)
						continue
					}
					weapon := Weapon{ForkID: forkID, Star: star}
					value, err := catalog.ResolveConditionalEffect(weapon, effect.ID, ForkEffectSelection{
						ConditionsConfirmed: true,
						RecipientIsWearer:   effect.Scope != "other_allies",
						DamageTags:          append(slices.Clone(effect.RequiredDamageTags), effect.AnyDamageTags...),
						Stacks:              1,
						ReplaceLegacy:       effect.LegacyConditionalProjection,
					})
					if err != nil {
						t.Errorf("rank %s: %v", rank, err)
						continue
					}
					if effect.LegacyConditionalProjection && !slices.Contains(arc.ConditionalByStar[rank], value.Stat) {
						t.Errorf("rank %s: missing exact legacy stat %+v", rank, value.Stat)
					}
				}
			})
		}
	}
}

func TestForkDamageFilterMatchesAllRequiredAndAnyAlternativeTag(t *testing.T) {
	filter := ForkDamageFilter{
		RequiredDamageTags: []string{"Damage.Direct"},
		AnyDamageTags:      []string{"Damage.Melee", "Damage.Branch"},
	}
	for _, tc := range []struct {
		name string
		tags []string
		want bool
	}{
		{"melee", []string{"Damage.Direct", "Damage.Melee"}, true},
		{"branch descendant", []string{"Damage.Direct", "Damage.Branch.AirAttack"}, true},
		{"both", []string{"Damage.Direct", "Damage.Melee", "Damage.Branch"}, true},
		{"missing required", []string{"Damage.Melee"}, false},
		{"missing alternative", []string{"Damage.Direct", "Damage.Other"}, false},
		{"nonsegment prefix", []string{"Damage.Direct", "Damage.Branching"}, false},
		{"parent only", []string{"Damage.Direct", "Damage"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := filter.Matches(tc.tags); got != tc.want {
				t.Fatalf("Matches(%v) = %t, want %t", tc.tags, got, tc.want)
			}
		})
	}
}

func TestFilteredWearerArcRequiresAConfirmedMatchingHit(t *testing.T) {
	var catalog ForkCatalog
	const raw = `{"forks":{"arc":{"effects_by_star":{"1":{"parameters":[{"name_id":"magnitude","value":0.25,"is_percent":true}]}},"conditional_effects":[{
		"id":"effect","scope":"wearer_filtered_damage","requires_any_damage_tags":["Damage.Melee","Damage.Branch"],
		"parameter":{"name_id":"magnitude","level_source":"exact_arc_rank"},
		"target":{"property_id":"DamageUpGeneralBase","operation":"add","percent":true}
	}]}}}`
	if err := json.Unmarshal([]byte(raw), &catalog); err != nil {
		t.Fatal(err)
	}
	weapon := Weapon{ForkID: "arc", Star: 1}
	for _, tc := range []struct {
		name      string
		selection ForkEffectSelection
		wantError bool
	}{
		{"matching descendant", ForkEffectSelection{ConditionsConfirmed: true, RecipientIsWearer: true, DamageTags: []string{"Damage.Branch.Air"}, Stacks: 1}, false},
		{"no active condition", ForkEffectSelection{RecipientIsWearer: true, DamageTags: []string{"Damage.Branch"}, Stacks: 1}, true},
		{"other recipient", ForkEffectSelection{ConditionsConfirmed: true, DamageTags: []string{"Damage.Branch"}, Stacks: 1}, true},
		{"ineligible hit", ForkEffectSelection{ConditionsConfirmed: true, RecipientIsWearer: true, DamageTags: []string{"Damage.Skill"}, Stacks: 1}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := catalog.ResolveConditionalEffect(weapon, "effect", tc.selection)
			if (err != nil) != tc.wantError {
				t.Fatalf("ResolveConditionalEffect() error = %v, want error %t", err, tc.wantError)
			}
		})
	}
	catalog.Forks["arc"].ConditionalEffects[0].AnyDamageTags = nil
	if _, err := catalog.ResolveConditionalEffect(weapon, "effect", ForkEffectSelection{ConditionsConfirmed: true, RecipientIsWearer: true, Stacks: 1}); err == nil {
		t.Fatal("filtered scope accepted a missing filter")
	}
}

func TestFixedArcStacksUseCatalogCountAfterConfirmation(t *testing.T) {
	var catalog ForkCatalog
	const raw = `{"forks":{"arc":{"effects_by_star":{"1":{"parameters":[{"name_id":"magnitude","value":0.125,"is_percent":true}]}},"conditional_effects":[{
		"id":"grant","scope":"wearer","stack_input":"fixed_stack_count","fixed_stacks":3,"max_stacks":5,
		"parameter":{"name_id":"magnitude","level_source":"exact_arc_rank"},
		"target":{"property_id":"CritDamageBase","operation":"add","percent":true}
	}]}}}`
	if err := json.Unmarshal([]byte(raw), &catalog); err != nil {
		t.Fatal(err)
	}
	rule := &catalog.Forks["arc"].ConditionalEffects[0]
	if rule.SelectableStacks() != 1 {
		t.Fatalf("fixed grant should use a checkbox, got %d selectable stacks", rule.SelectableStacks())
	}
	weapon := Weapon{ForkID: "arc", Star: 1}
	selection := ForkEffectSelection{ConditionsConfirmed: true, RecipientIsWearer: true, Stacks: 1}
	value, err := catalog.ResolveConditionalEffect(weapon, "grant", selection)
	if err != nil || value.Stat.Value != .375 {
		t.Fatalf("confirmed fixed grant = %+v, %v, want three stacks", value, err)
	}
	selection.Stacks = 2
	if _, err := catalog.ResolveConditionalEffect(weapon, "grant", selection); err == nil {
		t.Fatal("accepted a user-supplied stack count for a fixed grant")
	}
	selection.Stacks = 1
	rule.FixedStacks = 6
	if _, err := catalog.ResolveConditionalEffect(weapon, "grant", selection); err == nil {
		t.Fatal("accepted a fixed grant beyond its catalog limit")
	}
}

func TestPublishedArcConditionsResolveExactRanksWithoutChangingStats(t *testing.T) {
	const summonStacks = "dotted_horizon_wearer_summon_stacks"
	catalog, err := LoadForkCatalog(filepath.Join("..", "..", "data", "game", "equipment", "arcs.json"))
	if err != nil {
		t.Fatal(err)
	}
	for rank := 1; rank <= 5; rank++ {
		for _, tc := range []struct {
			fork, effect, property string
			values                 [5]float64
			stacks                 int
			wearer                 bool
			tags                   []string
		}{
			{"fork_elyms", "echoes_on_hold_team_defense_ignore", "DefIgnore", [5]float64{.12, .14, .16, .18, .20}, 1, false, nil},
			{"fork_elyms", "echoes_on_hold_team_cosmos_penetration", "DamagePenetrateCosmos", [5]float64{.08, .11, .14, .17, .20}, 1, false, nil},
			{"fork_exe", summonStacks, "DamageUpGeneralBase", [5]float64{.02, .025, .03, .035, .04}, 10, true, []string{"State.Damage.Summon"}},
		} {
			weapon := Weapon{ForkID: tc.fork, Level: 80, Breakthrough: 6, Star: rank}
			p, a, conditional, note := catalog.Stats(weapon)
			value, err := catalog.ResolveConditionalEffect(weapon, tc.effect, ForkEffectSelection{ConditionsConfirmed: true, Stacks: tc.stacks, RecipientIsWearer: tc.wearer, DamageTags: tc.tags})
			want := tc.values[rank-1] * float64(tc.stacks)
			if err != nil || value.Stat.PropertyID != tc.property || !value.Stat.Percent || math.Abs(value.Stat.Value-want) > 1e-10 {
				t.Fatalf("rank %d %s: %+v, want %v (%v)", rank, tc.effect, value, want, err)
			}
			if tc.effect == summonStacks && (value.Scope != "wearer_summon_damage" || value.Duration == nil || *value.Duration != 20) {
				t.Fatalf("wearer-only summon scope or duration changed: %+v", value)
			}
			p2, a2, c2, n2 := catalog.Stats(weapon)
			if !reflect.DeepEqual(p, p2) || !reflect.DeepEqual(a, a2) || !reflect.DeepEqual(conditional, c2) || note != n2 {
				t.Fatal("manual Arc selection mutated automatic stats")
			}
		}
		_, permanent, _, _ := catalog.Stats(Weapon{ForkID: "fork_exe", Level: 80, Breakthrough: 6, Star: rank})
		wantAttack := [5]float64{.28, .35, .42, .49, .56}[rank-1]
		if !slices.ContainsFunc(permanent, func(stat nte.Stat) bool {
			return stat.PropertyID == "AtkUp" && stat.Percent && math.Abs(stat.Value-wantAttack) < 1e-10
		}) {
			t.Fatalf("rank %d: missing permanent Dotted Horizon ATK bonus (%g)", rank, wantAttack)
		}
		weapon := Weapon{ForkID: "fork_elyms", Star: rank}
		if _, err := catalog.ResolveConditionalEffect(weapon, "echoes_on_hold_wearer_extra_defense_ignore", ForkEffectSelection{ConditionsConfirmed: true, Stacks: 1, RecipientIsWearer: true}); err == nil {
			t.Fatal("legacy Arc bonus counted twice")
		}
		_, _, legacy, _ := catalog.Stats(weapon)
		if len(legacy) != 1 || math.Abs(legacy[0].Value-.1*(1+float64(rank-1)*.25)) > 1e-10 {
			t.Fatal("legacy wearer bonus changed")
		}
	}
	for _, rank := range []int{0, 6, 99} {
		if _, err := catalog.ResolveConditionalEffect(Weapon{ForkID: "fork_elyms", Star: rank}, "echoes_on_hold_team_defense_ignore", ForkEffectSelection{ConditionsConfirmed: true, Stacks: 1}); err == nil {
			t.Fatal("invalid rank clamped")
		}
	}
	weapon := Weapon{ForkID: "fork_exe", Star: 1}
	for _, selected := range []ForkEffectSelection{
		{Stacks: 1, RecipientIsWearer: true, DamageTags: []string{"State.Damage.Summon"}},
		{ConditionsConfirmed: true, Stacks: 1, RecipientIsWearer: true},
		{ConditionsConfirmed: true, Stacks: 1, DamageTags: []string{"State.Damage.Summon"}},
		{ConditionsConfirmed: true, Stacks: 11, RecipientIsWearer: true, DamageTags: []string{"State.Damage.Summon"}},
	} {
		if _, err := catalog.ResolveConditionalEffect(weapon, summonStacks, selected); err == nil {
			t.Fatal("invalid Arc selection accepted")
		}
	}
}

func TestGame15ArcCompatibilityKeepsDistinctCharacterGroups(t *testing.T) {
	root := filepath.Join("..", "..", "data", "game")
	catalog, err := LoadArcCompatibilityCatalog(filepath.Join(root, "characters", "decode.json"), filepath.Join(root, "equipment", "arcs.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fork         string
		owner, other int
	}{{"fork_exe", 1074, 1090}, {"fork_elyms", 1090, 1074}} {
		ids := catalog.CompatibleCharacterIDs(tc.fork)
		if !slices.Contains(ids, tc.owner) || slices.Contains(ids, tc.other) {
			t.Fatalf("invalid compatibility for %s: %v", tc.fork, ids)
		}
	}
}
