package app

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

type syntheticModeFixture struct {
	modules []nte.Module
	profile scoring.Character
	goal    optimizer.ObjectiveGoal
	refs    scoring.References
}

type syntheticModeRun struct {
	mode     string
	plan     searchPlan
	goals    []optimizer.ObjectiveGoal
	pool     moduleCandidatePool
	selected []optimizer.Candidate
	seed     []optimizer.Placement
	solution optimizer.Solution
}

type syntheticOracleResult struct {
	ids   []string
	score float64
	stats map[string]float64
}

func newSyntheticModeFixture(specialistRequired bool) syntheticModeFixture {
	newModule := func(id string, crit, intensity float64, equipped bool, column int) nte.Module {
		stats := []nte.Stat{{PropertyID: "CritBase", Value: crit, Percent: true}}
		if intensity > 0 {
			stats = append(stats, nte.Stat{PropertyID: "UnbalIntensityBase", Value: intensity})
		}
		module := nte.Module{
			LocalID: id, SetID: "synthetic-set", Geometry: "H_2", Area: 2,
			SubStats: stats,
		}
		if equipped {
			module.EquippedCharacterID = 41
			module.EquippedPlacement = &nte.Placement{Row: 0, Column: column}
		}
		return module
	}

	strongGain := 30.0
	currentGain := 25.0
	if specialistRequired {
		strongGain = 0
		currentGain = 0
	}
	modules := []nte.Module{
		newModule("a", .100, strongGain, false, 0),
		newModule("b", .090, strongGain, false, 0),
		newModule("c", .080, strongGain, false, 0),
		newModule("d", .070, strongGain, false, 0),
		newModule("e", .005, 70, false, 0),
		newModule("f", .004, 0, false, 0),
		newModule("g", .003, currentGain, true, 0),
		newModule("h", .002, currentGain, true, 2),
		newModule("i", .001, currentGain, true, 4),
	}
	return syntheticModeFixture{
		modules: modules,
		profile: scoring.Character{CharacterID: 41, BaseStats: map[string]float64{"UnbalIntensityBase": 160}, Weights: map[string]float64{"CritBase": 1}},
		goal: optimizer.ObjectiveGoal{
			PropertyID: "UnbalIntensityBase", Minimum: 120, Maximum: 260,
			StrictMinimum: true, StrictFloor: 230, Importance: 1,
		},
		refs: scoring.References{"CritBase": .01, "UnbalIntensityBase": 12},
	}
}

func setSyntheticModuleStat(fixture *syntheticModeFixture, moduleID, propertyID string, value float64) bool {
	for moduleIndex := range fixture.modules {
		if fixture.modules[moduleIndex].LocalID != moduleID {
			continue
		}
		for statIndex := range fixture.modules[moduleIndex].SubStats {
			if fixture.modules[moduleIndex].SubStats[statIndex].PropertyID == propertyID {
				fixture.modules[moduleIndex].SubStats[statIndex].Value = value
				return true
			}
		}
		fixture.modules[moduleIndex].SubStats = append(fixture.modules[moduleIndex].SubStats, nte.Stat{PropertyID: propertyID, Value: value})
		return true
	}
	return false
}

func syntheticSetCatalog() optimizer.SetCatalog {
	return optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{
		"synthetic-set": {ID: "synthetic-set", InventorySetID: "synthetic-set", RequiredGeometries: []string{"H_2"}},
	}}
}

func syntheticShapes() optimizer.ShapeCatalog {
	return optimizer.ShapeCatalog{Shapes: map[string]optimizer.Shape{
		"H_2": {ID: "H_2", Cells: []optimizer.Point{{X: 0, Y: 0}, {X: 1, Y: 0}}},
	}}
}

func runSyntheticMode(ctx context.Context, fixture syntheticModeFixture, mode string) (syntheticModeRun, error) {
	plan, err := parseSearchPlan(mode)
	if err != nil {
		return syntheticModeRun{}, err
	}
	goals := []optimizer.ObjectiveGoal{fixture.goal}
	if mode == "beta" {
		if err := applyObjectiveScales(goals, fixture.refs, fixture.profile.BaseStats); err != nil {
			return syntheticModeRun{}, err
		}
	}
	service := OptimizerService{}
	pool := service.prepareModuleCandidates(fixture.modules, fixture.profile, fixture.refs, nil, goals, nil, false)
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 2
	config.Optimize.TopPerSet = 2
	selection, err := service.selectModuleCandidates(pool, config, plan, map[string]int{"H_2": 3})
	if err != nil {
		return syntheticModeRun{}, err
	}

	grid, err := optimizer.NewGrid(6, 1, nil)
	if err != nil {
		return syntheticModeRun{}, err
	}
	sets := syntheticSetCatalog()
	cartridges := []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: "synthetic-set"}}
	setup := service.prepareSearch(fixture.profile, fixture.refs, syntheticShapes(), sets, config, selection.selected, cartridges, goals, nil, plan, nil)
	setup.timeoutSeconds = 0
	// Keep this small benchmark fixture as a direct exhaustive comparison of
	// candidate preselection, without pruning or evaluator caching.
	setup.solver.DisableBound = true
	setup.solver.DisableGroups = true
	setup.solver.DisableStatReduction = true
	seed := currentBuildSeed(grid, syntheticShapes(), fixture.modules, fixture.profile.CharacterID, selection.selected)
	execution, err := service.executeSearch(ctx, searchRequest{grid: grid, setup: setup, selected: selection.selected, seed: seed})
	if err != nil {
		return syntheticModeRun{
			mode: mode, plan: plan, goals: goals, pool: pool, selected: selection.selected, seed: seed,
		}, err
	}
	return syntheticModeRun{
		mode: mode, plan: plan, goals: goals, pool: pool, selected: selection.selected, seed: seed, solution: execution.solution,
	}, nil
}

// Every fixture candidate has the same two-cell shape on a 6x1 grid, so any
// three distinct modules tile the board. Enumerating triples is therefore a
// complete oracle for both layout and candidate choice in these fixtures.
func exhaustiveSyntheticOracle(fixture syntheticModeFixture, goals []optimizer.ObjectiveGoal, candidates []optimizer.Candidate) syntheticOracleResult {
	sets := syntheticSetCatalog()
	cartridge := nte.Cartridge{LocalID: "synthetic-cartridge", SetID: "synthetic-set"}
	evaluator := optimizer.NewExactObjectiveEvaluator(sets, []nte.Cartridge{cartridge}, nil, candidates, fixture.profile, fixture.refs, goals, nil)
	byID := make(map[string]optimizer.Candidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.Module.LocalID] = candidate
	}
	best := syntheticOracleResult{score: math.Inf(-1)}
	for first := 0; first < len(candidates); first++ {
		for second := first + 1; second < len(candidates); second++ {
			for third := second + 1; third < len(candidates); third++ {
				ids := []string{candidates[first].Module.LocalID, candidates[second].Module.LocalID, candidates[third].Module.LocalID}
				placements := []optimizer.Placement{{ModuleID: ids[0]}, {ModuleID: ids[1]}, {ModuleID: ids[2]}}
				bonus := evaluator.Evaluate(placements)
				if math.IsInf(bonus.Score, -1) {
					continue
				}
				score := bonus.Score
				modules := make([]nte.Module, 0, len(ids))
				for _, id := range ids {
					candidate := byID[id]
					score += candidate.Score
					modules = append(modules, candidate.Module)
				}
				if score > best.score {
					summary := optimizer.BuildStatSummary(fixture.profile, modules, &cartridge, sets.Definitions["synthetic-set"], 1)
					best = syntheticOracleResult{ids: sortedStrings(ids), score: score, stats: summary.Derived}
				}
			}
		}
	}
	return best
}

func exhaustiveTwoModuleOracle(t testing.TB, grid optimizer.Grid, shapes optimizer.ShapeCatalog, candidates []optimizer.Candidate, evaluator optimizer.GlobalBonusEvaluator) syntheticOracleResult {
	t.Helper()
	placementsByID := make(map[string][]optimizer.Placement, len(candidates))
	for _, candidate := range candidates {
		shape, err := shapes.Shape(candidate.Module.Geometry)
		if err != nil {
			t.Fatalf("shape for %s: %v", candidate.Module.LocalID, err)
		}
		placementsByID[candidate.Module.LocalID] = optimizer.GeneratePlacements(grid, candidate.Module.LocalID, shape)
	}

	best := syntheticOracleResult{score: math.Inf(-1)}
	for first := 0; first < len(candidates); first++ {
		for second := first + 1; second < len(candidates); second++ {
			for _, firstPlacement := range placementsByID[candidates[first].Module.LocalID] {
				occupied := grid.Clone()
				if err := occupied.Place(firstPlacement.Cells); err != nil {
					continue
				}
				for _, secondPlacement := range placementsByID[candidates[second].Module.LocalID] {
					layout := occupied.Clone()
					if err := layout.Place(secondPlacement.Cells); err != nil || layout.FreeCells() != 0 {
						continue
					}
					placements := []optimizer.Placement{firstPlacement, secondPlacement}
					bonus := evaluator.Evaluate(placements)
					if math.IsInf(bonus.Score, -1) {
						continue
					}
					score := candidates[first].Score + candidates[second].Score + bonus.Score
					if score > best.score {
						best = syntheticOracleResult{
							ids:   sortedStrings([]string{candidates[first].Module.LocalID, candidates[second].Module.LocalID}),
							score: score,
						}
					}
				}
			}
		}
	}
	if math.IsInf(best.score, -1) {
		t.Fatal("exhaustive oracle found no complete two-module build")
	}
	return best
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func solutionModuleIDs(solution optimizer.Solution) []string {
	ids := make([]string, 0, len(solution.Placements))
	for _, placement := range solution.Placements {
		ids = append(ids, placement.ModuleID)
	}
	return sortedStrings(ids)
}

func assertModeMatchesOracle(t testing.TB, fixture syntheticModeFixture, run syntheticModeRun) syntheticOracleResult {
	t.Helper()
	oracle := exhaustiveSyntheticOracle(fixture, run.goals, run.pool.eligible)
	gotIDs := solutionModuleIDs(run.solution)
	if len(gotIDs) != len(oracle.ids) {
		t.Fatalf("%s retained-search build %v differs from exhaustive build %v", run.mode, gotIDs, oracle.ids)
	}
	for index := range oracle.ids {
		if gotIDs[index] != oracle.ids[index] {
			t.Fatalf("%s retained-search build %v differs from exhaustive build %v", run.mode, gotIDs, oracle.ids)
		}
	}
	if math.Abs(run.solution.Score-oracle.score) > 1e-9 {
		t.Fatalf("%s score %.12f differs from exhaustive score %.12f", run.mode, run.solution.Score, oracle.score)
	}
	if !run.solution.Complete {
		t.Fatalf("%s exact search over the retained pool was incomplete", run.mode)
	}
	goal := fixture.goal
	value := oracle.stats[goal.PropertyID]
	floor := goal.StrictFloor
	if floor <= 0 {
		floor = goal.Minimum * (1 - goal.Tolerance)
	}
	if goal.StrictMinimum && value < floor-1e-9 {
		t.Fatalf("%s oracle build violates strict floor: %s=%g floor=%g", run.mode, goal.PropertyID, value, floor)
	}
	if goal.Maximum > 0 && value > goal.Maximum+1e-9 {
		t.Fatalf("%s oracle build violates strict maximum: %s=%g maximum=%g", run.mode, goal.PropertyID, value, goal.Maximum)
	}
	return oracle
}

func TestFastAndBetaMatchExhaustiveSyntheticOracle(t *testing.T) {
	fixture := newSyntheticModeFixture(false)
	fast, err := runSyntheticMode(context.Background(), fixture, "fast")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := runSyntheticMode(context.Background(), fixture, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(fast.pool.eligible) != 9 || len(fast.selected) != 7 || len(beta.selected) != 8 {
		t.Fatalf("unexpected candidate counts: eligible=%d fast=%d beta=%d", len(fast.pool.eligible), len(fast.selected), len(beta.selected))
	}
	if fast.goals[0].ScoreScale != 0 || beta.goals[0].ScoreScale != ObjectiveScaleFactor*fixture.refs[fixture.goal.PropertyID] {
		t.Fatalf("unexpected mode score scales: fast=%g beta=%g", fast.goals[0].ScoreScale, beta.goals[0].ScoreScale)
	}
	fastOracle := assertModeMatchesOracle(t, fixture, fast)
	betaOracle := assertModeMatchesOracle(t, fixture, beta)
	if fmt.Sprint(fastOracle.ids) != "[a b c]" || fmt.Sprint(betaOracle.ids) != "[a b c]" {
		t.Fatalf("unexpected synthetic optimum: fast=%v beta=%v", fastOracle.ids, betaOracle.ids)
	}

	currentIDs := []string{"g", "h", "i"}
	for _, run := range []syntheticModeRun{fast, beta} {
		selectedIDs := candidateIDs(run.selected)
		for _, id := range currentIDs {
			if !selectedIDs[id] {
				t.Errorf("%s discarded currently equipped module %q", run.mode, id)
			}
		}
		if got := solutionModuleIDs(optimizer.Solution{Placements: run.seed}); fmt.Sprint(got) != "[g h i]" {
			t.Errorf("%s did not reconstruct the complete equipped build seed: %v", run.mode, got)
		}
	}
	current := []nte.Module{fixture.modules[6], fixture.modules[7], fixture.modules[8]}
	currentStats := optimizer.BuildStatSummary(fixture.profile, current, nil, optimizer.SetDefinition{}, 0).Derived
	if currentStats[fixture.goal.PropertyID] != 235 {
		t.Fatalf("equipped build stat = %g, want 235", currentStats[fixture.goal.PropertyID])
	}
}

func TestFastAndBetaMatchExhaustiveOracleWithStrictMaximum(t *testing.T) {
	fixture := newSyntheticModeFixture(false)
	fixture.goal.StrictMinimum = false
	fixture.goal.StrictFloor = 0
	fixture.goal.Maximum = 245
	// Keep the lower-ranked alternative below the retained leaders so the
	// exhaustive optimum remains identifiable after mode-specific preselection.
	if !setSyntheticModuleStat(&fixture, "f", "CritBase", .001) {
		t.Fatal("synthetic module f is missing")
	}

	for _, mode := range []string{"fast", "beta"} {
		run, err := runSyntheticMode(context.Background(), fixture, mode)
		if err != nil {
			t.Fatalf("%s search: %v", mode, err)
		}
		oracle := assertModeMatchesOracle(t, fixture, run)
		if fmt.Sprint(oracle.ids) != "[a b g]" {
			t.Errorf("%s strict-maximum optimum = %v, want [a b g]", mode, oracle.ids)
		}
		if value := oracle.stats[fixture.goal.PropertyID]; value != fixture.goal.Maximum {
			t.Errorf("%s build stat = %g, want maximum boundary %g", mode, value, fixture.goal.Maximum)
		}
		if len(run.seed) != 3 {
			t.Errorf("%s search did not receive a complete equipped-build seed: %#v", mode, run.seed)
		}
	}
}

// Fast currently can prune every specialist needed by a strict floor when the
// soft target is already met. Beta's additional strict-specialist selection
// restores the exhaustive feasible build; this test records the known gap.
func TestBetaRetainsStrictBuildMissedByFastPreselection(t *testing.T) {
	fixture := newSyntheticModeFixture(true)
	fast, fastErr := runSyntheticMode(context.Background(), fixture, "fast")
	if fastErr == nil || !strings.Contains(fastErr.Error(), "no complete build satisfies") {
		t.Fatalf("Fast error = %v, want no feasible retained candidate build", fastErr)
	}
	beta, err := runSyntheticMode(context.Background(), fixture, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(fast.pool.eligible) != 9 || len(fast.selected) != 7 || len(beta.selected) != 8 {
		t.Fatalf("unexpected candidate counts: eligible=%d fast=%d beta=%d", len(fast.pool.eligible), len(fast.selected), len(beta.selected))
	}
	fastOracle := exhaustiveSyntheticOracle(fixture, fast.goals, fast.pool.eligible)
	if fmt.Sprint(fastOracle.ids) != "[a b e]" || fastOracle.stats[fixture.goal.PropertyID] != 230 {
		t.Fatalf("Fast full-pool oracle should contain the strict-feasible build: build=%v stats=%v", fastOracle.ids, fastOracle.stats)
	}
	oracle := assertModeMatchesOracle(t, fixture, beta)
	if fmt.Sprint(oracle.ids) != "[a b e]" || oracle.stats[fixture.goal.PropertyID] != 230 {
		t.Fatalf("unexpected strict-floor oracle: build=%v stats=%v", oracle.ids, oracle.stats)
	}
	if !candidateIDs(beta.selected)["e"] || candidateIDs(fast.selected)["e"] {
		t.Fatalf("strict specialist retention differs from expected mode behavior: fast=%v beta=%v", candidateIDs(fast.selected), candidateIDs(beta.selected))
	}
}

func TestBetaRetainsEnoughSpecialistsForMultiModuleStrictFloor(t *testing.T) {
	fixture := newSyntheticModeFixture(true)
	if !setSyntheticModuleStat(&fixture, "e", "UnbalIntensityBase", 35) || !setSyntheticModuleStat(&fixture, "f", "UnbalIntensityBase", 35) {
		t.Fatal("synthetic strict-floor specialists are missing")
	}

	fast, fastErr := runSyntheticMode(context.Background(), fixture, "fast")
	if fastErr == nil || !strings.Contains(fastErr.Error(), "no complete build satisfies") {
		t.Fatalf("Fast error = %v, want no feasible build after omitting both required specialists", fastErr)
	}
	beta, err := runSyntheticMode(context.Background(), fixture, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(beta.selected) != 9 || candidateIDs(fast.selected)["e"] || candidateIDs(fast.selected)["f"] {
		t.Fatalf("strict specialist retention counts differ: Fast=%d Beta=%d", len(fast.selected), len(beta.selected))
	}
	if !candidateIDs(beta.selected)["e"] || !candidateIDs(beta.selected)["f"] {
		t.Fatalf("Beta did not retain both strict-floor specialists: %#v", candidateIDs(beta.selected))
	}
	oracle := assertModeMatchesOracle(t, fixture, beta)
	if fmt.Sprint(oracle.ids) != "[a e f]" || oracle.stats[fixture.goal.PropertyID] != fixture.goal.StrictFloor {
		t.Fatalf("multi-specialist strict-floor optimum = %v, stats=%v", oracle.ids, oracle.stats)
	}
}

func TestFastAndBetaMatchOracleWhenToleranceDefinesStrictFloor(t *testing.T) {
	fixture := newSyntheticModeFixture(true)
	if !setSyntheticModuleStat(&fixture, "e", "UnbalIntensityBase", 35) || !setSyntheticModuleStat(&fixture, "f", "UnbalIntensityBase", 35) {
		t.Fatal("synthetic strict-floor specialists are missing")
	}
	fixture.goal.Minimum = 250
	fixture.goal.Tolerance = .08
	fixture.goal.StrictFloor = 0

	for _, mode := range []string{"fast", "beta"} {
		run, err := runSyntheticMode(context.Background(), fixture, mode)
		if err != nil {
			t.Fatalf("%s search: %v", mode, err)
		}
		oracle := assertModeMatchesOracle(t, fixture, run)
		if fmt.Sprint(oracle.ids) != "[a e f]" || oracle.stats[fixture.goal.PropertyID] != strictGoalFloor(fixture.goal) {
			t.Errorf("%s tolerance-derived strict-floor build = %v, stats=%v", mode, oracle.ids, oracle.stats)
		}
		retained := candidateIDs(run.selected)
		if len(run.selected) != 7 || !retained["e"] || !retained["f"] {
			t.Errorf("%s did not retain the two needed specialists while reducing candidates: retained=%d eligible=%d ids=%v", mode, len(run.selected), len(run.pool.eligible), retained)
		}
	}
}

func TestFastAndBetaMatchExhaustiveOracleOnSanitizedGeometryInventory(t *testing.T) {
	inventory := loadSanitizedInventoryFixture(t)
	shapes, err := optimizer.LoadShapeCatalog(filepath.Join("..", "..", "data", "game", "equipment", "shapes.json"))
	if err != nil {
		t.Fatal(err)
	}
	grid, err := optimizer.NewGrid(2, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	profile := scoring.Character{CharacterID: 41, Weights: map[string]float64{
		"CritBase": 1, "CritDamageBase": .8, "AtkUp": .6, "UnbalIntensityBase": .4,
	}}
	refs := scoring.References{
		"CritBase": .064, "CritDamageBase": .128, "AtkUp": .12, "UnbalIntensityBase": 12,
	}
	sets := optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{
		"synthetic-a": {ID: "synthetic-a", InventorySetID: "synthetic-a"},
		"synthetic-b": {ID: "synthetic-b", InventorySetID: "synthetic-b"},
	}}
	service := OptimizerService{}
	pool := service.prepareModuleCandidates(inventory.Modules, profile, refs, nil, nil, nil, true)
	if len(pool.eligible) != 24 {
		t.Fatalf("all-equipment fixture pool has %d modules, want 24", len(pool.eligible))
	}

	for _, mode := range []string{"fast", "beta"} {
		plan, err := parseSearchPlan(mode)
		if err != nil {
			t.Fatal(err)
		}
		config := optimizerDataConfig{}
		config.Optimize.TopPerGeometry = 1
		config.Optimize.TopPerSet = 2
		selection, err := service.selectModuleCandidates(pool, config, plan)
		if err != nil {
			t.Fatalf("%s candidate selection: %v", mode, err)
		}
		selectedIDs := candidateIDs(selection.selected)
		if !selectedIDs["synthetic-02-h2-a"] || len(selection.selected) != 23 {
			t.Fatalf("%s did not retain the equipped piece and reduce only the fifth H_3 candidate: selected=%d eligible=%d", mode, len(selection.selected), len(pool.eligible))
		}

		cartridgePool, err := service.prepareCartridges(inventory.Cartridges, profile, sets, true, plan, len(pool.eligible))
		if err != nil {
			t.Fatalf("%s cartridge preparation: %v", mode, err)
		}
		setup := service.prepareSearch(profile, refs, shapes, sets, config, selection.selected, cartridgePool.available, nil, nil, plan, nil)
		setup.timeoutSeconds = 0
		setup.solver.DisableBound = true
		setup.solver.DisableGroups = true
		setup.solver.DisableStatReduction = true
		seed := currentBuildSeed(grid, shapes, inventory.Modules, profile.CharacterID, selection.selected)
		run, err := service.executeSearch(context.Background(), searchRequest{grid: grid, setup: setup, selected: selection.selected, seed: seed})
		if err != nil {
			t.Fatalf("%s search: %v", mode, err)
		}
		if got := solutionModuleIDs(optimizer.Solution{Placements: seed}); fmt.Sprint(got) != "[synthetic-02-h2-a]" {
			t.Fatalf("%s did not pass the equipped module as a seed: %v", mode, got)
		}
		fullEvaluator := optimizer.NewExactObjectiveEvaluator(sets, cartridgePool.available, profile.PreferredSets, pool.eligible, profile, refs, nil, nil)
		oracle := exhaustiveTwoModuleOracle(t, grid, shapes, pool.eligible, fullEvaluator)
		if fmt.Sprint(oracle.ids) != "[synthetic-02-v2-a synthetic-02-v2-b]" {
			t.Fatalf("unexpected exhaustive mixed-geometry optimum: %v", oracle.ids)
		}
		if got := solutionModuleIDs(run.solution); fmt.Sprint(got) != fmt.Sprint(oracle.ids) {
			t.Errorf("%s build %v differs from exhaustive layout oracle %v", mode, got, oracle.ids)
		}
		if math.Abs(run.solution.Score-oracle.score) > 1e-9 || !run.solution.Complete {
			t.Errorf("%s result score/complete = %g/%v, exhaustive = %g", mode, run.solution.Score, run.solution.Complete, oracle.score)
		}
	}
}

func syntheticBenchmarkModules(count int) []nte.Module {
	modules := make([]nte.Module, count)
	for index := range modules {
		crit := float64((index*17)%100+1) / 1000
		intensity := float64((index*7)%8) * 10
		module := nte.Module{
			LocalID: fmt.Sprintf("synthetic-%03d", index), SetID: "synthetic-set", Geometry: "H_2", Area: 2,
			SubStats: []nte.Stat{{PropertyID: "CritBase", Value: crit, Percent: true}},
		}
		if intensity > 0 {
			module.SubStats = append(module.SubStats, nte.Stat{PropertyID: "UnbalIntensityBase", Value: intensity})
		}
		if index >= count-3 {
			module.EquippedCharacterID = 41
			module.EquippedPlacement = &nte.Placement{Row: 0, Column: (index - (count - 3)) * 2}
		}
		modules[index] = module
	}
	return modules
}

func BenchmarkSyntheticFastBetaCandidateSearch(b *testing.B) {
	fixture := newSyntheticModeFixture(false)
	fixture.modules = syntheticBenchmarkModules(256)
	fixture.goal.Maximum = 0
	for _, mode := range []string{"fast", "beta"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				run, err := runSyntheticMode(context.Background(), fixture, mode)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(run.pool.eligible)), "eligible/op")
				b.ReportMetric(float64(len(run.selected)), "selected/op")
				b.ReportMetric(float64(run.solution.Visited), "leaves/op")
			}
		})
	}
}
