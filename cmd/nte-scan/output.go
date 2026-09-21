package main

import (
	"fmt"
	"strings"
	"time"

	scannerexport "nte-optimizer/internal/scanner/exporter"
)

type domainOutput struct {
	Status string `json:"status"`
	Source string `json:"source"`
	Count  int    `json:"count"`
	Note   string `json:"note,omitempty"`
	Data   any    `json:"data"`
}
type characterOutput struct {
	Format        string                `json:"format"`
	FormatVersion int                   `json:"formatVersion"`
	GeneratedAt   string                `json:"generatedAt"`
	Source        string                `json:"source"`
	Status        string                `json:"status"`
	Count         int                   `json:"count"`
	Note          string                `json:"note"`
	Validation    map[string]int        `json:"validation"`
	Characters    []characterExportItem `json:"characters"`
}
type characterExportItem struct {
	Identity        characterIdentity        `json:"identity"`
	Progression     characterProgression     `json:"progression"`
	Stats           characterStats           `json:"stats"`
	SavedState      characterSavedState      `json:"savedState"`
	Equipment       characterEquipment       `json:"equipment"`
	ObservedAtLogin characterObservedAtLogin `json:"observedAtLogin"`
}
type characterIdentity struct {
	CharacterID uint32    `json:"characterId"`
	Name        string    `json:"name"`
	Codename    string    `json:"codename"`
	NetID       itemNetID `json:"netId"`
}
type characterProgression struct {
	Level             int32                 `json:"level"`
	BreakthroughLevel int32                 `json:"breakthroughLevel"`
	AwakenLevel       int32                 `json:"awakenLevel"`
	Skills            []characterSkillLevel `json:"skills"`
}
type characterStats struct {
	MaxHP             float32             `json:"maxHp"`
	Attack            *float32            `json:"attack,omitempty"`
	Defense           *float32            `json:"defense,omitempty"`
	Endurance         *float32            `json:"endurance,omitempty"`
	CritRate          *float32            `json:"critRate,omitempty"`
	CritDamage        *float32            `json:"critDamage,omitempty"`
	ChargeEfficiency  *float32            `json:"chargeEfficiency,omitempty"`
	CycleIntensity    *float32            `json:"cycleIntensity,omitempty"`
	BreakIntensity    *float32            `json:"breakIntensity,omitempty"`
	UniversalDMGBonus *float32            `json:"universalDamageBonus,omitempty"`
	PanelBase         *characterBaseStats `json:"panelBase,omitempty"`
	Source            string              `json:"source,omitempty"`
}
type characterBaseStats struct {
	MaxHP   float32 `json:"maxHp"`
	Attack  float32 `json:"attack"`
	Defense float32 `json:"defense"`
}
type characterSavedState struct {
	HealthRatio float32 `json:"healthRatio"`
}
type characterEquipment struct {
	Weapon  *weaponItem     `json:"weapon,omitempty"`
	Cores   []inventoryItem `json:"cores"`
	Modules []inventoryItem `json:"modules"`
}
type characterObservedAtLogin struct {
	Loaded                bool                    `json:"loaded"`
	ActiveAwakeningLevels []int32                 `json:"activeAwakeningLevels,omitempty"`
	ActiveAwakeningBuffs  []string                `json:"activeAwakeningBuffs,omitempty"`
	ActiveEquipmentBuffs  []observedEquipmentBuff `json:"activeEquipmentBuffs,omitempty"`
	ActiveWeaponBuffs     []observedWeaponBuff    `json:"activeWeaponBuffs,omitempty"`
}
type activeCharacterEquipment struct {
	CharacterID   uint32          `json:"characterId"`
	CharacterName string          `json:"characterName"`
	Weapon        *weaponItem     `json:"weapon,omitempty"`
	Core          []inventoryItem `json:"cores"`
	Modules       []inventoryItem `json:"modules"`
}

func writeJSON(path string, v any) error {
	return scannerexport.WriteJSON(path, v)
}
func exportCharacters(characters []characterItem, equipmentByCharacter map[uint32]*activeCharacterEquipment) []characterExportItem {
	out := make([]characterExportItem, 0, len(characters))
	for _, character := range characters {
		equipment := characterEquipment{Cores: []inventoryItem{}, Modules: []inventoryItem{}}
		stats := characterStats{MaxHP: character.MaxHP}
		if panel := character.PanelStats; panel != nil {
			stats.Attack = &panel.Attack
			stats.Defense = &panel.Defense
			stats.Endurance = &panel.Endurance
			stats.CritRate = &panel.CritRate
			stats.CritDamage = &panel.CritDamage
			stats.ChargeEfficiency = &panel.ChargeEfficiency
			stats.CycleIntensity = &panel.CycleIntensity
			stats.BreakIntensity = &panel.BreakIntensity
			stats.UniversalDMGBonus = &panel.UniversalDMGBonus
			stats.PanelBase = &characterBaseStats{MaxHP: panel.BaseHP, Attack: panel.BaseAttack, Defense: panel.BaseDefense}
			stats.Source = "unreal_attribute_set_and_equipment"
		}
		if active := equipmentByCharacter[character.CharacterID]; active != nil {
			equipment.Weapon = active.Weapon
			equipment.Cores = active.Core
			equipment.Modules = active.Modules
		}
		out = append(out, characterExportItem{
			Identity:        characterIdentity{CharacterID: character.CharacterID, Name: character.Name, Codename: character.Codename, NetID: character.NetID},
			Progression:     characterProgression{Level: character.Level, BreakthroughLevel: character.BreakthroughLevel, AwakenLevel: character.AwakenLevel, Skills: character.SkillLevels},
			Stats:           stats,
			SavedState:      characterSavedState{HealthRatio: character.SavedHealthRatio},
			Equipment:       equipment,
			ObservedAtLogin: characterObservedAtLogin{Loaded: character.ObservedLoadedAtLogin, ActiveAwakeningLevels: character.ObservedActiveAwakeningLevels, ActiveAwakeningBuffs: character.ObservedActiveAwakeningBuffs, ActiveEquipmentBuffs: character.ObservedActiveEquipmentBuffs, ActiveWeaponBuffs: character.ObservedActiveWeaponBuffs},
		})
	}
	return out
}
func writeOutputDir(dir string, r report) error {
	return writeOutputDirWithWriter(dir, r, writeJSON)
}

func writeOutputDirWithWriter(dir string, r report, writer func(string, any) error) error {
	chars := append([]characterItem(nil), r.UDP.Characters...)
	weapons := append([]weaponItem(nil), r.UDP.Weapons...)
	items := append([]inventoryItem(nil), r.UDP.Items...)
	if err := validateOutputRelations(chars, weapons, items); err != nil {
		return fmt.Errorf("validate decoded output: %w", err)
	}
	weaponByChar := map[uint32]*weaponItem{}
	for i := range weapons {
		if weapons[i].EquippedCharacterID != nil {
			weaponByChar[*weapons[i].EquippedCharacterID] = &weapons[i]
		}
	}
	byChar := map[uint32]*activeCharacterEquipment{}
	for _, c := range chars {
		byChar[c.CharacterID] = &activeCharacterEquipment{CharacterID: c.CharacterID, CharacterName: c.Name, Weapon: weaponByChar[c.CharacterID], Core: []inventoryItem{}, Modules: []inventoryItem{}}
	}
	for _, it := range items {
		if it.CharacterNetID == nil {
			continue
		}
		for _, c := range chars {
			if c.NetID == *it.CharacterNetID {
				l := byChar[c.CharacterID]
				if it.Kind == "core" {
					l.Core = append(l.Core, it)
				} else {
					l.Modules = append(l.Modules, it)
				}
				break
			}
		}
	}
	generated := time.Now().UTC().Format(time.RFC3339)
	characterWeaponRefs, linkedWeapons, positionedModules, equippedModules := 0, 0, 0, 0
	observedWeaponBuffs, matchingObservedWeaponBuffs := 0, 0
	for _, c := range chars {
		if c.ForkNetID != nil {
			characterWeaponRefs++
		}
		for _, observed := range c.ObservedActiveWeaponBuffs {
			observedWeaponBuffs++
			weapon := weaponByChar[c.CharacterID]
			if weapon != nil && strings.EqualFold(weapon.ForkID, observed.ForkID) && weapon.Star == int32(observed.Star) {
				matchingObservedWeaponBuffs++
			}
		}
	}
	for _, w := range weapons {
		if w.EquippedCharacterID != nil {
			linkedWeapons++
		}
	}
	for _, it := range items {
		if it.Kind == "module" && it.CharacterNetID != nil {
			equippedModules++
			if it.EquippedPlacement != nil {
				positionedModules++
			}
		}
	}
	characterValidation := map[string]int{"weaponReferences": characterWeaponRefs, "linkedWeaponReferences": linkedWeapons, "missingWeaponReferences": characterWeaponRefs - linkedWeapons, "observedWeaponBuffs": observedWeaponBuffs, "matchingObservedWeaponBuffs": matchingObservedWeaponBuffs, "mismatchingObservedWeaponBuffs": observedWeaponBuffs - matchingObservedWeaponBuffs}
	manifest := map[string]any{"format": "nte-scan-output", "formatVersion": 3, "generatedAt": generated, "sourceCapture": r.Input, "files": []string{"character.json", "weapons.json", "equipment.json", "resources.json"}, "validation": map[string]any{"characterWeaponReferences": characterWeaponRefs, "linkedWeaponReferences": linkedWeapons, "missingWeaponReferences": characterWeaponRefs - linkedWeapons, "observedWeaponBuffs": observedWeaponBuffs, "matchingObservedWeaponBuffs": matchingObservedWeaponBuffs, "mismatchingObservedWeaponBuffs": observedWeaponBuffs - matchingObservedWeaponBuffs, "equippedModules": equippedModules, "positionedModules": positionedModules, "missingModulePositions": equippedModules - positionedModules}}
	resourceStatus := "decoded"
	resourceNote := "Quantités validées contre le catalogue statique officiel."
	if len(r.UDP.Resources) == 0 {
		resourceStatus = "not_present_in_capture"
		resourceNote = "Aucun enregistrement de ressource validé dans cette capture."
	}
	resources := domainOutput{resourceStatus, "packet_capture", len(r.UDP.Resources), resourceNote, r.UDP.Resources}
	characters := characterOutput{"nte-scan-characters", 2, generated, r.Input, "decoded", len(chars), "Une occurrence par personnage avec progression, compétences, état sauvegardé, équipement actif complet et observations au login.", characterValidation, exportCharacters(chars, byChar)}
	return scannerexport.PublishWithWriter(dir, []scannerexport.File{
		{Name: "character.json", Value: characters},
		{Name: "weapons.json", Value: weapons},
		{Name: "equipment.json", Value: items},
		{Name: "resources.json", Value: resources},
		{Name: "manifest.json", Value: manifest},
	}, writer)
}

func validateOutputRelations(characters []characterItem, weapons []weaponItem, equipment []inventoryItem) error {
	characterIDs := make(map[uint32]bool, len(characters))
	characterNetIDs := make(map[itemNetID]bool, len(characters))
	for _, character := range characters {
		if character.CharacterID == 0 || characterIDs[character.CharacterID] {
			return fmt.Errorf("missing or duplicate character id %d", character.CharacterID)
		}
		if !validNetID(character.NetID) || characterNetIDs[character.NetID] {
			return fmt.Errorf("missing or duplicate character network id %+v", character.NetID)
		}
		characterIDs[character.CharacterID] = true
		characterNetIDs[character.NetID] = true
	}
	weaponIDs := make(map[itemNetID]bool, len(weapons))
	for _, weapon := range weapons {
		if !validNetID(weapon.ID) || weaponIDs[weapon.ID] {
			return fmt.Errorf("missing or duplicate weapon network id %+v", weapon.ID)
		}
		if weapon.ForkID == "" {
			return fmt.Errorf("weapon %+v has no fork id", weapon.ID)
		}
		if weapon.EquippedCharacterID != nil && !characterIDs[*weapon.EquippedCharacterID] {
			return fmt.Errorf("weapon %+v references unknown character %d", weapon.ID, *weapon.EquippedCharacterID)
		}
		weaponIDs[weapon.ID] = true
	}
	for _, character := range characters {
		if character.ForkNetID != nil && !weaponIDs[*character.ForkNetID] {
			return fmt.Errorf("character %d references missing weapon %+v", character.CharacterID, *character.ForkNetID)
		}
	}
	equipmentIDs := make(map[itemNetID]bool, len(equipment))
	for _, item := range equipment {
		if !validNetID(item.ID) || equipmentIDs[item.ID] {
			return fmt.Errorf("missing or duplicate equipment network id %+v", item.ID)
		}
		if item.Kind != "module" && item.Kind != "core" {
			return fmt.Errorf("equipment %+v has unknown kind %q", item.ID, item.Kind)
		}
		if item.CharacterNetID != nil && !characterNetIDs[*item.CharacterNetID] {
			return fmt.Errorf("equipment %+v references unknown character %+v", item.ID, *item.CharacterNetID)
		}
		equipmentIDs[item.ID] = true
	}
	return nil
}
