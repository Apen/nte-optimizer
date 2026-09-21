package app

import (
	"fmt"
	"math"

	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

type searchPlan struct {
	requestedMode string
	solverMode    string
	approximate   bool
	compromise    bool
}

func parseSearchPlan(mode string) (searchPlan, error) {
	plan := searchPlan{requestedMode: mode, approximate: mode != "balanced" && mode != "exact-score", compromise: mode == "compromise"}
	switch mode {
	case "balanced":
		plan.solverMode = "balanced"
	case "fast-balanced", "compromise":
		plan.solverMode = "balanced"
	case "score", "exact-score":
		plan.solverMode = "score"
	default:
		return searchPlan{}, fmt.Errorf("méthode de recherche inconnue : %s", mode)
	}
	return plan, nil
}

func applyWeightOverrides(profile scoring.Character, overrides *WeightOverrides) (scoring.Character, error) {
	if overrides == nil {
		return profile, nil
	}
	for property, weight := range overrides.Weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 10 {
			return scoring.Character{}, fmt.Errorf("invalid weight for %s: expected 0 to 10", property)
		}
	}
	profile.MainWeights = nil
	profile.SubWeights = nil
	profile.Weights = cloneWeights(overrides.Weights)
	return profile, nil
}

func prepareObjectives(mode string, buildTarget *target.BuildTarget, tunings map[string]GoalTuning, overrides *WeightOverrides, profile scoring.Character, sets optimizer.SetCatalog) ([]optimizer.ObjectiveGoal, map[string]bool) {
	objectives := []optimizer.ObjectiveGoal{}
	requiredGeometry := map[string]bool{}
	if mode != "balanced" || buildTarget == nil {
		return objectives, requiredGeometry
	}
	for _, goal := range buildTarget.Goals {
		tuning, configured := tunings[goal.PropertyID]
		if tunings != nil && !configured {
			continue
		}
		if overrides != nil {
			tuning.Importance = goalWeight(profile, goal.PropertyID)
			if tuning.Importance == 0 {
				if tuning.StrictMinimum || tuning.Maximum > 0 {
					objectives = append(objectives, optimizer.ObjectiveGoal{PropertyID: goal.PropertyID, Minimum: goal.Minimum, Maximum: tuning.Maximum, Tolerance: tuning.Tolerance, StrictMinimum: tuning.StrictMinimum})
				}
				continue
			}
		}
		objectives = append(objectives, optimizer.ObjectiveGoal{PropertyID: goal.PropertyID, Minimum: goal.Minimum, Maximum: tuning.Maximum, Tolerance: tuning.Tolerance, Importance: tuning.Importance, StrictMinimum: tuning.StrictMinimum})
	}
	for _, preference := range profile.PreferredSets {
		for _, geometry := range sets.Definitions[preference.SetID].RequiredGeometries {
			requiredGeometry[geometry] = true
		}
	}
	return objectives, requiredGeometry
}
