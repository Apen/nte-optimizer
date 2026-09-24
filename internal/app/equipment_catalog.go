package app

import (
	"fmt"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

// EquipmentCatalog is the read-only account inventory displayed by the UI.
type EquipmentCatalog struct {
	Modules    []nte.Module          `json:"modules"`
	Cartridges []nte.Cartridge       `json:"cartridges"`
	Arcs       []EquipmentCatalogArc `json:"arcs"`
	Resources  []decoded.Resource    `json:"resources"`
}

type EquipmentCatalogArc struct {
	decoded.Weapon
	EquippedCharacterName string                   `json:"equipped_character_name,omitempty"`
	CompatibleCharacters  []ArcCompatibleCharacter `json:"compatible_characters,omitempty"`
}

type ArcCompatibleCharacter struct {
	CharacterID int    `json:"character_id"`
	Name        string `json:"name"`
}

func LoadEquipmentCatalog(projectDir, dataDir, language string) (EquipmentCatalog, error) {
	inventory, state, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return EquipmentCatalog{}, err
	}
	if !loaded {
		return EquipmentCatalog{Modules: []nte.Module{}, Cartridges: []nte.Cartridge{}, Arcs: []EquipmentCatalogArc{}, Resources: []decoded.Resource{}}, nil
	}
	catalog, err := ntelocale.Load(dataDir, language)
	if err != nil {
		return EquipmentCatalog{}, fmt.Errorf("load locale catalog: %w", err)
	}
	layout := datafiles.New(dataDir)
	compatibility, err := decoded.LoadArcCompatibilityCatalog(layout.DecodeCatalog("characters.json"), layout.Arcs())
	if err != nil {
		return EquipmentCatalog{}, err
	}
	sets, err := optimizer.LoadSetCatalog(layout.Sets())
	if err != nil {
		return EquipmentCatalog{}, err
	}
	setNames := make(map[string]string, len(sets.Definitions))
	for _, definition := range sets.Definitions {
		setNames[definition.InventorySetID] = catalog.SetName(definition.ID, definition.ID)
	}
	for index := range inventory.Modules {
		if name := setNames[inventory.Modules[index].SetID]; name != "" {
			inventory.Modules[index].SetName = name
		}
	}
	for index := range inventory.Cartridges {
		if name := setNames[inventory.Cartridges[index].SetID]; name != "" {
			inventory.Cartridges[index].SetName = name
		}
	}
	characterNames := make(map[int]string, len(state.Characters))
	for _, character := range state.Characters {
		characterNames[character.CharacterID] = cleanCharacterName(catalog.CharacterName(character.CharacterID, character.Name))
	}
	arcs := make([]EquipmentCatalogArc, 0, len(state.Weapons))
	for _, weapon := range state.Weapons {
		weapon.Name = cleanArcName(catalog.ItemName(weapon.ForkID, weapon.Name))
		compatibleCharacters := make([]ArcCompatibleCharacter, 0)
		for _, characterID := range compatibility.CompatibleCharacterIDs(weapon.ForkID) {
			compatibleCharacters = append(compatibleCharacters, ArcCompatibleCharacter{
				CharacterID: characterID,
				Name:        cleanCharacterName(catalog.CharacterName(characterID, "")),
			})
		}
		arcs = append(arcs, EquipmentCatalogArc{Weapon: weapon, EquippedCharacterName: characterNames[weapon.EquippedCharacterID], CompatibleCharacters: compatibleCharacters})
	}
	resources := append([]decoded.Resource(nil), state.Resources...)
	for index := range resources {
		resources[index].Name = catalog.ResourceName(resources[index].ItemID, resources[index].Name)
	}
	return EquipmentCatalog{Modules: inventory.Modules, Cartridges: inventory.Cartridges, Arcs: arcs, Resources: resources}, nil
}
