package app

import (
	"fmt"
	"slices"
	"strconv"

	"nte-optimizer/internal/apperrors"
	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

type ArcConditionKey struct {
	CharacterID int    `json:"character_id"`
	ForkID      string `json:"fork_id"`
	Trigger     string `json:"trigger"`
}

type ArcConditionSelection struct {
	ArcConditionKey
	Stacks int `json:"stacks"`
}

type ArcConditionOption struct {
	ArcConditionKey
	ArcName   string `json:"arc_name"`
	MaxStacks int    `json:"max_stacks"`
}

type arcScenarioEffect struct {
	key          ArcConditionKey
	weapon       decoded.Weapon
	effect       decoded.ForkConditionalEffect
	matchingTags []string
}

func (s OptimizerService) collectArcScenarioEffects(catalog decoded.ForkCatalog, activeID int, activeArc *decoded.Weapon, team *teamScenario) ([]arcScenarioEffect, error) {
	var effects []arcScenarioEffect
	add := func(characterID int, weapon *decoded.Weapon) {
		if weapon == nil {
			return
		}
		definition := catalog.Forks[weapon.ForkID]
		if _, rankKnown := definition.EffectsByStar[strconv.Itoa(weapon.Star)]; !rankKnown {
			return
		}
		for _, effect := range definition.ConditionalEffects {
			switch effect.Scope {
			case "wearer", "wearer_summon_damage", "wearer_filtered_damage":
				if characterID != activeID {
					continue
				}
			case "other_allies":
				if characterID == activeID {
					continue
				}
			case "team_including_wearer", "all_field_summon_damage":
			default:
				continue
			}
			effects = append(effects, arcScenarioEffect{
				key:    ArcConditionKey{CharacterID: characterID, ForkID: weapon.ForkID, Trigger: effect.Trigger},
				weapon: *weapon, effect: effect,
			})
		}
	}
	add(activeID, activeArc)
	if team != nil {
		for _, ally := range team.allies {
			add(ally.CharacterID, team.gear[ally.CharacterID].arc)
		}
	}
	if !slices.ContainsFunc(effects, func(effect arcScenarioEffect) bool { return effect.effect.HasTags() }) {
		return effects, nil
	}
	damageCatalog, err := damage.ReadCatalog(datafiles.New(s.DataDir).Damage())
	if err != nil {
		return nil, err
	}
	instances := damageCatalog.Characters[strconv.Itoa(activeID)].Instances
	eligible := effects[:0]
	for _, entry := range effects {
		if entry.effect.HasTags() {
			index := slices.IndexFunc(instances, func(instance damage.TieredInstance) bool {
				return entry.effect.Matches(instance.DamageTags)
			})
			if index < 0 {
				continue
			}
			entry.matchingTags = instances[index].DamageTags
		}
		eligible = append(eligible, entry)
	}
	return eligible, nil
}

func arcConditionOptions(effects []arcScenarioEffect, labels ntelocale.Catalog) []ArcConditionOption {
	var options []ArcConditionOption
	for _, effect := range effects {
		if effect.key.Trigger == "equipped" || slices.ContainsFunc(options, func(option ArcConditionOption) bool { return option.ArcConditionKey == effect.key }) {
			continue
		}
		options = append(options, ArcConditionOption{
			ArcConditionKey: effect.key,
			ArcName:         cleanArcName(labels.ItemName(effect.weapon.ForkID, effect.weapon.Name)),
			MaxStacks:       effect.effect.SelectableStacks(),
		})
	}
	return options
}

func arcConditionSelections(effects []arcScenarioEffect, input []ArcConditionSelection, labels ntelocale.Catalog) (map[ArcConditionKey]int, error) {
	options := arcConditionOptions(effects, labels)
	selected := make(map[ArcConditionKey]int, len(input))
	for _, item := range input {
		index := slices.IndexFunc(options, func(option ArcConditionOption) bool { return option.ArcConditionKey == item.ArcConditionKey })
		if index < 0 || item.Stacks < 1 || item.Stacks > options[index].MaxStacks || selected[item.ArcConditionKey] != 0 {
			return nil, invalidArcConditions(fmt.Errorf("invalid confirmation for character %d, Arc %q, trigger %q", item.CharacterID, item.ForkID, item.Trigger))
		}
		selected[item.ArcConditionKey] = item.Stacks
	}
	return selected, nil
}

func invalidArcConditions(err error) error {
	return apperrors.New("invalid_arc_conditions", err)
}

type arcScenarioResult struct {
	rows   []TeamPreviewEffect
	tagged []taggedArcBuff
}

type taggedArcBuff struct {
	filter       decoded.ForkDamageFilter
	stat         nte.Stat
	alwaysActive bool
}

func applyArcScenario(catalog decoded.ForkCatalog, activeID int, effects []arcScenarioEffect, selected map[ArcConditionKey]int, labels ntelocale.Catalog, sources map[string][]nte.Stat) (arcScenarioResult, error) {
	var result arcScenarioResult
	// A trigger such as an Ultimate that controls enemies also confirms the Ultimate itself.
	confirmed := make(map[ArcConditionKey]int, len(selected))
	for key, stacks := range selected {
		confirmed[key] = stacks
	}
	for _, effect := range effects {
		if effect.effect.ConfirmsTrigger != "" && selected[effect.key] > 0 {
			implied := effect.key
			implied.Trigger = effect.effect.ConfirmsTrigger
			confirmed[implied] = max(confirmed[implied], 1)
		}
	}
	seen := map[string]decoded.ForkConditionalValue{}
	for _, entry := range effects {
		effect := entry.effect
		stacks := confirmed[entry.key]
		if effect.Trigger == "equipped" {
			stacks = 1
		}
		if effect.SupersededByTrigger != "" {
			superseding := entry.key
			superseding.Trigger = effect.SupersededByTrigger
			if confirmed[superseding] > 0 {
				stacks = 0
			}
		}
		if stacks == 0 && !effect.LegacyConditionalProjection {
			continue
		}
		value, err := catalog.ResolveConditionalEffect(entry.weapon, effect.ID, decoded.ForkEffectSelection{
			ConditionsConfirmed: true,
			RecipientIsWearer:   entry.key.CharacterID == activeID,
			DamageTags:          entry.matchingTags,
			Stacks:              max(stacks, 1),
			ReplaceLegacy:       effect.LegacyConditionalProjection,
		})
		if err != nil {
			return result, invalidArcConditions(err)
		}
		if effect.LegacyConditionalProjection {
			stats := sources["weapon_conditional"]
			match := slices.Index(stats, value.Stat)
			if match < 0 {
				return result, invalidArcConditions(fmt.Errorf("missing legacy projection for Arc effect %q", effect.ID))
			}
			sources["weapon_conditional"] = slices.Delete(slices.Clone(stats), match, match+1)
		}
		if stacks == 0 {
			continue
		}
		if previous, exists := seen[effect.ID]; exists {
			if effect.MixedSourcePolicy == "non_stacking" && previous.Stat == value.Stat &&
				previous.Duration != nil && value.Duration != nil && *previous.Duration == *value.Duration {
				continue
			}
			return result, invalidArcConditions(fmt.Errorf("unresolved combination of multiple sources for Arc effect %q", effect.ID))
		}
		seen[effect.ID] = value
		stat := value.Stat
		if effect.Target.ProjectionPropertyID != "" {
			stat.PropertyID = effect.Target.ProjectionPropertyID
		}
		if effect.HasTags() {
			result.tagged = append(result.tagged, taggedArcBuff{
				filter: effect.ForkDamageFilter, stat: stat, alwaysActive: effect.Trigger == "equipped",
			})
		} else {
			sources[optimizer.ArcScenarioSource] = append(sources[optimizer.ArcScenarioSource], stat)
		}
		name := labels.CharacterName(entry.key.CharacterID, "")
		row := TeamPreviewEffect{
			Kind: "arc", ID: effect.ID, CharacterID: entry.key.CharacterID, Trigger: effect.Trigger,
			Star: entry.weapon.Star, SourceNames: []string{cleanCharacterName(name)},
			EquipmentName: cleanArcName(labels.ItemName(entry.weapon.ForkID, entry.weapon.Name)),
			Included:      true, ProjectedStats: []nte.Stat{stat},
			DamageTags: effect.RequiredDamageTags, AnyDamageTags: effect.AnyDamageTags,
			NonStacking: effect.MixedSourcePolicy == "non_stacking",
		}
		if value.Duration != nil {
			row.Duration = int(*value.Duration)
		}
		result.rows = append(result.rows, row)
	}
	return result, nil
}
