package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

const (
	ExperimentSchemaVersion = 1
	MaxExperimentVariants   = 16
	maxExperimentFileBytes  = 1 << 20
)

// ExperimentPlan is a one-shot overlay on prepared saved settings. Variants
// omit every setting they do not intend to change.
type ExperimentPlan struct {
	SchemaVersion int                    `json:"schema_version"`
	Variants      []ExperimentDefinition `json:"variants"`
}

type ExperimentDefinition struct {
	Name      string                            `json:"name"`
	Method    string                            `json:"method,omitempty"`
	ArcForkID *string                           `json:"arc_fork_id,omitempty"`
	MainStats *[]string                         `json:"main_stats,omitempty"`
	Weights   map[string]float64                `json:"weights,omitempty"`
	Goals     map[string]ExperimentGoalOverride `json:"goals,omitempty"`
	Search    *SearchLimitsOverride             `json:"search,omitempty"`
}

// ExperimentGoalOverride uses pointers so zero is distinguishable from absent.
type ExperimentGoalOverride struct {
	Target        *float64 `json:"target,omitempty"`
	Label         *string  `json:"label,omitempty"`
	Percent       *bool    `json:"percent,omitempty"`
	Custom        *bool    `json:"custom,omitempty"`
	Minimum       *float64 `json:"minimum,omitempty"`
	Maximum       *float64 `json:"maximum,omitempty"`
	Tolerance     *float64 `json:"tolerance,omitempty"`
	StrictMinimum *bool    `json:"strict_minimum,omitempty"`
	Disabled      *bool    `json:"disabled,omitempty"`
}

type ExperimentReport struct {
	SchemaVersion       int                       `json:"schema_version"`
	Profile             ExperimentProfile         `json:"profile"`
	SharedInputSnapshot bool                      `json:"shared_input_snapshot"`
	Interpretation      ExperimentInterpretation  `json:"interpretation"`
	Text                ExperimentReportText      `json:"-"`
	Variants            []ExperimentVariantReport `json:"variants"`
}

type ExperimentInterpretation struct {
	ActionDamageReference                string `json:"action_damage_reference"`
	DamageIsRotationDPS                  bool   `json:"damage_is_rotation_dps"`
	ValidatedRotationAvailable           bool   `json:"validated_rotation_available"`
	RankingScoresComparableAcrossMethods bool   `json:"ranking_scores_comparable_across_methods"`
}

type ExperimentReportText struct {
	Profile           string
	InputSnapshot     string
	Level             string
	SharedVariants    string
	Variant           string
	Method            string
	Status            string
	RetainedEligible  string
	Visited           string
	Duration          string
	Arc               string
	BasicDamage       string
	ActionDamageNote  string
	RotationNote      string
	ScoreScaleNote    string
	ScoresNotCompared string
}

type ExperimentProfile struct {
	ID          string `json:"id"`
	CharacterID int    `json:"character_id"`
	Name        string `json:"name"`
}

type ExperimentEffectiveConfig struct {
	Method    string                          `json:"method"`
	Arc       *ExperimentArc                  `json:"arc,omitempty"`
	MainStats []string                        `json:"main_stats"`
	Weights   map[string]float64              `json:"weights"`
	Goals     map[string]ExperimentGoalConfig `json:"goals"`
	Search    SearchLimits                    `json:"search"`
}

type ExperimentGoalConfig struct {
	Target        float64  `json:"target"`
	Label         string   `json:"label,omitempty"`
	Percent       bool     `json:"percent"`
	Custom        bool     `json:"custom"`
	Minimum       *float64 `json:"minimum"`
	Maximum       float64  `json:"maximum"`
	Tolerance     float64  `json:"tolerance"`
	Importance    float64  `json:"importance"`
	StrictMinimum bool     `json:"strict_minimum"`
}

type ExperimentArc struct {
	Name         string `json:"name"`
	Level        int    `json:"level"`
	Breakthrough int    `json:"breakthrough"`
	Star         int    `json:"star"`
}

type ExperimentEquipment struct {
	SetID     string     `json:"set_id"`
	Geometry  string     `json:"geometry,omitempty"`
	Area      int        `json:"area,omitempty"`
	Level     int        `json:"level"`
	MainStats []nte.Stat `json:"main_stats"`
	SubStats  []nte.Stat `json:"sub_stats"`
}

type ExperimentActionDamage struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ActionType    string   `json:"action_type"`
	BuildDamage   float64  `json:"build_damage"`
	CurrentDamage float64  `json:"current_damage"`
	GainPercent   *float64 `json:"gain_percent,omitempty"`
	Instances     int      `json:"instances"`
}

type ExperimentBasicDamage struct {
	BuildIndex   float64  `json:"build_index"`
	CurrentIndex float64  `json:"current_index"`
	GainPercent  *float64 `json:"gain_percent,omitempty"`
}

type ExperimentVariantReport struct {
	Name                  string                          `json:"name"`
	Status                string                          `json:"status"`
	FailureCode           string                          `json:"failure_code,omitempty"`
	SearchCompleted       bool                            `json:"search_completed"`
	DurationMS            int64                           `json:"duration_ms"`
	EffectiveConfig       ExperimentEffectiveConfig       `json:"effective_config"`
	InventoryModules      int                             `json:"inventory_modules"`
	EligibleCandidates    int                             `json:"eligible_candidates"`
	SelectedCandidates    int                             `json:"selected_candidates"`
	VisitedStates         uint64                          `json:"visited_states"`
	RankingScore          *float64                        `json:"ranking_score,omitempty"`
	SearchScoreComponents ExperimentSearchScoreComponents `json:"search_score_components"`
	Ranking               *optimizer.RankingBreakdown     `json:"ranking,omitempty"`
	Set                   OptimizationSet                 `json:"set"`
	Modules               []ExperimentEquipment           `json:"modules"`
	Cartridge             *ExperimentEquipment            `json:"cartridge,omitempty"`
	Stats                 optimizer.StatSummary           `json:"stats"`
	CurrentStats          *optimizer.StatSummary          `json:"current_stats,omitempty"`
	BasicDamage           *ExperimentBasicDamage          `json:"basic_damage,omitempty"`
	DamageStatus          string                          `json:"damage_status,omitempty"`
	ActionDamage          []ExperimentActionDamage        `json:"action_damage,omitempty"`
}

type ExperimentSearchScoreComponents struct {
	Modules   float64 `json:"modules"`
	SetBonus  float64 `json:"set_bonus"`
	Cartridge float64 `json:"cartridge"`
	Arc       float64 `json:"arc"`
}

// ReadExperimentPlan reads a small, strict JSON file and rejects unknown keys.
func ReadExperimentPlan(path string) (ExperimentPlan, error) {
	file, err := os.Open(path)
	if err != nil {
		return ExperimentPlan{}, fmt.Errorf("read experiment file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxExperimentFileBytes+1))
	if err != nil {
		return ExperimentPlan{}, fmt.Errorf("read experiment file: %w", err)
	}
	if len(data) > maxExperimentFileBytes {
		return ExperimentPlan{}, fmt.Errorf("experiment file exceeds %d bytes", maxExperimentFileBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plan ExperimentPlan
	if err := decoder.Decode(&plan); err != nil {
		return ExperimentPlan{}, fmt.Errorf("decode experiment file: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ExperimentPlan{}, err
	}
	if err := ValidateExperimentPlan(plan); err != nil {
		return ExperimentPlan{}, err
	}
	return plan, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("experiment file contains multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode experiment file: %w", err)
	}
	return nil
}

func ValidateExperimentPlan(plan ExperimentPlan) error {
	if plan.SchemaVersion != ExperimentSchemaVersion {
		return fmt.Errorf("unsupported experiment schema version %d", plan.SchemaVersion)
	}
	if len(plan.Variants) == 0 || len(plan.Variants) > MaxExperimentVariants {
		return fmt.Errorf("experiment plan must contain 1 to %d variants", MaxExperimentVariants)
	}
	seen := map[string]bool{}
	for _, variant := range plan.Variants {
		name := strings.TrimSpace(variant.Name)
		if name == "" || seen[name] {
			return fmt.Errorf("experiment variant names must be non-empty and unique: %q", variant.Name)
		}
		seen[name] = true
		if variant.Method != "" && variant.Method != "fast" && variant.Method != "beta" {
			return fmt.Errorf("variant %q has invalid method %q: choose fast or beta", name, variant.Method)
		}
		if variant.Search != nil {
			if err := validateSearchLimitOverride(*variant.Search); err != nil {
				return fmt.Errorf("variant %q: %w", name, err)
			}
		}
	}
	return nil
}

func ValidateSearchLimits(limits SearchLimits) error {
	if limits.TopPerGeometry < 1 || limits.TopPerGeometry > 10000 {
		return fmt.Errorf("top_per_geometry must be between 1 and 10000")
	}
	if limits.TopPerSet < 0 || limits.TopPerSet > 10000 {
		return fmt.Errorf("top_per_set must be between 0 and 10000")
	}
	if limits.TimeoutSeconds < 1 || limits.TimeoutSeconds > 86400 {
		return fmt.Errorf("timeout_seconds must be between 1 and 86400")
	}
	return nil
}

func validateSearchLimitOverride(limits SearchLimitsOverride) error {
	if limits.TopPerGeometry != nil && (*limits.TopPerGeometry < 1 || *limits.TopPerGeometry > 10000) {
		return fmt.Errorf("top_per_geometry must be between 1 and 10000")
	}
	if limits.TopPerSet != nil && (*limits.TopPerSet < 0 || *limits.TopPerSet > 10000) {
		return fmt.Errorf("top_per_set must be between 0 and 10000")
	}
	if limits.TimeoutSeconds != nil && (*limits.TimeoutSeconds < 1 || *limits.TimeoutSeconds > 86400) {
		return fmt.Errorf("timeout_seconds must be between 1 and 86400")
	}
	return nil
}

func applySearchLimitsOverride(limits *SearchLimits, override *SearchLimitsOverride) {
	if limits == nil || override == nil {
		return
	}
	if override.TopPerGeometry != nil {
		limits.TopPerGeometry = *override.TopPerGeometry
	}
	if override.TopPerSet != nil {
		limits.TopPerSet = *override.TopPerSet
	}
	if override.TimeoutSeconds != nil {
		limits.TimeoutSeconds = *override.TimeoutSeconds
	}
}

// RunSavedOptimizationExperiments loads account data once and runs every
// variant against that same in-memory inventory and decoded account snapshot.
func RunSavedOptimizationExperiments(ctx context.Context, service OptimizerService, stateDir, language, defaultMethod string, request SavedOptimizationRequest, plan ExperimentPlan) (ExperimentReport, error) {
	if defaultMethod != "fast" && defaultMethod != "beta" {
		return ExperimentReport{}, fmt.Errorf("invalid method %q: choose fast or beta", defaultMethod)
	}
	if err := ValidateExperimentPlan(plan); err != nil {
		return ExperimentReport{}, err
	}
	inventory, state, loaded, err := loadAccountData(stateDir)
	if err != nil {
		return ExperimentReport{}, err
	}
	if !loaded {
		return ExperimentReport{}, fmt.Errorf("no account has been imported")
	}
	var accountOverrides struct {
		Characters map[int]map[string]float64 `json:"characters"`
	}
	if loadedOverrides, readErr := readOptionalJSON(workspaceFile(stateDir, "account_overrides.json"), &accountOverrides); readErr != nil {
		return ExperimentReport{}, readErr
	} else if loadedOverrides {
		state.PanelOverrides = accountOverrides.Characters
	}
	service.ReservedModuleIDs, service.ReservedCartridgeIDs, service.ReservedArcIDs, err = higherPriorityReservationsWithSnapshot(stateDir, request.Profile.CharacterID, inventory, state)
	if err != nil {
		return ExperimentReport{}, err
	}
	catalogs, err := service.loadOptimizerCatalog()
	if err != nil {
		return ExperimentReport{}, err
	}
	presentation, err := ntelocale.LoadPresentation(service.DataDir, language)
	if err != nil {
		return ExperimentReport{}, fmt.Errorf("load experiment report labels: %w", err)
	}
	report := ExperimentReport{
		SchemaVersion:       ExperimentSchemaVersion,
		Profile:             ExperimentProfile{ID: request.Profile.ID, CharacterID: request.Profile.CharacterID, Name: request.Profile.Name},
		SharedInputSnapshot: true,
		Interpretation:      ExperimentInterpretation{ActionDamageReference: "currently_equipped_build", DamageIsRotationDPS: false, ValidatedRotationAvailable: false, RankingScoresComparableAcrossMethods: false},
		Text: ExperimentReportText{
			Profile: presentation.UI["experiment_profile"], InputSnapshot: presentation.UI["experiment_input_snapshot"], Level: presentation.UI["experiment_level"], SharedVariants: presentation.UI["experiment_shared_variants"],
			Variant: presentation.UI["experiment_variant"], Method: presentation.UI["experiment_method"], Status: presentation.UI["experiment_status"],
			RetainedEligible: presentation.UI["experiment_retained_eligible"], Visited: presentation.UI["experiment_visited"], Duration: presentation.UI["experiment_duration"], Arc: presentation.UI["experiment_arc"],
			BasicDamage:      presentation.UI["basic_damage"],
			ActionDamageNote: presentation.UI["experiment_action_damage_note"], RotationNote: presentation.UI["experiment_rotation_note"], ScoreScaleNote: presentation.UI["experiment_score_scale_note"], ScoresNotCompared: presentation.UI["experiment_scores_not_compared"],
		},
		Variants: make([]ExperimentVariantReport, 0, len(plan.Variants)),
	}
	for index, definition := range plan.Variants {
		if ctx.Err() != nil {
			for _, pending := range plan.Variants[index:] {
				report.Variants = append(report.Variants, ExperimentVariantReport{Name: pending.Name, Status: "canceled"})
			}
			break
		}
		effective, err := ApplyExperimentOverride(request, definition)
		if err != nil {
			return ExperimentReport{}, err
		}
		method := defaultMethod
		if definition.Method != "" {
			method = definition.Method
		}
		runService := service
		runService.WeightOverrides = &effective.Weights
		runService.SearchLimitsOverride = definition.Search
		started := time.Now()
		variantState := cloneExperimentState(state)
		result, runErr := runService.optimizeTunedWithArc(ctx, inventory, effective.Profile.ID, &variantState, true, language, method, goalTargets(effective.Goals), effective.Goals, effective.Weights.ArcForkID)
		elapsed := time.Since(started).Milliseconds()
		if runErr != nil {
			if ctx.Err() != nil {
				report.Variants = append(report.Variants, ExperimentVariantReport{Name: definition.Name, Status: "canceled", DurationMS: elapsed})
				for _, pending := range plan.Variants[index+1:] {
					report.Variants = append(report.Variants, ExperimentVariantReport{Name: pending.Name, Status: "canceled"})
				}
				break
			}
			if errors.Is(runErr, context.DeadlineExceeded) {
				limits := SearchLimits{TopPerGeometry: catalogs.config.Optimize.TopPerGeometry, TopPerSet: catalogs.config.Optimize.TopPerSet, TimeoutSeconds: catalogs.config.Optimize.TimeoutSeconds}
				applySearchLimitsOverride(&limits, definition.Search)
				report.Variants = append(report.Variants, ExperimentVariantReport{
					Name: definition.Name, Status: "timed_out", FailureCode: "deadline_before_valid_build", DurationMS: elapsed,
					EffectiveConfig: ExperimentEffectiveConfig{Method: method, MainStats: append([]string(nil), effective.Weights.MainStats...), Weights: cloneWeights(effective.Weights.Weights), Goals: experimentGoalConfigs(effective.Goals), Search: limits},
				})
				continue
			}
			return ExperimentReport{}, fmt.Errorf("experiment variant %q failed: %w", definition.Name, runErr)
		}
		report.Variants = append(report.Variants, summarizeExperimentVariant(definition.Name, method, effective, result, elapsed))
	}
	return report, nil
}

// ApplyExperimentOverride merges one variant into saved settings without
// changing either the request or files in the user's workspace.
func ApplyExperimentOverride(base SavedOptimizationRequest, variant ExperimentDefinition) (SavedOptimizationRequest, error) {
	effective := cloneSavedOptimizationRequest(base)
	effective.Weights.MainStats = append([]string(nil), base.Weights.MainStats...)
	effective.Weights.Weights = cloneWeights(base.Weights.Weights)
	effective.Weights.Goals = cloneSavedGoals(base.Weights.Goals)
	effective.Weights.ArcForkID = base.Weights.ArcForkID
	effective.Goals = cloneGoalTunings(base.Goals)
	if variant.MainStats != nil {
		effective.Weights.MainStats = append([]string(nil), (*variant.MainStats)...)
	}
	for property, weight := range variant.Weights {
		effective.Weights.Weights[property] = weight
	}
	if variant.ArcForkID != nil {
		effective.Weights.ArcForkID = *variant.ArcForkID
		if effective.Weights.ArcForkID == "none" {
			effective.Weights.ArcForkID = NoArcForkSelection
		}
	}
	for property, patch := range variant.Goals {
		if property == "" {
			return SavedOptimizationRequest{}, fmt.Errorf("variant %q has an empty goal property", variant.Name)
		}
		goal, exists := effective.Goals[property]
		if !exists {
			if patch.Target == nil {
				return SavedOptimizationRequest{}, fmt.Errorf("variant %q must set target when adding custom goal %q", variant.Name, property)
			}
			if patch.Custom != nil && !*patch.Custom {
				return SavedOptimizationRequest{}, fmt.Errorf("variant %q must mark new goal %q as custom", variant.Name, property)
			}
			goal = GoalTuning{Custom: true, Target: *patch.Target, Tolerance: .05}
		}
		if patch.Target != nil {
			goal.Target = *patch.Target
		}
		if patch.Label != nil {
			goal.Label = *patch.Label
		}
		if patch.Percent != nil {
			goal.Percent = *patch.Percent
		}
		if patch.Custom != nil {
			goal.Custom = *patch.Custom
		}
		if patch.Minimum != nil {
			goal.Minimum = *patch.Minimum
		}
		if patch.Maximum != nil {
			goal.Maximum = *patch.Maximum
		}
		if patch.Tolerance != nil {
			goal.Tolerance = *patch.Tolerance
		}
		if patch.StrictMinimum != nil {
			goal.StrictMinimum = *patch.StrictMinimum
		}
		if patch.Disabled != nil && *patch.Disabled {
			delete(effective.Goals, property)
			continue
		}
		effective.Goals[property] = goal
	}
	for property, goal := range effective.Goals {
		goal.Importance = savedGoalWeight(property, effective.Weights.Weights)
		effective.Goals[property] = goal
	}
	settings := effective.Weights
	settings.Goals = make(map[string]SavedGoalSettings, len(effective.Goals))
	for property, goal := range effective.Goals {
		minimum := goal.Minimum
		settings.Goals[property] = SavedGoalSettings{Target: goal.Target, Label: goal.Label, Percent: goal.Percent, Custom: goal.Custom, Minimum: &minimum, Maximum: goal.Maximum, Tolerance: goal.Tolerance, StrictMinimum: goal.StrictMinimum}
	}
	validated, err := ValidateProfileSettings(effective.Profile.ID, settings)
	if err != nil {
		return SavedOptimizationRequest{}, fmt.Errorf("variant %q: %w", variant.Name, err)
	}
	effective.Weights = validated
	return effective, nil
}

func cloneSavedOptimizationRequest(request SavedOptimizationRequest) SavedOptimizationRequest {
	request.Profile.MainStats = append([]string(nil), request.Profile.MainStats...)
	request.Profile.Weights = cloneWeights(request.Profile.Weights)
	return request
}

func cloneSavedGoals(source map[string]SavedGoalSettings) map[string]SavedGoalSettings {
	result := make(map[string]SavedGoalSettings, len(source))
	for key, goal := range source {
		if goal.Minimum != nil {
			value := *goal.Minimum
			goal.Minimum = &value
		}
		result[key] = goal
	}
	return result
}

func cloneGoalTunings(source map[string]GoalTuning) map[string]GoalTuning {
	result := make(map[string]GoalTuning, len(source))
	for key, goal := range source {
		result[key] = goal
	}
	return result
}

func experimentGoalConfigs(source map[string]GoalTuning) map[string]ExperimentGoalConfig {
	result := make(map[string]ExperimentGoalConfig, len(source))
	for property, goal := range source {
		minimum := goal.Minimum
		result[property] = ExperimentGoalConfig{
			Target: goal.Target, Label: goal.Label, Percent: goal.Percent, Custom: goal.Custom,
			Minimum: &minimum, Maximum: goal.Maximum, Tolerance: goal.Tolerance,
			Importance: goal.Importance, StrictMinimum: goal.StrictMinimum,
		}
	}
	return result
}

func cloneExperimentState(source decoded.State) decoded.State {
	copy := source
	copy.Characters = append([]decoded.Character(nil), source.Characters...)
	copy.Weapons = append([]decoded.Weapon(nil), source.Weapons...)
	copy.Resources = append([]decoded.Resource(nil), source.Resources...)
	if source.PanelOverrides != nil {
		copy.PanelOverrides = make(map[int]map[string]float64, len(source.PanelOverrides))
		for characterID, values := range source.PanelOverrides {
			copy.PanelOverrides[characterID] = cloneWeights(values)
		}
	}
	for index := range copy.Characters {
		if source.Characters[index].ForkNetID != nil {
			forkID := *source.Characters[index].ForkNetID
			copy.Characters[index].ForkNetID = &forkID
		}
		copy.Characters[index].Skills = append([]decoded.CharacterSkill(nil), source.Characters[index].Skills...)
	}
	return copy
}

func goalTargets(goals map[string]GoalTuning) map[string]float64 {
	result := make(map[string]float64, len(goals))
	for property, goal := range goals {
		result[property] = goal.Target
	}
	return result
}

func summarizeExperimentVariant(name, method string, effective SavedOptimizationRequest, result OptimizationResult, elapsed int64) ExperimentVariantReport {
	status := "complete"
	if !result.Solution.Complete || method == "fast" || method == "beta" {
		status = "approximate"
	}
	report := ExperimentVariantReport{
		Name: name, Status: status, SearchCompleted: result.Solution.Complete, DurationMS: elapsed,
		EffectiveConfig:  ExperimentEffectiveConfig{Method: method, MainStats: append([]string(nil), effective.Weights.MainStats...), Weights: cloneWeights(effective.Weights.Weights), Goals: experimentGoalConfigs(effective.Goals), Search: result.SearchLimits},
		InventoryModules: result.InventoryModules, EligibleCandidates: result.EligibleCandidates, SelectedCandidates: result.SelectedCandidates,
		VisitedStates:         result.Solution.Visited,
		SearchScoreComponents: ExperimentSearchScoreComponents{Modules: result.Solution.ModuleScore, SetBonus: result.Solution.SetBonusScore, Cartridge: result.Solution.CartridgeScore, Arc: result.Solution.WeaponScore},
		Set:                   result.Set, Stats: result.Stats, CurrentStats: result.CurrentStats, Modules: make([]ExperimentEquipment, 0, len(result.Modules)),
	}
	if result.Solution.Ranking != nil {
		copy := *result.Solution.Ranking
		report.Ranking = &copy
		rankingScore := copy.Score
		report.RankingScore = &rankingScore
	}
	if result.Weapon != nil {
		report.EffectiveConfig.Arc = &ExperimentArc{Name: result.Weapon.Name, Level: result.Weapon.Level, Breakthrough: result.Weapon.Breakthrough, Star: result.Weapon.Star}
	}
	if buildIndex, ok := result.Stats.Derived["BasicDamageIndex"]; ok && result.CurrentStats != nil {
		if currentIndex, currentOK := result.CurrentStats.Derived["BasicDamageIndex"]; currentOK {
			basicDamage := &ExperimentBasicDamage{BuildIndex: buildIndex, CurrentIndex: currentIndex}
			if currentIndex > 0 {
				gain := (buildIndex/currentIndex - 1) * 100
				if !math.IsNaN(gain) && !math.IsInf(gain, 0) {
					basicDamage.GainPercent = &gain
				}
			}
			report.BasicDamage = basicDamage
		}
	}
	for _, module := range result.Modules {
		report.Modules = append(report.Modules, ExperimentEquipment{SetID: module.Module.SetID, Geometry: module.Module.Geometry, Area: module.Module.Area, Level: module.Module.Level, MainStats: cloneStats(module.Module.MainStats), SubStats: cloneStats(module.Module.SubStats)})
	}
	if result.Cartridge != nil {
		report.Cartridge = &ExperimentEquipment{SetID: result.Cartridge.SetID, Level: result.Cartridge.Level, MainStats: cloneStats(result.Cartridge.MainStats), SubStats: cloneStats(result.Cartridge.SubStats)}
	}
	if result.Damage != nil {
		report.DamageStatus = result.Damage.Status
		for _, group := range result.Damage.Groups {
			action := ExperimentActionDamage{ID: group.ID, Name: group.Name, ActionType: group.ActionType, BuildDamage: group.BuildDamage, CurrentDamage: group.CurrentDamage, Instances: group.Instances}
			if group.CurrentDamage > 0 {
				gain := (group.BuildDamage/group.CurrentDamage - 1) * 100
				if !math.IsNaN(gain) && !math.IsInf(gain, 0) {
					action.GainPercent = &gain
				}
			}
			report.ActionDamage = append(report.ActionDamage, action)
		}
	}
	return report
}

func cloneStats(stats []nte.Stat) []nte.Stat { return append([]nte.Stat(nil), stats...) }
