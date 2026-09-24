package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

func (s OptimizerService) OptimizeFlexibleProject(ctx context.Context, projectDir, profileID string, includeEquipped bool, language, mode string, tunings map[string]GoalTuning) (OptimizationResult, error) {
	return s.OptimizeFlexibleProjectWithArc(ctx, projectDir, profileID, includeEquipped, language, mode, tunings, "")
}

func (s OptimizerService) OptimizeFlexibleProjectWithArc(ctx context.Context, projectDir, profileID string, includeEquipped bool, language, mode string, tunings map[string]GoalTuning, arcForkID string) (OptimizationResult, error) {
	goals := make(map[string]float64, len(tunings))
	for propertyID, tuning := range tunings {
		goals[propertyID] = tuning.Target
	}
	return s.optimizeProjectWithArc(ctx, projectDir, profileID, includeEquipped, language, mode, goals, tunings, arcForkID)
}

func (s OptimizerService) optimizeProjectWithArc(ctx context.Context, projectDir, profileID string, includeEquipped bool, language, mode string, goals map[string]float64, tunings map[string]GoalTuning, arcForkID string) (OptimizationResult, error) {
	inventory, state, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return OptimizationResult{}, err
	}
	if !loaded {
		return s.optimizeTunedWithArc(ctx, inventory, profileID, nil, includeEquipped, language, mode, goals, tunings, arcForkID)
	}
	var overrides struct {
		Characters map[int]map[string]float64 `json:"characters"`
	}
	loaded, err = readOptionalJSON(workspaceFile(projectDir, "account_overrides.json"), &overrides)
	if err != nil {
		return OptimizationResult{}, err
	}
	if loaded {
		state.PanelOverrides = overrides.Characters
	}
	return s.optimizeTunedWithArc(ctx, inventory, profileID, &state, includeEquipped, language, mode, goals, tunings, arcForkID)
}

func (s OptimizerService) loadCharacters() (map[string]scoring.Character, error) {
	characters := map[string]scoring.Character{}
	targetsDir := datafiles.New(s.DataDir).TargetsDir()
	entries, err := os.ReadDir(targetsDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		recommendation, err := target.Read(filepath.Join(targetsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("load recommendation %s: %w", entry.Name(), err)
		}
		for id, profile := range BuildRecommendationProfiles(recommendation) {
			if _, duplicate := characters[id]; duplicate {
				return nil, fmt.Errorf("duplicate recommendation profile %q", id)
			}
			characters[id] = profile
		}
	}
	var traits struct {
		Traits []struct {
			CharacterID    int     `json:"character_id"`
			Area           int     `json:"area"`
			PropertyID     string  `json:"property_id"`
			ValuePerModule float64 `json:"value_per_module"`
		} `json:"traits"`
	}
	loaded, err := readOptionalJSON(datafiles.New(s.DataDir).ConsoleTraits(), &traits)
	if err != nil {
		return nil, err
	}
	if loaded {
		byCharacter := map[int]*scoring.ConsoleTrait{}
		for _, trait := range traits.Traits {
			byCharacter[trait.CharacterID] = &scoring.ConsoleTrait{Area: trait.Area, PropertyID: trait.PropertyID, ValuePerModule: trait.ValuePerModule}
		}
		for id, character := range characters {
			character.ConsoleTrait = byCharacter[character.CharacterID]
			characters[id] = character
		}
	}
	for id, character := range characters {
		if character.GridID == "" && character.CharacterID > 0 {
			character.GridID = fmt.Sprintf("character_%d", character.CharacterID)
		}
		characters[id] = character
	}
	return characters, nil
}

type characterBaseStatsCatalog struct {
	SchemaVersion int                                      `json:"schema_version"`
	Stats         map[string]map[string]map[string]float64 `json:"stats"`
}

type characterBaseStatsCache struct {
	once    sync.Once
	catalog characterBaseStatsCatalog
	err     error
}

func (s OptimizerService) characterBaseStats(characterID, level, breakthrough int) (map[string]float64, bool, error) {
	catalog, err := s.loadCharacterBaseStats()
	if err != nil {
		return nil, false, err
	}
	if catalog.SchemaVersion != 1 {
		return nil, false, fmt.Errorf("data/game/characters/base_stats.json schema_version must be 1")
	}
	stats, ok := catalog.Stats[fmt.Sprint(characterID)][fmt.Sprintf("%d:%d", level, breakthrough)]
	return cloneWeights(stats), ok, nil
}

func (s OptimizerService) loadCharacterBaseStats() (characterBaseStatsCatalog, error) {
	load := func() (characterBaseStatsCatalog, error) {
		var catalog characterBaseStatsCatalog
		err := readJSON(datafiles.New(s.DataDir).CharacterBaseStats(), &catalog)
		return catalog, err
	}
	if s.baseStats == nil {
		return load()
	}
	s.baseStats.once.Do(func() {
		s.baseStats.catalog, s.baseStats.err = load()
	})
	return s.baseStats.catalog, s.baseStats.err
}

func sortProfileSummaries(profiles []ProfileSummary) {
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
}
