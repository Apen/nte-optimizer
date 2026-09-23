package app

import (
	"context"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func TestSelectedStrategyRejectsOtherSetCartridges(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	inv := nte.Inventory{
		Modules:    []nte.Module{{LocalID: "piece", Geometry: "H_2", Area: 2}},
		Cartridges: []nte.Cartridge{{LocalID: "wrong", SetID: "diabolos"}},
	}
	_, err := service.optimize(context.Background(), inv, "zankou", nil, true, "fr", "fast", nil)
	if err == nil || !strings.Contains(err.Error(), "no cartridge is available for the selected strategy set") {
		t.Fatalf("wrong set accepted: %v", err)
	}
}

func TestOptimizerServiceProfilesAreStable(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	profiles, err := service.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) < 24 {
		t.Fatalf("got only %d profiles", len(profiles))
	}
	byID := make(map[string]ProfileSummary, len(profiles))
	for _, profile := range profiles {
		byID[profile.ID] = profile
	}
	if byID["lacrimosa"].CharacterID != 1004 || byID["lacrimosa"].Weights["CritBase"] != .85 || !slices.Contains(byID["lacrimosa"].MainStats, "CritBase") {
		t.Fatalf("Lacrimosa profile not exposed: %#v", byID["lacrimosa"])
	}
	if byID["zankou"].CharacterID != 1036 || byID["zankou"].Weights["CritDamageBase"] != 1 || byID["zankou_sub_dps"].Weights["UnbalIntensityBase"] != .55 {
		t.Fatalf("Zankou profile weights not exposed: %#v", byID)
	}
	if byID["zero"].CharacterID != 1046 {
		t.Fatalf("Zero profile not exposed: %#v", byID["zero"])
	}
	if byID["zero_1051"].CharacterID != 1051 || byID["zero_1051"].GridID != "character_1051" {
		t.Fatalf("female Zero profile not exposed: %#v", byID["zero_1051"])
	}
}

func TestPinnedModuleCannotOverrideReservation(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data"), ReservedModuleIDs: map[string]bool{"reserved": true}, PinnedModuleIDs: map[string]bool{"reserved": true}}
	_, err := service.optimize(context.Background(), nte.Inventory{Modules: []nte.Module{{LocalID: "reserved", Geometry: "H_2", Area: 2}}}, "zankou", nil, true, "fr", "fast", nil)
	if err == nil {
		t.Fatal("reservation overridden by lock")
	}
}

func TestNormalizePublicScoreRemovesInternalSearchBonuses(t *testing.T) {
	solution := optimizer.Solution{Score: 20595.8, ModuleScore: 15.24, CartridgeScore: 10.94, SetBonusScore: 1}
	modules := []OptimizationModule{{Breakdown: scoring.Breakdown{Total: 7}}, {Breakdown: scoring.Breakdown{Total: 8.24}}}
	cartridge := &scoring.Breakdown{Total: 10.94}
	normalizePublicScore(&solution, modules, cartridge)
	if solution.ModuleScore != 15.24 || solution.CartridgeScore != 10.94 || solution.Score != 27.18 {
		t.Fatalf("unexpected public score: %#v", solution)
	}
}

func TestObjectivesStillMissingSkipsAlreadyReachedBaseline(t *testing.T) {
	goals := []optimizer.ObjectiveGoal{{PropertyID: "CritBase", Minimum: .6}, {PropertyID: "AtkFinal", Minimum: 2000}}
	got := objectivesStillMissing(map[string]float64{"CritBase": .75, "AtkFinal": 1700}, goals)
	if len(got) != 1 || got[0].PropertyID != "AtkFinal" {
		t.Fatalf("unexpected remaining objectives: %#v", got)
	}
}

func TestOptimizerServiceLoadsCharacterPanelBase(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	stats, ok, err := service.characterBaseStats(1004, 80, 6)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || stats["HPMaxBase"] != 15998 || stats["AtkBase"] != 636 || stats["CritBase"] != .05 {
		t.Fatalf("unexpected Lacrimosa base stats: %#v", stats)
	}
}

func TestOptimizerServiceLocalizesEquippedWeapon(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	weaponID := decoded.NetID{Slot: 1, Serial: 2}
	state := &decoded.State{
		Characters: []decoded.Character{{CharacterID: 1036, ForkNetID: &weaponID}},
		Weapons:    []decoded.Weapon{{ID: weaponID, ForkID: "fork_DemonBlade"}},
	}
	result, err := service.optimize(context.Background(), nte.Inventory{}, "zankou", state, false, "en", "score", nil)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ntelocale.Load(service.DataDir, "en")
	if err != nil {
		t.Fatal(err)
	}
	expected := cleanArcName(catalog.ItemName("fork_DemonBlade", "fork_DemonBlade"))
	if result.Locale != "en" || result.Weapon == nil || result.Weapon.Name != expected {
		t.Fatalf("unexpected localized weapon: %#v", result.Weapon)
	}
}

func TestLocalizedSetDefinitionUsesRequestedGameLocale(t *testing.T) {
	definition := optimizer.SetDefinition{ID: "Suit4", RequiredGeometries: []string{"H_2"}}
	got := localizedSetDefinition(definition, ntelocale.Catalog{Sets: map[string]string{"Suit4": "Localized set"}})
	if got.ID != "Suit4" || got.Name != "Localized set" || len(got.RequiredGeometries) != 1 {
		t.Fatalf("unexpected localized set: %#v", got)
	}
}

func TestOptimizerServiceAppliesEditableGoalOverrides(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	result, err := service.optimize(context.Background(), nte.Inventory{}, "zankou", nil, false, "fr", "score", map[string]float64{"CritBase": .72})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, goal := range result.Goals {
		if goal.PropertyID == "CritBase" {
			found = true
			if goal.Minimum != .72 {
				t.Fatalf("minimum = %v", goal.Minimum)
			}
		}
	}
	if !found {
		t.Fatal("CritBase goal not returned")
	}
}

func TestOptimizerServiceReturnsStructuredResult(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	inv := nte.Inventory{Modules: []nte.Module{
		{LocalID: "module_1", Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "CritBase", Value: 0.064}}},
	}}
	result, err := service.Optimize(context.Background(), inv, "zankou")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProfileID != "zankou" || result.GridID != "zankou" || len(result.Solution.Placements) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestOptimizerServiceRequiresExplicitOptionForOtherCharactersEquipment(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	inv := nte.Inventory{Modules: []nte.Module{{LocalID: "free", Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .01}}}, {LocalID: "borrowed", Geometry: "H_2", EquippedCharacterID: 1004, SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .5}}}}}
	protected, err := service.optimize(context.Background(), inv, "zankou", nil, false, "fr", "score", nil)
	if err != nil {
		t.Fatal(err)
	}
	included, err := service.optimize(context.Background(), inv, "zankou", nil, true, "fr", "score", nil)
	if err != nil {
		t.Fatal(err)
	}
	if protected.ExcludedEquipped != 1 || protected.IncludeEquipped || len(protected.Modules) != 1 || protected.Modules[0].Module.LocalID != "free" {
		t.Fatalf("unexpected protected result: %#v", protected)
	}
	if included.ExcludedEquipped != 0 || !included.IncludeEquipped || len(included.Modules) != 2 || included.Modules[0].Module.LocalID != "borrowed" {
		t.Fatalf("unexpected included result: %#v", included)
	}
}

func TestOptimizerServiceExcludesHigherPriorityReservations(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data"), ReservedModuleIDs: map[string]bool{"reserved": true}}
	inv := nte.Inventory{Modules: []nte.Module{{LocalID: "available", Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .01}}}, {LocalID: "reserved", Geometry: "H_2", SubStats: []nte.Stat{{PropertyID: "CritBase", Value: .5}}}}}
	result, err := service.optimize(context.Background(), inv, "zankou", nil, true, "fr", "score", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Modules) != 1 || result.Modules[0].Module.LocalID != "available" || result.ExcludedEquipped != 1 {
		t.Fatalf("reserved module was not protected: %#v", result)
	}
}

func TestOptimizerRejectsUnsupportedSearchMode(t *testing.T) {
	for _, mode := range []string{"deep", "balanced", "fast-balanced", "damage-direct", "damage-dot", "damage-reaction", "unknown"} {
		service := OptimizerService{}
		_, err := service.optimize(context.Background(), nte.Inventory{}, "zankou", nil, false, "fr", mode, nil)
		if err == nil || !strings.Contains(err.Error(), "unknown search method") {
			t.Fatalf("mode %q: %v", mode, err)
		}
	}
}

func TestRankingExplainsObjectiveTradeoffWithoutChangingPublicScore(t *testing.T) {
	solution := optimizer.Solution{Score: 1040.5}
	modules := []OptimizationModule{{Breakdown: scoring.Breakdown{Total: 10}}}
	normalizePublicScore(&solution, modules, nil)
	goals := []optimizer.ObjectiveGoal{
		{PropertyID: "AtkFinal", Minimum: 100, Importance: 2},
		{PropertyID: "CritBase", Minimum: .5},
		{PropertyID: "Inactive", Minimum: 0},
	}
	explainRanking(&solution, map[string]float64{"AtkFinal": 100, "CritBase": .7}, goals, "fast")
	r := solution.Ranking
	if solution.Score != 10 || math.Abs(r.Score-(30.5+10*optimizer.EquipmentTieBreakScale)) > 1e-9 || r.Equipment != 10 || r.Objectives != 30.5 || math.Abs(r.TieBreak-10*optimizer.EquipmentTieBreakScale) > 1e-9 || math.Abs(r.Structure-(1000-r.TieBreak)) > 1e-9 {
		t.Fatalf("incorrect decomposition: public=%g ranking=%+v", solution.Score, r)
	}
	if len(r.Contributions) != 2 || r.Contributions[0].Points != 20 || r.Contributions[1].Points != 10.5 || r.Contributions[1].Importance != 1 {
		t.Fatalf("incorrect contributions: %+v", r.Contributions)
	}
}

func TestGoalWeightFollowsMainAndSubWeights(t *testing.T) {
	profile := scoring.Character{MainWeights: map[string]float64{"CritBase": 0, "AtkUp": .4}, SubWeights: map[string]float64{"CritBase": .8, "AtkAdd": .2}}
	if goalWeight(profile, "HPFinal") != 0 {
		t.Fatal("unweighted HP must not contribute")
	}
	if goalWeight(profile, "CritBase") != .8 {
		t.Fatal("secondary CRIT weight must remain active")
	}
	if goalWeight(profile, "AtkFinal") != .4 {
		t.Fatal("ATK goal must follow its strongest family weight")
	}
	profile.SubWeights["CritBase"] = 0
	if goalWeight(profile, "CritBase") != 0 {
		t.Fatal("zero weights must disable goal")
	}
}

func TestGoalWeightUsesGeneralWeights(t *testing.T) {
	profile := scoring.Character{Weights: map[string]float64{"MagBase": .85, "AtkUp": .55}}
	if goalWeight(profile, "MagBase") != .85 || goalWeight(profile, "AtkFinal") != .55 || goalWeight(profile, "HPFinal") != 0 {
		t.Fatalf("unexpected general goal weights")
	}
}

func TestCartridgeMainStatsAllowed(t *testing.T) {
	cartridge := nte.Cartridge{MainStats: []nte.Stat{{PropertyID: "MagBase"}}}
	if !cartridgeMainStatsAllowed(cartridge, []string{"CritDamageBase", "MagBase"}) {
		t.Fatal("selected main stat rejected")
	}
	if cartridgeMainStatsAllowed(cartridge, []string{"CritDamageBase"}) || cartridgeMainStatsAllowed(cartridge, nil) {
		t.Fatal("unselected main stat accepted")
	}
}

func TestWeightOverridesDoNotMutateCachedProfile(t *testing.T) {
	service := OptimizerService{DataDir: filepath.Join("..", "..", "data")}
	profiles, err := service.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	var original ProfileSummary
	for _, profile := range profiles {
		if profile.ID == "zankou" {
			original = profile
		}
	}
	service.WeightOverrides = &WeightOverrides{MainStats: []string{"CritDamageBase"}, Weights: map[string]float64{"CritDamageBase": .3}}
	result, err := service.optimize(context.Background(), nte.Inventory{}, "zankou", nil, true, "fr", "fast", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, contribution := range result.Solution.Ranking.Contributions {
		if contribution.PropertyID == "HPFinal" {
			t.Fatal("HP should not be scored")
		}
		if contribution.PropertyID == "CritDamageBase" && contribution.Importance != .3 {
			t.Fatal("custom weight not applied")
		}
	}
	after, err := service.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range after {
		if profile.ID == "zankou" && profile.MainWeights["CritDamageBase"] != original.MainWeights["CritDamageBase"] {
			t.Fatal("cached profile mutated")
		}
	}
}

func TestBuildCurrentEquipmentStatsUsesOnlyCharacterEquipment(t *testing.T) {
	profile := scoring.Character{CharacterID: 7}
	inv := nte.Inventory{Modules: []nte.Module{
		{LocalID: "current", EquippedCharacterID: 7, MainStats: []nte.Stat{{PropertyID: "AtkAdd", Value: 42}}},
		{LocalID: "other", EquippedCharacterID: 8, MainStats: []nte.Stat{{PropertyID: "AtkAdd", Value: 99}}},
	}}
	stats := buildCurrentEquipmentStats(inv, profile, optimizer.SetCatalog{}, nil)
	if stats == nil || stats.Sources["modules"]["AtkAdd"] != 42 {
		t.Fatalf("unexpected current equipment stats: %#v", stats)
	}
	if got := buildCurrentEquipmentStats(inv, scoring.Character{}, optimizer.SetCatalog{}, nil); got != nil {
		t.Fatalf("profile without character must not have current stats: %#v", got)
	}
}

func TestBuildAlternativesMaterializesAndFlattensSolutions(t *testing.T) {
	module := nte.Module{LocalID: "module-1", Geometry: "SINGLE", MainStats: []nte.Stat{{PropertyID: "AtkAdd", Value: 12}}}
	cartridge := nte.Cartridge{LocalID: "cartridge-1", SetID: "set-inventory"}
	solutions := []optimizer.Solution{{
		Complete: true, SelectedSetID: "set-1", SelectedCartridgeID: cartridge.LocalID,
		Placements:   []optimizer.Placement{{ModuleID: module.LocalID}},
		Alternatives: []optimizer.Solution{{Complete: true}},
	}}
	ctx := alternativeBuildContext{
		moduleByID: map[string]nte.Module{module.LocalID: module}, cartridges: []nte.Cartridge{cartridge},
		profile: scoring.Character{}, setCatalog: optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{
			"set-1": {ID: "set-1", InventorySetID: cartridge.SetID},
		}},
		approximate: true,
	}
	got := (OptimizerService{}).buildAlternatives(solutions, ctx)
	if len(got) != 1 || got[0].Solution.Complete || got[0].Solution.Alternatives != nil {
		t.Fatalf("alternative was not normalized: %#v", got)
	}
	if len(got[0].Modules) != 1 || got[0].Modules[0].Module.LocalID != module.LocalID || got[0].Cartridge == nil || got[0].Cartridge.LocalID != cartridge.LocalID {
		t.Fatalf("alternative equipment was not materialized: %#v", got[0])
	}
}
