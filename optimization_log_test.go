package main

import "testing"

func TestPositiveWeightCount(t *testing.T) {
	weights := map[string]float64{"positive": 1, "zero": 0, "negative": -1}
	if got := positiveWeightCount(weights); got != 1 {
		t.Fatalf("positiveWeightCount() = %d, want 1", got)
	}
}

func TestStringSetIgnoresEmptyValues(t *testing.T) {
	got := stringSet([]string{"module-1", "", "module-1", "module-2"})
	if len(got) != 2 || !got["module-1"] || !got["module-2"] {
		t.Fatalf("stringSet() = %#v", got)
	}
}

func TestLastOptimizationLogReturnsEntryCopy(t *testing.T) {
	app := &DesktopApp{lastLog: OptimizationLog{Entries: []OptimizationLogEntry{{Message: "original"}}}}
	first := app.LastOptimizationLog()
	first.Entries[0].Message = "changed"
	second := app.LastOptimizationLog()
	if got := second.Entries[0].Message; got != "original" {
		t.Fatalf("stored log was modified through returned value: %q", got)
	}
}
