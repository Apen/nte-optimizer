package nte

import (
	"encoding/json"
)

// EquipmentDefinitions are supplied explicitly by the game-data boundary.
type EquipmentDefinition struct {
	Kind, Quality, Geometry, SetID string
	Area, MaxLevel                 int
}
type EquipmentDefinitions map[string]EquipmentDefinition

type Stat struct {
	PropertyID string  `json:"property_id"`
	Value      float64 `json:"value"`
	Percent    bool    `json:"percent"`
}

type Module struct {
	LocalID             string     `json:"local_id"`
	GameItemID          string     `json:"game_item_id,omitempty"`
	Quality             string     `json:"quality"`
	SetID               string     `json:"set_id"`
	SetName             string     `json:"set_name"`
	Geometry            string     `json:"geometry"`
	Area                int        `json:"area"`
	Level               int        `json:"level"`
	MainStats           []Stat     `json:"main_stats"`
	SubStats            []Stat     `json:"sub_stats"`
	Locked              bool       `json:"locked,omitempty"`
	EquippedCharacterID int        `json:"equipped_character_id,omitempty"`
	EquippedPlacement   *Placement `json:"equipped_placement,omitempty"`
}

type Placement struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// ActiveSubStats excludes serialized but inactive level-zero module substats.
// Positive levels preserve the imported values; intermediate unlock thresholds
// have not been established and must not be inferred from this level-zero rule.
func (m Module) ActiveSubStats() []Stat {
	if m.Level == 0 {
		return nil
	}
	return m.SubStats
}

// MarshalJSON exposes activation separately while preserving the raw substats
// for inventory inspection and future upgrades.
func (m Module) MarshalJSON() ([]byte, error) {
	type module Module
	return json.Marshal(struct {
		module
		SubStatsActive bool `json:"sub_stats_active"`
	}{module: module(m), SubStatsActive: m.Level != 0})
}

type Cartridge struct {
	LocalID                string `json:"local_id"`
	GameItemID             string `json:"game_item_id,omitempty"`
	SetID                  string `json:"set_id"`
	SetName                string `json:"set_name"`
	Quality                string `json:"quality"`
	Level                  int    `json:"level"`
	MainStats              []Stat `json:"main_stats"`
	SubStats               []Stat `json:"sub_stats"`
	Locked                 bool   `json:"locked,omitempty"`
	EquippedCharacterID    int    `json:"equipped_character_id,omitempty"`
	SimulatedMainStatLevel int    `json:"simulated_main_stat_level,omitempty"`
}

// Inventory is the decoded account equipment consumed by the optimizer.
type Inventory struct {
	SchemaVersion int         `json:"schema_version"`
	Modules       []Module    `json:"modules"`
	Cartridges    []Cartridge `json:"cartridges,omitempty"`
}

// Normalize restores stable protocol identifiers when loading inventories
// written by older application versions. GameItemID is authoritative because
// it comes from the captured game object and does not depend on localization.
func (i *Inventory) Normalize(definitions EquipmentDefinitions) {
	for index := range i.Cartridges {
		if definition, ok := definitions[i.Cartridges[index].GameItemID]; ok && definition.Kind == "core" {
			i.Cartridges[index].SetID = definition.SetID
		}
	}
}
