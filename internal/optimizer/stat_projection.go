package optimizer

import (
	"math"
	"nte-optimizer/internal/nte"
	"sort"
)

// A projection keeps every input read by the evaluator, not merely properties
// whose names match goals. Values are interned from the inventory; the census
// is evidence for repetition, never a hardcoded rule about future items.
type projectedContribution struct {
	property int
	value    float64
}
type projectedModule struct {
	area, trait int
	stats       []uint32
}
type statProjection struct {
	properties []string
	integer    []bool
	modules    map[string]projectedModule
	values     []projectedContribution
}

func objectiveInputs(property string) []string {
	switch property {
	case "AtkFinal":
		return []string{"AtkBase", "AtkUp", "AtkAdd"}
	case "HPFinal":
		return []string{"HPMaxBase", "HPMaxUp", "HPMaxAdd"}
	case "DefFinal":
		return []string{"DefBase", "DefUp", "DefAdd"}
	default:
		return []string{property}
	}
}

func (e CartridgeSetEvaluator) statProjection() statProjection {
	needed := map[string]bool{}
	for _, goal := range e.objectives {
		if goal.Minimum > 0 || goal.Maximum > 0 {
			for _, property := range objectiveInputs(goal.PropertyID) {
				needed[property] = true
			}
		}
	}
	result := statProjection{modules: make(map[string]projectedModule, len(e.modules))}
	for property := range needed {
		result.properties = append(result.properties, property)
	}
	sort.Strings(result.properties)
	indexes := map[string]int{}
	result.integer = make([]bool, len(result.properties))
	magnitude := make([]float64, len(result.properties))
	type atomKey struct {
		property int
		bits     uint64
	}
	atoms := map[atomKey]uint32{}
	for i, property := range result.properties {
		indexes[property] = i
		result.integer[i] = true
	}
	for id, module := range e.modules {
		projected := projectedModule{area: module.Area}
		if trait := e.character.ConsoleTrait; trait != nil && module.Area == trait.Area {
			projected.trait = 1
		}
		for _, stats := range [][]nte.Stat{module.MainStats, module.SubStats} {
			for _, stat := range stats {
				if index, ok := indexes[stat.PropertyID]; ok {
					key := atomKey{index, math.Float64bits(stat.Value)}
					atom, found := atoms[key]
					if !found {
						atom = uint32(len(result.values))
						atoms[key] = atom
						result.values = append(result.values, projectedContribution{index, stat.Value})
					}
					projected.stats = append(projected.stats, atom)
					magnitude[index] += math.Abs(stat.Value)
					result.integer[index] = result.integer[index] && math.Trunc(stat.Value) == stat.Value && !math.IsInf(stat.Value, 0) && !math.IsNaN(stat.Value)
				}
			}
		}
		result.modules[id] = projected
	}
	// This guarantees every intermediate integer addition is exactly represented
	// in float64, regardless of ordering. Fractions keep their original sequence.
	for i := range result.integer {
		result.integer[i] = result.integer[i] && magnitude[i] < 1<<52
	}
	return result
}

type projectionScratch struct {
	totals    []int64
	sequences [][]uint64
	key       []uint64
}

func (p statProjection) scratch() projectionScratch {
	return projectionScratch{totals: make([]int64, len(p.properties)), sequences: make([][]uint64, len(p.properties))}
}

// Equal keys imply bit-for-bit identical module source sums. Fractional values
// are NOT rounded or reassociated: .02+.03 and .03+.02 are different keys.
func (p statProjection) key(placements []Placement, scratch *projectionScratch) ([]uint64, uint64) {
	clear(scratch.totals)
	for i := range scratch.sequences {
		scratch.sequences[i] = scratch.sequences[i][:0]
	}
	area, trait := 0, 0
	for _, placement := range placements {
		module := p.modules[placement.ModuleID]
		area += module.area
		trait += module.trait
		for _, atom := range module.stats {
			stat := p.values[atom]
			if p.integer[stat.property] {
				scratch.totals[stat.property] += int64(stat.value)
			} else {
				scratch.sequences[stat.property] = append(scratch.sequences[stat.property], math.Float64bits(stat.value))
			}
		}
	}
	scratch.key = append(scratch.key[:0], uint64(area), uint64(trait))
	for i := range p.properties {
		if p.integer[i] {
			scratch.key = append(scratch.key, uint64(scratch.totals[i]))
		} else {
			scratch.key = append(scratch.key, uint64(len(scratch.sequences[i])))
			scratch.key = append(scratch.key, scratch.sequences[i]...)
		}
	}
	hash := uint64(14695981039346656037)
	for _, word := range scratch.key {
		hash ^= word
		hash *= 1099511628211
		hash ^= hash >> 32
	}
	return scratch.key, hash
}
