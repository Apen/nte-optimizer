package app

import (
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

func TestAccountSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	wantInventory := nte.Inventory{SchemaVersion: 1, Modules: []nte.Module{{LocalID: "snapshot-module"}}}
	wantState := decoded.State{SchemaVersion: 2, Characters: []decoded.Character{{CharacterID: 1036, Name: "Zankou"}}}
	if err := writeAccountData(dir, wantInventory, wantState); err != nil {
		t.Fatal(err)
	}

	inventory, state, loaded, err := loadAccountData(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded || len(inventory.Modules) != 1 || inventory.Modules[0].LocalID != "snapshot-module" || len(state.Characters) != 1 || state.Characters[0].CharacterID != 1036 {
		t.Fatalf("unexpected snapshot: loaded=%v inventory=%#v state=%#v", loaded, inventory, state)
	}
}

func TestAccountSnapshotTakesPrecedenceOverLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeAccountData(dir, nte.Inventory{Modules: []nte.Module{{LocalID: "current"}}}, decoded.State{}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "inventory.json"), nte.Inventory{Modules: []nte.Module{{LocalID: "legacy"}}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "decoded_state.json"), decoded.State{}); err != nil {
		t.Fatal(err)
	}

	inventory, _, _, err := loadAccountData(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := inventory.Modules[0].LocalID; got != "current" {
		t.Fatalf("loaded module %q, want current snapshot", got)
	}
}

func TestLoadAccountDataRejectsPartialLegacyPair(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "inventory.json"), nte.Inventory{}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := loadAccountData(dir); err == nil {
		t.Fatal("partial legacy account pair was accepted")
	}
}

func TestLoadAccountDataRejectsUnknownSnapshotVersion(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(workspaceFile(dir, "account_snapshot.json"), accountSnapshot{SchemaVersion: 99}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := loadAccountData(dir); err == nil {
		t.Fatal("unknown account snapshot schema was accepted")
	}
}
