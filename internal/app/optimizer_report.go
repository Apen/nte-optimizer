package app

import (
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

type optimizationReportInput struct {
	inventory         nte.Inventory
	profileID         string
	language          string
	plan              searchPlan
	profile           scoring.Character
	refs              scoring.References
	setCatalog        optimizer.SetCatalog
	displayCatalog    ntelocale.Catalog
	currentCharacter  *decoded.Character
	currentWeapon     *decoded.Weapon
	weaponNote        string
	additional        map[string][]nte.Stat
	characterNames    map[int]string
	buildTarget       *target.BuildTarget
	objectives        []optimizer.ObjectiveGoal
	candidates        []optimizer.Candidate
	selected          []optimizer.Candidate
	solution          optimizer.Solution
	grid              optimizer.GridDefinition
	timeoutSeconds    int
	excludedEquipped  int
	includeEquipped   bool
	phaseMilliseconds map[string]int64
}

func (s OptimizerService) buildOptimizationResult(input optimizationReportInput) OptimizationResult {
	moduleByID := make(map[string]nte.Module, len(input.candidates))
	for _, candidate := range input.candidates {
		moduleByID[candidate.Module.LocalID] = candidate.Module
	}
	optimizedModules, selectedModules := buildOptimizationModules(input.solution.Placements, moduleByID, input.profile, input.refs, input.characterNames)
	selectedCartridge, cartridgeBreakdown := findScoredCartridge(input.inventory.Cartridges, input.solution.SelectedCartridgeID, input.profile, input.refs)
	setDefinition := input.setCatalog.Definitions[input.solution.SelectedSetID]
	stats := optimizer.BuildStatSummary(input.profile, selectedModules, selectedCartridge, setDefinition, input.solution.SetMatchedCount, input.additional)
	displaySetDefinition := localizedSetDefinition(setDefinition, input.displayCatalog)
	mode := publicOptimizationMode(input.plan)
	if input.plan.approximate {
		input.solution.Complete = false
	}
	normalizePublicScore(&input.solution, optimizedModules, cartridgeBreakdown)
	explainRanking(&input.solution, stats.Derived, input.objectives, mode)
	currentStats := buildCurrentEquipmentStats(input.inventory, input.profile, input.setCatalog, input.additional)
	goals, currentGoals, conditionalGoals, currentConditionalGoals := compareBuildGoals(input.buildTarget, stats, currentStats)
	alternatives := s.buildAlternatives(input.solution.Alternatives, alternativeBuildContext{
		moduleByID: moduleByID, cartridges: input.inventory.Cartridges, profile: input.profile, refs: input.refs,
		characterNames: input.characterNames, setCatalog: input.setCatalog, additional: input.additional,
		objectives: input.objectives, mode: mode, approximate: input.plan.approximate, buildTarget: input.buildTarget,
		currentCharacter: input.currentCharacter, currentStats: currentStats, language: input.language, displayCatalog: input.displayCatalog,
	})
	input.solution.Alternatives = nil
	damageAnalysis := s.localizedDamageAnalysis(input.currentCharacter, stats, currentStats, input.language)
	result := OptimizationResult{
		SchemaVersion: 1, ProfileID: input.profileID, Locale: input.language, OptimizationMode: mode, GridID: input.profile.GridID,
		PhaseMS: input.phaseMilliseconds, InventoryModules: len(input.inventory.Modules), EligibleCandidates: len(input.candidates), SelectedCandidates: len(input.selected),
		TimeoutSeconds: input.timeoutSeconds, Grid: input.grid, Solution: input.solution, Modules: optimizedModules,
		Cartridge: selectedCartridge, CartridgeBreakdown: cartridgeBreakdown, Stats: stats, Set: displaySetDefinition,
		Target: input.buildTarget, Goals: goals,
		Character: input.currentCharacter, Weapon: input.currentWeapon, WeaponConditionalNote: input.weaponNote, ExcludedEquipped: input.excludedEquipped,
		CurrentStats: currentStats, CurrentGoals: currentGoals,
		ConditionalGoals: conditionalGoals, CurrentConditionalGoals: currentConditionalGoals,
		Alternatives: alternatives, Damage: damageAnalysis, IncludeEquipped: input.includeEquipped,
	}
	if selectedCartridge != nil {
		result.CartridgeEquippedCharacterName = input.characterNames[selectedCartridge.EquippedCharacterID]
	}
	return result
}

func publicOptimizationMode(plan searchPlan) string {
	if plan.compromise {
		return "compromise"
	}
	if plan.approximate {
		return "fast-" + plan.solverMode
	}
	return plan.solverMode
}

func compareBuildGoals(buildTarget *target.BuildTarget, stats optimizer.StatSummary, currentStats *optimizer.StatSummary) (goals, currentGoals, conditionalGoals, currentConditionalGoals []target.GoalProgress) {
	if buildTarget == nil {
		return nil, nil, nil, nil
	}
	goals = target.Compare(*buildTarget, stats.Derived)
	conditionalGoals = target.Compare(*buildTarget, stats.DerivedConditional)
	if currentStats != nil {
		currentGoals = target.Compare(*buildTarget, currentStats.Derived)
		currentConditionalGoals = target.Compare(*buildTarget, currentStats.DerivedConditional)
	}
	return goals, currentGoals, conditionalGoals, currentConditionalGoals
}
