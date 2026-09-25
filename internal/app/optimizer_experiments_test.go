package app

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

func TestApplyExperimentOverrideMergesPartialSettingsWithoutMutatingBase(t *testing.T) {
	base := SavedOptimizationRequest{
		Profile: ProfileSummary{ID: "zankou", CharacterID: 1036, Name: "Zankou"},
		Weights: WeightOverrides{MainStats: []string{"CritDamageBase", "MagBase"}, Weights: map[string]float64{"AtkUp": .55, "CritBase": .85, "CritDamageBase": 1, "DamageUpGeneralBase": .7, "MagBase": .85}, ArcForkID: "fork_DemonBlade"},
		Goals:   map[string]GoalTuning{"CritBase": {Target: .6, Tolerance: .05, Importance: .85}, "MagBase": {Target: 200, Minimum: 180, StrictMinimum: true, Tolerance: .05, Importance: .85}},
	}
	newTarget, newMinimum, maxValue, tolerance := 215.0, 205.0, 280.0, .03
	arc := "none"
	mainStats := []string{"CritDamageBase", "DamageUpIncantationBase"}
	variant := ExperimentDefinition{Name: "crit-and-incantation", Method: "beta", ArcForkID: &arc, MainStats: &mainStats,
		Weights: map[string]float64{"CritBase": 1.2, "DamageUpGeneralBase": .9},
		Goals:   map[string]ExperimentGoalOverride{"MagBase": {Target: &newTarget, Minimum: &newMinimum, Maximum: &maxValue, Tolerance: &tolerance}},
	}
	effective, err := ApplyExperimentOverride(base, variant)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Weights.Weights["CritBase"] != 1.2 || effective.Weights.Weights["DamageUpGeneralBase"] != .9 || effective.Weights.Weights["AtkUp"] != .55 {
		t.Fatalf("partial weights did not overlay saved defaults: %#v", effective.Weights.Weights)
	}
	if effective.Weights.ArcForkID != NoArcForkSelection || strings.Join(effective.Weights.MainStats, ",") != "CritDamageBase,DamageUpIncantationBase" {
		t.Fatalf("main stat or explicit no-Arc override was not applied: %#v", effective.Weights)
	}
	goal := effective.Goals["MagBase"]
	if goal.Target != newTarget || goal.Minimum != newMinimum || goal.Maximum != maxValue || goal.Tolerance != tolerance || !goal.StrictMinimum {
		t.Fatalf("goal constraints were not overlaid: %+v", goal)
	}
	if base.Weights.Weights["CritBase"] != .85 || base.Weights.ArcForkID != "fork_DemonBlade" || base.Goals["MagBase"].Target != 200 || len(base.Weights.MainStats) != 2 {
		t.Fatalf("base saved settings were mutated: %+v", base)
	}
}

func TestApplyExperimentOverrideAddsCustomGoalsDisablesGoalsAndRejectsBadSettings(t *testing.T) {
	base := SavedOptimizationRequest{
		Profile: ProfileSummary{ID: "zankou", CharacterID: 1036},
		Weights: WeightOverrides{MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"CritBase": .85, "DamageUpFireBase": .5}},
		Goals:   map[string]GoalTuning{"CritBase": {Target: .6, Tolerance: .05}, "MagBase": {Target: 200, Tolerance: .05}},
	}
	zero, customLabel := 0.0, "Fire damage"
	trueValue := true
	effective, err := ApplyExperimentOverride(base, ExperimentDefinition{Name: "custom", Goals: map[string]ExperimentGoalOverride{
		"CritBase":         {Disabled: &trueValue},
		"DamageUpFireBase": {Target: &zero, Label: &customLabel, Percent: &trueValue},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := effective.Goals["CritBase"]; ok {
		t.Fatal("disabled goal remained active")
	}
	custom, ok := effective.Goals["DamageUpFireBase"]
	if !ok || !custom.Custom || custom.Target != 0 || custom.Label != customLabel || !custom.Percent {
		t.Fatalf("custom zero-valued goal was not added correctly: %+v", custom)
	}
	if _, err := ApplyExperimentOverride(base, ExperimentDefinition{Name: "missing-target", Goals: map[string]ExperimentGoalOverride{"UnknownGoal": {}}}); err == nil {
		t.Fatal("custom goal without a target was accepted")
	}
	falseValue := false
	if _, err := ApplyExperimentOverride(base, ExperimentDefinition{Name: "not-custom", Goals: map[string]ExperimentGoalOverride{"UnknownGoal": {Target: &zero, Custom: &falseValue}}}); err == nil {
		t.Fatal("non-custom unknown goal was accepted")
	}
	if _, err := ApplyExperimentOverride(base, ExperimentDefinition{Name: "invalid-weight", Weights: map[string]float64{"CritBase": 11}}); err == nil {
		t.Fatal("invalid experiment weight was accepted")
	}
	if _, err := ApplyExperimentOverride(base, ExperimentDefinition{Name: "empty-main-stats", MainStats: &[]string{}}); err == nil {
		t.Fatal("empty cartridge main-stat selection was accepted")
	}
}

func TestExperimentValidationBoundsRunsAndSearchLimits(t *testing.T) {
	plan := ExperimentPlan{SchemaVersion: ExperimentSchemaVersion, Variants: []ExperimentDefinition{{Name: "one"}}}
	if err := ValidateExperimentPlan(plan); err != nil {
		t.Fatal(err)
	}
	tooMany := ExperimentPlan{SchemaVersion: ExperimentSchemaVersion, Variants: make([]ExperimentDefinition, MaxExperimentVariants+1)}
	for index := range tooMany.Variants {
		tooMany.Variants[index].Name = "variant-" + string(rune('a'+index))
	}
	if err := ValidateExperimentPlan(tooMany); err == nil {
		t.Fatal("accepted an experiment plan above the run limit")
	}
	for _, invalid := range []ExperimentPlan{
		{SchemaVersion: 9, Variants: []ExperimentDefinition{{Name: "one"}}},
		{SchemaVersion: ExperimentSchemaVersion},
		{SchemaVersion: ExperimentSchemaVersion, Variants: []ExperimentDefinition{{Name: "same"}, {Name: "same"}}},
		{SchemaVersion: ExperimentSchemaVersion, Variants: []ExperimentDefinition{{Name: "one", Method: "exact"}}},
		{SchemaVersion: ExperimentSchemaVersion, Variants: []ExperimentDefinition{{Name: "one", Search: &SearchLimitsOverride{TimeoutSeconds: intPointer(0)}}}},
	} {
		if err := ValidateExperimentPlan(invalid); err == nil {
			t.Errorf("accepted invalid experiment plan %+v", invalid)
		}
	}
	zero := 0
	plan.Variants[0].Search = &SearchLimitsOverride{TopPerGeometry: &zero}
	if err := ValidateExperimentPlan(plan); err == nil {
		t.Fatal("accepted an invalid candidate limit")
	}
	if err := ValidateSearchLimits(SearchLimits{TopPerGeometry: 1, TopPerSet: 0, TimeoutSeconds: 1}); err != nil {
		t.Fatalf("rejected allowed lower limit bounds: %v", err)
	}
	for _, invalid := range []SearchLimits{{TopPerGeometry: 0, TopPerSet: 0, TimeoutSeconds: 1}, {TopPerGeometry: 1, TopPerSet: -1, TimeoutSeconds: 1}, {TopPerGeometry: 1, TopPerSet: 0, TimeoutSeconds: 86401}} {
		if err := ValidateSearchLimits(invalid); err == nil {
			t.Errorf("accepted invalid search limits %+v", invalid)
		}
	}
}

func TestRunSavedOptimizationExperimentsValidatesModeAndRequiresInventory(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	request := SavedOptimizationRequest{Profile: ProfileSummary{ID: "zankou", CharacterID: 1036}}
	plan := ExperimentPlan{SchemaVersion: ExperimentSchemaVersion, Variants: []ExperimentDefinition{{Name: "one"}}}
	if _, err := RunSavedOptimizationExperiments(context.Background(), service, t.TempDir(), "en", "exact", request, plan); err == nil || !strings.Contains(err.Error(), "invalid method") {
		t.Fatalf("invalid base method was not rejected: %v", err)
	}
	if _, err := RunSavedOptimizationExperiments(context.Background(), service, t.TempDir(), "en", "fast", request, plan); err == nil || !strings.Contains(err.Error(), "no account has been imported") {
		t.Fatalf("missing account snapshot was not rejected: %v", err)
	}
}

func TestReadExperimentPlanRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experiments.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"variants":[{"name":"base","weights":{"CritBase":1},"unexpected":true}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadExperimentPlan(path); err == nil {
		t.Fatal("unknown experiment setting was accepted")
	}
	if _, err := ReadExperimentPlan(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("missing experiment plan was accepted")
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"variants":[{"name":"base"}]} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadExperimentPlan(path); err == nil {
		t.Fatal("multiple top-level JSON values were accepted")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxExperimentFileBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadExperimentPlan(path); err == nil {
		t.Fatal("oversized experiment file was accepted")
	}
}

func TestZankouExamplePlanIsBoundedAndVariesOneWeightAcrossBothModes(t *testing.T) {
	plan, err := ReadExperimentPlan(filepath.Join("..", "testdata", "optimizer", "zankou_experiments.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Variants) != 12 {
		t.Fatalf("example plan has %d variants, want 12", len(plan.Variants))
	}
	paired := map[string]map[string]ExperimentDefinition{}
	for _, variant := range plan.Variants {
		base := strings.TrimSuffix(variant.Name, "-fast")
		base = strings.TrimSuffix(base, "-beta")
		mode := variant.Method
		if paired[base] == nil {
			paired[base] = map[string]ExperimentDefinition{}
		}
		paired[base][mode] = variant
		if variant.ArcForkID == nil || *variant.ArcForkID != "fork_DemonBlade" {
			t.Errorf("variant %q does not select Ravenous Blade", variant.Name)
		}
	}
	if len(paired) != 6 {
		t.Fatalf("example should have baseline plus five one-weight variants, got %d groups", len(paired))
	}
	for name, modes := range paired {
		if modes["fast"].Name == "" || modes["beta"].Name == "" {
			t.Errorf("variant pair %q does not include both search methods", name)
		}
		fast, beta := modes["fast"], modes["beta"]
		if len(fast.Weights) != len(beta.Weights) {
			t.Errorf("variant pair %q changes different weight keys between methods", name)
		}
		for property, value := range fast.Weights {
			if beta.Weights[property] != value {
				t.Errorf("variant pair %q uses different %s weights", name, property)
			}
		}
	}
}

func TestRunSavedOptimizationExperimentsSharesSnapshotAndDoesNotPersistOverrides(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	inventory := loadSanitizedInventoryFixture(t)
	for index := range inventory.Modules {
		if inventory.Modules[index].SetID == "synthetic-a" {
			inventory.Modules[index].SetID = "Suit4"
		} else {
			inventory.Modules[index].SetID = "Suit5"
		}
		if inventory.Modules[index].EquippedCharacterID == 41 {
			inventory.Modules[index].EquippedCharacterID = 1036
		}
	}
	for index := range inventory.Cartridges {
		if inventory.Cartridges[index].SetID == "synthetic-a" {
			inventory.Cartridges[index].SetID = "Suit4"
		} else {
			inventory.Cartridges[index].SetID = "Suit5"
		}
	}
	state := decoded.State{SchemaVersion: 2, Characters: []decoded.Character{{CharacterID: 1036, Name: "Zankou", Level: 80, AwakenLevel: 1}}}
	if err := writeAccountData(dir, inventory, state); err != nil {
		t.Fatal(err)
	}
	snapshotPath := workspaceFile(dir, "account_snapshot.json")
	snapshotBefore, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	request, err := PrepareSavedOptimization(service, dir, "", "zankou")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveProfileSettings(dir, "zankou", request.Weights); err != nil {
		t.Fatal(err)
	}
	request, err = PrepareSavedOptimization(service, dir, "", "zankou")
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := workspaceFile(dir, "strategy_overrides.json")
	settingsBefore, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	noArc := "none"
	plan := ExperimentPlan{SchemaVersion: ExperimentSchemaVersion, Variants: []ExperimentDefinition{
		{Name: "fast-default", ArcForkID: &noArc, Search: &SearchLimitsOverride{TopPerGeometry: intPointer(2), TimeoutSeconds: intPointer(15)}},
		{Name: "beta-crit", Method: "beta", ArcForkID: &noArc, Weights: map[string]float64{"CritBase": 1.15}},
	}}
	report, err := RunSavedOptimizationExperiments(context.Background(), service, dir, "en", "fast", request, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !report.SharedInputSnapshot || len(report.Variants) != 2 {
		t.Fatalf("variants did not share one input snapshot: %+v", report)
	}
	for _, variant := range report.Variants {
		if variant.Status == "" || variant.InventoryModules != len(inventory.Modules) || variant.EligibleCandidates == 0 {
			t.Fatalf("variant report is missing engine counts: %+v", variant)
		}
		if variant.Ranking == nil || variant.RankingScore == nil || *variant.RankingScore != variant.Ranking.Score || variant.SelectedCandidates <= 0 {
			t.Fatalf("report ranking/candidates do not match optimizer output fields: %+v", variant)
		}
	}
	if report.Variants[0].EffectiveConfig.Method != "fast" || report.Variants[0].EffectiveConfig.Search.TopPerGeometry != 2 || report.Variants[0].EffectiveConfig.Search.TimeoutSeconds != 15 {
		t.Fatalf("CLI default method or temporary search limits missing from first variant: %+v", report.Variants[0].EffectiveConfig)
	}
	for index, definition := range plan.Variants {
		effective, err := ApplyExperimentOverride(request, definition)
		if err != nil {
			t.Fatal(err)
		}
		directInventory, directState, loaded, err := loadAccountData(dir)
		if err != nil || !loaded {
			t.Fatalf("reload synthetic snapshot for direct engine comparison: loaded=%v err=%v", loaded, err)
		}
		directService := service
		directService.WeightOverrides = &effective.Weights
		directService.SearchLimitsOverride = definition.Search
		directService.ReservedModuleIDs, directService.ReservedCartridgeIDs, directService.ReservedArcIDs, err = higherPriorityReservationsWithSnapshot(dir, request.Profile.CharacterID, directInventory, directState)
		if err != nil {
			t.Fatal(err)
		}
		method := definition.Method
		if method == "" {
			method = "fast"
		}
		direct, err := directService.optimizeTunedWithArc(context.Background(), directInventory, request.Profile.ID, &directState, true, "en", method, goalTargets(effective.Goals), effective.Goals, effective.Weights.ArcForkID)
		if err != nil {
			t.Fatalf("direct %s optimizer run: %v", definition.Method, err)
		}
		reported := report.Variants[index]
		if reported.RankingScore == nil || *reported.RankingScore != direct.Solution.Ranking.Score || reported.SelectedCandidates != direct.SelectedCandidates || reported.VisitedStates != direct.Solution.Visited {
			t.Fatalf("variant report drifted from direct engine result for %s: report=%+v direct=%+v", definition.Name, reported, direct.Solution)
		}
		if len(reported.Modules) != len(direct.Modules) || reported.Set.ID != direct.Set.ID || reported.Stats.Derived["AtkFinal"] != direct.Stats.Derived["AtkFinal"] {
			t.Fatalf("reported build summary does not match direct engine result for %s", definition.Name)
		}
	}
	if report.Variants[0].EligibleCandidates != report.Variants[1].EligibleCandidates || report.Variants[0].InventoryModules != report.Variants[1].InventoryModules {
		t.Fatalf("variants saw different candidate inputs: first=%+v second=%+v", report.Variants[0], report.Variants[1])
	}
	if report.Variants[0].EffectiveConfig.Weights["CritBase"] == report.Variants[1].EffectiveConfig.Weights["CritBase"] {
		t.Fatal("weight override was not isolated to the selected variant")
	}
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	canceledReport, err := RunSavedOptimizationExperiments(canceledContext, service, dir, "en", "fast", request, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(canceledReport.Variants) != len(plan.Variants) || canceledReport.Variants[0].Status != "canceled" || canceledReport.Variants[1].Status != "canceled" {
		t.Fatalf("cancellation did not mark current and pending variants: %+v", canceledReport.Variants)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"synthetic-", "account_snapshot.json", dir} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("experiment report exposed private/local data marker %q", private)
		}
	}
	snapshotAfter, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshotBefore) != string(snapshotAfter) {
		t.Fatal("experiment runs modified the saved account snapshot")
	}
	settingsAfter, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(settingsBefore) != string(settingsAfter) {
		t.Fatal("experiment runs modified saved strategy settings")
	}
}

func intPointer(value int) *int { return &value }

func TestSummarizeExperimentVariantUsesEngineResultsAndSanitizesEquipmentIDs(t *testing.T) {
	result := OptimizationResult{
		Solution:         optimizer.Solution{Score: 999, Visited: 44, Complete: true, Ranking: &optimizer.RankingBreakdown{Score: 12, Objectives: 10, Equipment: 2}},
		InventoryModules: 7, EligibleCandidates: 6, SelectedCandidates: 4,
		Stats:        optimizer.StatSummary{Derived: map[string]float64{"BasicDamageIndex": 1120}},
		CurrentStats: &optimizer.StatSummary{Derived: map[string]float64{"BasicDamageIndex": 1000}},
		Set:          OptimizationSet{ID: "Suit4", Name: "Chaos"},
		Modules:      []OptimizationModule{{Module: nte.Module{LocalID: "private-module-id", SetID: "Suit4", Geometry: "H_2", Level: 15}}},
		Cartridge:    &nte.Cartridge{LocalID: "private-cartridge-id", SetID: "Suit4", Level: 10},
		Damage:       &DamageAnalysis{Status: "atomic_preview", Groups: []DamageGroup{{ID: "action-a", Name: "Action A", ActionType: "skill", BuildDamage: 1125, CurrentDamage: 1000, Instances: 1}, {ID: "scorch", Name: "Scorch", ActionType: "reaction", BuildDamage: 920, CurrentDamage: 1000, Instances: 1}}},
	}
	effective := SavedOptimizationRequest{Weights: WeightOverrides{MainStats: []string{"CritBase"}, Weights: map[string]float64{"CritBase": 1}}, Goals: map[string]GoalTuning{}}
	report := summarizeExperimentVariant("fixture", "beta", effective, result, 17)
	if report.RankingScore == nil || *report.RankingScore != result.Solution.Ranking.Score || report.VisitedStates != result.Solution.Visited || report.SelectedCandidates != result.SelectedCandidates || report.DurationMS != 17 {
		t.Fatalf("report fields drifted from engine result: %+v", report)
	}
	if len(report.ActionDamage) != 2 || report.ActionDamage[0].GainPercent == nil || math.Abs(*report.ActionDamage[0].GainPercent-12.5) > 1e-10 || report.ActionDamage[1].GainPercent == nil || math.Abs(*report.ActionDamage[1].GainPercent+8) > 1e-10 {
		t.Fatalf("per-action deltas do not reflect the engine's grouped values: %+v", report.ActionDamage)
	}
	if report.BasicDamage == nil || report.BasicDamage.BuildIndex != 1120 || report.BasicDamage.CurrentIndex != 1000 || report.BasicDamage.GainPercent == nil || math.Abs(*report.BasicDamage.GainPercent-12) > 1e-10 {
		t.Fatalf("basic damage comparison does not reflect engine stats: %+v", report.BasicDamage)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"basic_damage":{"build_index":1120,"current_index":1000,"gain_percent":12.`) {
		t.Fatalf("JSON report omits the Basic DMG comparison: %s", encoded)
	}
	for _, localID := range []string{"private-module-id", "private-cartridge-id"} {
		if strings.Contains(string(encoded), localID) {
			t.Fatalf("sanitized result includes local equipment ID %q", localID)
		}
	}
	withoutBreakdown := summarizeExperimentVariant("partial", "score", effective, OptimizationResult{Solution: optimizer.Solution{Score: 4, Complete: false}}, 9)
	if withoutBreakdown.RankingScore != nil || withoutBreakdown.Status != "approximate" || withoutBreakdown.Ranking != nil || withoutBreakdown.DamageStatus != "" || withoutBreakdown.BasicDamage != nil {
		t.Fatalf("fallback or approximate report fields are incorrect: %+v", withoutBreakdown)
	}
}

func TestCloneExperimentStateDoesNotShareMutableAccountSlicesAndMaps(t *testing.T) {
	name := "Zankou"
	state := decoded.State{
		Characters:     []decoded.Character{{CharacterID: 1036, Name: name, Skills: []decoded.CharacterSkill{{AbilityID: "skill", Level: 1}}}},
		Weapons:        []decoded.Weapon{{ForkID: "fork_DemonBlade", Name: "Ravenous Blade"}},
		PanelOverrides: map[int]map[string]float64{1036: {"CritBase": .1}},
	}
	copy := cloneExperimentState(state)
	copy.Characters[0].Name = "localized copy"
	copy.Characters[0].Skills[0].Level = 10
	copy.Weapons[0].Name = "localized Arc"
	copy.PanelOverrides[1036]["CritBase"] = .9
	if state.Characters[0].Name != name || state.Characters[0].Skills[0].Level != 1 || state.Weapons[0].Name != "Ravenous Blade" || state.PanelOverrides[1036]["CritBase"] != .1 {
		t.Fatalf("variant state copy mutated the shared snapshot: %+v", state)
	}
}

func TestExperimentGoalConfigPreservesExplicitZeroStrictFloor(t *testing.T) {
	configs := experimentGoalConfigs(map[string]GoalTuning{"CritBase": {Target: .6, Minimum: 0, StrictMinimum: true, Tolerance: .05}})
	goal, ok := configs["CritBase"]
	if !ok || !goal.StrictMinimum || goal.Minimum == nil || *goal.Minimum != 0 {
		t.Fatalf("effective config lost explicit zero strict floor: %+v", goal)
	}
	encoded, err := json.Marshal(goal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"minimum":0`) {
		t.Fatalf("zero floor not serialized in report config: %s", encoded)
	}
}
