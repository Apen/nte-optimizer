package unreal

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func ExtractAtkAdd(data []byte) []int {
	needle := []byte("AtkAdd\x00")
	var result []int
	for position := 0; ; {
		index := bytes.Index(data[position:], needle)
		if index < 0 {
			break
		}
		index += position
		offset := index + len(needle) + 4
		if offset+22 <= len(data) && binary.LittleEndian.Uint32(data[offset:offset+4]) == 18 {
			blob := data[offset+4 : offset+22]
			if blob[13] == 8 && blob[14] == 0 && blob[16] == 0 && blob[17] == 0 && blob[15]&1 == 0 {
				result = append(result, int(blob[15]/2))
			}
		}
		position = index + len(needle)
	}
	return result
}

type PackageExport struct {
	GUID  uint32
	Outer uint32
	Path  string
}

type ContentBlock struct {
	GUID           uint32
	Bits           int
	Data           []byte
	Actor          bool
	HasReplication bool
}

type RPCBlock struct {
	Index uint32
	Bits  int
	Data  []byte
}

type Property struct {
	Name string
	Data []byte
	Bits int
}

func ParsePackageExports(data []byte, bits int) []PackageExport {
	exports, _ := ParsePackageExportsAt(data, bits)
	return exports
}

func ParsePackageExportsAt(data []byte, bits int) ([]PackageExport, int) {
	reader := bitReader{data: data, end: bits}
	hasReplication, ok := reader.bit()
	if !ok || hasReplication != 0 {
		return nil, 0
	}
	count, ok := reader.bits(32)
	if !ok || count > 10000 {
		return nil, 0
	}
	var result []PackageExport
	var load func(int) (uint32, bool)
	load = func(depth int) (uint32, bool) {
		if depth > 32 {
			return 0, false
		}
		guid, ok := reader.packed()
		if !ok {
			return 0, false
		}
		if guid == 0 {
			return 0, true
		}
		flags, ok := reader.bits(8)
		if !ok {
			return 0, false
		}
		if flags&1 == 0 {
			return guid, true
		}
		outer, valid := load(depth + 1)
		if !valid {
			return 0, false
		}
		path, ok := readFString(&reader)
		if !ok {
			return 0, false
		}
		if path != "" {
			result = append(result, PackageExport{GUID: guid, Outer: outer, Path: path})
		}
		if flags&4 != 0 {
			if _, ok = reader.bits(32); !ok {
				return 0, false
			}
		}
		return guid, true
	}
	for index := uint64(0); index < count; index++ {
		if _, ok := load(0); !ok {
			break
		}
	}
	return result, reader.pos
}

func ParseContentBlocks(data []byte, bits, start int) []ContentBlock {
	reader := bitReader{data: data, pos: start, end: bits}
	var result []ContentBlock
	for reader.pos+10 < bits-1 {
		hasReplication, ok := reader.bit()
		if !ok {
			break
		}
		actor, ok := reader.bit()
		if !ok {
			break
		}
		guid := uint32(0)
		if actor == 0 {
			guid, ok = reader.packed()
			if !ok {
				break
			}
		}
		count, ok := reader.packed()
		if !ok || int(count) > reader.end-reader.pos {
			break
		}
		payload, ok := reader.take(int(count))
		if !ok {
			break
		}
		result = append(result, ContentBlock{GUID: guid, Bits: int(count), Data: payload, Actor: actor != 0, HasReplication: hasReplication != 0})
	}
	return result
}

func ParseRPCs(data []byte, bits int, max uint32) []RPCBlock {
	reader := bitReader{data: data, end: bits}
	var result []RPCBlock
	for reader.pos < bits-1 {
		index, ok := readCompressed(&reader, max)
		if !ok {
			break
		}
		count, ok := reader.packed()
		if !ok || int(count) > reader.end-reader.pos {
			break
		}
		payload, ok := reader.take(int(count))
		if !ok {
			break
		}
		result = append(result, RPCBlock{Index: index, Bits: int(count), Data: payload})
	}
	return result
}

func ContainsPackedGUID(data []byte, bits int, guid uint32) bool {
	var pattern []byte
	patternBits := 0
	for {
		more := guid>>7 != 0
		value := byte((guid & 0x7f) << 1)
		if more {
			value |= 1
		}
		pattern = append(pattern, value)
		patternBits += 8
		guid >>= 7
		if !more {
			break
		}
	}
	for start := 0; start+patternBits <= bits; start++ {
		match := true
		for index := 0; index < patternBits; index++ {
			actual := (data[(start+index)/8] >> uint((start+index)%8)) & 1
			expected := (pattern[index/8] >> uint(index%8)) & 1
			if actual != expected {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func ParseHottaContainer(data []byte, bits int) (uint32, []string, bool) {
	reader := bitReader{data: data, end: bits}
	version, ok := reader.bits(32)
	if !ok || version != 1 {
		return 0, nil, false
	}
	if _, ok = reader.bits(17); !ok {
		return 0, nil, false
	}
	owner, ok := reader.packed()
	if !ok {
		return 0, nil, false
	}
	count, ok := reader.bits(32)
	if !ok || count > 10000 {
		return 0, nil, false
	}
	var names []string
	for index := uint64(0); index < count; index++ {
		name, ok := readName(&reader)
		if !ok {
			return owner, names, false
		}
		length, ok := reader.bits(32)
		if !ok || length > 1<<24 || int(length)*8 > reader.end-reader.pos {
			return owner, names, false
		}
		names = append(names, name)
		reader.pos += int(length) * 8
	}
	return owner, names, true
}

func FindHottaContainers(data []byte, bits int, target uint32) [][]Property {
	var result [][]Property
	for start := 0; start+81 < bits; start++ {
		reader := bitReader{data: data, pos: start, end: bits}
		version, ok := reader.bits(32)
		if !ok || version != 1 {
			continue
		}
		if _, ok = reader.bits(17); !ok {
			continue
		}
		owner, ok := reader.packed()
		if !ok || owner != target {
			continue
		}
		count, ok := reader.bits(32)
		if !ok || count == 0 || count > 4096 {
			continue
		}
		properties := make([]Property, 0, count)
		valid := true
		for index := uint64(0); index < count; index++ {
			name, ok := readName(&reader)
			if !ok || name == "" {
				valid = false
				break
			}
			length, ok := reader.bits(32)
			if !ok || length > 1<<24 || int(length)*8 > reader.end-reader.pos {
				valid = false
				break
			}
			payload, ok := reader.take(int(length) * 8)
			if !ok {
				valid = false
				break
			}
			properties = append(properties, Property{Name: name, Data: payload, Bits: int(length) * 8})
		}
		if valid {
			result = append(result, properties)
			start = reader.pos - 1
		}
	}
	return result
}

func readCompressed(reader *bitReader, max uint32) (uint32, bool) {
	mask := uint32(1)
	value := uint32(0)
	for value+mask < max && mask != 0 {
		bit, ok := reader.bit()
		if !ok {
			return 0, false
		}
		if bit != 0 {
			value |= mask
		}
		mask <<= 1
	}
	return value, true
}

func readName(reader *bitReader) (string, bool) {
	hardReference, ok := reader.bit()
	if !ok {
		return "", false
	}
	if hardReference != 0 {
		value, ok := reader.packed()
		if !ok {
			return "", false
		}
		return fmt.Sprintf("#%d", value), true
	}
	value, ok := readFString(reader)
	if !ok {
		return "", false
	}
	if _, ok = reader.bits(32); !ok {
		return "", false
	}
	return value, true
}

func readFString(reader *bitReader) (string, bool) {
	raw, ok := reader.bits(32)
	if !ok {
		return "", false
	}
	length := int32(raw)
	if length == 0 {
		return "", true
	}
	if length > 0 {
		if length > 4096 {
			return "", false
		}
		value, ok := reader.take(int(length) * 8)
		if !ok {
			return "", false
		}
		if len(value) > 0 && value[len(value)-1] == 0 {
			value = value[:len(value)-1]
		}
		return string(value), true
	}
	count := int(-length)
	if count > 4096 {
		return "", false
	}
	value, ok := reader.take(count * 16)
	if !ok {
		return "", false
	}
	runes := make([]rune, 0, count)
	for index := 0; index+1 < len(value); index += 2 {
		char := binary.LittleEndian.Uint16(value[index : index+2])
		if char == 0 {
			break
		}
		runes = append(runes, rune(char))
	}
	return string(runes), true
}
