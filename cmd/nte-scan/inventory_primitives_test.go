package main

import (
	"math"
	"reflect"
	"testing"
)

func TestInventoryBitReaderPrimitives(t *testing.T) {
	reader := invBits{data: []byte{0b10110110, 0x34, 0x12}, bitLen: 24}
	value, ok := reader.bits(4)
	if !ok || value != 0b0110 {
		t.Fatalf("bits(4) = %04b, %v", value, ok)
	}
	value, ok = reader.bits(4)
	if !ok || value != 0b1011 {
		t.Fatalf("second bits(4) = %04b, %v", value, ok)
	}
	word, ok := reader.u16()
	if !ok || word != 0x1234 {
		t.Fatalf("u16() = %#x, %v", word, ok)
	}
	if _, ok := reader.bits(1); ok {
		t.Fatal("read past end unexpectedly succeeded")
	}
}

func TestInventoryFStringAndNetID(t *testing.T) {
	stringData := []byte{4, 0, 0, 0, 'N', 'T', 'E', 0}
	reader := invBits{data: stringData, bitLen: len(stringData) * 8}
	if got, ok := reader.fstring(16); !ok || got != "NTE" {
		t.Fatalf("fstring() = %q, %v", got, ok)
	}

	idData := []byte{1, 0, 0, 0, 2, 0, 0, 0}
	reader = invBits{data: idData, bitLen: len(idData) * 8}
	if got, ok := reader.netID(); !ok || !reflect.DeepEqual(got, itemNetID{Solt: 1, Serial: 2}) || !validNetID(got) {
		t.Fatalf("netID() = %#v, %v", got, ok)
	}
	if validNetID(itemNetID{Solt: math.MaxUint32, Serial: 2}) {
		t.Fatal("validNetID() accepted sentinel value")
	}
}

func TestMainStatValueInterpolatesCurves(t *testing.T) {
	grid := uint32(3)
	catalog := &equipmentCatalog{Curves: map[string][][2]float32{
		"ATK_3_ITEM_QUALITY_ORANGE":   {{0, 10}, {10, 30}},
		"HP_Core_ITEM_QUALITY_PURPLE": {{0, 100}, {5, 200}},
	}}
	module := equipmentDefinition{Kind: "module", Quality: "orange", Grid: &grid}
	if got, ok := mainStatValue(catalog, module, "ATK", 5); !ok || got != 20 {
		t.Fatalf("module mainStatValue() = %v, %v", got, ok)
	}
	cartridge := equipmentDefinition{Kind: "cartridge", Quality: "purple"}
	if got, ok := mainStatValue(catalog, cartridge, "HP", 8); !ok || got != 200 {
		t.Fatalf("cartridge mainStatValue() = %v, %v", got, ok)
	}
	if _, ok := mainStatValue(catalog, equipmentDefinition{Kind: "module", Quality: "orange"}, "ATK", 1); ok {
		t.Fatal("mainStatValue() accepted a module without grid")
	}
}
