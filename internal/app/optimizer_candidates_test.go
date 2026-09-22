package app

import (
	"strings"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func TestPrepareModuleCandidatesAppliesAvailabilityRules(t *testing.T) {
	service := OptimizerService{
		ExcludedModuleIDs: map[string]bool{"excluded": true},
		ReservedModuleIDs: map[string]bool{"reserved": true},
	}
	modules := []nte.Module{
		{LocalID: "available", Geometry: "H_2"},
		{LocalID: "excluded", Geometry: "H_2"},
		{LocalID: "reserved", Geometry: "H_2"},
		{LocalID: "other-character", Geometry: "H_2", EquippedCharacterID: 2},
		{LocalID: "same-character", Geometry: "H_2", EquippedCharacterID: 1},
	}
	pool := service.prepareModuleCandidates(modules, scoring.Character{CharacterID: 1}, scoring.References{}, nil, nil, nil, false)
	if len(pool.eligible) != 2 || pool.eligible[0].Module.LocalID != "available" || pool.eligible[1].Module.LocalID != "same-character" {
		t.Fatalf("unexpected eligible candidates: %#v", pool.eligible)
	}
	if len(pool.raw) != 2 || pool.excludedEquipped != 2 {
		t.Fatalf("unexpected pool counts: %#v", pool)
	}
}

func TestSelectModuleCandidatesRejectsInvalidLocks(t *testing.T) {
	pool := moduleCandidatePool{eligible: []optimizer.Candidate{{Module: nte.Module{LocalID: "available", Geometry: "H_2"}}}}
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 1
	config.Optimize.TopPerSet = 1

	service := OptimizerService{PinnedModuleIDs: map[string]bool{"missing": true}}
	if _, err := service.selectModuleCandidates(pool, nil, 4, config, searchPlan{solverMode: "score", approximate: true}); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("missing lock error = %v", err)
	}

	service = OptimizerService{PinnedModuleIDs: map[string]bool{"available": true}, ExcludedModuleIDs: map[string]bool{"available": true}}
	if _, err := service.selectModuleCandidates(pool, nil, 4, config, searchPlan{solverMode: "score", approximate: true}); err == nil || !strings.Contains(err.Error(), "both locked and excluded") {
		t.Fatalf("conflicting lock error = %v", err)
	}
}

func TestAppendUniqueCandidatesPreservesOrder(t *testing.T) {
	first := optimizer.Candidate{Module: nte.Module{LocalID: "first"}}
	second := optimizer.Candidate{Module: nte.Module{LocalID: "second"}}
	got := appendUniqueCandidates([]optimizer.Candidate{first}, []optimizer.Candidate{first, second})
	if len(got) != 2 || got[0].Module.LocalID != "first" || got[1].Module.LocalID != "second" {
		t.Fatalf("unexpected candidates: %#v", got)
	}
}

func TestFastSelectionRetainsCurrentEquipment(t *testing.T) {
	best := optimizer.Candidate{Module: nte.Module{LocalID: "best", Geometry: "H_2"}, Score: 10}
	current := optimizer.Candidate{Module: nte.Module{LocalID: "current", Geometry: "H_2", EquippedCharacterID: 1}, Score: 1}
	pool := moduleCandidatePool{eligible: []optimizer.Candidate{best, current}, raw: []optimizer.Candidate{best, current}, current: []optimizer.Candidate{current}}
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 1
	config.Optimize.TopPerSet = 1
	selection, err := (OptimizerService{}).selectModuleCandidates(pool, nil, 4, config, searchPlan{solverMode: "score", approximate: true})
	if err != nil {
		t.Fatal(err)
	}
	ids := candidateIDs(selection.selected)
	if !ids["best"] || !ids["current"] {
		t.Fatalf("fast selection dropped current equipment: %#v", selection.selected)
	}
}

func TestPrepareCartridgesAppliesStrategyAndAvailability(t *testing.T) {
	service := OptimizerService{
		WeightOverrides:      &WeightOverrides{MainStats: []string{"Crit"}},
		ReservedCartridgeIDs: map[string]bool{"reserved": true},
	}
	profile := scoring.Character{CharacterID: 1, PreferredSets: []scoring.SetPreference{{SetID: "preferred"}}}
	sets := optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{"preferred": {InventorySetID: "inventory-set"}}}
	cartridges := []nte.Cartridge{
		{LocalID: "available", SetID: "inventory-set", MainStats: []nte.Stat{{PropertyID: "Crit"}}},
		{LocalID: "wrong-set", SetID: "other", MainStats: []nte.Stat{{PropertyID: "Crit"}}},
		{LocalID: "wrong-stat", SetID: "inventory-set", MainStats: []nte.Stat{{PropertyID: "Atk"}}},
		{LocalID: "reserved", SetID: "inventory-set", MainStats: []nte.Stat{{PropertyID: "Crit"}}},
		{LocalID: "other-character", SetID: "inventory-set", MainStats: []nte.Stat{{PropertyID: "Crit"}}, EquippedCharacterID: 2},
	}
	pool, err := service.prepareCartridges(cartridges, profile, sets, false, searchPlan{solverMode: "objective", approximate: true}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool.available) != 1 || pool.available[0].LocalID != "available" || pool.excludedEquipped != 2 {
		t.Fatalf("unexpected cartridge pool: %#v", pool)
	}
}

func TestPrepareCartridgesRequiresEligibleStrategySet(t *testing.T) {
	profile := scoring.Character{Name: "Test", PreferredSets: []scoring.SetPreference{{SetID: "preferred"}}}
	sets := optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{"preferred": {InventorySetID: "inventory-set"}}}
	service := OptimizerService{}
	if _, err := service.prepareCartridges(nil, profile, sets, true, searchPlan{solverMode: "objective", approximate: true}, 1); err == nil || !strings.Contains(err.Error(), "selected strategy set") {
		t.Fatalf("missing cartridge error = %v", err)
	}
	service.WeightOverrides = &WeightOverrides{MainStats: []string{"Crit"}}
	if _, err := service.prepareCartridges(nil, profile, sets, true, searchPlan{solverMode: "objective", approximate: true}, 1); err == nil || !strings.Contains(err.Error(), "selected set and main stats") {
		t.Fatalf("missing custom cartridge error = %v", err)
	}
}

func TestPrepareModuleCandidatesAddsObjectiveAndGeometryPriority(t *testing.T) {
	service := OptimizerService{}
	profile := scoring.Character{BaseStats: map[string]float64{"CritBase": .5}}
	goals := []optimizer.ObjectiveGoal{{PropertyID: "CritBase", Minimum: .75, Importance: 1}}
	modules := []nte.Module{{LocalID: "plain", Geometry: "V_2"}, {LocalID: "required", Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .1}}}}
	pool := service.prepareModuleCandidates(modules, profile, scoring.References{}, nil, goals, map[string]bool{"H_2": true}, true)
	if len(pool.eligible) != 2 || len(pool.selectionObjectives) != 1 {
		t.Fatalf("unexpected pool: %#v", pool)
	}
	if pool.eligible[1].ObjectiveValues["CritBase"] <= 0 || pool.eligible[1].Priority <= pool.eligible[0].Priority+24 {
		t.Fatalf("objective/geometry priority was not applied: %#v", pool.eligible)
	}
}
