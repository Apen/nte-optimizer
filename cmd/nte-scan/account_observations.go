package main

import (
	"bytes"
	"strings"

	"nte-optimizer/internal/nte"
	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

// namedBytePayloads recognizes bounded FName + byte-array property envelopes
// independently of the PlayerState function table, which varies by version.
func namedBytePayloads(data []byte, bitLen int, name string) [][]byte {
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
			offset := shift + index*8
			length, ok := likeabilityBitsAt(data, bitLen, offset-32, 32)
			if !ok || length != uint64(len(name)+1) {
				continue
			}
			r := invBits{data: data, bitLen: bitLen, cursor: offset + len(needle)*8}
			number, ok := r.u32()
			if !ok || number != 0 {
				continue
			}
			size, ok := r.u32()
			if !ok || size == 0 || size > 8192 {
				continue
			}
			payload, ok := r.bytes(int(size))
			if ok {
				result = append(result, payload)
			}
		}
	}
	return result
}

func parseFurnitureLevels(payload []byte) ([]nte.FurnitureLevel, bool) {
	data, ok := scannerunreal.DecodeLZ4BlockLimited(payload, 65536)
	if !ok {
		return nil, false
	}
	r := invBits{data: data, bitLen: len(data) * 8}
	count, ok := r.u16()
	if !ok || count == 0 || count > 256 {
		return nil, false
	}
	levels := make([]nte.FurnitureLevel, 0, count)
	seen := map[string]bool{}
	for index := uint16(0); index < count; index++ {
		id, ok := r.fstring(80)
		if !ok || !strings.HasPrefix(id, "SF_") || seen[id] {
			return nil, false
		}
		seen[id] = true
		// Only the level is interpreted. Neither observed byte flag has a
		// verified activation meaning, so it must not become confirmation.
		flag0, ok0 := r.bits(8)
		_, ok1 := r.u32()
		flag5, ok5 := r.bits(8)
		level, ok2 := r.u32()
		_, ok3 := r.u32()
		_, ok4 := r.i64()
		if !ok0 || !ok1 || !ok5 || !ok2 || !ok3 || !ok4 || flag0 > 1 || flag5 > 1 || level < 1 || level > 100 {
			return nil, false
		}
		levels = append(levels, nte.FurnitureLevel{ProgressionID: id, Level: int(level)})
	}
	if _, ok := r.u32(); !ok || r.cursor != r.bitLen {
		return nil, false
	}
	return levels, true
}

func decodeAccountObservations(report *udpReport, messages []completedMessage) {
	for _, message := range messages {
		if message.channel != 4 {
			continue
		}
		for _, payload := range namedBytePayloads(message.data, message.bits, "FurnitureActivateStates") {
			if levels, ok := parseFurnitureLevels(payload); ok {
				report.FurnitureLevels = levels
			}
		}
	}
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
