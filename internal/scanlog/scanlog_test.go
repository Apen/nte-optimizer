package scanlog

import (
	"path/filepath"
	"testing"
)

func TestStartAppendAndRead(t *testing.T) {
	path := Path(t.TempDir())
	if err := Start(path, 35); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, "capture", "complete", map[string]int{"packets": 42}); err != nil {
		t.Fatal(err)
	}
	events, err := Read(path)
	if err != nil || len(events) != 2 || events[0].Counts["duration_seconds"] != 35 || events[1].Counts["packets"] != 42 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	if err := Start(path, 60); err != nil {
		t.Fatal(err)
	}
	events, err = Read(path)
	if err != nil || len(events) != 1 || events[0].Counts["duration_seconds"] != 60 {
		t.Fatalf("new scan did not replace the old log: %#v, %v", events, err)
	}
	missing, err := Read(filepath.Join(t.TempDir(), "missing.jsonl"))
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing log=%#v err=%v", missing, err)
	}
}
