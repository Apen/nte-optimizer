package decoded

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"

	"nte-optimizer/internal/nte"
)

// Conditional effects describe selected scenarios, not additional panel stats.
type ForkConditionalEffect struct {
	ID                 string   `json:"id"`
	Scope              string   `json:"scope"`
	Trigger            string   `json:"trigger"`
	RequiredDamageTags []string `json:"requires_damage_tags"`
	DurationParameter  string   `json:"duration_parameter"`
	StackInput         string   `json:"stack_input"`
	MaxStacks          int      `json:"max_stacks"`
	MixedSourcePolicy  string   `json:"mixed_source_policy"`
	Parameter          struct {
		NameID      string `json:"name_id"`
		LevelSource string `json:"level_source"`
	} `json:"parameter"`
	Target struct {
		PropertyID string `json:"property_id"`
		Operation  string `json:"operation"`
		Percent    bool   `json:"percent"`
	} `json:"target"`
	ManualSelectionRequired      bool              `json:"manual_selection_required"`
	NativeExecutionSupported     bool              `json:"native_execution_supported"`
	AutomaticActivationSupported bool              `json:"automatic_activation_supported"`
	LegacyConditionalProjection  bool              `json:"legacy_conditional_projection"`
	Status                       string            `json:"status"`
	Provenance                   []json.RawMessage `json:"provenance"`
}

type ForkEffectSelection struct {
	ConditionsConfirmed bool
	RecipientIsWearer   bool
	DamageTags          []string
	Stacks              int
}

type ForkConditionalValue struct {
	ID       string
	Scope    string
	Stat     nte.Stat
	Duration *float64
}

// ResolveConditionalEffect resolves exact rank parameters without applying them.
// Legacy projections remain exclusively in Stats' conditional_by_star output.
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
		return ForkConditionalValue{}, fmt.Errorf("Arc effect conditions require manual confirmation")
	}
	if effect.LegacyConditionalProjection {
		return ForkConditionalValue{}, fmt.Errorf("Arc effect %s already represented by conditional_by_star", id)
	}
	if effect.Parameter.LevelSource != "exact_arc_rank" || effect.Target.Operation != "add" {
		return ForkConditionalValue{}, fmt.Errorf("unsupported Arc effect contract %s", id)
	}
	switch effect.Scope {
	case "wearer", "wearer_summon_damage":
		if !selected.RecipientIsWearer {
			return ForkConditionalValue{}, fmt.Errorf("Arc effect requires wearer recipient")
		}
	case "team_including_wearer", "all_field_summon_damage":
	default:
		return ForkConditionalValue{}, fmt.Errorf("unsupported Arc recipient scope %s", effect.Scope)
	}
	for _, tag := range effect.RequiredDamageTags {
		if !slices.Contains(selected.DamageTags, tag) {
			return ForkConditionalValue{}, fmt.Errorf("missing required damage tag %s", tag)
		}
	}
	limit := 1
	if effect.StackInput != "" {
		if effect.StackInput != "selected_stack_count" || effect.MaxStacks < 1 {
			return ForkConditionalValue{}, fmt.Errorf("unsupported Arc stack contract")
		}
		limit = effect.MaxStacks
	}
	if selected.Stacks < 1 || selected.Stacks > limit {
		return ForkConditionalValue{}, fmt.Errorf("Arc stack count outside declared bounds")
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
		return ForkConditionalValue{}, fmt.Errorf("Arc effect parameter units disagree")
	}
	out := ForkConditionalValue{ID: id, Scope: effect.Scope, Stat: nte.Stat{PropertyID: effect.Target.PropertyID, Value: value.Value * float64(selected.Stacks), Percent: effect.Target.Percent}}
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
