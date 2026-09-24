package damage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Catalog struct {
	SchemaVersion int                         `json:"schema_version"`
	GeneratedAt   string                      `json:"generated_at"`
	Source        string                      `json:"source"`
	Characters    map[string]CharacterCatalog `json:"characters"`
}

type CharacterCatalog struct {
	CharacterID int              `json:"character_id"`
	Name        string           `json:"name"`
	Instances   []TieredInstance `json:"instances"`
	Rules       CharacterRules   `json:"rules,omitempty"`
}

type CharacterRules struct {
	InstanceModifiers []InstanceModifier `json:"instance_modifiers,omitempty"`
	DerivedInstances  []DerivedInstance  `json:"derived_instances,omitempty"`
}

type InstanceModifier struct {
	MinAwakenLevel        int                             `json:"min_awaken_level"`
	InstanceIDContains    string                          `json:"instance_id_contains,omitempty"`
	CoefficientMultiplier float64                         `json:"coefficient_multiplier,omitempty"`
	StatAdditions         map[string]float64              `json:"stat_additions,omitempty"`
	CategoryStatAdditions map[Category]map[string]float64 `json:"category_stat_additions,omitempty"`
}

type DerivedInstance struct {
	ID                       string         `json:"id"`
	Kind                     string         `json:"kind"`
	Element                  string         `json:"element"`
	MagicProperty            string         `json:"magic_property"`
	CritDamageProperty       string         `json:"crit_damage_property"`
	AwakeningCritDamageBonus AwakeningBonus `json:"awakening_crit_damage_bonus,omitempty"`
}

type AwakeningBonus struct {
	MinAwakenLevel int     `json:"min_awaken_level"`
	Value          float64 `json:"value"`
}

type TieredInstance struct {
	ID                       string      `json:"id"`
	AbilityID                string      `json:"ability_id"`
	Name                     string      `json:"name"`
	Category                 Category    `json:"category"`
	Element                  string      `json:"element"`
	Scaling                  ScalingStat `json:"scaling"`
	Coefficients             []float64   `json:"coefficients"`
	FixedCritRate            *float64    `json:"fixed_crit_rate,omitempty"`
	BreakValue               float64     `json:"break_value,omitempty"`
	ClassificationConfidence Confidence  `json:"classification_confidence"`
	Provenance               Provenance  `json:"provenance"`
}

func ReadCatalog(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode damage catalog: %w", err)
	}
	if catalog.SchemaVersion != 1 {
		return Catalog{}, fmt.Errorf("unsupported damage catalog schema %d", catalog.SchemaVersion)
	}
	return catalog, nil
}

func (entry TieredInstance) Resolve(characterID string, level int) Instance {
	coefficient := 0.0
	if len(entry.Coefficients) > 0 {
		index := level - 1
		if index < 0 {
			index = 0
		}
		if index >= len(entry.Coefficients) {
			index = len(entry.Coefficients) - 1
		}
		coefficient = entry.Coefficients[index]
	}
	return Instance{
		ID:            entry.ID,
		CharacterID:   characterID,
		SkillID:       entry.AbilityID,
		Name:          entry.Name,
		Category:      entry.Category,
		Element:       entry.Element,
		Scaling:       entry.Scaling,
		Coefficient:   coefficient,
		FixedCritRate: entry.FixedCritRate,
		Provenance:    entry.Provenance,
	}
}

func StatsFromOptimizer(values map[string]float64, element string, conditional bool) Stats {
	damage := values["DamageUpGeneralBase"]
	elementSuffix := optimizerElementSuffix(element)
	elementDamage := 0.0
	resistancePen := 0.0
	if elementSuffix != "" {
		elementDamage = values["DamageUp"+elementSuffix+"Base"]
		resistancePen = values["ResistanceIgnore"+elementSuffix+"Base"]
	}
	return Stats{
		Attack:        values["AtkFinal"],
		HP:            values["HPFinal"],
		Defense:       values["DefFinal"],
		CritRate:      values["CritBase"],
		CritDamage:    values["CritDamageBase"],
		GeneralDamage: damage,
		ElementDamage: elementDamage,
		DefenseIgnore: values["DefenseIgnoreBase"],
		ResistancePen: resistancePen,
	}
}

func optimizerElementSuffix(element string) string {
	element = strings.TrimSpace(strings.TrimPrefix(element, "DAMAGE_TYPE_"))
	if element == "" {
		return ""
	}
	element = strings.ToLower(element)
	return strings.ToUpper(element[:1]) + element[1:]
}
