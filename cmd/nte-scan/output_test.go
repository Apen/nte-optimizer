package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	report := report{Input: "capture.pcapng", UDP: udpReport{
		Characters: []characterItem{{CharacterID: characterID, Name: "zankou", NetID: characterNetID, ForkNetID: &weaponID, PanelStats: &characterPanelStats{ElementalDMGBonus: .1}}},
		Weapons:    []weaponItem{{ID: weaponID, ForkID: "fork_test", EquippedCharacterID: &characterID}},
		Items: []inventoryItem{
			{ID: itemNetID{Solt: 50, Serial: 60}, Kind: "core", CharacterNetID: &characterNetID},
			{ID: itemNetID{Solt: 70, Serial: 80}, Kind: "module", CharacterNetID: &characterNetID},
		},
	}}

	if err := writeOutputDir(dir, report); err != nil {
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
	err := writeOutputDirWithWriter(dir, report{}, func(path string, value any) error {
		writes++
		if writes == 3 {
			return os.ErrPermission
		}
		return writeJSON(path, value)
	})
	if err == nil {
		t.Fatal("staging failure was ignored")
	}
	content, readErr := os.ReadFile(marker)
	if readErr != nil || string(content) != "valid" {
		t.Fatalf("previous generation was not preserved: content=%q err=%v", content, readErr)
	}
}

func TestWriteOutputDirPublishesManifestAfterCompleteGeneration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	written := []string{}
	if err := writeOutputDirWithWriter(dir, report{}, func(path string, value any) error {
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
	err := writeOutputDir(dir, report{UDP: udpReport{Characters: []characterItem{{CharacterID: 1036, NetID: itemNetID{Solt: 1, Serial: 2}, ForkNetID: &missingWeapon}}}})
	if err == nil {
		t.Fatal("broken character-to-weapon relation was accepted")
	}
	if data, readErr := os.ReadFile(marker); readErr != nil || string(data) != "valid" {
		t.Fatalf("previous generation changed after validation failure: %q, %v", data, readErr)
	}
}

func TestWriteOutputDirRejectsInvalidStagedJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scan-output")
	err := writeOutputDirWithWriter(dir, report{}, func(path string, _ any) error {
		return os.WriteFile(path, []byte("not json"), 0o644)
	})
	if err == nil {
		t.Fatal("invalid staged JSON was published")
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("output directory exists after invalid staging: %v", statErr)
	}
}
