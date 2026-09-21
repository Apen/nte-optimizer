package app

import (
	"fmt"

	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

// CharacterGameState is a read-only snapshot of what the latest account import
// reported as equipped in game. It is deliberately separate from LocalBuild.
type CharacterGameState struct {
	Character  decoded.Character      `json:"character"`
	Weapon     *EquipmentCatalogArc   `json:"weapon,omitempty"`
	Cartridges []nte.Cartridge        `json:"cartridges"`
	Modules    []nte.Module           `json:"modules"`
	Stats      *optimizer.StatSummary `json:"stats,omitempty"`
	ImportedAt string                 `json:"imported_at,omitempty"`
}

func LoadCharacterGameState(projectDir, dataDir string, characterID int, language string) (CharacterGameState, error) {
	_, state, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return CharacterGameState{}, err
	}
	if !loaded {
		return CharacterGameState{}, fmt.Errorf("no account has been imported")
	}
	var overrides struct {
		Characters map[int]map[string]float64 `json:"characters"`
	}
	if loaded, err := readOptionalJSON(workspaceFile(projectDir, "account_overrides.json"), &overrides); err != nil {
		return CharacterGameState{}, err
	} else if loaded {
		state.PanelOverrides = overrides.Characters
	}
	equipment, err := LoadEquipmentCatalog(projectDir, dataDir, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	catalog, err := ntelocale.Load(dataDir, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	result := CharacterGameState{Cartridges: []nte.Cartridge{}, Modules: []nte.Module{}}
	if !state.ImportedAt.IsZero() {
		result.ImportedAt = state.ImportedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	found := false
	for _, character := range state.Characters {
		if character.CharacterID != characterID {
			continue
		}
		character.Name = cleanCharacterName(catalog.CharacterName(character.CharacterID, character.Name))
		result.Character = character
		found = true
		break
	}
	if !found {
		return CharacterGameState{}, fmt.Errorf("character %d is absent from the imported account", characterID)
	}
	for _, arc := range equipment.Arcs {
		if arc.EquippedCharacterID != characterID && (result.Character.ForkNetID == nil || arc.ID.Key() != result.Character.ForkNetID.Key()) {
			continue
		}
		arc.EquippedCharacterName = result.Character.Name
		result.Weapon = &arc
		break
	}
	for _, cartridge := range equipment.Cartridges {
		if cartridge.EquippedCharacterID == characterID {
			result.Cartridges = append(result.Cartridges, cartridge)
		}
	}
	for _, module := range equipment.Modules {
		if module.EquippedCharacterID == characterID {
			result.Modules = append(result.Modules, module)
		}
	}
	stats, err := loadCurrentCharacterStats(dataDir, &state, result.Character, result.Modules, result.Cartridges, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	result.Stats = stats
	return result, nil
}

func loadCurrentCharacterStats(dataDir string, state *decoded.State, character decoded.Character, modules []nte.Module, cartridges []nte.Cartridge, language string) (*optimizer.StatSummary, error) {
	service := NewOptimizerService(dataDir)
	profiles, err := service.loadCharacters()
	if err != nil {
		return nil, err
	}
	var profile scoring.Character
	for _, candidate := range profiles {
		if candidate.CharacterID == character.CharacterID {
			profile = candidate
			break
		}
	}
	if profile.CharacterID == 0 {
		return nil, nil
	}
	if baseStats, ok, err := service.characterBaseStats(character.CharacterID, character.Level, character.BreakthroughLevel); err != nil {
		return nil, err
	} else if ok {
		profile.BaseStats = baseStats
	}
	_, _, _, additional, _, err := service.resolveAccountContext(state, profile, language)
	if err != nil {
		return nil, err
	}
	catalogs, err := service.loadOptimizerCatalog()
	if err != nil {
		return nil, err
	}
	var cartridge *nte.Cartridge
	if len(cartridges) > 0 {
		cartridge = &cartridges[0]
	}
	currentSet := optimizer.SetDefinition{}
	if cartridge != nil {
		for _, definition := range catalogs.sets.Definitions {
			if definition.InventorySetID == cartridge.SetID {
				currentSet = definition
				break
			}
		}
	}
	matched := matchedRequiredGeometries(modules, currentSet.RequiredGeometries)
	stats := optimizer.BuildStatSummary(profile, modules, cartridge, currentSet, matched, additional)
	return &stats, nil
}
