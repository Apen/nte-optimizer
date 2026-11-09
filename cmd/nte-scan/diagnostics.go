package main

import (
	"time"

	"nte-optimizer/internal/scanlog"
)

// Candidates are counted only after recognizing a catalog name. A rejected
// candidate can be a reference or a partial record, not necessarily corrupt data.
// Sightings include repeated scans of raw packets and reconstructed streams.
type recordDiagnostics struct {
	counts map[string]int
}

func (d *recordDiagnostics) add(key string, value int) {
	if d == nil {
		return
	}
	if d.counts == nil {
		d.counts = map[string]int{}
	}
	d.counts[key] += value
}

func (d *recordDiagnostics) maximum(key string, value int) {
	if d != nil && value > d.counts[key] {
		d.add(key, value-d.counts[key])
	}
}

func (d *recordDiagnostics) record(phase string, accepted bool) {
	if d == nil {
		return
	}
	if d.counts == nil {
		d.counts = map[string]int{}
	}
	d.counts["candidates"]++
	if accepted {
		d.counts["accepted_sightings"]++
	} else {
		d.counts["rejected_"+phase]++
	}
}

func (r *udpReport) diagnostic(stage, status, code string, counts map[string]int) {
	event := scanlog.Event{
		Time:  time.Now().UTC().Format(time.RFC3339),
		Stage: stage, Status: status, Code: code, Counts: counts,
	}
	r.Diagnostics = append(r.Diagnostics, event)
	// Persist each stage immediately so an interrupted helper leaves a useful
	// last-known phase instead of losing all intermediate decoder diagnostics.
	if r.writeDiagnostic != nil && r.diagnosticErr == nil {
		r.diagnosticErr = r.writeDiagnostic(event)
	}
}

func (r *udpReport) domainDiagnostic(stage string, d *recordDiagnostics, unique int) {
	if d.counts == nil {
		d.counts = map[string]int{}
	}
	d.counts["unique_records"] = unique
	status, code := "complete", "records_decoded"
	if unique == 0 {
		status, code = "partial", "no_records_decoded"
	}
	r.diagnostic(stage, status, code, d.counts)
}
