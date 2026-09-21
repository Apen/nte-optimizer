package optimizer

import (
	"math"
	"nte-optimizer/internal/nte"
)

// reduceStatCandidates preserves the top K, not just the single optimum.
// If at most M pieces of this geometry fit and M+K-1 pieces strictly dominate
// X, every build using X has at least K distinct strictly better replacements.
// Maxima require equality on all their inputs, so a beneficial increase never
// silently breaks a cap. Locked pieces are never removed.
func (e CartridgeSetEvaluator) reduceStatCandidates(input []Candidate, catalog ShapeCatalog, capacity, keep int, required map[string]bool, geometrySlots ...map[string]int) []Candidate {
	if e.sourceWeighted() {
		return input
	}
	p := e.statProjection()
	equal := map[string]bool{}
	for _, goal := range e.objectives {
		if goal.Maximum > 0 {
			for _, property := range objectiveInputs(goal.PropertyID) {
				equal[property] = true
			}
		}
	}
	// Panel monotonicity requires nonnegative factors. Unknown negative data
	// conservatively disables this reduction, without excluding any candidate.
	nonnegative := func(stats []nte.Stat) bool {
		for _, stat := range stats {
			if stat.Value < 0 || math.IsNaN(stat.Value) || math.IsInf(stat.Value, 0) {
				return false
			}
		}
		return true
	}
	for _, values := range []map[string]float64{e.character.BaseStats, e.character.CurrentStats} {
		for _, value := range values {
			if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return input
			}
		}
	}
	for _, stats := range e.additional {
		if !nonnegative(stats) {
			return input
		}
	}
	if e.character.ConsoleTrait != nil && e.character.ConsoleTrait.ValuePerModule < 0 {
		return input
	}
	for _, set := range e.sets {
		if !nonnegative(set.cartridge.MainStats) || !nonnegative(set.cartridge.SubStats) {
			return input
		}
		for _, bonus := range set.definition.Bonuses {
			if !nonnegative(bonus.Stats) {
				return input
			}
		}
	}
	vectors := make([][]float64, len(input))
	for i, candidate := range input {
		if !nonnegative(candidate.Module.MainStats) || !nonnegative(candidate.Module.SubStats) {
			return input
		}
		vectors[i] = make([]float64, len(p.properties))
		for _, atom := range p.modules[candidate.Module.LocalID].stats {
			stat := p.values[atom]
			vectors[i][stat.property] += stat.value
		}
	}
	result := make([]Candidate, 0, len(input))
	for i, candidate := range input {
		if required[candidate.Module.LocalID] {
			result = append(result, candidate)
			continue
		}
		shape, err := catalog.Shape(candidate.Module.Geometry)
		if err != nil || len(shape.Cells) == 0 {
			result = append(result, candidate)
			continue
		}
		maxUsed := capacity / len(shape.Cells)
		if len(geometrySlots) > 0 {
			maxUsed = geometrySlots[0][candidate.Module.Geometry]
		}
		threshold := maxUsed + max(1, keep) - 1
		dominators := 0
		for j, other := range input {
			if i == j || other.Module.Geometry != candidate.Module.Geometry || other.Module.Area != candidate.Module.Area {
				continue
			}
			// A strict raw score advantage survives objective saturation. Never use
			// merely greater stats as a strict order: both builds might reach a cap.
			if !(other.Score > candidate.Score+1e-7*math.Max(1, math.Max(math.Abs(other.Score), math.Abs(candidate.Score)))) {
				continue
			}
			dominates := true
			for k, property := range p.properties {
				left, right := vectors[j][k], vectors[i][k]
				if left < right || equal[property] && left != right {
					dominates = false
					break
				}
			}
			if dominates {
				dominators++
				if dominators >= threshold {
					break
				}
			}
		}
		if dominators < threshold {
			result = append(result, candidate)
		}
	}
	return result
}
