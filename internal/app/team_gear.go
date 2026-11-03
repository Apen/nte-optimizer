package app

import (
	"slices"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

type teamGear struct {
	arc       *decoded.Weapon
	cartridge *nte.Cartridge
	modules   []nte.Module
	saved     bool
}

// Resolve one complete equipment source for both the builder preview and search.
func resolveTeamGear(character decoded.Character, build LocalBuild, inv nte.Inventory, weapons []decoded.Weapon) teamGear {
	findArc := func(id string) *decoded.Weapon {
		if id == "" {
			return nil
		}
		i := slices.IndexFunc(weapons, func(w decoded.Weapon) bool { return w.ID.Key() == id })
		if i < 0 {
			return nil
		}
		return &weapons[i]
	}
	if build.CharacterID == character.CharacterID {
		saved := teamGear{arc: findArc(build.ArcID), saved: true}
		if build.ArcID == "" || saved.arc != nil {
			for i := range inv.Cartridges {
				if inv.Cartridges[i].LocalID == build.CartridgeID {
					saved.cartridge = &inv.Cartridges[i]
					break
				}
			}
			for _, id := range build.ModuleIDs {
				i := slices.IndexFunc(inv.Modules, func(m nte.Module) bool { return m.LocalID == id })
				if i >= 0 {
					saved.modules = append(saved.modules, inv.Modules[i])
				}
			}
			if (build.CartridgeID == "" || saved.cartridge != nil) && len(saved.modules) == len(build.ModuleIDs) {
				return saved
			}
		}
	}
	var scanned teamGear
	if character.ForkNetID != nil {
		scanned.arc = findArc(character.ForkNetID.Key())
	}
	for i := range inv.Cartridges {
		if inv.Cartridges[i].EquippedCharacterID == character.CharacterID {
			scanned.cartridge = &inv.Cartridges[i]
			break
		}
	}
	for _, module := range inv.Modules {
		if module.EquippedCharacterID == character.CharacterID {
			scanned.modules = append(scanned.modules, module)
		}
	}
	return scanned
}
