package main

import (
	"context"
	"fmt"
	"os"
	"sort"
)

func decodeDomains(ctx context.Context, report *udpReport, packets []timedUDPPacket, completed []completedMessage, catalogPath, characterCatalogPath, forkCatalogPath, resourceCatalogPath string, probeCharacters bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rawPayloads := udpPayloads(packets)
	streams := reassembleInventoryStreams(rawPayloads)
	report.diagnostic("decode_sources", "complete", "", map[string]int{"raw_payloads": len(rawPayloads), "inventory_streams": len(streams), "completed_messages": len(completed)})
	if err := decodeEquipmentDomain(ctx, report, rawPayloads, streams, completed, catalogPath); err != nil {
		return err
	}
	if err := decodeCharacterDomain(ctx, report, rawPayloads, streams, completed, catalogPath, characterCatalogPath, probeCharacters); err != nil {
		return err
	}
	if err := decodeWeaponDomain(ctx, report, rawPayloads, streams, forkCatalogPath); err != nil {
		return err
	}
	if err := decodeResourceDomain(ctx, report, rawPayloads, streams, resourceCatalogPath); err != nil {
		return err
	}
	if err := attachModulePlacements(ctx, report, rawPayloads, streams, completed, catalogPath, characterCatalogPath); err != nil {
		return err
	}
	decodeAccountObservations(report, completed)
	return ctx.Err()
}

func udpPayloads(packets []timedUDPPacket) [][]byte {
	payloads := make([][]byte, len(packets))
	for index, packet := range packets {
		payloads[index] = packet.data
	}
	return payloads
}

func decodeEquipmentDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, completed []completedMessage, catalogPath string) error {
	report.diagnostic("decode_equipment", "started", "", nil)
	diagnostics := &recordDiagnostics{}
	catalog, err := loadEquipmentCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "equipment catalog warning:", err)
		report.diagnostic("decode_equipment", "failed", "catalog_unavailable", nil)
		return nil
	}
	diagnostics.add("catalog_entries", len(catalog.Items))
	decoded := map[itemNetID]inventoryItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, item := range parseInventoryItems(payload, len(payload)*8, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[item.ID] = item
		}
	}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, item := range parseInventoryItems(stream.data, stream.bits, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[item.ID] = item
		}
	}
	for _, message := range completed {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, item := range parseInventoryItems(message.data, message.bits, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[item.ID] = item
		}
	}
	report.Items = make([]inventoryItem, 0, len(decoded))
	for _, item := range decoded {
		if err := ctx.Err(); err != nil {
			return err
		}
		report.Items = append(report.Items, item)
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].ID.Solt == report.Items[j].ID.Solt {
			return report.Items[i].ID.Serial < report.Items[j].ID.Serial
		}
		return report.Items[i].ID.Solt < report.Items[j].ID.Solt
	})
	report.DecodedItems = len(report.Items)
	report.domainDiagnostic("decode_equipment", diagnostics, len(report.Items))
	return ctx.Err()
}

func decodeCharacterDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, completed []completedMessage, equipmentCatalogPath, characterCatalogPath string, probeCharacters bool) error {
	report.diagnostic("decode_characters", "started", "", nil)
	diagnostics := &recordDiagnostics{}
	catalog, err := loadCharacterCatalog(characterCatalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "character catalog warning:", err)
		report.diagnostic("decode_characters", "failed", "catalog_unavailable", nil)
		return nil
	}
	diagnostics.add("catalog_entries", len(catalog.Characters))
	decoded := map[itemNetID]characterItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, character := range parseCharacters(payload, len(payload)*8, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[character.NetID] = character
		}
	}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, character := range parseCharacters(stream.data, stream.bits, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if previous := decoded[character.NetID]; character.RecordedAwakeningLevels == nil && previous.AwakenLevel == character.AwakenLevel {
				character.RecordedAwakeningLevels = previous.RecordedAwakeningLevels
			}
			if len(character.PassiveReferences) == 0 {
				character.PassiveReferences = decoded[character.NetID].PassiveReferences
			}
			decoded[character.NetID] = character
		}
	}
	if probeCharacters {
		probes := map[itemNetID]characterProbe{}
		for _, payload := range rawPayloads {
			if err := ctx.Err(); err != nil {
				return err
			}
			for _, probe := range parseCharacterProbes(payload, len(payload)*8, catalog) {
				if err := ctx.Err(); err != nil {
					return err
				}
				probes[probe.NetID] = probe
			}
		}
		for _, stream := range streams {
			if err := ctx.Err(); err != nil {
				return err
			}
			for _, probe := range parseCharacterProbes(stream.data, stream.bits, catalog) {
				if err := ctx.Err(); err != nil {
					return err
				}
				probes[probe.NetID] = probe
			}
		}
		for _, probe := range probes {
			if err := ctx.Err(); err != nil {
				return err
			}
			report.CharacterProbes = append(report.CharacterProbes, probe)
		}
		sort.Slice(report.CharacterProbes, func(i, j int) bool {
			return report.CharacterProbes[i].CharacterID < report.CharacterProbes[j].CharacterID
		})
	}
	for _, character := range decoded {
		if err := ctx.Err(); err != nil {
			return err
		}
		report.Characters = append(report.Characters, character)
	}
	sort.Slice(report.Characters, func(i, j int) bool { return report.Characters[i].CharacterID < report.Characters[j].CharacterID })
	attachBondLevels(report.Characters, completed)
	equipmentCatalog, _ := loadEquipmentCatalog(equipmentCatalogPath)
	attachObservedLoginState(report.Characters, report.ExportNames, equipmentCatalog)
	if err := attachCharacterPanels(ctx, report.Characters, completed); err != nil {
		return err
	}
	attachEquipmentPanelStats(report.Characters, report.Items)
	counts := map[string]int{"characters": len(report.Characters)}
	for _, character := range report.Characters {
		if character.PanelStats == nil {
			counts["missing_panels"]++
		}
		if character.BondLevel == nil {
			counts["missing_bonds"]++
		}
		if len(character.SkillLevels) == 0 {
			counts["missing_skills"]++
		}
	}
	status := "complete"
	if counts["missing_panels"]+counts["missing_bonds"]+counts["missing_skills"] > 0 {
		status = "partial"
	}
	report.diagnostic("decode_observations", status, "optional_observations", counts)
	report.domainDiagnostic("decode_characters", diagnostics, len(report.Characters))
	return ctx.Err()
}

func decodeWeaponDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, catalogPath string) error {
	report.diagnostic("decode_arcs", "started", "", nil)
	diagnostics := &recordDiagnostics{}
	catalog, err := loadForkCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Arc catalog warning:", err)
		report.diagnostic("decode_arcs", "failed", "catalog_unavailable", nil)
		return nil
	}
	diagnostics.add("catalog_entries", len(catalog.Forks))
	decoded := map[itemNetID]weaponItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, weapon := range parseWeapons(payload, len(payload)*8, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[weapon.ID] = weapon
		}
	}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, weapon := range parseWeapons(stream.data, stream.bits, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[weapon.ID] = weapon
		}
	}
	for _, weapon := range decoded {
		if err := ctx.Err(); err != nil {
			return err
		}
		report.Weapons = append(report.Weapons, weapon)
	}
	owners := map[itemNetID]uint32{}
	for _, character := range report.Characters {
		if err := ctx.Err(); err != nil {
			return err
		}
		if character.ForkNetID != nil {
			owners[*character.ForkNetID] = character.CharacterID
		}
	}
	for index := range report.Weapons {
		if err := ctx.Err(); err != nil {
			return err
		}
		if characterID, ok := owners[report.Weapons[index].ID]; ok {
			report.Weapons[index].EquippedCharacterID = &characterID
		}
	}
	sort.Slice(report.Weapons, func(i, j int) bool {
		if report.Weapons[i].ForkID == report.Weapons[j].ForkID {
			return report.Weapons[i].ID.Solt < report.Weapons[j].ID.Solt
		}
		return report.Weapons[i].ForkID < report.Weapons[j].ForkID
	})
	report.domainDiagnostic("decode_arcs", diagnostics, len(report.Weapons))
	return ctx.Err()
}

func decodeResourceDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, catalogPath string) error {
	report.diagnostic("decode_resources", "started", "", nil)
	diagnostics := &recordDiagnostics{}
	catalog, err := loadResourceCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "resource catalog warning:", err)
		report.diagnostic("decode_resources", "failed", "catalog_unavailable", nil)
		return nil
	}
	diagnostics.add("catalog_entries", len(catalog.Items))
	decoded := map[itemNetID]resourceItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, resource := range parseResources(payload, len(payload)*8, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[resource.ID] = resource
		}
	}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, resource := range parseResources(stream.data, stream.bits, catalog, diagnostics) {
			if err := ctx.Err(); err != nil {
				return err
			}
			decoded[resource.ID] = resource
		}
	}
	for _, resource := range decoded {
		if err := ctx.Err(); err != nil {
			return err
		}
		report.Resources = append(report.Resources, resource)
	}
	sort.Slice(report.Resources, func(i, j int) bool { return report.Resources[i].ItemID < report.Resources[j].ItemID })
	report.domainDiagnostic("decode_resources", diagnostics, len(report.Resources))
	return ctx.Err()
}

func attachModulePlacements(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, completed []completedMessage, equipmentCatalogPath, characterCatalogPath string) error {
	report.diagnostic("decode_placements", "started", "", nil)
	equipmentCatalog, equipmentErr := loadEquipmentCatalog(equipmentCatalogPath)
	characterCatalog, characterErr := loadCharacterCatalog(characterCatalogPath)
	if equipmentErr != nil || characterErr != nil {
		report.diagnostic("decode_placements", "partial", "catalog_unavailable", nil)
		report.diagnostic("decode_links", "partial", "catalog_unavailable", nil)
		return nil
	}
	known := map[itemNetID]inventoryItem{}
	index := map[itemNetID]int{}
	for itemIndex, item := range report.Items {
		known[item.ID] = item
		index[item.ID] = itemIndex
	}
	diagnostics := newPlacementDiagnostics()
	counts := map[string]int{"raw_payloads": len(rawPayloads), "inventory_streams": len(streams), "completed_messages": len(completed)}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		characters := parseCharacters(stream.data, stream.bits, characterCatalog, nil)
		positions := parseCompactPlacements(stream.data, stream.bits, known, equipmentCatalog, diagnostics)
		if len(characters) == 0 {
			counts["placement_streams_no_character"]++
		}
		if len(characters) > 1 {
			counts["placement_streams_multiple_characters"]++
		}
		if len(characters) != 1 {
			counts["placement_groups_in_skipped_streams"] += len(positions)
			continue
		}
		counts["placement_eligible_streams"]++
		counts["placement_groups_in_eligible_streams"] += len(positions)
		for id, position := range positions {
			if itemIndex, ok := index[id]; ok {
				placement := position
				report.Items[itemIndex].EquippedPlacement = &placement
			}
		}
	}
	// Other sources are diagnostic only. Do not infer a character association
	// or merge cell groups across unrelated packets or messages.
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		counts["placement_groups_in_raw_payloads"] += len(parseCompactPlacements(payload, len(payload)*8, known, equipmentCatalog, diagnostics))
	}
	for _, message := range completed {
		if err := ctx.Err(); err != nil {
			return err
		}
		counts["placement_groups_in_completed_messages"] += len(parseCompactPlacements(message.data, message.bits, known, equipmentCatalog, diagnostics))
	}
	for key, value := range diagnostics.records.counts {
		counts[key] = value
	}
	for _, key := range []string{"equipped_modules", "modules_with_placement", "modules_valid_outside_scope", "modules_rejected_groups", "modules_rejected_cells", "modules_no_recognized_candidate"} {
		counts[key] = 0
	}
	for _, item := range report.Items {
		if item.Kind != "module" || item.CharacterNetID == nil {
			continue
		}
		counts["equipped_modules"]++
		switch {
		case item.EquippedPlacement != nil:
			counts["modules_with_placement"]++
		case diagnostics.valid[item.ID]:
			counts["modules_valid_outside_scope"]++
		case diagnostics.groups[item.ID]:
			counts["modules_rejected_groups"]++
		case diagnostics.candidates[item.ID]:
			counts["modules_rejected_cells"]++
		default:
			counts["modules_no_recognized_candidate"]++
		}
	}
	status := "complete"
	if counts["equipped_modules"] != counts["modules_with_placement"] {
		status = "partial"
	}
	report.diagnostic("decode_placements", status, "placement_evidence", counts)
	counts = map[string]int{"equipped_records": 0, "unresolved_links": 0, "missing_placements": 0}
	owners := map[itemNetID]bool{}
	for _, character := range report.Characters {
		owners[character.NetID] = true
	}
	for _, item := range report.Items {
		if item.CharacterNetID == nil {
			continue
		}
		counts["equipped_records"]++
		if !owners[*item.CharacterNetID] {
			counts["unresolved_links"]++
		}
		if item.Kind == "module" && item.EquippedPlacement == nil {
			counts["missing_placements"]++
		}
	}
	weapons := map[itemNetID]bool{}
	for _, weapon := range report.Weapons {
		weapons[weapon.ID] = true
	}
	for _, character := range report.Characters {
		if character.ForkNetID != nil && !weapons[*character.ForkNetID] {
			counts["unresolved_links"]++
		}
	}
	status = "complete"
	if counts["unresolved_links"]+counts["missing_placements"] > 0 {
		status = "partial"
	}
	report.diagnostic("decode_links", status, "equipment_links", counts)
	return ctx.Err()
}
