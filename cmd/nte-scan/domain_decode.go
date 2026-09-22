package main

import (
	"fmt"
	"os"
	"sort"
)

func decodeDomains(report *udpReport, packets []timedUDPPacket, completed []completedMessage, catalogPath, characterCatalogPath, forkCatalogPath, resourceCatalogPath string, probeCharacters bool) {
	rawPayloads := udpPayloads(packets)
	streams := reassembleInventoryStreams(rawPayloads)
	decodeEquipmentDomain(report, rawPayloads, streams, completed, catalogPath)
	decodeCharacterDomain(report, rawPayloads, streams, completed, catalogPath, characterCatalogPath, probeCharacters)
	decodeWeaponDomain(report, rawPayloads, streams, forkCatalogPath)
	decodeResourceDomain(report, rawPayloads, streams, resourceCatalogPath)
	attachModulePlacements(report, streams, catalogPath, characterCatalogPath)
}

func udpPayloads(packets []timedUDPPacket) [][]byte {
	payloads := make([][]byte, len(packets))
	for index, packet := range packets {
		payloads[index] = packet.data
	}
	return payloads
}

func decodeEquipmentDomain(report *udpReport, rawPayloads [][]byte, streams []invStream, completed []completedMessage, catalogPath string) {
	catalog, err := loadEquipmentCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "equipment catalog warning:", err)
		return
	}
	decoded := map[itemNetID]inventoryItem{}
	for _, payload := range rawPayloads {
		for _, item := range parseInventoryItems(payload, len(payload)*8, catalog) {
			decoded[item.ID] = item
		}
	}
	for _, stream := range streams {
		for _, item := range parseInventoryItems(stream.data, stream.bits, catalog) {
			decoded[item.ID] = item
		}
	}
	for _, message := range completed {
		for _, item := range parseInventoryItems(message.data, message.bits, catalog) {
			decoded[item.ID] = item
		}
	}
	report.Items = make([]inventoryItem, 0, len(decoded))
	for _, item := range decoded {
		report.Items = append(report.Items, item)
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].ID.Solt == report.Items[j].ID.Solt {
			return report.Items[i].ID.Serial < report.Items[j].ID.Serial
		}
		return report.Items[i].ID.Solt < report.Items[j].ID.Solt
	})
	report.DecodedItems = len(report.Items)
}

func decodeCharacterDomain(report *udpReport, rawPayloads [][]byte, streams []invStream, completed []completedMessage, equipmentCatalogPath, characterCatalogPath string, probeCharacters bool) {
	catalog, err := loadCharacterCatalog(characterCatalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "character catalog warning:", err)
		return
	}
	decoded := map[itemNetID]characterItem{}
	for _, payload := range rawPayloads {
		for _, character := range parseCharacters(payload, len(payload)*8, catalog) {
			decoded[character.NetID] = character
		}
	}
	for _, stream := range streams {
		for _, character := range parseCharacters(stream.data, stream.bits, catalog) {
			decoded[character.NetID] = character
		}
	}
	if probeCharacters {
		probes := map[itemNetID]characterProbe{}
		for _, payload := range rawPayloads {
			for _, probe := range parseCharacterProbes(payload, len(payload)*8, catalog) {
				probes[probe.NetID] = probe
			}
		}
		for _, stream := range streams {
			for _, probe := range parseCharacterProbes(stream.data, stream.bits, catalog) {
				probes[probe.NetID] = probe
			}
		}
		for _, probe := range probes {
			report.CharacterProbes = append(report.CharacterProbes, probe)
		}
		sort.Slice(report.CharacterProbes, func(i, j int) bool {
			return report.CharacterProbes[i].CharacterID < report.CharacterProbes[j].CharacterID
		})
	}
	for _, character := range decoded {
		report.Characters = append(report.Characters, character)
	}
	sort.Slice(report.Characters, func(i, j int) bool { return report.Characters[i].CharacterID < report.Characters[j].CharacterID })
	equipmentCatalog, _ := loadEquipmentCatalog(equipmentCatalogPath)
	attachObservedActiveAwakenings(report.Characters, report.ExportNames, equipmentCatalog)
	for index := range report.Characters {
		for _, message := range completed {
			if stats, ok := parseCharacterPanelStats(message.data, message.bits, report.Characters[index].MaxHP); ok {
				report.Characters[index].PanelStats = &stats
				break
			}
		}
	}
	attachEquipmentPanelStats(report.Characters, report.Items)
}

func decodeWeaponDomain(report *udpReport, rawPayloads [][]byte, streams []invStream, catalogPath string) {
	catalog, err := loadForkCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Arc catalog warning:", err)
		return
	}
	decoded := map[itemNetID]weaponItem{}
	for _, payload := range rawPayloads {
		for _, weapon := range parseWeapons(payload, len(payload)*8, catalog) {
			decoded[weapon.ID] = weapon
		}
	}
	for _, stream := range streams {
		for _, weapon := range parseWeapons(stream.data, stream.bits, catalog) {
			decoded[weapon.ID] = weapon
		}
	}
	for _, weapon := range decoded {
		report.Weapons = append(report.Weapons, weapon)
	}
	owners := map[itemNetID]uint32{}
	for _, character := range report.Characters {
		if character.ForkNetID != nil {
			owners[*character.ForkNetID] = character.CharacterID
		}
	}
	for index := range report.Weapons {
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
}

func decodeResourceDomain(report *udpReport, rawPayloads [][]byte, streams []invStream, catalogPath string) {
	catalog, err := loadResourceCatalog(catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "resource catalog warning:", err)
		return
	}
	decoded := map[itemNetID]resourceItem{}
	for _, payload := range rawPayloads {
		for _, resource := range parseResources(payload, len(payload)*8, catalog) {
			decoded[resource.ID] = resource
		}
	}
	for _, stream := range streams {
		for _, resource := range parseResources(stream.data, stream.bits, catalog) {
			decoded[resource.ID] = resource
		}
	}
	for _, resource := range decoded {
		report.Resources = append(report.Resources, resource)
	}
	sort.Slice(report.Resources, func(i, j int) bool { return report.Resources[i].ItemID < report.Resources[j].ItemID })
}

func attachModulePlacements(report *udpReport, streams []invStream, equipmentCatalogPath, characterCatalogPath string) {
	equipmentCatalog, equipmentErr := loadEquipmentCatalog(equipmentCatalogPath)
	characterCatalog, characterErr := loadCharacterCatalog(characterCatalogPath)
	if equipmentErr != nil || characterErr != nil {
		return
	}
	known := map[itemNetID]inventoryItem{}
	index := map[itemNetID]int{}
	for itemIndex, item := range report.Items {
		known[item.ID] = item
		index[item.ID] = itemIndex
	}
	for _, stream := range streams {
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
}
