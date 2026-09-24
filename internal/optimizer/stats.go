package optimizer

import (
	"math"
	"sort"
	"strings"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

type StatSummary struct {
	Completeness       string                        `json:"completeness"`
	MissingSources     []string                      `json:"missing_sources"`
	Sources            map[string]map[string]float64 `json:"sources"`
	Unconditional      map[string]float64            `json:"unconditional"`
	WithConditions     map[string]float64            `json:"with_conditions"`
	Derived            map[string]float64            `json:"derived"`
	DerivedConditional map[string]float64            `json:"derived_conditional"`
}

func BuildStatSummary(character scoring.Character, modules []nte.Module, cartridge *nte.Cartridge, set SetDefinition, matched int, additional ...map[string][]nte.Stat) StatSummary {
	s := StatSummary{
		Completeness:   "partial",
		MissingSources: []string{"weapon/Arc", "awakening and passives", "account-specific affinity and bonuses"},
		Sources:        map[string]map[string]float64{},
	}
	s.Sources["base"] = cloneValues(character.BaseStats)
	s.Sources["profile_overrides"] = cloneValues(character.CurrentStats)
	s.Sources["modules"] = map[string]float64{}
	for _, module := range modules {
		addStats(s.Sources["modules"], module.MainStats)
		addStats(s.Sources["modules"], module.SubStats)
	}
	if cartridge != nil {
		s.Sources["cartridge"] = map[string]float64{}
		addStats(s.Sources["cartridge"], cartridge.MainStats)
		addStats(s.Sources["cartridge"], cartridge.SubStats)
	}
	s.Sources["console_trait"] = map[string]float64{}
	if character.ConsoleTrait != nil {
		count := 0
		for _, module := range modules {
			if module.Area == character.ConsoleTrait.Area {
				count++
			}
		}
		s.Sources["console_trait"][character.ConsoleTrait.PropertyID] = float64(count) * character.ConsoleTrait.ValuePerModule
	}
	s.Sources["set"] = map[string]float64{}
	s.Sources["set_conditional"] = map[string]float64{}
	for _, bonus := range set.Bonuses {
		if matched >= bonus.Count {
			addStats(s.Sources["set"], bonus.Stats)
			addStats(s.Sources["set_conditional"], bonus.ConditionalStats)
		}
	}
	for _, sources := range additional {
		for name, stats := range sources {
			if s.Sources[name] == nil {
				s.Sources[name] = map[string]float64{}
			}
			addStats(s.Sources[name], stats)
		}
	}
	if len(s.Sources["weapon"]) > 0 {
		s.MissingSources = without(s.MissingSources, "weapon/Arc")
	}
	s.Unconditional = sumSources(s.Sources, false)
	s.WithConditions = sumSources(s.Sources, true)
	s.Derived = derive(s.Unconditional)
	s.DerivedConditional = derive(s.WithConditions)
	return s
}

func without(values []string, remove string) []string {
	out := values[:0]
	for _, value := range values {
		if value != remove {
			out = append(out, value)
		}
	}
	return out
}

func addStats(dst map[string]float64, stats []nte.Stat) {
	for _, stat := range stats {
		dst[stat.PropertyID] += stat.Value
	}
}
func cloneValues(src map[string]float64) map[string]float64 {
	dst := map[string]float64{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
func sumSources(sources map[string]map[string]float64, conditional bool) map[string]float64 {
	out := map[string]float64{}
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, source := range keys {
		if strings.HasSuffix(source, "_conditional") && !conditional {
			continue
		}
		for key, value := range sources[source] {
			out[key] += value
		}
	}
	return out
}
func derive(stats map[string]float64) map[string]float64 {
	out := cloneValues(stats)
	out["HPFinal"] = panelTotal(stats["HPMaxBase"], stats["HPMaxUp"], stats["HPMaxAdd"])
	out["AtkFinal"] = panelTotal(stats["AtkBase"], stats["AtkUp"], stats["AtkAdd"])
	out["DefFinal"] = panelTotal(stats["DefBase"], stats["DefUp"], stats["DefAdd"])
	out["BasicDamageIndex"] = BasicDamageIndex(out)
	return out
}

// BasicDamageIndex compares builds with a coefficient-1 attack before enemy
// mitigation or skill-specific effects. CritDamageBase is the additive critical
// damage bonus (for example 0.50 for a 50% bonus).
func BasicDamageIndex(stats map[string]float64) float64 {
	attack := math.Max(0, stats["AtkFinal"])
	critRate := math.Max(0, math.Min(1, stats["CritBase"]))
	critDamage := math.Max(0, stats["CritDamageBase"])
	damageMultiplier := math.Max(0, 1+stats["DamageUpGeneralBase"])
	expectedCritMultiplier := 1 + critRate*critDamage
	return attack * damageMultiplier * expectedCritMultiplier
}

func panelTotal(base, percent, flat float64) float64 {
	return base + math.Floor(base*percent+flat+1e-9)
}
