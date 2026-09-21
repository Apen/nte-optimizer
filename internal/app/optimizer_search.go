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
	grid                optimizer.Grid
	setup               searchSetup
	selected            []optimizer.Candidate
	allCandidates       []optimizer.Candidate
	rawCandidates       []optimizer.Candidate
	selectionObjectives []optimizer.ObjectiveGoal
	objectives          []optimizer.ObjectiveGoal
	cartridges          []nte.Cartridge
	profile             scoring.Character
	refs                scoring.References
	sets                optimizer.SetCatalog
	additional          map[string][]nte.Stat
	config              optimizerDataConfig
	plan                searchPlan
	perGeometry         int
}

type searchExecution struct {
	solution     optimizer.Solution
	searchMS     int64
	refinementMS int64
}

func (s OptimizerService) prepareSearch(profile scoring.Character, refs scoring.References, shapes optimizer.ShapeCatalog, sets optimizer.SetCatalog, config optimizerDataConfig, selected []optimizer.Candidate, cartridges []nte.Cartridge, objectives []optimizer.ObjectiveGoal, additional map[string][]nte.Stat, plan searchPlan) searchSetup {
	evaluator := optimizer.GlobalBonusEvaluator(optimizer.NewCartridgeSetEvaluator(sets, cartridges, profile.PreferredSets, selected, profile, refs))
	timeoutSeconds := config.Optimize.TimeoutSeconds
	if plan.compromise && timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}
	exact := !plan.approximate || plan.solverMode == "balanced"
	if exact {
		evaluator = optimizer.NewExactObjectiveEvaluator(sets, cartridges, profile.PreferredSets, selected, profile, refs, objectives, additional)
		switch {
		case plan.requestedMode == "exact-score":
			timeoutSeconds = 0
		case plan.requestedMode == "balanced" && timeoutSeconds < 15:
			timeoutSeconds = 15
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

	seed := []optimizer.Placement(nil)
	if request.plan.requestedMode == "balanced" {
		quickCandidates := quickSearchCandidates(request.allCandidates, request.rawCandidates, request.selectionObjectives, request.config, request.perGeometry, s.PinnedModuleIDs)
		seedContext, seedCancel := context.WithTimeout(searchContext, 2*time.Second)
		seedSolver := request.setup.solver
		seedSolver.Exact = false
		seedSolver.KeepBest = 1
		seedSolution, seedErr := seedSolver.SolveWithBonus(seedContext, request.grid, quickCandidates, request.setup.evaluator)
		seedCancel()
		if seedErr == nil && len(seedSolution.Placements) > 0 && seedSolution.SelectedCartridgeID != "" {
			seed = seedSolution.Placements
		}
	}

	initialContext := searchContext
	initialCancel := func() {}
	if request.plan.compromise {
		initialContext, initialCancel = context.WithTimeout(searchContext, time.Duration(request.setup.timeoutSeconds)*time.Second*7/10)
	}
	solution, err := request.setup.solver.SolveWithBonusSeed(initialContext, request.grid, request.selected, request.setup.evaluator, seed)
	initialCancel()
	if err != nil {
		return searchExecution{}, err
	}

	refinementMS := int64(0)
	if request.plan.compromise && searchContext.Err() == nil {
		refinementStarted := time.Now()
		fullEvaluator := optimizer.NewExactObjectiveEvaluator(request.sets, request.cartridges, request.profile.PreferredSets, request.allCandidates, request.profile, request.refs, request.objectives, request.additional)
		if len(solution.Placements) == 0 {
			solution, err = request.setup.solver.SolveWithBonus(searchContext, request.grid, request.allCandidates, fullEvaluator)
			if err != nil {
				return searchExecution{}, err
			}
		} else {
			solution = optimizer.RefineCompromise(searchContext, solution, request.allCandidates, fullEvaluator, s.PinnedModuleIDs, request.setup.solver.KeepBest)
		}
		refinementMS = time.Since(refinementStarted).Milliseconds()
	}
	if request.setup.solver.Exact && len(request.selected) > 0 && len(solution.Placements) == 0 {
		if searchContext.Err() != nil {
			return searchExecution{}, fmt.Errorf("recherche interrompue avant de trouver un build admissible: %w", searchContext.Err())
		}
		return searchExecution{}, fmt.Errorf("aucun build complet ne respecte les formes, minimums tolérés et maximums demandés")
	}
	return searchExecution{solution: solution, searchMS: time.Since(started).Milliseconds() - refinementMS, refinementMS: refinementMS}, nil
}

func quickSearchCandidates(candidates, rawCandidates []optimizer.Candidate, objectives []optimizer.ObjectiveGoal, config optimizerDataConfig, perGeometry int, pinned map[string]bool) []optimizer.Candidate {
	quick := optimizer.SelectObjectiveCandidates(candidates, objectives, max(perGeometry, 4), 1)
	base := optimizer.SelectCandidates(rawCandidates, config.Optimize.TopPerGeometry, config.Optimize.TopPerSet)
	quick = appendUniqueCandidates(quick, base)
	return appendPinnedCandidates(quick, candidates, pinned)
}
