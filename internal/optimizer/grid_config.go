package optimizer

import (
	"encoding/json"
	"fmt"
	"os"
)

type GridDefinition struct {
	ID       string  `json:"id"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Playable []Point `json:"playable"`
	Source   string  `json:"source,omitempty"`
}

type GridCatalog struct {
	Definitions map[string]GridDefinition
}

type gridFile struct {
	SchemaVersion int              `json:"schema_version"`
	Grids         []GridDefinition `json:"grids"`
}

func LoadGridCatalog(path string) (GridCatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return GridCatalog{}, fmt.Errorf("read grid catalog %s: %w", path, err)
	}
	var file gridFile
	if err := json.Unmarshal(b, &file); err != nil {
		return GridCatalog{}, fmt.Errorf("decode grid catalog %s: %w", path, err)
	}
	if file.SchemaVersion != 1 {
		return GridCatalog{}, fmt.Errorf("grid catalog schema_version must be 1")
	}
	catalog := GridCatalog{Definitions: make(map[string]GridDefinition, len(file.Grids))}
	for _, definition := range file.Grids {
		if definition.ID == "" || definition.Width <= 0 || definition.Height <= 0 {
			return GridCatalog{}, fmt.Errorf("invalid grid definition %q", definition.ID)
		}
		if _, exists := catalog.Definitions[definition.ID]; exists {
			return GridCatalog{}, fmt.Errorf("duplicate grid definition %q", definition.ID)
		}
		seen := map[Point]bool{}
		for _, cell := range definition.Playable {
			if cell.X < 0 || cell.X >= definition.Width || cell.Y < 0 || cell.Y >= definition.Height {
				return GridCatalog{}, fmt.Errorf("grid %q playable cell %+v is out of bounds", definition.ID, cell)
			}
			if seen[cell] {
				return GridCatalog{}, fmt.Errorf("grid %q repeats playable cell %+v", definition.ID, cell)
			}
			seen[cell] = true
		}
		catalog.Definitions[definition.ID] = definition
	}
	return catalog, nil
}

func (c GridCatalog) Grid(id string) (Grid, error) {
	definition, ok := c.Definitions[id]
	if !ok {
		return Grid{}, fmt.Errorf("unknown grid %q", id)
	}
	playable := make(map[Point]bool, len(definition.Playable))
	for _, cell := range definition.Playable {
		playable[cell] = true
	}
	blocked := make([]Point, 0, definition.Width*definition.Height-len(playable))
	for y := 0; y < definition.Height; y++ {
		for x := 0; x < definition.Width; x++ {
			cell := Point{X: x, Y: y}
			if !playable[cell] {
				blocked = append(blocked, cell)
			}
		}
	}
	return NewGrid(definition.Width, definition.Height, blocked)
}
