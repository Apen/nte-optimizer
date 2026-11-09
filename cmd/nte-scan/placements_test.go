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

func TestPlacementImportAssociatesMultipleCharactersByInventoryOwner(t *testing.T) {
	equipmentPath, characterPath := placementAssociationCatalogs(t)
	firstOwner, secondOwner := itemNetID{Solt: 100, Serial: 200}, itemNetID{Solt: 101, Serial: 201}
	r := udpReport{Characters: []characterItem{{NetID: firstOwner, CharacterID: 1003}, {NetID: secondOwner, CharacterID: 1004}}}
	for i, owner := range []itemNetID{firstOwner, secondOwner} {
		ownerCopy := owner
		r.Items = append(r.Items, inventoryItem{ID: itemNetID{Solt: uint32(i + 1), Serial: 10}, ItemID: "cell_single", Kind: "module", CharacterNetID: &ownerCopy})
	}
	w := recordWriter{}
	putPlacementTestCharacter(&w, firstOwner, "1003")
	putPlacementTestCharacter(&w, secondOwner, "1004")
	// Equal coordinates are valid on different characters' grids.
	for _, item := range r.Items {
		putPlacementCell(&w, item.ItemID, item.ID, true, 2, 3)
	}
	streams := []invStream{{data: w.data, bits: w.bits}, {data: w.data, bits: w.bits}}
	if err := attachModulePlacements(context.Background(), &r, nil, streams, nil, equipmentPath, characterPath); err != nil {
		t.Fatal(err)
	}
	for _, item := range r.Items {
		if item.EquippedPlacement == nil || *item.EquippedPlacement != (equipmentGridCell{2, 3}) {
			t.Fatalf("multi-character stream lost placement: %+v", item)
		}
	}
	counts := placementAssociationCounts(t, r)
	if counts["modules_with_placement"] != 2 || counts["modules_conflicting_positions"] != 0 || counts["modules_valid_outside_scope"] != 0 {
		t.Fatalf("unexpected association counters: %v", counts)
	}
}

func TestPlacementImportRejectsOwnerMismatchConflictsAndOverlaps(t *testing.T) {
	for _, scenario := range []string{"wrong owner", "wrong roster identity", "unknown item", "conflicting snapshots", "conflicting snapshots reversed", "overlapping modules"} {
		t.Run(scenario, func(t *testing.T) {
			equipmentPath, characterPath := placementAssociationCatalogs(t)
			owner, otherOwner := itemNetID{Solt: 100, Serial: 200}, itemNetID{Solt: 101, Serial: 201}
			id := itemNetID{Solt: 1, Serial: 10}
			r := udpReport{Characters: []characterItem{{NetID: owner, CharacterID: 1003}, {NetID: otherOwner, CharacterID: 1004}}, Items: []inventoryItem{{ID: id, ItemID: "cell_single", Kind: "module", CharacterNetID: &owner}}}
			w := recordWriter{}
			switch scenario {
			case "wrong owner":
				putPlacementTestCharacter(&w, otherOwner, "1004")
			case "wrong roster identity":
				putPlacementTestCharacter(&w, owner, "1004")
			default:
				putPlacementTestCharacter(&w, owner, "1003")
			}
			seenID := id
			if scenario == "unknown item" {
				seenID = itemNetID{Solt: 50, Serial: 60}
			}
			putPlacementCell(&w, "cell_single", seenID, true, 2, 3)
			streams := []invStream{{data: w.data, bits: w.bits}}
			want := "placement_groups_owner_mismatch"
			if strings.HasPrefix(scenario, "conflicting snapshots") {
				second := recordWriter{}
				putPlacementTestCharacter(&second, owner, "1003")
				putPlacementCell(&second, "cell_single", id, true, 3, 3)
				streams = append(streams, invStream{data: second.data, bits: second.bits})
				if scenario == "conflicting snapshots reversed" {
					streams[0], streams[1] = streams[1], streams[0]
				}
				want = "modules_conflicting_positions"
			}
			if scenario == "overlapping modules" {
				secondID := itemNetID{Solt: 2, Serial: 10}
				r.Items = append(r.Items, inventoryItem{ID: secondID, ItemID: "cell_single", Kind: "module", CharacterNetID: &owner})
				putPlacementCell(&w, "cell_single", secondID, true, 2, 3)
				streams[0] = invStream{data: w.data, bits: w.bits}
				want = "modules_overlapping_positions"
			}
			if err := attachModulePlacements(context.Background(), &r, nil, streams, nil, equipmentPath, characterPath); err != nil {
				t.Fatal(err)
			}
			for _, item := range r.Items {
				if item.EquippedPlacement != nil {
					t.Fatalf("unsafe placement imported in %s: %+v", scenario, item)
				}
			}
			if counts := placementAssociationCounts(t, r); counts[want] == 0 {
				t.Fatalf("rejection cause not logged: %v", counts)
			}
		})
	}
}

func placementAssociationCatalogs(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	equipmentPath, characterPath := filepath.Join(dir, "equipment.json"), filepath.Join(dir, "characters.json")
	data, err := json.Marshal(placementTestCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(equipmentPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(characterPath, []byte(`{"characters":{"1003":{"codename":"SyntheticA"},"1004":{"codename":"SyntheticB"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	return equipmentPath, characterPath
}

func placementAssociationCounts(t *testing.T, r udpReport) map[string]int {
	t.Helper()
	for _, event := range r.Diagnostics {
		if event.Stage == "decode_placements" && event.Status != "started" {
			return event.Counts
		}
	}
	t.Fatal("missing placement diagnostic")
	return nil
}
