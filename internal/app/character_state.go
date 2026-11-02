package app

import (
	"fmt"
	"math"
	"sort"

	"nte-optimizer/internal/apperrors"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

// CharacterGameState is a read-only snapshot of what the latest account import
// reported as equipped in game. It is deliberately separate from LocalBuild.
type CharacterGameState struct {
	Character            decoded.Character       `json:"character"`
	ObservedPanelStats   *decoded.CharacterStats `json:"observed_panel_stats,omitempty"`
	PossiblePanelBonuses []PanelBonus            `json:"possible_panel_bonuses,omitempty"`
	Weapon               *EquipmentCatalogArc    `json:"weapon,omitempty"`
	Cartridges           []nte.Cartridge         `json:"cartridges"`
	Modules              []nte.Module            `json:"modules"`
	Stats                *optimizer.StatSummary  `json:"stats,omitempty"`
	ImportedAt           string                  `json:"imported_at,omitempty"`
}

// PanelBonus is a positive difference between the scanned panel and the
// optimizer's current model. It is diagnostic only; the source is unknown.
type PanelBonus struct {
	PropertyID      string  `json:"property_id"`
	ObservedValue   float64 `json:"observed_value"`
	CalculatedValue float64 `json:"calculated_value"`
	Difference      float64 `json:"difference"`
}

func LoadCharacterGameState(projectDir, dataDir string, characterID int, language string) (CharacterGameState, error) {
	inventory, state, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return CharacterGameState{}, err
	}
	if !loaded {
		return CharacterGameState{}, apperrors.New("account_required", fmt.Errorf("no account has been imported"))
	}
	var overrides struct {
		Characters map[int]map[string]float64 `json:"characters"`
	}
	if loaded, err := readOptionalJSON(workspaceFile(projectDir, "account_overrides.json"), &overrides); err != nil {
		return CharacterGameState{}, err
	} else if loaded {
		state.PanelOverrides = overrides.Characters
	}
	service := NewOptimizerService(dataDir)
	equipment, err := service.equipmentCatalog(inventory, state, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	catalog, err := service.loadLocale(language)
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
		if hasObservedPanelStats(character.Stats) {
			panelStats := character.Stats
			result.ObservedPanelStats = &panelStats
		}
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
	stats, err := service.currentCharacterStats(&state, result.Character, result.Modules, result.Cartridges, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	result.Stats = stats
	result.PossiblePanelBonuses = inferPossiblePanelBonuses(result.ObservedPanelStats, stats)
	return result, nil
}

func inferPossiblePanelBonuses(panel *decoded.CharacterStats, calculated *optimizer.StatSummary) []PanelBonus {
	if panel == nil {
		return nil
	}
	bonuses := []PanelBonus{}
	for _, row := range compareObservedPanel(*panel, calculated) {
		difference := row.Difference
		switch row.PropertyID {
		case "HPFinal", "AtkFinal", "DefFinal", "Endurance", "MagBase", "UnbalIntensityBase":
			difference = math.Round(difference)
		default:
			difference = math.Round(difference*10000) / 10000
			if difference <= .001 {
				continue
			}
		}
		if difference > 0 {
			bonuses = append(bonuses, PanelBonus{PropertyID: row.PropertyID, ObservedValue: row.Observed, CalculatedValue: row.Calculated, Difference: difference})
		}
	}
	sort.Slice(bonuses, func(i, j int) bool { return bonuses[i].PropertyID < bonuses[j].PropertyID })
	return bonuses
}
func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func hasObservedPanelStats(stats decoded.CharacterStats) bool {
	return stats.Source != "" || stats.Attack != nil || stats.Defense != nil || stats.Endurance != nil ||
		stats.CritRate != nil || stats.CritDamage != nil || stats.ChargeEfficiency != nil ||
		stats.CycleIntensity != nil || stats.BreakIntensity != nil || stats.UniversalDMGBonus != nil ||
		stats.ElementalDMGBonus != nil
}

func loadCurrentCharacterStats(dataDir string, state *decoded.State, character decoded.Character, modules []nte.Module, cartridges []nte.Cartridge, language string) (*optimizer.StatSummary, error) {
	return NewOptimizerService(dataDir).currentCharacterStats(state, character, modules, cartridges, language)
}

func (service OptimizerService) currentCharacterStats(state *decoded.State, character decoded.Character, modules []nte.Module, cartridges []nte.Cartridge, language string) (*optimizer.StatSummary, error) {
	profile, err := service.characterInputs(character)
	if err != nil {
		return nil, err
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
