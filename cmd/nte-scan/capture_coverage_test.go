package main

import (
	"math"
	"testing"

	scannerpcap "nte-optimizer/internal/scanner/pcap"
)

func TestCaptureCoverageDistinguishesObservedWindowFromRequestedDuration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		packets int
		span    float64
		guided  bool
		code    string
	}{
		{"tail only", 300000, 6, true, "capture_window_short"},
		{"full window", 100, 29, true, "capture_timestamp_span"},
		{"offline", 100, 6, false, "capture_timestamp_span"},
		{"empty", 0, 0, true, "capture_no_packets"},
		{"invalid", 100, math.Inf(1), true, "capture_timestamp_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := captureCoverage(scannerpcap.SelectionSummary{SourcePackets: tc.packets, FirstTimestamp: 100, LastTimestamp: 100 + tc.span}, tc.guided, 30)
			if event.Code != tc.code {
				t.Fatalf("unexpected window diagnosis: %+v", event)
			}
			if tc.name == "tail only" && (event.Counts["capture_span_ms"] != 6000 || event.Counts["expected_capture_ms"] != 30000) {
				t.Fatalf("missing coverage counters: %+v", event)
			}
		})
	}
}
