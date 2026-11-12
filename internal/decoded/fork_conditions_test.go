package decoded

import (
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestPublishedArcConditionsResolveExactRanksWithoutChangingStats(t *testing.T) {
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
			{"fork_exe", "dotted_horizon_wearer_summon_damage", "DamageUpGeneralBase", [5]float64{.28, .35, .42, .49, .56}, 1, true, []string{"State.Damage.Summon"}},
			{"fork_exe", "dotted_horizon_team_summon_stacks", "DamageUpGeneralBase", [5]float64{.02, .025, .03, .035, .04}, 10, false, []string{"State.Damage.Summon"}},
		} {
			weapon := Weapon{ForkID: tc.fork, Level: 80, Breakthrough: 6, Star: rank}
			p, a, conditional, note := catalog.Stats(weapon)
			value, err := catalog.ResolveConditionalEffect(weapon, tc.effect, ForkEffectSelection{ConditionsConfirmed: true, Stacks: tc.stacks, RecipientIsWearer: tc.wearer, DamageTags: tc.tags})
			want := tc.values[rank-1] * float64(tc.stacks)
			if err != nil || value.Stat.PropertyID != tc.property || !value.Stat.Percent || math.Abs(value.Stat.Value-want) > 1e-10 {
				t.Fatalf("rank %d %s: %+v, want %v (%v)", rank, tc.effect, value, want, err)
			}
			if tc.stacks == 10 && (value.Duration == nil || *value.Duration != 20) {
				t.Fatal("lost selected Arc duration")
			}
			p2, a2, c2, n2 := catalog.Stats(weapon)
			if !reflect.DeepEqual(p, p2) || !reflect.DeepEqual(a, a2) || !reflect.DeepEqual(conditional, c2) || note != n2 {
				t.Fatal("manual Arc selection mutated automatic stats")
			}
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
		{ConditionsConfirmed: true, Stacks: 2, RecipientIsWearer: true, DamageTags: []string{"State.Damage.Summon"}},
	} {
		if _, err := catalog.ResolveConditionalEffect(weapon, "dotted_horizon_wearer_summon_damage", selected); err == nil {
			t.Fatal("invalid Arc selection accepted")
		}
	}
	if _, err := catalog.ResolveConditionalEffect(weapon, "dotted_horizon_team_summon_stacks", ForkEffectSelection{ConditionsConfirmed: true, Stacks: 11, DamageTags: []string{"State.Damage.Summon"}}); err == nil {
		t.Fatal("Arc stack cap ignored")
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
