package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

// OptimizerService is the UI-agnostic boundary shared by the CLI and Wails.
// It returns data structures and never writes files or logs by itself.
type WeightOverrides struct {
	MainStats []string                     `json:"main_stats"`
	Weights   map[string]float64           `json:"weights"`
	Goals     map[string]SavedGoalSettings `json:"goals,omitempty"`
	ArcForkID string                       `json:"arc_fork_id,omitempty"`
}

type SavedGoalSettings struct {
	Target        float64  `json:"target"`
	Label         string   `json:"label,omitempty"`
	Percent       bool     `json:"percent,omitempty"`
	Custom        bool     `json:"custom,omitempty"`
	Minimum       *float64 `json:"minimum,omitempty"`
	Maximum       float64  `json:"maximum,omitempty"`
	Tolerance     float64  `json:"tolerance"`
	StrictMinimum bool     `json:"strict_minimum,omitempty"`
	Disabled      bool     `json:"disabled,omitempty"`
}

type OptimizerService struct {
	WeightOverrides      *WeightOverrides
	Measure              bool
	DataDir              string
	Progress             func(optimizer.SearchProgress)
	ReservedModuleIDs    map[string]bool
	ReservedCartridgeIDs map[string]bool
	ReservedArcIDs       map[string]bool
	PinnedModuleIDs      map[string]bool
	ExcludedModuleIDs    map[string]bool
	catalogs             *optimizerCatalogCache
	baseStats            *characterBaseStatsCache
	blueprints           *optimizer.BlueprintCache
}

type optimizerDataConfig struct {
	Optimize struct {
		TopPerGeometry int `json:"top_per_geometry"`
		TopPerSet      int `json:"top_per_set"`
		TimeoutSeconds int `json:"timeout_seconds"`
	} `json:"optimize"`
}

type ProfileSummary struct {
	ID            string                       `json:"id"`
	CharacterID   int                          `json:"character_id"`
	Name          string                       `json:"name"`
	GridID        string                       `json:"grid_id"`
	PreferredSets []scoring.SetPreference      `json:"preferred_sets"`
	MainWeights   map[string]float64           `json:"main_weights"`
	SubWeights    map[string]float64           `json:"sub_weights"`
	MainStats     []string                     `json:"main_stats"`
	Weights       map[string]float64           `json:"weights"`
	Caps          map[string]scoring.StatCap   `json:"caps"`
	ConsoleTrait  *scoring.ConsoleTrait        `json:"console_trait,omitempty"`
	SavedGoals    map[string]SavedGoalSettings `json:"saved_goals,omitempty"`
	ArcForkID     string                       `json:"arc_fork_id,omitempty"`
}

type OptimizationResult struct {
	PhaseMS                        map[string]int64          `json:"phase_ms,omitempty"`
	SchemaVersion                  int                       `json:"schema_version"`
	ProfileID                      string                    `json:"profile_id"`
	Locale                         string                    `json:"locale"`
	OptimizationMode               string                    `json:"optimization_mode"`
	GridID                         string                    `json:"grid_id"`
	InventoryModules               int                       `json:"inventory_modules"`
	EligibleCandidates             int                       `json:"eligible_candidates"`
	SelectedCandidates             int                       `json:"selected_candidates"`
	TimeoutSeconds                 int                       `json:"timeout_seconds"`
	Grid                           optimizer.GridDefinition  `json:"grid"`
	Solution                       optimizer.Solution        `json:"solution"`
	Modules                        []OptimizationModule      `json:"modules"`
	Cartridge                      *nte.Cartridge            `json:"cartridge,omitempty"`
	CartridgeBreakdown             *scoring.Breakdown        `json:"cartridge_breakdown,omitempty"`
	Stats                          optimizer.StatSummary     `json:"stats"`
	Set                            OptimizationSet           `json:"set"`
	Target                         *target.BuildTarget       `json:"target,omitempty"`
	Goals                          []target.GoalProgress     `json:"goals,omitempty"`
	Character                      *decoded.Character        `json:"character,omitempty"`
	Weapon                         *decoded.Weapon           `json:"weapon,omitempty"`
	WeaponConditionalNote          string                    `json:"weapon_conditional_note,omitempty"`
	ExcludedEquipped               int                       `json:"excluded_equipped"`
	CurrentStats                   *optimizer.StatSummary    `json:"current_stats,omitempty"`
	CurrentGoals                   []target.GoalProgress     `json:"current_goals,omitempty"`
	ConditionalGoals               []target.GoalProgress     `json:"conditional_goals,omitempty"`
	CurrentConditionalGoals        []target.GoalProgress     `json:"current_conditional_goals,omitempty"`
	IncludeEquipped                bool                      `json:"include_equipped"`
	CartridgeEquippedCharacterName string                    `json:"cartridge_equipped_character_name,omitempty"`
	Alternatives                   []OptimizationAlternative `json:"alternatives,omitempty"`
	Damage                         *DamageAnalysis           `json:"damage,omitempty"`
}

type OptimizationAlternative struct {
	Solution              optimizer.Solution    `json:"solution"`
	Modules               []OptimizationModule  `json:"modules"`
	Weapon                *decoded.Weapon       `json:"weapon,omitempty"`
	WeaponConditionalNote string                `json:"weapon_conditional_note,omitempty"`
	Cartridge             *nte.Cartridge        `json:"cartridge,omitempty"`
	CartridgeBreakdown    *scoring.Breakdown    `json:"cartridge_breakdown,omitempty"`
	Stats                 optimizer.StatSummary `json:"stats"`
	Set                   OptimizationSet       `json:"set"`
	Goals                 []target.GoalProgress `json:"goals,omitempty"`
	ConditionalGoals      []target.GoalProgress `json:"conditional_goals,omitempty"`
	Damage                *DamageAnalysis       `json:"damage,omitempty"`
}

type OptimizationModule struct {
	Module                nte.Module        `json:"module"`
	Breakdown             scoring.Breakdown `json:"breakdown"`
	EquippedCharacterName string            `json:"equipped_character_name,omitempty"`
}

// OptimizationSet is the presentation-safe subset returned to the UI. Its
// name always comes from data/game/locales for the requested language.
type OptimizationSet struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	RequiredGeometries []string `json:"required_geometries"`
}

type GoalTuning struct {
	Target        float64 `json:"target"`
	Label         string  `json:"label,omitempty"`
	Percent       bool    `json:"percent,omitempty"`
	Custom        bool    `json:"custom,omitempty"`
	Minimum       float64 `json:"minimum,omitempty"`
	Maximum       float64 `json:"maximum,omitempty"`
	Tolerance     float64 `json:"tolerance"`
	Importance    float64 `json:"importance"`
	StrictMinimum bool    `json:"strict_minimum,omitempty"`
}

func (s OptimizerService) Profiles() ([]ProfileSummary, error) {
	characters, err := s.loadCharacters()
	if err != nil {
		return nil, err
	}
	result := make([]ProfileSummary, 0, len(characters))
	for id, profile := range characters {
		mainStats := append([]string(nil), profile.MainStats...)
		if len(mainStats) == 0 {
			mainStats = make([]string, 0, len(profile.MainWeights))
			for property, weight := range profile.MainWeights {
				if weight > 0 {
					mainStats = append(mainStats, property)
				}
			}
		}
		sort.Strings(mainStats)
		weights := cloneWeights(profile.Weights)
		if len(weights) == 0 {
			weights = cloneWeights(profile.SubWeights)
		}
		if len(weights) == 0 {
			weights = cloneWeights(profile.MainWeights)
		}
		result = append(result, ProfileSummary{ID: id, CharacterID: profile.CharacterID, Name: profile.Name, GridID: profile.GridID, PreferredSets: profile.PreferredSets, MainWeights: profile.MainWeights, SubWeights: profile.SubWeights, MainStats: mainStats, Weights: weights, Caps: profile.Caps, ConsoleTrait: profile.ConsoleTrait})
	}
	sortProfileSummaries(result)
	return result, nil
}

func (s OptimizerService) Target(profileID string) (target.BuildTarget, error) {
	characters, err := s.loadCharacters()
	if err != nil {
		return target.BuildTarget{}, err
	}
	targetID := profileID
	if profile, ok := characters[profileID]; ok && profile.TargetID != "" {
		targetID = profile.TargetID
	}
	return target.Read(datafiles.New(s.DataDir).Target(targetID))
}

func (s OptimizerService) Optimize(ctx context.Context, inv nte.Inventory, profileID string) (OptimizationResult, error) {
	return s.optimize(ctx, inv, profileID, nil, false, "fr", "score", nil)
}

func (s OptimizerService) optimize(ctx context.Context, inv nte.Inventory, profileID string, state *decoded.State, includeEquipped bool, language, mode string, goalOverrides map[string]float64) (OptimizationResult, error) {
	return s.optimizeTuned(ctx, inv, profileID, state, includeEquipped, language, mode, goalOverrides, nil)
}

func (s OptimizerService) optimizeTuned(ctx context.Context, inv nte.Inventory, profileID string, state *decoded.State, includeEquipped bool, language, mode string, goalOverrides map[string]float64, goalTunings map[string]GoalTuning) (OptimizationResult, error) {
	return s.optimizeTunedWithArc(ctx, inv, profileID, state, includeEquipped, language, mode, goalOverrides, goalTunings, "")
}

func (s OptimizerService) optimizeTunedWithArc(ctx context.Context, inv nte.Inventory, profileID string, state *decoded.State, includeEquipped bool, language, mode string, goalOverrides map[string]float64, goalTunings map[string]GoalTuning, arcForkID string) (OptimizationResult, error) {
	started := time.Now()
	language = ntelocale.Normalize(language)
	plan, err := parseSearchPlan(mode)
	if err != nil {
		return OptimizationResult{}, err
	}
	characters, err := s.loadCharacters()
	if err != nil {
		return OptimizationResult{}, err
	}
	profile, ok := characters[profileID]
	if !ok {
		return OptimizationResult{}, fmt.Errorf("unknown character profile %q", profileID)
	}
	catalogs, err := s.loadOptimizerCatalog()
	if err != nil {
		return OptimizationResult{}, err
	}
	displayCatalog, err := ntelocale.Load(s.DataDir, language)
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("load %s game labels: %w", language, err)
	}
	profile, err = applyWeightOverrides(profile, s.WeightOverrides)
	if err != nil {
		return OptimizationResult{}, err
	}
	refs := catalogs.references
	migrateLegacyDerivedStatWeights(&profile, refs)
	if err := scoring.ValidateCharacter(profileID, profile, refs); err != nil {
		return OptimizationResult{}, err
	}
	for _, preference := range profile.PreferredSets {
		if _, ok := catalogs.sets.Definitions[preference.SetID]; !ok {
			return OptimizationResult{}, fmt.Errorf("character profile %q references unknown set %q", profileID, preference.SetID)
		}
	}
	if profile.GridID == "" {
		return OptimizationResult{}, fmt.Errorf("character profile %q has no grid_id", profileID)
	}
	grid, err := catalogs.grids.Grid(profile.GridID)
	if err != nil {
		return OptimizationResult{}, err
	}
	shapeCatalog, setCatalog, cfg := catalogs.shapes, catalogs.sets, catalogs.config
	currentCharacter, currentWeapon, _, currentAdditional, characterNames, err := s.resolveAccountContext(state, profile, language)
	if err != nil {
		return OptimizationResult{}, err
	}
	arcScoreScale := optimizer.EquipmentTieBreakScale
	if plan.solverMode == "score" {
		arcScoreScale = 1
	}
	arcs, err := s.prepareArcSelection(state, profile, currentWeapon, currentAdditional, arcForkID, language, arcScoreScale, refs)
	if err != nil {
		return OptimizationResult{}, err
	}
	additional := arcs.additional[""]
	var arcOption *optimizer.ArcOption
	if arcs.option != nil {
		arcOption = arcs.option
		additional = arcOption.Additional
	}
	if currentCharacter != nil {
		baseStats, ok, err := s.characterBaseStats(currentCharacter.CharacterID, currentCharacter.Level, currentCharacter.BreakthroughLevel)
		if err != nil {
			return OptimizationResult{}, err
		}
		if ok {
			profile.BaseStats = baseStats
		}
	}
	var buildTarget *target.BuildTarget
	if loaded, readErr := s.Target(profileID); readErr == nil {
		appendCustomTargetGoals(&loaded, goalTunings)
		for i := range loaded.Goals {
			if value, ok := goalOverrides[loaded.Goals[i].PropertyID]; ok && value >= 0 {
				loaded.Goals[i].Minimum = value
			}
		}
		buildTarget = &loaded
	}
	objectives, requiredGeometry := prepareObjectives(plan.solverMode, buildTarget, goalTunings, s.WeightOverrides, profile, setCatalog)
	if mode == "beta" {
		if err := applyObjectiveScales(objectives, refs, profile.BaseStats); err != nil {
			return OptimizationResult{}, err
		}
	}
	selectionStarted := time.Now()
	pool := s.prepareModuleCandidates(inv.Modules, profile, refs, additional, objectives, requiredGeometry, includeEquipped)
	candidates, excludedEquipped := pool.eligible, pool.excludedEquipped
	selection, err := s.selectModuleCandidates(pool, cfg, plan, geometryModuleSlots(catalogs.grids.Definitions[profile.GridID], shapeCatalog, candidates))
	if err != nil {
		return OptimizationResult{}, err
	}
	selected := selection.selected
	selectionFinished := time.Now()
	cartridges, err := s.prepareCartridges(inv.Cartridges, profile, setCatalog, includeEquipped, plan, len(candidates))
	if err != nil {
		return OptimizationResult{}, err
	}
	availableCartridges := cartridges.available
	excludedEquipped += cartridges.excludedEquipped
	setup := s.prepareSearch(profile, refs, shapeCatalog, setCatalog, cfg, selected, availableCartridges, objectives, additional, plan, arcOption)
	var seed []optimizer.Placement
	if plan.approximate {
		seed = currentBuildSeed(grid, shapeCatalog, inv.Modules, profile.CharacterID, selected)
	}
	searchStarted := time.Now()
	execution, err := s.executeSearch(ctx, searchRequest{
		grid: grid, setup: setup, selected: selected, seed: seed,
	})
	searchFinished := time.Now()
	if err != nil {
		return OptimizationResult{}, err
	}
	selectedWeapon := arcs.weapons[execution.solution.SelectedWeaponID]
	var selectedWeaponPtr *decoded.Weapon
	if execution.solution.SelectedWeaponID != "" {
		copy := selectedWeapon
		selectedWeaponPtr = &copy
	}
	selectedAdditional := arcs.additional[execution.solution.SelectedWeaponID]
	if selectedAdditional == nil {
		selectedAdditional = additional
	}
	result := s.buildOptimizationResult(optimizationReportInput{
		inventory: inv, profileID: profileID, language: language, plan: plan, profile: profile, refs: refs,
		setCatalog: setCatalog, displayCatalog: displayCatalog, currentCharacter: currentCharacter, selectedWeapon: selectedWeaponPtr,
		weaponNote: arcs.notes[execution.solution.SelectedWeaponID], additional: selectedAdditional, currentAdditional: currentAdditional,
		arcWeapons: arcs.weapons, arcNotes: arcs.notes, arcAdditional: arcs.additional, characterNames: characterNames, buildTarget: buildTarget,
		objectives: objectives, candidates: candidates, selected: selected, solution: execution.solution,
		grid: catalogs.grids.Definitions[profile.GridID], timeoutSeconds: setup.timeoutSeconds, excludedEquipped: excludedEquipped,
		includeEquipped: includeEquipped,
		phaseMilliseconds: map[string]int64{
			"load_and_context": selectionStarted.Sub(started).Milliseconds(), "selection": selectionFinished.Sub(selectionStarted).Milliseconds(),
			"evaluator_and_seed": searchStarted.Sub(selectionFinished).Milliseconds(), "search": execution.searchMS,
		},
	})
	result.PhaseMS["reporting"] = time.Since(searchFinished).Milliseconds()
	return result, nil
}

func appendCustomTargetGoals(buildTarget *target.BuildTarget, tunings map[string]GoalTuning) {
	if buildTarget == nil || len(tunings) == 0 {
		return
	}
	known := make(map[string]bool, len(buildTarget.Goals))
	for _, goal := range buildTarget.Goals {
		known[goal.PropertyID] = true
	}
	customIDs := make([]string, 0, len(tunings))
	for propertyID, tuning := range tunings {
		if propertyID != "" && tuning.Custom && !known[propertyID] {
			customIDs = append(customIDs, propertyID)
		}
	}
	sort.Strings(customIDs)
	for _, propertyID := range customIDs {
		tuning := tunings[propertyID]
		buildTarget.Goals = append(buildTarget.Goals, target.Goal{
			PropertyID: propertyID,
			Label:      tuning.Label,
			Minimum:    tuning.Target,
			Percent:    tuning.Percent,
		})
	}
}

type alternativeBuildContext struct {
	moduleByID       map[string]nte.Module
	cartridges       []nte.Cartridge
	profile          scoring.Character
	refs             scoring.References
	characterNames   map[int]string
	setCatalog       optimizer.SetCatalog
	additional       map[string][]nte.Stat
	objectives       []optimizer.ObjectiveGoal
	mode             string
	approximate      bool
	buildTarget      *target.BuildTarget
	currentCharacter *decoded.Character
	currentStats     *optimizer.StatSummary
	language         string
	displayCatalog   ntelocale.Catalog
	arcWeapons       map[string]decoded.Weapon
	arcNotes         map[string]string
	arcAdditional    map[string]map[string][]nte.Stat
}

func (s OptimizerService) buildAlternatives(solutions []optimizer.Solution, ctx alternativeBuildContext) []OptimizationAlternative {
	result := make([]OptimizationAlternative, 0, len(solutions))
	for _, solution := range solutions {
		solution.Alternatives = nil
		if ctx.approximate {
			solution.Complete = false
		}
		detailedModules, modules := buildOptimizationModules(solution.Placements, ctx.moduleByID, ctx.profile, ctx.refs, ctx.characterNames)
		cartridge, cartridgeBreakdown := findScoredCartridge(ctx.cartridges, solution.SelectedCartridgeID, ctx.profile, ctx.refs)
		setDefinition := ctx.setCatalog.Definitions[solution.SelectedSetID]
		additional := ctx.arcAdditional[solution.SelectedWeaponID]
		if additional == nil {
			additional = ctx.additional
		}
		normalizePublicScore(&solution, detailedModules, cartridgeBreakdown)
		stats := optimizer.BuildStatSummary(ctx.profile, modules, cartridge, setDefinition, solution.SetMatchedCount, additional)
		explainRanking(&solution, stats.Derived, ctx.objectives, ctx.mode)
		var weapon *decoded.Weapon
		if selected, exists := ctx.arcWeapons[solution.SelectedWeaponID]; exists {
			copy := selected
			weapon = &copy
		}
		alternative := OptimizationAlternative{
			Solution: solution, Modules: detailedModules, Weapon: weapon, WeaponConditionalNote: ctx.arcNotes[solution.SelectedWeaponID], Cartridge: cartridge,
			CartridgeBreakdown: cartridgeBreakdown, Stats: stats, Set: localizedSetDefinition(setDefinition, ctx.displayCatalog),
			Damage: s.localizedDamageAnalysis(ctx.currentCharacter, stats, ctx.currentStats, ctx.language),
		}
		if ctx.buildTarget != nil {
			alternative.Goals = target.Compare(*ctx.buildTarget, stats.Derived)
			alternative.ConditionalGoals = target.Compare(*ctx.buildTarget, stats.DerivedConditional)
		}
		result = append(result, alternative)
	}
	return result
}

func localizedSetDefinition(definition optimizer.SetDefinition, catalog ntelocale.Catalog) OptimizationSet {
	return OptimizationSet{
		ID:                 definition.ID,
		Name:               catalog.SetName(definition.ID, definition.ID),
		RequiredGeometries: definition.RequiredGeometries,
	}
}

func buildCurrentEquipmentStats(inv nte.Inventory, profile scoring.Character, setCatalog optimizer.SetCatalog, additional map[string][]nte.Stat) *optimizer.StatSummary {
	if profile.CharacterID == 0 {
		return nil
	}
	currentModules := make([]nte.Module, 0)
	for _, module := range inv.Modules {
		if module.EquippedCharacterID == profile.CharacterID {
			currentModules = append(currentModules, module)
		}
	}
	var currentCartridge *nte.Cartridge
	for i := range inv.Cartridges {
		if inv.Cartridges[i].EquippedCharacterID == profile.CharacterID {
			currentCartridge = &inv.Cartridges[i]
			break
		}
	}
	if len(currentModules) == 0 && currentCartridge == nil {
		return nil
	}
	currentSet := optimizer.SetDefinition{}
	if currentCartridge != nil {
		for _, definition := range setCatalog.Definitions {
			if definition.InventorySetID == currentCartridge.SetID {
				currentSet = definition
				break
			}
		}
	}
	matched := matchedRequiredGeometries(currentModules, currentSet.RequiredGeometries)
	value := optimizer.BuildStatSummary(profile, currentModules, currentCartridge, currentSet, matched, additional)
	return &value
}

func buildOptimizationModules(placements []optimizer.Placement, moduleByID map[string]nte.Module, profile scoring.Character, refs scoring.References, characterNames map[int]string) ([]OptimizationModule, []nte.Module) {
	detailed := make([]OptimizationModule, 0, len(placements))
	plain := make([]nte.Module, 0, len(placements))
	for _, placement := range placements {
		module := moduleByID[placement.ModuleID]
		plain = append(plain, module)
		detailed = append(detailed, OptimizationModule{
			Module:                module,
			Breakdown:             scoring.ScoreDetailed(module, profile, refs),
			EquippedCharacterName: characterNames[module.EquippedCharacterID],
		})
	}
	return detailed, plain
}

func findScoredCartridge(cartridges []nte.Cartridge, localID string, profile scoring.Character, refs scoring.References) (*nte.Cartridge, *scoring.Breakdown) {
	for i := range cartridges {
		if cartridges[i].LocalID != localID {
			continue
		}
		breakdown := scoring.ScoreCartridgeDetailed(cartridges[i], profile, refs)
		return &cartridges[i], &breakdown
	}
	return nil, nil
}

func objectivesStillMissing(baseline map[string]float64, objectives []optimizer.ObjectiveGoal) []optimizer.ObjectiveGoal {
	result := make([]optimizer.ObjectiveGoal, 0, len(objectives))
	for _, objective := range objectives {
		if objective.Minimum > 0 && baseline[objective.PropertyID] < objective.Minimum {
			result = append(result, objective)
		}
	}
	return result
}

func ownedSetGeometryRequirements(cartridges []nte.Cartridge, catalog optimizer.SetCatalog) [][]string {
	owned := make(map[string]bool, len(cartridges))
	for _, cartridge := range cartridges {
		owned[cartridge.SetID] = true
	}
	result := make([][]string, 0, len(owned))
	for _, definition := range catalog.Definitions {
		if owned[definition.InventorySetID] {
			result = append(result, append([]string(nil), definition.RequiredGeometries...))
		}
	}
	return result
}

// normalizePublicScore removes search-only bonuses (board occupancy, objective
// breakpoints and amplified set identity) from values exposed to the UI. Those
// bonuses order the search but are not comparable to human-readable equipment
// scores.
func normalizePublicScore(solution *optimizer.Solution, modules []OptimizationModule, cartridge *scoring.Breakdown) {
	solution.Ranking = &optimizer.RankingBreakdown{Score: solution.Score}
	moduleScore := 0.0
	for _, module := range modules {
		moduleScore += module.Breakdown.Total
	}
	solution.ModuleScore = moduleScore
	if cartridge != nil {
		solution.CartridgeScore = cartridge.Total
	}
	solution.Score = solution.ModuleScore + solution.CartridgeScore + solution.SetBonusScore + solution.WeaponScore
	solution.Ranking.Equipment = solution.Score
	solution.Ranking.Structure = solution.Ranking.Score - solution.Score
}

// Explain the search result using the same panel values and utility as the
// evaluator. The public ranking deliberately excludes search-only structural
// guidance so it remains readable and comparable across constraint changes.
func explainRanking(solution *optimizer.Solution, values map[string]float64, goals []optimizer.ObjectiveGoal, mode string) {
	if solution.Ranking == nil {
		return
	}
	if mode == "fast" || mode == "beta" || mode == "exact-objective" {
		for _, goal := range goals {
			if goal.Minimum <= 0 {
				continue
			}
			importance := goal.Importance
			if importance <= 0 && !goal.ExplicitWeight {
				importance = 1
			}
			points := optimizer.ObjectiveScore(values, []optimizer.ObjectiveGoal{goal})
			solution.Ranking.Contributions = append(solution.Ranking.Contributions, optimizer.RankingContribution{PropertyID: goal.PropertyID, Value: values[goal.PropertyID], Target: goal.Minimum, Importance: importance, Points: points, ScoreScale: goal.ScoreScale})
			solution.Ranking.Objectives += points
		}
		solution.Ranking.TieBreak = solution.Ranking.Equipment * optimizer.EquipmentTieBreakScale
	}
	solution.Ranking.Structure -= solution.Ranking.Objectives + solution.Ranking.TieBreak
	// Final panel objectives already include every selected piece and set
	// effect. Equipment relevance remains diagnostic and is only used as an
	// internal tie-breaker, otherwise the same stats would be rewarded twice.
	solution.Ranking.Score = solution.Ranking.Objectives + solution.Ranking.TieBreak
}

// Final panel goals use the strongest configured weight in their stat family.
func goalWeight(profile scoring.Character, property string) float64 {
	keys := []string{property}
	switch property {
	case "AtkFinal":
		keys = []string{"AtkUp", "AtkAdd"}
	case "HPFinal":
		keys = []string{"HPMaxUp", "HPMaxAdd"}
	case "DefFinal":
		keys = []string{"DefUp", "DefAdd"}
	}
	weight := 0.0
	for _, key := range keys {
		weight = max(weight, profile.Weights[key], profile.MainWeights[key], profile.SubWeights[key])
	}
	return weight
}

func cloneWeights(source map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(source))
	for property, weight := range source {
		result[property] = weight
	}
	return result
}

func cartridgeMainStatsAllowed(cartridge nte.Cartridge, selected []string) bool {
	if len(selected) == 0 || len(cartridge.MainStats) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(selected))
	for _, property := range selected {
		allowed[property] = true
	}
	for _, stat := range cartridge.MainStats {
		if !allowed[stat.PropertyID] {
			return false
		}
	}
	return true
}
