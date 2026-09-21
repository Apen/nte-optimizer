package app

import (
	"os"
	"path/filepath"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func TestLoadEquipmentCatalogLocalizesEquipment(t *testing.T) {
	projectDir, dataDir := t.TempDir(), t.TempDir()
	if err := ensureWorkspace(projectDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "presentation"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "game", "equipment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "game", "locales"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dataDir, "presentation", "fr.json"), `{"schema":"nte-optimizer-locale","schema_version":1,"locale":"fr"}`)
	writeTestFile(t, filepath.Join(dataDir, "game", "locales", "fr.json"), `{"schema_version":1,"language":"fr","tables":{"characters":{"42":"Personnage·Shinku"},"forks":{"fork_test":"Arc·Test"},"resources":{"Gold":"Pièce coléoptère"},"sets":{"Suit1":"Set localisé"}}}`)
	writeTestFile(t, filepath.Join(dataDir, "game", "equipment", "sets.json"), `{"schema_version":1,"sets":[{"id":"Suit1","inventory_set_id":"set_test","name_fr":"Set français","required_geometries":["H_2"],"bonuses":[]}]}`)
	if err := writeJSON(workspaceFile(projectDir, "inventory.json"), nte.Inventory{
		Modules:    []nte.Module{{LocalID: "m1", SetID: "set_test", SetName: "English set"}},
		Cartridges: []nte.Cartridge{{LocalID: "c1", SetID: "set_test", SetName: "English set"}},
	}); err != nil {
		t.Fatal(err)
	}
	state := decoded.State{
		Characters: []decoded.Character{{CharacterID: 42, Name: "Raw name"}},
		Weapons:    []decoded.Weapon{{ForkID: "fork_test", Name: "Raw arc", EquippedCharacterID: 42}},
		Resources:  []decoded.Resource{{ItemID: "Gold", Name: "Raw resource", Quantity: 123}},
	}
	if err := writeJSON(workspaceFile(projectDir, "decoded_state.json"), state); err != nil {
		t.Fatal(err)
	}

	catalog, err := LoadEquipmentCatalog(projectDir, dataDir, "fr")
	if err != nil {
		t.Fatal(err)
	}
	if got := catalog.Cartridges[0].SetName; got != "Set localisé" {
		t.Fatalf("cartridge set name = %q", got)
	}
	if got := catalog.Modules[0].SetName; got != "Set localisé" {
		t.Fatalf("module set name = %q", got)
	}
	if got := catalog.Arcs[0].Name; got != "Test" {
		t.Fatalf("arc name = %q", got)
	}
	if got := catalog.Arcs[0].EquippedCharacterName; got != "Shinku" {
		t.Fatalf("arc owner = %q", got)
	}
	if got := catalog.Resources[0]; got.Name != "Pièce coléoptère" || got.Quantity != 123 {
		t.Fatalf("unexpected resource: %#v", got)
	}
}

func TestReadInventoryRepairsLegacyLocalizedSetID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inventory.json")
	writeTestFile(t, path, `{"schema_version":1,"modules":[],"cartridges":[{"local_id":"c1","game_item_id":"Incantation_orange","set_id":"ecarlate_:_papillons_jumeaux","set_name":"Crimson","quality":"orange","level":20,"main_stats":[],"sub_stats":[]}]}`)

	inventory, err := readInventory(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := inventory.Cartridges[0].SetID; got != "Suit4" {
		t.Fatalf("normalized set id = %q, want Suit4", got)
	}
}

func TestLoadEquipmentCatalogAllowsEmptyWorkspace(t *testing.T) {
	catalog, err := LoadEquipmentCatalog(t.TempDir(), t.TempDir(), "fr")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Modules == nil || catalog.Cartridges == nil || catalog.Arcs == nil || catalog.Resources == nil {
		t.Fatalf("empty catalog must expose initialized collections: %#v", catalog)
	}
}

func TestLoadEquipmentCatalogRejectsPartialWorkspace(t *testing.T) {
	projectDir := t.TempDir()
	if err := ensureWorkspace(projectDir); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(projectDir, "inventory.json"), nte.Inventory{}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEquipmentCatalog(projectDir, t.TempDir(), "fr"); err == nil {
		t.Fatal("partial workspace was accepted")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
