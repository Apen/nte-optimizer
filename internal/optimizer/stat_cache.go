package optimizer

import (
	"math"
	"slices"
)

type statCacheCounts struct{ hits, misses uint64 }
type statCacheEntry struct {
	used    bool
	hash    uint64
	key     []uint64
	results []BonusResult
}

// Each blueprint owns a bounded cache. We cache the best K cartridges for a
// statistical state, not the IDs of its modules: every identity combination
// still participates in the top K. Full key comparison makes hash collisions
// harmless. An eviction changes only running time, never the search space.
func (e CartridgeSetEvaluator) cachedBlueprintEvaluations(geometries []string, keep int, p statProjection, counts *statCacheCounts) func([]Placement, func(BonusResult)) {
	evaluate := e.prepareBlueprintEvaluations(geometries)

	entries := make([]statCacheEntry, 1024)
	scratch := p.scratch()
	keep = max(1, keep)
	var current *statCacheEntry
	better := func(a, b BonusResult) bool {
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.CartridgeID+"|"+a.SetID < b.CartridgeID+"|"+b.SetID
	}
	record := func(result BonusResult) {
		if math.IsInf(result.Score, -1) {
			return
		}
		position := len(current.results)
		for i, previous := range current.results {
			if better(result, previous) {
				position = i
				break
			}
		}
		if position >= keep {
			return
		}
		current.results = append(current.results, result)
		copy(current.results[position+1:], current.results[position:len(current.results)-1])
		current.results[position] = result
		if len(current.results) > keep {
			current.results = current.results[:keep]
		}
	}
	return func(placements []Placement, accept func(BonusResult)) {
		key, hash := p.key(placements, &scratch)
		if e.sourceWeighted() {
			// The ordinary projection merges main/sub stats. Include their separate
			// sums so equal panel totals cannot reuse a differently weighted result.
			main, sub := map[string]float64{}, map[string]float64{}
			for _, placement := range placements {
				module := e.modules[placement.ModuleID]
				addStats(main, module.MainStats)
				addStats(sub, module.SubStats)
			}
			for _, property := range p.properties {
				for _, value := range []float64{main[property], sub[property]} {
					bits := math.Float64bits(value)
					key = append(key, bits)
					for shift := 0; shift < 64; shift += 8 {
						hash = (hash ^ ((bits >> shift) & 255)) * 1099511628211
					}
				}
			}
		}
		current = &entries[hash%uint64(len(entries))]
		if current.used && current.hash == hash && slices.Equal(current.key, key) {
			counts.hits++
		} else {
			counts.misses++
			current.used = true
			current.hash = hash
			current.key = append(current.key[:0], key...)
			current.results = current.results[:0]
			evaluate(placements, record)
		}
		for _, result := range current.results {
			accept(result)
		}
	}
}
