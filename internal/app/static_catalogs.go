package app

import (
	"fmt"
	"maps"
	"nte-optimizer/internal/apperrors"
	"sync"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/equipment"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

// Static catalogs belong to one service/data directory. Account observations
// are never cached here; imports replace them independently. Create a new
// service after changing the installed data generation.
type staticCatalogs struct {
	traitOnce     sync.Once
	traits        map[int]*scoring.ConsoleTrait
	traitErr      error
	equipmentOnce sync.Once
	equipment     nte.EquipmentDefinitions
	equipmentErr  error
	profileOnce   sync.Once
	profiles      map[string]scoring.Character
	profileErr    error
	arcOnce       sync.Once
	arcs          decoded.ForkCatalog
	arcErr        error
	buffOnce      sync.Once
	buffs         []BuffOption
	bonds         map[int][]nte.Stat
	buffErr       error
	localeMu      sync.Mutex
	locales       map[string]localeResult
}

type localeResult struct {
	catalog ntelocale.Catalog
	err     error
}

func (s OptimizerService) loadLocale(language string) (ntelocale.Catalog, error) {
	if s.static == nil {
		return ntelocale.Load(s.DataDir, language)
	}
	language = ntelocale.Normalize(language)
	s.static.localeMu.Lock()
	defer s.static.localeMu.Unlock()
	if s.static.locales == nil {
		s.static.locales = map[string]localeResult{}
	}
	if result, ok := s.static.locales[language]; ok {
		return result.catalog, result.err
	}
	catalog, err := ntelocale.Load(s.DataDir, language)
	s.static.locales[language] = localeResult{catalog, err}
	return catalog, err
}

func (s OptimizerService) loadArcs() (decoded.ForkCatalog, error) {
	if s.static == nil {
		return decoded.LoadForkCatalog(datafiles.New(s.DataDir).Arcs())
	}
	s.static.arcOnce.Do(func() { s.static.arcs, s.static.arcErr = decoded.LoadForkCatalog(datafiles.New(s.DataDir).Arcs()) })
	return s.static.arcs, s.static.arcErr
}

func (s OptimizerService) accountBuffSources(state *decoded.State, characterID int, calibration map[string]float64) (map[string][]nte.Stat, error) {
	var options []BuffOption
	var bonds map[int][]nte.Stat
	var err error
	if s.static == nil {
		options, bonds, err = buffOptions(s.DataDir)
	} else {
		s.static.buffOnce.Do(func() { s.static.buffs, s.static.bonds, s.static.buffErr = buffOptions(s.DataDir) })
		options, bonds, err = s.static.buffs, s.static.bonds, s.static.buffErr
	}
	if err != nil {
		return nil, err
	}
	return selectedAccountBuffSources(state, options, bonds, characterID, calibration)
}

func (s OptimizerService) loadCharacters() (map[string]scoring.Character, error) {
	if s.static == nil {
		return s.readCharacters()
	}
	s.static.profileOnce.Do(func() { s.static.profiles, s.static.profileErr = s.readCharacters() })
	if s.static.profileErr != nil {
		return nil, s.static.profileErr
	}
	result := make(map[string]scoring.Character, len(s.static.profiles))
	for id, profile := range s.static.profiles {
		profile.BaseStats = maps.Clone(profile.BaseStats)
		profile.CurrentStats = maps.Clone(profile.CurrentStats)
		profile.Weights = maps.Clone(profile.Weights)
		profile.MainWeights = maps.Clone(profile.MainWeights)
		profile.SubWeights = maps.Clone(profile.SubWeights)
		profile.Caps = maps.Clone(profile.Caps)
		profile.MainStats = append([]string(nil), profile.MainStats...)
		profile.PreferredSets = append([]scoring.SetPreference(nil), profile.PreferredSets...)
		if profile.ConsoleTrait != nil {
			trait := *profile.ConsoleTrait
			profile.ConsoleTrait = &trait
		}
		result[id] = profile
	}
	return result, nil
}

func (s OptimizerService) normalizeInventory(inventory *nte.Inventory) error {
	needed := false
	for _, item := range inventory.Cartridges {
		needed = needed || item.GameItemID != ""
	}
	if !needed {
		return nil
	}
	var definitions nte.EquipmentDefinitions
	var err error
	if s.static == nil {
		definitions, err = equipment.Load(s.DataDir)
	} else {
		s.static.equipmentOnce.Do(func() { s.static.equipment, s.static.equipmentErr = equipment.Load(s.DataDir) })
		definitions, err = s.static.equipment, s.static.equipmentErr
	}
	if err != nil {
		return err
	}
	for _, item := range inventory.Cartridges {
		if item.GameItemID == "" {
			continue
		}
		if definition, ok := definitions[item.GameItemID]; !ok || definition.Kind != "core" {
			return apperrors.New("data_missing", fmt.Errorf("equipment/decode.json: missing core items[%q]", item.GameItemID))
		}
	}
	inventory.Normalize(definitions)
	return nil
}

func (s OptimizerService) loadConsoleTraits() (map[int]*scoring.ConsoleTrait, error) {
	if s.static == nil {
		return s.readConsoleTraits()
	}
	s.static.traitOnce.Do(func() { s.static.traits, s.static.traitErr = s.readConsoleTraits() })
	return s.static.traits, s.static.traitErr
}

func (s OptimizerService) readConsoleTraits() (map[int]*scoring.ConsoleTrait, error) {
	var file struct {
		Traits []struct {
			CharacterID    int     `json:"character_id"`
			Area           int     `json:"area"`
			PropertyID     string  `json:"property_id"`
			ValuePerModule float64 `json:"value_per_module"`
		} `json:"traits"`
	}
	if _, err := readOptionalJSON(datafiles.New(s.DataDir).ConsoleTraits(), &file); err != nil {
		return nil, err
	}
	result := map[int]*scoring.ConsoleTrait{}
	for _, trait := range file.Traits {
		result[trait.CharacterID] = &scoring.ConsoleTrait{Area: trait.Area, PropertyID: trait.PropertyID, ValuePerModule: trait.ValuePerModule}
	}
	return result, nil
}

// Reconstruct character facts independently of strategy weights or selection.
// Legacy profile corrections remain explicit inputs until their provenance is
// resolved in game data; conflicting corrections must never depend on map order.
func (s OptimizerService) characterInputs(character decoded.Character) (scoring.Character, error) {
	base, err := s.requiredCharacterBaseStats(character.CharacterID, character.Level, character.BreakthroughLevel)
	if err != nil {
		return scoring.Character{}, err
	}
	traits, err := s.loadConsoleTraits()
	if err != nil {
		return scoring.Character{}, err
	}
	result := scoring.Character{CharacterID: character.CharacterID, BaseStats: base}
	if trait := traits[character.CharacterID]; trait != nil {
		copy := *trait
		result.ConsoleTrait = &copy
	}
	profiles, err := s.loadCharacters()
	if err != nil {
		return scoring.Character{}, err
	}
	found := false
	for _, profile := range profiles {
		if profile.CharacterID == character.CharacterID {
			if found && !maps.Equal(result.CurrentStats, profile.CurrentStats) {
				return scoring.Character{}, fmt.Errorf("conflicting recommendation stat corrections for character %d", character.CharacterID)
			}
			result.CurrentStats = maps.Clone(profile.CurrentStats)
			found = true
		}
	}
	return result, nil
}
