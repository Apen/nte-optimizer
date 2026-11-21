package app

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"nte-optimizer/internal/combatdata"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/team"
)

// AllyBuffSummary describes potential and applied ally contributions. Stat
// calculation remains in the team evaluator and temporary buff projection.
type AllyBuffSummary struct {
	ID             string   `json:"id"`
	CharacterID    int      `json:"character_id"`
	CharacterName  string   `json:"character_name"`
	AbilityName    string   `json:"ability_name,omitempty"`
	PropertyID     string   `json:"property_id"`
	Percent        bool     `json:"percent"`
	Value          *float64 `json:"value,omitempty"`
	Duration       *float64 `json:"duration_seconds,omitempty"`
	Condition      string   `json:"condition"`
	PerStack       bool     `json:"per_stack,omitempty"`
	Included       bool     `json:"included"`
	LegacyEffectID string   `json:"legacy_effect_id,omitempty"`
	abilityID      string
}

func (s OptimizerService) allyBuffSummaries(state *decoded.State, scenario *teamScenario, labels ntelocale.Catalog) ([]AllyBuffSummary, error) {
	result := []AllyBuffSummary{}
	if scenario == nil || len(scenario.allies) == 0 {
		return result, nil
	}
	catalog, err := combatdata.LoadBuffs(s.DataDir)
	if err != nil {
		return nil, err
	}
	resonances, err := s.loadAwakeningResonances()
	if err != nil {
		return nil, err
	}
	for _, ally := range scenario.allies {
		index := slices.IndexFunc(state.Characters, func(character decoded.Character) bool { return character.CharacterID == ally.CharacterID })
		if index < 0 {
			return nil, fmt.Errorf("buff source is absent from imported team")
		}
		character := state.Characters[index]
		actor, err := s.allyBuffActor(character, scenario.gear[ally.CharacterID].arc)
		if err != nil {
			return nil, err
		}
		for _, b := range catalog.Effects {
			if b.CharacterID != ally.CharacterID || b.Recipients.Scope != "team" {
				continue
			}
			b.Conditions.RequiredAwakening = max(b.Conditions.RequiredAwakening, resonances.effectRank(ally.CharacterID, b.ID))
			if !b.Unlocked(actor) {
				continue
			}
			if b.Conditions.ActiveCharacterOnly || b.Conditions.RequiresCasterActive {
				continue
			}
			ability := b.AbilityID
			if ability == "" {
				ability = b.Conditions.PassiveAbilityID
			}
			entry := AllyBuffSummary{ID: b.ID, CharacterID: character.CharacterID, CharacterName: cleanCharacterName(labels.CharacterName(character.CharacterID, character.Name)), AbilityName: localizedAbilityLabel(labels.Abilities, ability), PropertyID: b.Target.PropertyID, Percent: b.Target.Percent, Condition: allyBuffCondition(b, labels), PerStack: b.Stacking.ValuesPerStack, abilityID: ability}
			entry.Included = slices.ContainsFunc(scenario.resonanceContributions, func(effect team.Contribution) bool {
				return effect.RuleID == b.ID && slices.Contains(effect.SourceIDs, ally.CharacterID)
			})
			if magnitude, err := b.PreviewMagnitude(actor); err == nil {
				entry.Value = &magnitude.Value
				entry.Duration = magnitude.Duration
			}
			result = append(result, entry)
		}
	}
	return result, nil
}

func (s OptimizerService) allyBuffActor(character decoded.Character, arc *decoded.Weapon) (combatdata.Actor, error) {
	actor := combatdata.Actor{CharacterID: character.CharacterID, Level: character.Level, Awakening: character.AwakenLevel, Breakthrough: character.BreakthroughLevel, Levels: map[string]int{}}
	resonances, err := s.loadAwakeningResonances()
	if err != nil {
		return actor, err
	}
	resonances.apply(&character)
	for _, skill := range character.Skills {
		actor.Levels[skill.AbilityID] = skill.Level
	}
	base, known, err := s.characterBaseStats(character.CharacterID, character.Level, character.BreakthroughLevel)
	if err != nil {
		return actor, err
	}
	if !known {
		return actor, nil
	}
	actor.Stats = maps.Clone(base)
	if arc != nil {
		catalog, err := s.loadArcs()
		if err != nil {
			return actor, err
		}
		panel, _, _, _ := catalog.Stats(*arc)
		arcAttackKnown := false
		for _, stat := range panel {
			actor.Stats[stat.PropertyID] += stat.Value
			if stat.PropertyID == "AtkBase" {
				arcAttackKnown = true
			}
		}
		if !arcAttackKnown {
			delete(actor.Stats, "AtkBase")
		}
	}
	return actor, nil
}

func allyBuffCondition(b combatdata.Buff, labels ntelocale.Catalog) string {
	condition := b.Conditions.ReviewedCondition
	if condition == "" {
		condition = b.Conditions.Trigger
	}
	if condition == "" {
		condition = b.Trigger
	}
	if condition == "" {
		condition = b.Conditions.TargetState
	}
	if condition == "" {
		condition = b.Conditions.DamageFamily
	}
	text := labels.UI["ally_buff_condition_"+condition]
	if text == "" {
		text = labels.UI["ally_buff_condition_manual"]
	}
	parts := []string{text}
	rank := b.Conditions.RequiredAwakening
	if b.Conditions.CandidateAwakening != nil {
		rank = max(rank, *b.Conditions.CandidateAwakening)
	}
	if rank > 0 {
		parts = append(parts, strings.ReplaceAll(labels.UI["ally_buff_awakening"], "{level}", strconv.Itoa(rank)))
	}
	if b.ModifierScope == "damage_evaluation" {
		parts = append(parts, labels.UI["ally_buff_filtered_damage"])
	}
	if b.Supersedes != "" {
		parts = append(parts, labels.UI["ally_buff_replaces_base"])
	}
	return strings.Join(parts, " · ")
}

func matchAllyBuffProjections(summaries []AllyBuffSummary, effects []TeamPreviewEffect) []AllyBuffSummary {
	result := slices.Clone(summaries)
	for i := range result {
		entry := &result[i]
		for _, effect := range effects {
			if effect.Kind != "skill" || effect.ID != entry.abilityID || entry.Value == nil {
				continue
			}
			if effect.CharacterID != entry.CharacterID && !slices.Contains(effect.SourceIDs, entry.CharacterID) {
				continue
			}
			stats := effect.ProjectedStats
			if effect.Bonus != nil {
				stats = append(slices.Clone(stats), *effect.Bonus)
			}
			if slices.ContainsFunc(stats, func(stat nte.Stat) bool {
				return stat.PropertyID == entry.PropertyID && stat.Percent == entry.Percent && math.Abs(stat.Value-*entry.Value) < 1e-8
			}) {
				entry.LegacyEffectID = effect.ID
				entry.Included = effect.Included
				break
			}
		}
	}
	return result
}
