package app

import (
	"math"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/optimizer"
)

// PanelComparison preserves signed residuals. A mismatch is diagnostic, not a
// new stat source, and may include unmodeled combat effects or missing inputs.
type PanelComparison struct {
	PropertyID string  `json:"property_id"`
	Observed   float64 `json:"observed"`
	Calculated float64 `json:"calculated"`
	Difference float64 `json:"difference"`
	Matches    bool    `json:"matches"`
}

func compareObservedPanel(panel decoded.CharacterStats, stats *optimizer.StatSummary) []PanelComparison {
	if stats == nil {
		return []PanelComparison{}
	}
	type observedValue struct {
		property  string
		value     float64
		available bool
		rounded   bool
	}
	values := []observedValue{
		{"HPFinal", panel.MaxHP, panel.MaxHP > 0, true},
		{"AtkFinal", valueOrZero(panel.Attack), panel.Attack != nil, true},
		{"DefFinal", valueOrZero(panel.Defense), panel.Defense != nil, true},
		{"Endurance", valueOrZero(panel.Endurance), panel.Endurance != nil, true},
		{"CritBase", valueOrZero(panel.CritRate), panel.CritRate != nil, false},
		{"CritDamageBase", valueOrZero(panel.CritDamage), panel.CritDamage != nil, false},
		{"ChargeGetEfficiencyBase", valueOrZero(panel.ChargeEfficiency), panel.ChargeEfficiency != nil, false},
		{"MagBase", valueOrZero(panel.CycleIntensity), panel.CycleIntensity != nil, true},
		{"UnbalIntensityBase", valueOrZero(panel.BreakIntensity), panel.BreakIntensity != nil, true},
		{"DamageUpGeneralBase", valueOrZero(panel.UniversalDMGBonus), panel.UniversalDMGBonus != nil, false},
		{"ElementalDMGBonus", valueOrZero(panel.ElementalDMGBonus), panel.ElementalDMGBonus != nil, false},
	}
	result := []PanelComparison{}
	for _, entry := range values {
		calculated, ok := stats.Derived[entry.property]
		if !entry.available || !ok {
			continue
		}
		difference := entry.value - calculated
		tolerance := .001
		if entry.rounded {
			tolerance = 1
		}
		result = append(result, PanelComparison{entry.property, entry.value, calculated, difference, math.Abs(difference) <= tolerance})
	}
	return result
}
