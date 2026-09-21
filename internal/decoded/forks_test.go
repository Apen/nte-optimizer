package decoded

import (
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
