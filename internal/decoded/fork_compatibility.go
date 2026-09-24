package decoded

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
)

type ArcCompatibilityCatalog struct {
	characterGroupTypes map[int]string
	forkGroupTypes      map[string]string
}

func LoadArcCompatibilityCatalog(characterPath, arcsPath string) (ArcCompatibilityCatalog, error) {
	var characters struct {
		Characters map[string]struct {
			GroupType string `json:"character_group_type"`
		} `json:"characters"`
	}
	if err := readCompatibilityCatalog(characterPath, &characters); err != nil {
		return ArcCompatibilityCatalog{}, fmt.Errorf("load character Arc compatibility: %w", err)
	}
	var forks struct {
		Forks map[string]struct {
			GroupType string `json:"apply_group_type"`
		} `json:"forks"`
	}
	if err := readCompatibilityCatalog(arcsPath, &forks); err != nil {
		return ArcCompatibilityCatalog{}, fmt.Errorf("load Arc compatibility: %w", err)
	}
	if len(characters.Characters) == 0 || len(forks.Forks) == 0 {
		return ArcCompatibilityCatalog{}, fmt.Errorf("arc compatibility catalogs are empty")
	}

	catalog := ArcCompatibilityCatalog{
		characterGroupTypes: make(map[int]string, len(characters.Characters)),
		forkGroupTypes:      make(map[string]string, len(forks.Forks)),
	}
	for id, character := range characters.Characters {
		characterID, err := strconv.Atoi(id)
		if err != nil || characterID <= 0 {
			return ArcCompatibilityCatalog{}, fmt.Errorf("invalid character ID in compatibility catalog: %q", id)
		}
		if character.GroupType == "" {
			return ArcCompatibilityCatalog{}, fmt.Errorf("character %s has no compatibility group", id)
		}
		catalog.characterGroupTypes[characterID] = character.GroupType
	}
	for id, fork := range forks.Forks {
		if fork.GroupType == "" {
			return ArcCompatibilityCatalog{}, fmt.Errorf("arc %s has no compatibility group", id)
		}
		catalog.forkGroupTypes[id] = fork.GroupType
	}
	return catalog, nil
}

func (c ArcCompatibilityCatalog) CompatibleCharacterIDs(forkID string) []int {
	groupType, ok := c.forkGroupTypes[forkID]
	if !ok {
		return nil
	}
	ids := make([]int, 0)
	for id, characterGroupType := range c.characterGroupTypes {
		if characterGroupType == groupType {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

func readCompatibilityCatalog(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}
	return nil
}
