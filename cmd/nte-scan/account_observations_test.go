package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func observationString(b *bytes.Buffer, s string) {
	binary.Write(b, binary.LittleEndian, uint32(len(s)+1))
	b.WriteString(s)
	b.WriteByte(0)
}
func furniturePayload(level uint32) []byte {
	b := new(bytes.Buffer)
	binary.Write(b, binary.LittleEndian, uint16(1))
	observationString(b, "SF_test")
	b.WriteByte(0)
	binary.Write(b, binary.LittleEndian, uint32(0))
	b.WriteByte(1)
	binary.Write(b, binary.LittleEndian, level)
	binary.Write(b, binary.LittleEndian, uint32(0))
	binary.Write(b, binary.LittleEndian, uint64(0))
	binary.Write(b, binary.LittleEndian, uint32(0))
	data := b.Bytes()
	out := []byte{0xf0}
	n := len(data) - 15
	for n >= 255 {
		out = append(out, 255)
		n -= 255
	}
	out = append(out, byte(n))
	return append(out, data...)
}
func TestFurnitureLevelsAndUnalignedProperty(t *testing.T) {
	payload := furniturePayload(3)
	levels, ok := parseFurnitureLevels(payload)
	if !ok || len(levels) != 1 || levels[0].Level != 3 || levels[0].ProgressionID != "SF_test" {
		t.Fatalf("unexpected levels: %v %v", levels, ok)
	}
	for _, invalid := range [][]byte{furniturePayload(0), furniturePayload(101), payload[:len(payload)-1]} {
		if _, ok := parseFurnitureLevels(invalid); ok {
			t.Fatal("accepted invalid furniture payload")
		}
	}
	b := new(bytes.Buffer)
	observationString(b, "FurnitureActivateStates")
	binary.Write(b, binary.LittleEndian, uint32(0))
	binary.Write(b, binary.LittleEndian, uint32(len(payload)))
	b.Write(payload)
	for shift := 0; shift < 8; shift++ {
		w := recordWriter{}
		w.put(0, shift)
		for _, v := range b.Bytes() {
			w.put(uint64(v), 8)
		}
		got := namedBytePayloads(w.data, w.bits, "FurnitureActivateStates")
		if len(got) != 1 || !bytes.Equal(got[0], payload) {
			t.Fatalf("property not decoded at shift %d", shift)
		}
		if got := namedBytePayloads(w.data, w.bits-8, "FurnitureActivateStates"); len(got) != 0 {
			t.Fatal("accepted truncated property")
		}
	}
}
func TestBondFlagsDoNotConsumeReader(t *testing.T) {
	b := new(bytes.Buffer)
	binary.Write(b, binary.LittleEndian, uint32(0))
	b.WriteByte(0)
	observationString(b, "None")
	observationString(b, "None")
	binary.Write(b, binary.LittleEndian, uint32(0))
	binary.Write(b, binary.LittleEndian, uint32(2))
	b.Write([]byte{1, 0})
	binary.Write(b, binary.LittleEndian, uint32(0))
	r := invBits{data: b.Bytes(), bitLen: b.Len() * 8}
	flags, ok := readBondStageFlags(&r)
	if !ok || len(flags) != 2 || !flags[0] || flags[1] || r.cursor != 0 {
		t.Fatalf("unexpected flags: %v %v", flags, ok)
	}
	r.bitLen -= 8
	if _, ok := readBondStageFlags(&r); ok {
		t.Fatal("accepted truncated flags")
	}
}
