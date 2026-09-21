package nte

import "strings"

var cartridgeSetIDs = map[string]string{
	"Chaos": "Suit1", "Nature": "Suit2", "Psyche": "Suit3", "Incantation": "Suit4",
	"Lakshana": "Suit5", "Cosmos": "Suit6", "Shield": "Suit7", "Attack": "Suit8",
	"Heal": "Suit9", "Mag": "Suit10", "GetEfficiency": "Suit11", "Psychically": "Suit12",
}

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

type Cartridge struct {
	LocalID             string `json:"local_id"`
	GameItemID          string `json:"game_item_id,omitempty"`
	SetID               string `json:"set_id"`
	SetName             string `json:"set_name"`
	Quality             string `json:"quality"`
	Level               int    `json:"level"`
	MainStats           []Stat `json:"main_stats"`
	SubStats            []Stat `json:"sub_stats"`
	Locked              bool   `json:"locked,omitempty"`
	EquippedCharacterID int    `json:"equipped_character_id,omitempty"`
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
func (i *Inventory) Normalize() {
	for index := range i.Cartridges {
		if setID, ok := CartridgeSetID(i.Cartridges[index].GameItemID); ok {
			i.Cartridges[index].SetID = setID
		}
	}
}

func CartridgeSetID(gameItemID string) (string, bool) {
	prefix, _, found := strings.Cut(gameItemID, "_")
	if !found {
		return "", false
	}
	setID, ok := cartridgeSetIDs[prefix]
	return setID, ok
}
