package combatdata

import (
	"errors"
	"fmt"
	"reflect"

	"nte-optimizer/internal/damage"
)

var ErrForeignDamageEffect = errors.New("sheet references another character's damage effect")

// ResolveDamageEffect requires an exact executable binding in the same character.
// A numerically resolved sheet parameter alone never establishes that binding.
func (p Parameter) ResolveDamageEffect(characterID string, catalog damage.Catalog) (damage.TieredInstance, error) {
	if p.SourceKind != "damage_effect" || p.Status == "unused_source_slot" || (p.DamageCalculation != nil && !p.DamageCalculation.Supported) {
		return damage.TieredInstance{}, fmt.Errorf("parameter %s has no executable damage binding", p.SourceID)
	}
	for _, effect := range catalog.Characters[characterID].Instances {
		if effect.ID != p.SourceID {
			continue
		}
		if !reflect.DeepEqual(p.Values, effect.Coefficients) || p.Scaling != string(effect.Scaling) {
			return damage.TieredInstance{}, fmt.Errorf("sheet and damage coefficients or scaling disagree: %s", p.SourceID)
		}
		return effect, nil
	}
	for owner, character := range catalog.Characters {
		for _, effect := range character.Instances {
			if effect.ID == p.SourceID {
				return damage.TieredInstance{}, fmt.Errorf("%w: %s -> %s (%s)", ErrForeignDamageEffect, characterID, owner, p.SourceID)
			}
		}
	}
	return damage.TieredInstance{}, fmt.Errorf("no damage effect for character %s: %s", characterID, p.SourceID)
}
