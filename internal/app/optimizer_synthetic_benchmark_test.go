package app

import (
	"context"
	"testing"
)

// BenchmarkSanitizedSearchModes compares mode costs on a deterministic,
// synthetic inventory fixture and never reads account data.
func BenchmarkSanitizedSearchModes(b *testing.B) {
	fixture := newSyntheticModeFixture(false)
	for _, mode := range []string{"fast", "beta"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			var visited uint64
			var selected, eligible int
			for i := 0; i < b.N; i++ {
				run, err := runSyntheticMode(context.Background(), fixture, mode)
				if err != nil {
					b.Fatal(err)
				}
				visited += run.solution.Visited
				selected += len(run.selected)
				eligible += len(run.pool.eligible)
			}
			if b.N > 0 {
				b.ReportMetric(float64(visited)/float64(b.N), "visited/op")
				b.ReportMetric(float64(selected)/float64(b.N), "selected/op")
				b.ReportMetric(float64(eligible)/float64(b.N), "eligible/op")
			}
		})
	}
}
