package main

import (
	"math"
	"testing"
)

func putPanelTestHandle(data []byte, off int, value byte) {
	for bit := 0; bit < 8; bit++ {
		mask := byte(1 << uint((off+bit)%8))
		data[(off+bit)/8] &= ^mask
		if value&(1<<uint(bit)) != 0 {
			data[(off+bit)/8] |= mask
		}
	}
}

func syntheticPanelFields(fields []struct {
	handle byte
	value  float32
}) []byte {
	data := make([]byte, 512)
	putFloat := func(off int, value float32) {
		raw := math.Float32bits(value)
		for bit := 0; bit < 32; bit++ {
			if raw&(1<<uint(bit)) != 0 {
				data[(off+bit)/8] |= 1 << uint((off+bit)%8)
			}
		}
	}
	for i, field := range fields {
		off := 400 + i*40
		putPanelTestHandle(data, off-8, field.handle)
		putFloat(off, field.value)
	}
	for i, value := range []float32{16000, 16000, 16000, 1500, 200} {
		putFloat(2500+i*40, value)
	}
	return data
}

func TestPanelFieldsAllowOmittedCycleAndIndependentCurrentValues(t *testing.T) {
	fields := []struct {
		handle byte
		value  float32
	}{
		{2, 10000}, {4, 10000}, {42, 500}, {44, 500}, {46, .2}, {48, .6},
		{62, .8}, {64, .8}, {70, 1.2}, {72, 1.2}, {78, 100}, {80, 100},
		{84, .18}, {130, .1}, {132, .3}, {194, 600}, {196, 600}, {198, .1}, {200, .1}, {202, 20}, {204, 20},
	}
	data := syntheticPanelFields(fields)
	got, ok := parseCharacterPanelStats(data, len(data)*8, 16000)
	if !ok {
		t.Fatal("valid sparse panel was not decoded")
	}
	if got.BaseAttack != 500 || got.CritRate != .6 || got.UniversalDMGBonus != .18 || got.Defense != 680 || got.CycleIntensity != 0 || got.HasCycleIntensity || !got.HasElementalDMGBonus || got.ElementalDMGBonus != .3 || got.Attack != 1500 {
		t.Fatalf("incorrect field association: %#v", got)
	}
	exported := exportCharacters([]characterItem{{CharacterID: 1004, PanelStats: &got}}, nil)
	if exported[0].Stats.CycleIntensity != nil {
		t.Fatal("omitted cycle intensity was exported as an observed zero")
	}
}

func TestPanelFieldsDoNotChooseBetweenDifferentElementalBonuses(t *testing.T) {
	fields := []struct {
		handle byte
		value  float32
	}{
		{2, 10000}, {4, 10000}, {30, 60}, {32, 160}, {42, 500}, {44, 500}, {46, .2}, {48, .2},
		{62, .8}, {64, .8}, {70, 1}, {72, 1}, {82, .2}, {84, .2}, {106, .1}, {108, .1}, {140, .15}, {194, 600}, {196, 600},
	}
	data := syntheticPanelFields(fields)
	got, ok := parseCharacterPanelStats(data, len(data)*8, 16000)
	if !ok || got.CycleIntensity != 160 || got.HasElementalDMGBonus {
		t.Fatalf("ambiguous elements were collapsed or cycle was lost: %#v, %v", got, ok)
	}
	exported := exportCharacters([]characterItem{{CharacterID: 1004, PanelStats: &got}}, nil)
	if exported[0].Stats.ElementalDMGBonus != nil {
		t.Fatal("ambiguous elemental bonus was exported as a known value")
	}
}

func TestPanelFieldsRejectMalformedOrMismatchedData(t *testing.T) {
	valid := []struct {
		handle byte
		value  float32
	}{{2, 10000}, {4, 10000}, {42, 500}, {44, 500}, {48, .2}, {64, .8}, {72, 1}, {84, .2}, {194, 600}, {196, 600}}
	for _, test := range []struct {
		name   string
		change func([]byte)
	}{
		{"duplicate handle", func(data []byte) { putPanelTestHandle(data, 400+4*40-8, 44) }},
		{"unknown required handle", func(data []byte) { putPanelTestHandle(data, 400+4*40-8, 50) }},
		{"non-finite crit", func(data []byte) {
			off := 400 + 4*40
			raw := math.Float32bits(float32(math.NaN()))
			for bit := 0; bit < 32; bit++ {
				data[(off+bit)/8] &= ^(1 << uint((off+bit)%8))
				if raw&(1<<uint(bit)) != 0 {
					data[(off+bit)/8] |= 1 << uint((off+bit)%8)
				}
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := syntheticPanelFields(valid)
			test.change(data)
			if _, ok := parseCharacterPanelStats(data, len(data)*8, 16000); ok {
				t.Fatal("malformed panel accepted")
			}
		})
	}
	data := syntheticPanelFields(valid)
	if _, ok := parseCharacterPanelStats(data, len(data)*8, 17000); ok {
		t.Fatal("panel assigned to a different HP anchor")
	}
	if _, ok := parseCharacterPanelStats(data, 800, 16000); ok {
		t.Fatal("truncated panel accepted")
	}
	if _, ok := parseCharacterPanelStats(data, len(data)*8+1, 16000); ok {
		t.Fatal("invalid bit length accepted")
	}
}
