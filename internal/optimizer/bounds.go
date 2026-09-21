package optimizer

import "math"

// All goal utilities are at most 10.5*importance, irrespective of minima,
// tolerances or caps. A cap penalty is nonpositive. Geometry fixes matched
// set bonuses; maximum area per slot overestimates the occupancy reward.
func (e CartridgeSetEvaluator) blueprintUpperBound(geometries []string) float64 {
	area := 0
	for _, geometry := range geometries {
		largest := 0
		for _, module := range e.modules {
			if module.Geometry == geometry {
				largest = max(largest, module.Area)
			}
		}
		area += largest
	}
	utility := 0.0
	for _, goal := range e.objectives {
		if goal.Minimum > 0 {
			importance := goal.Importance
			if importance <= 0 {
				importance = 1
			}
			utility = math.Nextafter(utility+10.5*importance, math.Inf(1))
		}
	}
	bound := 0.0
	for _, set := range e.sets {
		matched := map[string]bool{}
		for _, required := range set.definition.RequiredGeometries {
			for _, geometry := range geometries {
				if required == geometry {
					matched[required] = true
				}
			}
		}
		bonus := scoreForMatched(set.definition.Bonuses, len(matched)) * set.priority
		score := bonus + set.cartridgeScore
		if len(e.objectives) > 0 {
			score += bonus*500 + utility + float64(area)*1000
		}
		bound = math.Max(bound, score)
	}
	return math.Nextafter(bound, math.Inf(1))
}

// combinationBounds chooses the best k scores in each suffix, independently
// of Priority ordering. Each addition is rounded toward positive infinity.
func combinationBounds(pool []Candidate, k int) [][]float64 {
	bounds := make([][]float64, len(pool)+1)
	for i := range bounds {
		bounds[i] = make([]float64, k+1)
		for j := 1; j <= k; j++ {
			bounds[i][j] = math.Inf(-1)
		}
	}
	for i := len(pool) - 1; i >= 0; i-- {
		for j := 1; j <= k; j++ {
			bounds[i][j] = math.Max(bounds[i+1][j], math.Nextafter(pool[i].Score+bounds[i+1][j-1], math.Inf(1)))
		}
	}
	return bounds
}
