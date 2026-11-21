package app

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"

	"nte-optimizer/internal/combatdata"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

type awakeningResonance struct {
	RequiredAwakening int            `json:"required_awakening"`
	SkillLevels       map[string]int `json:"skill_levels,omitempty"`
	StatEffects       []struct {
		ID         string `json:"id"`
		PropertyID string `json:"property_id,omitempty"`
		Source     string `json:"source,omitempty"`
	} `json:"stat_effects,omitempty"`
	stats []resonanceStat
}

type resonanceStat struct {
	id   string
	stat nte.Stat
	team bool
	self bool
}

type awakeningResonances map[int][]awakeningResonance

func readAwakeningResonances(dataDir string) (awakeningResonances, error) {
	path := datafiles.New(dataDir).Game("characters", "awakening_resonances.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // Older data sets have no catalog of permanent resonances.
	}
	if err != nil {
		return nil, err
	}
	var catalog struct {
		SchemaVersion int                 `json:"schema_version"`
		Characters    awakeningResonances `json:"characters"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("decode awakening resonances: %w", err)
	}
	if catalog.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported awakening resonance schema %d", catalog.SchemaVersion)
	}
	for characterID, resonances := range catalog.Characters {
		ranks := map[int]bool{}
		effects := map[string]bool{}
		for _, resonance := range resonances {
			rank := resonance.RequiredAwakening
			if rank <= 0 || ranks[rank] {
				return nil, fmt.Errorf("invalid or duplicate awakening resonance threshold %d for character %d", rank, characterID)
			}
			ranks[rank] = true
			for ability, bonus := range resonance.SkillLevels {
				if ability == "" || bonus <= 0 {
					return nil, fmt.Errorf("invalid awakening skill level bonus for ability %q on character %d", ability, characterID)
				}
			}
			for _, ref := range resonance.StatEffects {
				if ref.ID == "" || effects[ref.ID] {
					return nil, fmt.Errorf("invalid or duplicate awakening resonance effect %q for character %d", ref.ID, characterID)
				}
				effects[ref.ID] = true
			}
		}
	}
	return catalog.Characters, nil
}

func resolveAwakeningResonances(dataDir string) (awakeningResonances, error) {
	catalog, err := readAwakeningResonances(dataDir)
	if err != nil || len(catalog) == 0 {
		return catalog, err
	}
	var modifiers map[string]combatdata.Modifier
	buffs, err := combatdata.LoadBuffs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load awakening resonance effects: %w", err)
	}
	byID := make(map[string]combatdata.Buff, len(buffs.Effects))
	for _, effect := range buffs.Effects {
		byID[effect.ID] = effect
	}
	for characterID, resonances := range catalog {
		for i := range resonances {
			resonance := &resonances[i]
			actor := combatdata.Actor{CharacterID: characterID, Awakening: resonance.RequiredAwakening}
			for _, ref := range resonance.StatEffects {
				if ref.Source == "modifier_values" {
					if modifiers == nil {
						data, err := combatdata.LoadModifiers(dataDir)
						if err != nil {
							return nil, fmt.Errorf("load awakening modifier values: %w", err)
						}
						modifiers = make(map[string]combatdata.Modifier, len(data.Entries))
						for _, entry := range data.Entries {
							modifiers[entry.ID] = entry
						}
					}
					stat, err := permanentResonanceModifier(modifiers[ref.ID], characterID)
					if err != nil {
						return nil, fmt.Errorf("read awakening resonance effect %q: %w", ref.ID, err)
					}
					if ref.PropertyID != "" {
						stat.PropertyID = ref.PropertyID
					}
					resonance.stats = append(resonance.stats, resonanceStat{id: ref.ID, stat: stat, self: true})
					continue
				}
				effect, ok := byID[ref.ID]
				team := effect.Recipients.Scope == "team"
				validRecipient := effect.Recipients.Scope == "caster" || (team &&
					(effect.Conditions.ReviewedCondition == "caster_on_team" || effect.Conditions.ReviewedCondition == "caster_on_team_and_alive"))
				if ref.Source != "" || !ok || !effect.Unlocked(actor) || !validRecipient ||
					effect.DurationPolicy != "infinite" || effect.Calculation.Kind != "flat_stat" || !effect.LevelIndependent ||
					effect.Conditions.ActiveCharacterOnly || effect.Conditions.RequiresCasterActive || effect.Conditions.RequiresSourceTags ||
					len(effect.Requirements) > 0 || len(effect.DamageRequirements) > 0 {
					return nil, fmt.Errorf("invalid permanent awakening resonance effect %q for character %d", ref.ID, characterID)
				}
				value, err := effect.PreviewMagnitude(actor)
				if err != nil {
					return nil, fmt.Errorf("read awakening resonance effect %q: %w", ref.ID, err)
				}
				property := ref.PropertyID
				if property == "" {
					property = value.Target.PropertyID
				}
				resonance.stats = append(resonance.stats, resonanceStat{
					id:   ref.ID,
					stat: nte.Stat{PropertyID: property, Value: value.Value, Percent: value.Target.Percent},
					team: team,
					self: !team || effect.Recipients.IncludeCaster,
				})
			}
		}
	}
	return catalog, nil
}

func permanentResonanceModifier(entry combatdata.Modifier, characterID int) (nte.Stat, error) {
	if entry.CharacterID != strconv.Itoa(characterID) || entry.PropertyID == "" || entry.Operation != "EGameplayModOp::Additive" ||
		entry.DisplayUnit.Percent == nil || len(entry.ValuesByLevel) == 0 {
		return nte.Stat{}, fmt.Errorf("expected an additive modifier with known units for character %d", characterID)
	}
	value := entry.ValuesByLevel[0]
	for _, levelValue := range entry.ValuesByLevel[1:] {
		if levelValue != value {
			return nte.Stat{}, fmt.Errorf("expected a level-independent modifier")
		}
	}
	return nte.Stat{PropertyID: entry.PropertyID, Value: value, Percent: *entry.DisplayUnit.Percent}, nil
}

func (s OptimizerService) loadAwakeningResonances() (awakeningResonances, error) {
	if s.static == nil {
		return resolveAwakeningResonances(s.DataDir)
	}
	s.static.resonanceOnce.Do(func() {
		s.static.resonances, s.static.resonanceErr = resolveAwakeningResonances(s.DataDir)
	})
	return s.static.resonances, s.static.resonanceErr
}

func (catalog awakeningResonances) apply(character *decoded.Character) []nte.Stat {
	var stats []nte.Stat
	skillsCopied := false
	for _, resonance := range catalog[character.CharacterID] {
		// Resonance thresholds count awakened nodes, independently of their order.
		if character.AwakenLevel < resonance.RequiredAwakening {
			continue
		}
		for _, effect := range resonance.stats {
			if effect.self {
				stats = append(stats, effect.stat)
			}
		}
		for i, skill := range character.Skills {
			bonus := resonance.SkillLevels[skill.AbilityID]
			if bonus == 0 || skill.Level <= 0 {
				continue
			}
			if !skillsCopied {
				character.Skills = slices.Clone(character.Skills)
				skillsCopied = true
			}
			character.Skills[i].Level += bonus
		}
	}
	return stats
}

func (catalog awakeningResonances) teamStats(character decoded.Character) []resonanceStat {
	var stats []resonanceStat
	for _, resonance := range catalog[character.CharacterID] {
		if character.AwakenLevel < resonance.RequiredAwakening {
			continue
		}
		for _, effect := range resonance.stats {
			if effect.team {
				stats = append(stats, effect)
			}
		}
	}
	return stats
}

func (catalog awakeningResonances) effectRank(characterID int, effectID string) int {
	for _, resonance := range catalog[characterID] {
		for _, ref := range resonance.StatEffects {
			if ref.ID == effectID {
				return resonance.RequiredAwakening
			}
		}
	}
	return 0
}
