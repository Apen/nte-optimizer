package optimizer

import (
	"math"
	"nte-optimizer/internal/nte"
	"sort"
)

// prepareStatBound relaxes correlations between properties: each property can
// independently take its best k remaining values. Thus every real continuation
// is inside the interval, including negative stats and multiplicative panels.
// Minima stay soft objectives. Only a definitely exceeded strict maximum can
// make an interval infeasible. No Pareto pruning is used.
func (e CartridgeSetEvaluator) prepareStatBound(groups []blueprintGroup, pools map[string][]Candidate) func([]Candidate, int) float64 {
	properties := map[string]int{}
	add := func(key string) {
		if _, ok := properties[key]; !ok {
			properties[key] = len(properties)
		}
	}
	for _, goal := range e.objectives {
		add(goal.PropertyID)
	}
	for _, key := range []string{"HPMaxBase", "HPMaxUp", "HPMaxAdd", "AtkBase", "AtkUp", "AtkAdd", "DefBase", "DefUp", "DefAdd"} {
		add(key)
	}
	vectors := map[string][]float64{}
	moduleStats := func(module nte.Module) map[string]float64 {
		values := map[string]float64{}
		addStats(values, module.MainStats)
		addStats(values, module.SubStats)
		if trait := e.character.ConsoleTrait; trait != nil && module.Area == trait.Area {
			values[trait.PropertyID] += trait.ValuePerModule
		}
		return values
	}
	for _, pool := range pools {
		for _, candidate := range pool {
			for key := range moduleStats(candidate.Module) {
				add(key)
			}
		}
	}
	type option struct {
		base  []float64
		score float64
	}
	options := []option{}
	geometry := []string{}
	area := 0
	for _, group := range groups {
		for range group.count {
			geometry = append(geometry, group.geometry)
		}
		largest := 0
		for _, candidate := range pools[group.geometry] {
			largest = max(largest, candidate.Module.Area)
		}
		area += largest * group.count
	}
	summaries := []map[string]float64{}
	for _, set := range e.sets {
		matched := map[string]bool{}
		for _, required := range set.definition.RequiredGeometries {
			for _, g := range geometry {
				if required == g {
					matched[g] = true
				}
			}
		}
		if e.exact && len(matched) != len(set.definition.RequiredGeometries) {
			continue
		}
		values := BuildStatSummary(e.character, nil, &set.cartridge, set.definition, len(matched), e.additional).Unconditional
		for key := range values {
			add(key)
		}
		summaries = append(summaries, values)
		bonus := scoreForMatched(set.definition.Bonuses, len(matched)) * set.priority
		score := bonus + set.cartridgeScore
		if len(e.objectives) > 0 {
			score += 500*bonus + 1000*float64(area)
		}
		options = append(options, option{score: score})
	}
	for i, values := range summaries {
		options[i].base = make([]float64, len(properties))
		for key, value := range values {
			options[i].base[properties[key]] = value
		}
	}
	for _, pool := range pools {
		for _, candidate := range pool {
			vector := make([]float64, len(properties))
			for key, value := range moduleStats(candidate.Module) {
				vector[properties[key]] = value
			}
			vectors[candidate.Module.LocalID] = vector
		}
	}
	low, high := make([][]float64, len(groups)+1), make([][]float64, len(groups)+1)
	low[len(groups)], high[len(groups)] = make([]float64, len(properties)), make([]float64, len(properties))
	for g := len(groups) - 1; g >= 0; g-- {
		low[g], high[g] = append([]float64(nil), low[g+1]...), append([]float64(nil), high[g+1]...)
		pool := pools[groups[g].geometry]
		values := make([]float64, len(pool))
		for property := range properties {
			index := properties[property]
			for i, candidate := range pool {
				values[i] = vectors[candidate.Module.LocalID][index]
			}
			sort.Float64s(values)
			for i := 0; i < groups[g].count; i++ {
				low[g][index] += values[i]
				high[g][index] += values[len(values)-1-i]
			}
		}
	}
	selectedValues := make([]float64, len(properties))
	lo, hi := make([]float64, len(properties)), make([]float64, len(properties))
	objectiveValues := map[string]float64{}
	goals := append([]ObjectiveGoal(nil), e.objectives...)
	for i := range goals {
		goals[i].Maximum = 0
	}
	return func(selected []Candidate, next int) float64 {
		clear(selectedValues)
		for _, candidate := range selected {
			for i, value := range vectors[candidate.Module.LocalID] {
				selectedValues[i] += value
			}
		}
		best := math.Inf(-1)
		if !e.exact {
			best = 0
		}
		for _, option := range options {
			for i, value := range option.base {
				lo[i] = value + selectedValues[i] + low[next][i]
				hi[i] = value + selectedValues[i] + high[next][i]
				margin := 1e-9 * math.Max(1, math.Max(math.Abs(lo[i]), math.Abs(hi[i])))
				lo[i] -= margin
				hi[i] += margin
			}
			feasible := true
			for _, goal := range e.objectives {
				lower, upper := lo[properties[goal.PropertyID]], hi[properties[goal.PropertyID]]
				base, up, flat := "", "", ""
				switch goal.PropertyID {
				case "HPFinal":
					base, up, flat = "HPMaxBase", "HPMaxUp", "HPMaxAdd"
				case "AtkFinal":
					base, up, flat = "AtkBase", "AtkUp", "AtkAdd"
				case "DefFinal":
					base, up, flat = "DefBase", "DefUp", "DefAdd"
				}
				if base != "" {
					b, u, f := properties[base], properties[up], properties[flat]
					products := []float64{lo[b] * lo[u], lo[b] * hi[u], hi[b] * lo[u], hi[b] * hi[u]}
					pmin, pmax := products[0], products[0]
					for _, p := range products[1:] {
						pmin = math.Min(pmin, p)
						pmax = math.Max(pmax, p)
					}
					lower = lo[b] + math.Floor(pmin+lo[f]+1e-9)
					upper = hi[b] + math.Floor(pmax+hi[f]+1e-9)
				}
				if e.exact && exceedsLimit(lower, goal.Maximum) {
					feasible = false
					break
				}
				objectiveValues[goal.PropertyID] = upper
			}
			if feasible {
				best = math.Max(best, option.score+ObjectiveScore(objectiveValues, goals))
			}
		}
		return best
	}
}
