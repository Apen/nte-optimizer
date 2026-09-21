package app

import (
	"sort"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func matchedRequiredGeometries(modules []nte.Module, required []string) int {
	needed := map[string]bool{}
	for _, geometry := range required {
		needed[geometry] = true
	}
	matched := map[string]bool{}
	for _, module := range modules {
		if needed[module.Geometry] {
			matched[module.Geometry] = true
		}
	}
	return len(matched)
}

func statsFromValues(values map[string]float64) []nte.Stat {
	result := make([]nte.Stat, 0, len(values))
	for property, value := range values {
		result = append(result, nte.Stat{PropertyID: property, Value: value})
	}
	return result
}

type refinementState struct {
	modules  []nte.Module
	rawScore float64
	score    float64
}

func refineFixedGeometry(placements []optimizer.Placement, seed []nte.Module, candidates []optimizer.Candidate, profile scoring.Character, cartridge *nte.Cartridge, set optimizer.SetDefinition, matched int, additional map[string][]nte.Stat, goals []optimizer.ObjectiveGoal, required map[string]bool) []nte.Module {
	if len(seed) != len(placements) {
		return seed
	}
	geometryByID := map[string]string{}
	for _, candidate := range candidates {
		geometryByID[candidate.Module.LocalID] = candidate.Module.Geometry
	}
	beam := []refinementState{{}}
	for _, placement := range placements {
		geometry := geometryByID[placement.ModuleID]
		options := make([]optimizer.Candidate, 0)
		for _, candidate := range candidates {
			if candidate.Module.Geometry == geometry {
				options = append(options, candidate)
			}
		}
		sort.SliceStable(options, func(i, j int) bool { return options[i].Priority > options[j].Priority })
		if len(options) > 16 {
			options = options[:16]
		}
		next := make([]refinementState, 0, len(beam)*len(options))
		for _, state := range beam {
			for _, option := range options {
				if containsModule(state.modules, option.Module.LocalID) {
					continue
				}
				modules := append(append([]nte.Module(nil), state.modules...), option.Module)
				summary := optimizer.BuildStatSummary(profile, modules, cartridge, set, matched, additional)
				rawScore := state.rawScore + option.Score
				lockedBonus := float64(requiredModuleCount(modules, required)) * 1e12
				next = append(next, refinementState{modules: modules, rawScore: rawScore, score: lockedBonus + optimizer.ObjectiveScore(summary.Derived, goals) + rawScore*.01})
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return next[i].score > next[j].score })
		if len(next) > 600 {
			next = next[:600]
		}
		beam = next
		if len(beam) == 0 {
			return seed
		}
	}
	for _, state := range beam {
		if requiredModuleCount(state.modules, required) == len(required) {
			return state.modules
		}
	}
	return seed
}

func requiredModuleCount(modules []nte.Module, required map[string]bool) int {
	count := 0
	for _, module := range modules {
		if required[module.LocalID] {
			count++
		}
	}
	return count
}

func containsModule(modules []nte.Module, localID string) bool {
	for _, module := range modules {
		if module.LocalID == localID {
			return true
		}
	}
	return false
}
