package decoded

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/nte"
)

func TestProductionForkCatalogResolvesZankouWeapon(t *testing.T) {
	catalog, err := LoadForkCatalog(filepath.Join("..", "..", "data", "game", "equipment", "arcs.json"))
	if err != nil {
		t.Fatal(err)
	}
	panel, permanent, conditional, _ := catalog.Stats(Weapon{ForkID: "fork_DemonBlade", Level: 80, Breakthrough: 6, Star: 1})
	if len(panel) != 2 || panel[0].PropertyID != "AtkBase" || panel[0].Value != 570 || len(permanent) != 1 || permanent[0].Value != .16 || len(conditional) != 1 || conditional[0].Value != .63 {
		t.Fatalf("unexpected fork stats: %#v %#v %#v", panel, permanent, conditional)
	}
}

func TestProductionForkCatalogKeepsPassiveEffects(t *testing.T) {
	catalog, err := LoadForkCatalog(filepath.Join("..", "..", "data", "game", "equipment", "arcs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Forks) == 0 {
		t.Fatal("production fork catalog is empty")
	}
	for id, fork := range catalog.Forks {
		if len(fork.EffectsByStar) == 0 {
			t.Errorf("fork %s has no passive effects projection", id)
		}
	}
}

func TestForkCatalogCombinesLevelAndBreakthroughStats(t *testing.T) {
	catalog := ForkCatalog{Forks: map[string]ForkDefinition{"fork_test": {LevelStats: map[string][]nte.Stat{"40": {{PropertyID: "AtkBase", Value: 200}}}, BreakthroughStats: map[string][]nte.Stat{"3": {{PropertyID: "CritBase", Value: .12}}}}}}
	panel, _, _, _ := catalog.Stats(Weapon{ForkID: "fork_test", Level: 40, Breakthrough: 3, Star: 1})
	if len(panel) != 2 || panel[0].Value != 200 || panel[1].Value != .12 {
		t.Fatalf("unexpected combined panel: %#v", panel)
	}
}

func TestForkEffectPreservesOrderedDescriptionParameters(t *testing.T) {
	var effect ForkEffect
	if err := json.Unmarshal([]byte(`{"parameters":[{"name_id":"crit_rate","value":0.16,"is_percent":true},{"name_id":"crit_damage","value":0.09,"is_percent":true},{"name_id":"duration","value":15,"is_percent":false}]}`), &effect); err != nil {
		t.Fatal(err)
	}
	if len(effect.OrderedParameters) != 3 {
		t.Fatalf("ordered parameters = %#v, want 3 entries", effect.OrderedParameters)
	}
	want := []ForkEffectParameter{{NameID: "crit_rate", Value: .16, IsPercent: true}, {NameID: "crit_damage", Value: .09, IsPercent: true}, {NameID: "duration", Value: 15}}
	for index, parameter := range want {
		if effect.OrderedParameters[index] != parameter {
			t.Errorf("parameter %d = %#v, want %#v", index, effect.OrderedParameters[index], parameter)
		}
	}
}

func TestForkEffectDoesNotInventOrderFromLegacyParameterMap(t *testing.T) {
	var effect ForkEffect
	if err := json.Unmarshal([]byte(`{"parameters":{"duration":15,"crit_rate":0.16}}`), &effect); err != nil {
		t.Fatal(err)
	}
	if len(effect.Parameters) != 2 || len(effect.OrderedParameters) != 0 {
		t.Fatalf("legacy parameters = %#v, ordered = %#v", effect.Parameters, effect.OrderedParameters)
	}
}
