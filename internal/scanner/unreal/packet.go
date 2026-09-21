package unreal

type Bunch struct {
	Channel  uint32
	Partial  bool
	Initial  bool
	Final    bool
	Data     []byte
	DataBits int
	Exports  bool
}

type bitReader struct {
	data []byte
	pos  int
	end  int
}

func (r *bitReader) bit() (uint64, bool) {
	if r.pos >= r.end {
		return 0, false
	}
	value := uint64((r.data[r.pos/8] >> uint(r.pos%8)) & 1)
	r.pos++
	return value, true
}

func (r *bitReader) bits(count int) (uint64, bool) {
	var value uint64
	for index := 0; index < count; index++ {
		bit, ok := r.bit()
		if !ok {
			return 0, false
		}
		value |= bit << uint(index)
	}
	return value, true
}

func (r *bitReader) packed() (uint32, bool) {
	var value uint32
	for shift := 0; shift <= 28; shift += 7 {
		more, ok := r.bit()
		if !ok {
			return 0, false
		}
		part, ok := r.bits(7)
		if !ok {
			return 0, false
		}
		value |= uint32(part) << uint(shift)
		if more == 0 {
			return value, true
		}
	}
	return 0, false
}

func (r *bitReader) take(count int) ([]byte, bool) {
	if count < 0 || r.pos+count > r.end {
		return nil, false
	}
	result := make([]byte, (count+7)/8)
	for index := 0; index < count; index++ {
		value, _ := r.bit()
		if value != 0 {
			result[index/8] |= 1 << uint(index%8)
		}
	}
	return result, true
}

func TerminatedBits(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	index := len(data) - 1
	for index >= 0 && data[index] == 0 {
		index--
	}
	if index < 0 {
		return 0
	}
	value := data[index]
	high := 0
	for value>>1 != 0 {
		value >>= 1
		high++
	}
	return index*8 + high + 1
}

func ParsePacket(data []byte) ([]Bunch, bool) {
	end := TerminatedBits(data) - 1
	if end < 80 {
		return nil, false
	}
	reader := bitReader{data: data, end: end}
	if _, ok := reader.bits(5); !ok {
		return nil, false
	}
	handshake, ok := reader.bit()
	if !ok || handshake != 0 {
		return nil, false
	}
	packetHeader, ok := reader.bits(32)
	if !ok {
		return nil, false
	}
	headerWords := int(packetHeader&15) + 1
	if _, ok = reader.bits(headerWords * 32); !ok {
		return nil, false
	}
	info, ok := reader.bit()
	if !ok {
		return nil, false
	}
	if info != 0 {
		if _, ok = reader.bits(10); !ok {
			return nil, false
		}
		if _, ok = reader.bit(); !ok {
			return nil, false
		}
	}
	var result []Bunch
	for reader.pos+20 <= reader.end {
		control, ok := reader.bit()
		if !ok {
			return nil, false
		}
		var open, close uint64
		if control != 0 {
			open, _ = reader.bit()
			close, _ = reader.bit()
		}
		if close != 0 {
			if _, ok = reader.bits(4); !ok {
				return nil, false
			}
		}
		if _, ok = reader.bit(); !ok {
			return nil, false
		}
		reliable, ok := reader.bit()
		if !ok {
			return nil, false
		}
		channel, ok := reader.packed()
		if !ok || channel > 65535 {
			return nil, false
		}
		exports, ok := reader.bit()
		if !ok {
			return nil, false
		}
		if _, ok = reader.bit(); !ok {
			return nil, false
		}
		partial, ok := reader.bit()
		if !ok {
			return nil, false
		}
		if reliable != 0 {
			if _, ok = reader.bits(10); !ok {
				return nil, false
			}
		}
		initial, final := false, false
		if partial != 0 {
			initialBit, ok := reader.bit()
			if !ok {
				return nil, false
			}
			initial = initialBit != 0
			if _, ok = reader.bit(); !ok {
				return nil, false
			}
			finalBit, ok := reader.bit()
			if !ok {
				return nil, false
			}
			final = finalBit != 0
		}
		if reliable != 0 || open != 0 {
			hardReference, ok := reader.bit()
			if !ok {
				return nil, false
			}
			if hardReference != 0 {
				if _, ok = reader.packed(); !ok {
					return nil, false
				}
			} else {
				length, ok := reader.bits(32)
				if !ok || length > 1024 {
					return nil, false
				}
				if _, ok = reader.bits(int(length)*8 + 32); !ok {
					return nil, false
				}
			}
		}
		payloadBits, ok := reader.bits(13)
		if !ok || int(payloadBits) > reader.end-reader.pos {
			return nil, false
		}
		payload, ok := reader.take(int(payloadBits))
		if !ok {
			return nil, false
		}
		result = append(result, Bunch{Channel: channel, Partial: partial != 0, Initial: initial, Final: final, Data: payload, DataBits: int(payloadBits), Exports: exports != 0})
	}
	return result, len(result) > 0
}

func AppendBits(destination []byte, destinationBits int, source []byte, sourceBits int) ([]byte, int) {
	total := destinationBits + sourceBits
	if len(destination) < (total+7)/8 {
		destination = append(destination, make([]byte, (total+7)/8-len(destination))...)
	}
	for index := 0; index < sourceBits; index++ {
		if source[index/8]&(1<<uint(index%8)) != 0 {
			position := destinationBits + index
			destination[position/8] |= 1 << uint(position%8)
		}
	}
	return destination, total
}
