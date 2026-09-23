package app

import (
	"context"
	"fmt"
	"time"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

type searchSetup struct {
	evaluator      optimizer.GlobalBonusEvaluator
	solver         optimizer.SearchSolver
	timeoutSeconds int
}

type searchRequest struct {
	grid     optimizer.Grid
	setup    searchSetup
	selected []optimizer.Candidate
}

type searchExecution struct {
	solution optimizer.Solution
	searchMS int64
}

func (s OptimizerService) prepareSearch(profile scoring.Character, refs scoring.References, shapes optimizer.ShapeCatalog, sets optimizer.SetCatalog, config optimizerDataConfig, selected []optimizer.Candidate, cartridges []nte.Cartridge, objectives []optimizer.ObjectiveGoal, additional map[string][]nte.Stat, plan searchPlan) searchSetup {
	evaluator := optimizer.GlobalBonusEvaluator(optimizer.NewCartridgeSetEvaluator(sets, cartridges, profile.PreferredSets, selected, profile, refs))
	timeoutSeconds := config.Optimize.TimeoutSeconds
	exact := !plan.approximate || plan.solverMode == "objective"
	if exact {
		evaluator = optimizer.NewExactObjectiveEvaluator(sets, cartridges, profile.PreferredSets, selected, profile, refs, objectives, additional)
		switch {
		case plan.requestedMode == "exact-score" || plan.requestedMode == "exact-objective":
			timeoutSeconds = 0
		}
	}
	return searchSetup{
		evaluator:      evaluator,
		timeoutSeconds: timeoutSeconds,
		solver: optimizer.SearchSolver{
			Measure:              s.Measure,
			Exact:                exact,
			Catalog:              shapes,
			Progress:             s.Progress,
			BlueprintCache:       s.blueprints,
			KeepBest:             50,
			RequiredIDs:          s.PinnedModuleIDs,
			GeometryRequirements: ownedSetGeometryRequirements(cartridges, sets),
		},
	}
}

func (s OptimizerService) executeSearch(ctx context.Context, request searchRequest) (searchExecution, error) {
	started := time.Now()
	searchContext := ctx
	cancel := func() {}
	if request.setup.timeoutSeconds > 0 {
		searchContext, cancel = context.WithTimeout(ctx, time.Duration(request.setup.timeoutSeconds)*time.Second)
	}
	defer cancel()

	solution, err := request.setup.solver.SolveWithBonus(searchContext, request.grid, request.selected, request.setup.evaluator)
	if err != nil {
		return searchExecution{}, err
	}

	if request.setup.solver.Exact && len(request.selected) > 0 && len(solution.Placements) == 0 {
		if searchContext.Err() != nil {
			return searchExecution{}, fmt.Errorf("search interrupted before finding a valid build: %w", searchContext.Err())
		}
		return searchExecution{}, fmt.Errorf("no complete build satisfies the required shapes, tolerated minimums, and requested maximums")
	}
	return searchExecution{solution: solution, searchMS: time.Since(started).Milliseconds()}, nil
}
