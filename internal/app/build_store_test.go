package app

import (
	"os"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func TestLocalBuildsPreservePriorityAndReservations(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	state := decoded.State{Characters: []decoded.Character{{CharacterID: 20, Name: "Second"}, {CharacterID: 10, Name: "First"}}}
	if err := writeJSON(workspaceFile(dir, "decoded_state.json"), state); err != nil {
		t.Fatal(err)
	}
	inv := nte.Inventory{Modules: []nte.Module{{LocalID: "m1"}, {LocalID: "m2"}}, Cartridges: []nte.Cartridge{{LocalID: "c1"}}}
	if err := writeJSON(workspaceFile(dir, "inventory.json"), inv); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveCharacterPriority(dir, []int{10, 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveLocalBuild(dir, "first", 10, []string{"m1"}, "c1", map[string]float64{"CritBase": .6}); err != nil {
		t.Fatal(err)
	}
	modules, cartridges, err := HigherPriorityReservations(dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !modules["m1"] || !cartridges["c1"] {
		t.Fatalf("missing reservations: %#v %#v", modules, cartridges)
	}
	modules, _, err = HigherPriorityReservations(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 0 {
		t.Fatalf("rank one should have no reservations: %#v", modules)
	}
	if _, err := os.Stat(workspaceFile(dir, "local_builds.json")); err != nil {
		t.Fatal(err)
	}
}

func TestHigherPriorityReservationsIncludeCurrentEquipmentWithoutSavedBuild(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	state := decoded.State{Characters: []decoded.Character{{CharacterID: 20, Name: "Second"}, {CharacterID: 10, Name: "First"}}}
	if err := writeJSON(workspaceFile(dir, "decoded_state.json"), state); err != nil {
		t.Fatal(err)
	}
	inv := nte.Inventory{
		Modules: []nte.Module{
			{LocalID: "worn-module", EquippedCharacterID: 10},
			{LocalID: "free-module"},
		},
		Cartridges: []nte.Cartridge{
			{LocalID: "worn-cartridge", EquippedCharacterID: 10},
			{LocalID: "free-cartridge"},
		},
	}
	if err := writeJSON(workspaceFile(dir, "inventory.json"), inv); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveCharacterPriority(dir, []int{10, 20}); err != nil {
		t.Fatal(err)
	}

	modules, cartridges, err := HigherPriorityReservations(dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !modules["worn-module"] || !cartridges["worn-cartridge"] {
		t.Fatalf("current equipment of higher-priority character was not reserved: %#v %#v", modules, cartridges)
	}
	if modules["free-module"] || cartridges["free-cartridge"] {
		t.Fatalf("free equipment was incorrectly reserved: %#v %#v", modules, cartridges)
	}
}

func TestCleanCharacterNameRemovesLocaleCategory(t *testing.T) {
	for input, expected := range map[string]string{
		"Personnage·Zankou": "Zankou",
		"Character·Zankou":  "Zankou",
		"Zankou":            "Zankou",
	} {
		if actual := cleanCharacterName(input); actual != expected {
			t.Fatalf("cleanCharacterName(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestLoadBuildWorkspaceAllowsFirstLaunch(t *testing.T) {
	workspace, err := LoadBuildWorkspace(t.TempDir(), t.TempDir(), "fr")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Characters) != 0 {
		t.Fatalf("first-launch workspace contains characters: %#v", workspace)
	}
}

func TestLoadBuildWorkspaceRejectsBuildsWithoutAccountState(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "local_builds.json"), LocalBuildState{SchemaVersion: 1, Builds: map[string]LocalBuild{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBuildWorkspace(dir, t.TempDir(), "fr"); err == nil {
		t.Fatal("local builds without decoded account state were accepted")
	}
}

func TestSavedBuildKeepsCompleteOptimizationResult(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	character := decoded.Character{CharacterID: 1036, Name: "Zankou"}
	if err := writeJSON(workspaceFile(dir, "decoded_state.json"), decoded.State{Characters: []decoded.Character{character}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "inventory.json"), nte.Inventory{Modules: []nte.Module{{LocalID: "m1"}}, Cartridges: []nte.Cartridge{{LocalID: "c1"}}}); err != nil {
		t.Fatal(err)
	}
	result := OptimizationResult{ProfileID: "zankou", Character: &character, Modules: []OptimizationModule{{Module: nte.Module{LocalID: "m1"}}}, Cartridge: &nte.Cartridge{LocalID: "c1"}}
	result.Solution.Score = 12.34
	if _, err := SaveOptimizationResult(dir, result); err != nil {
		t.Fatal(err)
	}
	loaded, err := SavedOptimizationResult(dir, 1036)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProfileID != "zankou" || loaded.Solution.Score != 12.34 || len(loaded.Modules) != 1 {
		t.Fatalf("saved result was not preserved: %#v", loaded)
	}
}
