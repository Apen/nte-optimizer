package decoded

import (
	"fmt"
	"time"
)

type NetID struct {
	Slot   int `json:"solt"`
	Serial int `json:"serial"`
}

func (id NetID) Key() string      { return key(id.Slot, id.Serial) }
func key(slot, serial int) string { return fmt.Sprintf("%d:%d", slot, serial) }

type RawStat struct {
	Property string  `json:"property"`
	Value    float64 `json:"value"`
}
type Equipment struct {
	ID                NetID               `json:"id"`
	ItemID            string              `json:"itemId"`
	Name              string              `json:"name"`
	Kind              string              `json:"kind"`
	Level             int                 `json:"level"`
	MainStats         []RawStat           `json:"mainStats"`
	SubStats          []RawStat           `json:"subStats"`
	Locked            bool                `json:"locked"`
	Discarded         bool                `json:"discarded"`
	CharacterNetID    *NetID              `json:"characterNetId,omitempty"`
	EquippedPlacement *EquipmentPlacement `json:"equippedPlacement,omitempty"`
}
type EquipmentPlacement struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}
type WeaponPassive struct {
	Title       string                   `json:"title"`
	Description string                   `json:"description"`
	Parameters  []WeaponPassiveParameter `json:"parameters,omitempty"`
}
type WeaponPassiveParameter struct {
	Index   int     `json:"index"`
	NameID  string  `json:"nameId"`
	Value   float64 `json:"value"`
	Percent bool    `json:"percent"`
}
type Weapon struct {
	ID                  NetID          `json:"id"`
	ForkID              string         `json:"forkId"`
	Name                string         `json:"name"`
	Quality             string         `json:"quality"`
	Level               int            `json:"level"`
	Breakthrough        int            `json:"breakthrough"`
	Star                int            `json:"star"`
	EquippedCharacterID int            `json:"equippedCharacterId,omitempty"`
	Description         string         `json:"description,omitempty"`
	ActivePassive       *WeaponPassive `json:"activePassive,omitempty"`
}
type CharacterSkill struct {
	AbilityID string `json:"abilityId"`
	Category  string `json:"category"`
	Level     int    `json:"level"`
}
type CharacterStats struct {
	MaxHP float64 `json:"maxHp,omitempty"`
}
type CharacterSavedState struct {
	HealthRatio float64 `json:"healthRatio,omitempty"`
}
type ActiveAwakeningBuff struct {
	Level int    `json:"level"`
	Buff  string `json:"buff"`
}
type ActiveEquipmentBuff struct {
	EquipmentID int    `json:"equipmentId"`
	Buff        string `json:"buff"`
	SetID       string `json:"setId"`
	SetName     string `json:"setName"`
	Pieces      int    `json:"pieces"`
	Effect      string `json:"effect"`
}
type ActiveWeaponBuff struct {
	ForkID string `json:"forkId"`
	Buff   string `json:"buff"`
	Star   int    `json:"star"`
}
type ObservedLoginState struct {
	Loaded                bool                  `json:"loaded"`
	ActiveAwakeningLevels []int                 `json:"activeAwakeningLevels,omitempty"`
	ActiveAwakeningBuffs  []string              `json:"activeAwakeningBuffs,omitempty"`
	ActiveEquipmentBuffs  []ActiveEquipmentBuff `json:"activeEquipmentBuffs,omitempty"`
	ActiveWeaponBuffs     []ActiveWeaponBuff    `json:"activeWeaponBuffs,omitempty"`
}
type Character struct {
	NetID               NetID               `json:"netId"`
	CharacterID         int                 `json:"characterId"`
	Name                string              `json:"name"`
	Codename            string              `json:"codename"`
	Level               int                 `json:"level"`
	BreakthroughLevel   int                 `json:"breakthroughLevel"`
	AwakenLevel         int                 `json:"awakenLevel"`
	ReportedAwakenLevel int                 `json:"reportedAwakenLevel,omitempty"`
	ForkNetID           *NetID              `json:"forkNetId,omitempty"`
	Skills              []CharacterSkill    `json:"skills,omitempty"`
	Stats               CharacterStats      `json:"stats,omitempty"`
	SavedState          CharacterSavedState `json:"savedState,omitempty"`
	ObservedAtLogin     ObservedLoginState  `json:"observedAtLogin,omitempty"`
}
type Account struct {
	Game    string `json:"game"`
	UserUID string `json:"userUid"`
	Source  string `json:"source"`
}
type Manifest struct {
	Format        string    `json:"format"`
	FormatVersion int       `json:"formatVersion"`
	GeneratedAt   time.Time `json:"generatedAt"`
}

type Resource struct {
	ID       NetID  `json:"id"`
	ItemID   string `json:"itemId"`
	Name     string `json:"name"`
	Quality  string `json:"quality"`
	Quantity int    `json:"quantity"`
}

type State struct {
	SchemaVersion     int                        `json:"schema_version"`
	ImportedAt        time.Time                  `json:"imported_at"`
	SourceGeneratedAt time.Time                  `json:"source_generated_at"`
	Account           Account                    `json:"account"`
	Characters        []Character                `json:"characters"`
	Weapons           []Weapon                   `json:"weapons"`
	Resources         []Resource                 `json:"resources,omitempty"`
	PanelOverrides    map[int]map[string]float64 `json:"-"`
}
