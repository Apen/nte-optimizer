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
	if err := attachModulePlacements(ctx, report, streams, catalogPath, characterCatalogPath); err != nil {
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
	catalog, err := loadEquipmentCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "equipment catalog warning:", err)
		return nil
	}
	decoded := map[itemNetID]inventoryItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, item := range parseInventoryItems(payload, len(payload)*8, catalog) {
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
		for _, item := range parseInventoryItems(stream.data, stream.bits, catalog) {
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
		for _, item := range parseInventoryItems(message.data, message.bits, catalog) {
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
	return ctx.Err()
}

func decodeCharacterDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, completed []completedMessage, equipmentCatalogPath, characterCatalogPath string, probeCharacters bool) error {
	catalog, err := loadCharacterCatalog(characterCatalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "character catalog warning:", err)
		return nil
	}
	decoded := map[itemNetID]characterItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, character := range parseCharacters(payload, len(payload)*8, catalog) {
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
		for _, character := range parseCharacters(stream.data, stream.bits, catalog) {
			if err := ctx.Err(); err != nil {
				return err
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
	attachObservedActiveAwakenings(report.Characters, report.ExportNames, equipmentCatalog)
	if err := attachCharacterPanels(ctx, report.Characters, completed); err != nil {
		return err
	}
	attachEquipmentPanelStats(report.Characters, report.Items)
	return ctx.Err()
}

func decodeWeaponDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, catalogPath string) error {
	catalog, err := loadForkCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Arc catalog warning:", err)
		return nil
	}
	decoded := map[itemNetID]weaponItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, weapon := range parseWeapons(payload, len(payload)*8, catalog) {
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
		for _, weapon := range parseWeapons(stream.data, stream.bits, catalog) {
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
	return ctx.Err()
}

func decodeResourceDomain(ctx context.Context, report *udpReport, rawPayloads [][]byte, streams []invStream, catalogPath string) error {
	catalog, err := loadResourceCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "resource catalog warning:", err)
		return nil
	}
	decoded := map[itemNetID]resourceItem{}
	for _, payload := range rawPayloads {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, resource := range parseResources(payload, len(payload)*8, catalog) {
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
		for _, resource := range parseResources(stream.data, stream.bits, catalog) {
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
	return ctx.Err()
}

func attachModulePlacements(ctx context.Context, report *udpReport, streams []invStream, equipmentCatalogPath, characterCatalogPath string) error {
	equipmentCatalog, equipmentErr := loadEquipmentCatalog(equipmentCatalogPath)
	characterCatalog, characterErr := loadCharacterCatalog(characterCatalogPath)
	if equipmentErr != nil || characterErr != nil {
		return nil
	}
	known := map[itemNetID]inventoryItem{}
	index := map[itemNetID]int{}
	for itemIndex, item := range report.Items {
		known[item.ID] = item
		index[item.ID] = itemIndex
	}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		characters := parseCharacters(stream.data, stream.bits, characterCatalog)
		if len(characters) != 1 {
			continue
		}
		for id, position := range parseCompactPlacements(stream.data, stream.bits, known, equipmentCatalog) {
			if itemIndex, ok := index[id]; ok {
				placement := position
				report.Items[itemIndex].EquippedPlacement = &placement
			}
		}
	}
	return ctx.Err()
}
