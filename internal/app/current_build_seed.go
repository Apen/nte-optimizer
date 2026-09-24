package app

import (
	"sort"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

const currentBuildSeedSearchLimit = 100_000

// currentBuildSeed packs the modules already equipped by the character into a
// valid layout so an approximate search starts with a known build.
func currentBuildSeed(grid optimizer.Grid, shapes optimizer.ShapeCatalog, modules []nte.Module, characterID int, candidates []optimizer.Candidate) []optimizer.Placement {
	if characterID == 0 {
		return nil
	}

	searchable := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		searchable[candidate.Module.LocalID] = true
	}
	type seedModule struct {
		id         string
		area       int
		placements []optimizer.Placement
	}
	seedModules := make([]seedModule, 0)
	for _, module := range modules {
		if module.EquippedCharacterID != characterID {
			continue
		}
		if module.LocalID == "" || !searchable[module.LocalID] {
			return nil
		}
		shape, err := shapes.Shape(module.Geometry)
		if err != nil {
			return nil
		}
		placements := optimizer.GeneratePlacements(grid, module.LocalID, shape)
		if len(placements) == 0 {
			return nil
		}
		seedModules = append(seedModules, seedModule{id: module.LocalID, area: len(shape.Cells), placements: placements})
	}
	if len(seedModules) == 0 {
		return nil
	}
	sort.SliceStable(seedModules, func(i, j int) bool {
		if len(seedModules[i].placements) != len(seedModules[j].placements) {
			return len(seedModules[i].placements) < len(seedModules[j].placements)
		}
		if seedModules[i].area != seedModules[j].area {
			return seedModules[i].area > seedModules[j].area
		}
		return seedModules[i].id < seedModules[j].id
	})

	occupied := grid.Clone()
	seed := make([]optimizer.Placement, 0, len(seedModules))
	remaining := make([]int, len(seedModules))
	for index := range remaining {
		remaining[index] = index
	}
	visited := 0
	var placeCurrent func([]int) bool
	placeCurrent = func(remaining []int) bool {
		if len(remaining) == 0 {
			return true
		}
		visited++
		if visited > currentBuildSeedSearchLimit {
			return false
		}

		selectedIndex := -1
		var available []optimizer.Placement
		for _, index := range remaining {
			placements := make([]optimizer.Placement, 0, len(seedModules[index].placements))
			for _, placement := range seedModules[index].placements {
				if occupied.CanPlace(placement.Cells) {
					placements = append(placements, placement)
				}
			}
			if len(placements) == 0 {
				return false
			}
			if selectedIndex < 0 || len(placements) < len(available) {
				selectedIndex, available = index, placements
			}
		}

		next := make([]int, 0, len(remaining)-1)
		for _, index := range remaining {
			if index != selectedIndex {
				next = append(next, index)
			}
		}
		for _, placement := range available {
			if err := occupied.Place(placement.Cells); err != nil {
				continue
			}
			seed = append(seed, placement)
			if placeCurrent(next) {
				return true
			}
			seed = seed[:len(seed)-1]
			occupied.Remove(placement.Cells)
			if visited > currentBuildSeedSearchLimit {
				return false
			}
		}
		return false
	}
	if !placeCurrent(remaining) {
		return nil
	}
	return seed
}
