package damage

import "math"

// ScorchLevel80Base is the official level-76–80 reaction curve value for
// ordinary Scorch and Zankou's dedicated Scorch variant.
const ScorchLevel80Base = 2700.0

// CalculateScorch resolves one Scorch tick at one stack. Zankou's passive can
// then expose up to three stacks in the presentation layer. Scorch does not
// consume ATK or generic/element damage bonuses; it uses Cycle Intensity,
// enemy mitigation, DOT-final bonuses and a fixed 50% critical rate.
func CalculateScorch(characterLevel int, cycleIntensity, critDamage float64, enemy Enemy, stats Stats) Result {
	defenseZone, exactDefense := DefenseMultiplier(characterLevel, enemy, stats.DefenseIgnore, stats.DefenseReduction)
	resistanceZone := ResistanceMultiplier(enemy.Resistance, 0, stats.ResistancePen)
	strengthZone := 1 + math.Max(0, cycleIntensity)/600
	dotFinalZone := productBonuses(stats.DOTFinalBonuses)
	base := ScorchLevel80Base * strengthZone * defenseZone * resistanceZone *
		(1 + stats.Vulnerability) * dotFinalZone
	nonCrit := math.Floor(base)
	crit := math.Floor(base * (1 + critDamage))
	expected := nonCrit*.5 + crit*.5
	result := Result{
		InstanceID: "reaction_scorch_zankou",
		Name:       "Scorch",
		Category:   CategoryReaction,
		NonCrit:    nonCrit,
		Crit:       crit,
		Expected:   expected,
		Hits:       1,
		Total:      expected,
		Factors: Multipliers{
			ScalingStat:       ScorchLevel80Base,
			Coefficient:       1,
			DamageZone:        strengthZone,
			DefenseZone:       defenseZone,
			ResistanceZone:    resistanceZone,
			VulnerabilityZone: 1 + stats.Vulnerability,
			IndependentZone:   1,
			DOTFinalZone:      dotFinalZone,
			CritRate:          .5,
			CritDamage:        critDamage,
		},
		Provenance: Provenance{
			Confidence: ConfidenceGameData,
			Source:     "DT_ReactionDamageData · Buff_Reaction_5_new_1036",
			SourceID:   "reaction_scorch_zankou",
		},
	}
	if !exactDefense {
		result.Warnings = append(result.Warnings, "défense ennemie approximée à partir du niveau")
	}
	return result
}
