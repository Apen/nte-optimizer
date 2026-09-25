package decoded

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"nte-optimizer/internal/nte"
)

type ForkDefinition struct {
	PanelByLevelStage map[string][]nte.Stat `json:"panel_by_level_stage"`
	LevelStats        map[string][]nte.Stat `json:"level_stats,omitempty"`
	BreakthroughStats map[string][]nte.Stat `json:"breakthrough_stats,omitempty"`
	PermanentByStar   map[string][]nte.Stat `json:"permanent_by_star"`
	ConditionalByStar map[string][]nte.Stat `json:"conditional_by_star"`
	ConditionalNote   string                `json:"conditional_note"`
	EffectsByStar     map[string]ForkEffect `json:"effects_by_star,omitempty"`
}

type ForkEffect struct {
	// Parameters is retained for older catalogs. Its map order is not meaningful,
	// so only OrderedParameters can be used to fill positional description tokens.
	Parameters        map[string]float64    `json:"-"`
	OrderedParameters []ForkEffectParameter `json:"-"`
}

type ForkEffectParameter struct {
	NameID    string  `json:"name_id"`
	Value     float64 `json:"value"`
	IsPercent bool    `json:"is_percent"`
}

func (effect *ForkEffect) UnmarshalJSON(data []byte) error {
	var raw struct {
		Parameters     json.RawMessage       `json:"parameters"`
		ParameterOrder []ForkEffectParameter `json:"parameter_order,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Parameters) == 0 || string(raw.Parameters) == "null" {
		*effect = ForkEffect{}
		return nil
	}

	if raw.Parameters[0] == '[' {
		if err := json.Unmarshal(raw.Parameters, &effect.OrderedParameters); err != nil {
			return err
		}
		return nil
	}
	if err := json.Unmarshal(raw.Parameters, &effect.Parameters); err != nil {
		return err
	}
	if len(raw.ParameterOrder) > 0 {
		effect.OrderedParameters = make([]ForkEffectParameter, 0, len(raw.ParameterOrder))
		for _, parameter := range raw.ParameterOrder {
			value, ok := effect.Parameters[parameter.NameID]
			if !ok {
				continue
			}
			parameter.Value = value
			effect.OrderedParameters = append(effect.OrderedParameters, parameter)
		}
	}
	return nil
}

func (effect ForkEffect) HasParameters() bool {
	return len(effect.Parameters) > 0 || len(effect.OrderedParameters) > 0
}

type ForkCatalog struct {
	SchemaVersion int                       `json:"schema_version"`
	Forks         map[string]ForkDefinition `json:"forks"`
}

func LoadForkCatalog(path string) (ForkCatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ForkCatalog{}, err
	}
	var c ForkCatalog
	if err := json.Unmarshal(b, &c); err != nil {
		return ForkCatalog{}, err
	}
	if c.SchemaVersion != 1 {
		return ForkCatalog{}, fmt.Errorf("unsupported fork catalog schema")
	}
	return c, nil
}
func (c ForkCatalog) Stats(weapon Weapon) (panel, permanent, conditional []nte.Stat, note string) {
	definition, ok := c.Forks[weapon.ForkID]
	if !ok {
		return
	}
	panel = definition.PanelByLevelStage[fmt.Sprintf("%d:%d", weapon.Level, weapon.Breakthrough)]
	if len(panel) == 0 {
		panel = append(panel, definition.LevelStats[strconv.Itoa(weapon.Level)]...)
		panel = append(panel, definition.BreakthroughStats[strconv.Itoa(weapon.Breakthrough)]...)
	}
	star := strconv.Itoa(weapon.Star)
	permanent = definition.PermanentByStar[star]
	conditional = definition.ConditionalByStar[star]
	note = definition.ConditionalNote
	if note == "" && definition.EffectsByStar[star].HasParameters() && len(conditional) == 0 {
		note = "The Arc effect was imported, but its conditions are not yet included in the maximum projection."
	}
	return panel, permanent, conditional, note
}
