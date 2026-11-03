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

// damageBatch shares immutable inputs and the equipped baseline across alternatives.
type damageBatch struct {
	character     *decoded.Character
	catalog       damage.Catalog
	catalogErr    error
	labels        ntelocale.PresentationCatalog
	labelsErr     error
	current       []damage.Result
	currentBuffed []damage.Result
}

func (s OptimizerService) prepareDamageBatch(character *decoded.Character, current *optimizer.StatSummary, language string) damageBatch {
	catalog, catalogErr := damage.ReadCatalog(datafiles.New(s.DataDir).Damage())
	labels, labelsErr := ntelocale.LoadPresentation(s.DataDir, language)
	batch := damageBatch{character: character, catalog: catalog, catalogErr: catalogErr, labels: labels, labelsErr: labelsErr}
	if current != nil {
		batch.currentBuffed = batch.calculate(*current, true).Build
		batch.current = batch.calculate(*current, false).Build
	}
	return batch
}

func (batch damageBatch) evaluate(build optimizer.StatSummary) *DamageAnalysis {
	analysis := batch.calculate(build, false)
	analysis.Current = batch.current
	if analysis.Status == "atomic_preview" {
		analysis.Groups = groupDamageResults(batch.character.CharacterID, analysis.Build, analysis.Current, nil)
		buffed := batch.calculate(build, true)
		groups := groupDamageResults(batch.character.CharacterID, buffed.Build, batch.currentBuffed, nil)
		byID := make(map[string]DamageGroup, len(groups))
		for _, group := range groups {
			byID[group.ID] = group
		}
		for index := range analysis.Groups {
			group := &analysis.Groups[index]
			if projection, ok := byID[group.ID]; ok {
				group.Buffed = &DamageBuffedValues{
					Build: projection.BuildDamage, Current: projection.CurrentDamage,
					BuildCrit: projection.BuildCrit, CurrentCrit: projection.CurrentCrit,
				}
				if group.Category == "dot" || group.Category == "reaction" {
					group.Buffed = &DamageBuffedValues{
						Build: projection.BuildMaxTick, Current: projection.CurrentMaxTick,
						BuildCrit: projection.BuildMaxCritTick, CurrentCrit: projection.CurrentMaxCritTick,
					}
				}
			}
		}
	}
	if batch.labelsErr != nil {
		return analysis
	}
	catalog := batch.labels
	for index := range analysis.Groups {
		if localized, ok := catalog.Damage[analysis.Groups[index].ID]; ok {
			if localized.Description != "" {
				analysis.Groups[index].Description = localized.Description
			}
			if localized.Name != "" {
				analysis.Groups[index].Name = localized.Name
				continue
			}
		}
		if localized := localizedAbilityLabel(catalog.Abilities, analysis.Groups[index].ID); localized != "" {
			analysis.Groups[index].Name = localized
		} else if strings.HasPrefix(analysis.Groups[index].Name, "GE_") {
			analysis.Groups[index].Name = fallbackActionName(catalog.UI, analysis.Groups[index].ActionType)
		}
	}
	return analysis
}

func (batch damageBatch) calculate(stats optimizer.StatSummary, buffed bool) *DamageAnalysis {
	if buffed && stats.DerivedWithAllBuffs != nil {
		stats.DerivedConditional = stats.DerivedWithAllBuffs
	} else if !buffed && stats.Derived != nil {
		stats.DerivedConditional = stats.Derived
	}
	return damageAnalysisFromCatalog(batch.character, stats, nil, batch.catalog, batch.catalogErr)
}

func localizedAbilityLabel(labels map[string]string, abilityID string) string {
	for _, candidate := range abilityLabelCandidates(abilityID) {
		for id, label := range labels {
			if strings.EqualFold(id, candidate) && strings.TrimSpace(label) != "" {
				return strings.TrimSpace(label)
			}
		}
	}
	return ""
}

func abilityLabelCandidates(abilityID string) []string {
	parts := strings.Split(abilityID, "_")
	if len(parts) < 3 || parts[0] != "GA" {
		return []string{abilityID}
	}
	runtimeCharacter := parts[1]
	character := regexp.MustCompile(`[0-9]{3}$`).ReplaceAllString(runtimeCharacter, "")
	character = strings.TrimSuffix(character, "ActorWind")
	action := strings.Join(parts[2:], "_")
	normalizedAction := action
	switch {
	case strings.Contains(action, "UltraSkill"):
		normalizedAction = "UltraSkill"
	case strings.Contains(action, "QTE"):
		normalizedAction = "QTE"
	case strings.Contains(action, "Skill"):
		normalizedAction = "Skill"
	case strings.Contains(action, "Melee") || strings.Contains(action, "Appear") || strings.Contains(action, "Branch"):
		normalizedAction = "Melee"
	}
	runtimeBase := "GA_" + runtimeCharacter + "_" + normalizedAction
	base := "GA_" + character + "_" + normalizedAction
	return []string{
		abilityID,
		runtimeBase,
		strings.Replace(runtimeBase, "UltraSkill", "UtraSkill", 1),
		runtimeBase + "1",
		base,
		strings.Replace(base, "UltraSkill", "UtraSkill", 1),
		base + "1",
	}
}

func fallbackActionName(labels map[string]string, actionType string) string {
	keys := map[string]string{
		"ultimate":        "skill_ultimate",
		"skill":           "skill_ability",
		"basic_attack":    "skill_basic_attack",
		"qte":             "skill_qte",
		"charged_attack":  "damage_type_charged_attack",
		"dodge_counter":   "damage_type_dodge_counter",
		"plunging_attack": "damage_type_plunging_attack",
	}
	if label := strings.TrimSpace(labels[keys[actionType]]); label != "" {
		return label
	}
	return "Damage action"
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
	ActionCondition    *damage.ActionCondition `json:"action_condition,omitempty"`
	Buffed             *DamageBuffedValues     `json:"buffed,omitempty"`
	ID                 string                  `json:"id"`
	Name               string                  `json:"name"`
	Description        string                  `json:"description"`
	Category           string                  `json:"category"`
	ActionType         string                  `json:"action_type"`
	BuildDamage        float64                 `json:"build_damage"`
	CurrentDamage      float64                 `json:"current_damage,omitempty"`
	BuildNonCrit       float64                 `json:"build_non_crit"`
	CurrentNonCrit     float64                 `json:"current_non_crit,omitempty"`
	BuildCrit          float64                 `json:"build_crit"`
	CurrentCrit        float64                 `json:"current_crit,omitempty"`
	Instances          int                     `json:"instances"`
	MaxStacks          int                     `json:"max_stacks,omitempty"`
	BuildMaxTick       float64                 `json:"build_max_tick,omitempty"`
	CurrentMaxTick     float64                 `json:"current_max_tick,omitempty"`
	BuildMaxCritTick   float64                 `json:"build_max_crit_tick,omitempty"`
	CurrentMaxCritTick float64                 `json:"current_max_crit_tick,omitempty"`
}

type DamageBuffedValues struct {
	Build       float64 `json:"build"`
	Current     float64 `json:"current"`
	BuildCrit   float64 `json:"build_crit"`
	CurrentCrit float64 `json:"current_crit"`
}

func (s OptimizerService) damageAnalysis(character *decoded.Character, build optimizer.StatSummary, current *optimizer.StatSummary) *DamageAnalysis {
	catalog, err := damage.ReadCatalog(datafiles.New(s.DataDir).Damage())
	return damageAnalysisFromCatalog(character, build, current, catalog, err)
}

func damageAnalysisFromCatalog(character *decoded.Character, build optimizer.StatSummary, current *optimizer.StatSummary, catalog damage.Catalog, err error) *DamageAnalysis {
	analysis := &DamageAnalysis{Status: "unavailable"}
	if character == nil {
		analysis.MissingInputs = append(analysis.MissingInputs, "missing_character")
		return analysis
	}
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
			Source:     "default profile: level 80 boss",
		},
	}
	analysis.Enemy = enemy
	analysis.CatalogSource = catalog.Source
	for _, entry := range characterCatalog.Instances {
		level, known := levels[entry.AbilityID]
		if !known {
			level, known = levels[characterCatalog.Rules.SkillLevelSources[entry.AbilityID]]
		}
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
	analysis.Groups = groupDamageResults(character.CharacterID, analysis.Build, analysis.Current, nil)
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

func damageGroupDescriptor(characterID int, result damage.Result) (groupDescriptor, bool) {
	if result.OmitFromActionPreview {
		return groupDescriptor{}, false
	}
	id := result.InstanceID
	if characterID != 1036 {
		if result.SkillID == "" {
			return groupDescriptor{}, false
		}
		return genericDamageGroupDescriptor(result), true
	}
	switch {
	case strings.Contains(id, "reaction_scorch"):
		return groupDescriptor{"scorch", "Scorch", "Tick triggered by Zankou's Incantation + Chaos reaction", "reaction", "reaction", 3}, true
	case strings.Contains(id, "SkillForce"):
		// This GE carries the forced break payload, not a meaningful damage hit.
		return groupDescriptor{}, false
	case strings.Contains(id, "DotUltra"):
		return groupDescriptor{"vile-ash", "Vile Ash", "DoT tick applied by Bloodfeast Reverie", "dot", "dot", 10}, true
	case strings.Contains(id, "DotDamage"):
		return groupDescriptor{"heartwrench", "Heartwrench", "DoT tick applied and spread by Zankou", "dot", "dot", 10}, true
	case strings.Contains(id, "ForceUltraSkill"):
		return groupDescriptor{"inferno-enhanced", "Enhanced Inferno Flamenco", "All hits of the enhanced ultimate", "direct", "ultimate", 0}, true
	case strings.Contains(id, "MagicUltraSkill"):
		return groupDescriptor{"bloodfeast", "Bloodfeast Reverie", "All ultimate hits in Illusion form", "direct", "ultimate", 0}, true
	case strings.Contains(id, "UltraSkill"):
		return groupDescriptor{"inferno", "Inferno Flamenco", "All ultimate hits", "direct", "ultimate", 0}, true
	case strings.Contains(id, "Skill1"):
		return groupDescriptor{"sanguine-normal", "Sanguine Dash", "All hits of the normal version", "direct", "skill", 0}, true
	case strings.Contains(id, "Skill2"):
		return groupDescriptor{"sanguine-enhanced", "Enhanced Sanguine Dash", "All hits of the enhanced version", "direct", "skill", 0}, true
	case strings.Contains(id, "Skill3"):
		return groupDescriptor{"soulcross-normal", "Soulcross", "All hits of the normal version", "direct", "skill", 0}, true
	case strings.Contains(id, "Skill4"):
		return groupDescriptor{"soulcross-enhanced", "Enhanced Soulcross", "All hits of the enhanced version", "direct", "skill", 0}, true
	case strings.Contains(id, "MagicMelee"):
		return groupDescriptor{"nightmare-waltz", "Nightmare Waltz", "Full combo in Illusion form", "direct", "basic_attack", 0}, true
	case numberedMelee.MatchString(id):
		return groupDescriptor{"wildfire", "Wildfire", "Full combo in Reality form", "direct", "basic_attack", 0}, true
	case strings.Contains(id, "MagicBranch"):
		return groupDescriptor{"moonfall", "Moonfall", "All hits of the charged attack", "direct", "charged_attack", 0}, true
	case strings.Contains(id, "Branch"):
		return groupDescriptor{"flickering-shadow", "Flickering Shadow", "All hits of the charged attack", "direct", "charged_attack", 0}, true
	case strings.Contains(id, "PerfectEvadeAttack"):
		return groupDescriptor{"voidstep", "Voidstep", "Full counter after a critical dodge", "direct", "dodge_counter", 0}, true
	case strings.Contains(id, "AirAttack"):
		return groupDescriptor{"broken-twigs", "Broken Twigs", "Plunging attack", "direct", "plunging_attack", 0}, true
	case strings.Contains(id, "QTE"):
		return groupDescriptor{"stoked-flame", "Stoked Flame", "Full support skill", "direct", "qte", 0}, true
	default:
		return groupDescriptor{}, false
	}
}

func genericDamageGroupDescriptor(result damage.Result) groupDescriptor {
	actionType := "skill"
	actionID := result.ActionID
	if actionID == "" {
		actionID = result.SkillID
	}
	lowerID := strings.ToLower(actionID)
	switch {
	case strings.Contains(lowerID, "ultraskill") || strings.Contains(lowerID, "ultimate"):
		actionType = "ultimate"
	case strings.Contains(lowerID, "melee"):
		actionType = "basic_attack"
	case strings.Contains(lowerID, "branch"):
		actionType = "charged_attack"
	case strings.Contains(lowerID, "evade"):
		actionType = "dodge_counter"
	case strings.Contains(lowerID, "airattack"):
		actionType = "plunging_attack"
	case strings.Contains(lowerID, "qte"):
		actionType = "qte"
	}
	category := string(result.Category)
	if category == "" {
		category = "direct"
	}
	return groupDescriptor{
		id:         actionID,
		name:       result.Name,
		category:   category,
		actionType: actionType,
	}
}

// Missing buff states keep each scenario as its own action row.
func groupDamageResults(characterID int, build, current []damage.Result, buffs map[string]bool) []DamageGroup {
	groups := map[string]*DamageGroup{}
	add := func(results []damage.Result, currentValues bool) {
		for _, result := range results {
			if condition := result.ActionCondition; condition != nil {
				if active, known := buffs[condition.BuffID]; known && active != condition.BuffActive {
					continue
				}
			}
			descriptor, ok := damageGroupDescriptor(characterID, result)
			if !ok {
				continue
			}
			group := groups[descriptor.id]
			if group == nil {
				group = &DamageGroup{ID: descriptor.id, Name: descriptor.name, Description: descriptor.description, Category: descriptor.category, ActionType: descriptor.actionType, MaxStacks: descriptor.maxStacks, ActionCondition: result.ActionCondition}
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
				group.Instances += result.Hits
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
