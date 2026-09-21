package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

type bitBuffer struct {
	data    []byte
	bits    int
	exports bool
}

func assembleUnrealMessages(report *udpReport, packets []timedUDPPacket, dumpDir string) ([]completedMessage, map[uint32]bool, string) {
	seenReadable := map[string]bool{}
	channels := map[uint32]bool{}
	pending := map[uint32]bitBuffer{}
	hints := map[string]bool{}
	assemblyHashes := map[[32]byte]bool{}
	exportNames := map[string]bool{}
	inventoryGUIDs := map[uint32]bool{}
	inventoryChildren := map[string]bool{}
	completed := []completedMessage{}
	assemblyIndex := 0
	if dumpDir != "" {
		if err := os.MkdirAll(dumpDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "avertissement dump UDP:", err)
			dumpDir = ""
		}
	}
	for _, packet := range packets {
		if bunches, ok := scannerunreal.ParsePacket(packet.data); ok {
			report.UnrealPackets++
			report.UnrealBunches += len(bunches)
			for _, bunch := range bunches {
				channels[bunch.Channel] = true
				collectBunchExports(report, bunch, inventoryGUIDs, inventoryChildren, exportNames)
				if !bunch.Partial {
					completed = append(completed, completedMessage{bunch.Data, bunch.DataBits, bunch.Exports, bunch.Channel})
					continue
				}
				report.PartialBunches++
				if bunch.Initial {
					pending[bunch.Channel] = bitBuffer{exports: bunch.Exports}
				}
				buffer, exists := pending[bunch.Channel]
				if exists || bunch.Initial {
					buffer.data, buffer.bits = scannerunreal.AppendBits(buffer.data, buffer.bits, bunch.Data, bunch.DataBits)
					buffer.exports = buffer.exports || bunch.Exports
					pending[bunch.Channel] = buffer
				}
				if bunch.Final {
					if buffer, exists := pending[bunch.Channel]; exists {
						completed = append(completed, completedMessage{buffer.data, buffer.bits, buffer.exports, bunch.Channel})
						assemblyIndex++
						recordAssembly(report, buffer, bunch.Channel, assemblyIndex, assemblyHashes, inventoryGUIDs, exportNames, hints, dumpDir)
						delete(pending, bunch.Channel)
					}
				}
			}
		}
		for _, readable := range asciiStrings(packet.data, 6) {
			if !seenReadable[readable] && len(report.Readable) < 20 {
				seenReadable[readable] = true
				report.Readable = append(report.Readable, readable)
			}
		}
	}
	for channel := range channels {
		report.Channels = append(report.Channels, channel)
	}
	for guid := range inventoryGUIDs {
		report.InventoryGUIDs = append(report.InventoryGUIDs, guid)
	}
	sort.Slice(report.Channels, func(i, j int) bool { return report.Channels[i] < report.Channels[j] })
	sort.Slice(report.InventoryGUIDs, func(i, j int) bool { return report.InventoryGUIDs[i] < report.InventoryGUIDs[j] })
	return completed, inventoryGUIDs, dumpDir
}

func collectBunchExports(report *udpReport, bunch scannerunreal.Bunch, inventoryGUIDs map[uint32]bool, inventoryChildren, exportNames map[string]bool) {
	if !bunch.Exports {
		return
	}
	report.ExportBunches++
	entries := scannerunreal.ParsePackageExports(bunch.Data, bunch.DataBits)
	if len(entries) > 0 {
		report.ExportParseSuccess++
	}
	for _, entry := range entries {
		name := entry.Path
		if name == "InventoryComponent" {
			inventoryGUIDs[entry.GUID] = true
		}
		if inventoryGUIDs[entry.Outer] && !inventoryChildren[name] {
			inventoryChildren[name] = true
			report.InventoryChildren = append(report.InventoryChildren, fmt.Sprintf("%d:%s", entry.GUID, name))
		}
		if !exportNames[name] && len(report.ExportNames) < 2000 {
			exportNames[name] = true
			report.ExportNames = append(report.ExportNames, name)
		}
	}
}

func recordAssembly(report *udpReport, buffer bitBuffer, channel uint32, index int, hashes map[[32]byte]bool, inventoryGUIDs map[uint32]bool, exportNames, hints map[string]bool, dumpDir string) {
	report.Assemblies++
	report.AssemblyBytes += (buffer.bits + 7) / 8
	hash := sha256.Sum256(buffer.data)
	if !hashes[hash] {
		hashes[hash] = true
		report.UniqueAssemblies++
		report.UniqueAssemblyBytes += (buffer.bits + 7) / 8
	}
	if dumpDir != "" {
		name := fmt.Sprintf("message-%04d-channel-%d-bits-%d.bin", index, channel, buffer.bits)
		if err := os.WriteFile(filepath.Join(dumpDir, name), buffer.data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "avertissement export:", err)
		}
	}
	if buffer.exports {
		for _, entry := range scannerunreal.ParsePackageExports(buffer.data, buffer.bits) {
			if entry.Path == "InventoryComponent" {
				inventoryGUIDs[entry.GUID] = true
			}
			if !exportNames[entry.Path] && len(report.ExportNames) < 2000 {
				exportNames[entry.Path] = true
				report.ExportNames = append(report.ExportNames, entry.Path)
			}
		}
	}
	for _, hint := range asciiStrings(buffer.data, 5) {
		lower := strings.ToLower(hint)
		if strings.Contains(lower, "invent") || strings.Contains(lower, "module") || strings.Contains(lower, "equip") || strings.Contains(lower, "drive") || strings.Contains(lower, "item") {
			if !hints[hint] && len(report.AssemblyHints) < 30 {
				hints[hint] = true
				report.AssemblyHints = append(report.AssemblyHints, hint)
			}
		}
	}
}

func inspectCompletedMessages(report *udpReport, completed []completedMessage, inventoryGUIDs map[uint32]bool, dumpDir string) {
	seenPayload := map[[32]byte]bool{}
	propertyNames := map[string]bool{}
	rpcOwners := map[uint32]bool{}
	hitChannels := map[uint32]bool{}
	atkValues := map[int]bool{}
	invIndex, rpcDumpIndex, hitIndex := 0, 0, 0
	for _, message := range completed {
		for _, value := range scannerunreal.ExtractAtkAdd(message.data) {
			atkValues[value] = true
		}
		for guid := range inventoryGUIDs {
			for _, properties := range scannerunreal.FindHottaContainers(message.data, message.bits, guid) {
				report.InventoryContainers++
				for _, property := range properties {
					if !propertyNames[property.Name] {
						propertyNames[property.Name] = true
						report.InventoryProperties = append(report.InventoryProperties, property.Name)
					}
				}
			}
			if scannerunreal.ContainsPackedGUID(message.data, message.bits, guid) {
				report.InventoryGUIDHits++
				hitChannels[message.channel] = true
				hitIndex++
				if dumpDir != "" {
					name := fmt.Sprintf("inventory-guid-hit-%03d-channel-%d-bits-%d.bin", hitIndex, message.channel, message.bits)
					_ = os.WriteFile(filepath.Join(dumpDir, name), message.data, 0o644)
				}
			}
		}
		start := 0
		if message.exports {
			_, start = scannerunreal.ParsePackageExportsAt(message.data, message.bits)
		}
		for _, block := range scannerunreal.ParseContentBlocks(message.data, message.bits, start) {
			if inventoryGUIDs[block.GUID] {
				hash := sha256.Sum256(block.Data)
				if seenPayload[hash] {
					continue
				}
				seenPayload[hash] = true
				report.InventoryPayloads++
				report.InventoryBytes += (block.Bits + 7) / 8
				invIndex++
				if dumpDir != "" {
					name := fmt.Sprintf("inventory-%03d-channel-%d-guid-%d-bits-%d.bin", invIndex, message.channel, block.GUID, block.Bits)
					_ = os.WriteFile(filepath.Join(dumpDir, name), block.Data, 0o644)
				}
			}
			if message.channel == 4 && block.Actor && !block.HasReplication {
				inspectRPCBlocks(report, block, message.channel, inventoryGUIDs, propertyNames, rpcOwners, dumpDir, &rpcDumpIndex)
			}
		}
	}
	for owner := range rpcOwners {
		report.RPC256Owners = append(report.RPC256Owners, owner)
	}
	for channel := range hitChannels {
		report.InventoryHitChannels = append(report.InventoryHitChannels, channel)
	}
	for value := range atkValues {
		report.AtkAddValues = append(report.AtkAddValues, value)
	}
	sort.Ints(report.AtkAddValues)
	sort.Slice(report.InventoryHitChannels, func(i, j int) bool { return report.InventoryHitChannels[i] < report.InventoryHitChannels[j] })
	sort.Slice(report.RPC256Owners, func(i, j int) bool { return report.RPC256Owners[i] < report.RPC256Owners[j] })
}

func inspectRPCBlocks(report *udpReport, block scannerunreal.ContentBlock, channel uint32, inventoryGUIDs map[uint32]bool, propertyNames map[string]bool, rpcOwners map[uint32]bool, dumpDir string, dumpIndex *int) {
	for _, rpc := range scannerunreal.ParseRPCs(block.Data, block.Bits, 425) {
		if rpc.Index != 256 {
			continue
		}
		report.RPC256++
		*dumpIndex++
		if dumpDir != "" {
			name := fmt.Sprintf("rpc256-%03d-channel-%d-bits-%d.bin", *dumpIndex, channel, rpc.Bits)
			_ = os.WriteFile(filepath.Join(dumpDir, name), rpc.Data, 0o644)
		}
		owner, names, ok := scannerunreal.ParseHottaContainer(rpc.Data, rpc.Bits)
		if ok {
			report.RPC256Decoded++
			rpcOwners[owner] = true
		}
		if ok && inventoryGUIDs[owner] {
			for _, name := range names {
				if !propertyNames[name] {
					propertyNames[name] = true
					report.InventoryProperties = append(report.InventoryProperties, name)
				}
			}
		}
	}
}
