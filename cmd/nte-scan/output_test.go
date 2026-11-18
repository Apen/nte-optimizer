package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/scanlog"
)

func TestWriteOutputDirMergesCharacterExports(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "scan-output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{"characters.json", "character-progression.json", "equipment-loadouts.json"} {
		if err := os.WriteFile(filepath.Join(dir, legacy), []byte("legacy"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	characterNetID := itemNetID{Solt: 10, Serial: 20}
	characterID := uint32(1036)
	weaponID := itemNetID{Solt: 30, Serial: 40}
	trainingRank := uint32(0)
	report := report{Input: "capture.pcapng", UDP: udpReport{
		PukaTrainingRank: &trainingRank,
		Characters: []characterItem{{
			CharacterID: characterID, Name: "zankou", NetID: characterNetID, ForkNetID: &weaponID,
			BondStageFlags:            []bool{true},
			PanelStats:                &characterPanelStats{BaseAttack: 660, ElementalDMGBonus: .1, HasElementalDMGBonus: true},
			ObservedActiveWeaponBuffs: []observedWeaponBuff{{ForkID: "fork_test", Star: 1}},
		}},
		Weapons: []weaponItem{{ID: weaponID, ForkID: "fork_test", Star: 1, EquippedCharacterID: &characterID}},
		Items: []inventoryItem{
			{ID: itemNetID{Solt: 50, Serial: 60}, Kind: "core", CharacterNetID: &characterNetID},
			{ID: itemNetID{Solt: 70, Serial: 80}, Kind: "module", CharacterNetID: &characterNetID},
		},
	}}

	if err := writeOutputDir(dir, &report); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{"characters.json", "character-progression.json", "equipment-loadouts.json"} {
		if _, err := os.Stat(filepath.Join(dir, legacy)); !os.IsNotExist(err) {
			t.Fatalf("legacy export %s was not removed", legacy)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "character.json"))
	if err != nil {
		t.Fatal(err)
	}
	var output characterOutput
	if err := json.Unmarshal(b, &output); err != nil {
		t.Fatal(err)
	}
	if output.Count != 1 || len(output.Characters) != 1 || output.Characters[0].Identity.CharacterID != 1036 {
		t.Fatalf("unexpected merged character output: %#v", output)
	}
	if output.PukaTrainingRank == nil || *output.PukaTrainingRank != 0 {
		t.Fatal("rank zero was lost in the character export")
	}
	for _, field := range []string{"savedState", "healthRatio", "loaded", "activeWeaponBuffs"} {
		if bytes.Contains(b, []byte(`"`+field+`"`)) {
			t.Errorf("obsolete field %q was exported", field)
		}
	}
	if output.Validation["observedWeaponBuffs"] != 1 || output.Validation["matchingObservedWeaponBuffs"] != 1 {
		t.Fatalf("Arc validation was not preserved: %#v", output.Validation)
	}
	character := output.Characters[0]
	if !reflect.DeepEqual(character.Progression.BondStageFlags, []bool{true}) ||
		character.Stats.PanelBase == nil || character.Stats.PanelBase.Attack != 660 {
		t.Fatalf("character diagnostics were not preserved: %#v", character)
	}
	if bonus := output.Characters[0].Stats.ElementalDMGBonus; bonus == nil || *bonus != .1 {
		t.Fatalf("elemental damage bonus was not exported: %#v", bonus)
	}
	equipment := output.Characters[0].Equipment
	if equipment.Weapon == nil || equipment.Weapon.ID != weaponID || len(equipment.Cores) != 1 || len(equipment.Modules) != 1 {
		t.Fatalf("active equipment was not merged into character output: %#v", equipment)
	}
}

func TestWriteOutputDirKeepsPreviousGenerationWhenStagingFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "previous.txt")
	if err := os.WriteFile(marker, []byte("valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	writes := 0
	r := &report{}
	err := writeOutputDirWithWriter(dir, r, func(path string, value any) error {
		writes++
		if writes == 3 {
			return os.ErrPermission
		}
		return writeJSON(path, value)
	})
	if err == nil {
		t.Fatal("staging failure was ignored")
	}
	if events := r.UDP.Diagnostics; len(events) != 3 || events[2].Stage != "export" ||
		events[2].Status != "failed" || events[2].Code != "export_write_failed" {
		t.Fatalf("staging failure was not reported as an export error: %+v", events)
	}
	content, readErr := os.ReadFile(marker)
	if readErr != nil || string(content) != "valid" {
		t.Fatalf("previous generation was not preserved: content=%q err=%v", content, readErr)
	}
}

func TestWriteOutputDirPublishesManifestAfterCompleteGeneration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	written := []string{}
	if err := writeOutputDirWithWriter(dir, &report{}, func(path string, value any) error {
		written = append(written, filepath.Base(path))
		return writeJSON(path, value)
	}); err != nil {
		t.Fatal(err)
	}
	if got := written[len(written)-1]; got != "manifest.json" {
		t.Fatalf("last staged file = %q, want manifest.json", got)
	}
	for _, name := range []string{"character.json", "weapons.json", "equipment.json", "resources.json", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("published file %s: %v", name, err)
		}
	}
}

func TestWriteOutputDirExportsResourcesFromDifferentCatalogGroups(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	resources := []resourceItem{
		{ID: itemNetID{Solt: 21, Serial: 31}, ItemID: "Annulith", Quantity: 4},
		{ID: itemNetID{Solt: 22, Serial: 32}, ItemID: "SyntheticEventToken", Quantity: 9},
		{ID: itemNetID{Solt: 23, Serial: 33}, ItemID: "Furniture_FlowerPot_001", Quantity: 1},
	}
	if err := writeOutputDir(dir, &report{UDP: udpReport{Resources: resources}}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "resources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Count int            `json:"count"`
		Data  []resourceItem `json:"data"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Count != len(resources) || !reflect.DeepEqual(output.Data, resources) {
		t.Fatalf("resource output = %#v, want count=%d data=%#v", output, len(resources), resources)
	}
}

func TestWriteOutputDirRejectsBrokenRelationsBeforePublication(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "previous.txt")
	if err := os.WriteFile(marker, []byte("valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingWeapon := itemNetID{Solt: 7, Serial: 8}
	r := &report{UDP: udpReport{Characters: []characterItem{{CharacterID: 1036, NetID: itemNetID{Solt: 1, Serial: 2}, ForkNetID: &missingWeapon}}}}
	err := writeOutputDir(dir, r)
	if err == nil {
		t.Fatal("broken character-to-weapon relation was accepted")
	}
	if events := r.UDP.Diagnostics; len(events) != 1 || events[0].Stage != "validation" ||
		events[0].Status != "failed" || events[0].Code != "output_relations_invalid" {
		t.Fatalf("broken relation was not reported as a validation error: %+v", events)
	}
	if data, readErr := os.ReadFile(marker); readErr != nil || string(data) != "valid" {
		t.Fatalf("previous generation changed after validation failure: %q, %v", data, readErr)
	}
}

func TestWriteOutputDirImportsEquipmentWithUnknownOwnerAsUnassigned(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "scan-output")
	logPath := scanlog.Path(root)
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	knownOwner := itemNetID{Solt: 10, Serial: 20}
	unknownOwner := itemNetID{Solt: 30, Serial: 40}
	placement := equipmentGridCell{Row: 1, Column: 2}
	module := inventoryItem{
		ID: itemNetID{Solt: 1, Serial: 2}, ItemID: "cell3_style6_1_Orange", Kind: "module", Level: 20,
		MainStats: []inventoryStat{{Property: "AtkAdd", Value: 63}},
		Locked:    true, CharacterNetID: &unknownOwner, EquippedPlacement: &placement,
	}
	core := inventoryItem{
		ID: itemNetID{Solt: 3, Serial: 4}, ItemID: "Incantation_orange", Kind: "core", Level: 20,
		MainStats: []inventoryStat{{Property: "CritDamageBase", Value: .6}}, CharacterNetID: &unknownOwner,
	}
	knownModule := module
	knownModule.ID = itemNetID{Solt: 5, Serial: 6}
	knownModule.CharacterNetID = &knownOwner
	r := &report{UDP: udpReport{
		writeDiagnostic: func(event scanlog.Event) error { return scanlog.AppendEvent(logPath, event) },
		Characters:      []characterItem{{CharacterID: 1036, NetID: knownOwner}},
		Items:           []inventoryItem{module, core, knownModule},
	}}
	if err := writeOutputDir(dir, r); err != nil {
		t.Fatal(err)
	}
	if want := []inventoryItem{module, core, knownModule}; !reflect.DeepEqual(r.UDP.Items, want) {
		t.Fatalf("export modified the decoded equipment: %+v", r.UDP.Items)
	}

	events, err := scanlog.Read(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Stage != "validation" || events[0].Status != "partial" ||
		events[0].Code != "equipment_owner_unresolved" || events[0].Counts["unresolved_equipment_owner_links"] != 2 ||
		events[1].Stage != "export" || events[1].Status != "started" ||
		events[2].Stage != "export" || events[2].Status != "complete" {
		t.Fatalf("expected a validation warning followed by a successful export: %+v", events)
	}

	b, err := os.ReadFile(filepath.Join(dir, "equipment.json"))
	if err != nil {
		t.Fatal(err)
	}
	var equipment []inventoryItem
	if err := json.Unmarshal(b, &equipment); err != nil {
		t.Fatal(err)
	}
	module.CharacterNetID, module.EquippedPlacement = nil, nil
	core.CharacterNetID = nil
	if want := []inventoryItem{module, core, knownModule}; !reflect.DeepEqual(equipment, want) {
		t.Fatalf("exported equipment differs from the expected assignments: %+v", equipment)
	}

	imported, err := decoded.ImportDirectory(dir, filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	modules, cores := imported.Inventory.Modules, imported.Inventory.Cartridges
	if imported.AssignedEquipment != 1 || len(modules) != 2 || len(cores) != 1 ||
		modules[0].EquippedCharacterID != 0 || modules[0].EquippedPlacement != nil || !modules[0].Locked ||
		modules[1].EquippedCharacterID != 1036 || modules[1].EquippedPlacement == nil ||
		cores[0].EquippedCharacterID != 0 {
		t.Fatalf("unexpected imported inventory: %+v", imported.Inventory)
	}
}

func TestWriteOutputDirRejectsInvalidStagedJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	err := writeOutputDirWithWriter(dir, &report{}, func(path string, _ any) error {
		return os.WriteFile(path, []byte("not json"), 0o644)
	})
	if err == nil {
		t.Fatal("invalid staged JSON was published")
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("output directory exists after invalid staging: %v", statErr)
	}
}
