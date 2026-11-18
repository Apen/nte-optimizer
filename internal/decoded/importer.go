package decoded

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nte-optimizer/internal/equipment"
	"nte-optimizer/internal/nte"
)

type ImportResult struct {
	Diagnostics       []ImportDiagnostic
	Inventory         nte.Inventory
	State             State
	AssignedEquipment int
	Warnings          []string
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
			Level                   int              `json:"level"`
			BreakthroughLevel       int              `json:"breakthroughLevel"`
			AwakenLevel             int              `json:"awakenLevel"`
			RecordedAwakeningLevels []int            `json:"recordedAwakeningLevels,omitempty"`
			PassiveReferences       []string         `json:"passiveReferences,omitempty"`
			BondLevel               *int             `json:"bondLevel,omitempty"`
			BondStageFlags          []bool           `json:"bondStageFlags,omitempty"`
			Skills                  []CharacterSkill `json:"skills"`
		} `json:"progression"`
		Stats     CharacterStats `json:"stats"`
		Equipment struct {
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

func ImportDirectory(dir, dataDir string) (result ImportResult, err error) {
	diagnostics := &importDiagnostics{}
	defer func() {
		diagnostics.finish(err)
		result.Diagnostics = diagnostics.events
	}()
	diagnostics.step("import_catalog")
	definitions, err := equipment.Load(dataDir)
	if err != nil {
		return ImportResult{}, err
	}
	var equipment []Equipment
	var weapons []Weapon
	var account Account
	var manifest Manifest
	// Read in a stable order so diagnostics identify the first rejected domain.
	for _, input := range []struct {
		name, stage string
		dst         any
	}{
		{"equipment.json", "import_equipment_file", &equipment},
		{"weapons.json", "import_arcs_file", &weapons},
		{"manifest.json", "import_manifest", &manifest},
	} {
		diagnostics.step(input.stage)
		if err := read(filepath.Join(dir, input.name), input.dst); err != nil {
			return ImportResult{}, err
		}
	}
	diagnostics.step("import_characters")
	characters, placements, embeddedWeapons, err := readCharacters(dir)
	if err != nil {
		return ImportResult{}, err
	}
	weapons = mergeWeapons(weapons, embeddedWeapons)
	diagnostics.counts = map[string]int{"characters": len(characters), "arcs": len(weapons)}
	diagnostics.step("import_resources")
	var resources resourceExport
	if _, err := os.Stat(filepath.Join(dir, "resources.json")); err == nil {
		if err := read(filepath.Join(dir, "resources.json"), &resources); err != nil {
			return ImportResult{}, err
		}
	} else if !os.IsNotExist(err) {
		return ImportResult{}, err
	}
	diagnostics.counts = map[string]int{"resources": len(resources.Data)}
	diagnostics.step("import_account")
	accountPath := filepath.Join(dir, "account.json")
	if _, err := os.Stat(accountPath); err == nil {
		if err := read(accountPath, &account); err != nil {
			return ImportResult{}, err
		}
	} else if !os.IsNotExist(err) {
		return ImportResult{}, err
	}
	diagnostics.step("import_contract")
	if manifest.Format != "nte-scan-output" || manifest.FormatVersion < 1 || manifest.FormatVersion > 3 {
		diagnostics.code = "manifest_unsupported"
		return ImportResult{}, fmt.Errorf("unsupported decoded manifest %q version %d", manifest.Format, manifest.FormatVersion)
	}
	diagnostics.step("import_character_ids")
	characterByNet := map[string]int{}
	characterIDs := map[int]bool{}
	diagnostics.counts = map[string]int{"characters": len(characters)}
	for _, character := range characters {
		diagnostics.counts["checked_records"]++
		if character.CharacterID <= 0 || characterIDs[character.CharacterID] {
			diagnostics.code = "character_id_invalid"
			return ImportResult{}, fmt.Errorf("missing or duplicate character id %d", character.CharacterID)
		}
		characterIDs[character.CharacterID] = true
		characterByNet[character.NetID.Key()] = character.CharacterID
	}
	seen := map[string]bool{}
	diagnostics.step("import_equipment_validation")
	diagnostics.counts = map[string]int{"equipment": len(equipment)}
	modules := make([]nte.Module, 0)
	cartridges := make([]nte.Cartridge, 0)
	assigned := 0
	for _, item := range equipment {
		diagnostics.counts["checked_records"]++
		if item.Level < 0 {
			diagnostics.code = "equipment_level_invalid"
			return ImportResult{}, fmt.Errorf("equipment %q has a negative level", item.ItemID)
		}
		localID := fmt.Sprintf("game_%d_%d", item.ID.Slot, item.ID.Serial)
		if seen[localID] {
			diagnostics.code = "equipment_duplicate"
			return ImportResult{}, fmt.Errorf("duplicate equipment id %s", localID)
		}
		seen[localID] = true
		equipped := 0
		if item.CharacterNetID != nil {
			var ok bool
			equipped, ok = characterByNet[item.CharacterNetID.Key()]
			if !ok {
				diagnostics.code = "equipment_owner_unknown"
				return ImportResult{}, fmt.Errorf("equipment %s references an unknown character", localID)
			}
			assigned++
		}
		main, err := convertStats(item.MainStats)
		if err != nil {
			diagnostics.code = "equipment_main_stats_invalid"
			return ImportResult{}, fmt.Errorf("equipment %s: %w", localID, err)
		}
		sub, err := convertStats(item.SubStats)
		if err != nil {
			diagnostics.code = "equipment_sub_stats_invalid"
			return ImportResult{}, fmt.Errorf("equipment %s: %w", localID, err)
		}
		definition, ok := definitions[item.ItemID]
		if !ok || definition.Kind != item.Kind {
			diagnostics.code = "equipment_catalog_mismatch"
			return ImportResult{}, fmt.Errorf("equipment/decode.json: missing or incompatible items[%q]", item.ItemID)
		}
		if item.Level > definition.MaxLevel {
			diagnostics.code = "equipment_level_invalid"
			return ImportResult{}, fmt.Errorf("equipment %q level exceeds catalog max_level %d", item.ItemID, definition.MaxLevel)
		}
		if item.Kind == "core" {
			cartridges = append(cartridges, nte.Cartridge{LocalID: localID, GameItemID: item.ItemID, SetID: definition.SetID, SetName: item.Name, Quality: definition.Quality, Level: item.Level, MainStats: main, SubStats: sub, Locked: item.Locked, EquippedCharacterID: equipped})
			continue
		}
		var placement *nte.Placement
		if found, ok := placements[item.ID.Key()]; ok {
			placement = &nte.Placement{Row: found.Row, Column: found.Column}
		}
		modules = append(modules, nte.Module{LocalID: localID, GameItemID: item.ItemID, Quality: definition.Quality, Geometry: definition.Geometry, Area: definition.Area, Level: item.Level, MainStats: main, SubStats: sub, Locked: item.Locked, EquippedCharacterID: equipped, EquippedPlacement: placement})
	}
	inv := nte.Inventory{SchemaVersion: 1, Modules: modules, Cartridges: cartridges}
	if errs := validateInventory(inv, definitions); len(errs) > 0 {
		diagnostics.code = "equipment_invalid"
		return ImportResult{}, fmt.Errorf("decoded inventory validation: %v", errs)
	}
	diagnostics.counts = map[string]int{"modules": len(modules), "cartridges": len(cartridges), "equipped_records": assigned}
	diagnostics.step("import_links")
	warnings := validateWeaponLinks(characters, weapons)
	diagnostics.counts = map[string]int{"unresolved_links": len(warnings)}
	diagnostics.step("import_furniture")
	state := State{SchemaVersion: 2, ImportedAt: time.Now().UTC(), SourceGeneratedAt: manifest.GeneratedAt.UTC(), Account: account, Characters: characters, Weapons: weapons, Resources: resources.Data}
	var observations struct {
		FurnitureLevels  []nte.FurnitureLevel `json:"furnitureLevels"`
		PukaTrainingRank *uint32              `json:"pukaTrainingRank"`
	}
	if _, err := os.Stat(filepath.Join(dir, "character.json")); err == nil {
		if err := read(filepath.Join(dir, "character.json"), &observations); err != nil {
			return ImportResult{}, err
		}
		seen := map[string]bool{}
		for _, level := range observations.FurnitureLevels {
			if level.ProgressionID == "" || level.Level < 1 || level.Level > 100 || seen[level.ProgressionID] {
				diagnostics.code = "furniture_invalid"
				return ImportResult{}, fmt.Errorf("invalid or duplicate furniture level observation")
			}
			seen[level.ProgressionID] = true
		}
		state.FurnitureLevels = observations.FurnitureLevels
		state.PukaTrainingRank = observations.PukaTrainingRank
	} else if !os.IsNotExist(err) {
		return ImportResult{}, err
	}
	diagnostics.counts = map[string]int{"furniture": len(state.FurnitureLevels)}
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
	if export.Format != "nte-scan-characters" || export.FormatVersion < 2 || export.FormatVersion > 3 {
		return nil, nil, nil, fmt.Errorf("unsupported character export %q version %d", export.Format, export.FormatVersion)
	}
	characters := make([]Character, 0, len(export.Characters))
	placements := map[string]EquipmentPlacement{}
	embeddedWeapons := make([]Weapon, 0, len(export.Characters))
	for _, source := range export.Characters {
		character := Character{
			NetID: source.Identity.NetID, CharacterID: source.Identity.CharacterID,
			Name: source.Identity.Name, Codename: source.Identity.Codename,
			Level: source.Progression.Level, BreakthroughLevel: source.Progression.BreakthroughLevel,
			AwakenLevel: source.Progression.AwakenLevel, RecordedAwakeningLevels: source.Progression.RecordedAwakeningLevels, PassiveReferences: source.Progression.PassiveReferences, BondLevel: source.Progression.BondLevel, Skills: source.Progression.Skills,
			Stats: source.Stats, ObservedAtLogin: source.ObservedAtLogin,
			BondStageFlags: source.Progression.BondStageFlags,
		}
		if len(character.BondStageFlags) > 10 {
			return nil, nil, nil, fmt.Errorf("invalid bond stage observations")
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
