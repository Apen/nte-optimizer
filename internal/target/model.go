package target

import "nte-optimizer/internal/scoring"

type Goal struct {
	PropertyID string  `json:"property_id"`
	Label      string  `json:"label"`
	Minimum    float64 `json:"minimum"`
	Maximum    float64 `json:"maximum,omitempty"`
	Percent    bool    `json:"percent"`
}

type Arc struct {
	Rank       int                `json:"rank"`
	Name       string             `json:"name"`
	Refinement string             `json:"refinement,omitempty"`
	BaseATK    float64            `json:"base_atk,omitempty"`
	Stats      map[string]float64 `json:"stats,omitempty"`
	Note       string             `json:"note,omitempty"`
}

type BuildTarget struct {
	SchemaVersion int             `json:"schema_version"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	MainStats     []string        `json:"main_stats"`
	Stats         []string        `json:"stats"`
	Goals         []Goal          `json:"goals"`
	Arcs          []Arc           `json:"arcs"`
	Sets          []string        `json:"sets"`
	Notes         []string        `json:"notes,omitempty"`
	Character     CharacterConfig `json:"character"`
	Builds        []BuildProfile  `json:"builds,omitempty"`
}

type CharacterConfig struct {
	CharacterID  int                        `json:"character_id"`
	GridID       string                     `json:"grid_id"`
	CurrentStats map[string]float64         `json:"current_stats,omitempty"`
	BaseStats    map[string]float64         `json:"base_stats,omitempty"`
	ConsoleTrait *scoring.ConsoleTrait      `json:"console_trait,omitempty"`
	Caps         map[string]scoring.StatCap `json:"caps,omitempty"`
}

type ProfileVariant struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	PreferredSets []scoring.SetPreference `json:"preferred_sets,omitempty"`
}

// BuildProfile describes one optimizer strategy and its selectable set variants.
type BuildProfile struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	MainStats []string           `json:"main_stats"`
	Stats     []string           `json:"stats"`
	Weights   map[string]float64 `json:"weights"`
	Variants  []ProfileVariant   `json:"variants,omitempty"`
}
