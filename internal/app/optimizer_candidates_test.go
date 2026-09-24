package app

import (
	"context"
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
	if _, err := service.selectModuleCandidates(pool, config, searchPlan{solverMode: "score", approximate: true}); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("missing lock error = %v", err)
	}

	service = OptimizerService{PinnedModuleIDs: map[string]bool{"available": true}, ExcludedModuleIDs: map[string]bool{"available": true}}
	if _, err := service.selectModuleCandidates(pool, config, searchPlan{solverMode: "score", approximate: true}); err == nil || !strings.Contains(err.Error(), "both locked and excluded") {
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
	selection, err := (OptimizerService{}).selectModuleCandidates(pool, config, searchPlan{solverMode: "score", approximate: true})
	if err != nil {
		t.Fatal(err)
	}
	ids := candidateIDs(selection.selected)
	if !ids["best"] || !ids["current"] {
		t.Fatalf("fast selection dropped current equipment: %#v", selection.selected)
	}
}

func TestFastSelectionKeepsCandidatesBelowStrictMaximum(t *testing.T) {
	tooHigh := optimizer.Candidate{Module: nte.Module{LocalID: "too-high", Geometry: "H_2"}, Score: 11, Priority: 2, ObjectiveValues: map[string]float64{"CritBase": .25}}
	admissible := optimizer.Candidate{Module: nte.Module{LocalID: "admissible", Geometry: "H_2"}, Score: 10, Priority: 1, ObjectiveValues: map[string]float64{"CritBase": .15}}
	goal := optimizer.ObjectiveGoal{PropertyID: "CritBase", Minimum: .6, Maximum: .6, Importance: 1}
	pool := moduleCandidatePool{eligible: []optimizer.Candidate{tooHigh, admissible}, raw: []optimizer.Candidate{tooHigh, admissible}, selectionObjectives: []optimizer.ObjectiveGoal{goal}}
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 2
	config.Optimize.TopPerSet = 2
	selection, err := (OptimizerService{}).selectModuleCandidates(pool, config, searchPlan{solverMode: "objective", approximate: true})
	if err != nil {
		t.Fatal(err)
	}
	if !candidateIDs(selection.selected)["admissible"] {
		t.Fatal("Fast discarded the admissible candidate before enforcing the maximum")
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

func TestBetaKeepsSpecialistsForStrictFloorAboveSoftTarget(t *testing.T) {
	goal := optimizer.ObjectiveGoal{PropertyID: "UnbalIntensityBase", Minimum: 150, StrictMinimum: true, StrictFloor: 230, Importance: 0, ExplicitWeight: true}
	baseline := map[string]float64{"UnbalIntensityBase": 160}
	if got := objectivesStillMissing(baseline, []optimizer.ObjectiveGoal{goal}); len(got) != 0 {
		t.Fatalf("soft target should already be met: %#v", got)
	}
	strict := strictObjectivesStillMissing(baseline, []optimizer.ObjectiveGoal{goal})
	if len(strict) != 1 {
		t.Fatalf("strict floor was not selected: %#v", strict)
	}
	prepared := (OptimizerService{}).prepareModuleCandidates(
		[]nte.Module{{LocalID: "specialist", Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "UnbalIntensityBase", Value: 30}}}},
		scoring.Character{BaseStats: baseline}, scoring.References{}, nil, []optimizer.ObjectiveGoal{goal}, nil, true,
	)
	if len(prepared.strictObjectives) != 1 || len(prepared.selectionObjectives) != 0 {
		t.Fatalf("unexpected prepared objectives: %#v", prepared)
	}
	pool := moduleCandidatePool{strictObjectives: strict, baselineStats: baseline}
	for i := 0; i < 6; i++ {
		pool.eligible = append(pool.eligible, optimizer.Candidate{Module: nte.Module{LocalID: string(rune('a' + i)), Geometry: "H_2"}, Priority: float64(10 - i), ObjectiveValues: map[string]float64{"UnbalIntensityBase": 0}})
	}
	for i := 0; i < 3; i++ {
		gain := float64(30 - i)
		pool.eligible = append(pool.eligible, optimizer.Candidate{Module: nte.Module{LocalID: string(rune('x' + i)), Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "UnbalIntensityBase", Value: gain}}}, Priority: float64(i), ObjectiveValues: map[string]float64{"UnbalIntensityBase": gain}})
	}
	pool.raw = pool.eligible
	if count := strictSpecialistCount(pool.eligible, baseline[goal.PropertyID], goal, 3); count != 3 {
		t.Fatalf("strict specialist budget = %d, want 3", count)
	}
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 1
	config.Optimize.TopPerSet = 1
	plan := searchPlan{requestedMode: "beta", solverMode: "objective", approximate: true}
	selected, err := (OptimizerService{}).selectModuleCandidates(pool, config, plan, map[string]int{"H_2": 3})
	if err != nil {
		t.Fatal(err)
	}
	ids := candidateIDs(selected.selected)
	for _, id := range []string{"x", "y", "z"} {
		if !ids[id] {
			t.Fatalf("strict specialist %s was removed: %#v", id, selected.selected)
		}
	}
	grid, err := optimizer.NewGrid(6, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	shapes := optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{"H_2": {ID: "H_2", Cells: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}}}}}
	sets := optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{"test": {ID: "test", InventorySetID: "test", RequiredGeometries: []string{"H_2"}}}}
	cartridges := []nte.Cartridge{{LocalID: "cartridge", SetID: "test"}}
	character := scoring.Character{BaseStats: baseline}
	full := optimizer.NewExactObjectiveEvaluator(sets, cartridges, nil, pool.eligible, character, nil, []optimizer.ObjectiveGoal{goal}, nil)
	oracle, err := (optimizer.SearchSolver{Catalog: shapes, Exact: true, KeepBest: 1}).SolveWithBonus(context.Background(), grid, pool.eligible, full)
	if err != nil {
		t.Fatal(err)
	}
	retained := optimizer.NewExactObjectiveEvaluator(sets, cartridges, nil, selected.selected, character, nil, []optimizer.ObjectiveGoal{goal}, nil)
	got, err := (optimizer.SearchSolver{Catalog: shapes, Exact: true, KeepBest: 1}).SolveWithBonus(context.Background(), grid, selected.selected, retained)
	if err != nil {
		t.Fatal(err)
	}
	if len(oracle.Placements) != 3 || len(got.Placements) != 3 || got.Score != oracle.Score {
		t.Fatalf("strict search differs from full-pool oracle: got %#v, want %#v", got, oracle)
	}
	for _, placement := range got.Placements {
		if !strings.Contains("xyz", placement.ModuleID) {
			t.Fatalf("strict build contains a non-specialist: %#v", got.Placements)
		}
	}
}

func TestGeometryModuleSlotsUsesPlayableCellsAndShapeSize(t *testing.T) {
	grid := optimizer.GridDefinition{Playable: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}, {X: 4, Y: 0}, {X: 5, Y: 0}}}
	shapes := optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{"H_2": {Cells: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}}}, "H_3": {Cells: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}}}}}
	candidates := []optimizer.Candidate{{Module: nte.Module{Geometry: "H_2"}}, {Module: nte.Module{Geometry: "H_3"}}}
	limits := geometryModuleSlots(grid, shapes, candidates)
	if limits["H_2"] != 3 || limits["H_3"] != 2 {
		t.Fatalf("unexpected geometry capacity: %#v", limits)
	}
}
