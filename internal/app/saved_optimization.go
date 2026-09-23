package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// SavedOptimizationRequest contains the same persisted inputs used by the build planner.
type SavedOptimizationRequest struct {
	Profile ProfileSummary
	Weights WeightOverrides
	Goals   map[string]GoalTuning
}

// PrepareSavedOptimization loads profile defaults and user overrides without modifying them.
// A numeric character selects its first profile, matching the planner's initial selection.
func PrepareSavedOptimization(service OptimizerService, stateDir, character, profileID string) (SavedOptimizationRequest, error) {
	profiles, err := service.Profiles()
	if err != nil {
		return SavedOptimizationRequest{}, err
	}
	var selected *ProfileSummary
	if profileID != "" {
		for i := range profiles {
			if profiles[i].ID == profileID {
				selected = &profiles[i]
				break
			}
		}
		if selected == nil {
			return SavedOptimizationRequest{}, fmt.Errorf("unknown profile %q", profileID)
		}
	}
	if character != "" {
		characterID, parseErr := strconv.Atoi(character)
		for i := range profiles {
			if (parseErr == nil && profiles[i].CharacterID == characterID) || strings.EqualFold(profiles[i].ID, character) {
				if selected != nil && selected.CharacterID != profiles[i].CharacterID {
					return SavedOptimizationRequest{}, fmt.Errorf("profile %q does not belong to character %q", profileID, character)
				}
				if selected == nil {
					selected = &profiles[i]
				}
				break
			}
		}
		if selected == nil {
			return SavedOptimizationRequest{}, fmt.Errorf("unknown character %q", character)
		}
	}
	if selected == nil {
		return SavedOptimizationRequest{}, fmt.Errorf("specify --character or --profile")
	}
	overrides, err := LoadStrategyOverrides(stateDir)
	if err != nil {
		return SavedOptimizationRequest{}, err
	}
	weights := WeightOverrides{MainStats: selected.MainStats, Weights: selected.Weights}
	if saved, ok := overrides.Profiles[selected.ID]; ok {
		weights = saved
	}
	target, err := service.Target(selected.ID)
	if err != nil {
		return SavedOptimizationRequest{}, err
	}
	goals := make(map[string]GoalTuning, len(target.Goals))
	for _, goal := range target.Goals {
		saved, exists := weights.Goals[goal.PropertyID]
		if exists && saved.Disabled {
			continue
		}
		tuning := GoalTuning{Target: goal.Minimum, Maximum: goal.Maximum, Tolerance: .05, Importance: savedGoalWeight(goal.PropertyID, weights.Weights)}
		if exists {
			tuning.Target, tuning.Maximum, tuning.Tolerance, tuning.StrictMinimum = saved.Target, saved.Maximum, saved.Tolerance, saved.StrictMinimum
			if saved.Minimum != nil {
				tuning.Minimum = *saved.Minimum
			} else if saved.StrictMinimum {
				tuning.Minimum = saved.Target * (1 - saved.Tolerance)
			}
		}
		goals[goal.PropertyID] = tuning
	}
	return SavedOptimizationRequest{Profile: *selected, Weights: weights, Goals: goals}, nil
}

func savedGoalWeight(property string, weights map[string]float64) float64 {
	keys := []string{property}
	switch property {
	case "AtkFinal":
		keys = []string{"AtkBase", "AtkUp", "AtkAdd"}
	case "HPFinal":
		keys = []string{"HPMaxBase", "HPMaxUp", "HPMaxAdd", "HPUp"}
	case "DefFinal":
		keys = []string{"DefBase", "DefUp", "DefAdd"}
	}
	weight := 0.0
	for _, key := range keys {
		weight = max(weight, weights[key])
	}
	return weight
}

// OptimizeSavedProject uses the prepared planner settings and priority reservations.
func OptimizeSavedProject(ctx context.Context, service OptimizerService, stateDir, language, mode string, request SavedOptimizationRequest) (OptimizationResult, error) {
	var err error
	service.ReservedModuleIDs, service.ReservedCartridgeIDs, err = HigherPriorityReservations(stateDir, request.Profile.CharacterID)
	if err != nil {
		return OptimizationResult{}, err
	}
	service.WeightOverrides = &request.Weights
	return service.OptimizeFlexibleProject(ctx, stateDir, request.Profile.ID, true, language, mode, request.Goals)
}
