package app

import (
	"fmt"
	"math"
	"sort"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/housing"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

// CharacterGameState is a read-only snapshot of what the latest account import
// reported as equipped in game. It is deliberately separate from LocalBuild.
type CharacterGameState struct {
	Character               decoded.Character          `json:"character"`
	ObservedPanelStats      *decoded.CharacterStats    `json:"observed_panel_stats,omitempty"`
	PossiblePanelBonuses    []PanelBonus               `json:"possible_panel_bonuses,omitempty"`
	Weapon                  *EquipmentCatalogArc       `json:"weapon,omitempty"`
	Cartridges              []nte.Cartridge            `json:"cartridges"`
	Modules                 []nte.Module               `json:"modules"`
	Stats                   *optimizer.StatSummary     `json:"stats,omitempty"`
	PotentialHousingEffects []housing.InferredModifier `json:"potential_housing_effects,omitempty"`
	ImportedAt              string                     `json:"imported_at,omitempty"`
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
	stats, err := loadCurrentCharacterStats(dataDir, &state, result.Character, result.Modules, result.Cartridges, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	result.Stats = stats
	result.PossiblePanelBonuses = inferPossiblePanelBonuses(result.ObservedPanelStats, stats)
	result.PotentialHousingEffects, err = inferHousingEffects(dataDir, &state, equipment, language)
	if err != nil {
		return CharacterGameState{}, err
	}
	return result, nil
}

func inferPossiblePanelBonuses(panel *decoded.CharacterStats, calculated *optimizer.StatSummary) []PanelBonus {
	if panel == nil || calculated == nil {
		return nil
	}
	type panelValue struct {
		property  string
		value     float64
		available bool
		rounded   bool
	}
	values := []panelValue{
		{property: "HPFinal", value: panel.MaxHP, available: panel.MaxHP > 0, rounded: true},
		{property: "AtkFinal", value: valueOrZero(panel.Attack), available: panel.Attack != nil, rounded: true},
		{property: "DefFinal", value: valueOrZero(panel.Defense), available: panel.Defense != nil, rounded: true},
		{property: "Endurance", value: valueOrZero(panel.Endurance), available: panel.Endurance != nil, rounded: true},
		{property: "CritBase", value: valueOrZero(panel.CritRate), available: panel.CritRate != nil},
		{property: "CritDamageBase", value: valueOrZero(panel.CritDamage), available: panel.CritDamage != nil},
		{property: "ChargeGetEfficiencyBase", value: valueOrZero(panel.ChargeEfficiency), available: panel.ChargeEfficiency != nil},
		{property: "MagBase", value: valueOrZero(panel.CycleIntensity), available: panel.CycleIntensity != nil, rounded: true},
		{property: "UnbalIntensityBase", value: valueOrZero(panel.BreakIntensity), available: panel.BreakIntensity != nil, rounded: true},
		{property: "DamageUpGeneralBase", value: valueOrZero(panel.UniversalDMGBonus), available: panel.UniversalDMGBonus != nil},
		{property: "ElementalDMGBonus", value: valueOrZero(panel.ElementalDMGBonus), available: panel.ElementalDMGBonus != nil},
	}
	bonuses := make([]PanelBonus, 0)
	for _, observed := range values {
		calculatedValue, exists := calculated.Derived[observed.property]
		if !observed.available || !exists {
			continue
		}
		difference := observed.value - calculatedValue
		if observed.rounded {
			// Integer-valued panel stats can differ from the model by a fraction
			// because the game and optimizer round at different stages.
			difference = math.Round(difference)
		} else {
			// Ignore float serialization noise while preserving small percent
			// bonuses such as 0.4% (0.004 in the stat model).
			difference = math.Round(difference*10000) / 10000
			if difference <= 0.001 {
				continue
			}
		}
		if difference <= 0 {
			continue
		}
		bonuses = append(bonuses, PanelBonus{
			PropertyID: observed.property, ObservedValue: observed.value,
			CalculatedValue: calculatedValue, Difference: difference,
		})
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

func inferHousingEffects(dataDir string, state *decoded.State, equipment EquipmentCatalog, language string) ([]housing.InferredModifier, error) {
	catalog, err := housing.Load(datafiles.New(dataDir).SpecialFurnitureModifiers())
	if err != nil {
		return nil, fmt.Errorf("load special-furniture modifiers: %w", err)
	}
	observations := make([]housing.Observation, 0, len(state.Characters))
	for _, character := range state.Characters {
		if !character.ObservedAtLogin.Loaded || !hasObservedPanelStats(character.Stats) {
			continue
		}
		modules := make([]nte.Module, 0)
		for _, module := range equipment.Modules {
			if module.EquippedCharacterID == character.CharacterID {
				modules = append(modules, module)
			}
		}
		cartridges := make([]nte.Cartridge, 0, 1)
		for _, cartridge := range equipment.Cartridges {
			if cartridge.EquippedCharacterID == character.CharacterID {
				cartridges = append(cartridges, cartridge)
			}
		}
		stats, err := loadCurrentCharacterStats(dataDir, state, character, modules, cartridges, language)
		if err != nil {
			return nil, err
		}
		if stats == nil {
			continue
		}
		panel := map[string]float64{}
		if character.Stats.Attack != nil {
			panel["AtkFinal"] = *character.Stats.Attack
		}
		if character.Stats.Defense != nil {
			panel["DefFinal"] = *character.Stats.Defense
		}
		if character.Stats.CritDamage != nil {
			panel["CritDamageBase"] = *character.Stats.CritDamage
		}
		calculated := map[string]float64{}
		for _, property := range []string{"AtkFinal", "DefFinal", "CritDamageBase"} {
			if value, ok := stats.Derived[property]; ok {
				calculated[property] = value
			}
		}
		observations = append(observations, housing.Observation{
			CharacterID: character.CharacterID,
			Loaded:      character.ObservedAtLogin.Loaded,
			Panel:       panel,
			Calculated:  calculated,
		})
	}
	return housing.InferShared(catalog, observations), nil
}

func hasObservedPanelStats(stats decoded.CharacterStats) bool {
	return stats.Source != "" || stats.Attack != nil || stats.Defense != nil || stats.Endurance != nil ||
		stats.CritRate != nil || stats.CritDamage != nil || stats.ChargeEfficiency != nil ||
		stats.CycleIntensity != nil || stats.BreakIntensity != nil || stats.UniversalDMGBonus != nil ||
		stats.ElementalDMGBonus != nil
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
