package optimizer

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type geometryBlueprint struct {
	geometries []string
	placements []Placement
}

type assignmentJob struct {
	blueprint geometryBlueprint
	first     int
}

type candidateReduction struct {
	usable     []Candidate
	projection statProjection
	statValues int
	profiles   int
}

type solutionLeaderboard struct {
	mu      sync.Mutex
	best    Solution
	leaders []Solution
	indexes map[string]int
	limit   int
}

func newSolutionLeaderboard(seed Solution, limit int) *solutionLeaderboard {
	limit = max(1, limit)
	board := &solutionLeaderboard{
		best:    seed,
		leaders: make([]Solution, 0, limit),
		indexes: make(map[string]int, limit),
		limit:   limit,
	}
	if len(seed.Placements) > 0 {
		board.leaders, board.indexes = retainTopSolution(board.leaders, board.indexes, seed, limit)
	}
	return board
}

func (b *solutionLeaderboard) snapshot() (Solution, []Solution) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.best, append([]Solution(nil), b.leaders...)
}

func (b *solutionLeaderboard) merge(solutions []Solution) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, solution := range solutions {
		b.leaders, b.indexes = retainTopSolution(b.leaders, b.indexes, solution, b.limit)
	}
	if len(b.leaders) > 0 {
		b.best = b.leaders[0]
	}
}

// BlueprintCache stores the expensive geometry-only packing results. Module
// statistics and objective weights do not affect these layouts, so subsequent
// optimizations of the same board can reuse them safely.
type BlueprintCache struct {
	mu      sync.RWMutex
	entries map[string][]geometryBlueprint
}

func NewBlueprintCache() *BlueprintCache {
	return &BlueprintCache{entries: make(map[string][]geometryBlueprint)}
}

func (c *BlueprintCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// solveByBlueprint separates the geometric puzzle from item selection. A
// geometry multiset needs to be laid out only once because neither statistics
// nor set activation depend on the position of a module inside the board.
func (s SearchSolver) solveByBlueprint(ctx context.Context, base Grid, candidates []Candidate, bonusEvaluator GlobalBonusEvaluator, seed []Placement) (Solution, bool, error) {
	started := time.Now()
	metrics := SearchMetrics{InputCandidates: len(candidates)}
	var projection statProjection
	metrics.RetainedCandidates = len(candidates)
	usable, byGeometry, err := s.prepareCandidatePools(candidates)
	if err != nil {
		return Solution{}, false, err
	}

	if s.BlueprintCache != nil {
		s.BlueprintCache.mu.RLock()
		_, metrics.CacheHit = s.BlueprintCache.entries[s.blueprintCacheKey(base, byGeometry)]
		s.BlueprintCache.mu.RUnlock()
	}
	blueprints, complete := s.geometryBlueprints(ctx, base, byGeometry)
	blueprints = compatibleBlueprints(blueprints, s.GeometryRequirements)
	if len(s.RequiredIDs) > 0 {
		requiredGeometry := []string{}
		for _, candidate := range usable {
			if s.RequiredIDs[candidate.Module.LocalID] {
				requiredGeometry = append(requiredGeometry, candidate.Module.Geometry)
			}
		}
		if len(requiredGeometry) != len(s.RequiredIDs) {
			blueprints = nil
		} else {
			blueprints = compatibleBlueprints(blueprints, [][]string{requiredGeometry})
		}
	}
	metrics.BlueprintNS = time.Since(started).Nanoseconds()
	metrics.Blueprints = len(blueprints)
	if len(blueprints) == 0 {
		if s.Exact {
			return Solution{Complete: complete && ctx.Err() == nil, Metrics: metrics}, true, nil
		}
		return Solution{}, false, nil
	}
	metrics.TheoreticalBeforeReduction = blueprintAssignmentTotal(blueprints, byGeometry)
	if evaluator, ok := bonusEvaluator.(CartridgeSetEvaluator); ok && s.Exact && !s.DisableStatReduction {
		reductionStarted := time.Now()
		reduction := s.reduceBlueprintCandidates(base, usable, byGeometry, blueprints, evaluator)
		usable = reduction.usable
		projection = reduction.projection
		metrics.StatValues = reduction.statValues
		metrics.StatProfiles = reduction.profiles
		metrics.RetainedCandidates = len(usable)
		metrics.DominatedCandidates = metrics.InputCandidates - metrics.RetainedCandidates
		metrics.ReductionNS = time.Since(reductionStarted).Nanoseconds()
	}
	assignmentTotal := blueprintAssignmentTotal(blueprints, byGeometry)
	metrics.Theoretical = assignmentTotal
	assignmentStarted := time.Now()
	var groupOptions map[blueprintGroup][]groupOption
	if !s.DisableGroups {
		groupOptions = prepareGroupOptions(ctx, blueprints, byGeometry)
	}
	jobList, workerCount := prepareAssignmentJobs(blueprints, byGeometry, assignmentTotal, runtime.GOMAXPROCS(0))
	best := seededSolution(seed, usable, bonusEvaluator)
	if !placementsContainRequired(best.Placements, s.RequiredIDs) {
		best = Solution{Complete: true}
	}
	keepBest := max(1, s.KeepBest)
	leaderboard := newSolutionLeaderboard(best, keepBest)
	var progressMu sync.Mutex
	var lastProgress uint64
	var visited atomic.Uint64
	var pruned atomic.Uint64
	var evaluationNS atomic.Int64
	var statCacheHits, statCacheMisses atomic.Uint64
	var stopped atomic.Bool
	evaluate := func(job assignmentJob) {
		blueprint := job.blueprint
		best, leaders := leaderboard.snapshot()
		leaderIndex := make(map[string]int, len(leaders))
		for i, solution := range leaders {
			leaderIndex[solutionSignature(solution)] = i
		}
		defer func() { leaderboard.merge(leaders) }()
		evaluateBonus := bonusEvaluator.Evaluate
		var evaluateVariants func([]Placement, func(BonusResult))
		if evaluator, ok := bonusEvaluator.(CartridgeSetEvaluator); ok {
			if s.Exact {
				if !s.DisableStatReduction {
					counts := &statCacheCounts{}
					evaluateVariants = evaluator.cachedBlueprintEvaluations(blueprint.geometries, keepBest, projection, counts)
					defer func() { statCacheHits.Add(counts.hits); statCacheMisses.Add(counts.misses) }()
				} else {
					evaluateVariants = evaluator.prepareBlueprintEvaluations(blueprint.geometries)
				}
			} else {
				evaluateBonus = evaluator.prepareBlueprintEvaluator(blueprint.geometries)
			}
		}
		groups := blueprintGroups(blueprint)
		bounds := make([][][]float64, len(groups))
		var statBound func([]Candidate, int) float64
		if evaluator, ok := bonusEvaluator.(CartridgeSetEvaluator); ok && len(evaluator.objectives) > 0 && !s.DisableBound {
			statBound = evaluator.prepareStatBound(groups, byGeometry)
		}
		requiredSuffix := make([][]int, len(groups))
		suffix := make([]float64, len(groups)+1)
		for g := len(groups) - 1; g >= 0; g-- {
			pool := byGeometry[groups[g].geometry]
			requiredSuffix[g] = make([]int, len(pool)+1)
			for i := len(pool) - 1; i >= 0; i-- {
				requiredSuffix[g][i] = requiredSuffix[g][i+1]
				if s.RequiredIDs[pool[i].Module.LocalID] {
					requiredSuffix[g][i]++
				}
			}
			bounds[g] = combinationBounds(byGeometry[groups[g].geometry], groups[g].count)
			suffix[g] = math.Nextafter(suffix[g+1]+bounds[g][0][groups[g].count], math.Inf(1))
		}
		bonusBound := bonusEvaluator.UpperBound()
		if evaluator, ok := bonusEvaluator.(CartridgeSetEvaluator); ok {
			bonusBound = evaluator.blueprintUpperBound(blueprint.geometries)
		}
		placements, slots := prepareBlueprintAssignments(blueprint)
		selected := make([]Candidate, 0, len(blueprint.placements))
		currentModuleScore := 0.0
		acceptBonus := func(bonus BonusResult) {
			if math.IsInf(bonus.Score, -1) {
				return
			}
			total := currentModuleScore + bonus.Score
			if total > best.Score || len(best.Placements) == 0 {
				best = blueprintSolution(total, currentModuleScore, bonus, placements)
			}
			if len(leaders) < keepBest || total >= leaders[len(leaders)-1].Score {
				leaders, leaderIndex = retainTopSolution(leaders, leaderIndex, blueprintSolution(total, currentModuleScore, bonus, placements), keepBest)
			}
		}
		var chooseGroups func(int, float64)
		chooseGroups = func(groupIndex int, moduleScore float64) {
			if stopped.Load() {
				return
			}
			if !s.DisableBound && len(leaders) == keepBest {
				upper := moduleScore + suffix[groupIndex] + bonusBound
				if statBound != nil && groupIndex < len(groups) {
					upper = math.Min(upper, moduleScore+suffix[groupIndex]+statBound(selected, groupIndex))
				}
				if upper < leaders[len(leaders)-1].Score-1e-9*math.Max(1, math.Abs(upper)) {
					pruned.Add(1)
					return
				}
			}
			if ctx.Err() != nil {
				stopped.Store(true)
				return
			}
			if visited.Load()&1023 == 0 {
				select {
				case <-ctx.Done():
					stopped.Store(true)
					return
				default:
				}
			}
			if groupIndex == len(groups) {
				currentVisited := visited.Add(1)
				assignBlueprintModules(placements, slots, groups, selected)
				if !placementsContainRequired(placements, s.RequiredIDs) {
					return
				}
				var evaluationStarted time.Time
				if s.Measure {
					evaluationStarted = time.Now()
				}
				currentModuleScore = moduleScore
				if evaluateVariants != nil {
					evaluateVariants(placements, acceptBonus)
				} else {
					acceptBonus(evaluateBonus(placements))
				}
				if s.Measure {
					evaluationNS.Add(time.Since(evaluationStarted).Nanoseconds())
				}
				bestScore := best.Score
				if s.Progress != nil && currentVisited&16383 == 0 {
					progressMu.Lock()
					if currentVisited > lastProgress {
						lastProgress = currentVisited
						s.Progress(SearchProgress{Pruned: pruned.Load(), Visited: currentVisited, Total: assignmentTotal, BestScore: bestScore, ElapsedMS: time.Since(started).Milliseconds(), Candidates: len(usable), Workers: workerCount})
					}
					progressMu.Unlock()
				}
				return
			}
			group := groups[groupIndex]
			pool := byGeometry[group.geometry]
			if len(pool) < group.count {
				return
			}
			if options, ok := groupOptions[group]; ok {
				for _, option := range options {
					if stopped.Load() {
						return
					}
					if groupIndex == 0 && job.first >= 0 && option.indices[0] != job.first {
						continue
					}
					found := 0
					for _, index := range option.indices {
						if s.RequiredIDs[pool[index].Module.LocalID] {
							found++
						}
					}
					if found != requiredSuffix[groupIndex][0] {
						pruned.Add(1)
						continue
					}
					for _, index := range option.indices {
						selected = append(selected, pool[index])
					}
					chooseGroups(groupIndex+1, moduleScore+option.score)
					selected = selected[:len(selected)-group.count]
				}
				return
			}
			var combinations func(int, int, float64)
			combinations = func(start, remaining int, score float64) {
				if ctx.Err() != nil {
					stopped.Store(true)
					return
				}
				if requiredSuffix[groupIndex][start] > remaining {
					pruned.Add(1)
					return
				}
				if !s.DisableBound {
					upper := moduleScore + score + bounds[groupIndex][start][remaining] + suffix[groupIndex+1] + bonusBound
					cut := len(leaders) == keepBest && upper < leaders[len(leaders)-1].Score-1e-9*math.Max(1, math.Abs(upper))
					if cut {
						pruned.Add(1)
						return
					}
				}
				if remaining == 0 {
					chooseGroups(groupIndex+1, moduleScore+score)
					return
				}
				for i := start; i <= len(pool)-remaining && !stopped.Load(); i++ {
					if groupIndex == 0 && remaining == group.count && job.first >= 0 && i != job.first {
						if s.RequiredIDs[pool[i].Module.LocalID] {
							break
						}
						continue
					}
					selected = append(selected, pool[i])
					combinations(i+1, remaining-1, score+pool[i].Score)
					selected = selected[:len(selected)-1]
					if s.RequiredIDs[pool[i].Module.LocalID] {
						break
					}
				}
			}
			combinations(0, group.count, 0)
		}
		chooseGroups(0, 0)
	}
	runAssignmentWorkers(jobList, workerCount, &stopped, evaluate)
	best, leaders := leaderboard.snapshot()
	best = finalizeBlueprintSolution(metrics, best, leaders, visited.Load(), pruned.Load(), time.Since(assignmentStarted), evaluationNS.Load(), statCacheHits.Load(), statCacheMisses.Load(), complete && !stopped.Load() && ctx.Err() == nil)
	if s.Progress != nil {
		s.Progress(SearchProgress{Finished: best.Complete, Pruned: pruned.Load(), Visited: best.Visited, Total: assignmentTotal, BestScore: best.Score, ElapsedMS: time.Since(started).Milliseconds(), Candidates: len(usable), Workers: workerCount})
	}
	return best, true, nil
}

func runAssignmentWorkers(jobs []assignmentJob, workerCount int, stopped *atomic.Bool, evaluate func(assignmentJob)) {
	if workerCount <= 0 || len(jobs) == 0 {
		return
	}
	queue := make(chan assignmentJob)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for job := range queue {
				evaluate(job)
			}
		}()
	}
	for _, job := range jobs {
		if stopped.Load() {
			break
		}
		queue <- job
	}
	close(queue)
	workers.Wait()
}

func finalizeBlueprintSolution(metrics SearchMetrics, best Solution, leaders []Solution, visited, pruned uint64, assignmentDuration time.Duration, evaluationNS int64, statCacheHits, statCacheMisses uint64, complete bool) Solution {
	best.Visited = visited
	metrics.Evaluated = visited
	metrics.Pruned = pruned
	metrics.AssignmentNS = assignmentDuration.Nanoseconds()
	metrics.EvaluationNS = evaluationNS
	metrics.StatCacheHits = statCacheHits
	metrics.StatCacheMisses = statCacheMisses
	best.Metrics = metrics
	best.Complete = complete
	if len(leaders) > 1 {
		best.Alternatives = append([]Solution(nil), leaders[1:]...)
	}
	return best
}

func (s SearchSolver) reduceBlueprintCandidates(base Grid, usable []Candidate, byGeometry map[string][]Candidate, blueprints []geometryBlueprint, evaluator CartridgeSetEvaluator) candidateReduction {
	maxSlots := map[string]int{}
	for _, blueprint := range blueprints {
		for _, group := range blueprintGroups(blueprint) {
			maxSlots[group.geometry] = max(maxSlots[group.geometry], group.count)
		}
	}
	reduced := evaluator.reduceStatCandidates(usable, s.Catalog, base.FreeCells(), s.KeepBest, s.RequiredIDs, maxSlots)
	retained := make(map[string]bool, len(reduced))
	for _, candidate := range reduced {
		retained[candidate.Module.LocalID] = true
	}
	for geometry, pool := range byGeometry {
		filtered := pool[:0]
		for _, candidate := range pool {
			if retained[candidate.Module.LocalID] {
				filtered = append(filtered, candidate)
			}
		}
		byGeometry[geometry] = filtered
	}
	projection := evaluator.statProjection()
	profiles := map[string]bool{}
	scratch := projection.scratch()
	for _, candidate := range reduced {
		key, _ := projection.key([]Placement{{ModuleID: candidate.Module.LocalID}}, &scratch)
		profiles[fmt.Sprint(candidate.Module.Geometry, "|", key, "|", math.Float64bits(candidate.Score))] = true
	}
	return candidateReduction{
		usable:     reduced,
		projection: projection,
		statValues: len(projection.values),
		profiles:   len(profiles),
	}
}

func (s SearchSolver) prepareCandidatePools(candidates []Candidate) ([]Candidate, map[string][]Candidate, error) {
	usable := make([]Candidate, 0, len(candidates))
	byGeometry := map[string][]Candidate{}
	for _, candidate := range candidates {
		if candidate.Score <= 0 && !s.Exact {
			continue
		}
		if _, err := s.Catalog.Shape(candidate.Module.Geometry); err != nil {
			return nil, nil, err
		}
		usable = append(usable, candidate)
		byGeometry[candidate.Module.Geometry] = append(byGeometry[candidate.Module.Geometry], candidate)
	}
	for geometry := range byGeometry {
		sort.SliceStable(byGeometry[geometry], func(i, j int) bool {
			left, right := byGeometry[geometry][i], byGeometry[geometry][j]
			if left.Priority != right.Priority {
				return left.Priority > right.Priority
			}
			if left.Score != right.Score {
				return left.Score > right.Score
			}
			return left.Module.LocalID < right.Module.LocalID
		})
	}
	return usable, byGeometry, nil
}

func prepareAssignmentJobs(blueprints []geometryBlueprint, byGeometry map[string][]Candidate, assignmentTotal uint64, maxWorkers int) ([]assignmentJob, int) {
	jobs := make([]assignmentJob, 0, len(blueprints))
	for _, blueprint := range blueprints {
		groups := blueprintGroups(blueprint)
		if len(blueprints) < maxWorkers && assignmentTotal > 50000 && len(groups) > 0 {
			for first := 0; first <= len(byGeometry[groups[0].geometry])-groups[0].count; first++ {
				jobs = append(jobs, assignmentJob{blueprint: blueprint, first: first})
			}
		} else {
			jobs = append(jobs, assignmentJob{blueprint: blueprint, first: -1})
		}
	}
	return jobs, min(maxWorkers, len(jobs))
}

func compatibleBlueprints(blueprints []geometryBlueprint, requirements [][]string) []geometryBlueprint {
	if len(requirements) == 0 {
		return blueprints
	}
	result := make([]geometryBlueprint, 0, len(blueprints))
	for _, blueprint := range blueprints {
		available := make(map[string]int, len(blueprint.geometries))
		for _, geometry := range blueprint.geometries {
			available[geometry]++
		}
		for _, required := range requirements {
			needed := make(map[string]int, len(required))
			for _, geometry := range required {
				needed[geometry]++
			}
			compatible := true
			for geometry, count := range needed {
				if available[geometry] < count {
					compatible = false
					break
				}
			}
			if compatible {
				result = append(result, blueprint)
				break
			}
		}
	}
	return result
}

func placementsContainRequired(placements []Placement, required map[string]bool) bool {
	if len(required) == 0 {
		return true
	}
	found := make(map[string]bool, len(required))
	for _, placement := range placements {
		if required[placement.ModuleID] {
			found[placement.ModuleID] = true
		}
	}
	return len(found) == len(required)
}

func blueprintSolution(total, moduleScore float64, bonus BonusResult, placements []Placement) Solution {
	return Solution{
		Score: total, ModuleScore: moduleScore, SetBonusScore: bonus.SetBonusScore, CartridgeScore: bonus.CartridgeScore,
		SelectedCartridgeID: bonus.CartridgeID, SelectedSetID: bonus.SetID, SetMatchedCount: bonus.MatchedCount,
		Placements: clonePlacements(placements), Complete: true,
	}
}

func retainTopSolution(leaders []Solution, indexes map[string]int, candidate Solution, limit int) ([]Solution, map[string]int) {
	signature := solutionSignature(candidate)
	if index, found := indexes[signature]; found {
		if leaders[index].Score >= candidate.Score {
			return leaders, indexes
		}
		leaders[index] = candidate
	} else {
		leaders = append(leaders, candidate)
	}
	sort.SliceStable(leaders, func(i, j int) bool {
		if leaders[i].Score == leaders[j].Score {
			return solutionSignature(leaders[i]) < solutionSignature(leaders[j])
		}
		return leaders[i].Score > leaders[j].Score
	})
	if len(leaders) > limit {
		leaders = leaders[:limit]
	}
	indexes = make(map[string]int, len(leaders))
	for index, solution := range leaders {
		indexes[solutionSignature(solution)] = index
	}
	return leaders, indexes
}

func solutionSignature(solution Solution) string {
	moduleIDs := make([]string, len(solution.Placements))
	for index, placement := range solution.Placements {
		moduleIDs[index] = placement.ModuleID
	}
	sort.Strings(moduleIDs)
	return solution.SelectedCartridgeID + "|" + solution.SelectedSetID + "|" + strings.Join(moduleIDs, ",")
}

func blueprintAssignmentTotal(blueprints []geometryBlueprint, byGeometry map[string][]Candidate) uint64 {
	var total uint64
	for _, blueprint := range blueprints {
		combinations := uint64(1)
		for _, group := range blueprintGroups(blueprint) {
			combinations = saturatingMultiply(combinations, binomial(len(byGeometry[group.geometry]), group.count))
		}
		if math.MaxUint64-total < combinations {
			return math.MaxUint64
		}
		total += combinations
	}
	return total
}

func binomial(n, k int) uint64 {
	if k < 0 || n < k {
		return 0
	}
	if k > n-k {
		k = n - k
	}
	result := uint64(1)
	for i := 1; i <= k; i++ {
		factor := uint64(n - k + i)
		divisor := uint64(i)
		g := gcd(factor, divisor)
		factor /= g
		divisor /= g
		g = gcd(result, divisor)
		result /= g
		divisor /= g
		if result > math.MaxUint64/factor {
			return math.MaxUint64
		}
		result = result * factor / divisor
	}
	return result
}

func gcd(a, b uint64) uint64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func saturatingMultiply(left, right uint64) uint64 {
	if left == 0 || right == 0 {
		return 0
	}
	if left > math.MaxUint64/right {
		return math.MaxUint64
	}
	return left * right
}

type blueprintGroup struct {
	geometry string
	count    int
}

func blueprintGroups(blueprint geometryBlueprint) []blueprintGroup {
	counts := map[string]int{}
	for _, geometry := range blueprint.geometries {
		counts[geometry]++
	}
	keys := make([]string, 0, len(counts))
	for geometry := range counts {
		keys = append(keys, geometry)
	}
	sort.Strings(keys)
	groups := make([]blueprintGroup, 0, len(keys))
	for _, geometry := range keys {
		groups = append(groups, blueprintGroup{geometry: geometry, count: counts[geometry]})
	}
	return groups
}

func prepareBlueprintAssignments(blueprint geometryBlueprint) ([]Placement, map[string][]int) {
	placements := make([]Placement, len(blueprint.placements))
	slots := make(map[string][]int)
	for i, template := range blueprint.placements {
		placement := template
		placement.Cells = append([]Point(nil), template.Cells...)
		placements[i] = placement
		geometry := blueprint.geometries[i]
		slots[geometry] = append(slots[geometry], i)
	}
	return placements, slots
}

func assignBlueprintModules(placements []Placement, slots map[string][]int, groups []blueprintGroup, selected []Candidate) {
	selectedIndex := 0
	for _, group := range groups {
		for slotIndex := 0; slotIndex < group.count; slotIndex++ {
			placements[slots[group.geometry][slotIndex]].ModuleID = selected[selectedIndex].Module.LocalID
			selectedIndex++
		}
	}
}

func seededSolution(seed []Placement, candidates []Candidate, evaluator GlobalBonusEvaluator) Solution {
	best := Solution{Complete: true}
	if len(seed) == 0 {
		return best
	}
	scores := map[string]float64{}
	for _, candidate := range candidates {
		scores[candidate.Module.LocalID] = candidate.Score
	}
	for _, placement := range seed {
		best.ModuleScore += scores[placement.ModuleID]
	}
	bonus := evaluator.Evaluate(seed)
	// An exact evaluator returns no cartridge when the seed violates a hard
	// objective or set constraint. Do not let such a seed become the incumbent.
	if _, exact := evaluator.(CartridgeSetEvaluator); exact && bonus.CartridgeID == "" {
		return Solution{Complete: true}
	}
	best.Score = best.ModuleScore + bonus.Score
	best.SetBonusScore = bonus.SetBonusScore
	best.CartridgeScore = bonus.CartridgeScore
	best.SelectedCartridgeID = bonus.CartridgeID
	best.SelectedSetID = bonus.SetID
	best.SetMatchedCount = bonus.MatchedCount
	best.Placements = append([]Placement(nil), seed...)
	return best
}

func (s SearchSolver) geometryBlueprints(ctx context.Context, base Grid, candidates map[string][]Candidate) ([]geometryBlueprint, bool) {
	cacheKey := s.blueprintCacheKey(base, candidates)
	if s.BlueprintCache != nil {
		s.BlueprintCache.mu.RLock()
		cached, found := s.BlueprintCache.entries[cacheKey]
		s.BlueprintCache.mu.RUnlock()
		if found {
			return cached, true
		}
	}
	placements := map[string][]Placement{}
	keys := make([]string, 0, len(candidates))
	for geometry, pool := range candidates {
		shape, _ := s.Catalog.Shape(geometry)
		placements[geometry] = GeneratePlacements(base, geometry, shape)
		keys = append(keys, geometry)
		_ = pool
	}
	sort.Strings(keys)
	grid := base.Clone()
	currentGeometry := []string{}
	currentPlacements := []Placement{}
	geometryCounts := make(map[string]int, len(keys))
	seen := map[string]bool{}
	result := []geometryBlueprint{}
	complete := true
	var visit func()
	visit = func() {
		if !complete {
			return
		}
		select {
		case <-ctx.Done():
			complete = false
			return
		default:
		}
		anchor, found := firstFreeCell(grid)
		if !found {
			signatureParts := append([]string(nil), currentGeometry...)
			sort.Strings(signatureParts)
			signature := strings.Join(signatureParts, "|")
			if !seen[signature] {
				seen[signature] = true
				result = append(result, geometryBlueprint{geometries: append([]string(nil), currentGeometry...), placements: clonePlacements(currentPlacements)})
			}
			return
		}
		for _, geometry := range keys {
			if geometryCounts[geometry] >= len(candidates[geometry]) {
				continue
			}
			for _, placement := range placements[geometry] {
				if !containsPoint(placement.Cells, anchor) || !grid.CanPlace(placement.Cells) {
					continue
				}
				_ = grid.Place(placement.Cells)
				currentGeometry = append(currentGeometry, geometry)
				currentPlacements = append(currentPlacements, placement)
				geometryCounts[geometry]++
				visit()
				geometryCounts[geometry]--
				currentPlacements = currentPlacements[:len(currentPlacements)-1]
				currentGeometry = currentGeometry[:len(currentGeometry)-1]
				grid.Remove(placement.Cells)
			}
		}
	}
	visit()
	if complete && s.BlueprintCache != nil {
		s.BlueprintCache.mu.Lock()
		s.BlueprintCache.entries[cacheKey] = result
		s.BlueprintCache.mu.Unlock()
	}
	return result, complete
}

func (s SearchSolver) blueprintCacheKey(base Grid, candidates map[string][]Candidate) string {
	keys := make([]string, 0, len(candidates))
	for geometry := range candidates {
		keys = append(keys, geometry)
	}
	sort.Strings(keys)
	var key strings.Builder
	fmt.Fprintf(&key, "%dx%d:%v", base.Width, base.Height, base.bits)
	for _, geometry := range keys {
		shape, _ := s.Catalog.Shape(geometry)
		fmt.Fprintf(&key, "|%s:%d:%v:%t", geometry, min(len(candidates[geometry]), base.FreeCells()/len(shape.Cells)), shape.Cells, shape.AllowRotations)
	}
	return key.String()
}

func firstFreeCell(grid Grid) (Point, bool) {
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			point := Point{X: x, Y: y}
			if !grid.Occupied(point) {
				return point, true
			}
		}
	}
	return Point{}, false
}

func containsPoint(points []Point, target Point) bool {
	for _, point := range points {
		if point == target {
			return true
		}
	}
	return false
}

func clonePlacements(input []Placement) []Placement {
	result := make([]Placement, len(input))
	for i, placement := range input {
		result[i] = placement
		result[i].Cells = append([]Point(nil), placement.Cells...)
	}
	return result
}
