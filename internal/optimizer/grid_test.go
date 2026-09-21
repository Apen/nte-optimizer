package optimizer

import "testing"

func TestRotationsDeduplicateSymmetricShapes(t *testing.T) {
	line := Shape{ID: "H_3", Cells: []Point{{0, 0}, {1, 0}, {2, 0}}, AllowRotations: true}
	if got := len(Rotations(line)); got != 2 {
		t.Fatalf("line rotations = %d, want 2", got)
	}
	lShape := Shape{ID: "L_3", Cells: []Point{{0, 0}, {1, 0}, {0, 1}}, AllowRotations: true}
	if got := len(Rotations(lShape)); got != 4 {
		t.Fatalf("L rotations = %d, want 4", got)
	}
}

func TestGeneratePlacementsHonorsBoundsAndBlockedCells(t *testing.T) {
	grid, err := NewGrid(3, 2, []Point{{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	shape := Shape{ID: "H_2", Cells: []Point{{0, 0}, {1, 0}}, AllowRotations: true}
	placements := GeneratePlacements(grid, "module_1", shape)
	// Horizontal: two candidates on the bottom row. Vertical: columns 0 and 2.
	if len(placements) != 4 {
		t.Fatalf("placements = %d, want 4: %#v", len(placements), placements)
	}
	for _, placement := range placements {
		if placement.ModuleID != "module_1" || !grid.CanPlace(placement.Cells) {
			t.Fatalf("invalid placement: %#v", placement)
		}
	}
}

func TestGridSupportsMoreThanSixtyFourCells(t *testing.T) {
	grid, err := NewGrid(10, 10, []Point{{9, 9}})
	if err != nil {
		t.Fatal(err)
	}
	if !grid.Occupied(Point{9, 9}) || grid.Occupied(Point{8, 9}) {
		t.Fatal("bitset did not preserve cells above index 63")
	}
	if grid.FreeCells() != 99 {
		t.Fatalf("free cells = %d, want 99", grid.FreeCells())
	}
}

func TestPlaceRejectsCollision(t *testing.T) {
	grid, _ := NewGrid(2, 2, nil)
	if err := grid.Place([]Point{{0, 0}, {1, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := grid.Place([]Point{{1, 0}}); err == nil {
		t.Fatal("expected collision")
	}
}
