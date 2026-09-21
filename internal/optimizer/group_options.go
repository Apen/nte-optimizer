package optimizer

import "context"

type groupOption struct {
	indices []int
	score   float64
}

// A per-search cache cannot become stale when inventory or goals change.
// Oversized groups keep the streaming enumerator: the memory limit never
// truncates the search space. Store indices, not copies of module structures.
func prepareGroupOptions(ctx context.Context, blueprints []geometryBlueprint, pools map[string][]Candidate) map[blueprintGroup][]groupOption {
	const budget = 16 << 20
	remaining := uint64(budget)
	result := map[blueprintGroup][]groupOption{}
	uses := map[blueprintGroup]int{}
	for _, blueprint := range blueprints {
		for _, group := range blueprintGroups(blueprint) {
			uses[group]++
		}
	}
	seen := map[blueprintGroup]bool{}
	for _, blueprint := range blueprints {
		for _, group := range blueprintGroups(blueprint) {
			if seen[group] {
				continue
			}
			seen[group] = true
			count := binomial(len(pools[group.geometry]), group.count)
			if uses[group] < 2 || count > 4096 {
				continue
			}
			bytes := saturatingMultiply(count, uint64(32+group.count*8))
			if bytes > remaining {
				continue
			}
			options := make([]groupOption, 0, int(count))
			indices := make([]int, 0, group.count)
			var enumerate func(int, float64)
			enumerate = func(start int, score float64) {
				if ctx.Err() != nil {
					return
				}
				if len(indices) == group.count {
					options = append(options, groupOption{append([]int(nil), indices...), score})
					return
				}
				for i := start; i <= len(pools[group.geometry])-(group.count-len(indices)); i++ {
					indices = append(indices, i)
					enumerate(i+1, score+pools[group.geometry][i].Score)
					indices = indices[:len(indices)-1]
				}
			}
			enumerate(0, 0)
			if ctx.Err() != nil {
				return result
			}
			result[group] = options
			remaining -= bytes
		}
	}
	return result
}
