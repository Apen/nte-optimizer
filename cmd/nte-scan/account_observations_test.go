package main

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"nte-optimizer/internal/nte"
)

func observationString(b *bytes.Buffer, s string) {
	binary.Write(b, binary.LittleEndian, uint32(len(s)+1))
	b.WriteString(s)
	b.WriteByte(0)
}
func furniturePayload(level uint32) []byte {
	encoded := encodeFurnitureLevels([]nte.FurnitureLevel{{ProgressionID: "SF_test", Level: int(level)}}, false)
	return furnitureLiteralBlock(encoded.data)
}

func furnitureLiteralBlock(data []byte) []byte {
	out := []byte{0xf0}
	n := len(data) - 15
	for n >= 255 {
		out = append(out, 255)
		n -= 255
	}
	out = append(out, byte(n))
	return append(out, data...)
}

func encodeFurnitureLevels(levels []nte.FurnitureLevel, packed bool) recordWriter {
	var w recordWriter
	w.put(uint64(len(levels)), 16)
	for i, level := range levels {
		flagBits := 8
		if packed {
			w.putString(level.ProgressionID)
			flagBits = 1
		} else {
			var name bytes.Buffer
			observationString(&name, level.ProgressionID)
			for _, b := range name.Bytes() {
				w.put(uint64(b), 8)
			}
		}
		w.put(uint64(i%2), flagBits)
		w.put(59, 32)
		w.put(uint64((i+1)%2), flagBits)
		w.put(uint64(level.Level), 32)
		w.put(0, 32)
		w.put(1234, 64)
	}
	w.put(0, 32)
	return w
}

func TestPackedFurnitureLevels(t *testing.T) {
	want := []nte.FurnitureLevel{
		{ProgressionID: "SF_0011", Level: 2},
		{ProgressionID: "SF_0012", Level: 7},
		{ProgressionID: "SF_0013", Level: 4},
	}
	valid := encodeFurnitureLevels(want, true)
	got, ok := parseFurnitureLevels(furnitureLiteralBlock(valid.data), nil)
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("packed levels = %v, accepted=%t, want %v", got, ok, want)
	}

	badPadding := encodeFurnitureLevels(want, true)
	badPadding.put(1, 1)
	for name, data := range map[string][]byte{
		"nonzero padding": badPadding.data,
		"extra byte":      append(slices.Clone(valid.data), 0),
		"truncated":       valid.data[:len(valid.data)-1],
	} {
		t.Run(name, func(t *testing.T) {
			if got, ok := parseFurnitureLevels(furnitureLiteralBlock(data), nil); ok || got != nil {
				t.Fatalf("invalid packed levels accepted: %v", got)
			}
		})
	}
}

func TestFurnitureDuplicateProgressions(t *testing.T) {
	want := []nte.FurnitureLevel{
		{ProgressionID: "SF_0011", Level: 2},
		{ProgressionID: "SF_0012", Level: 7},
		{ProgressionID: "SF_0013", Level: 4},
	}
	observed := []nte.FurnitureLevel{
		want[0],
		want[0],
		{ProgressionID: "SF_exploration", Level: 3},
		{ProgressionID: "SF_exploration", Level: 2},
		{ProgressionID: "SF_exploration", Level: 3},
		want[1],
		want[2],
	}
	for _, tc := range []struct {
		name   string
		packed bool
	}{{"packed", true}, {"legacy", false}} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := &recordDiagnostics{}
			encoded := encodeFurnitureLevels(observed, tc.packed)
			got, ok := parseFurnitureLevels(furnitureLiteralBlock(encoded.data), diagnostics)
			if !ok || !slices.Equal(got, want) {
				t.Fatalf("levels = %v, accepted=%t, want %v", got, ok, want)
			}
			if diagnostics.counts["furniture_level_conflicts"] != 1 {
				t.Fatalf("expected one conflicting progression: %v", diagnostics.counts)
			}

			invalid := slices.Clone(observed)
			invalid[1].Level = 101
			encoded = encodeFurnitureLevels(invalid, tc.packed)
			if got, ok := parseFurnitureLevels(furnitureLiteralBlock(encoded.data), nil); ok || got != nil {
				t.Fatalf("accepted malformed duplicate: %v", got)
			}
		})
	}
}
func TestFurnitureLevelsAndUnalignedProperty(t *testing.T) {
	payload := furniturePayload(3)
	levels, ok := parseFurnitureLevels(payload, nil)
	if !ok || len(levels) != 1 || levels[0].Level != 3 || levels[0].ProgressionID != "SF_test" {
		t.Fatalf("unexpected levels: %v %v", levels, ok)
	}
	for _, invalid := range [][]byte{furniturePayload(0), furniturePayload(101), payload[:len(payload)-1]} {
		if _, ok := parseFurnitureLevels(invalid, nil); ok {
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
		got := namedBytePayloads(w.data, w.bits, "FurnitureActivateStates", nil)
		if len(got) != 1 || !bytes.Equal(got[0], payload) {
			t.Fatalf("property not decoded at shift %d", shift)
		}
		if got := namedBytePayloads(w.data, w.bits-8, "FurnitureActivateStates", nil); len(got) != 0 {
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
