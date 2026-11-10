package combatdata

import "fmt"

type AttributeExtraction struct {
	ID                       string `json:"id"`
	CharacterID              int    `json:"character_id"`
	AbilityID                string `json:"ability_id"`
	NativeExecutionSupported bool   `json:"native_execution_supported"`
	BuilderScenario          struct {
		TargetPropertyID string     `json:"target_property_id"`
		SourcePropertyID string     `json:"source_property_id"`
		Recipients       Recipients `json:"recipients"`
		Calculation      struct {
			Kind             string    `json:"kind"`
			Source           string    `json:"source"`
			SourcePropertyID string    `json:"source_property_id"`
			Ratios           []float64 `json:"ratio_by_effect_level"`
			Caps             []float64 `json:"cap_ratio_by_effect_level"`
			Input            Input     `json:"input"`
			CapPolicy        string    `json:"cap_policy"`
		} `json:"calculation"`
		Assumptions []string `json:"assumptions"`
	} `json:"builder_scenario"`
}

func (e AttributeExtraction) Evaluate(caster Actor, effectLevel int, count float64) (BuffValue, error) {
	out := BuffValue{ID: e.ID, Target: Target{PropertyID: e.BuilderScenario.TargetPropertyID, Operation: "add"}}
	c := e.BuilderScenario.Calculation
	if caster.CharacterID != e.CharacterID || c.Kind != "capped_source_ratio_per_selected_drain" || c.Source != "caster" || c.CapPolicy != "multiply_nonnegative_count_then_clamp_ratio_then_multiply_base_attribute" {
		return out, fmt.Errorf("unsupported attribute extraction contract")
	}
	if err := c.Input.Validate(count); err != nil {
		return out, err
	}
	ratio, err := Exact(c.Ratios, effectLevel)
	if err != nil {
		return out, err
	}
	cap, err := Exact(c.Caps, effectLevel)
	if err != nil {
		return out, err
	}
	base, ok := caster.Stats[c.SourcePropertyID]
	if !ok {
		return out, fmt.Errorf("missing source attribute %s", c.SourcePropertyID)
	}
	out.Value = min(cap, max(0, ratio*count)) * base
	return out, nil
}
