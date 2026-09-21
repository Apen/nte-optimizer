package unreal

import (
	"encoding/binary"
	"fmt"
)

func ExtractFrames(stream []byte) ([][]byte, error) {
	var blocks [][]byte
	for offset := 0; offset+4 <= len(stream); {
		size := int(binary.LittleEndian.Uint32(stream[offset : offset+4]))
		end := offset + 4 + size
		if size == 0 || end > len(stream) {
			return blocks, fmt.Errorf("invalid game frame at offset %d (size %d)", offset, size)
		}
		frame := stream[offset+4 : end]
		if len(frame) >= 16 {
			headerSize := int(binary.LittleEndian.Uint32(frame[:4]))
			dataOffset := 12 + headerSize
			if dataOffset+4 <= len(frame) {
				dataSize := int(binary.LittleEndian.Uint32(frame[dataOffset : dataOffset+4]))
				if dataSize > 0 && dataOffset+4+dataSize <= len(frame) {
					blocks = append(blocks, append([]byte(nil), frame[dataOffset+4:dataOffset+4+dataSize]...))
				}
			}
		}
		offset = end
	}
	return blocks, nil
}

func DecodeLZ4Block(input []byte) ([]byte, bool) {
	output := []byte{}
	for offset := 0; offset < len(input); {
		if zeroPadding(input[offset:]) {
			return output, len(output) > 0
		}
		token := input[offset]
		offset++
		literalLength, nextOffset, ok := extendedLength(input, offset, int(token>>4))
		if !ok {
			return nil, false
		}
		offset = nextOffset
		if offset+literalLength > len(input) {
			return nil, false
		}
		output = append(output, input[offset:offset+literalLength]...)
		offset += literalLength
		if offset == len(input) {
			return output, true
		}
		if zeroPadding(input[offset:]) {
			return output, true
		}
		if offset+2 > len(input) {
			return nil, false
		}
		matchOffset := int(binary.LittleEndian.Uint16(input[offset : offset+2]))
		offset += 2
		if matchOffset == 0 || matchOffset > len(output) {
			return nil, false
		}
		matchLength, nextOffset, ok := extendedLength(input, offset, int(token&15))
		if !ok {
			return nil, false
		}
		offset = nextOffset
		for index := 0; index < matchLength+4; index++ {
			output = append(output, output[len(output)-matchOffset])
		}
	}
	return output, len(output) > 0
}

func extendedLength(input []byte, offset, length int) (int, int, bool) {
	if length != 15 {
		return length, offset, true
	}
	for offset < len(input) {
		value := int(input[offset])
		offset++
		length += value
		if value != 255 {
			return length, offset, true
		}
	}
	return 0, offset, false
}

func zeroPadding(input []byte) bool {
	if len(input) > 3 {
		return false
	}
	for _, value := range input {
		if value != 0 {
			return false
		}
	}
	return true
}
