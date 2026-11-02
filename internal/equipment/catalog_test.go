package equipment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishedEquipmentUsesCanonicalGeometryAndSets(t *testing.T) {
	definitions, err := Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	if definitions["cell3_style6_1_Orange"].Geometry != "L_3_TL" || definitions["Incantation_orange"].SetID != "Suit4" {
		t.Fatal("published equipment contract changed")
	}
}

func TestNewEquipmentDoesNotRequireCodeMappings(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "game", "equipment")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("shapes.json", `{"schema_version":1,"shapes":[{"id":"Novel5","cells":[{"x":0,"y":0},{"x":1,"y":0},{"x":2,"y":0},{"x":3,"y":0},{"x":4,"y":0}]}]}`)
	write("decode.json", `{"items":{"new_module":{"kind":"module","geometry":"runtime_new","quality":"purple","max_level":30}},"shapes":{"runtime_new":{"cells":[{"row":2,"column":3},{"row":2,"column":4},{"row":2,"column":5},{"row":2,"column":6},{"row":2,"column":7}]}}}`)
	definitions, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := definitions["new_module"]; got.Geometry != "Novel5" || got.Area != 5 || got.MaxLevel != 30 {
		t.Fatalf("data-driven definition: %+v", got)
	}
	write("shapes.json", `{"schema_version":1,"shapes":[]}`)
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "equipment/shapes.json") {
		t.Fatalf("missing canonical contract: %v", err)
	}
}
