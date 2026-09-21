package optimizer

import (
	"context"
	"sort"
)

// SelectCompromiseCandidates ranks by the character's overall equipment score.
// Keep enough copies to fill even a board made entirely of a single shape.
func SelectCompromiseCandidates(input []Candidate, capacity int) []Candidate {
	ordered := append([]Candidate(nil), input...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Score == ordered[j].Score {
			return ordered[i].Module.LocalID < ordered[j].Module.LocalID
		}
		return ordered[i].Score > ordered[j].Score
	})
	counts := map[string]int{}
	result := make([]Candidate, 0)
	for _, candidate := range ordered {
		area := max(1, candidate.Module.Area)
		limit := max(12, capacity/area+2)
		if counts[candidate.Module.Geometry] < limit {
			result = append(result, candidate)
			counts[candidate.Module.Geometry]++
		}
	}
	return result
}

// RefineCompromise explores same-shape single and pair replacements using all
// eligible pieces. The exact evaluator checks every cartridge and hard cap.
// The time budget and two rounds deliberately make this a heuristic search.
func RefineCompromise(ctx context.Context, seed Solution, candidates []Candidate, evaluator CartridgeSetEvaluator, required map[string]bool, keep int) Solution {
	if len(seed.Placements) == 0 || ctx.Err() != nil {
		seed.Complete = false
		return seed
	}
	keep = max(1, keep)
	byID := map[string]Candidate{}
	byGeometry := map[string][]Candidate{}
	for _, candidate := range candidates {
		byID[candidate.Module.LocalID] = candidate
		byGeometry[candidate.Module.Geometry] = append(byGeometry[candidate.Module.Geometry], candidate)
	}
	for geometry := range byGeometry {
		sort.Slice(byGeometry[geometry], func(i, j int) bool {
			a, b := byGeometry[geometry][i], byGeometry[geometry][j]
			if a.Score == b.Score {
				return a.Module.LocalID < b.Module.LocalID
			}
			return a.Score > b.Score
		})
	}
	leaders, indexes := []Solution{}, map[string]int{}
	initial := append([]Solution{seed}, seed.Alternatives...)
	for _, solution := range initial {
		solution.Alternatives = nil
		leaders, indexes = retainTopSolution(leaders, indexes, solution, keep)
	}
	var evaluated uint64
	for round := 0; round < 2 && ctx.Err() == nil; round++ {
		starts := append([]Solution(nil), leaders...)
		for _, start := range starts {
			placements := clonePlacements(start.Placements)
			geometries := make([]string, len(placements))
			for i, placement := range placements {
				geometries[i] = byID[placement.ModuleID].Module.Geometry
			}
			evaluate := evaluator.prepareBlueprintEvaluations(geometries)
			visit := func() {
				used := map[string]bool{}
				raw := 0.0
				for _, placement := range placements {
					if used[placement.ModuleID] {
						return
					}
					used[placement.ModuleID] = true
					raw += byID[placement.ModuleID].Score
				}
				evaluated++
				evaluate(placements, func(bonus BonusResult) {
					if len(leaders) < keep || raw+bonus.Score >= leaders[len(leaders)-1].Score {
						leaders, indexes = retainTopSolution(leaders, indexes, blueprintSolution(raw+bonus.Score, raw, bonus, placements), keep)
					}
				})
			}
			// Visit all single changes before spending time on pairs.
			for width := 1; width <= 2 && ctx.Err() == nil; width++ {
				for i, original := range start.Placements {
					if required[original.ModuleID] {
						continue
					}
					for _, a := range byGeometry[geometries[i]] {
						if ctx.Err() != nil {
							break
						}
						placements[i].ModuleID = a.Module.LocalID
						if width == 1 {
							visit()
							continue
						}
						for j := i + 1; j < len(placements) && ctx.Err() == nil; j++ {
							if required[start.Placements[j].ModuleID] {
								continue
							}
							for _, b := range byGeometry[geometries[j]] {
								if ctx.Err() != nil {
									break
								}
								placements[j].ModuleID = b.Module.LocalID
								visit()
							}
							placements[j] = start.Placements[j]
						}
					}
					placements[i] = original
				}
			}
			if ctx.Err() != nil {
				break
			}
		}
	}
	result := leaders[0]
	result.Metrics = seed.Metrics
	result.Metrics.Evaluated += evaluated
	result.Visited = seed.Visited + evaluated
	result.Complete = false
	result.Alternatives = append([]Solution(nil), leaders[1:]...)
	for i := range result.Alternatives {
		result.Alternatives[i].Complete = false
	}
	return result
}
