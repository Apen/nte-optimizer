package optimizer

import (
	"sort"

	"nte-optimizer/internal/nte"
)

type Candidate struct {
	Module          nte.Module         `json:"module"`
	Score           float64            `json:"score"`
	Priority        float64            `json:"-"`
	ObjectiveValues map[string]float64 `json:"-"`
}

// SelectCandidates keeps the best candidates in both geometry and set buckets.
// Taking the union preserves set-building options that a geometry-only ranking
// could discard.
func SelectCandidates(input []Candidate, perGeometry, perSet int) []Candidate {
	ordered := append([]Candidate(nil), input...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i].Score, ordered[j].Score
		if ordered[i].Priority > 0 {
			left = ordered[i].Priority
		}
		if ordered[j].Priority > 0 {
			right = ordered[j].Priority
		}
		if left == right {
			return ordered[i].Module.LocalID < ordered[j].Module.LocalID
		}
		return left > right
	})
	geometryCounts := map[string]int{}
	setCounts := map[string]int{}
	selected := map[string]Candidate{}
	for _, candidate := range ordered {
		if perGeometry > 0 && geometryCounts[candidate.Module.Geometry] < perGeometry {
			selected[candidate.Module.LocalID] = candidate
			geometryCounts[candidate.Module.Geometry]++
		}
		if perSet > 0 && setCounts[candidate.Module.SetID] < perSet {
			selected[candidate.Module.LocalID] = candidate
			setCounts[candidate.Module.SetID]++
		}
	}
	result := make([]Candidate, 0, len(selected))
	for _, candidate := range ordered {
		if _, ok := selected[candidate.Module.LocalID]; ok {
			result = append(result, candidate)
		}
	}
	return result
}

// SelectObjectiveCandidates keeps generalists as well as the strongest item for
// each requested stat inside every geometry. This prevents a weighted average
// from discarding a specialist needed to hit a hard breakpoint later.
func SelectObjectiveCandidates(input []Candidate, goals []ObjectiveGoal, perGeometry, perGoal int) []Candidate {
	if perGeometry < 1 {
		perGeometry = 1
	}
	if perGoal < 1 {
		perGoal = 1
	}
	groups := map[string][]Candidate{}
	for _, candidate := range input {
		groups[candidate.Module.Geometry] = append(groups[candidate.Module.Geometry], candidate)
	}
	selected := map[string]Candidate{}
	for _, group := range groups {
		ordered := append([]Candidate(nil), group...)
		sort.SliceStable(ordered, func(i, j int) bool {
			left, right := ordered[i].Priority, ordered[j].Priority
			if left == right {
				left, right = ordered[i].Score, ordered[j].Score
			}
			if left == right {
				return ordered[i].Module.LocalID < ordered[j].Module.LocalID
			}
			return left > right
		})
		for i := 0; i < len(ordered) && i < perGeometry; i++ {
			selected[ordered[i].Module.LocalID] = ordered[i]
		}
		for _, goal := range goals {
			statOrdered := append([]Candidate(nil), group...)
			sort.SliceStable(statOrdered, func(i, j int) bool {
				left := statOrdered[i].ObjectiveValues[goal.PropertyID]
				right := statOrdered[j].ObjectiveValues[goal.PropertyID]
				if left == right {
					return statOrdered[i].Priority > statOrdered[j].Priority
				}
				return left > right
			})
			for i := 0; i < len(statOrdered) && i < perGoal; i++ {
				selected[statOrdered[i].Module.LocalID] = statOrdered[i]
			}
		}
	}
	result := make([]Candidate, 0, len(selected))
	for _, candidate := range input {
		if _, ok := selected[candidate.Module.LocalID]; ok {
			result = append(result, candidate)
		}
	}
	return result
}
