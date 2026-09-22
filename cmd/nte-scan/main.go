package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nte-optimizer/internal/buildinfo"
	"nte-optimizer/internal/datafiles"
	scannerpcap "nte-optimizer/internal/scanner/pcap"
	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

type packet struct {
	ts   float64
	data []byte
}
type blockReport struct {
	Frame           int      `json:"frame"`
	CompressedBytes int      `json:"compressedBytes"`
	DecodedBytes    int      `json:"decodedBytes"`
	Markers         []string `json:"markers,omitempty"`
	Records         []string `json:"records,omitempty"`
}
type report struct {
	Input                string        `json:"input"`
	RawPackets           int           `json:"rawPackets"`
	UniquePackets        int           `json:"uniquePackets"`
	DuplicateAppearances int           `json:"duplicateAppearances"`
	TCPFlow              string        `json:"tcpFlow"`
	TCPSegments          int           `json:"tcpSegments"`
	TCPReassembledBytes  int           `json:"tcpReassembledBytes"`
	Frames               int           `json:"frames"`
	LZ4BlocksDecoded     int           `json:"lz4BlocksDecoded"`
	LargestDecodedBlock  int           `json:"largestDecodedBlock"`
	Blocks               []blockReport `json:"blocks"`
	UDP                  udpReport     `json:"udp"`
}

type udpReport struct {
	Flow                 string           `json:"flow"`
	Packets              int              `json:"packets"`
	PayloadBytes         int              `json:"payloadBytes"`
	LargestPacket        int              `json:"largestPacket"`
	PeakBurstBytes       int              `json:"peakBurstBytes100ms"`
	PeakAtSeconds        float64          `json:"peakAtSeconds"`
	Readable             []string         `json:"readableStrings,omitempty"`
	UnrealPackets        int              `json:"unrealPackets"`
	UnrealBunches        int              `json:"unrealBunches"`
	PartialBunches       int              `json:"partialBunches"`
	Channels             []uint32         `json:"channels,omitempty"`
	Assemblies           int              `json:"reassembledMessages"`
	AssemblyBytes        int              `json:"reassembledBytes"`
	AssemblyHints        []string         `json:"reassembledHints,omitempty"`
	UniqueAssemblies     int              `json:"uniqueReassembledMessages"`
	UniqueAssemblyBytes  int              `json:"uniqueReassembledBytes"`
	ExportNames          []string         `json:"exportNames,omitempty"`
	ExportBunches        int              `json:"exportBunches"`
	ExportParseSuccess   int              `json:"exportParseSuccess"`
	InventoryGUIDs       []uint32         `json:"inventoryGuids,omitempty"`
	InventoryPayloads    int              `json:"inventoryPayloads"`
	InventoryBytes       int              `json:"inventoryPayloadBytes"`
	RPC256               int              `json:"rpc256"`
	RPC256Decoded        int              `json:"rpc256Decoded"`
	RPC256Owners         []uint32         `json:"rpc256Owners,omitempty"`
	InventoryProperties  []string         `json:"inventoryProperties,omitempty"`
	InventoryGUIDHits    int              `json:"inventoryGuidBitHits"`
	InventoryHitChannels []uint32         `json:"inventoryHitChannels,omitempty"`
	AtkAddValues         []int            `json:"atkAddValues,omitempty"`
	InventoryContainers  int              `json:"inventoryContainers"`
	InventoryChildren    []string         `json:"inventoryChildren,omitempty"`
	DecodedItems         int              `json:"decodedItems"`
	Items                []inventoryItem  `json:"items,omitempty"`
	Characters           []characterItem  `json:"characters,omitempty"`
	Weapons              []weaponItem     `json:"weapons,omitempty"`
	Resources            []resourceItem   `json:"resources,omitempty"`
	CharacterProbes      []characterProbe `json:"characterProbes,omitempty"`
}

type completedMessage struct {
	data    []byte
	bits    int
	exports bool
	channel uint32
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	input := flag.String("input", "nte-login.pcapng", "capture PCAPNG")
	jsonOut := flag.Bool("json", false, "print JSON output")
	capture := flag.String("capture", "", "start an interactive capture with this name")
	dumpUDP := flag.String("dump-udp", "", "directory for reconstructed UDP messages")
	catalog := flag.String("catalog", "equipment.json", "NTE equipment.json catalog")
	inventoryOut := flag.String("inventory-out", "", "write decoded equipment to this JSON file")
	charactersOut := flag.String("characters-out", "", "write decoded characters to this JSON file")
	characterProbeOut := flag.String("character-probe-out", "", "write unknown character fields to this JSON file")
	characterCatalogPath := flag.String("character-catalog", "characters.json", "NTE characters.json catalog")
	weaponsOut := flag.String("weapons-out", "", "write decoded Arcs to this JSON file")
	forkCatalogPath := flag.String("fork-catalog", "forks.json", "NTE forks.json catalog")
	resourceCatalogPath := flag.String("resource-catalog", "resources.json", "NTE resources.json catalog")
	outputDir := flag.String("output-dir", "", "write all domain JSON files to this directory")
	loginCapture := flag.Bool("login-capture", false, "capture login traffic and export all JSON files")
	loginSeconds := flag.Int("login-seconds", 30, "maximum guided capture duration")
	cancelFile := flag.String("cancel-file", "", "file used to signal capture cancellation")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	ctx, cancelFileWatcher := contextWithCancelFile(ctx, *cancelFile)
	defer cancelFileWatcher()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return
	}
	*catalog = resolveDataFile(*catalog, "equipment.json")
	*characterCatalogPath = resolveDataFile(*characterCatalogPath, "characters.json")
	*forkCatalogPath = resolveDataFile(*forkCatalogPath, "forks.json")
	*resourceCatalogPath = resolveDataFile(*resourceCatalogPath, "resources.json")
	if *capture != "" {
		must(captureMode(ctx, *capture))
		return
	}
	if *loginCapture {
		pcap, err := captureLoginAuto(ctx, "", time.Duration(*loginSeconds)*time.Second)
		must(err)
		*input = pcap
		if *outputDir == "" {
			*outputDir = filepath.Join(filepath.Dir(filepath.Dir(pcap)), "workspace", "scan-output")
		}
	}

	packets, err := readPCAPNG(*input)
	must(err)
	mustScanContext(ctx)
	unique := dedupe(packets)
	r := report{Input: *input, RawPackets: len(packets), UniquePackets: len(unique), DuplicateAppearances: len(packets) - len(unique)}
	r.UDP = analyzeUDP(unique, 0, *dumpUDP, *catalog, *characterCatalogPath, *forkCatalogPath, *resourceCatalogPath, *characterProbeOut != "")
	mustScanContext(ctx)
	if *loginCapture && (r.UDP.DecodedItems == 0 || len(r.UDP.Characters) == 0 || len(r.UDP.Weapons) == 0) {
		must(fmt.Errorf("incomplete capture: equipment=%d, characters=%d, Arcs=%d; start another guided scan and log in while the capture is active", r.UDP.DecodedItems, len(r.UDP.Characters), len(r.UDP.Weapons)))
	}
	if *inventoryOut != "" {
		b, err := json.MarshalIndent(r.UDP.Items, "", "  ")
		must(err)
		must(os.WriteFile(*inventoryOut, b, 0644))
	}
	if *charactersOut != "" {
		b, err := json.MarshalIndent(r.UDP.Characters, "", "  ")
		must(err)
		must(os.WriteFile(*charactersOut, b, 0644))
	}
	if *characterProbeOut != "" {
		b, err := json.MarshalIndent(r.UDP.CharacterProbes, "", "  ")
		must(err)
		must(os.WriteFile(*characterProbeOut, append(b, '\n'), 0644))
	}
	if *weaponsOut != "" {
		b, err := json.MarshalIndent(r.UDP.Weapons, "", "  ")
		must(err)
		must(os.WriteFile(*weaponsOut, b, 0644))
	}
	key, segs := scannerpcap.LargestInboundTCPFlow(toScannerPackets(unique), 30031)
	var frames [][]byte
	if len(segs) > 0 {
		stream, reErr := scannerpcap.Reassemble(segs)
		must(reErr)
		r.TCPFlow = key.String()
		r.TCPSegments = len(segs)
		r.TCPReassembledBytes = len(stream)
		frames, err = scannerunreal.ExtractFrames(stream)
		must(err)
	}
	r.Frames = len(frames)
	for i, raw := range frames {
		decoded, ok := scannerunreal.DecodeLZ4Block(raw)
		if !ok {
			continue
		}
		markers := interestingMarkers(decoded)
		records := recordNames(decoded)
		r.Blocks = append(r.Blocks, blockReport{Frame: i, CompressedBytes: len(raw), DecodedBytes: len(decoded), Markers: markers, Records: records})
		r.LZ4BlocksDecoded++
		if len(decoded) > r.LargestDecodedBlock {
			r.LargestDecodedBlock = len(decoded)
		}
	}
	if *outputDir != "" {
		mustScanContext(ctx)
		must(writeOutputDir(*outputDir, r))
	}
	if *loginCapture {
		must(removeCaptureFiles(*input))
		fmt.Printf("Validated login export written to %s\n", *outputDir)
	}
	if *jsonOut {
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
		return
	}
	fmt.Printf("NTE scanner\nCapture: %s\npktmon packets: %d raw, %d unique (%d duplicates)\nFlow: %s\nReassembled TCP: %d bytes (%d segments)\nFrames: %d\nDecoded LZ4 blocks: %d\nLargest block: %d bytes\n", r.Input, r.RawPackets, r.UniquePackets, r.DuplicateAppearances, r.TCPFlow, r.TCPReassembledBytes, r.TCPSegments, r.Frames, r.LZ4BlocksDecoded, r.LargestDecodedBlock)
	fmt.Printf("Primary UDP flow: %s\nUDP: %d packets, %d payload bytes; largest packet %d; 100 ms peak %d bytes at +%.3fs\n", r.UDP.Flow, r.UDP.Packets, r.UDP.PayloadBytes, r.UDP.LargestPacket, r.UDP.PeakBurstBytes, r.UDP.PeakAtSeconds)
	if len(r.UDP.Readable) > 0 {
		fmt.Printf("UDP strings: %s\n", strings.Join(r.UDP.Readable, ", "))
	}
	fmt.Printf("Recognized Unreal data: %d packets, %d bunches, %d fragments; channels: %v\n", r.UDP.UnrealPackets, r.UDP.UnrealBunches, r.UDP.PartialBunches, r.UDP.Channels)
	fmt.Printf("Reassembled fragmented messages: %d (%d bytes), including %d unique (%d bytes); hints: %v\n", r.UDP.Assemblies, r.UDP.AssemblyBytes, r.UDP.UniqueAssemblies, r.UDP.UniqueAssemblyBytes, r.UDP.AssemblyHints)
	if len(r.UDP.ExportNames) > 0 {
		fmt.Printf("Exports Unreal (%d): %s\n", len(r.UDP.ExportNames), strings.Join(r.UDP.ExportNames, ", "))
	}
	fmt.Printf("Export-marked blocks: %d; decoded blocks: %d\n", r.UDP.ExportBunches, r.UDP.ExportParseSuccess)
	fmt.Printf("InventoryComponent GUID: %v\n", r.UDP.InventoryGUIDs)
	fmt.Printf("InventoryComponent payloads: %d blocks (%d bytes)\n", r.UDP.InventoryPayloads, r.UDP.InventoryBytes)
	fmt.Printf("RPC PlayerState/256: %d, decoded containers: %d, owners: %v; inventory properties: %v\n", r.UDP.RPC256, r.UDP.RPC256Decoded, r.UDP.RPC256Owners, r.UDP.InventoryProperties)
	fmt.Printf("Bit-exact inventory GUID references: %d; channels: %v\n", r.UDP.InventoryGUIDHits, r.UDP.InventoryHitChannels)
	fmt.Printf("Decoded Hotta Inventory containers: %d\n", r.UDP.InventoryContainers)
	fmt.Printf("InventoryComponent child export objects: %d\n", len(r.UDP.InventoryChildren))
	fmt.Printf("Decoded equipment items: %d\n", r.UDP.DecodedItems)
	fmt.Printf("Decoded characters: %d\n", len(r.UDP.Characters))
	fmt.Printf("Decoded Arcs: %d\n", len(r.UDP.Weapons))
	fmt.Printf("Decoded resources: %d\n", len(r.UDP.Resources))
	for _, b := range r.Blocks {
		if len(b.Markers) > 0 {
			fmt.Printf("  frame %d: %d -> %d bytes; components: %s\n", b.Frame, b.CompressedBytes, b.DecodedBytes, strings.Join(b.Markers, ", "))
		}
		if len(b.Records) > 0 {
			fmt.Printf("  records frame %d (%d): %s\n", b.Frame, len(b.Records), strings.Join(b.Records, ", "))
		}
	}
}

func mustScanContext(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		must(fmt.Errorf("scan cancelled: %w", err))
	}
}

func resolveDataFile(value, defaultName string) string {
	if value != defaultName {
		return value
	}
	if exe, err := os.Executable(); err == nil {
		candidate := datafiles.New(filepath.Join(filepath.Dir(exe), "data")).DecodeCatalog(defaultName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		candidate = filepath.Join(filepath.Dir(exe), "data", defaultName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate := datafiles.New(filepath.Join(cwd, "data")).DecodeCatalog(defaultName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		candidate = filepath.Join(cwd, "data", defaultName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return value
}

func recordNames(b []byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range asciiStrings(b, 6) {
		if len(s) > 80 || (!strings.HasSuffix(s, "Record") && !strings.HasSuffix(s, "Rec")) {
			continue
		}
		valid := true
		for _, c := range s {
			if !(c == '_' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
				valid = false
				break
			}
		}
		if valid && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func analyzeUDP(ps []packet, port uint16, dumpDir, catalogPath, characterCatalogPath, forkCatalogPath, resourceCatalogPath string, probeCharacters bool) udpReport {
	key, packets, best := selectPrimaryUDPFlow(ps, port)
	r := udpReport{Flow: fmt.Sprintf("%s:%d -> %s:%d", key.src, key.sport, key.dst, key.dport), Packets: len(packets), PayloadBytes: best}
	if len(packets) == 0 {
		return r
	}
	r.LargestPacket, r.PeakBurstBytes, r.PeakAtSeconds = measureUDPFlow(packets)
	completed, inventoryGUIDs, dumpDir := assembleUnrealMessages(&r, packets, dumpDir)
	inspectCompletedMessages(&r, completed, inventoryGUIDs, dumpDir)
	decodeDomains(&r, packets, completed, catalogPath, characterCatalogPath, forkCatalogPath, resourceCatalogPath, probeCharacters)
	return r
}

func isPrivateIPv4(s string) bool {
	ip := net.ParseIP(s).To4()
	if ip == nil {
		return true
	}
	return ip[0] == 10 || ip[0] == 127 || (ip[0] == 192 && ip[1] == 168) || (ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) || (ip[0] == 169 && ip[1] == 254) || ip[0] >= 224
}

func asciiStrings(b []byte, min int) []string {
	var out []string
	for i := 0; i < len(b); {
		if b[i] < 32 || b[i] > 126 {
			i++
			continue
		}
		j := i
		for j < len(b) && b[j] >= 32 && b[j] <= 126 {
			j++
		}
		if j-i >= min {
			s := string(b[i:j])
			if strings.IndexFunc(s, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }) >= 0 {
				out = append(out, s)
			}
		}
		i = j
	}
	return out
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func readPCAPNG(path string) ([]packet, error) {
	packets, err := scannerpcap.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := make([]packet, len(packets))
	for index, capturedPacket := range packets {
		out[index] = packet{ts: capturedPacket.Timestamp, data: capturedPacket.Data}
	}
	return out, nil
}

func dedupe(in []packet) []packet {
	unique := scannerpcap.Deduplicate(toScannerPackets(in), .002)
	out := make([]packet, len(unique))
	for index, capturedPacket := range unique {
		out[index] = packet{ts: capturedPacket.Timestamp, data: capturedPacket.Data}
	}
	return out
}

func toScannerPackets(in []packet) []scannerpcap.Packet {
	packets := make([]scannerpcap.Packet, len(in))
	for index, capturedPacket := range in {
		packets[index] = scannerpcap.Packet{Timestamp: capturedPacket.ts, Data: capturedPacket.data}
	}
	return packets
}

func interestingMarkers(b []byte) []string {
	names := []string{"AchievementRecord", "TraceItemDataRec", "LockerRecord", "SystematicPlayerRecord", "GashaponHistoryRecord", "StoreBrandItemRecord", "RandomItemFixedRecord", "RandomItemDynamicRecord"}
	var found []string
	for _, n := range names {
		if bytes.Contains(b, append([]byte(n), 0)) {
			found = append(found, n)
		}
	}
	return found
}
