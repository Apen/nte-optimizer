package optimizer

import (
	"path/filepath"
	"testing"
)

func TestLoadProductionShapeCatalog(t *testing.T) {
	path := filepath.Join("..", "..", "data", "game", "equipment", "shapes.json")
	catalog, err := LoadShapeCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Shapes) != 12 {
		t.Fatalf("shape count = %d, want 12", len(catalog.Shapes))
	}
	for id, wantArea := range map[string]int{
		"H_2": 2, "V_2": 2,
		"H_3": 3, "V_3": 3,
		"H_4": 4, "V_4": 4,
		"Trap_4_H": 4, "Trap_4_V": 4,
		"L_3_BL": 3, "L_3_TL": 3, "L_3_TR": 3, "L_3_BR": 3,
	} {
		shape, err := catalog.Shape(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(shape.Cells) != wantArea {
			t.Fatalf("%s area = %d, want %d", id, len(shape.Cells), wantArea)
		}
	}
}

func TestOfficialTrapOrientations(t *testing.T) {
	catalog, err := LoadShapeCatalog(filepath.Join("..", "..", "data", "game", "equipment", "shapes.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Trap_4_H": "1,0;2,0;0,1;1,1;",
		"Trap_4_V": "1,0;0,1;1,1;0,2;",
	}
	for id, key := range want {
		shape, err := catalog.Shape(id)
		if err != nil {
			t.Fatal(err)
		}
		if got := cellsKey(shape.Cells); got != key {
			t.Fatalf("%s cells = %q, want %q", id, got, key)
		}
		if len(Rotations(shape)) != 1 {
			t.Fatalf("%s must keep its official fixed orientation", id)
		}
	}
}
