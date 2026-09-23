package app

import (
	"fmt"
	"math"
	"sort"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

type moduleCandidatePool struct {
	eligible            []optimizer.Candidate
	raw                 []optimizer.Candidate
	current             []optimizer.Candidate
	selectionObjectives []optimizer.ObjectiveGoal
	strictObjectives    []optimizer.ObjectiveGoal
	baselineStats       map[string]float64
	excludedEquipped    int
}

type candidateSelection struct {
	selected    []optimizer.Candidate
	perGeometry int
}

type cartridgePool struct {
	available        []nte.Cartridge
	excludedEquipped int
}

func (s OptimizerService) prepareModuleCandidates(modules []nte.Module, profile scoring.Character, refs scoring.References, additional map[string][]nte.Stat, objectives []optimizer.ObjectiveGoal, requiredGeometry map[string]bool, includeEquipped bool) moduleCandidatePool {
	pool := moduleCandidatePool{
		eligible: make([]optimizer.Candidate, 0, len(modules)),
		raw:      make([]optimizer.Candidate, 0, len(modules)),
	}
	baselineStats := optimizer.BuildStatSummary(profile, nil, nil, optimizer.SetDefinition{}, 0, additional).Derived
	pool.baselineStats = baselineStats
	pool.selectionObjectives = objectivesStillMissing(baselineStats, objectives)
	pool.strictObjectives = strictObjectivesStillMissing(baselineStats, objectives)
	baselineObjective := 0.0
	if len(objectives) > 0 {
		baselineObjective = optimizer.ObjectiveScore(baselineStats, objectives)
	}
	for _, module := range modules {
		if s.ExcludedModuleIDs[module.LocalID] {
			continue
		}
		if s.ReservedModuleIDs[module.LocalID] {
			pool.excludedEquipped++
			continue
		}
		if !includeEquipped && module.EquippedCharacterID != 0 && profile.CharacterID != 0 && module.EquippedCharacterID != profile.CharacterID {
			pool.excludedEquipped++
			continue
		}
		score := scoring.Score(module, profile, refs)
		searchScore := score
		if len(objectives) > 0 {
			searchScore *= optimizer.EquipmentTieBreakScale
		}
		pool.raw = append(pool.raw, optimizer.Candidate{Module: module, Score: searchScore})
		candidate := optimizer.Candidate{Module: module, Score: searchScore}
		if len(objectives) > 0 {
			summary := optimizer.BuildStatSummary(profile, []nte.Module{module}, nil, optimizer.SetDefinition{}, 0, additional)
			candidate.ObjectiveValues = make(map[string]float64, len(objectives))
			for _, goal := range objectives {
				candidate.ObjectiveValues[goal.PropertyID] = summary.Derived[goal.PropertyID] - baselineStats[goal.PropertyID]
			}
			candidate.Priority = optimizer.ObjectiveScore(summary.Derived, objectives) - baselineObjective + score*optimizer.EquipmentTieBreakScale
			if requiredGeometry[module.Geometry] {
				candidate.Priority += 25
			}
		}
		pool.eligible = append(pool.eligible, candidate)
		if profile.CharacterID != 0 && module.EquippedCharacterID == profile.CharacterID {
			pool.current = append(pool.current, candidate)
		}
	}
	return pool
}

func (s OptimizerService) selectModuleCandidates(pool moduleCandidatePool, config optimizerDataConfig, plan searchPlan, strictCapacity ...map[string]int) (candidateSelection, error) {
	perGeometry, perSet := config.Optimize.TopPerGeometry, config.Optimize.TopPerSet
	if plan.solverMode == "objective" {
		perGeometry = max(perGeometry, 3)
		perSet = max(perSet, 3)
	}
	selected := pool.eligible
	if plan.approximate {
		selected = optimizer.SelectCandidates(pool.eligible, perGeometry, perSet)
	}
	if plan.approximate && plan.solverMode == "objective" {
		selected = optimizer.SelectObjectiveCandidates(pool.eligible, pool.selectionObjectives, max(perGeometry, 4), 2)
		if plan.requestedMode == "beta" && len(pool.strictObjectives) > 0 {
			capacities := map[string]int{}
			if len(strictCapacity) > 0 {
				capacities = strictCapacity[0]
			}
			byGeometry := map[string][]optimizer.Candidate{}
			for _, candidate := range pool.eligible {
				byGeometry[candidate.Module.Geometry] = append(byGeometry[candidate.Module.Geometry], candidate)
			}
			geometries := make([]string, 0, len(byGeometry))
			for geometry := range byGeometry {
				geometries = append(geometries, geometry)
			}
			sort.Strings(geometries)
			for _, goal := range pool.strictObjectives {
				for _, geometry := range geometries {
					group := byGeometry[geometry]
					capacity, known := capacities[geometry]
					if !known {
						capacity = 2
					}
					if capacity <= 0 {
						continue
					}
					limit := strictSpecialistCount(group, pool.baselineStats[goal.PropertyID], goal, capacity)
					selected = appendUniqueCandidates(selected, optimizer.SelectObjectiveCandidates(group, []optimizer.ObjectiveGoal{goal}, max(perGeometry, 4), limit))
				}
			}
		}
		seedCandidates := optimizer.SelectCandidates(pool.raw, config.Optimize.TopPerGeometry, config.Optimize.TopPerSet)
		selected = appendUniqueCandidates(selected, seedCandidates)
	}
	selected = appendPinnedCandidates(selected, pool.eligible, s.PinnedModuleIDs)
	selected = appendCurrentEquipmentCandidates(selected, pool.current, plan)
	selectedIDs := candidateIDs(selected)
	for moduleID := range s.PinnedModuleIDs {
		if s.ExcludedModuleIDs[moduleID] {
			return candidateSelection{}, fmt.Errorf("module %s cannot be both locked and excluded", moduleID)
		}
		if !selectedIDs[moduleID] {
			return candidateSelection{}, fmt.Errorf("locked module %s is not available", moduleID)
		}
	}
	return candidateSelection{selected: selected, perGeometry: perGeometry}, nil
}

// A strict floor can exceed its soft target. Such a goal still needs
// specialists even when the target itself is already met by base stats.
func strictObjectivesStillMissing(baseline map[string]float64, objectives []optimizer.ObjectiveGoal) []optimizer.ObjectiveGoal {
	result := make([]optimizer.ObjectiveGoal, 0)
	for _, goal := range objectives {
		if !goal.StrictMinimum {
			continue
		}
		floor := strictGoalFloor(goal)
		if baseline[goal.PropertyID] < floor {
			result = append(result, goal)
		}
	}
	return result
}

func strictGoalFloor(goal optimizer.ObjectiveGoal) float64 {
	if goal.StrictFloor > 0 {
		return goal.StrictFloor
	}
	return goal.Minimum * (1 - goal.Tolerance)
}

// Retain enough specialists to cover the strict deficit even when one module
// is insufficient, while bounding the budget by the grid's physical capacity.
func strictSpecialistCount(candidates []optimizer.Candidate, baseline float64, goal optimizer.ObjectiveGoal, capacity int) int {
	bestGain := 0.0
	for _, candidate := range candidates {
		bestGain = max(bestGain, candidate.ObjectiveValues[goal.PropertyID])
	}
	if bestGain <= 0 {
		return min(capacity, 2)
	}
	deficit := strictGoalFloor(goal) - baseline
	needed := math.Ceil(deficit / bestGain)
	if needed >= float64(capacity) {
		return capacity
	}
	return min(capacity, max(2, int(needed)))
}

// Playable area gives a safe upper bound on the number of modules of each
// geometry that a build could use. Actual tiling may allow fewer.
func geometryModuleSlots(grid optimizer.GridDefinition, shapes optimizer.ShapeCatalog, candidates []optimizer.Candidate) map[string]int {
	limits := map[string]int{}
	for _, candidate := range candidates {
		geometry := candidate.Module.Geometry
		if _, exists := limits[geometry]; exists {
			continue
		}
		shape, ok := shapes.Shapes[geometry]
		if !ok || len(shape.Cells) == 0 {
			continue
		}
		limits[geometry] = len(grid.Playable) / len(shape.Cells)
	}
	return limits
}

func appendCurrentEquipmentCandidates(selected, candidates []optimizer.Candidate, plan searchPlan) []optimizer.Candidate {
	if !plan.approximate {
		return selected
	}
	selectedIDs := candidateIDs(selected)
	for _, candidate := range candidates {
		if candidate.Module.EquippedCharacterID != 0 && !selectedIDs[candidate.Module.LocalID] {
			selected = append(selected, candidate)
			selectedIDs[candidate.Module.LocalID] = true
		}
	}
	return selected
}

func appendPinnedCandidates(selected, candidates []optimizer.Candidate, pinned map[string]bool) []optimizer.Candidate {
	selectedIDs := candidateIDs(selected)
	for _, candidate := range candidates {
		if pinned[candidate.Module.LocalID] && !selectedIDs[candidate.Module.LocalID] {
			selected = append(selected, candidate)
			selectedIDs[candidate.Module.LocalID] = true
		}
	}
	return selected
}

func appendUniqueCandidates(selected, candidates []optimizer.Candidate) []optimizer.Candidate {
	selectedIDs := candidateIDs(selected)
	for _, candidate := range candidates {
		if !selectedIDs[candidate.Module.LocalID] {
			selected = append(selected, candidate)
			selectedIDs[candidate.Module.LocalID] = true
		}
	}
	return selected
}

func candidateIDs(candidates []optimizer.Candidate) map[string]bool {
	ids := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		ids[candidate.Module.LocalID] = true
	}
	return ids
}

func (s OptimizerService) prepareCartridges(cartridges []nte.Cartridge, profile scoring.Character, sets optimizer.SetCatalog, includeEquipped bool, plan searchPlan, eligibleModules int) (cartridgePool, error) {
	allowedSets := make(map[string]bool, len(profile.PreferredSets))
	for _, preference := range profile.PreferredSets {
		allowedSets[sets.Definitions[preference.SetID].InventorySetID] = true
	}
	pool := cartridgePool{available: make([]nte.Cartridge, 0, len(cartridges))}
	for _, cartridge := range cartridges {
		if len(allowedSets) > 0 && !allowedSets[cartridge.SetID] {
			continue
		}
		if s.WeightOverrides != nil && !cartridgeMainStatsAllowed(cartridge, s.WeightOverrides.MainStats) {
			continue
		}
		if s.ReservedCartridgeIDs[cartridge.LocalID] {
			pool.excludedEquipped++
			continue
		}
		if !includeEquipped && cartridge.EquippedCharacterID != 0 && profile.CharacterID != 0 && cartridge.EquippedCharacterID != profile.CharacterID {
			pool.excludedEquipped++
			continue
		}
		pool.available = append(pool.available, cartridge)
	}
	requiresCartridge := (!plan.approximate || plan.solverMode == "objective") && eligibleModules > 0 && len(allowedSets) > 0
	if requiresCartridge && len(pool.available) == 0 {
		if s.WeightOverrides != nil {
			return cartridgePool{}, fmt.Errorf("no cartridge is available with the selected set and main stats")
		}
		return cartridgePool{}, fmt.Errorf("no cartridge is available for the selected strategy set: %s", profile.Name)
	}
	return pool, nil
}
