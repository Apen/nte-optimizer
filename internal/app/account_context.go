package app

import (
	"fmt"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func (s OptimizerService) resolveAccountContext(state *decoded.State, profile scoring.Character, language string) (*decoded.Character, *decoded.Weapon, string, map[string][]nte.Stat, map[int]string, error) {
	additional := map[string][]nte.Stat{}
	characterNames := map[int]string{}
	if state == nil {
		return nil, nil, "", additional, characterNames, nil
	}
	localizedNames, err := ntelocale.Load(s.DataDir, language)
	if err != nil {
		return nil, nil, "", nil, nil, fmt.Errorf("load locale catalog: %w", err)
	}
	for _, character := range state.Characters {
		characterNames[character.CharacterID] = localizedNames.CharacterName(character.CharacterID, character.Name)
	}
	var currentCharacter *decoded.Character
	for i := range state.Characters {
		if state.Characters[i].CharacterID == profile.CharacterID {
			currentCharacter = &state.Characters[i]
			currentCharacter.Name = localizedNames.CharacterName(currentCharacter.CharacterID, currentCharacter.Name)
			break
		}
	}
	var currentWeapon *decoded.Weapon
	if currentCharacter != nil && currentCharacter.ForkNetID != nil {
		for i := range state.Weapons {
			if state.Weapons[i].ID.Key() == currentCharacter.ForkNetID.Key() {
				currentWeapon = &state.Weapons[i]
				break
			}
		}
	}
	weaponNote := ""
	if currentWeapon != nil {
		currentWeapon.Name = cleanArcName(localizedNames.ItemName(currentWeapon.ForkID, currentWeapon.Name))
		catalog, err := decoded.LoadForkCatalog(datafiles.New(s.DataDir).Arcs())
		if err != nil {
			return nil, nil, "", nil, nil, err
		}
		panel, permanent, conditional, note := catalog.Stats(*currentWeapon)
		additional["weapon"], additional["weapon_permanent"], additional["weapon_conditional"], weaponNote = panel, permanent, conditional, note
	}
	if values := state.PanelOverrides[profile.CharacterID]; len(values) > 0 {
		additional["panel_calibration"] = statsFromValues(values)
	}
	return currentCharacter, currentWeapon, weaponNote, additional, characterNames, nil
}
