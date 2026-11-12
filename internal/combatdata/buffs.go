package combatdata

import (
	"fmt"
	"math"
	"slices"
)

type BuffSelection struct {
	ID                  string   `json:"id"`
	Stacks              int      `json:"stacks"`
	Counter             *float64 `json:"counter,omitempty"`
	ConditionsConfirmed bool     `json:"conditions_confirmed"`
	DamageEffectIDs     []string `json:"damage_effect_ids,omitempty"`
}
type BuffValue struct {
	ID              string   `json:"id"`
	Target          Target   `json:"target"`
	Value           float64  `json:"value"`
	ModifierScope   string   `json:"modifier_scope,omitempty"`
	DamageEffectIDs []string `json:"damage_effect_ids,omitempty"`
	Duration        *float64 `json:"duration_seconds,omitempty"`
	ActiveWindow    *float64 `json:"active_window_seconds,omitempty"`
}

func buffCurve(values []float64, level int, independent bool) (float64, error) {
	if level > 0 {
		return Exact(values, level)
	}
	if independent && len(values) > 0 {
		for _, v := range values {
			if v != values[0] {
				return 0, fmt.Errorf("level-independent flag disagrees with values")
			}
		}
		return Exact(values, 1)
	}
	return 0, fmt.Errorf("missing imported skill level")
}

func (b Buff) Evaluate(selection BuffSelection, caster Actor, recipientID, activeID int) (BuffValue, error) {
	out := BuffValue{ID: b.ID, Target: b.Target, ModifierScope: b.ModifierScope}
	if caster.CharacterID != b.CharacterID {
		return out, fmt.Errorf("wrong buff caster")
	}
	if !selection.ConditionsConfirmed {
		return out, fmt.Errorf("manual conditions must be confirmed")
	}
	if b.Target.Operation != "add" {
		return out, fmt.Errorf("unsupported buff operation %s", b.Target.Operation)
	}
	if !b.Unlocked(caster) {
		return out, fmt.Errorf("buff unlock requirement not met")
	}
	if (b.Conditions.ActiveCharacterOnly || b.Conditions.RequiresCasterActive) && caster.CharacterID != activeID {
		return out, fmt.Errorf("requires active caster")
	}
	switch b.Recipients.Scope {
	case "caster":
		if recipientID != caster.CharacterID {
			return out, fmt.Errorf("caster-only buff")
		}
	case "team":
		if !b.Recipients.IncludeCaster && recipientID == caster.CharacterID {
			return out, fmt.Errorf("buff excludes caster")
		}
	case "enemy":
		return out, fmt.Errorf("enemy attribute projection is unsupported")
	default:
		return out, fmt.Errorf("unsupported recipient scope %s", b.Recipients.Scope)
	}
	out, err := b.magnitude(caster, selection)
	if err != nil {
		return out, err
	}
	if b.ModifierScope == "damage_evaluation" {
		if len(selection.DamageEffectIDs) == 0 {
			return out, fmt.Errorf("select eligible damage effects explicitly")
		}
		for _, id := range selection.DamageEffectIDs {
			if required := b.Conditions.RequiredDamageEffectID; required != "" && id != required {
				return out, fmt.Errorf("damage effect does not match required effect %s", required)
			}
			if len(b.DamageEffectIDs) > 0 && !slices.Contains(b.DamageEffectIDs, id) {
				return out, fmt.Errorf("damage effect does not match catalog filter")
			}
		}
		out.DamageEffectIDs = slices.Clone(selection.DamageEffectIDs)
	}
	return out, nil
}

func (b Buff) Unlocked(caster Actor) bool {
	if caster.CharacterID != b.CharacterID || caster.Awakening < b.Conditions.RequiredAwakening || (b.Conditions.CandidateAwakening != nil && caster.Awakening < *b.Conditions.CandidateAwakening) {
		return false
	}
	if b.Conditions.RequiresPassive && len(b.Conditions.Unlocks) > 0 {
		for _, requirement := range b.Conditions.Unlocks {
			if caster.Breakthrough >= requirement.Breakthrough && caster.Awakening >= requirement.Awakening {
				return true
			}
		}
		return false
	}
	return true
}

// PreviewMagnitude returns a conditional amount without activating the effect,
// choosing native states or applying it to panel/damage statistics.
func (b Buff) PreviewMagnitude(caster Actor) (BuffValue, error) {
	return b.magnitude(caster, BuffSelection{Stacks: 1})
}

func (b Buff) magnitude(caster Actor, selection BuffSelection) (BuffValue, error) {
	out := BuffValue{ID: b.ID, Target: b.Target, ModifierScope: b.ModifierScope}
	if caster.CharacterID != b.CharacterID {
		return out, fmt.Errorf("wrong buff caster")
	}
	if b.Target.Operation != "add" {
		return out, fmt.Errorf("unsupported buff operation %s", b.Target.Operation)
	}
	level := caster.Levels[b.LevelSource]
	curve := func(values []float64) (float64, error) { return buffCurve(values, level, b.LevelIndependent) }
	var value float64
	var err error
	switch b.Calculation.Kind {
	case "flat_stat":
		value, err = curve(b.Calculation.Values)
	case "source_attribute_ratio":
		if b.Calculation.Source != "caster" {
			return out, fmt.Errorf("unsupported source actor")
		}
		base, ok := caster.Stats[b.Calculation.SourcePropertyID]
		if !ok {
			return out, fmt.Errorf("missing source attribute %s", b.Calculation.SourcePropertyID)
		}
		var ratio float64
		ratio, err = curve(b.Calculation.Ratios)
		value = base * ratio
	case "counter_scaled_stat":
		if b.Calculation.Counter == nil || selection.Counter == nil {
			return out, fmt.Errorf("missing selected counter")
		}
		if err := b.Calculation.Counter.Validate(*selection.Counter); err != nil {
			return out, err
		}
		var perCount float64
		perCount, err = curve(b.Calculation.PerCount)
		value = perCount * *selection.Counter
	default:
		return out, fmt.Errorf("unsupported buff calculation %s", b.Calculation.Kind)
	}
	if err != nil {
		return out, err
	}
	if len(b.Calculation.Caps) > 0 {
		cap, err := curve(b.Calculation.Caps)
		if err != nil {
			return out, err
		}
		value = math.Min(cap, value)
	}
	if selection.Stacks < 1 {
		return out, fmt.Errorf("select at least one stack")
	}
	if b.Stacking.ValuesPerStack {
		if b.Stacking.SameCasterPolicy != "multiply_by_selected_stacks" || b.Stacking.Limit < selection.Stacks {
			return out, fmt.Errorf("unsupported stack count")
		}
		value *= float64(selection.Stacks)
	} else if selection.Stacks != 1 {
		return out, fmt.Errorf("catalog does not define per-stack values")
	}
	out.Value = value
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return out, fmt.Errorf("non-finite selected buff value")
	}
	if len(b.Durations) > 0 {
		value, err := curve(b.Durations)
		if err != nil {
			return out, err
		}
		out.Duration = &value
	}
	if len(b.ActiveWindows) > 0 {
		value, err := curve(b.ActiveWindows)
		if err != nil {
			return out, err
		}
		out.ActiveWindow = &value
	}
	return out, nil
}

// ValidateBuffSelection rejects alternatives instead of guessing which source
// wins. Replacement and shared-group effects must never be added together.
func ValidateBuffSelection(effects []Buff, selections []BuffSelection) error {
	definitions := map[string]Buff{}
	for _, effect := range effects {
		definitions[effect.ID] = effect
	}
	selected := map[string]bool{}
	groups := map[string]bool{}
	exclusive := map[string]bool{}
	randomOutcomes := map[string]string{}
	for _, choice := range selections {
		b, ok := definitions[choice.ID]
		if !ok {
			return fmt.Errorf("unknown buff %s", choice.ID)
		}
		if selected[choice.ID] {
			return fmt.Errorf("duplicate buff %s", choice.ID)
		}
		selected[choice.ID] = true
		if group := b.Conditions.ExclusiveGroup; group != "" {
			if exclusive[group] {
				return fmt.Errorf("mutually exclusive condition group %s", group)
			}
			exclusive[group] = true
		}
		if group := b.Conditions.RandomOutcomeGroup; group != "" {
			if outcome, ok := randomOutcomes[group]; ok && outcome != b.Conditions.SelectedRandomOutcome {
				return fmt.Errorf("mutually exclusive random outcomes in %s", group)
			}
			if b.Conditions.SelectedRandomOutcome == "" {
				return fmt.Errorf("missing selected random outcome")
			}
			randomOutcomes[group] = b.Conditions.SelectedRandomOutcome
		}
		if b.Stacking.GroupID != "" {
			if groups[b.Stacking.GroupID] {
				return fmt.Errorf("mutually exclusive buff group %s", b.Stacking.GroupID)
			}
			groups[b.Stacking.GroupID] = true
		}
	}
	for id := range selected {
		if supersedes := definitions[id].Supersedes; supersedes != "" && selected[supersedes] {
			return fmt.Errorf("%s replaces %s", id, supersedes)
		}
	}
	return nil
}
