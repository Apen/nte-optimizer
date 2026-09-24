package main

import (
	"math"
	"reflect"
	"testing"
)

func TestAttachObservedCharacterState(t *testing.T) {
	characters := []characterItem{
		{CharacterID: 1003},
		{CharacterID: 1036},
		{CharacterID: 1075},
	}
	exports := []string{
		"/Game/Blueprints/Character/Player/Player_003_Sagiri",
		"/Game/Blueprints/Abilities/Buff/Equipment/Equipment_015/Buff_Equipment_GetEfficiency2_4",
		"/Game/Blueprints/Abilities/Buff/Fork/Fork_mofeikesi/Buff_Fork_mofeikesi_1_1",
		"/Game/Blueprints/Character/Player/player_036_zankou",
		"/Game/Blueprints/Abilities/Buff/Equipment/Equipment_006/Buff_Equipment_Incantation_4",
		"/Game/Blueprints/Abilities/Buff/Fork/Fork_DemonBlade/Buff_Fork_DemonBlade_Lv1",
		"/Game/Blueprints/Abilities/Player/Ability_036_Zankou/Upgrade/Level5/Buff_Zankou_Level5",
		"/Game/Blueprints/Abilities/Player/Ability_036_Zankou/Upgrade/Level5/Buff_Zankou_Lv5UnbalUp",
		"/Game/Blueprints/Abilities/Player/Ability_003_Sagiri/Upgrade/Level4/Buff_Sagiri003_Level4",
	}

	attachObservedActiveAwakenings(characters, exports, nil)

	if !characters[0].ObservedLoadedAtLogin || !characters[1].ObservedLoadedAtLogin {
		t.Fatalf("expected Sagiri and Zankou to be observed as loaded: %#v", characters)
	}
	if characters[2].ObservedLoadedAtLogin {
		t.Fatalf("Oneiroi must not be inferred without a player export")
	}
	if !reflect.DeepEqual(characters[0].ObservedActiveAwakeningLevels, []int32{4}) {
		t.Fatalf("unexpected Sagiri awakening effects: %v", characters[0].ObservedActiveAwakeningLevels)
	}
	if !reflect.DeepEqual(characters[1].ObservedActiveAwakeningLevels, []int32{5}) {
		t.Fatalf("duplicate exports must collapse to one effect: %v", characters[1].ObservedActiveAwakeningLevels)
	}
	if !reflect.DeepEqual(characters[1].ObservedActiveAwakeningBuffs, []string{"Buff_Zankou_Level5", "Buff_Zankou_Lv5UnbalUp"}) {
		t.Fatalf("unexpected Zankou awakening buffs: %v", characters[1].ObservedActiveAwakeningBuffs)
	}
	if !reflect.DeepEqual(characters[0].ObservedActiveEquipmentBuffs, []observedEquipmentBuff{{EquipmentID: 15, Buff: "GetEfficiency2_4"}}) {
		t.Fatalf("unexpected Sagiri equipment buffs: %v", characters[0].ObservedActiveEquipmentBuffs)
	}
	if !reflect.DeepEqual(characters[1].ObservedActiveEquipmentBuffs, []observedEquipmentBuff{{EquipmentID: 6, Buff: "Incantation_4"}}) {
		t.Fatalf("unexpected Zankou equipment buffs: %v", characters[1].ObservedActiveEquipmentBuffs)
	}
	if !reflect.DeepEqual(characters[0].ObservedActiveWeaponBuffs, []observedWeaponBuff{{ForkID: "fork_mofeikesi", Buff: "Buff_Fork_mofeikesi_1_1", Star: 1}}) {
		t.Fatalf("unexpected Sagiri weapon buffs: %v", characters[0].ObservedActiveWeaponBuffs)
	}
	if !reflect.DeepEqual(characters[1].ObservedActiveWeaponBuffs, []observedWeaponBuff{{ForkID: "fork_demonblade", Buff: "Buff_Fork_DemonBlade_Lv1", Star: 1}}) {
		t.Fatalf("unexpected Zankou weapon buffs: %v", characters[1].ObservedActiveWeaponBuffs)
	}
}

func TestParseCharacterPanelStats(t *testing.T) {
	data := make([]byte, 512)
	putFloat := func(off int, value float32) {
		raw := math.Float32bits(value)
		for bit := 0; bit < 32; bit++ {
			if raw&(1<<uint(bit)) != 0 {
				data[(off+bit)/8] |= 1 << uint((off+bit)%8)
			}
		}
	}
	baseOff := 400
	// Paired layout: HP, cycle, ATK, CRIT, CRIT DMG, charge,
	// auxiliary value, universal DMG, auxiliary value, DEF, DEF%, DEF flat.
	values := []float32{15514, 15514, 100, 100, 1230, 1230, .59, .75, 2.364, 2.364, 1, 1, 120, 120, .16, .16, .1, .1, 909, 909, .0525, .0525, 41, 41}
	for i, value := range values {
		putFloat(baseOff+i*40, value)
	}
	totalOff := 2500
	for i, value := range []float32{25222.875, 25222.875, 25222.875, 1702, 200} {
		putFloat(totalOff+i*40, value)
	}
	stats, ok := parseCharacterPanelStats(data, len(data)*8, 25222.875)
	if !ok {
		t.Fatal("panel stats were not decoded")
	}
	if stats.BaseHP != 15514 || stats.BaseAttack != 1230 || stats.BaseDefense != 909 || stats.Attack != 1702 || stats.Defense != 997 || stats.CritRate != .75 || stats.CritDamage != 2.364 || stats.CycleIntensity != 100 || stats.Endurance != 200 {
		t.Fatalf("unexpected panel stats: %#v", stats)
	}
}

func TestParseCharacterPanelStatsWithChangedCycleIntensity(t *testing.T) {
	data := make([]byte, 512)
	putFloat := func(off int, value float32) {
		raw := math.Float32bits(value)
		for bit := 0; bit < 32; bit++ {
			if raw&(1<<uint(bit)) != 0 {
				data[(off+bit)/8] |= 1 << uint((off+bit)%8)
			}
		}
	}
	baseOff := 400
	values := []float32{15514, 15514, 72, 172, 1230, 1230, .57, .73, 2.304, 2.304, 1, 1, 120, 120, .2, .2, .1, .1, 909, 909, .07, .07, 33, 33}
	for i, value := range values {
		putFloat(baseOff+i*40, value)
	}
	totalOff := 2500
	for i, value := range []float32{23471.475, 23471.475, 23471.475, 1748.125, 200} {
		putFloat(totalOff+i*40, value)
	}
	stats, ok := parseCharacterPanelStats(data, len(data)*8, 23471.475)
	if !ok {
		t.Fatal("panel stats with changed cycle intensity were not decoded")
	}
	if stats.BaseHP != 15514 || stats.BaseAttack != 1230 || stats.BaseDefense != 909 || stats.Attack != 1748.125 || stats.Defense != 1005 || stats.CycleIntensity != 172 || stats.CritRate != .73 || stats.UniversalDMGBonus != .2 || stats.ElementalDMGBonus != .1 {
		t.Fatalf("unexpected panel stats: %#v", stats)
	}
}

func TestParseCharacterPanelStatsWithSingleCycleIntensity(t *testing.T) {
	data := make([]byte, 512)
	putFloat := func(off int, value float32) {
		raw := math.Float32bits(value)
		for bit := 0; bit < 32; bit++ {
			if raw&(1<<uint(bit)) != 0 {
				data[(off+bit)/8] |= 1 << uint((off+bit)%8)
			}
		}
	}
	baseOff := 400
	values := []float32{15514, 15514, 172, 1230, 1230, .57, .73, 2.304, 2.304, 1, 1, 120, 120, .2, .2, .1, .1, 909, 909, .07, .07, 33, 33}
	for i, value := range values {
		putFloat(baseOff+i*40, value)
	}
	totalOff := 2500
	for i, value := range []float32{23471.475, 23471.475, 23471.475, 1748.125, 200} {
		putFloat(totalOff+i*40, value)
	}
	stats, ok := parseCharacterPanelStats(data, len(data)*8, 23471.475)
	if !ok {
		t.Fatal("panel stats with a single cycle intensity value were not decoded")
	}
	if stats.BaseAttack != 1230 || stats.CycleIntensity != 172 || stats.Defense != 1005 {
		t.Fatalf("unexpected panel stats: %#v", stats)
	}
}

func TestEnrichObservedEquipmentBuff(t *testing.T) {
	catalog := &equipmentCatalog{
		Items: map[string]equipmentDefinition{
			"GetEfficiency_orange": {Kind: "core", Suit: "Suit11"},
		},
		Suits: map[string]equipmentSuit{
			"Suit11": {
				NameEN:  "Speedy Hedgehog",
				Effects: []equipmentSuitEffect{{Count: 4, TextEN: "Team ATK +15%."}},
			},
		},
	}
	buff := observedEquipmentBuff{EquipmentID: 15, Buff: "GetEfficiency2_4"}

	enrichObservedEquipmentBuff(&buff, catalog)

	want := observedEquipmentBuff{
		EquipmentID: 15,
		Buff:        "GetEfficiency2_4",
		SetID:       "Suit11",
		SetName:     "Speedy Hedgehog",
		Pieces:      4,
		Effect:      "Team ATK +15%.",
	}
	if buff != want {
		t.Fatalf("unexpected enriched buff: %#v", buff)
	}
}
