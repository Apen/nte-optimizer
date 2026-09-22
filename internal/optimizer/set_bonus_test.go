package optimizer

import (
	"os"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func TestLoadProductionSetCatalog(t *testing.T) {
	catalog, err := LoadSetCatalog(filepath.Join("..", "..", "data", "game", "equipment", "sets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Definitions) != 12 {
		t.Fatalf("set count = %d, want 12", len(catalog.Definitions))
	}
	if got := catalog.Definitions["Suit4"]; got.InventorySetID != "Suit4" || len(got.RequiredGeometries) != 4 || got.RequiredGeometries[2] != "L_3_TL" {
		t.Fatalf("unexpected Suit4: %#v", got)
	}
	if got := catalog.Definitions["Suit2"].Bonuses[1].ConditionalStats; len(got) != 1 || got[0].PropertyID != "CritDamageBase" || got[0].Value != .56 {
		t.Fatalf("unexpected Suit2 four-piece projection: %#v", got)
	}
	for id, set := range catalog.Definitions {
		projected := 0
		for _, bonus := range set.Bonuses {
			projected += len(bonus.Stats) + len(bonus.ConditionalStats)
		}
		if projected == 0 {
			t.Errorf("set %s has no structured stat projection", id)
		}
	}
}

func TestLoadSetCatalogNormalizesDataminedInventoryIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sets.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"sets":[{"id":"Suit4","inventory_set_id":"suit4","name_fr":"Crimson","required_geometries":["V_2"],"bonuses":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadSetCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := catalog.Definitions["Suit4"].InventorySetID; got != "Suit4" {
		t.Fatalf("normalized inventory set id = %q", got)
	}
}

func TestObjectiveScoreUsesTargetAndImportance(t *testing.T) {
	goal := []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .60, Tolerance: .05, Importance: 1}}
	atTarget := ObjectiveScore(map[string]float64{"CritBase": .60}, goal)
	nearTarget := ObjectiveScore(map[string]float64{"CritBase": .59}, goal)
	belowTolerance := ObjectiveScore(map[string]float64{"CritBase": .50}, goal)
	if atTarget-nearTarget >= 1 {
		t.Fatalf("59%% should remain close to the 60%% target: target=%v near=%v", atTarget, nearTarget)
	}
	if nearTarget-belowTolerance <= 1.5 {
		t.Fatalf("falling below tolerance should cost materially more: near=%v below=%v", nearTarget, belowTolerance)
	}
	high := ObjectiveScore(map[string]float64{"CritBase": .59}, []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .60, Tolerance: .05, Importance: 2}})
	if high != nearTarget*2 {
		t.Fatalf("importance should scale utility: normal=%v high=%v", nearTarget, high)
	}
}

func TestToleranceFloorIsStrictForBuildAcceptance(t *testing.T) {
	goal := ObjectiveGoal{PropertyID: "CritDamageBase", Minimum: 2.40, Tolerance: .05, StrictMinimum: true}
	if belowToleranceFloor(2.28, goal) {
		t.Fatal("the exact tolerance boundary should be accepted")
	}
	if !belowToleranceFloor(2.279, goal) {
		t.Fatal("a value below the tolerance boundary should be rejected")
	}
	goal.StrictMinimum = false
	if belowToleranceFloor(2.0, goal) {
		t.Fatal("a soft target should never reject a build")
	}
}

func TestStrictMinimumDoesNotDependOnScoringImportance(t *testing.T) {
	goal := ObjectiveGoal{PropertyID: "HPFinal", Minimum: 20000, Importance: 0, StrictMinimum: true}
	if !belowToleranceFloor(19999, goal) {
		t.Fatal("a zero scoring weight must not disable a strict minimum")
	}
	if belowToleranceFloor(20000, goal) {
		t.Fatal("the exact strict minimum should remain admissible")
	}
}

func TestConstraintsDoNotChangeObjectiveScore(t *testing.T) {
	strict := ObjectiveGoal{PropertyID: "CritDamageBase", Minimum: 2.30, Tolerance: 10.0 / 230.0, Importance: 1, StrictMinimum: true}
	soft := strict
	soft.StrictMinimum = false
	soft.Tolerance = 0

	strictFloor := ObjectiveScore(map[string]float64{"CritDamageBase": 2.20}, []ObjectiveGoal{strict})
	strictTarget := ObjectiveScore(map[string]float64{"CritDamageBase": 2.30}, []ObjectiveGoal{strict})
	softFloor := ObjectiveScore(map[string]float64{"CritDamageBase": 2.20}, []ObjectiveGoal{soft})
	softTarget := ObjectiveScore(map[string]float64{"CritDamageBase": 2.30}, []ObjectiveGoal{soft})

	if strictTarget-strictFloor < 1.5 {
		t.Fatalf("a value at the strict floor should remain materially below target: floor=%v target=%v", strictFloor, strictTarget)
	}
	if strictFloor != softFloor || strictTarget != softTarget {
		t.Fatalf("constraints must not alter ranking: strict=%v/%v soft=%v/%v", strictFloor, strictTarget, softFloor, softTarget)
	}
}

func TestCartridgeSetEvaluatorRequiresOwnedCartridgeAndDistinctShapes(t *testing.T) {
	catalog := SetCatalog{Definitions: map[string]SetDefinition{
		"Suit4": {
			ID: "Suit4", InventorySetID: "scarlet", RequiredGeometries: []string{"V_2", "H_3", "L_3_BR", "Trap_4_H"},
			Bonuses: []SetBonus{{Count: 2, Score: 1}, {Count: 4, Score: 3}},
		},
	}}
	modules := []Candidate{
		{Module: nte.Module{LocalID: "v", Geometry: "V_2"}},
		{Module: nte.Module{LocalID: "h", Geometry: "H_3"}},
		{Module: nte.Module{LocalID: "h2", Geometry: "H_3"}},
	}
	placements := []Placement{{ModuleID: "v"}, {ModuleID: "h"}, {ModuleID: "h2"}}
	preferences := []scoring.SetPreference{{SetID: "Suit4", Priority: 2}}

	character := scoring.Character{Weights: map[string]float64{"CritBase": 1}}
	refs := scoring.References{"CritBase": .03}
	missing := NewCartridgeSetEvaluator(catalog, nil, preferences, modules, character, refs)
	if got := missing.Evaluate(placements); got.Score != 0 {
		t.Fatalf("bonus without cartridge = %#v", got)
	}
	owned := NewCartridgeSetEvaluator(catalog, []nte.Cartridge{{LocalID: "c", SetID: "scarlet"}}, preferences, modules, character, refs)
	got := owned.Evaluate(placements)
	if got.Score != 2 || got.SetID != "Suit4" || got.MatchedCount != 2 {
		t.Fatalf("unexpected bonus: %#v", got)
	}
}

func TestCartridgeSetEvaluatorConsidersOwnedNonPreferredSets(t *testing.T) {
	catalog := SetCatalog{Definitions: map[string]SetDefinition{
		"recommended": {ID: "recommended", InventorySetID: "recommended-core", RequiredGeometries: []string{"H_2"}, Bonuses: []SetBonus{{Count: 1, Score: 1}}},
		"alternative": {ID: "alternative", InventorySetID: "alternative-core", RequiredGeometries: []string{"V_2"}, Bonuses: []SetBonus{{Count: 1, Score: 3}}},
	}}
	modules := []Candidate{{Module: nte.Module{LocalID: "v", Geometry: "V_2"}}}
	evaluator := NewCartridgeSetEvaluator(catalog, []nte.Cartridge{
		{LocalID: "recommended-cartridge", SetID: "recommended-core"},
		{LocalID: "alternative-cartridge", SetID: "alternative-core"},
	}, []scoring.SetPreference{{SetID: "recommended", Priority: 1}}, modules, scoring.Character{}, scoring.References{})
	got := evaluator.Evaluate([]Placement{{ModuleID: "v"}})
	if got.SetID != "alternative" || got.CartridgeID != "alternative-cartridge" {
		t.Fatalf("owned alternative set was excluded by recommendation: %#v", got)
	}
}

func TestObjectiveEvaluatorDoesNotRewardOccupiedArea(t *testing.T) {
	catalog := SetCatalog{Definitions: map[string]SetDefinition{
		"set": {ID: "set", InventorySetID: "set", RequiredGeometries: []string{"H_2"}, Bonuses: []SetBonus{{Count: 1, Score: 3}}},
	}}
	modules := []Candidate{
		{Module: nte.Module{LocalID: "required", Geometry: "H_2", Area: 2}},
		{Module: nte.Module{LocalID: "filler", Geometry: "V_3", Area: 3}},
	}
	evaluator := NewExactObjectiveEvaluator(catalog, []nte.Cartridge{{LocalID: "c", SetID: "set"}}, nil, modules, scoring.Character{}, nil, []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .7}}, nil)
	compact := evaluator.Evaluate([]Placement{{ModuleID: "required"}})
	filled := evaluator.Evaluate([]Placement{{ModuleID: "required"}, {ModuleID: "filler"}})
	if compact.Score != filled.Score {
		t.Fatalf("occupied area changed objective score: compact=%v filled=%v", compact.Score, filled.Score)
	}
}

func TestObjectiveScoreStronglyDiminishesExcess(t *testing.T) {
	goals := []ObjectiveGoal{{PropertyID: "MagBase", Minimum: 200}, {PropertyID: "CritBase", Minimum: .6}}
	balanced := ObjectiveScore(map[string]float64{"MagBase": 200, "CritBase": .6}, goals)
	overcapped := ObjectiveScore(map[string]float64{"MagBase": 280, "CritBase": .6}, goals)
	missingCrit := ObjectiveScore(map[string]float64{"MagBase": 280, "CritBase": .5}, goals)
	if overcapped-balanced > 6 {
		t.Fatalf("cycle excess is worth too much: %.2f", overcapped-balanced)
	}
	if overcapped <= missingCrit {
		t.Fatalf("meeting crit target should dominate cycle excess")
	}
}

func TestObjectiveScoreTreatsMaximumAsHardConstraint(t *testing.T) {
	goals := []ObjectiveGoal{{PropertyID: "MagBase", Minimum: 200, Maximum: 210}}
	inside := ObjectiveScore(map[string]float64{"MagBase": 205}, goals)
	over := ObjectiveScore(map[string]float64{"MagBase": 280}, goals)
	if over >= inside-10000 {
		t.Fatalf("maximum constraint must dominate ordinary utility: inside=%v over=%v", inside, over)
	}
}
