package optimizer

import (
	"fmt"
	"math/bits"
)

type Grid struct {
	Width  int
	Height int
	bits   []uint64
}

func (g Grid) Clone() Grid {
	clone := g
	clone.bits = append([]uint64(nil), g.bits...)
	return clone
}

func (g Grid) FreeCells() int {
	occupied := 0
	for _, word := range g.bits {
		occupied += bits.OnesCount64(word)
	}
	return g.Width*g.Height - occupied
}

func NewGrid(width, height int, blocked []Point) (Grid, error) {
	if width <= 0 || height <= 0 {
		return Grid{}, fmt.Errorf("grid dimensions must be positive")
	}
	grid := Grid{Width: width, Height: height, bits: make([]uint64, (width*height+63)/64)}
	for _, cell := range blocked {
		if !grid.InBounds(cell) {
			return Grid{}, fmt.Errorf("blocked cell %+v is outside %dx%d grid", cell, width, height)
		}
		grid.set(cell, true)
	}
	return grid, nil
}

func (g Grid) InBounds(cell Point) bool {
	return cell.X >= 0 && cell.X < g.Width && cell.Y >= 0 && cell.Y < g.Height
}

func (g Grid) Occupied(cell Point) bool {
	if !g.InBounds(cell) {
		return true
	}
	index := cell.Y*g.Width + cell.X
	return g.bits[index/64]&(uint64(1)<<uint(index%64)) != 0
}

func (g *Grid) Place(cells []Point) error {
	if !g.CanPlace(cells) {
		return fmt.Errorf("placement collides with occupied or out-of-bounds cells")
	}
	for _, cell := range cells {
		g.set(cell, true)
	}
	return nil
}

func (g *Grid) Remove(cells []Point) {
	for _, cell := range cells {
		if g.InBounds(cell) {
			g.set(cell, false)
		}
	}
}

func (g Grid) CanPlace(cells []Point) bool {
	for _, cell := range cells {
		if g.Occupied(cell) {
			return false
		}
	}
	return true
}

func (g *Grid) set(cell Point, occupied bool) {
	index := cell.Y*g.Width + cell.X
	mask := uint64(1) << uint(index%64)
	if occupied {
		g.bits[index/64] |= mask
	} else {
		g.bits[index/64] &^= mask
	}
}

func GeneratePlacements(grid Grid, moduleID string, shape Shape) []Placement {
	rotations := Rotations(shape)
	placements := make([]Placement, 0)
	for rotation, rotated := range rotations {
		width, height := shapeSize(rotated)
		for y := 0; y <= grid.Height-height; y++ {
			for x := 0; x <= grid.Width-width; x++ {
				cells := make([]Point, len(rotated.Cells))
				for i, cell := range rotated.Cells {
					cells[i] = Point{X: x + cell.X, Y: y + cell.Y}
				}
				if grid.CanPlace(cells) {
					placements = append(placements, Placement{ModuleID: moduleID, X: x, Y: y, Rotation: rotation * 90, Cells: cells})
				}
			}
		}
	}
	return placements
}
