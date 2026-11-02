package app

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/housing"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
)

var errBuffCalibrationConflict = errors.New("scanned account bonus overlaps panel calibration")

// accountBuffSelection is derived from the latest import, never persisted.
type accountBuffSelection struct {
	FurnitureIDs   []string
	BondCharacters []int
}

type BuffOption struct {
	ProgressionID string     `json:"progression_id,omitempty"`
	Level         int        `json:"level,omitempty"`
	ID            string     `json:"id"`
	Stats         []nte.Stat `json:"stats"`
}

type BuffCharacter struct {
	BondBonusActive bool       `json:"bond_bonus_active"`
	CharacterID     int        `json:"character_id"`
	Name            string     `json:"name"`
	BondLevel       *int       `json:"bond_level,omitempty"`
	BondStats       []nte.Stat `json:"bond_stats"`
}

type AccountBuffPage struct {
	DetectedFurnitureIDs []string        `json:"detected_furniture_ids"`
	Furniture            []BuffOption    `json:"furniture"`
	Characters           []BuffCharacter `json:"characters"`
}

type bondCatalog struct {
	SchemaVersion int `json:"schema_version"`
	Bonuses       []struct {
		CharacterID string                 `json:"character_id"`
		Stats       []housing.ModifierStat `json:"stats"`
		Conditions  []any                  `json:"conditions"`
	} `json:"bonuses"`
}

func buffOptions(dataDir string) ([]BuffOption, map[int][]nte.Stat, error) {
	furniture, err := housing.Load(datafiles.New(dataDir).SpecialFurnitureModifiers())
	if err != nil {
		return nil, nil, err
	}
	options := []BuffOption{}
	for _, modifier := range furniture.Modifiers {
		if len(modifier.Conditions) != 0 || len(modifier.Stats) != 1 || modifier.Stats[0].ModifierOp != "MODIFY_MODOP_ADDITIVE" {
			continue
		}
		stat := modifier.Stats[0]
		options = append(options, BuffOption{ID: modifier.ID, ProgressionID: modifier.ProgressionID, Level: modifier.Level, Stats: []nte.Stat{{PropertyID: stat.PropertyID, Value: stat.Value, Percent: stat.Percent}}})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Stats[0].PropertyID != options[j].Stats[0].PropertyID {
			return options[i].Stats[0].PropertyID < options[j].Stats[0].PropertyID
		}
		return options[i].Stats[0].Value < options[j].Stats[0].Value
	})
	var bonds bondCatalog
	if err := readJSON(datafiles.New(dataDir).Game("characters", "bond_bonuses.json"), &bonds); err != nil {
		return nil, nil, err
	}
	if bonds.SchemaVersion != 1 {
		return nil, nil, fmt.Errorf("unsupported bond bonus schema version %d", bonds.SchemaVersion)
	}
	byCharacter := map[int][]nte.Stat{}
	for _, bonus := range bonds.Bonuses {
		id, err := strconv.Atoi(bonus.CharacterID)
		if err != nil || id <= 0 {
			return nil, nil, fmt.Errorf("invalid bond bonus character ID")
		}
		if _, duplicate := byCharacter[id]; duplicate {
			return nil, nil, fmt.Errorf("duplicate bond bonus character %d", id)
		}
		if len(bonus.Conditions) != 0 {
			continue
		}
		stats := []nte.Stat{}
		for _, stat := range bonus.Stats {
			if stat.ModifierOp != "MODIFY_MODOP_ADDITIVE" || stat.PropertyID == "" || !finiteNonNegative(stat.Value) {
				return nil, nil, fmt.Errorf("unsupported bond bonus stat for character %d", id)
			}
			stats = append(stats, nte.Stat{PropertyID: stat.PropertyID, Value: stat.Value, Percent: stat.Percent})
		}
		byCharacter[id] = stats
	}
	return options, byCharacter, nil
}

// Scanned bonuses are assumed active in projected deployed builds. Furniture is
// shared; the displayed level-10 bond reward belongs to its own character.
func scannedBuffSelection(state *decoded.State, options []BuffOption, bonds map[int][]nte.Stat) (accountBuffSelection, error) {
	selection := accountBuffSelection{}
	if state == nil {
		return selection, nil
	}
	levels := map[string]int{}
	byProgression := map[string]map[int]string{}
	for _, option := range options {
		if option.ProgressionID == "" {
			continue
		}
		if byProgression[option.ProgressionID] == nil {
			byProgression[option.ProgressionID] = map[int]string{}
		}
		byProgression[option.ProgressionID][option.Level] = option.ID
	}
	for _, observed := range state.FurnitureLevels {
		supported, ok := byProgression[observed.ProgressionID]
		if !ok {
			continue
		}
		if previous, exists := levels[observed.ProgressionID]; exists {
			if previous != observed.Level {
				return selection, fmt.Errorf("conflicting imported furniture levels for %s", observed.ProgressionID)
			}
			continue
		}
		id, ok := supported[observed.Level]
		if !ok {
			return selection, fmt.Errorf("missing housing/special_furniture_modifiers.json record for progression %s level %d", observed.ProgressionID, observed.Level)
		}
		levels[observed.ProgressionID] = observed.Level
		selection.FurnitureIDs = append(selection.FurnitureIDs, id)
	}
	seen := map[int]bool{}
	for _, character := range state.Characters {
		if character.BondLevel != nil && *character.BondLevel >= 10 && len(bonds[character.CharacterID]) > 0 && !seen[character.CharacterID] {
			selection.BondCharacters = append(selection.BondCharacters, character.CharacterID)
			seen[character.CharacterID] = true
		}
	}
	sort.Strings(selection.FurnitureIDs)
	sort.Ints(selection.BondCharacters)
	return selection, nil
}
func scannedAccountBuffSources(state *decoded.State, dataDir string, characterID int, calibration map[string]float64) (map[string][]nte.Stat, error) {
	options, bonds, err := buffOptions(dataDir)
	if err != nil {
		return nil, err
	}
	return selectedAccountBuffSources(state, options, bonds, characterID, calibration)
}

func selectedAccountBuffSources(state *decoded.State, options []BuffOption, bonds map[int][]nte.Stat, characterID int, calibration map[string]float64) (map[string][]nte.Stat, error) {
	sources := map[string][]nte.Stat{}
	selection, err := scannedBuffSelection(state, options, bonds)
	if err != nil {
		return nil, err
	}
	byID := map[string]BuffOption{}
	for _, option := range options {
		byID[option.ID] = option
	}
	seen := map[string]bool{}
	for _, id := range selection.FurnitureIDs {
		option, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown furniture modifier %q", id)
		}
		property := option.Stats[0].PropertyID
		if seen[property] {
			return nil, fmt.Errorf("multiple furniture tiers selected for %s", property)
		}
		seen[property] = true
		sources["account_furniture"] = append(sources["account_furniture"], option.Stats...)
	}
	seenCharacters := map[int]bool{}
	for _, id := range selection.BondCharacters {
		stats, ok := bonds[id]
		if !ok || len(stats) == 0 || seenCharacters[id] {
			return nil, fmt.Errorf("invalid or duplicate scanned bond bonus for character %d", id)
		}
		seenCharacters[id] = true
		if id == characterID {
			sources["account_bond"] = append([]nte.Stat(nil), stats...)
		}
	}
	// Calibration is an unclassified residual. Do not combine an overlapping
	// correction with an identified bonus: its original meaning is unknown.
	for _, stats := range sources {
		for _, stat := range stats {
			family := stat.PropertyID
			switch family {
			case "AtkAdd", "AtkUp":
				family = "Atk"
			case "DefAdd", "DefUp":
				family = "Def"
			case "HPMaxAdd", "HPMaxUp":
				family = "HP"
			}
			for property, value := range calibration {
				if value == 0 {
					continue
				}
				conflict := property == stat.PropertyID
				switch family {
				case "Atk":
					conflict = property == "AtkAdd" || property == "AtkUp" || property == "AtkBase" || property == "AtkFinal"
				case "Def":
					conflict = property == "DefAdd" || property == "DefUp" || property == "DefBase" || property == "DefFinal"
				case "HP":
					conflict = property == "HPMaxAdd" || property == "HPMaxUp" || property == "HPMaxBase" || property == "HPFinal"
				}
				if conflict {
					return nil, fmt.Errorf("%w for character %d (%s)", errBuffCalibrationConflict, characterID, property)
				}
			}
		}
	}
	return sources, nil
}

func LoadAccountBuffPage(projectDir, dataDir, language string) (AccountBuffPage, error) {
	page := AccountBuffPage{Characters: []BuffCharacter{}, DetectedFurnitureIDs: []string{}}
	var err error
	var bonds map[int][]nte.Stat
	page.Furniture, bonds, err = buffOptions(dataDir)
	if err != nil {
		return page, err
	}
	_, state, loaded, err := loadAccountData(projectDir)
	if err != nil || !loaded {
		return page, err
	}
	selection, err := scannedBuffSelection(&state, page.Furniture, bonds)
	if err != nil {
		return page, err
	}
	page.DetectedFurnitureIDs = selection.FurnitureIDs
	activeBonds := map[int]bool{}
	for _, id := range selection.BondCharacters {
		activeBonds[id] = true
	}
	labels, err := ntelocale.Load(dataDir, language)
	if err != nil {
		return page, err
	}
	for _, character := range state.Characters {
		page.Characters = append(page.Characters, BuffCharacter{CharacterID: character.CharacterID, Name: cleanCharacterName(labels.CharacterName(character.CharacterID, character.Name)), BondLevel: character.BondLevel, BondStats: bonds[character.CharacterID], BondBonusActive: activeBonds[character.CharacterID]})
	}
	sort.Slice(page.Characters, func(i, j int) bool { return page.Characters[i].Name < page.Characters[j].Name })
	return page, nil
}
