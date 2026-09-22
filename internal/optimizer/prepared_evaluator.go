package optimizer

import (
	"math"
	"sort"
	"strings"
)

// prepareBlueprintEvaluator compiles the invariant geometry/set work once.
// The returned closure owns scratch arrays and must only be used by one worker.
// Source order matches BuildStatSummary, including additional source collisions.
func (e CartridgeSetEvaluator) prepareBlueprintEvaluator(geometries []string) func([]Placement) BonusResult {
	evaluate := e.prepareBlueprintEvaluations(geometries)
	best := BonusResult{}
	accept := func(result BonusResult) {
		if result.Score > best.Score {
			best = result
		}
	}
	return func(placements []Placement) BonusResult {
		best = BonusResult{}
		if e.exact {
			best.Score = math.Inf(-1)
		}
		evaluate(placements, accept)
		return best
	}
}

func (e CartridgeSetEvaluator) prepareBlueprintEvaluations(geometries []string) func([]Placement, func(BonusResult)) {
	projection := e.statProjection()
	properties := map[string]int{}
	add := func(key string) {
		if _, ok := properties[key]; !ok {
			properties[key] = len(properties)
		}
	}
	for _, key := range projection.properties {
		add(key)
	}
	for _, goal := range e.objectives {
		add(goal.PropertyID)
		for _, property := range objectiveInputs(goal.PropertyID) {
			add(property)
		}
	}
	type preparedSet struct {
		main, sub                 map[string]float64
		result                    BonusResult
		sources                   [][]float64
		moduleSource, traitSource int
	}
	sets := make([]preparedSet, 0, len(e.sets))
	summaries := make([]StatSummary, len(e.sets))
	matchedCounts := make([]int, len(e.sets))
	for i, set := range e.sets {
		matched := map[string]bool{}
		for _, required := range set.definition.RequiredGeometries {
			for _, geometry := range geometries {
				if required == geometry {
					matched[required] = true
				}
			}
		}
		matchedCounts[i] = len(matched)
		if e.exact && len(matched) != len(set.definition.RequiredGeometries) {
			continue
		}
		summaries[i] = BuildStatSummary(e.character, nil, &set.cartridge, set.definition, len(matched), e.additional)
	}
	for i, set := range e.sets {
		if e.exact && matchedCounts[i] != len(set.definition.RequiredGeometries) {
			continue
		}
		score := scoreForMatched(set.definition.Bonuses, matchedCounts[i]) * set.priority
		prepared := preparedSet{result: BonusResult{Score: score + set.cartridgeScore, SetBonusScore: score, CartridgeScore: set.cartridgeScore, SetID: set.definition.ID, CartridgeID: set.cartridge.LocalID, MatchedCount: matchedCounts[i]}, moduleSource: -1, traitSource: -1}
		prepared.main, prepared.sub = map[string]float64{}, map[string]float64{}
		addStats(prepared.main, set.cartridge.MainStats)
		addStats(prepared.sub, set.cartridge.SubStats)
		keys := []string{}
		for key := range summaries[i].Sources {
			if !strings.HasSuffix(key, "_conditional") {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			if strings.HasSuffix(key, "_conditional") {
				continue
			}
			if key == "modules" {
				prepared.moduleSource = len(prepared.sources)
			}
			if key == "console_trait" {
				prepared.traitSource = len(prepared.sources)
			}
			values := make([]float64, len(properties))
			for property, value := range summaries[i].Sources[key] {
				if index, ok := properties[property]; ok {
					values[index] = value
				}
			}
			prepared.sources = append(prepared.sources, values)
		}
		sets = append(sets, prepared)
	}
	moduleValues := make([]float64, len(properties))
	values := make([]float64, len(properties))
	goalIndices := make([][4]int, len(e.objectives))
	for i, goal := range e.objectives {
		indices := [4]int{properties[goal.PropertyID], -1, -1, -1}
		switch goal.PropertyID {
		case "HPFinal":
			indices = [4]int{-1, properties["HPMaxBase"], properties["HPMaxUp"], properties["HPMaxAdd"]}
		case "AtkFinal":
			indices = [4]int{-1, properties["AtkBase"], properties["AtkUp"], properties["AtkAdd"]}
		case "DefFinal":
			indices = [4]int{-1, properties["DefBase"], properties["DefUp"], properties["DefAdd"]}
		}
		goalIndices[i] = indices
	}
	traitIndex := -1
	if e.character.ConsoleTrait != nil {
		if index, ok := properties[e.character.ConsoleTrait.PropertyID]; ok {
			traitIndex = index
		}
	}
	moduleAdditional := make([][]float64, len(properties))
	for _, stat := range e.additional["modules"] {
		if i, ok := properties[stat.PropertyID]; ok {
			moduleAdditional[i] = append(moduleAdditional[i], stat.Value)
		}
	}
	traitAdditional := []float64{}
	if e.character.ConsoleTrait != nil {
		for _, stat := range e.additional["console_trait"] {
			if stat.PropertyID == e.character.ConsoleTrait.PropertyID {
				traitAdditional = append(traitAdditional, stat.Value)
			}
		}
	}
	sourceWeighted := e.sourceWeighted()
	main, sub, totals := map[string]float64{}, map[string]float64{}, map[string]float64{}
	return func(placements []Placement, accept func(BonusResult)) {
		clear(moduleValues)
		traitCount := 0
		for _, placement := range placements {
			module := projection.modules[placement.ModuleID]
			traitCount += module.trait
			for _, atom := range module.stats {
				stat := projection.values[atom]
				moduleValues[stat.property] += stat.value
			}
		}
		for _, set := range sets {
			if sourceWeighted {
				clear(main)
				clear(sub)
				for key, value := range set.main {
					main[key] = value
				}
				for key, value := range set.sub {
					sub[key] = value
				}
				for _, placement := range placements {
					module := e.modules[placement.ModuleID]
					addStats(main, module.MainStats)
					addStats(sub, module.SubStats)
				}
			}
			result := set.result
			if len(e.objectives) > 0 {
				clear(values)
				for sourceIndex, source := range set.sources {
					for index, value := range source {
						if sourceIndex == set.moduleSource {
							value = moduleValues[index]
							for _, extra := range moduleAdditional[index] {
								value += extra
							}
						}
						if sourceIndex == set.traitSource && traitIndex == index {
							value = float64(traitCount) * e.character.ConsoleTrait.ValuePerModule
							for _, extra := range traitAdditional {
								value += extra
							}
						}
						values[index] += value
					}
				}
				utility := 0.0
				feasible := true
				for i, goal := range e.objectives {
					indices := goalIndices[i]
					value := 0.0
					if indices[0] >= 0 {
						value = values[indices[0]]
					} else {
						value = panelTotal(values[indices[1]], values[indices[2]], values[indices[3]])
					}
					if e.exact && (exceedsLimit(value, goal.Maximum) || belowToleranceFloor(value, goal)) {
						feasible = false
						break
					}
					utility += objectiveUtility(value, goal)
				}
				if !feasible {
					continue
				}
				if sourceWeighted {
					clear(totals)
					for key, index := range properties {
						totals[key] = values[index]
					}
					utility, _ = sourceObjectiveValues(totals, main, sub, e.character, e.objectives)
				}
				result.Score += utility
			}
			accept(result)
		}
	}
}
