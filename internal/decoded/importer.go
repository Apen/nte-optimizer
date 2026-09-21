package decoded

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nte-optimizer/internal/nte"
)

type ImportResult struct {
	Inventory         nte.Inventory
	State             State
	AssignedEquipment int
	Warnings          []string
}
type itemDefinition struct {
	geometry string
	area     int
	quality  string
	setID    string
}

type characterExport struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"formatVersion"`
	Characters    []struct {
		Identity struct {
			CharacterID int    `json:"characterId"`
			Name        string `json:"name"`
			Codename    string `json:"codename"`
			NetID       NetID  `json:"netId"`
		} `json:"identity"`
		Progression struct {
			Level             int              `json:"level"`
			BreakthroughLevel int              `json:"breakthroughLevel"`
			AwakenLevel       int              `json:"awakenLevel"`
			Skills            []CharacterSkill `json:"skills"`
		} `json:"progression"`
		Stats      CharacterStats      `json:"stats"`
		SavedState CharacterSavedState `json:"savedState"`
		Equipment  struct {
			Weapon  *Weapon     `json:"weapon,omitempty"`
			Cores   []Equipment `json:"cores"`
			Modules []Equipment `json:"modules"`
		} `json:"equipment"`
		ObservedAtLogin ObservedLoginState `json:"observedAtLogin"`
	} `json:"characters"`
}

type resourceExport struct {
	Data []Resource `json:"data"`
}

var moduleDefinitions = map[string]itemDefinition{
	"cell2_style1_1": {"H_2", 2, "", ""}, "cell2_style2_1": {"V_2", 2, "", ""},
	"cell3_style1_1": {"H_3", 3, "", ""}, "cell3_style2_1_1": {"V_3", 3, "", ""},
	"cell3_style2_1": {"V_3", 3, "", ""}, "cell3_style3_1": {"L_3_TR", 3, "", ""},
	"cell3_style4_1": {"L_3_BR", 3, "", ""}, "cell3_style5_1": {"L_3_BL", 3, "", ""}, "cell3_style6_1": {"L_3_TL", 3, "", ""},
	"cell4_style1_1": {"H_4", 4, "", ""}, "cell4_style2_1": {"V_4", 4, "", ""}, "cell4_style5_1": {"Trap_4_H", 4, "", ""}, "cell4_style6_1": {"Trap_4_V", 4, "", ""},
}

func ImportDirectory(dir string) (ImportResult, error) {
	var equipment []Equipment
	var weapons []Weapon
	var account Account
	var manifest Manifest
	for path, dst := range map[string]any{"equipment.json": &equipment, "weapons.json": &weapons, "manifest.json": &manifest} {
		if err := read(filepath.Join(dir, path), dst); err != nil {
			return ImportResult{}, err
		}
	}
	characters, placements, embeddedWeapons, err := readCharacters(dir)
	if err != nil {
		return ImportResult{}, err
	}
	weapons = mergeWeapons(weapons, embeddedWeapons)
	var resources resourceExport
	if _, err := os.Stat(filepath.Join(dir, "resources.json")); err == nil {
		if err := read(filepath.Join(dir, "resources.json"), &resources); err != nil {
			return ImportResult{}, err
		}
	} else if !os.IsNotExist(err) {
		return ImportResult{}, err
	}
	accountPath := filepath.Join(dir, "account.json")
	if _, err := os.Stat(accountPath); err == nil {
		if err := read(accountPath, &account); err != nil {
			return ImportResult{}, err
		}
	} else if !os.IsNotExist(err) {
		return ImportResult{}, err
	}
	if manifest.Format != "nte-scan-output" || manifest.FormatVersion < 1 || manifest.FormatVersion > 3 {
		return ImportResult{}, fmt.Errorf("unsupported decoded manifest %q version %d", manifest.Format, manifest.FormatVersion)
	}
	characterByNet := map[string]int{}
	characterIDs := map[int]bool{}
	for _, character := range characters {
		if character.CharacterID <= 0 || characterIDs[character.CharacterID] {
			return ImportResult{}, fmt.Errorf("missing or duplicate character id %d", character.CharacterID)
		}
		characterIDs[character.CharacterID] = true
		characterByNet[character.NetID.Key()] = character.CharacterID
	}
	seen := map[string]bool{}
	modules := make([]nte.Module, 0)
	cartridges := make([]nte.Cartridge, 0)
	assigned := 0
	for _, item := range equipment {
		localID := fmt.Sprintf("game_%d_%d", item.ID.Slot, item.ID.Serial)
		if seen[localID] {
			return ImportResult{}, fmt.Errorf("duplicate equipment id %s", localID)
		}
		seen[localID] = true
		equipped := 0
		if item.CharacterNetID != nil {
			var ok bool
			equipped, ok = characterByNet[item.CharacterNetID.Key()]
			if !ok {
				return ImportResult{}, fmt.Errorf("equipment %s references an unknown character", localID)
			}
			assigned++
		}
		main, err := convertStats(item.MainStats)
		if err != nil {
			return ImportResult{}, fmt.Errorf("equipment %s: %w", localID, err)
		}
		sub, err := convertStats(item.SubStats)
		if err != nil {
			return ImportResult{}, fmt.Errorf("equipment %s: %w", localID, err)
		}
		if item.Kind == "core" {
			setID, quality, ok := coreSetID(item.ItemID)
			if !ok {
				return ImportResult{}, fmt.Errorf("unknown core itemId %q", item.ItemID)
			}
			cartridges = append(cartridges, nte.Cartridge{LocalID: localID, GameItemID: item.ItemID, SetID: setID, SetName: item.Name, Quality: quality, Level: item.Level, MainStats: main, SubStats: sub, Locked: item.Locked, EquippedCharacterID: equipped})
			continue
		}
		base, quality := splitModuleItemID(item.ItemID)
		definition, ok := moduleDefinitions[base]
		if item.Kind != "module" || !ok {
			return ImportResult{}, fmt.Errorf("unknown module itemId %q", item.ItemID)
		}
		var placement *nte.Placement
		if found, ok := placements[item.ID.Key()]; ok {
			placement = &nte.Placement{Row: found.Row, Column: found.Column}
		}
		modules = append(modules, nte.Module{LocalID: localID, GameItemID: item.ItemID, Quality: quality, Geometry: definition.geometry, Area: definition.area, Level: item.Level, MainStats: main, SubStats: sub, Locked: item.Locked, EquippedCharacterID: equipped, EquippedPlacement: placement})
	}
	inv := nte.Inventory{SchemaVersion: 1, Modules: modules, Cartridges: cartridges}
	if errs := validateInventory(inv); len(errs) > 0 {
		return ImportResult{}, fmt.Errorf("decoded inventory validation: %v", errs)
	}
	warnings := validateWeaponLinks(characters, weapons)
	state := State{SchemaVersion: 2, ImportedAt: time.Now().UTC(), SourceGeneratedAt: manifest.GeneratedAt.UTC(), Account: account, Characters: characters, Weapons: weapons, Resources: resources.Data}
	return ImportResult{Inventory: inv, State: state, AssignedEquipment: assigned, Warnings: warnings}, nil
}

func readCharacters(dir string) ([]Character, map[string]EquipmentPlacement, []Weapon, error) {
	legacyPath := filepath.Join(dir, "characters.json")
	if _, err := os.Stat(legacyPath); err == nil {
		var characters []Character
		if err := read(legacyPath, &characters); err != nil {
			return nil, nil, nil, err
		}
		return characters, map[string]EquipmentPlacement{}, nil, nil
	} else if !os.IsNotExist(err) {
		return nil, nil, nil, err
	}

	var export characterExport
	if err := read(filepath.Join(dir, "character.json"), &export); err != nil {
		return nil, nil, nil, err
	}
	if export.Format != "nte-scan-characters" || export.FormatVersion != 2 {
		return nil, nil, nil, fmt.Errorf("unsupported character export %q version %d", export.Format, export.FormatVersion)
	}
	characters := make([]Character, 0, len(export.Characters))
	placements := map[string]EquipmentPlacement{}
	embeddedWeapons := make([]Weapon, 0, len(export.Characters))
	for _, source := range export.Characters {
		awakenLevel := source.Progression.AwakenLevel
		if source.ObservedAtLogin.Loaded {
			for _, observedLevel := range source.ObservedAtLogin.ActiveAwakeningLevels {
				if observedLevel > awakenLevel {
					awakenLevel = observedLevel
				}
			}
		}
		character := Character{
			NetID: source.Identity.NetID, CharacterID: source.Identity.CharacterID,
			Name: source.Identity.Name, Codename: source.Identity.Codename,
			Level: source.Progression.Level, BreakthroughLevel: source.Progression.BreakthroughLevel,
			AwakenLevel: awakenLevel, ReportedAwakenLevel: source.Progression.AwakenLevel, Skills: source.Progression.Skills,
			Stats: source.Stats, SavedState: source.SavedState, ObservedAtLogin: source.ObservedAtLogin,
		}
		if source.Equipment.Weapon != nil {
			weapon := *source.Equipment.Weapon
			weapon.EquippedCharacterID = character.CharacterID
			character.ForkNetID = &weapon.ID
			embeddedWeapons = append(embeddedWeapons, weapon)
		}
		for _, module := range source.Equipment.Modules {
			if module.EquippedPlacement != nil {
				placements[module.ID.Key()] = *module.EquippedPlacement
			}
		}
		characters = append(characters, character)
	}
	return characters, placements, embeddedWeapons, nil
}

func mergeWeapons(weapons, embedded []Weapon) []Weapon {
	byID := make(map[string]int, len(weapons))
	for index := range weapons {
		byID[weapons[index].ID.Key()] = index
	}
	for _, weapon := range embedded {
		if index, ok := byID[weapon.ID.Key()]; ok {
			weapons[index].EquippedCharacterID = weapon.EquippedCharacterID
			if weapons[index].Description == "" {
				weapons[index].Description = weapon.Description
			}
			if weapons[index].ActivePassive == nil {
				weapons[index].ActivePassive = weapon.ActivePassive
			}
			continue
		}
		byID[weapon.ID.Key()] = len(weapons)
		weapons = append(weapons, weapon)
	}
	return weapons
}

func coreSetID(itemID string) (setID, quality string, ok bool) {
	parts := strings.Split(itemID, "_")
	if len(parts) < 2 {
		return "", "", false
	}
	setID, ok = nte.CartridgeSetID(itemID)
	return setID, parts[len(parts)-1], ok
}

func convertStats(input []RawStat) ([]nte.Stat, error) {
	out := make([]nte.Stat, 0, len(input))
	for _, stat := range input {
		if stat.Property == "" {
			return nil, fmt.Errorf("stat without property")
		}
		out = append(out, nte.Stat{PropertyID: stat.Property, Value: stat.Value, Percent: isPercent(stat.Property)})
	}
	return out, nil
}
func isPercent(id string) bool {
	return strings.HasSuffix(id, "Up") || strings.HasSuffix(id, "Base") && id != "MagBase" && id != "UnbalIntensityBase" && id != "AtkBase" && id != "DefBase" && id != "HPMaxBase"
}
func splitModuleItemID(id string) (string, string) {
	for _, quality := range []string{"_Orange", "_Purple", "_Blue"} {
		if strings.HasSuffix(id, quality) {
			return strings.TrimSuffix(id, quality), strings.ToLower(strings.TrimPrefix(quality, "_"))
		}
	}
	return id, "unknown"
}
func validateWeaponLinks(characters []Character, weapons []Weapon) []string {
	known := map[string]bool{}
	for _, weapon := range weapons {
		known[weapon.ID.Key()] = true
	}
	warnings := []string{}
	for _, character := range characters {
		if character.ForkNetID != nil && !known[character.ForkNetID.Key()] {
			warnings = append(warnings, fmt.Sprintf("character %d (%s) references a weapon absent from weapons.json", character.CharacterID, character.Name))
		}
	}
	return warnings
}
func read(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
