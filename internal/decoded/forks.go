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
	Parameters map[string]float64 `json:"parameters,omitempty"`
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
	if note == "" && len(definition.EffectsByStar[star].Parameters) > 0 && len(conditional) == 0 {
		note = "The Arc effect was imported, but its conditions are not yet included in the maximum projection."
	}
	return panel, permanent, conditional, note
}
