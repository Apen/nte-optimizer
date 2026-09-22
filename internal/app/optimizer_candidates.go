package app

import (
	"fmt"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

type moduleCandidatePool struct {
	eligible            []optimizer.Candidate
	raw                 []optimizer.Candidate
	current             []optimizer.Candidate
	selectionObjectives []optimizer.ObjectiveGoal
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
	pool.selectionObjectives = objectivesStillMissing(baselineStats, objectives)
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
			candidate.Priority = optimizer.ObjectiveScore(summary.Derived, objectives) - baselineObjective + score*.01
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

func (s OptimizerService) selectModuleCandidates(pool moduleCandidatePool, objectives []optimizer.ObjectiveGoal, freeCells int, config optimizerDataConfig, plan searchPlan) (candidateSelection, error) {
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
		selected = optimizer.SelectObjectiveCandidates(pool.eligible, pool.selectionObjectives, max(perGeometry, 4), 1)
		seedCandidates := optimizer.SelectCandidates(pool.raw, config.Optimize.TopPerGeometry, config.Optimize.TopPerSet)
		selected = appendUniqueCandidates(selected, seedCandidates)
		selected = optimizer.PruneDominatedCandidates(selected, objectives, freeCells)
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
			return cartridgePool{}, fmt.Errorf("aucune cartouche disponible avec le set et les stats principales sélectionnés")
		}
		return cartridgePool{}, fmt.Errorf("aucune cartouche disponible pour le set de la stratégie sélectionnée : %s", profile.Name)
	}
	return pool, nil
}
