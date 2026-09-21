package optimizer

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Shape struct {
	ID             string  `json:"id"`
	Cells          []Point `json:"cells"`
	AllowRotations bool    `json:"allow_rotations,omitempty"`
}

type ShapeCatalog struct {
	Shapes map[string]Shape
}

type shapeFile struct {
	SchemaVersion int     `json:"schema_version"`
	Shapes        []Shape `json:"shapes"`
}

func LoadShapeCatalog(path string) (ShapeCatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ShapeCatalog{}, fmt.Errorf("read geometry catalog %s: %w", path, err)
	}
	var file shapeFile
	if err := json.Unmarshal(b, &file); err != nil {
		return ShapeCatalog{}, fmt.Errorf("decode geometry catalog %s: %w", path, err)
	}
	if file.SchemaVersion != 1 {
		return ShapeCatalog{}, fmt.Errorf("geometry catalog schema_version must be 1")
	}
	catalog := ShapeCatalog{Shapes: make(map[string]Shape, len(file.Shapes))}
	for _, shape := range file.Shapes {
		shape.Cells = normalizeCells(shape.Cells)
		if err := validateShape(shape); err != nil {
			return ShapeCatalog{}, err
		}
		if _, exists := catalog.Shapes[shape.ID]; exists {
			return ShapeCatalog{}, fmt.Errorf("duplicate geometry %q", shape.ID)
		}
		catalog.Shapes[shape.ID] = shape
	}
	return catalog, nil
}

func (c ShapeCatalog) Shape(id string) (Shape, error) {
	shape, ok := c.Shapes[id]
	if !ok {
		return Shape{}, fmt.Errorf("unknown geometry %q", id)
	}
	return shape, nil
}

func Rotations(shape Shape) []Shape {
	if !shape.AllowRotations {
		return []Shape{{ID: shape.ID, Cells: normalizeCells(shape.Cells)}}
	}
	result := make([]Shape, 0, 4)
	seen := map[string]bool{}
	cells := append([]Point(nil), shape.Cells...)
	for quarterTurn := 0; quarterTurn < 4; quarterTurn++ {
		normalized := normalizeCells(cells)
		key := cellsKey(normalized)
		if !seen[key] {
			seen[key] = true
			result = append(result, Shape{ID: shape.ID, Cells: normalized, AllowRotations: true})
		}
		for i := range cells {
			cells[i] = Point{X: -cells[i].Y, Y: cells[i].X}
		}
	}
	return result
}

func validateShape(shape Shape) error {
	if strings.TrimSpace(shape.ID) == "" {
		return fmt.Errorf("geometry id is required")
	}
	if len(shape.Cells) == 0 {
		return fmt.Errorf("geometry %q has no cells", shape.ID)
	}
	seen := map[Point]bool{}
	for _, cell := range shape.Cells {
		if cell.X < 0 || cell.Y < 0 {
			return fmt.Errorf("geometry %q contains a negative cell", shape.ID)
		}
		if seen[cell] {
			return fmt.Errorf("geometry %q contains duplicate cell %+v", shape.ID, cell)
		}
		seen[cell] = true
	}
	return nil
}

func normalizeCells(cells []Point) []Point {
	if len(cells) == 0 {
		return nil
	}
	minX, minY := cells[0].X, cells[0].Y
	for _, cell := range cells[1:] {
		if cell.X < minX {
			minX = cell.X
		}
		if cell.Y < minY {
			minY = cell.Y
		}
	}
	result := make([]Point, len(cells))
	for i, cell := range cells {
		result[i] = Point{X: cell.X - minX, Y: cell.Y - minY}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Y == result[j].Y {
			return result[i].X < result[j].X
		}
		return result[i].Y < result[j].Y
	})
	return result
}

func cellsKey(cells []Point) string {
	var b strings.Builder
	for _, cell := range cells {
		fmt.Fprintf(&b, "%d,%d;", cell.X, cell.Y)
	}
	return b.String()
}

func shapeSize(shape Shape) (width, height int) {
	for _, cell := range shape.Cells {
		if cell.X+1 > width {
			width = cell.X + 1
		}
		if cell.Y+1 > height {
			height = cell.Y + 1
		}
	}
	return width, height
}
