package main

import "strconv"

const likeabilityInfoType = "FLikeabilityInfo"

type likeabilityRecord struct {
	key   uint32
	level int
}

func currentCharacterIDForLikeabilityKey(key uint32) uint32 {
	// These relationship keys use older character config IDs than the current roster.
	switch key {
	case 1005:
		return 1054 // Daffodill
	case 1014:
		return 1055 // Jiuyuan
	case 1029:
		return 1052 // Hotori
	default:
		return key
	}
}

func parseLikeabilityRecords(data []byte, bitLen int) []likeabilityRecord {
	if !containsLikeabilityInfoType(data, bitLen) {
		return nil
	}

	var records []likeabilityRecord
	for offset := 0; offset < bitLen; offset++ {
		length, ok := likeabilityBitsAt(data, bitLen, offset, 32)
		if !ok || length != 5 {
			continue
		}
		reader := invBits{data: data, bitLen: bitLen, cursor: offset}
		keyText, ok := reader.fstring(8)
		if !ok || len(keyText) != 4 {
			continue
		}
		key, err := strconv.ParseUint(keyText, 10, 32)
		if err != nil {
			continue
		}
		affection, ok := reader.u32()
		if !ok || affection > 1_000_000 {
			continue
		}
		level, ok := reader.u32()
		if !ok || level > 10 {
			continue
		}
		records = append(records, likeabilityRecord{key: uint32(key), level: int(level)})
	}
	return records
}

func containsLikeabilityInfoType(data []byte, bitLen int) bool {
	for offset := 0; offset < bitLen; offset++ {
		length, ok := likeabilityBitsAt(data, bitLen, offset, 32)
		if !ok || length != uint64(len(likeabilityInfoType)+1) || offset+32+(len(likeabilityInfoType)+1)*8 > bitLen {
			continue
		}
		matches := true
		for index := 0; index <= len(likeabilityInfoType); index++ {
			value, ok := likeabilityBitsAt(data, bitLen, offset+32+index*8, 8)
			want := byte(0)
			if index < len(likeabilityInfoType) {
				want = likeabilityInfoType[index]
			}
			if !ok || byte(value) != want {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func likeabilityBitsAt(data []byte, bitLen, offset, count int) (uint64, bool) {
	if count < 0 || count > 64 || offset < 0 || offset+count > bitLen || offset+count > len(data)*8 {
		return 0, false
	}
	firstByte := offset / 8
	bitOffset := offset % 8
	byteCount := (bitOffset + count + 7) / 8
	var value uint64
	for index := 0; index < byteCount; index++ {
		value |= uint64(data[firstByte+index]) << (index * 8)
	}
	value >>= bitOffset
	if count < 64 {
		value &= (uint64(1) << count) - 1
	}
	return value, true
}

func attachBondLevels(characters []characterItem, messages []completedMessage) {
	characterIDs := make(map[uint32]bool, len(characters))
	for _, character := range characters {
		characterIDs[character.CharacterID] = true
	}

	levels := make(map[uint32]int)
	for _, message := range messages {
		if message.channel != 4 {
			continue
		}
		for _, record := range parseLikeabilityRecords(message.data, message.bits) {
			characterID := currentCharacterIDForLikeabilityKey(record.key)
			if characterIDs[characterID] {
				levels[characterID] = record.level
			}
		}
	}

	for index := range characters {
		if level, ok := levels[characters[index].CharacterID]; ok {
			characters[index].BondLevel = &level
		}
	}
}
