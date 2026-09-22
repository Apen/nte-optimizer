package damage

import "math"

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func productBonuses(values []float64) float64 {
	result := 1.0
	for _, value := range values {
		result *= 1 + value
	}
	return result
}

func scalingValue(stats Stats, scaling ScalingStat) float64 {
	switch scaling {
	case ScalingHP:
		return stats.HP
	case ScalingDefense:
		return stats.Defense
	default:
		return stats.Attack
	}
}

// DefenseMultiplier uses an official enemy panel when one is available. When
// it is not, it falls back to the documented level approximation.
func DefenseMultiplier(characterLevel int, enemy Enemy, ignore, reduction float64) (float64, bool) {
	levelTerm := float64(characterLevel + 100)
	defense := 0.0
	exact := enemy.DefenseBase != nil
	if exact {
		defense = (*enemy.DefenseBase*(1+enemy.DefenseUp) + enemy.DefenseAdd) / 6
	} else {
		base := 90.0
		if enemy.Approximation == "open_world" {
			base = 100
		}
		defense = float64(enemy.Level) + base
	}
	defense *= 1 - clamp(ignore, 0, 1)
	defense *= 1 - clamp(reduction, 0, 1)
	if defense < 0 {
		defense = 0
	}
	return levelTerm / (defense + levelTerm), exact
}

func ResistanceMultiplier(resistance, reduction, penetration float64) float64 {
	effective := resistance - reduction - penetration
	if effective >= 0 {
		return 1 - effective
	}
	return 1 - effective/1.10
}

// Calculate resolves one atomic direct or DOT hit. Rounding happens once at
// the damage outlets; the expected value is a statistical blend of the two
// independently rounded candidates.
func Calculate(characterLevel int, stats Stats, enemy Enemy, instance Instance) Result {
	hits := instance.Hits
	if hits <= 0 {
		hits = 1
	}
	critRate := clamp(stats.CritRate, 0, 1)
	if instance.FixedCritRate != nil {
		critRate = clamp(*instance.FixedCritRate, 0, 1)
	}
	damageZone := 1 + stats.GeneralDamage + stats.ElementDamage + stats.SkillDamage + stats.StateDamage
	defenseZone, exactDefense := DefenseMultiplier(characterLevel, enemy, stats.DefenseIgnore, stats.DefenseReduction)
	resistanceZone := ResistanceMultiplier(enemy.Resistance, 0, stats.ResistancePen)
	factors := Multipliers{
		ScalingStat:       scalingValue(stats, instance.Scaling),
		Coefficient:       instance.Coefficient,
		DamageZone:        damageZone,
		DefenseZone:       defenseZone,
		ResistanceZone:    resistanceZone,
		VulnerabilityZone: 1 + stats.Vulnerability,
		IndependentZone:   productBonuses(stats.IndependentBonuses),
		DOTFinalZone:      1,
		CritRate:          critRate,
		CritDamage:        stats.CritDamage,
	}
	if instance.Category == CategoryDOT {
		factors.DOTFinalZone = productBonuses(stats.DOTFinalBonuses)
	}
	base := factors.Coefficient * factors.ScalingStat * factors.DamageZone *
		factors.DefenseZone * factors.ResistanceZone * factors.VulnerabilityZone *
		factors.IndependentZone * factors.DOTFinalZone
	nonCrit := math.Floor(base)
	crit := math.Floor(base * (1 + stats.CritDamage))
	expected := nonCrit*(1-critRate) + crit*critRate
	result := Result{
		InstanceID: instance.ID,
		Name:       instance.Name,
		Category:   instance.Category,
		NonCrit:    nonCrit,
		Crit:       crit,
		Expected:   expected,
		Hits:       hits,
		Total:      expected * float64(hits),
		Factors:    factors,
		Provenance: instance.Provenance,
	}
	if !exactDefense {
		result.Warnings = append(result.Warnings, "enemy defense estimated from level")
	}
	if instance.Provenance.Confidence == ConfidenceInferred || instance.Provenance.Confidence == ConfidenceUnknown {
		result.Warnings = append(result.Warnings, "unconfirmed damage rule")
	}
	return result
}
