package decoded

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"nte-optimizer/internal/nte"
)

// Conditional effects describe selected scenarios, not additional panel stats.
type ForkConditionalEffect struct {
	ID      string `json:"id"`
	Scope   string `json:"scope"`
	Trigger string `json:"trigger"`
	ForkDamageFilter
	DurationParameter   string `json:"duration_parameter"`
	StackInput          string `json:"stack_input"`
	MaxStacks           int    `json:"max_stacks"`
	FixedStacks         int    `json:"fixed_stacks,omitempty"`
	MixedSourcePolicy   string `json:"mixed_source_policy"`
	ConfirmsTrigger     string `json:"confirms_trigger"`
	SupersededByTrigger string `json:"superseded_by_trigger,omitempty"`
	Parameter           struct {
		NameID      string `json:"name_id"`
		LevelSource string `json:"level_source"`
	} `json:"parameter"`
	Target struct {
		PropertyID           string `json:"property_id"`
		ProjectionPropertyID string `json:"projection_property_id"`
		Operation            string `json:"operation"`
		Percent              bool   `json:"percent"`
	} `json:"target"`
	ManualSelectionRequired      bool              `json:"manual_selection_required"`
	NativeExecutionSupported     bool              `json:"native_execution_supported"`
	AutomaticActivationSupported bool              `json:"automatic_activation_supported"`
	LegacyConditionalProjection  bool              `json:"legacy_conditional_projection"`
	Status                       string            `json:"status"`
	Provenance                   []json.RawMessage `json:"provenance"`
}

// SelectableStacks is the upper bound on user input. Fixed stack effects
// use a single confirmation and take their count from the Arc catalog.
func (effect ForkConditionalEffect) SelectableStacks() int {
	if effect.StackInput == "selected_stack_count" {
		return max(1, effect.MaxStacks)
	}
	return 1
}

type ForkDamageFilter struct {
	RequiredDamageTags []string `json:"requires_damage_tags,omitempty"`
	AnyDamageTags      []string `json:"requires_any_damage_tags,omitempty"`
}

func (f ForkDamageFilter) HasTags() bool {
	return len(f.RequiredDamageTags) > 0 || len(f.AnyDamageTags) > 0
}

// Matches uses gameplay tag ancestry. A child tag satisfies its parent
// filter, while a parent tag does not satisfy a child filter.
func (f ForkDamageFilter) Matches(tags []string) bool {
	matches := func(required string) bool {
		return required != "" && slices.ContainsFunc(tags, func(tag string) bool {
			return tag == required || strings.HasPrefix(tag, required+".")
		})
	}
	for _, required := range f.RequiredDamageTags {
		if !matches(required) {
			return false
		}
	}
	return len(f.AnyDamageTags) == 0 || slices.ContainsFunc(f.AnyDamageTags, matches)
}

type ForkEffectSelection struct {
	ConditionsConfirmed bool
	RecipientIsWearer   bool
	DamageTags          []string
	Stacks              int
	ReplaceLegacy       bool
}

type ForkConditionalValue struct {
	ID       string
	Scope    string
	Stat     nte.Stat
	Duration *float64
}

// ResolveConditionalEffect resolves exact rank parameters without applying them.
// Callers replacing a legacy projection must remove the original contribution.
func (c ForkCatalog) ResolveConditionalEffect(weapon Weapon, id string, selected ForkEffectSelection) (ForkConditionalValue, error) {
	definition, ok := c.Forks[weapon.ForkID]
	if !ok {
		return ForkConditionalValue{}, fmt.Errorf("unknown Arc %s", weapon.ForkID)
	}
	index := slices.IndexFunc(definition.ConditionalEffects, func(e ForkConditionalEffect) bool { return e.ID == id })
	if index < 0 {
		return ForkConditionalValue{}, fmt.Errorf("unknown Arc effect %s", id)
	}
	effect := definition.ConditionalEffects[index]
	if !selected.ConditionsConfirmed {
		return ForkConditionalValue{}, errors.New("manual confirmation required for Arc effect conditions")
	}
	if effect.LegacyConditionalProjection && !selected.ReplaceLegacy {
		return ForkConditionalValue{}, fmt.Errorf("conditional_by_star already represents Arc effect %s", id)
	}
	if effect.Parameter.LevelSource != "exact_arc_rank" || effect.Target.Operation != "add" {
		return ForkConditionalValue{}, fmt.Errorf("unsupported Arc effect contract %s", id)
	}
	switch effect.Scope {
	case "wearer", "wearer_summon_damage", "wearer_filtered_damage":
		if !selected.RecipientIsWearer {
			return ForkConditionalValue{}, errors.New("wearer must be the recipient of the Arc effect")
		}
	case "other_allies":
		if selected.RecipientIsWearer {
			return ForkConditionalValue{}, errors.New("wearer is not a recipient of the Arc effect")
		}
	case "team_including_wearer", "all_field_summon_damage":
	default:
		return ForkConditionalValue{}, fmt.Errorf("unsupported Arc recipient scope %s", effect.Scope)
	}
	if effect.Scope == "wearer_filtered_damage" && !effect.HasTags() {
		return ForkConditionalValue{}, fmt.Errorf("missing damage filter for scoped Arc effect %s", id)
	}
	if !effect.Matches(selected.DamageTags) {
		return ForkConditionalValue{}, fmt.Errorf("damage tags do not match Arc effect %s", id)
	}
	stacks := selected.Stacks
	switch effect.StackInput {
	case "":
		if effect.FixedStacks != 0 {
			return ForkConditionalValue{}, errors.New("fixed Arc stacks require fixed_stack_count")
		}
	case "selected_stack_count":
		if effect.MaxStacks < 1 || effect.FixedStacks != 0 {
			return ForkConditionalValue{}, errors.New("invalid selectable Arc stack contract")
		}
	case "fixed_stack_count":
		if effect.FixedStacks < 1 || (effect.MaxStacks > 0 && effect.FixedStacks > effect.MaxStacks) {
			return ForkConditionalValue{}, errors.New("invalid fixed Arc stack count")
		}
		stacks = effect.FixedStacks
	default:
		return ForkConditionalValue{}, errors.New("unsupported Arc stack contract")
	}
	if selected.Stacks < 1 || selected.Stacks > effect.SelectableStacks() {
		return ForkConditionalValue{}, errors.New("selected Arc stack count is outside declared bounds")
	}
	parameters, ok := definition.EffectsByStar[strconv.Itoa(weapon.Star)]
	if !ok {
		return ForkConditionalValue{}, fmt.Errorf("unresolved Arc rank %d", weapon.Star)
	}
	parameter := func(name string) (ForkEffectParameter, error) {
		for _, p := range parameters.OrderedParameters {
			if p.NameID == name && !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
				return p, nil
			}
		}
		return ForkEffectParameter{}, fmt.Errorf("unresolved Arc parameter %s at rank %d", name, weapon.Star)
	}
	value, err := parameter(effect.Parameter.NameID)
	if err != nil {
		return ForkConditionalValue{}, err
	}
	if value.IsPercent != effect.Target.Percent {
		return ForkConditionalValue{}, errors.New("inconsistent units for Arc effect parameter")
	}
	out := ForkConditionalValue{ID: id, Scope: effect.Scope, Stat: nte.Stat{PropertyID: effect.Target.PropertyID, Value: value.Value * float64(stacks), Percent: effect.Target.Percent}}
	if effect.DurationParameter != "" {
		duration, err := parameter(effect.DurationParameter)
		if err != nil {
			return ForkConditionalValue{}, err
		}
		if duration.IsPercent || duration.Value < 0 {
			return ForkConditionalValue{}, fmt.Errorf("invalid Arc effect duration")
		}
		out.Duration = &duration.Value
	}
	return out, nil
}
