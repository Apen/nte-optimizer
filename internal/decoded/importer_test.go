package decoded

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportDirectoryBuildsInventoryAndLinksEquipment(t *testing.T) {
	dir := t.TempDir()
	characterNet := NetID{Slot: 10, Serial: 20}
	weaponNet := NetID{Slot: 30, Serial: 40}
	writeFixture(t, dir, "manifest.json", Manifest{Format: "nte-scan-output", FormatVersion: 1, GeneratedAt: time.Date(2026, 9, 14, 1, 2, 3, 0, time.UTC)})
	writeFixture(t, dir, "characters.json", []Character{{NetID: characterNet, CharacterID: 1036, Name: "Zankou", ForkNetID: &weaponNet}})
	writeFixture(t, dir, "weapons.json", []Weapon{{ID: weaponNet, ForkID: "fork_DemonBlade", Level: 80, Breakthrough: 6, Star: 1}})
	writeFixture(t, dir, "equipment.json", []Equipment{
		{ID: NetID{Slot: 1, Serial: 2}, ItemID: "cell3_style6_1_Orange", Kind: "module", Level: 20, MainStats: []RawStat{{Property: "AtkAdd", Value: 63}}, CharacterNetID: &characterNet},
		{ID: NetID{Slot: 3, Serial: 4}, ItemID: "Incantation_orange", Name: "Crimson", Kind: "core", Level: 20, MainStats: []RawStat{{Property: "CritDamageBase", Value: .6}}, CharacterNetID: &characterNet},
	})
	got, err := ImportDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Inventory.Modules) != 1 || got.Inventory.Modules[0].Geometry != "L_3_TL" || got.Inventory.Modules[0].EquippedCharacterID != 1036 {
		t.Fatalf("unexpected module: %#v", got.Inventory.Modules)
	}
	if len(got.Inventory.Cartridges) != 1 || got.Inventory.Cartridges[0].SetID != "Suit4" || got.AssignedEquipment != 2 {
		t.Fatalf("unexpected import: %#v", got)
	}
}

func TestImportDirectorySupportsVersionThreeCharacterExport(t *testing.T) {
	dir := t.TempDir()
	characterNet := NetID{Slot: 10, Serial: 20}
	weaponNet := NetID{Slot: 30, Serial: 40}
	moduleNet := NetID{Slot: 1, Serial: 2}
	writeFixture(t, dir, "manifest.json", Manifest{Format: "nte-scan-output", FormatVersion: 3, GeneratedAt: time.Date(2026, 9, 16, 7, 54, 38, 0, time.UTC)})
	export := characterExport{Format: "nte-scan-characters", FormatVersion: 2}
	export.Characters = append(export.Characters, struct {
		Identity struct {
			CharacterID int    `json:"characterId"`
			Name        string `json:"name"`
			Codename    string `json:"codename"`
			NetID       NetID  `json:"netId"`
		} `json:"identity"`
		Progression struct {
			Level             int              `json:"level"`
			BreakthroughLevel int              `json:"breakthroughLevel"`
			AwakenLevel       int              `json:"awakenLevel"`
			Skills            []CharacterSkill `json:"skills"`
		} `json:"progression"`
		Stats      CharacterStats      `json:"stats"`
		SavedState CharacterSavedState `json:"savedState"`
		Equipment  struct {
			Weapon  *Weapon     `json:"weapon,omitempty"`
			Cores   []Equipment `json:"cores"`
			Modules []Equipment `json:"modules"`
		} `json:"equipment"`
		ObservedAtLogin ObservedLoginState `json:"observedAtLogin"`
	}{})
	source := &export.Characters[0]
	source.Identity.CharacterID, source.Identity.Name, source.Identity.NetID = 1036, "Zankou", characterNet
	source.Progression.Level, source.Progression.BreakthroughLevel, source.Progression.AwakenLevel = 80, 6, 1
	source.Progression.Skills = []CharacterSkill{{AbilityID: "GA_Zankou_Skill", Category: "Ability.Skill", Level: 8}}
	source.Equipment.Weapon = &Weapon{ID: weaponNet, ForkID: "fork_DemonBlade", Level: 80, Star: 1}
	source.Equipment.Modules = []Equipment{{ID: moduleNet, EquippedPlacement: &EquipmentPlacement{Row: 2, Column: 3}}}
	source.ObservedAtLogin = ObservedLoginState{Loaded: true, ActiveAwakeningLevels: []int{5}}
	writeFixture(t, dir, "character.json", export)
	writeFixture(t, dir, "weapons.json", []Weapon{{ID: weaponNet, ForkID: "fork_DemonBlade", Level: 80, Star: 1}})
	writeFixture(t, dir, "equipment.json", []Equipment{{ID: moduleNet, ItemID: "cell3_style6_1_Orange", Kind: "module", Level: 20, MainStats: []RawStat{{Property: "AtkAdd", Value: 16}}, CharacterNetID: &characterNet}})
	writeFixture(t, dir, "resources.json", resourceExport{Data: []Resource{{ItemID: "CharacterUpMaterial_lv3", Quantity: 218}}})

	got, err := ImportDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	character := got.State.Characters[0]
	if character.AwakenLevel != 5 || character.ReportedAwakenLevel != 1 || len(character.Skills) != 1 || !character.ObservedAtLogin.Loaded || len(got.State.Resources) != 1 {
		t.Fatalf("new character data was not preserved: %#v", character)
	}
	if character.ForkNetID == nil || character.ForkNetID.Key() != weaponNet.Key() || got.State.Weapons[0].EquippedCharacterID != 1036 {
		t.Fatalf("embedded weapon was not linked: %#v %#v", character.ForkNetID, got.State.Weapons[0])
	}
	if got.Inventory.Modules[0].EquippedPlacement == nil || got.Inventory.Modules[0].EquippedPlacement.Row != 2 || got.Inventory.Modules[0].EquippedPlacement.Column != 3 {
		t.Fatalf("module placement was not preserved: %#v", got.Inventory.Modules[0])
	}
}

func writeFixture(t *testing.T, dir, name string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
		t.Fatal(err)
	}
}
