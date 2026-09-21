package buildinfo

import "testing"

func TestWindowTitleOmitsDevelopmentVersion(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })

	Version = "dev"
	if got := WindowTitle("NTE Optimizer"); got != "NTE Optimizer" {
		t.Fatalf("WindowTitle() = %q", got)
	}

	Version = "v1.2.3"
	if got := WindowTitle("NTE Optimizer"); got != "NTE Optimizer v1.2.3" {
		t.Fatalf("WindowTitle() = %q", got)
	}
}

func TestCurrentReturnsInjectedValues(t *testing.T) {
	previousVersion, previousCommit, previousBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = previousVersion, previousCommit, previousBuildDate })

	Version, Commit, BuildDate = "v2", "abc123", "2026-09-21T12:00:00Z"
	got := Current()
	if got.Version != Version || got.Commit != Commit || got.BuildDate != BuildDate {
		t.Fatalf("Current() = %#v", got)
	}
}
