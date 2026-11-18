package main

import (
	"bytes"
	"strings"

	"nte-optimizer/internal/nte"
	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

// namedBytePayloads recognizes bounded FName + byte-array property envelopes
// independently of the PlayerState function table, which varies by version.
func namedBytePayloads(data []byte, bitLen int, name string, diagnostics *recordDiagnostics) [][]byte {
	if bitLen < 0 || bitLen > len(data)*8 {
		return nil
	}
	var result [][]byte
	needle := append([]byte(name), 0)
	for shift := 0; shift < 8 && shift < bitLen; shift++ {
		aligned, ok := extractBitRange(data, shift, bitLen-shift)
		if !ok {
			continue
		}
		for pos := 0; pos < len(aligned); {
			index := bytes.Index(aligned[pos:], needle)
			if index < 0 {
				break
			}
			index += pos
			pos = index + len(needle)
			diagnostics.add("property_matches", 1)
			offset := shift + index*8
			length, ok := likeabilityBitsAt(data, bitLen, offset-32, 32)
			if !ok || length != uint64(len(name)+1) {
				diagnostics.add("rejected_property_name", 1)
				continue
			}
			r := invBits{data: data, bitLen: bitLen, cursor: offset + len(needle)*8}
			number, ok := r.u32()
			if !ok || number != 0 {
				diagnostics.add("rejected_property_number", 1)
				continue
			}
			size, ok := r.u32()
			if !ok || size == 0 || size > 8192 {
				diagnostics.add("rejected_property_size", 1)
				continue
			}
			payload, ok := r.bytes(int(size))
			if ok {
				result = append(result, payload)
				diagnostics.maximum("largest_payload_bytes", len(payload))
			} else {
				diagnostics.add("rejected_property_truncated", 1)
			}
		}
	}
	return result
}

func parseFurnitureLevels(payload []byte, diagnostics *recordDiagnostics) (_ []nte.FurnitureLevel, accepted bool) {
	phase := "lz4"
	defer func() { diagnostics.record(phase, accepted) }()
	data, ok := scannerunreal.DecodeLZ4BlockLimited(payload, 65536)
	if !ok {
		return nil, false
	}
	diagnostics.maximum("largest_decoded_bytes", len(data))
	phase = "furniture_count"
	r := invBits{data: data, bitLen: len(data) * 8}
	count, ok := r.u16()
	if !ok || count == 0 || count > 256 {
		return nil, false
	}
	// Packed records use dynamic names and one-bit flags. Older records start
	// with a plain FString and use byte-sized flags throughout the array.
	first := r
	firstID, ok := first.fstring(80)
	packed := !ok || !strings.HasPrefix(firstID, "SF_")
	flagBits := 8
	if packed {
		flagBits = 1
	}
	levels := make([]nte.FurnitureLevel, 0, count)
	seen := map[string]bool{}
	for index := uint16(0); index < count; index++ {
		phase = "furniture_id"
		var id string
		if packed {
			id, ok = r.dynamicName()
		} else {
			id, ok = r.fstring(80)
		}
		if !ok || !strings.HasPrefix(id, "SF_") || seen[id] {
			return nil, false
		}
		seen[id] = true
		phase = "furniture_fields"
		// Only the level is interpreted. Neither observed flag has a
		// verified activation meaning, so it must not become confirmation.
		flag0, ok0 := r.bits(flagBits)
		_, ok1 := r.u32()
		flag1, okFlag1 := r.bits(flagBits)
		level, ok2 := r.u32()
		_, ok3 := r.u32()
		_, ok4 := r.i64()
		if !ok0 || !ok1 || !okFlag1 || !ok2 || !ok3 || !ok4 || flag0 > 1 || flag1 > 1 || level < 1 || level > 100 {
			return nil, false
		}
		levels = append(levels, nte.FurnitureLevel{ProgressionID: id, Level: int(level)})
	}
	phase = "furniture_trailer"
	if _, ok := r.u32(); !ok {
		return nil, false
	}
	remaining := r.bitLen - r.cursor
	if remaining != 0 {
		if !packed || remaining >= 8 {
			return nil, false
		}
		if padding, _ := r.bits(remaining); padding != 0 {
			return nil, false
		}
	}
	return levels, true
}

func decodeAccountObservations(report *udpReport, messages []completedMessage) {
	report.diagnostic("decode_furniture", "started", "", nil)
	diagnostics := &recordDiagnostics{counts: map[string]int{}}
	for _, message := range messages {
		if message.channel != 4 {
			continue
		}
		diagnostics.add("channel4_messages", 1)
		for _, payload := range namedBytePayloads(message.data, message.bits, "FurnitureActivateStates", diagnostics) {
			if levels, ok := parseFurnitureLevels(payload, diagnostics); ok {
				report.FurnitureLevels = levels
			}
		}
	}
	diagnostics.counts["furniture"] = len(report.FurnitureLevels)
	status, code := "complete", "furniture_decoded"
	if diagnostics.counts["channel4_messages"] == 0 {
		status, code = "partial", "channel4_absent"
	} else if diagnostics.counts["property_matches"] == 0 {
		status, code = "partial", "furniture_property_absent"
	} else if len(report.FurnitureLevels) == 0 {
		status, code = "partial", "furniture_rejected"
	}
	report.diagnostic("decode_furniture", status, code, diagnostics.counts)
}

func readBondStageFlags(reader *invBits) ([]bool, bool) {
	r := *reader
	if _, ok := r.u32(); !ok {
		return nil, false
	}
	flag, ok := r.bits(8)
	if !ok || flag > 1 {
		return nil, false
	}
	if _, ok := r.fstring(128); !ok {
		return nil, false
	}
	if _, ok := r.fstring(128); !ok {
		return nil, false
	}
	if _, ok := r.u32(); !ok {
		return nil, false
	}
	count, ok := r.u32()
	if !ok || count > 10 {
		return nil, false
	}
	flags := make([]bool, count)
	for index := range flags {
		value, ok := r.bits(8)
		if !ok || value > 1 {
			return nil, false
		}
		flags[index] = value == 1
	}
	if _, ok := r.u32(); !ok {
		return nil, false
	}
	return flags, true
}
