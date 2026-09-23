package optimizer

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// EquipmentTieBreakScale keeps equipment relevance available as a stable
// tie-breaker without allowing stats already present in objective values to be
// counted a second time in the primary ranking.
const EquipmentTieBreakScale = 1e-6

type RankingContribution struct {
	Source          string  `json:"source,omitempty"`
	InputPropertyID string  `json:"input_property_id,omitempty"`
	PropertyID      string  `json:"property_id"`
	Value           float64 `json:"value"`
	Target          float64 `json:"target"`
	Importance      float64 `json:"importance"`
	Points          float64 `json:"points"`
	ScoreScale      float64 `json:"score_scale,omitempty"`
}
type RankingBreakdown struct {
	Score         float64               `json:"score"`
	Equipment     float64               `json:"equipment"`
	TieBreak      float64               `json:"tie_break,omitempty"`
	Objectives    float64               `json:"objectives"`
	Structure     float64               `json:"structure"`
	Contributions []RankingContribution `json:"contributions"`
}
type Solution struct {
	Ranking             *RankingBreakdown `json:"ranking,omitempty"`
	Metrics             SearchMetrics     `json:"metrics"`
	Score               float64           `json:"score"`
	ModuleScore         float64           `json:"module_score"`
	SetBonusScore       float64           `json:"set_bonus_score"`
	CartridgeScore      float64           `json:"cartridge_score"`
	SelectedCartridgeID string            `json:"selected_cartridge_id,omitempty"`
	SelectedSetID       string            `json:"selected_set_id,omitempty"`
	SetMatchedCount     int               `json:"set_matched_count,omitempty"`
	Placements          []Placement       `json:"placements"`
	Complete            bool              `json:"complete"`
	Visited             uint64            `json:"visited_states"`
	Alternatives        []Solution        `json:"alternatives,omitempty"`
}

type SearchSolver struct {
	// Exact searches every supplied candidate and requires a complete grid.
	Exact         bool
	Measure       bool
	DisableGroups bool
	// Reference switch for exhaustive regression tests and measurements.
	DisableStatReduction bool
	Catalog              ShapeCatalog
	DisableBound         bool
	Progress             func(SearchProgress)
	BlueprintCache       *BlueprintCache
	KeepBest             int
	RequiredIDs          map[string]bool
	GeometryRequirements [][]string
}

type SearchProgress struct {
	Pruned     uint64  `json:"pruned_branches"`
	Finished   bool    `json:"finished"`
	Visited    uint64  `json:"visited"`
	Total      uint64  `json:"total,omitempty"`
	BestScore  float64 `json:"best_score"`
	ElapsedMS  int64   `json:"elapsed_ms"`
	Candidates int     `json:"candidates"`
	Workers    int     `json:"workers"`
}

type SearchMetrics struct {
	InputCandidates            int    `json:"input_candidates"`
	RetainedCandidates         int    `json:"retained_candidates"`
	DominatedCandidates        int    `json:"dominated_candidates"`
	StatProfiles               int    `json:"stat_profiles"`
	StatValues                 int    `json:"stat_values"`
	StatCacheHits              uint64 `json:"stat_cache_hits"`
	StatCacheMisses            uint64 `json:"stat_cache_misses"`
	Blueprints                 int    `json:"blueprints"`
	Theoretical                uint64 `json:"theoretical"`
	TheoreticalBeforeReduction uint64 `json:"theoretical_before_reduction"`
	ReductionNS                int64  `json:"reduction_ns"`
	Evaluated                  uint64 `json:"evaluated"`
	Pruned                     uint64 `json:"pruned_branches"`
	BlueprintNS                int64  `json:"blueprint_ns"`
	AssignmentNS               int64  `json:"assignment_ns"`
	EvaluationNS               int64  `json:"evaluation_ns"`
	CacheHit                   bool   `json:"cache_hit"`
}

type preparedCandidate struct {
	Candidate
	placements []Placement
	area       int
}

func (s SearchSolver) Solve(ctx context.Context, base Grid, candidates []Candidate) (Solution, error) {
	return s.SolveWithBonus(ctx, base, candidates, NoBonusEvaluator{})
}

func (s SearchSolver) SolveWithBonus(ctx context.Context, base Grid, candidates []Candidate, bonusEvaluator GlobalBonusEvaluator) (Solution, error) {
	return s.SolveWithBonusSeed(ctx, base, candidates, bonusEvaluator, nil)
}

func (s SearchSolver) SolveWithBonusSeed(ctx context.Context, base Grid, candidates []Candidate, bonusEvaluator GlobalBonusEvaluator, seed []Placement) (Solution, error) {
	if solution, covered, err := s.solveByBlueprint(ctx, base, candidates, bonusEvaluator, seed); err != nil || covered {
		return solution, err
	}
	if s.Exact {
		return Solution{Complete: ctx.Err() == nil}, nil
	}
	return s.solveLegacy(ctx, base, candidates, bonusEvaluator, seed)
}

func (s SearchSolver) solveLegacy(ctx context.Context, base Grid, candidates []Candidate, bonusEvaluator GlobalBonusEvaluator, seed []Placement) (Solution, error) {
	started := time.Now()
	prepared := make([]preparedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Score <= 0 && !s.RequiredIDs[candidate.Module.LocalID] {
			continue
		}
		shape, err := s.Catalog.Shape(candidate.Module.Geometry)
		if err != nil {
			return Solution{}, fmt.Errorf("module %s: %w", candidate.Module.LocalID, err)
		}
		placements := GeneratePlacements(base, candidate.Module.LocalID, shape)
		if len(placements) > 0 {
			prepared = append(prepared, preparedCandidate{Candidate: candidate, placements: placements, area: len(shape.Cells)})
		}
	}
	sort.SliceStable(prepared, func(i, j int) bool {
		leftRank, rightRank := prepared[i].Score, prepared[j].Score
		if prepared[i].Priority > 0 {
			leftRank = prepared[i].Priority
		}
		if prepared[j].Priority > 0 {
			rightRank = prepared[j].Priority
		}
		leftDensity := leftRank / float64(prepared[i].area)
		rightDensity := rightRank / float64(prepared[j].area)
		if leftDensity == rightDensity && prepared[i].Score == prepared[j].Score {
			return prepared[i].Module.LocalID < prepared[j].Module.LocalID
		}
		if leftDensity == rightDensity {
			return prepared[i].Score > prepared[j].Score
		}
		return leftDensity > rightDensity
	})

	best := Solution{Complete: true}
	if len(seed) > 0 {
		scores := make(map[string]float64, len(candidates))
		for _, candidate := range candidates {
			scores[candidate.Module.LocalID] = candidate.Score
		}
		moduleScore := 0.0
		for _, placement := range seed {
			moduleScore += scores[placement.ModuleID]
		}
		bonus := bonusEvaluator.Evaluate(seed)
		best = Solution{Score: moduleScore + bonus.Score, ModuleScore: moduleScore, SetBonusScore: bonus.SetBonusScore, CartridgeScore: bonus.CartridgeScore, SelectedCartridgeID: bonus.CartridgeID, SelectedSetID: bonus.SetID, SetMatchedCount: bonus.MatchedCount, Placements: append([]Placement(nil), seed...), Complete: true}
	}
	grid := base.Clone()
	if !placementsContainRequired(best.Placements, s.RequiredIDs) {
		best = Solution{Complete: true}
	}
	current := make([]Placement, 0)
	var search func(int, float64)
	search = func(index int, score float64) {
		best.Visited++
		if s.Progress != nil && best.Visited&16383 == 0 {
			s.Progress(SearchProgress{Visited: best.Visited, BestScore: best.Score, ElapsedMS: time.Since(started).Milliseconds(), Candidates: len(prepared), Workers: 1})
		}
		bonus := bonusEvaluator.Evaluate(current)
		totalScore := score + bonus.Score
		if (totalScore > best.Score || len(best.Placements) == 0 && len(current) > 0) && placementsContainRequired(current, s.RequiredIDs) {
			best.Score = totalScore
			best.ModuleScore = score
			best.SetBonusScore = bonus.SetBonusScore
			best.CartridgeScore = bonus.CartridgeScore
			best.SelectedCartridgeID = bonus.CartridgeID
			best.SelectedSetID = bonus.SetID
			best.SetMatchedCount = bonus.MatchedCount
			best.Placements = append([]Placement(nil), current...)
		}
		if best.Visited&1023 == 0 {
			select {
			case <-ctx.Done():
				best.Complete = false
				return
			default:
			}
		}
		if !best.Complete || (!s.DisableBound && score+fractionalBound(prepared, index, grid.FreeCells())+bonusEvaluator.UpperBound() <= best.Score) {
			return
		}
		if index == len(prepared) || grid.FreeCells() == 0 {
			return
		}
		candidate := prepared[index]
		for _, placement := range candidate.placements {
			if !grid.CanPlace(placement.Cells) {
				continue
			}
			_ = grid.Place(placement.Cells)
			current = append(current, placement)
			search(index+1, score+candidate.Score)
			current = current[:len(current)-1]
			grid.Remove(placement.Cells)
			if !best.Complete {
				return
			}
		}
		search(index+1, score)
	}
	search(0, 0)
	if s.Progress != nil {
		s.Progress(SearchProgress{Visited: best.Visited, BestScore: best.Score, ElapsedMS: time.Since(started).Milliseconds(), Candidates: len(prepared), Workers: 1})
	}
	if err := ctx.Err(); err != nil && best.Complete {
		best.Complete = false
	}
	return best, nil
}

// fractionalBound is an admissible fractional-knapsack upper bound. It ignores
// geometry and therefore can only overestimate the score still reachable.
func fractionalBound(candidates []preparedCandidate, index, capacity int) float64 {
	// Priority ordering need not follow score density. Unlimited supply of
	// the highest remaining density is an admissible relaxation of either.
	density := 0.0
	for ; index < len(candidates); index++ {
		if candidates[index].area > 0 {
			density = max(density, candidates[index].Score/float64(candidates[index].area))
		}
	}
	return density * float64(capacity)
}
