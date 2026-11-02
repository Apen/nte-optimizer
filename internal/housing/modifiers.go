// Package housing reads and validates extracted special-furniture modifiers.
package housing

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Catalog struct {
	SchemaVersion int        `json:"schema_version"`
	Modifiers     []Modifier `json:"modifiers"`
}

type Modifier struct {
	ProgressionID string            `json:"progression_id,omitempty"`
	Level         int               `json:"level,omitempty"`
	ID            string            `json:"id"`
	Stats         []ModifierStat    `json:"stats"`
	Conditions    []json.RawMessage `json:"conditions"`
}

type ModifierStat struct {
	PropertyID string  `json:"property_id"`
	Value      float64 `json:"value"`
	Percent    bool    `json:"percent"`
	ModifierOp string  `json:"modifier_op"`
}

func Load(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func (c Catalog) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported special-furniture modifier schema version %d", c.SchemaVersion)
	}
	seen := make(map[string]bool, len(c.Modifiers))
	progressions := make(map[string]map[int]bool)
	for _, modifier := range c.Modifiers {
		if modifier.ID == "" || seen[modifier.ID] {
			return fmt.Errorf("missing or duplicate special-furniture modifier id %q", modifier.ID)
		}
		seen[modifier.ID] = true
		if modifier.ProgressionID != "" || modifier.Level != 0 {
			if modifier.ProgressionID == "" || modifier.Level < 1 || modifier.Level > 100 {
				return fmt.Errorf("special-furniture modifier %q has invalid progression metadata", modifier.ID)
			}
			if progressions[modifier.ProgressionID] == nil {
				progressions[modifier.ProgressionID] = make(map[int]bool)
			}
			if progressions[modifier.ProgressionID][modifier.Level] {
				return fmt.Errorf("duplicate special-furniture progression level for %q", modifier.ID)
			}
			progressions[modifier.ProgressionID][modifier.Level] = true
		}
		if len(modifier.Stats) == 0 {
			return fmt.Errorf("special-furniture modifier %q has no stats", modifier.ID)
		}
		for _, stat := range modifier.Stats {
			if stat.PropertyID == "" || math.IsNaN(stat.Value) || math.IsInf(stat.Value, 0) || stat.ModifierOp == "" {
				return fmt.Errorf("special-furniture modifier %q has an invalid stat", modifier.ID)
			}
		}
	}
	return nil
}
