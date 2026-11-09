package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func putPlacementCell(w *recordWriter, name string, id itemNetID, first bool, row, column int32) {
	w.put(1, 32)
	w.putString(name)
	w.putNetID(id)
	if first {
		w.put(1, 1)
	} else {
		w.put(0, 1)
	}
	w.put(uint64(uint32(row)), 32)
	w.put(uint64(uint32(column)), 32)
	w.put(0, 32)
}

func placementTestCatalog() *equipmentCatalog {
	one, two := uint32(1), uint32(2)
	return &equipmentCatalog{
		Items: map[string]equipmentDefinition{
			"cell_single": {Kind: "module", Grid: &one, Geometry: "single"},
			"cell_pair":   {Kind: "module", Grid: &two, Geometry: "pair"},
		},
		Shapes: map[string]equipmentShape{
			"single": {Cells: []equipmentGridCell{{0, 0}}},
			"pair":   {Cells: []equipmentGridCell{{0, 0}, {0, 1}}},
		},
	}
}

func TestPlacementDiagnosticsValidateGroupsAtAllBitAlignments(t *testing.T) {
	id := itemNetID{Solt: 1, Serial: 2}
	known := map[itemNetID]inventoryItem{id: {ID: id, ItemID: "cell_pair", Kind: "module"}}
	for shift := 0; shift < 8; shift++ {
		w := recordWriter{}
		w.put(0, shift)
		putPlacementCell(&w, "cell_pair", id, true, 2, 3)
		putPlacementCell(&w, "cell_pair", id, false, 2, 4)
		d := newPlacementDiagnostics()
		got := parseCompactPlacements(w.data, w.bits, known, placementTestCatalog(), d)
		if !reflect.DeepEqual(got, map[itemNetID]equipmentGridCell{id: {2, 3}}) || !d.valid[id] || d.records.counts["accepted_sightings"] != 2 {
			t.Fatalf("alignment %d: positions=%v counters=%v", shift, got, d.records.counts)
		}
		plain := parseCompactPlacements(w.data, w.bits, known, placementTestCatalog(), nil)
		if !reflect.DeepEqual(got, plain) {
			t.Fatal("diagnostics changed placement selection")
		}
	}
}

func TestPlacementDiagnosticsExplainRejectedGroupsAndCells(t *testing.T) {
	id := itemNetID{Solt: 1, Serial: 2}
	known := map[itemNetID]inventoryItem{id: {ID: id, ItemID: "cell_pair", Kind: "module"}}
	for _, tc := range []struct {
		name, counter string
		write         func(*recordWriter)
	}{
		{"incomplete", "placement_incomplete_cells", func(w *recordWriter) { putPlacementCell(w, "cell_pair", id, true, 0, 0) }},
		{"missing origin", "placement_missing_origin", func(w *recordWriter) { putPlacementCell(w, "cell_pair", id, false, 0, 0) }},
		{"duplicate cell", "placement_invalid_groups", func(w *recordWriter) {
			putPlacementCell(w, "cell_pair", id, true, 0, 0)
			putPlacementCell(w, "cell_pair", id, false, 0, 0)
		}},
		{"multiple origins", "placement_multiple_origins", func(w *recordWriter) {
			putPlacementCell(w, "cell_pair", id, true, 0, 0)
			putPlacementCell(w, "cell_pair", id, true, 0, 1)
		}},
		{"wrong shape", "placement_shape_mismatch", func(w *recordWriter) {
			putPlacementCell(w, "cell_pair", id, true, 0, 0)
			putPlacementCell(w, "cell_pair", id, false, 1, 0)
		}},
		{"outside grid", "rejected_placement_coordinates", func(w *recordWriter) { putPlacementCell(w, "cell_pair", id, true, 7, 0) }},
		{"truncated cell", "rejected_placement_cell_fields", func(w *recordWriter) { putPlacementCell(w, "cell_pair", id, true, 0, 0); w.bits-- }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := recordWriter{}
			tc.write(&w)
			d := newPlacementDiagnostics()
			if got := parseCompactPlacements(w.data, w.bits, known, placementTestCatalog(), d); len(got) != 0 {
				t.Fatalf("invalid group accepted: %v", got)
			}
			if !d.candidates[id] || d.valid[id] || d.records.counts[tc.counter] != 1 {
				t.Fatalf("unexpected evidence: %+v", d)
			}
		})
	}
}

func putPlacementTestCharacter(w *recordWriter, id itemNetID, characterID string) {
	w.putString(characterID)
	w.putNetID(id)
	w.put(1, 64)
	w.put(0, 32)
	w.put(500, 64)
	w.put(0, 64)
	w.put(1, 16)
	w.put(70, 32)
	w.put(5, 32)
	w.putNetID(itemNetID{})
	w.put(0, 32)
	for i := 0; i < 7; i++ {
		w.putFloat(float32(i))
	}
	w.put(0, 1)
	w.put(0, 32)
	w.put(0, 16)
}

func TestPlacementEvidenceSeparatesMissingRejectedAndExcludedSources(t *testing.T) {
	dir := t.TempDir()
	equipmentPath, characterPath := filepath.Join(dir, "equipment.json"), filepath.Join(dir, "characters.json")
	encoded, err := json.Marshal(placementTestCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(equipmentPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(characterPath, []byte(`{"characters":{"1003":{"codename":"Synthetic"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	owner := itemNetID{Solt: 100, Serial: 200}
	r := udpReport{Characters: []characterItem{{NetID: owner, CharacterID: 1003}}}
	for i := uint32(1); i <= 5; i++ {
		name := "cell_single"
		if i == 2 {
			name = "cell_pair"
		}
		r.Items = append(r.Items, inventoryItem{ID: itemNetID{Solt: i, Serial: i}, ItemID: name, Kind: "module", CharacterNetID: &owner})
	}
	excluded, incomplete, rejected, eligible := recordWriter{}, recordWriter{}, recordWriter{}, recordWriter{}
	putPlacementCell(&excluded, "cell_single", r.Items[0].ID, true, 0, 0)
	putPlacementCell(&incomplete, "cell_pair", r.Items[1].ID, true, 0, 0)
	putPlacementCell(&rejected, "cell_single", r.Items[2].ID, true, 7, 0)
	putPlacementTestCharacter(&eligible, owner, "1003")
	putPlacementCell(&eligible, "cell_single", r.Items[4].ID, true, 1, 1)
	streams := []invStream{{data: excluded.data, bits: excluded.bits}, {data: incomplete.data, bits: incomplete.bits}, {data: eligible.data, bits: eligible.bits}}
	if err := attachModulePlacements(context.Background(), &r, [][]byte{rejected.data}, streams, []completedMessage{{data: excluded.data, bits: excluded.bits}}, equipmentPath, characterPath); err != nil {
		t.Fatal(err)
	}
	for _, event := range r.Diagnostics {
		if event.Stage != "decode_placements" || event.Status == "started" {
			continue
		}
		for key, want := range map[string]int{"equipped_modules": 5, "modules_with_placement": 1, "modules_valid_outside_scope": 1, "modules_rejected_groups": 1, "modules_rejected_cells": 1, "modules_no_recognized_candidate": 1} {
			if event.Counts[key] != want {
				t.Errorf("%s=%d, want %d; all counters: %v", key, event.Counts[key], want, event.Counts)
			}
		}
		if r.Items[0].EquippedPlacement != nil || r.Items[4].EquippedPlacement == nil {
			t.Fatal("diagnostic-only sources changed import behavior")
		}
		log, err := json.Marshal(r.Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(log), "cell_single") || strings.Contains(string(log), "Synthetic") {
			t.Fatal("placement diagnostics leaked record values")
		}
		return
	}
	t.Fatal("missing placement evidence diagnostic")
}
