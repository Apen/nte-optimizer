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

func TestAppendEventPreservesDecoderTimestamp(t *testing.T) {
	path := Path(t.TempDir())
	if err := Start(path, 30); err != nil {
		t.Fatal(err)
	}
	event := Event{Time: "2026-01-01T00:00:00Z", Stage: "decode_furniture", Status: "partial", Code: "furniture_rejected", Counts: map[string]int{"rejected_lz4": 1}}
	if err := AppendEvent(path, event); err != nil {
		t.Fatal(err)
	}
	events, err := Read(path)
	if err != nil || len(events) != 2 || events[1].Time != event.Time || events[1].Counts["rejected_lz4"] != 1 {
		t.Fatalf("event lost: %+v, %v", events, err)
	}
}
