package optimizer

import (
	"path/filepath"
	"testing"
)

func TestLoadZankouGrid(t *testing.T) {
	catalog, err := LoadGridCatalog(filepath.Join("..", "..", "data", "game", "equipment", "grids.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Definitions) != 23 {
		t.Fatalf("grid count = %d, want 23", len(catalog.Definitions))
	}
	definition := catalog.Definitions["zankou"]
	if definition.Width != 5 || definition.Height != 5 || len(definition.Playable) != 20 {
		t.Fatalf("unexpected Zankou grid: %#v", definition)
	}
	grid, err := catalog.Grid("zankou")
	if err != nil {
		t.Fatal(err)
	}
	for _, blocked := range []Point{{0, 0}, {3, 2}, {2, 3}, {3, 3}, {4, 4}} {
		if !grid.Occupied(blocked) {
			t.Fatalf("expected blocked cell %+v", blocked)
		}
	}
}
