package app

import (
	"context"
	"nte-optimizer/internal/optimizer"
	"os"
	"testing"
	"time"
)

// Explicit opt-in keeps private account data out of ordinary test/CI runs.
func BenchmarkRealInventory(b *testing.B) {
	if os.Getenv("NTE_REAL_BENCH") != "1" {
		b.Skip("set NTE_REAL_BENCH=1 to measure workspace inventory")
	}
	b.Run("fast", func(b *testing.B) {
		mode := "fast"
		service := OptimizerService{Measure: true, DataDir: "../../data", blueprints: optimizer.NewBlueprintCache()}
		b.ReportAllocs()
		for b.Loop() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			result, err := service.OptimizeFlexibleProject(ctx, "../..", "zankou", false, "fr", mode, nil)
			cancel()
			if err != nil {
				b.Fatal(err)
			}
			if result.Solution.SelectedSetID != "Suit4" {
				b.Fatalf("selected strategy set ignored: %s", result.Solution.SelectedSetID)
			}
			for _, alternative := range result.Alternatives {
				if alternative.Solution.SelectedSetID != "Suit4" {
					b.Fatalf("alternative ignored strategy set: %s", alternative.Solution.SelectedSetID)
				}
			}
			b.Logf("mode=%s eligible=%d selected=%d complete=%v modules=%d score=%g metrics=%+v", mode, result.EligibleCandidates, result.SelectedCandidates, result.Solution.Complete, len(result.Modules), result.Solution.Score, result.Solution.Metrics)
			b.ReportMetric(float64(result.Solution.Visited), "leaves/op")
			b.Logf("phase_ms=%v", result.PhaseMS)
		}
	})
}
