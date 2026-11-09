package decoded

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportDiagnosticsReportSafeFailurePhase(t *testing.T) {
	for _, tc := range []struct {
		name, file, content, stage, code string
	}{
		{"malformed equipment", "equipment.json", `[{"private-user@example.test":`, "import_equipment_file", "import_json_invalid"},
		{"wrong Arc structure", "weapons.json", `{"private-user@example.test":1}`, "import_arcs_file", "import_json_type_invalid"},
		{"unsupported version", "manifest.json", `{"format":"private-user@example.test","formatVersion":99}`, "import_contract", "manifest_unsupported"},
		{"duplicate character", "characters.json", `[{"characterId":1},{"characterId":1}]`, "import_character_ids", "character_id_invalid"},
		{"unknown equipment", "equipment.json", `[{"id":{"solt":1,"serial":2},"itemId":"private-user@example.test","kind":"module"}]`, "import_equipment_validation", "equipment_catalog_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, "manifest.json", Manifest{Format: "nte-scan-output", FormatVersion: 1})
			writeFixture(t, dir, "characters.json", []Character{})
			writeFixture(t, dir, "weapons.json", []Weapon{})
			writeFixture(t, dir, "equipment.json", []Equipment{})
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := ImportDirectory(dir, filepath.Join("..", "..", "data"))
			if err == nil {
				t.Fatal("invalid import accepted")
			}
			if len(result.Diagnostics) == 0 {
				t.Fatal("failure lost its diagnostics")
			}
			last := result.Diagnostics[len(result.Diagnostics)-1]
			if last.Stage != tc.stage || last.Status != "failed" || last.Code != tc.code {
				t.Fatalf("unexpected failure diagnostic: %+v", last)
			}
			encoded, marshalErr := json.Marshal(result.Diagnostics)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if strings.Contains(string(encoded), "private-user") || strings.Contains(string(encoded), dir) {
				t.Fatal("import values or paths leaked into diagnostics")
			}
			if len(result.State.Characters) != 0 || len(result.Inventory.Modules) != 0 {
				t.Fatal("failed import returned partial account data")
			}
		})
	}
}

func TestImportDiagnosticsDoNotHideUnresolvedLinks(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "manifest.json", Manifest{Format: "nte-scan-output", FormatVersion: 1})
	writeFixture(t, dir, "characters.json", []Character{{CharacterID: 1, ForkNetID: &NetID{Slot: 1, Serial: 2}}})
	writeFixture(t, dir, "weapons.json", []Weapon{})
	writeFixture(t, dir, "equipment.json", []Equipment{})
	result, err := ImportDirectory(dir, filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range result.Diagnostics {
		if event.Stage == "import_links" && event.Status == "partial" && event.Counts["unresolved_links"] == 1 {
			return
		}
	}
	t.Fatalf("unresolved link not reported: %+v", result.Diagnostics)
}
