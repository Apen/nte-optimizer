package app

import (
	"fmt"
	"math"
	"sort"
)

const strategyOverridesSchemaVersion = 1

type StrategyOverrides struct {
	SchemaVersion int                        `json:"schema_version"`
	Profiles      map[string]WeightOverrides `json:"profiles"`
}

func LoadStrategyOverrides(projectDir string) (StrategyOverrides, error) {
	result := StrategyOverrides{SchemaVersion: strategyOverridesSchemaVersion, Profiles: map[string]WeightOverrides{}}
	loaded, err := readOptionalJSON(workspaceFile(projectDir, "strategy_overrides.json"), &result)
	if err != nil {
		return StrategyOverrides{}, err
	}
	if !loaded {
		return result, nil
	}
	if result.SchemaVersion != strategyOverridesSchemaVersion {
		return StrategyOverrides{}, fmt.Errorf("unsupported strategy overrides schema version %d", result.SchemaVersion)
	}
	if result.Profiles == nil {
		result.Profiles = map[string]WeightOverrides{}
	}
	return result, nil
}

func SaveProfileStrategy(projectDir, profileID string, settings WeightOverrides) (WeightOverrides, error) {
	state, err := LoadStrategyOverrides(projectDir)
	if err != nil {
		return WeightOverrides{}, err
	}
	if previous, ok := state.Profiles[profileID]; ok {
		settings.Goals = previous.Goals
	}
	return saveProfileSettings(projectDir, profileID, settings, state)
}

func SaveProfileSettings(projectDir, profileID string, settings WeightOverrides) (WeightOverrides, error) {
	state, err := LoadStrategyOverrides(projectDir)
	if err != nil {
		return WeightOverrides{}, err
	}
	return saveProfileSettings(projectDir, profileID, settings, state)
}

func saveProfileSettings(projectDir, profileID string, settings WeightOverrides, state StrategyOverrides) (WeightOverrides, error) {
	if profileID == "" {
		return WeightOverrides{}, fmt.Errorf("profile id is required")
	}
	settings = normalizeWeightOverrides(settings)
	if len(settings.MainStats) == 0 {
		return WeightOverrides{}, fmt.Errorf("select at least one cartridge main stat")
	}
	for property, weight := range settings.Weights {
		if property == "" || math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 10 {
			return WeightOverrides{}, fmt.Errorf("invalid weight for %s: expected 0 to 10", property)
		}
	}
	for property, goal := range settings.Goals {
		if property == "" || !finiteNonNegative(goal.Target) || !finiteNonNegative(goal.Maximum) || math.IsNaN(goal.Tolerance) || math.IsInf(goal.Tolerance, 0) || goal.Tolerance < 0 || goal.Tolerance > .25 {
			return WeightOverrides{}, fmt.Errorf("invalid goal settings for %s", property)
		}
	}
	state.Profiles[profileID] = settings
	return settings, writeJSON(workspaceFile(projectDir, "strategy_overrides.json"), state)
}

func DeleteProfileStrategy(projectDir, profileID string) error {
	state, err := LoadStrategyOverrides(projectDir)
	if err != nil {
		return err
	}
	delete(state.Profiles, profileID)
	return writeJSON(workspaceFile(projectDir, "strategy_overrides.json"), state)
}

func normalizeWeightOverrides(settings WeightOverrides) WeightOverrides {
	seen := map[string]bool{}
	mainStats := make([]string, 0, len(settings.MainStats))
	for _, property := range settings.MainStats {
		if property != "" && !seen[property] {
			seen[property] = true
			mainStats = append(mainStats, property)
		}
	}
	sort.Strings(mainStats)
	goals := make(map[string]SavedGoalSettings, len(settings.Goals))
	for property, goal := range settings.Goals {
		goals[property] = goal
	}
	return WeightOverrides{MainStats: mainStats, Weights: cloneWeights(settings.Weights), Goals: goals}
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}
