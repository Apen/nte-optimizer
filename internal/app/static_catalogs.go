package app

import (
	"maps"
	"nte-optimizer/internal/scoring"
	"sync"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/equipment"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
)

// Static catalogs belong to one service/data directory. Account observations
// are never cached here; imports replace them independently. Create a new
// service after changing the installed data generation.
type staticCatalogs struct {
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
	inventory.Normalize(definitions)
	return nil
}
