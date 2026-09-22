package app

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/optimizer"
)

func (s OptimizerService) localizedDamageAnalysis(character *decoded.Character, build optimizer.StatSummary, current *optimizer.StatSummary, language string) *DamageAnalysis {
	analysis := s.damageAnalysis(character, build, current)
	if analysis == nil {
		return nil
	}
	catalog, err := ntelocale.LoadPresentation(s.DataDir, language)
	if err != nil {
		return analysis
	}
	for index := range analysis.Groups {
		if localized, ok := catalog.Damage[analysis.Groups[index].ID]; ok {
			if localized.Name != "" {
				analysis.Groups[index].Name = localized.Name
			}
			if localized.Description != "" {
				analysis.Groups[index].Description = localized.Description
			}
		}
	}
	return analysis
}

type DamageAnalysis struct {
	Status         string          `json:"status"`
	Enemy          damage.Enemy    `json:"enemy"`
	Build          []damage.Result `json:"build"`
	Current        []damage.Result `json:"current,omitempty"`
	MissingInputs  []string        `json:"missing_inputs,omitempty"`
	CatalogSource  string          `json:"catalog_source"`
	CatalogVersion string          `json:"catalog_version,omitempty"`
	Groups         []DamageGroup   `json:"groups,omitempty"`
}

type DamageGroup struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Category           string  `json:"category"`
	ActionType         string  `json:"action_type"`
	BuildDamage        float64 `json:"build_damage"`
	CurrentDamage      float64 `json:"current_damage,omitempty"`
	BuildNonCrit       float64 `json:"build_non_crit"`
	CurrentNonCrit     float64 `json:"current_non_crit,omitempty"`
	BuildCrit          float64 `json:"build_crit"`
	CurrentCrit        float64 `json:"current_crit,omitempty"`
	Instances          int     `json:"instances"`
	MaxStacks          int     `json:"max_stacks,omitempty"`
	BuildMaxTick       float64 `json:"build_max_tick,omitempty"`
	CurrentMaxTick     float64 `json:"current_max_tick,omitempty"`
	BuildMaxCritTick   float64 `json:"build_max_crit_tick,omitempty"`
	CurrentMaxCritTick float64 `json:"current_max_crit_tick,omitempty"`
}

func (s OptimizerService) damageAnalysis(character *decoded.Character, build optimizer.StatSummary, current *optimizer.StatSummary) *DamageAnalysis {
	analysis := &DamageAnalysis{Status: "unavailable"}
	if character == nil {
		analysis.MissingInputs = append(analysis.MissingInputs, "missing_character")
		return analysis
	}
	catalog, err := damage.ReadCatalog(datafiles.New(s.DataDir).Damage())
	if err != nil {
		analysis.MissingInputs = append(analysis.MissingInputs, "missing_damage_catalog")
		return analysis
	}
	characterCatalog, ok := catalog.Characters[strconv.Itoa(character.CharacterID)]
	if !ok {
		analysis.MissingInputs = append(analysis.MissingInputs, "missing_character_coefficients")
		return analysis
	}
	levels := make(map[string]int, len(character.Skills))
	for _, skill := range character.Skills {
		if skill.Level > 0 {
			levels[skill.AbilityID] = skill.Level
		}
	}
	enemy := damage.Enemy{
		Level:         80,
		Resistance:    .20,
		Approximation: "outer_realm",
		Provenance: damage.Provenance{
			Confidence: damage.ConfidenceDocumented,
			Source:     "profil par défaut: boss niveau 80",
		},
	}
	analysis.Enemy = enemy
	analysis.CatalogSource = catalog.Source
	for _, entry := range characterCatalog.Instances {
		level, known := levels[entry.AbilityID]
		if !known {
			continue
		}
		instance := entry.Resolve(strconv.Itoa(character.CharacterID), level)
		stats := damage.StatsFromOptimizer(build.DerivedConditional, entry.Element, true)
		buildInstance := instance
		applyCombatRules(character.AwakenLevel, characterCatalog.Rules, &stats, &buildInstance)
		analysis.Build = append(analysis.Build, damage.Calculate(character.Level, stats, enemy, buildInstance))
		if current != nil {
			currentStats := damage.StatsFromOptimizer(current.DerivedConditional, entry.Element, true)
			currentInstance := instance
			applyCombatRules(character.AwakenLevel, characterCatalog.Rules, &currentStats, &currentInstance)
			analysis.Current = append(analysis.Current, damage.Calculate(character.Level, currentStats, enemy, currentInstance))
		}
	}
	for _, derived := range characterCatalog.Rules.DerivedInstances {
		analysis.Build = append(analysis.Build, calculateDerivedDamage(character, build, enemy, derived))
		if current != nil {
			analysis.Current = append(analysis.Current, calculateDerivedDamage(character, *current, enemy, derived))
		}
	}
	sort.Slice(analysis.Build, func(i, j int) bool { return analysis.Build[i].Total > analysis.Build[j].Total })
	sort.Slice(analysis.Current, func(i, j int) bool { return analysis.Current[i].Total > analysis.Current[j].Total })
	if len(analysis.Build) == 0 {
		analysis.MissingInputs = append(analysis.MissingInputs, "missing_skill_levels")
		return analysis
	}
	analysis.Status = "atomic_preview"
	analysis.Groups = groupDamageResults(analysis.Build, analysis.Current)
	analysis.MissingInputs = append(analysis.MissingInputs,
		"missing_rotation",
		"missing_enemy_profile",
		"missing_buff_timeline",
	)
	return analysis
}

func calculateDerivedDamage(character *decoded.Character, summary optimizer.StatSummary, enemy damage.Enemy, rule damage.DerivedInstance) damage.Result {
	values := summary.DerivedConditional
	stats := damage.StatsFromOptimizer(values, rule.Element, true)
	critDamage := values[rule.CritDamageProperty]
	if character.AwakenLevel >= rule.AwakeningCritDamageBonus.MinAwakenLevel {
		critDamage += rule.AwakeningCritDamageBonus.Value
	}
	switch rule.Kind {
	case "scorch":
		return damage.CalculateScorch(character.Level, values[rule.MagicProperty], critDamage, enemy, stats)
	default:
		return damage.Result{InstanceID: rule.ID}
	}
}

func applyCombatRules(awakenLevel int, rules damage.CharacterRules, stats *damage.Stats, instance *damage.Instance) {
	for _, rule := range rules.InstanceModifiers {
		if awakenLevel < rule.MinAwakenLevel || (rule.InstanceIDContains != "" && !strings.Contains(instance.ID, rule.InstanceIDContains)) {
			continue
		}
		applyDamageStatAdditions(stats, rule.StatAdditions)
		applyDamageStatAdditions(stats, rule.CategoryStatAdditions[instance.Category])
		if rule.CoefficientMultiplier != 0 {
			instance.Coefficient *= rule.CoefficientMultiplier
		}
	}
}

func applyDamageStatAdditions(stats *damage.Stats, additions map[string]float64) {
	stats.GeneralDamage += additions["general_damage"]
	stats.ElementDamage += additions["element_damage"]
	stats.CritRate += additions["crit_rate"]
	stats.CritDamage += additions["crit_damage"]
	stats.DefenseIgnore += additions["defense_ignore"]
	stats.ResistancePen += additions["resistance_pen"]
}

var numberedMelee = regexp.MustCompile(`Zankou_Melee[1-6]_`)

type groupDescriptor struct {
	id, name, description, category, actionType string
	maxStacks                                   int
}

func damageGroupDescriptor(id string) (groupDescriptor, bool) {
	switch {
	case strings.Contains(id, "reaction_scorch"):
		return groupDescriptor{"scorch", "Scorch", "Tick de la réaction Incantation + Chaos de Zankou", "reaction", "reaction", 3}, true
	case strings.Contains(id, "SkillForce"):
		// This GE carries the forced break payload, not a meaningful damage hit.
		return groupDescriptor{}, false
	case strings.Contains(id, "DotUltra"):
		return groupDescriptor{"vile-ash", "Vile Ash", "Tick du DoT appliqué par Bloodfeast Reverie", "dot", "dot", 10}, true
	case strings.Contains(id, "DotDamage"):
		return groupDescriptor{"heartwrench", "Heartwrench", "Tick du DoT appliqué et propagé par Zankou", "dot", "dot", 10}, true
	case strings.Contains(id, "ForceUltraSkill"):
		return groupDescriptor{"inferno-enhanced", "Inferno Flamenco renforcé", "Tous les coups de l'ultime renforcé", "direct", "ultimate", 0}, true
	case strings.Contains(id, "MagicUltraSkill"):
		return groupDescriptor{"bloodfeast", "Bloodfeast Reverie", "Tous les coups de l'ultime en forme Illusion", "direct", "ultimate", 0}, true
	case strings.Contains(id, "UltraSkill"):
		return groupDescriptor{"inferno", "Inferno Flamenco", "Tous les coups de l'ultime", "direct", "ultimate", 0}, true
	case strings.Contains(id, "Skill1"):
		return groupDescriptor{"sanguine-normal", "Sanguine Dash", "Tous les coups de la version normale", "direct", "skill", 0}, true
	case strings.Contains(id, "Skill2"):
		return groupDescriptor{"sanguine-enhanced", "Sanguine Dash renforcé", "Tous les coups de la version renforcée", "direct", "skill", 0}, true
	case strings.Contains(id, "Skill3"):
		return groupDescriptor{"soulcross-normal", "Soulcross", "Tous les coups de la version normale", "direct", "skill", 0}, true
	case strings.Contains(id, "Skill4"):
		return groupDescriptor{"soulcross-enhanced", "Soulcross renforcé", "Tous les coups de la version renforcée", "direct", "skill", 0}, true
	case strings.Contains(id, "MagicMelee"):
		return groupDescriptor{"nightmare-waltz", "Nightmare Waltz", "Combo complet en forme Illusion", "direct", "basic_attack", 0}, true
	case numberedMelee.MatchString(id):
		return groupDescriptor{"wildfire", "Wildfire", "Combo complet en forme Réalité", "direct", "basic_attack", 0}, true
	case strings.Contains(id, "MagicBranch"):
		return groupDescriptor{"moonfall", "Moonfall", "Tous les coups de l'attaque maintenue", "direct", "charged_attack", 0}, true
	case strings.Contains(id, "Branch"):
		return groupDescriptor{"flickering-shadow", "Flickering Shadow", "Tous les coups de l'attaque maintenue", "direct", "charged_attack", 0}, true
	case strings.Contains(id, "PerfectEvadeAttack"):
		return groupDescriptor{"voidstep", "Voidstep", "Riposte complète après esquive critique", "direct", "dodge_counter", 0}, true
	case strings.Contains(id, "AirAttack"):
		return groupDescriptor{"broken-twigs", "Broken Twigs", "Attaque plongeante", "direct", "plunging_attack", 0}, true
	case strings.Contains(id, "QTE"):
		return groupDescriptor{"stoked-flame", "Stoked Flame", "Compétence de soutien complète", "direct", "qte", 0}, true
	default:
		return groupDescriptor{}, false
	}
}

func groupDamageResults(build, current []damage.Result) []DamageGroup {
	groups := map[string]*DamageGroup{}
	add := func(results []damage.Result, currentValues bool) {
		for _, result := range results {
			descriptor, ok := damageGroupDescriptor(result.InstanceID)
			if !ok {
				continue
			}
			group := groups[descriptor.id]
			if group == nil {
				group = &DamageGroup{ID: descriptor.id, Name: descriptor.name, Description: descriptor.description, Category: descriptor.category, ActionType: descriptor.actionType, MaxStacks: descriptor.maxStacks}
				groups[descriptor.id] = group
			}
			if currentValues {
				group.CurrentDamage += result.Total
				group.CurrentNonCrit += result.NonCrit * float64(result.Hits)
				group.CurrentCrit += result.Crit * float64(result.Hits)
			} else {
				group.BuildDamage += result.Total
				group.BuildNonCrit += result.NonCrit * float64(result.Hits)
				group.BuildCrit += result.Crit * float64(result.Hits)
				group.Instances++
			}
		}
	}
	add(build, false)
	add(current, true)
	result := make([]DamageGroup, 0, len(groups))
	for _, group := range groups {
		if group.MaxStacks > 0 {
			group.BuildMaxTick = group.BuildDamage * float64(group.MaxStacks)
			group.CurrentMaxTick = group.CurrentDamage * float64(group.MaxStacks)
			group.BuildMaxCritTick = group.BuildCrit * float64(group.MaxStacks)
			group.CurrentMaxCritTick = group.CurrentCrit * float64(group.MaxStacks)
		}
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			order := map[string]int{"reaction": 0, "dot": 1, "direct": 2}
			return order[result[i].Category] < order[result[j].Category]
		}
		return result[i].BuildDamage > result[j].BuildDamage
	})
	return result
}
