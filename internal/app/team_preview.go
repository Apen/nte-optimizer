package app

import (
	"fmt"
	"slices"

	"nte-optimizer/internal/apperrors"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

type TeamPreview struct {
	Applied     []TeamPreviewBonus  `json:"applied"`
	Conditional []TeamPreviewEffect `json:"conditional"`
}

type TeamPreviewBonus struct {
	RuleID    string   `json:"rule_id"`
	SourceIDs []int    `json:"source_ids"`
	Stat      nte.Stat `json:"stat"`
}

type TeamPreviewEffect struct {
	RotationAction  string     `json:"rotation_action,omitempty"`
	Included        bool       `json:"included,omitempty"`
	ProjectedStats  []nte.Stat `json:"projected_stats,omitempty"`
	SourceNames     []string   `json:"source_names,omitempty"`
	EquipmentName   string     `json:"equipment_name,omitempty"`
	projection      []nte.Stat
	projectionGroup string
	Kind            string    `json:"kind"`
	ID              string    `json:"id"`
	CharacterID     int       `json:"character_id"`
	Detail          string    `json:"detail"`
	Trigger         string    `json:"trigger"`
	Pieces          int       `json:"pieces,omitempty"`
	Star            int       `json:"star,omitempty"`
	SourceIDs       []int     `json:"source_ids,omitempty"`
	NonStacking     bool      `json:"non_stacking,omitempty"`
	Duration        int       `json:"duration_seconds,omitempty"`
	Bonus           *nte.Stat `json:"bonus,omitempty"`
}

type teamPreviewDefinition struct {
	TeamPreviewEffect
	DurationParameter string            `json:"duration_parameter"`
	BaseAttackRatio   float64           `json:"base_attack_ratio"`
	RatiosByLevel     []float64         `json:"base_attack_ratios_by_level"`
	Parameters        map[string]string `json:"projection_parameters"`
	ProjectionGroup   string            `json:"projection_group"`
	MaxAttackBonus    float64           `json:"max_attack_bonus"`
}

func (s OptimizerService) PreviewTeam(projectDir, profileID, language, arcForkID string, allies []TeamAlly) (TeamPreview, error) {
	preview := TeamPreview{Applied: []TeamPreviewBonus{}, Conditional: []TeamPreviewEffect{}}
	if len(allies) == 0 {
		return preview, nil
	}
	profiles, err := s.loadCharacters()
	if err != nil {
		return preview, err
	}
	profile, ok := profiles[profileID]
	if !ok {
		return preview, fmt.Errorf("unknown profile %q", profileID)
	}
	inv, state, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return preview, err
	}
	if !loaded {
		return preview, apperrors.New("account_required", fmt.Errorf("no account has been imported"))
	}
	builds, err := loadLocalBuilds(projectDir, true)
	if err != nil {
		return preview, err
	}
	s.teamBuilds = builds.Builds
	s.WeightOverrides = &WeightOverrides{TeamAllies: allies}
	_, currentArc, _, additional, _, err := s.resolveAccountContext(&state, profile, language)
	if err != nil {
		return preview, err
	}
	scenario, err := s.prepareTeam(inv, &state, profile.CharacterID, currentArc)
	if err != nil {
		return preview, err
	}
	catalogs, err := s.loadOptimizerCatalog()
	if err != nil {
		return preview, err
	}
	arcs, err := s.prepareArcSelection(&state, profile, currentArc, additional, arcForkID, language, optimizer.EquipmentTieBreakScale, catalogs.references)
	if err != nil {
		return preview, err
	}
	var activeArc *decoded.Weapon
	if arcs.option != nil {
		weapon := arcs.weapons[arcs.option.ID]
		activeArc = &weapon
	}
	evaluated, err := scenario.evaluate(activeArc)
	if err != nil {
		return preview, err
	}
	for _, bonus := range evaluated.Contributions {
		if bonus.RecipientID == profile.CharacterID {
			preview.Applied = append(preview.Applied, TeamPreviewBonus{RuleID: bonus.RuleID, SourceIDs: bonus.SourceIDs, Stat: bonus.Stat})
		}
	}
	preview.Conditional, err = s.conditionalTeamEffects(&state, scenario, catalogs.sets)
	return preview, err
}

func (s OptimizerService) conditionalTeamEffects(state *decoded.State, scenario *teamScenario, sets optimizer.SetCatalog) ([]TeamPreviewEffect, error) {
	effects := []TeamPreviewEffect{}
	var catalog struct {
		SchemaVersion int                     `json:"schema_version"`
		Effects       []teamPreviewDefinition `json:"effects"`
	}
	if err := readJSON(datafiles.New(s.DataDir).Game("combat", "team_effect_previews.json"), &catalog); err != nil {
		return nil, err
	}
	if catalog.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported team preview schema")
	}
	grouped := map[string]int{}
	for _, ally := range scenario.allies {
		gear := scenario.gear[ally.CharacterID]
		for _, definition := range catalog.Effects {
			effect := definition.TeamPreviewEffect
			effect.projectionGroup = definition.ProjectionGroup
			switch effect.Kind {
			case "skill":
				if effect.CharacterID != ally.CharacterID {
					continue
				}
				index := slices.IndexFunc(state.Characters, func(character decoded.Character) bool {
					return character.CharacterID == ally.CharacterID
				})
				if index >= 0 && effect.Bonus == nil {
					attack, err := s.previewSkillAttackBonus(definition, state.Characters[index], gear.arc)
					if err != nil {
						return nil, err
					}
					if attack != nil {
						effect.Bonus = &nte.Stat{PropertyID: "AtkAdd", Value: *attack}
					}
				}
			case "arc":
				if gear.arc == nil || gear.arc.ForkID != effect.ID {
					continue
				}
				effect.Star = gear.arc.Star
				if len(definition.Parameters) > 0 || definition.DurationParameter != "" {
					arcs, err := s.loadArcs()
					if err != nil {
						return nil, err
					}
					rank := arcs.Forks[gear.arc.ForkID].EffectsByStar[fmt.Sprint(gear.arc.Star)]
					for _, parameter := range rank.OrderedParameters {
						if definition.DurationParameter != "" && parameter.NameID == definition.DurationParameter && !parameter.IsPercent {
							effect.Duration = int(parameter.Value)
						}
						if property := definition.Parameters[parameter.NameID]; property != "" {
							effect.projection = append(effect.projection, nte.Stat{PropertyID: property, Value: parameter.Value, Percent: parameter.IsPercent})
						}
					}
				}
			case "set":
				if gear.cartridge == nil || gear.cartridge.SetID != effect.ID {
					continue
				}
				matched := 0
				for _, definition := range sets.Definitions {
					if definition.InventorySetID == effect.ID {
						matched = matchedRequiredGeometries(gear.modules, definition.RequiredGeometries)
						for _, bonus := range definition.Bonuses {
							if bonus.Count == effect.Pieces {
								effect.projection = bonus.ConditionalStats
								break
							}
						}
						break
					}
				}
				if matched < effect.Pieces {
					continue
				}
			default:
				return nil, fmt.Errorf("unsupported team preview effect kind %q", effect.Kind)
			}
			effect.CharacterID = ally.CharacterID
			if effect.NonStacking {
				if index, ok := grouped[effect.ID]; ok {
					effects[index].SourceIDs = append(effects[index].SourceIDs, ally.CharacterID)
					continue
				}
				grouped[effect.ID] = len(effects)
				effect.SourceIDs = []int{ally.CharacterID}
			}
			effects = append(effects, effect)
		}
	}
	return effects, nil
}

func (s OptimizerService) previewSkillAttackBonus(definition teamPreviewDefinition, character decoded.Character, weapon *decoded.Weapon) (*float64, error) {
	ratio := definition.BaseAttackRatio
	if len(definition.RatiosByLevel) > 0 {
		index := slices.IndexFunc(character.Skills, func(skill decoded.CharacterSkill) bool {
			return skill.AbilityID == definition.ID
		})
		if index < 0 {
			return nil, nil
		}
		level := character.Skills[index].Level
		if level < 1 || level > len(definition.RatiosByLevel) {
			return nil, nil
		}
		ratio = definition.RatiosByLevel[level-1]
	}
	if ratio <= 0 {
		return nil, nil
	}
	stats, ok, err := s.characterBaseStats(character.CharacterID, character.Level, character.BreakthroughLevel)
	if err != nil || !ok || stats["AtkBase"] <= 0 {
		return nil, err
	}
	baseAttack := stats["AtkBase"]
	if weapon != nil {
		catalog, err := s.loadArcs()
		if err != nil {
			return nil, err
		}
		panel, _, _, _ := catalog.Stats(*weapon)
		arcAttack := 0.0
		for _, stat := range panel {
			if stat.PropertyID == "AtkBase" {
				arcAttack += stat.Value
			}
		}
		if arcAttack <= 0 {
			return nil, nil
		}
		baseAttack += arcAttack
	}
	value := baseAttack * ratio
	if definition.MaxAttackBonus > 0 {
		value = min(value, definition.MaxAttackBonus)
	}
	return &value, nil
}

func projectTeamBuffs(stats *optimizer.StatSummary, effects []TeamPreviewEffect, set optimizer.SetDefinition, matched int) []TeamPreviewEffect {
	effects = slices.Clone(effects)
	var buffs []nte.Stat
	groups := map[string][2]int{}
	for i := range effects {
		effect := &effects[i]
		if effect.Kind == "set" && effect.NonStacking && effect.ID == set.InventorySetID && matched >= effect.Pieces {
			continue
		}
		buffs = append(buffs, effect.projection...)
		effect.ProjectedStats = effect.projection
		effect.Included = len(effect.projection) > 0
		if effect.Bonus == nil {
			continue
		}
		if effect.projectionGroup != "" {
			key := effect.projectionGroup + ":" + effect.Bonus.PropertyID
			if group, ok := groups[key]; ok {
				index, previous := group[0], group[1]
				if effect.Bonus.Value > buffs[index].Value {
					buffs[index] = *effect.Bonus
					effects[previous].Included = false
					effect.Included = true
					groups[key] = [2]int{index, i}
				}
				continue
			}
			groups[key] = [2]int{len(buffs), i}
		}
		effect.Included = true
		buffs = append(buffs, *effect.Bonus)
	}
	stats.ProjectTeamBuffs(buffs)
	return effects
}
