package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nte-optimizer/internal/scanlog"
	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

func furnitureMessage(payload []byte) completedMessage {
	return accountPropertyMessage("FurnitureActivateStates", payload)
}

func accountPropertyMessage(name string, payload []byte) completedMessage {
	b := new(bytes.Buffer)
	observationString(b, name)
	binary.Write(b, binary.LittleEndian, uint32(0))
	binary.Write(b, binary.LittleEndian, uint32(len(payload)))
	b.Write(payload)
	return completedMessage{data: b.Bytes(), bits: b.Len() * 8, channel: 4}
}

func TestDecoderPersistsStagesImmediatelyAndReturnsLogFailures(t *testing.T) {
	var events []scanlog.Event
	_, err := analyzeUDP(context.Background(), nil, 0, "", "", "", "", "", false, func(event scanlog.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil || len(events) != 2 || events[0].Status != "started" || events[1].Code != "no_udp_flow" {
		t.Fatalf("stages not persisted: %+v, %v", events, err)
	}
	writeFailure := errors.New("synthetic log write failure")
	_, err = analyzeUDP(context.Background(), nil, 0, "", "", "", "", "", false, func(scanlog.Event) error { return writeFailure })
	if !errors.Is(err, writeFailure) {
		t.Fatalf("log write failure lost: %v", err)
	}
}

func TestFurnitureDiagnosticsDistinguishMissingEnvelopeCompressionAndStructure(t *testing.T) {
	valid := furnitureMessage(furniturePayload(3))
	truncated := valid
	truncated.bits -= 8
	oversized := furnitureMessage(make([]byte, 8193))
	wrongChannel := valid
	wrongChannel.channel = 5
	for _, tc := range []struct {
		name          string
		messages      []completedMessage
		code, counter string
		retained      int
	}{
		{"channel absent", []completedMessage{wrongChannel}, "channel4_absent", "", 0},
		{"property absent", []completedMessage{{data: []byte("private-user@example.test"), bits: 200, channel: 4}}, "furniture_property_absent", "", 0},
		{"envelope truncated", []completedMessage{truncated}, "furniture_rejected", "rejected_property_truncated", 0},
		{"envelope size", []completedMessage{oversized}, "furniture_rejected", "rejected_property_size", 0},
		{"lz4 rejected", []completedMessage{furnitureMessage([]byte{0xff})}, "furniture_rejected", "rejected_lz4", 0},
		{"level rejected", []completedMessage{furnitureMessage(furniturePayload(101))}, "furniture_rejected", "rejected_furniture_fields", 0},
		{"valid then invalid", []completedMessage{valid, furnitureMessage(furniturePayload(0))}, "furniture_decoded", "rejected_furniture_fields", 1},
		{"valid", []completedMessage{valid}, "furniture_decoded", "accepted_sightings", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := udpReport{}
			decodeAccountObservations(&r, tc.messages)
			index := slices.IndexFunc(r.Diagnostics, func(event scanlog.Event) bool {
				return event.Stage == "decode_furniture" && event.Status != "started"
			})
			if index < 0 {
				t.Fatal("missing furniture result")
			}
			event := r.Diagnostics[index]
			if event.Code != tc.code || len(r.FurnitureLevels) != tc.retained || (tc.counter != "" && event.Counts[tc.counter] != 1) {
				t.Fatalf("unexpected diagnostic: %+v, retained=%d", event, len(r.FurnitureLevels))
			}
			encoded, err := json.Marshal(r.Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"private-user", "SF_test", "FurnitureActivateStates"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("diagnostic contains payload data: %s", encoded)
				}
			}
		})
	}
}

func TestDecodePukaTrainingRank(t *testing.T) {
	zero := accountPropertyMessage("PukalandTrainingLevel", []byte{0, 0, 0, 0})
	ranked := accountPropertyMessage("PukalandTrainingLevel", []byte{5, 0, 0, 0})
	ranked.data, ranked.bits = scannerunreal.AppendBits([]byte{0}, 3, ranked.data, ranked.bits)
	invalid := accountPropertyMessage("PukalandTrainingLevel", []byte{1, 0})
	program := accountPropertyMessage("PukalandLevel", []byte{9, 0, 0, 0})
	wrongChannel := zero
	wrongChannel.channel = 5

	absent := udpReport{}
	decodeAccountObservations(&absent, []completedMessage{wrongChannel, program, invalid})
	if absent.PukaTrainingRank != nil {
		t.Fatalf("unobserved training rank = %d", *absent.PukaTrainingRank)
	}
	for _, tc := range []struct {
		message completedMessage
		rank    uint32
	}{{zero, 0}, {ranked, 5}} {
		var r udpReport
		decodeAccountObservations(&r, []completedMessage{tc.message, program, invalid})
		if r.PukaTrainingRank == nil || *r.PukaTrainingRank != tc.rank {
			t.Fatalf("expected observed training rank %d", tc.rank)
		}
	}
}

func TestRecordDiagnosticsIgnoreUnknownNamesAndLocateHeaderRejections(t *testing.T) {
	for _, tc := range []struct {
		name, known string
		decode      func([]byte, int, *recordDiagnostics)
	}{
		{"equipment", "cell_test", func(b []byte, n int, d *recordDiagnostics) {
			parseInventoryItems(b, n, &equipmentCatalog{Items: map[string]equipmentDefinition{"cell_test": {}}}, d)
		}},
		{"characters", "1003", func(b []byte, n int, d *recordDiagnostics) {
			parseCharacters(b, n, &characterCatalog{Characters: map[string]characterDefinition{"1003": {}}}, d)
		}},
		{"arcs", "fork_test", func(b []byte, n int, d *recordDiagnostics) {
			parseWeapons(b, n, &forkCatalog{Forks: map[string]forkDefinition{"fork_test": {}}}, d)
		}},
		{"resources", "resource_test", func(b []byte, n int, d *recordDiagnostics) {
			parseResources(b, n, &resourceCatalog{Items: map[string]struct{}{"resource_test": {}}}, d)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unknown := recordWriter{}
			unknown.putString("private-unknown-name")
			d := &recordDiagnostics{}
			tc.decode(unknown.data, unknown.bits, d)
			if len(d.counts) != 0 {
				t.Fatalf("unknown reference counted as a malformed record: %v", d.counts)
			}
			known := recordWriter{}
			known.putString(tc.known)
			tc.decode(known.data, known.bits, d)
			if d.counts["candidates"] != 1 || d.counts["rejected_record_header"] != 1 || d.counts["accepted_sightings"] != 0 {
				t.Fatalf("unexpected rejection counters: %v", d.counts)
			}
		})
	}
}

func TestDomainDiagnosticsSeparateSightingsFromUniqueResources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(`{"items":{"resource_test":{}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	w := resourceFixtureWriter{}
	w.add("resource_test", itemNetID{Solt: 1, Serial: 2}, 3)
	r := udpReport{}
	if err := decodeResourceDomain(context.Background(), &r, [][]byte{w.data}, []invStream{{data: w.data, bits: w.bitLen}}, path); err != nil {
		t.Fatal(err)
	}
	event := r.Diagnostics[len(r.Diagnostics)-1]
	if len(r.Resources) != 1 || event.Counts["unique_records"] != 1 || event.Counts["accepted_sightings"] != 2 {
		t.Fatalf("sightings confused with inventory size: %+v", event)
	}
	r = udpReport{}
	if err := decodeResourceDomain(context.Background(), &r, nil, nil, path+"-private-missing"); err != nil {
		t.Fatal(err)
	}
	event = r.Diagnostics[len(r.Diagnostics)-1]
	if event.Status != "failed" || event.Code != "catalog_unavailable" {
		t.Fatalf("missing catalog not reported: %+v", event)
	}
	encoded, _ := json.Marshal(r.Diagnostics)
	if strings.Contains(string(encoded), "private-missing") {
		t.Fatal("catalog path leaked")
	}
}
