package main

import (
	"math"
	"reflect"
	"testing"
)

// recordWriter packs fields the way the game serializes inventory records:
// least-significant bit first.
type recordWriter struct {
	data []byte
	bits int
}

func (w *recordWriter) bit(value uint64) {
	if len(w.data)*8 == w.bits {
		w.data = append(w.data, 0)
	}
	if value&1 != 0 {
		w.data[w.bits/8] |= 1 << uint(w.bits%8)
	}
	w.bits++
}

func (w *recordWriter) put(value uint64, count int) {
	for i := 0; i < count; i++ {
		w.bit(value >> uint(i))
	}
}

func (w *recordWriter) putString(value string) {
	w.put(0, 1) // soft reference
	w.put(uint64(len(value)+1), 32)
	for _, c := range []byte(value) {
		w.put(uint64(c), 8)
	}
	w.put(0, 8) // NUL terminator
	w.put(0, 32)
}

func (w *recordWriter) putNetID(id itemNetID) {
	w.put(uint64(id.Solt), 32)
	w.put(uint64(id.Serial), 32)
}

func (w *recordWriter) putFloat(value float32) {
	w.put(uint64(math.Float32bits(value)), 32)
}

func TestParseRecordLayoutWithAdditionalHeaderField(t *testing.T) {
	// Current inventory records include a 64-bit field after the creation
	// timestamp. Verify that all three record types decode with this layout.
	grid := uint32(1)
	catalog := &equipmentCatalog{
		Items: map[string]equipmentDefinition{
			"cell4_style1_1_Orange": {Kind: "module", Quality: "orange", Grid: &grid, MaxLevel: 20, MainCount: 1, SubCount: 2},
		},
		Attributes: map[string]struct{}{"AtkAdd": {}, "CritBase": {}},
		Curves: map[string][][2]float32{
			"AtkAdd_1_ITEM_QUALITY_ORANGE": {{0, 42}},
		},
	}

	item := &recordWriter{}
	item.putString("cell4_style1_1_Orange")
	item.putNetID(itemNetID{Solt: 11, Serial: 22})
	item.put(1, 64)   // quantity
	item.put(0, 32)   // unknown int32
	item.put(500, 64) // creation timestamp
	item.put(0, 64)   // appended header field
	item.put(0, 16)
	item.put(0, 16)
	item.put(1, 16)
	item.put(7, 32) // level
	item.putNetID(itemNetID{})
	item.put(0, 32) // durability
	item.put(0, 1)  // locked
	item.put(0, 1)  // discarded
	item.put(0, 16)
	item.put(1, 16) // main stat count
	item.putString("AtkAdd")
	item.put(2, 16) // sub stat count
	item.putString("CritBase")
	item.putFloat(0.04)
	item.putString("AtkAdd")
	item.putFloat(64)
	item.putFloat(0) // timer
	item.put(0, 1)   // unknown flag
	item.put(0, 32)  // tail
	item.put(0, 32)  // trailing raw fstring (empty)
	decoded := parseInventoryItems(item.data, item.bits, catalog)
	if len(decoded) != 1 {
		t.Fatalf("expected the appended 64-bit header field to keep equipment decodable, got %d items", len(decoded))
	}
	want := inventoryStat{Property: "AtkAdd", Value: 42}
	if !reflect.DeepEqual(decoded[0].MainStats, []inventoryStat{want}) {
		t.Fatalf("unexpected main stats: %#v", decoded[0].MainStats)
	}
	if decoded[0].Level != 7 {
		t.Fatalf("unexpected level: %d", decoded[0].Level)
	}

	weapon := &recordWriter{}
	weapon.putString("fork_test")
	weapon.putNetID(itemNetID{Solt: 33, Serial: 44})
	weapon.put(1, 64)
	weapon.put(0, 32)
	weapon.put(500, 64)
	weapon.put(0, 64) // appended header field
	weapon.put(0, 16)
	weapon.put(1, 16)
	weapon.put(40, 32) // level
	weapon.put(2, 32)  // breakthrough
	weapon.put(1, 32)  // star
	forkCatalog := &forkCatalog{Forks: map[string]forkDefinition{
		"fork_test": {Quality: "orange", MaxBreakthrough: 5, MaxStar: 6},
	}}
	weapons := parseWeapons(weapon.data, weapon.bits, forkCatalog)
	if len(weapons) != 1 {
		t.Fatalf("expected the appended 64-bit header field to keep Arcs decodable, got %d weapons", len(weapons))
	}
	if weapons[0].Level != 40 || weapons[0].Breakthrough != 2 || weapons[0].Star != 1 {
		t.Fatalf("unexpected weapon fields: %#v", weapons[0])
	}

	character := &recordWriter{}
	character.putString("1003")
	character.putNetID(itemNetID{Solt: 55, Serial: 66})
	character.put(1, 64)
	character.put(0, 32)
	character.put(500, 64)
	character.put(0, 64) // appended header field
	character.put(1, 16)
	character.put(70, 32) // level
	character.put(5, 32)  // breakthrough
	character.putNetID(itemNetID{})
	character.put(2, 32) // awaken level
	for i := 0; i < 7; i++ {
		character.putFloat(float32(i))
	}
	character.put(0, 1) // unknown flag
	character.put(1, 32)
	character.put(1, 16) // skill count
	character.putString("Skill_A")
	character.putString("Melee")
	character.put(6, 32)
	characterCatalog := &characterCatalog{Characters: map[string]characterDefinition{
		"1003": {Codename: "Sagiri"},
	}}
	characters := parseCharacters(character.data, character.bits, characterCatalog)
	if len(characters) != 1 {
		t.Fatalf("expected the appended 64-bit header field to keep characters decodable, got %d characters", len(characters))
	}
	if characters[0].Level != 70 || characters[0].BreakthroughLevel != 5 || characters[0].AwakenLevel != 2 {
		t.Fatalf("unexpected character fields: %#v", characters[0])
	}
	if len(characters[0].SkillLevels) != 1 || characters[0].SkillLevels[0].Level != 6 {
		t.Fatalf("unexpected character skills: %#v", characters[0].SkillLevels)
	}
}
