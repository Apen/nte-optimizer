package main

type placementCandidate struct {
	itemID  string
	cells   map[equipmentGridCell]bool
	first   *equipmentGridCell
	invalid bool
}

// Per-module evidence stays in memory. Only aggregate counts enter the log.
// A recognized candidate is not proof of a placement: inventory references can
// resemble its prefix. A complete validated cell group is stronger evidence.
type placementDiagnostics struct {
	records    recordDiagnostics
	candidates map[itemNetID]bool
	groups     map[itemNetID]bool
	valid      map[itemNetID]bool
}

func newPlacementDiagnostics() *placementDiagnostics {
	return &placementDiagnostics{candidates: map[itemNetID]bool{}, groups: map[itemNetID]bool{}, valid: map[itemNetID]bool{}}
}

// Repeated snapshots may repeat a placement, but conflicting coordinates must
// not be resolved by stream iteration order. Ownership comes from the decoded
// inventory and must also be present in the stream's character records.
type placementObservations struct {
	positions   map[itemNetID]equipmentGridCell
	conflicting map[itemNetID]bool
}

func newPlacementObservations() *placementObservations {
	return &placementObservations{positions: map[itemNetID]equipmentGridCell{}, conflicting: map[itemNetID]bool{}}
}

func (o *placementObservations) add(id itemNetID, position equipmentGridCell) {
	if previous, exists := o.positions[id]; exists && previous != position {
		o.conflicting[id] = true
	}
	o.positions[id] = position
}

func (o *placementObservations) nonOverlapping(known map[itemNetID]inventoryItem, catalog *equipmentCatalog) (map[itemNetID]equipmentGridCell, map[itemNetID]bool) {
	cells := map[itemNetID]map[equipmentGridCell]itemNetID{}
	overlaps := map[itemNetID]bool{}
	for id, position := range o.positions {
		item, exists := known[id]
		if !exists || item.Kind != "module" || item.CharacterNetID == nil || o.conflicting[id] {
			continue
		}
		owner := *item.CharacterNetID
		if cells[owner] == nil {
			cells[owner] = map[equipmentGridCell]itemNetID{}
		}
		for _, relative := range catalog.Shapes[catalog.Items[item.ItemID].Geometry].Cells {
			cell := equipmentGridCell{Row: position.Row + relative.Row, Column: position.Column + relative.Column}
			if other, exists := cells[owner][cell]; exists && other != id {
				overlaps[id], overlaps[other] = true, true
			}
			cells[owner][cell] = id
		}
	}
	positions := map[itemNetID]equipmentGridCell{}
	for id, position := range o.positions {
		item, exists := known[id]
		if exists && item.Kind == "module" && item.CharacterNetID != nil && !o.conflicting[id] && !overlaps[id] {
			positions[id] = position
		}
	}
	return positions, overlaps
}

type placementCellRecord struct {
	id     itemNetID
	itemID string
	first  bool
	cell   equipmentGridCell
	end    int
}

func parsePlacementCellAt(data []byte, bitLen, off int, known map[itemNetID]inventoryItem, c *equipmentCatalog, diagnostics *placementDiagnostics) (_ placementCellRecord, accepted bool) {
	r := invBits{data: data, bitLen: bitLen, cursor: off}
	one, ok := r.i32()
	if !ok || one != 1 {
		return placementCellRecord{}, false
	}
	itemID, ok := r.dynamicName()
	if !ok {
		return placementCellRecord{}, false
	}
	def, ok := c.Items[itemID]
	if !ok || def.Kind != "module" {
		return placementCellRecord{}, false
	}
	phase := "placement_id"
	defer func() {
		if diagnostics != nil {
			diagnostics.records.record(phase, accepted)
		}
	}()
	id, ok := r.netID()
	if !ok || !validNetID(id) {
		return placementCellRecord{}, false
	}
	item, exists := known[id]
	if diagnostics != nil && exists && item.ItemID == itemID {
		diagnostics.candidates[id] = true
	}
	phase = "placement_cell_fields"
	first, ok := r.boolean()
	if !ok {
		return placementCellRecord{}, false
	}
	row, ok := r.i32()
	if !ok {
		return placementCellRecord{}, false
	}
	col, ok := r.i32()
	if !ok {
		return placementCellRecord{}, false
	}
	nf, ok := r.i32()
	if !ok {
		return placementCellRecord{}, false
	}
	phase = "placement_coordinates"
	if row < 0 || row >= 7 || col < 0 || col >= 7 {
		return placementCellRecord{}, false
	}
	phase = "placement_flags"
	if nf != 0 {
		return placementCellRecord{}, false
	}
	phase = "placement_item_mismatch"
	if exists && item.ItemID != itemID {
		return placementCellRecord{}, false
	}
	return placementCellRecord{id: id, itemID: itemID, first: first, cell: equipmentGridCell{row, col}, end: r.cursor}, true
}

func parseCompactPlacements(data []byte, bitLen int, known map[itemNetID]inventoryItem, c *equipmentCatalog, diagnostics *placementDiagnostics) map[itemNetID]equipmentGridCell {
	groups := map[itemNetID]*placementCandidate{}
	if bitLen < 0 || bitLen > len(data)*8 {
		return nil
	}
	add := func(key string) {
		if diagnostics != nil {
			diagnostics.records.add(key, 1)
		}
	}
	for off := 0; off < bitLen; {
		record, ok := parsePlacementCellAt(data, bitLen, off, known, c, diagnostics)
		if !ok {
			off++
			continue
		}
		g := groups[record.id]
		if g == nil {
			g = &placementCandidate{itemID: record.itemID, cells: map[equipmentGridCell]bool{}}
			groups[record.id] = g
		}
		if g.itemID != record.itemID || g.cells[record.cell] {
			g.invalid = true
			add("placement_duplicate_or_mixed_cells")
		}
		g.cells[record.cell] = true
		if record.first {
			if g.first != nil {
				g.invalid = true
				add("placement_multiple_origins")
			}
			x := record.cell
			g.first = &x
		}
		off = record.end
	}
	out := map[itemNetID]equipmentGridCell{}
	for id, g := range groups {
		if diagnostics != nil && diagnostics.candidates[id] {
			diagnostics.groups[id] = true
		}
		if g.invalid {
			add("placement_invalid_groups")
			continue
		}
		if g.first == nil {
			add("placement_missing_origin")
			continue
		}
		def := c.Items[g.itemID]
		shape, ok := c.Shapes[def.Geometry]
		if !ok || def.Grid == nil {
			add("placement_missing_geometry")
			continue
		}
		if len(g.cells) != int(*def.Grid) || len(shape.Cells) != len(g.cells) {
			add("placement_incomplete_cells")
			continue
		}
		valid := true
		for _, rel := range shape.Cells {
			if !g.cells[equipmentGridCell{g.first.Row + rel.Row, g.first.Column + rel.Column}] {
				valid = false
				break
			}
		}
		if !valid {
			add("placement_shape_mismatch")
			continue
		}
		out[id] = *g.first
		add("placement_valid_groups")
		if diagnostics != nil && diagnostics.candidates[id] {
			diagnostics.valid[id] = true
		}
	}
	return out
}
