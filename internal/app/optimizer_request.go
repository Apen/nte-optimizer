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
}

func parseSearchPlan(mode string) (searchPlan, error) {
	plan := searchPlan{requestedMode: mode, approximate: mode != "exact-score" && mode != "exact-objective"}
	switch mode {
	case "fast", "beta", "exact-objective":
		plan.solverMode = "objective"
	case "score", "exact-score":
		plan.solverMode = "score"
	default:
		return searchPlan{}, fmt.Errorf("unknown search method: %s", mode)
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

// migrateLegacyDerivedStatWeights moves weights saved against panel base stats
// to the corresponding equipment stat when that source has no scoring reference.
func migrateLegacyDerivedStatWeights(profile *scoring.Character, refs scoring.References) {
	if profile == nil {
		return
	}
	aliases := []struct{ legacy, supported string }{
		{"AtkBase", "AtkUp"},
		{"HPMaxBase", "HPMaxUp"},
		{"HPUp", "HPMaxUp"},
		{"DefBase", "DefUp"},
	}
	for _, weights := range []map[string]float64{profile.Weights, profile.MainWeights, profile.SubWeights} {
		for _, alias := range aliases {
			weight, exists := weights[alias.legacy]
			if !exists || refs[alias.legacy] > 0 {
				continue
			}
			if refs[alias.supported] > 0 && weight > weights[alias.supported] {
				weights[alias.supported] = weight
			}
			delete(weights, alias.legacy)
		}
	}
}

func prepareObjectives(mode string, buildTarget *target.BuildTarget, tunings map[string]GoalTuning, overrides *WeightOverrides, profile scoring.Character, sets optimizer.SetCatalog) ([]optimizer.ObjectiveGoal, map[string]bool) {
	objectives := []optimizer.ObjectiveGoal{}
	requiredGeometry := map[string]bool{}
	if mode != "objective" || buildTarget == nil {
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
					objectives = append(objectives, optimizer.ObjectiveGoal{PropertyID: goal.PropertyID, Minimum: goal.Minimum, Maximum: tuning.Maximum, Tolerance: tuning.Tolerance, StrictMinimum: tuning.StrictMinimum, StrictFloor: tuning.Minimum, ExplicitWeight: true})
				}
				continue
			}
		}
		objectives = append(objectives, optimizer.ObjectiveGoal{PropertyID: goal.PropertyID, Minimum: goal.Minimum, Maximum: tuning.Maximum, Tolerance: tuning.Tolerance, Importance: tuning.Importance, StrictMinimum: tuning.StrictMinimum, StrictFloor: tuning.Minimum, ExplicitWeight: overrides != nil})
	}
	for _, preference := range profile.PreferredSets {
		for _, geometry := range sets.Definitions[preference.SetID].RequiredGeometries {
			requiredGeometry[geometry] = true
		}
	}
	return objectives, requiredGeometry
}
