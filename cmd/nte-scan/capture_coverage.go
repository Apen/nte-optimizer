package main

import (
	"math"
	"time"

	"nte-optimizer/internal/scanlog"
	scannerpcap "nte-optimizer/internal/scanner/pcap"
)

func captureCoverage(summary scannerpcap.SelectionSummary, guided bool, seconds int) scanlog.Event {
	event := scanlog.Event{Time: time.Now().UTC().Format(time.RFC3339), Stage: "capture_coverage", Status: "complete", Code: "capture_timestamp_span", Counts: map[string]int{}}
	spanMS := (summary.LastTimestamp - summary.FirstTimestamp) * 1000
	// Keep integer counters representable in both Go and the frontend even for
	// malformed timestamps. Actual guided captures are at most a few minutes.
	if math.IsNaN(spanMS) || math.IsInf(spanMS, 0) || spanMS < 0 || spanMS > 1<<50 {
		event.Status, event.Code = "partial", "capture_timestamp_invalid"
		return event
	}
	event.Counts["capture_span_ms"] = int(math.Round(spanMS))
	if summary.SourcePackets == 0 {
		event.Status, event.Code = "partial", "capture_no_packets"
	} else if guided {
		expectedMS := max(10, seconds) * 1000
		event.Counts["expected_capture_ms"] = expectedMS
		// A short packet window can mean quiet traffic or overwritten events.
		// It is evidence to investigate, not proof of capture completeness/loss.
		if spanMS+2000 < float64(expectedMS) {
			event.Status, event.Code = "partial", "capture_window_short"
		}
	}
	return event
}
