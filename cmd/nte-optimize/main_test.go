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

func TestPrintExperimentReportComparesActionsWithoutRankingVariants(t *testing.T) {
	// Exercise the rendering of the user's reference deltas without requiring
	// any search mode to reproduce those values.
	up, down := 5.2, -2.3
	basicDamageGain := 3.4
	fastScore, betaScore := 8888.0, 9999.0
	report := app.ExperimentReport{
		Profile:        app.ExperimentProfile{ID: "zankou", Name: "Zankou", CharacterID: 1036},
		Interpretation: app.ExperimentInterpretation{ActionDamageReference: "currently_equipped_build", DamageIsRotationDPS: false, RankingScoresComparableAcrossMethods: false},
		Text: app.ExperimentReportText{
			Profile: "Profil", InputSnapshot: "Instantané", SharedVariants: "partagé entre {count} variantes", Variant: "Variante", Method: "Méthode", Status: "État",
			RetainedEligible: "Retenus/éligibles", Visited: "États visités", Duration: "Durée", Arc: "Arc", BasicDamage: "Basic DMG", ActionDamageNote: "Dégâts attendus par action.",
			RotationNote: "Pas de DPS de rotation.", ScoreScaleNote: "Échelles de score distinctes.", ScoresNotCompared: "Scores non comparés entre méthodes.",
		},
		Variants: []app.ExperimentVariantReport{
			{Name: "fast", Status: "approximate", EffectiveConfig: app.ExperimentEffectiveConfig{Method: "fast"}, SelectedCandidates: 10, EligibleCandidates: 20, VisitedStates: 100, DurationMS: 25, RankingScore: &fastScore, BasicDamage: &app.ExperimentBasicDamage{BuildIndex: 1034, CurrentIndex: 1000, GainPercent: &basicDamageGain}, ActionDamage: []app.ExperimentActionDamage{{ID: "basic", Name: "Basic", GainPercent: &up}, {ID: "scorch", Name: "Scorch", GainPercent: &down}}},
			{Name: "beta", Status: "approximate", EffectiveConfig: app.ExperimentEffectiveConfig{Method: "beta"}, SelectedCandidates: 12, EligibleCandidates: 20, VisitedStates: 200, DurationMS: 40, RankingScore: &betaScore, BasicDamage: &app.ExperimentBasicDamage{BuildIndex: 1034, CurrentIndex: 1000, GainPercent: &basicDamageGain}, ActionDamage: []app.ExperimentActionDamage{{ID: "basic", Name: "Basic", GainPercent: &up}, {ID: "scorch", Name: "Scorch", GainPercent: &down}}},
		},
	}
	var output bytes.Buffer
	if err := printExperimentReport(&output, report); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"partagé entre 2 variantes", "Basic DMG Δ", "Basic Δ", "Scorch Δ", "+3.4%", "+5.2%", "-2.3%", "Scores non comparés entre méthodes."} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("comparison table missing %q:\n%s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "8888") || strings.Contains(output.String(), "9999") {
		t.Fatalf("comparison table must not compare internal ranking scales:\n%s", output.String())
	}
}
