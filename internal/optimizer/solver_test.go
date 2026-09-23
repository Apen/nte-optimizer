package optimizer

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nte-optimizer/internal/nte"
)

func TestSearchSolverFindsOptimalPacking(t *testing.T) {
	grid, _ := NewGrid(2, 2, nil)
	catalog := ShapeCatalog{Shapes: map[string]Shape{
		"DOMINO": {ID: "DOMINO", Cells: []Point{{0, 0}, {1, 0}}},
		"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}},
	}}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "domino_a", Geometry: "DOMINO"}, Score: 5},
		{Module: nte.Module{LocalID: "domino_b", Geometry: "DOMINO"}, Score: 4},
		{Module: nte.Module{LocalID: "single", Geometry: "SINGLE"}, Score: 3},
	}
	got, err := (SearchSolver{Catalog: catalog}).Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Complete || got.Score != 9 || len(got.Placements) != 2 {
		t.Fatalf("unexpected solution: %#v", got)
	}
}

func TestSearchSolverHonorsBlockedCells(t *testing.T) {
	grid, _ := NewGrid(2, 1, []Point{{1, 0}})
	catalog := ShapeCatalog{Shapes: map[string]Shape{
		"DOMINO": {ID: "DOMINO", Cells: []Point{{0, 0}, {1, 0}}},
		"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}},
	}}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "domino", Geometry: "DOMINO"}, Score: 10},
		{Module: nte.Module{LocalID: "single", Geometry: "SINGLE"}, Score: 2},
	}
	got, err := (SearchSolver{Catalog: catalog}).Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if got.Score != 2 || len(got.Placements) != 1 || got.Placements[0].ModuleID != "single" {
		t.Fatalf("unexpected solution: %#v", got)
	}
}

func TestSearchSolverReportsRealProgress(t *testing.T) {
	grid, _ := NewGrid(1, 1, nil)
	catalog := ShapeCatalog{Shapes: map[string]Shape{"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}}}}
	updates := []SearchProgress{}
	_, err := (SearchSolver{Catalog: catalog, Progress: func(progress SearchProgress) { updates = append(updates, progress) }}).Solve(context.Background(), grid, []Candidate{{Module: nte.Module{LocalID: "one", Geometry: "SINGLE"}, Score: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) == 0 || updates[len(updates)-1].Visited == 0 || updates[len(updates)-1].Candidates != 1 || updates[len(updates)-1].Total != 1 {
		t.Fatalf("unexpected progress: %#v", updates)
	}
}

func TestBlueprintAssignmentTotal(t *testing.T) {
	blueprints := []geometryBlueprint{{geometries: []string{"A", "A", "B"}}, {geometries: []string{"B"}}}
	byGeometry := map[string][]Candidate{"A": make([]Candidate, 4), "B": make([]Candidate, 3)}
	// C(4,2) * C(3,1) + C(3,1) = 21.
	if got := blueprintAssignmentTotal(blueprints, byGeometry); got != 21 {
		t.Fatalf("blueprintAssignmentTotal() = %d, want 21", got)
	}
}

func TestRunAssignmentWorkersProcessesEveryJob(t *testing.T) {
	jobs := make([]assignmentJob, 20)
	var stopped atomic.Bool
	var evaluated atomic.Int64
	runAssignmentWorkers(jobs, 4, &stopped, func(assignmentJob) { evaluated.Add(1) })
	if got := evaluated.Load(); got != int64(len(jobs)) {
		t.Fatalf("evaluated jobs = %d, want %d", got, len(jobs))
	}
}

func TestFinalizeBlueprintSolutionBuildsMetricsAndAlternatives(t *testing.T) {
	best := Solution{Score: 10}
	leaders := []Solution{best, {Score: 9}}
	got := finalizeBlueprintSolution(SearchMetrics{}, best, leaders, 12, 3, 2*time.Millisecond, 400, 5, 2, true)
	if !got.Complete || got.Visited != 12 || len(got.Alternatives) != 1 {
		t.Fatalf("unexpected finalized solution: %#v", got)
	}
	if got.Metrics.Evaluated != 12 || got.Metrics.Pruned != 3 || got.Metrics.AssignmentNS != (2*time.Millisecond).Nanoseconds() || got.Metrics.EvaluationNS != 400 || got.Metrics.StatCacheHits != 5 || got.Metrics.StatCacheMisses != 2 {
		t.Fatalf("unexpected finalized metrics: %#v", got.Metrics)
	}
}

func TestPrepareCandidatePoolsFiltersValidatesAndSorts(t *testing.T) {
	catalog := ShapeCatalog{Shapes: map[string]Shape{"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}}}}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "low", Geometry: "SINGLE"}, Score: 1, Priority: 1},
		{Module: nte.Module{LocalID: "high", Geometry: "SINGLE"}, Score: 2, Priority: 2},
		{Module: nte.Module{LocalID: "zero", Geometry: "SINGLE"}, Score: 0},
	}
	usable, byGeometry, err := (SearchSolver{Catalog: catalog}).prepareCandidatePools(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(usable) != 2 || len(byGeometry["SINGLE"]) != 2 || byGeometry["SINGLE"][0].Module.LocalID != "high" {
		t.Fatalf("unexpected candidate pools: %#v, %#v", usable, byGeometry)
	}
	if _, _, err := (SearchSolver{Catalog: catalog}).prepareCandidatePools([]Candidate{{Module: nte.Module{LocalID: "bad", Geometry: "UNKNOWN"}, Score: 1}}); err == nil {
		t.Fatal("unknown geometry was accepted")
	}
	exact, _, err := (SearchSolver{Catalog: catalog, Exact: true}).prepareCandidatePools(candidates)
	if err != nil || len(exact) != 3 {
		t.Fatalf("exact candidate pool = %#v, %v", exact, err)
	}
}

func TestPrepareAssignmentJobsSplitsLargeSingleBlueprint(t *testing.T) {
	blueprint := geometryBlueprint{geometries: []string{"A"}}
	pool := map[string][]Candidate{"A": make([]Candidate, 4)}
	jobs, workers := prepareAssignmentJobs([]geometryBlueprint{blueprint}, pool, 50001, 4)
	if len(jobs) != 4 || workers != 4 {
		t.Fatalf("jobs=%d workers=%d, want 4/4", len(jobs), workers)
	}
	for index, job := range jobs {
		if job.first != index {
			t.Fatalf("job %d first=%d", index, job.first)
		}
	}
}

func TestPrepareAssignmentJobsKeepsSmallSearchesWhole(t *testing.T) {
	blueprint := geometryBlueprint{geometries: []string{"A"}}
	jobs, workers := prepareAssignmentJobs([]geometryBlueprint{blueprint}, map[string][]Candidate{"A": make([]Candidate, 4)}, 50000, 8)
	if len(jobs) != 1 || jobs[0].first != -1 || workers != 1 {
		t.Fatalf("unexpected jobs: %#v, workers=%d", jobs, workers)
	}
}

func TestSolutionLeaderboardMergesConcurrentResults(t *testing.T) {
	board := newSolutionLeaderboard(Solution{}, 3)
	var workers sync.WaitGroup
	for score := 1; score <= 10; score++ {
		workers.Add(1)
		go func(score int) {
			defer workers.Done()
			board.merge([]Solution{{Score: float64(score), Placements: []Placement{{ModuleID: string(rune('a' + score))}}}})
		}(score)
	}
	workers.Wait()
	best, leaders := board.snapshot()
	if best.Score != 10 || len(leaders) != 3 || leaders[0].Score != 10 || leaders[1].Score != 9 || leaders[2].Score != 8 {
		t.Fatalf("unexpected leaderboard: best=%#v leaders=%#v", best, leaders)
	}
}

func TestSearchSolverCachesGeometryBlueprints(t *testing.T) {
	grid, _ := NewGrid(2, 1, nil)
	catalog := ShapeCatalog{Shapes: map[string]Shape{"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}}}}
	cache := NewBlueprintCache()
	solver := SearchSolver{Catalog: catalog, BlueprintCache: cache}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "one", Geometry: "SINGLE"}, Score: 1},
		{Module: nte.Module{LocalID: "two", Geometry: "SINGLE"}, Score: 1},
	}
	if _, err := solver.Solve(context.Background(), grid, candidates); err != nil {
		t.Fatal(err)
	}
	if cache.Len() != 1 {
		t.Fatalf("cache contains %d entries, want 1", cache.Len())
	}
	if _, err := solver.Solve(context.Background(), grid, candidates); err != nil {
		t.Fatal(err)
	}
	if cache.Len() != 1 {
		t.Fatalf("cache grew after identical search: %d", cache.Len())
	}
}

func TestSearchSolverKeepsDistinctBestSolutions(t *testing.T) {
	grid, _ := NewGrid(2, 1, nil)
	catalog := ShapeCatalog{Shapes: map[string]Shape{"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}}}}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "one", Geometry: "SINGLE"}, Score: 3},
		{Module: nte.Module{LocalID: "two", Geometry: "SINGLE"}, Score: 2},
		{Module: nte.Module{LocalID: "three", Geometry: "SINGLE"}, Score: 1},
	}
	got, err := (SearchSolver{Catalog: catalog, KeepBest: 3}).Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if got.Score != 5 || len(got.Alternatives) != 2 || got.Alternatives[0].Score != 4 || got.Alternatives[1].Score != 3 {
		t.Fatalf("unexpected ranked solutions: %#v", got)
	}
}

func TestSearchSolverRequiresLockedModule(t *testing.T) {
	grid, _ := NewGrid(2, 1, nil)
	catalog := ShapeCatalog{Shapes: map[string]Shape{"SINGLE": {ID: "SINGLE", Cells: []Point{{0, 0}}}}}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "best", Geometry: "SINGLE"}, Score: 10},
		{Module: nte.Module{LocalID: "second", Geometry: "SINGLE"}, Score: 9},
		{Module: nte.Module{LocalID: "locked", Geometry: "SINGLE"}, Score: 1},
	}
	got, err := (SearchSolver{Catalog: catalog, RequiredIDs: map[string]bool{"locked": true}}).Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if got.Score != 11 || !placementsContainRequired(got.Placements, map[string]bool{"locked": true}) {
		t.Fatalf("locked module missing from solution: %#v", got)
	}
}

func TestCompatibleBlueprintsRejectsLayoutsWithoutCompleteOwnedSet(t *testing.T) {
	blueprints := []geometryBlueprint{
		{geometries: []string{"A", "B", "C"}},
		{geometries: []string{"A", "A", "C"}},
		{geometries: []string{"B", "C", "D"}},
	}
	got := compatibleBlueprints(blueprints, [][]string{{"A", "B"}, {"C", "D"}})
	if len(got) != 2 || got[0].geometries[1] != "B" || got[1].geometries[0] != "B" {
		t.Fatalf("unexpected compatible blueprints: %#v", got)
	}
}

func TestSelectCandidatesTakesUnionOfGeometryAndSetBuckets(t *testing.T) {
	input := []Candidate{
		{Module: nte.Module{LocalID: "a", Geometry: "H", SetID: "one"}, Score: 10},
		{Module: nte.Module{LocalID: "b", Geometry: "H", SetID: "two"}, Score: 9},
		{Module: nte.Module{LocalID: "c", Geometry: "V", SetID: "one"}, Score: 8},
	}
	got := SelectCandidates(input, 1, 1)
	if len(got) != 3 {
		t.Fatalf("selected %d candidates, want union of all 3: %#v", len(got), got)
	}
}

func TestSelectObjectiveCandidatesKeepsBreakpointSpecialists(t *testing.T) {
	input := []Candidate{
		{Module: nte.Module{LocalID: "general", Geometry: "H_2"}, Score: 10, Priority: 10, ObjectiveValues: map[string]float64{"CritBase": .02}},
		{Module: nte.Module{LocalID: "crit", Geometry: "H_2"}, Score: 5, Priority: 5, ObjectiveValues: map[string]float64{"CritBase": .10}},
		{Module: nte.Module{LocalID: "weak", Geometry: "H_2"}, Score: 1, Priority: 1, ObjectiveValues: map[string]float64{"CritBase": .01}},
	}
	got := SelectObjectiveCandidates(input, []ObjectiveGoal{{PropertyID: "CritBase", Minimum: .75}}, 1, 1)
	if len(got) != 2 || got[0].Module.LocalID != "general" || got[1].Module.LocalID != "crit" {
		t.Fatalf("breakpoint specialist was lost: %#v", got)
	}
}

func TestSelectObjectiveCandidatesKeepsMultipleSpecialists(t *testing.T) {
	input := []Candidate{
		{Module: nte.Module{LocalID: "general", Geometry: "H_2"}, Priority: 10, ObjectiveValues: map[string]float64{"UnbalIntensityBase": 0}},
		{Module: nte.Module{LocalID: "break-a", Geometry: "H_2"}, Priority: 2, ObjectiveValues: map[string]float64{"UnbalIntensityBase": 24}},
		{Module: nte.Module{LocalID: "break-b", Geometry: "H_2"}, Priority: 1, ObjectiveValues: map[string]float64{"UnbalIntensityBase": 18}},
	}
	got := SelectObjectiveCandidates(input, []ObjectiveGoal{{PropertyID: "UnbalIntensityBase", Minimum: 360}}, 1, 2)
	if len(got) != 3 {
		t.Fatalf("multiple Break specialists were dropped: %#v", got)
	}
}

func TestBlueprintSolverEvaluatesEachItemCombinationOnce(t *testing.T) {
	grid, _ := NewGrid(2, 2, nil)
	catalog := ShapeCatalog{Shapes: map[string]Shape{
		"DOMINO": {ID: "DOMINO", Cells: []Point{{0, 0}, {1, 0}}},
	}}
	candidates := []Candidate{
		{Module: nte.Module{LocalID: "a", Geometry: "DOMINO"}, Score: 4},
		{Module: nte.Module{LocalID: "b", Geometry: "DOMINO"}, Score: 3},
		{Module: nte.Module{LocalID: "c", Geometry: "DOMINO"}, Score: 2},
		{Module: nte.Module{LocalID: "d", Geometry: "DOMINO"}, Score: 1},
	}
	got, err := (SearchSolver{Catalog: catalog, DisableBound: true}).Solve(context.Background(), grid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Complete || got.Score != 7 || got.Visited != 6 {
		t.Fatalf("unexpected blueprint solution: %#v", got)
	}
}
