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
		{Level: "INFO", Stage: "configuration", Message: fmt.Sprintf("profil=%s méthode=%s objectifs=%d", profileID, mode, len(goals))},
		{Level: "DEBUG", Stage: "contraintes", Message: fmt.Sprintf("modules verrouillés=%d exclus=%d", len(pinned), len(excluded))},
	}
	goalNames := make([]string, 0, len(goals))
	for name := range goals {
		goalNames = append(goalNames, name)
	}
	sort.Strings(goalNames)
	for _, name := range goalNames {
		goal := goals[name]
		entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "objectif", Message: fmt.Sprintf("%s cible=%.4f poids=%.2f maximum=%.4f tolérance=%.2f%% minimum_strict=%t", name, goal.Target, goal.Importance, goal.Maximum, goal.Tolerance*100, goal.StrictMinimum)})
	}
	if weights != nil {
		entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "configuration", Message: fmt.Sprintf("stats principales=%d poids actifs=%d", len(weights.MainStats), positiveWeightCount(weights.Weights))})
		for _, name := range weights.MainStats {
			entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "principale", Message: name})
		}
		weightNames := make([]string, 0, len(weights.Weights))
		for name, weight := range weights.Weights {
			if weight > 0 {
				weightNames = append(weightNames, name)
			}
		}
		sort.Strings(weightNames)
		for _, name := range weightNames {
			entries = append(entries, OptimizationLogEntry{Level: "DEBUG", Stage: "poids", Message: fmt.Sprintf("%s=%.2f", name, weights.Weights[name])})
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
			OptimizationLogEntry{Level: "ERROR", Stage: "arrêt", Message: runErr.Error()},
			OptimizationLogEntry{Level: "DEBUG", Stage: "recherche", Message: fmt.Sprintf("visités=%d total=%d candidats=%d branches écartées=%d durée=%d ms", progress.Visited, progress.Total, progress.Candidates, progress.Pruned, progress.ElapsedMS)},
		)
		return
	}
	log.Status = "success"
	rankingScore := result.Solution.Score
	if result.Solution.Ranking != nil {
		rankingScore = result.Solution.Ranking.Score
	}
	log.Entries = append(log.Entries,
		OptimizationLogEntry{Level: "INFO", Stage: "sélection", Message: fmt.Sprintf("modules inventaire=%d admissibles=%d retenus=%d réservés écartés=%d", result.InventoryModules, result.EligibleCandidates, result.SelectedCandidates, result.ExcludedEquipped)},
		OptimizationLogEntry{Level: "INFO", Stage: "résultat", Message: fmt.Sprintf("score=%.4f builds=%d complet=%t", rankingScore, len(result.Alternatives)+1, result.Solution.Complete)},
	)
	phaseNames := make([]string, 0, len(result.PhaseMS))
	for name := range result.PhaseMS {
		phaseNames = append(phaseNames, name)
	}
	sort.Strings(phaseNames)
	for _, name := range phaseNames {
		log.Entries = append(log.Entries, OptimizationLogEntry{Level: "DEBUG", Stage: "temps", Message: fmt.Sprintf("%s=%d ms", name, result.PhaseMS[name])})
	}
	m := result.Solution.Metrics
	log.Entries = append(log.Entries,
		OptimizationLogEntry{Level: "DEBUG", Stage: "réduction", Message: fmt.Sprintf("entrée=%d retenus=%d dominés=%d profils=%d valeurs=%d", m.InputCandidates, m.RetainedCandidates, m.DominatedCandidates, m.StatProfiles, m.StatValues)},
		OptimizationLogEntry{Level: "DEBUG", Stage: "recherche", Message: fmt.Sprintf("blueprints=%d théorique=%d avant réduction=%d évalués=%d élagués=%d cache=%t", m.Blueprints, m.Theoretical, m.TheoreticalBeforeReduction, m.Evaluated, m.Pruned, m.CacheHit)},
	)
	if !result.Solution.Complete {
		log.Entries = append(log.Entries, OptimizationLogEntry{Level: "WARN", Stage: "résultat", Message: "optimalité non confirmée : recherche approximative ou interrompue"})
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
