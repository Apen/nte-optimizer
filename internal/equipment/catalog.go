// Package equipment resolves published game definitions without depending on
// scanner protocol parsing or optimizer search code.
package equipment

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/nte"
)

type cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func cellKey(cells []cell) string {
	if len(cells) == 0 {
		return ""
	}
	minX, minY := cells[0].X, cells[0].Y
	for _, c := range cells {
		minX = min(minX, c.X)
		minY = min(minY, c.Y)
	}
	normalized := make([][2]int, len(cells))
	for i, c := range cells {
		normalized[i] = [2]int{c.X - minX, c.Y - minY}
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i][1] != normalized[j][1] {
			return normalized[i][1] < normalized[j][1]
		}
		return normalized[i][0] < normalized[j][0]
	})
	data, _ := json.Marshal(normalized)
	return string(data)
}

func Load(dataDir string) (nte.EquipmentDefinitions, error) {
	layout := datafiles.New(dataDir)
	var shapes struct {
		SchemaVersion int `json:"schema_version"`
		Shapes        []struct {
			ID    string `json:"id"`
			Cells []cell `json:"cells"`
		} `json:"shapes"`
	}
	if err := read(layout.Shapes(), &shapes); err != nil {
		return nil, err
	}
	if shapes.SchemaVersion != 1 {
		return nil, fmt.Errorf("equipment/shapes.json: unsupported schema_version")
	}
	canonical := map[string]string{}
	for _, shape := range shapes.Shapes {
		key := cellKey(shape.Cells)
		if key == "" || shape.ID == "" || canonical[key] != "" {
			return nil, fmt.Errorf("equipment/shapes.json: missing or ambiguous shape %q", shape.ID)
		}
		canonical[key] = shape.ID
	}
	var catalog struct {
		Items map[string]struct {
			Kind     string `json:"kind"`
			Quality  string `json:"quality"`
			Geometry string `json:"geometry"`
			SetID    string `json:"suit"`
			MaxLevel int    `json:"max_level"`
		} `json:"items"`
		Shapes map[string]struct {
			Cells []struct {
				Row    int `json:"row"`
				Column int `json:"column"`
			} `json:"cells"`
		} `json:"shapes"`
	}
	if err := read(layout.DecodeCatalog("equipment.json"), &catalog); err != nil {
		return nil, err
	}
	definitions := nte.EquipmentDefinitions{}
	for id, item := range catalog.Items {
		definition := nte.EquipmentDefinition{Kind: item.Kind, Quality: item.Quality, SetID: item.SetID, MaxLevel: item.MaxLevel}
		if item.MaxLevel <= 0 {
			return nil, fmt.Errorf("equipment/decode.json: missing items[%q].max_level", id)
		}
		switch item.Kind {
		case "module":
			shape, ok := catalog.Shapes[item.Geometry]
			if !ok {
				return nil, fmt.Errorf("equipment/decode.json: missing shapes[%q]", item.Geometry)
			}
			cells := make([]cell, len(shape.Cells))
			for i, c := range shape.Cells {
				cells[i] = cell{c.Column, c.Row}
			}
			definition.Geometry = canonical[cellKey(cells)]
			definition.Area = len(cells)
			if definition.Geometry == "" {
				return nil, fmt.Errorf("equipment/shapes.json: missing canonical geometry for %q", item.Geometry)
			}
		case "core":
			if item.SetID == "" {
				return nil, fmt.Errorf("equipment/decode.json: missing items[%q].suit", id)
			}
		default:
			return nil, fmt.Errorf("equipment/decode.json: unsupported items[%q].kind %q", id, item.Kind)
		}
		definitions[id] = definition
	}
	return definitions, nil
}

func read(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read equipment data: %w", err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
