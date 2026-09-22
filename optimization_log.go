package main

import (
	"fmt"
	"sort"
	"time"

	appservice "nte-optimizer/internal/app"
	"nte-optimizer/internal/optimizer"
)

type OptimizationLogEntry struct {
	Level   string `json:"level"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type OptimizationLog struct {
	StartedAt  string                 `json:"started_at"`
	FinishedAt string                 `json:"finished_at,omitempty"`
	Status     string                 `json:"status"`
	Entries    []OptimizationLogEntry `json:"entries"`
}

func (a *DesktopApp) LastOptimizationLog() OptimizationLog {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	result := a.lastLog
	result.Entries = append([]OptimizationLogEntry(nil), a.lastLog.Entries...)
	return result
}

func (a *DesktopApp) beginOptimizationLog(profileID, mode string, goals map[string]appservice.GoalTuning, pinned, excluded []string, weights *appservice.WeightOverrides) {
	entries := []OptimizationLogEntry{
		{Level: "INFO", Stage: "configuration", Message: fmt.Sprintf("profile=%s method=%s objectives=%d", profileID, mode, len(goals))},
		{Level: "DEBUG", Stage: "constraints", Message: fmt.Sprintf("locked_modules=%d excluded=%d", len(pinned), len(excluded))},
	}
	goalNames := make([]string, 0, len(goals))
	for name := range goals {
		goalNames = append(goalNames, name)
	}
	sort.Strings(goalNames)
	for _, name := range goalNames {
		goal := goals[name]
		entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "objective", Message: fmt.Sprintf("%s target=%.4f weight=%.2f maximum=%.4f tolerance=%.2f%% strict_minimum=%t", name, goal.Target, goal.Importance, goal.Maximum, goal.Tolerance*100, goal.StrictMinimum)})
	}
	if weights != nil {
		entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "configuration", Message: fmt.Sprintf("main_stats=%d active_weights=%d", len(weights.MainStats), positiveWeightCount(weights.Weights))})
		for _, name := range weights.MainStats {
			entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "main_stat", Message: name})
		}
		weightNames := make([]string, 0, len(weights.Weights))
		for name, weight := range weights.Weights {
			if weight > 0 {
				weightNames = append(weightNames, name)
			}
		}
		sort.Strings(weightNames)
		for _, name := range weightNames {
			entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "weight", Message: fmt.Sprintf("%s=%.2f", name, weights.Weights[name])})
		}
	}
	a.logMu.Lock()
	a.lastLog = OptimizationLog{StartedAt: time.Now().Format(time.RFC3339), Status: "running", Entries: entries}
	a.logProgress = optimizer.SearchProgress{}
	a.logMu.Unlock()
}

func positiveWeightCount(weights map[string]float64) int {
	count := 0
	for _, weight := range weights {
		if weight > 0 {
			count++
		}
	}
	return count
}

func (a *DesktopApp) finishOptimizationLog(result appservice.OptimizationResult, runErr error) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	log := &a.lastLog
	log.FinishedAt = time.Now().Format(time.RFC3339)
	progress := a.logProgress
	if runErr != nil {
		log.Status = "error"
		log.Entries = append(log.Entries,
			OptimizationLogEntry{Level: "ERROR", Stage: "stop", Message: runErr.Error()},
			OptimizationLogEntry{Level: "DEBUG", Stage: "search", Message: fmt.Sprintf("visited=%d total=%d candidates=%d pruned=%d duration=%d ms", progress.Visited, progress.Total, progress.Candidates, progress.Pruned, progress.ElapsedMS)},
		)
		return
	}
	log.Status = "success"
	rankingScore := result.Solution.Score
	if result.Solution.Ranking != nil {
		rankingScore = result.Solution.Ranking.Score
	}
	log.Entries = append(log.Entries,
		OptimizationLogEntry{Level: "INFO", Stage: "selection", Message: fmt.Sprintf("inventory_modules=%d eligible=%d selected=%d reserved_excluded=%d", result.InventoryModules, result.EligibleCandidates, result.SelectedCandidates, result.ExcludedEquipped)},
		OptimizationLogEntry{Level: "INFO", Stage: "result", Message: fmt.Sprintf("score=%.4f builds=%d complete=%t", rankingScore, len(result.Alternatives)+1, result.Solution.Complete)},
	)
	phaseNames := make([]string, 0, len(result.PhaseMS))
	for name := range result.PhaseMS {
		phaseNames = append(phaseNames, name)
	}
	sort.Strings(phaseNames)
	for _, name := range phaseNames {
		log.Entries = append(log.Entries, OptimizationLogEntry{Level: "DEBUG", Stage: "timing", Message: fmt.Sprintf("%s=%d ms", name, result.PhaseMS[name])})
	}
	m := result.Solution.Metrics
	log.Entries = append(log.Entries,
		OptimizationLogEntry{Level: "DEBUG", Stage: "reduction", Message: fmt.Sprintf("input=%d retained=%d dominated=%d profiles=%d values=%d", m.InputCandidates, m.RetainedCandidates, m.DominatedCandidates, m.StatProfiles, m.StatValues)},
		OptimizationLogEntry{Level: "DEBUG", Stage: "search", Message: fmt.Sprintf("blueprints=%d theoretical=%d before_reduction=%d evaluated=%d pruned=%d cache=%t", m.Blueprints, m.Theoretical, m.TheoreticalBeforeReduction, m.Evaluated, m.Pruned, m.CacheHit)},
	)
	if !result.Solution.Complete {
		log.Entries = append(log.Entries, OptimizationLogEntry{Level: "WARN", Stage: "result", Message: "optimality not confirmed: the search was approximate or interrupted"})
	}
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = true
		}
	}
	return result
}
