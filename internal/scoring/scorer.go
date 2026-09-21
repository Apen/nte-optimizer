package scoring

import (
	"fmt"

	"nte-optimizer/internal/nte"
)

type StatCap struct {
	SoftCap        float64 `json:"soft_cap,omitempty"`
	HardCap        float64 `json:"hard_cap,omitempty"`
	AfterSoftScale float64 `json:"after_soft_scale,omitempty"`
}

type SetPreference struct {
	SetID    string  `json:"set_id"`
	Priority float64 `json:"priority"`
}

type ConsoleTrait struct {
	Area           int     `json:"area"`
	PropertyID     string  `json:"property_id"`
	ValuePerModule float64 `json:"value_per_module"`
}

type Character struct {
	CharacterID   int                `json:"character_id,omitempty"`
	Name          string             `json:"name,omitempty"`
	TargetID      string             `json:"target_id,omitempty"`
	GridID        string             `json:"grid_id,omitempty"`
	Weights       map[string]float64 `json:"weights,omitempty"`
	MainStats     []string           `json:"main_stats,omitempty"`
	MainWeights   map[string]float64 `json:"main_weights,omitempty"`
	SubWeights    map[string]float64 `json:"sub_weights,omitempty"`
	CurrentStats  map[string]float64 `json:"current_stats,omitempty"`
	BaseStats     map[string]float64 `json:"base_stats,omitempty"`
	ConsoleTrait  *ConsoleTrait      `json:"console_trait,omitempty"`
	Caps          map[string]StatCap `json:"caps,omitempty"`
	PreferredSets []SetPreference    `json:"preferred_sets,omitempty"`
}

type References map[string]float64

type Contribution struct {
	Source          string  `json:"source"`
	PropertyID      string  `json:"property_id"`
	Value           float64 `json:"value"`
	EffectiveValue  float64 `json:"effective_value"`
	ReferenceValue  float64 `json:"reference_value"`
	NormalizedValue float64 `json:"normalized_value"`
	Weight          float64 `json:"weight"`
	Score           float64 `json:"score"`
	Capped          bool    `json:"capped,omitempty"`
}

type Breakdown struct {
	Total         float64        `json:"total"`
	Contributions []Contribution `json:"contributions"`
}

func ValidateCharacter(id string, c Character, refs References) error {
	if len(c.Weights) == 0 && len(c.MainWeights) == 0 && len(c.SubWeights) == 0 {
		return fmt.Errorf("character %q has no stat weights", id)
	}
	for group, weights := range map[string]map[string]float64{
		"weights": c.Weights, "main_weights": c.MainWeights, "sub_weights": c.SubWeights,
	} {
		for propertyID, weight := range weights {
			if weight < 0 {
				return fmt.Errorf("character %q %s.%s must be non-negative", id, group, propertyID)
			}
			if refs[propertyID] <= 0 {
				return fmt.Errorf("character %q %s.%s has no positive reference value", id, group, propertyID)
			}
		}
	}
	for propertyID, cap := range c.Caps {
		if cap.SoftCap < 0 || cap.HardCap < 0 || (cap.HardCap > 0 && cap.SoftCap > cap.HardCap) {
			return fmt.Errorf("character %q has invalid caps for %s", id, propertyID)
		}
		if cap.AfterSoftScale < 0 || cap.AfterSoftScale > 1 {
			return fmt.Errorf("character %q caps.%s.after_soft_scale must be between 0 and 1", id, propertyID)
		}
	}
	for _, preference := range c.PreferredSets {
		if preference.SetID == "" || preference.Priority < 0 {
			return fmt.Errorf("character %q has an invalid preferred set", id)
		}
	}
	if c.ConsoleTrait != nil && (c.ConsoleTrait.Area <= 0 || c.ConsoleTrait.PropertyID == "" || c.ConsoleTrait.ValuePerModule < 0) {
		return fmt.Errorf("character %q has an invalid console trait", id)
	}
	return nil
}

func Score(m nte.Module, c Character, refs References) float64 {
	return ScoreDetailed(m, c, refs).Total
}

func ScoreDetailed(m nte.Module, c Character, refs References) Breakdown {
	return ScoreStatsDetailed(m.MainStats, m.SubStats, c, refs)
}

func ScoreCartridgeDetailed(ca nte.Cartridge, c Character, refs References) Breakdown {
	return ScoreStatsDetailed(ca.MainStats, ca.SubStats, c, refs)
}

func ScoreStatsDetailed(mainStats, subStats []nte.Stat, c Character, refs References) Breakdown {
	result := Breakdown{Contributions: make([]Contribution, 0, len(mainStats)+len(subStats))}
	current := cloneStats(c.CurrentStats)
	for _, group := range []struct {
		name  string
		stats []nte.Stat
	}{
		{name: "main", stats: mainStats},
		{name: "sub", stats: subStats},
	} {
		for _, stat := range group.stats {
			ref := refs[stat.PropertyID]
			weight := statWeight(c, group.name, stat.PropertyID)
			if ref <= 0 || weight == 0 {
				continue
			}
			effective, capped := effectiveValue(current[stat.PropertyID], stat.Value, c.Caps[stat.PropertyID])
			normalized := effective / ref
			contribution := Contribution{
				Source: group.name, PropertyID: stat.PropertyID, Value: stat.Value,
				EffectiveValue: effective, ReferenceValue: ref, NormalizedValue: normalized,
				Weight: weight, Score: normalized * weight, Capped: capped,
			}
			result.Total += contribution.Score
			result.Contributions = append(result.Contributions, contribution)
			current[stat.PropertyID] += stat.Value
		}
	}
	return result
}

func statWeight(c Character, source, propertyID string) float64 {
	if source == "main" {
		if weight, ok := c.MainWeights[propertyID]; ok {
			return weight
		}
	} else if weight, ok := c.SubWeights[propertyID]; ok {
		return weight
	}
	return c.Weights[propertyID]
}

func effectiveValue(current, value float64, cap StatCap) (float64, bool) {
	if value <= 0 || (cap.SoftCap <= 0 && cap.HardCap <= 0) {
		return value, false
	}
	end := current + value
	if cap.HardCap > 0 && end > cap.HardCap {
		end = cap.HardCap
	}
	if end <= current {
		return 0, true
	}
	if cap.SoftCap <= 0 || end <= cap.SoftCap {
		return end - current, end-current != value
	}
	scale := cap.AfterSoftScale
	if scale == 0 {
		scale = 0.25
	}
	belowSoft := 0.0
	if current < cap.SoftCap {
		belowSoft = cap.SoftCap - current
	}
	aboveSoft := end - maxFloat(current, cap.SoftCap)
	return belowSoft + aboveSoft*scale, true
}

func cloneStats(input map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
