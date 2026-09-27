// Package housing reads extracted special-furniture modifiers and cautiously
// matches them against residuals in observed character panels.
package housing

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type Catalog struct {
	SchemaVersion int        `json:"schema_version"`
	Modifiers     []Modifier `json:"modifiers"`
}

type Modifier struct {
	ID         string            `json:"id"`
	Stats      []ModifierStat    `json:"stats"`
	Conditions []json.RawMessage `json:"conditions"`
}

type ModifierStat struct {
	PropertyID string  `json:"property_id"`
	Value      float64 `json:"value"`
	Percent    bool    `json:"percent"`
	ModifierOp string  `json:"modifier_op"`
}

// Observation contains a final panel value and its independently calculated
// optimizer value for one statistic on one character loaded during login.
type Observation struct {
	CharacterID int
	Loaded      bool
	Panel       map[string]float64
	Calculated  map[string]float64
}

// InferredModifier is a candidate supported by repeated panel residuals. It
// is diagnostic only and must not be applied to optimizer calculations.
type InferredModifier struct {
	ModifierID         string  `json:"modifier_id"`
	PropertyID         string  `json:"property_id"`
	PanelProperty      string  `json:"panel_property"`
	Value              float64 `json:"value"`
	Percent            bool    `json:"percent"`
	MatchingCharacters int     `json:"matching_characters"`
	ObservedCharacters int     `json:"observed_characters"`
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
	for _, modifier := range c.Modifiers {
		if modifier.ID == "" || seen[modifier.ID] {
			return fmt.Errorf("missing or duplicate special-furniture modifier id %q", modifier.ID)
		}
		seen[modifier.ID] = true
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

type candidate struct {
	modifier Modifier
	stat     ModifierStat
	panelKey string
	matches  int
}

// InferShared reports only additive, unconditional modifiers whose values
// match panel residuals across most login-loaded characters. This is a hint,
// not packet-level identification of the active furniture state.
func InferShared(catalog Catalog, observations []Observation) []InferredModifier {
	panelKeys := map[string]string{
		"AtkAdd":         "AtkFinal",
		"DefAdd":         "DefFinal",
		"CritDamageBase": "CritDamageBase",
	}
	eligible := map[string]int{}
	for _, observation := range observations {
		if !observation.Loaded {
			continue
		}
		for _, panelKey := range panelKeys {
			if _, panelOK := observation.Panel[panelKey]; !panelOK {
				continue
			}
			if _, calculatedOK := observation.Calculated[panelKey]; calculatedOK {
				eligible[panelKey]++
			}
		}
	}

	byProperty := map[string][]candidate{}
	for _, modifier := range catalog.Modifiers {
		if len(modifier.Conditions) != 0 || len(modifier.Stats) != 1 {
			continue
		}
		stat := modifier.Stats[0]
		if stat.ModifierOp != "MODIFY_MODOP_ADDITIVE" {
			continue
		}
		panelKey, supported := panelKeys[stat.PropertyID]
		if !supported || eligible[panelKey] < 2 {
			continue
		}
		item := candidate{modifier: modifier, stat: stat, panelKey: panelKey}
		for _, observation := range observations {
			if !observation.Loaded {
				continue
			}
			panel, panelOK := observation.Panel[panelKey]
			calculated, calculatedOK := observation.Calculated[panelKey]
			if !panelOK || !calculatedOK {
				continue
			}
			tolerance := residualTolerance(stat.Percent)
			if math.Abs((panel-calculated)-stat.Value) <= tolerance {
				item.matches++
			}
		}
		byProperty[panelKey] = append(byProperty[panelKey], item)
	}

	var inferred []InferredModifier
	for panelKey, candidates := range byProperty {
		bestIndex, bestCount, tied := -1, -1, false
		for index, item := range candidates {
			if item.matches > bestCount {
				bestIndex, bestCount, tied = index, item.matches, false
			} else if item.matches == bestCount {
				tied = true
			}
		}
		observedCount := eligible[panelKey]
		required := (3*observedCount + 3) / 4
		if required < 2 {
			required = 2
		}
		if bestIndex < 0 || tied || bestCount < required {
			continue
		}
		best := candidates[bestIndex]
		inferred = append(inferred, InferredModifier{
			ModifierID: best.modifier.ID, PropertyID: best.stat.PropertyID,
			PanelProperty: panelKey, Value: best.stat.Value, Percent: best.stat.Percent,
			MatchingCharacters: best.matches, ObservedCharacters: observedCount,
		})
	}
	sort.Slice(inferred, func(i, j int) bool { return inferred[i].PanelProperty < inferred[j].PanelProperty })
	return inferred
}

func residualTolerance(percent bool) float64 {
	if percent {
		return 0.001
	}
	// The scanner panel may expose fractional intermediates while the modeled
	// stat uses the game's integer panel rounding.
	return 1.0
}
