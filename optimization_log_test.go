package main

import (
	"errors"
	appservice "nte-optimizer/internal/app"
	"nte-optimizer/internal/optimizer"
	"testing"
)

func TestOldSearchCannotOverwriteNewLog(t *testing.T) {
	app := &DesktopApp{}
	first := app.beginOptimizationLog("first", "fast", nil, nil, nil, nil)
	second := app.beginOptimizationLog("second", "fast", nil, nil, nil, nil)
	if app.recordOptimizationProgress(first, optimizer.SearchProgress{Visited: 999}) {
		t.Fatal("accepted stale progress")
	}
	app.finishOptimizationLog(first, appservice.OptimizationResult{}, errors.New("old failure"))
	if app.LastOptimizationLog().Status != "running" {
		t.Fatal("old completion replaced current log")
	}
	if !app.recordOptimizationProgress(second, optimizer.SearchProgress{Visited: 1}) {
		t.Fatal("rejected current progress")
	}
	app.finishOptimizationLog(second, appservice.OptimizationResult{}, nil)
	if app.LastOptimizationLog().Status != "success" {
		t.Fatal("current completion missing")
	}
}

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
