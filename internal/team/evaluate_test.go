package team

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"nte-optimizer/internal/decoded"
)

func TestPublishedTeamEffects(t *testing.T) {
	data := filepath.Join("..", "..", "data", "game")
	catalog, err := Load(filepath.Join(data, "equipment", "arcs.json"), filepath.Join(data, "characters", "decode.json"))
	if err != nil {
		t.Fatal(err)
	}
	same := []int{1003}
	other := 0
	for id, esper := range catalog.types {
		if id == 1003 {
			continue
		}
		if esper == catalog.types[1003] {
			same = append(same, id)
		} else {
			other = id
		}
	}
	slices.Sort(same)
	if len(same) < 3 || other == 0 {
		t.Fatal("catalog lacks a testable team composition")
	}
	a, b, c := same[0], same[1], same[2]
	bitgame := decoded.Weapon{ForkID: "fork_BitGame", Star: 1}
	thief := decoded.Weapon{ForkID: "fork_tuansanlang", Star: 1}
	tests := []struct {
		name       string
		members    []Member
		active     int
		property   string
		value      float64
		percent    bool
		recipients []int
		sources    int
		reason     string
		unresolved int
	}{
		{name: "active ally", members: []Member{{CharacterID: a, Arc: bitgame}, {CharacterID: b}}, active: b, property: "AtkUp", value: .1, percent: true, recipients: []int{b}, sources: 1},
		{name: "active wearer", members: []Member{{CharacterID: a, Arc: bitgame}, {CharacterID: b}}, active: a},
		{name: "multiple off-field sources", members: []Member{{CharacterID: a, Arc: bitgame}, {CharacterID: b, Arc: bitgame}, {CharacterID: c}}, active: c, reason: "multiple_sources_unsupported", unresolved: 1},
		{name: "two matching", members: []Member{{CharacterID: a, Arc: thief}, {CharacterID: b}, {CharacterID: other}}, active: b},
		{name: "three matching including wearer", members: []Member{{CharacterID: a, Arc: thief}, {CharacterID: b}, {CharacterID: c}, {CharacterID: other}}, active: other, property: "UnbalIntensityBase", value: 70, recipients: []int{a, b, c}, sources: 1},
		{name: "equal ranks", members: []Member{{CharacterID: a, Arc: thief}, {CharacterID: b, Arc: thief}, {CharacterID: c}}, active: b, property: "UnbalIntensityBase", value: 70, recipients: []int{a, b, c}, sources: 2},
		{name: "mixed ranks", members: []Member{{CharacterID: a, Arc: thief}, {CharacterID: b, Arc: decoded.Weapon{ForkID: thief.ForkID, Star: 2}}, {CharacterID: c}}, active: b, reason: "mixed_rank_priority_unknown", unresolved: 3},
		{name: "missing exact rank", members: []Member{{CharacterID: a, Arc: decoded.Weapon{ForkID: bitgame.ForkID, Star: 999}}, {CharacterID: b}}, active: b, reason: "missing_rank_parameter", unresolved: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := catalog.Evaluate(tc.members, tc.active)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Contributions) != len(tc.recipients) || len(result.Unresolved) != tc.unresolved {
				t.Fatalf("unexpected result: %+v", result)
			}
			for i, item := range result.Contributions {
				if item.RecipientID != tc.recipients[i] || item.Stat.PropertyID != tc.property || item.Stat.Value != tc.value || item.Stat.Percent != tc.percent || len(item.SourceIDs) != tc.sources {
					t.Fatalf("unexpected contribution: %+v", item)
				}
			}
			for _, item := range result.Unresolved {
				if item.Reason != tc.reason {
					t.Fatalf("unexpected reason: %+v", item)
				}
			}
			reversed := slices.Clone(tc.members)
			slices.Reverse(reversed)
			again, err := catalog.Evaluate(reversed, tc.active)
			if err != nil || !reflect.DeepEqual(again, result) {
				t.Fatalf("evaluation depends on input order: %+v, %v", again, err)
			}
		})
	}
}
