package main

import (
	"bytes"
	"strings"
	"testing"

	"nte-optimizer/internal/app"
	"nte-optimizer/internal/optimizer"
)

func TestRunRejectsInvalidMode(t *testing.T) {
	err := run([]string{"--character", "1036", "--method", "deep"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid method") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintReportExplainsApproximateRanking(t *testing.T) {
	request := app.SavedOptimizationRequest{Profile: app.ProfileSummary{ID: "example", Name: "Example", CharacterID: 1}, Goals: map[string]app.GoalTuning{"AtkFinal": {Target: 2000, Importance: .7}}}
	result := app.OptimizationResult{OptimizationMode: "fast", Stats: optimizer.StatSummary{Derived: map[string]float64{"AtkFinal": 1900}}, Solution: optimizer.Solution{Ranking: &optimizer.RankingBreakdown{Score: 5, Objectives: 5}}}
	var output bytes.Buffer
	if err := printReport(&output, request, result, 0); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"approximate; optimality not guaranteed", "AtkFinal", "1900.0000", "Objective fit", "DAMAGE PREVIEW"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("report missing %q:\n%s", expected, output.String())
		}
	}
}
