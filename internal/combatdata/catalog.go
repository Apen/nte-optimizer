// Package combatdata consumes numerical builder catalogs without simulating
// gameplay, activation, animation order or automatic uptime.
package combatdata

import (
	"encoding/json"
	"fmt"
	"os"

	"nte-optimizer/internal/datafiles"
)

type Label struct {
	Table string `json:"table"`
	Key   string `json:"key"`
}
type SkillCatalog struct {
	SchemaVersion int                  `json:"schema_version"`
	MaxSkillLevel int                  `json:"max_skill_level"`
	Characters    map[string]Character `json:"characters"`
}
type Character struct {
	Status    string    `json:"status,omitempty"`
	Abilities []Ability `json:"abilities"`
}
type Ability struct {
	ID               string            `json:"ability_id"`
	Category         string            `json:"category"`
	Name             Label             `json:"name"`
	ActivationEffect *ActivationEffect `json:"activation_effect"`
	Lines            []Line            `json:"lines"`
}
type ActivationEffect struct {
	EffectID string `json:"effect_id"`
	ClassID  string `json:"class_id"`
	File     string `json:"file"`
	Status   string `json:"status"`
}
type Line struct {
	ID         string      `json:"id"`
	Label      Label       `json:"label"`
	Format     Label       `json:"format"`
	Status     string      `json:"status"`
	Parameters []Parameter `json:"parameters"`
}
type Parameter struct {
	DamageCalculation *DisplayDamageCalculation `json:"damage_calculation,omitempty"`
	Index             int                       `json:"index"`
	SourceID          string                    `json:"source_id"`
	SourceKind        string                    `json:"source_kind"`
	SkillLevelSource  string                    `json:"skill_level_source,omitempty"`
	AssociationStatus string                    `json:"association_status,omitempty"`
	AssociationReason string                    `json:"association_reason,omitempty"`
	Percent           bool                      `json:"percent"`
	Scaling           string                    `json:"scaling,omitempty"`
	Status            string                    `json:"status"`
	InputStatus       string                    `json:"input_status,omitempty"`
	Values            []float64                 `json:"values_by_level,omitempty"`
	CharacterValues   []float64                 `json:"values_by_character_level,omitempty"`
	Curve             *Curve                    `json:"curve,omitempty"`
	Multiplier        *float64                  `json:"multiplier,omitempty"`
	Input             *Input                    `json:"input,omitempty"`
}
type DisplayDamageCalculation struct {
	Status         string            `json:"status"`
	DamageEffectID *string           `json:"damage_effect_id"`
	Supported      bool              `json:"supported"`
	Reason         string            `json:"reason"`
	Provenance     []json.RawMessage `json:"provenance"`
}
type Curve struct {
	Interpolation string     `json:"InterpMode"`
	Keys          []CurveKey `json:"Keys"`
	PreInfinity   string     `json:"PreInfinityExtrap"`
	PostInfinity  string     `json:"PostInfinityExtrap"`
}
type CurveKey struct {
	Time  float64 `json:"Time"`
	Value float64 `json:"Value"`
}
type Input struct {
	ID              string   `json:"id,omitempty"`
	Kind            string   `json:"kind"`
	Minimum         *float64 `json:"minimum,omitempty"`
	Maximum         *float64 `json:"maximum,omitempty"`
	Selection       string   `json:"selection,omitempty"`
	StatID          string   `json:"stat_id,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	SourceParameter string   `json:"source_parameter,omitempty"`
	Actor           string   `json:"actor,omitempty"`
	SemanticStatus  string   `json:"semantic_status,omitempty"`
}
type Target struct {
	PropertyID string `json:"property_id"`
	Operation  string `json:"operation"`
	Percent    bool   `json:"percent"`
}
type Recipients struct {
	Scope                 string `json:"scope"`
	IncludeCaster         bool   `json:"include_caster"`
	IncludeSourceIfActive bool   `json:"include_source_if_active,omitempty"`
}
type Unlock struct {
	Level        int `json:"level"`
	Breakthrough int `json:"required_breakthrough"`
	Awakening    int `json:"required_awakening"`
}
type Conditions struct {
	Trigger                  string   `json:"trigger,omitempty"`
	ReviewedCondition        string   `json:"reviewed_condition,omitempty"`
	DescriptionKey           string   `json:"description_key,omitempty"`
	PassiveAbilityID         string   `json:"passive_ability_id,omitempty"`
	TargetState              string   `json:"target_state,omitempty"`
	ExclusiveGroup           string   `json:"exclusive_group,omitempty"`
	RandomOutcomeGroup       string   `json:"random_outcome_group,omitempty"`
	SelectedRandomOutcome    string   `json:"selected_random_outcome,omitempty"`
	RequiresCasterActive     bool     `json:"requires_caster_active,omitempty"`
	Selection                string   `json:"selection,omitempty"`
	CandidateAwakening       *int     `json:"candidate_awakening_rank,omitempty"`
	RequiredAwakening        int      `json:"required_awakening,omitempty"`
	ActiveCharacterOnly      bool     `json:"active_character_only,omitempty"`
	RequiresPassive          bool     `json:"requires_passive_unlocked,omitempty"`
	Unlocks                  []Unlock `json:"passive_unlock_requirements,omitempty"`
	DamageFamily             string   `json:"damage_family,omitempty"`
	DamageApplicationFilter  string   `json:"damage_application_filter,omitempty"`
	RequiredDamageTags       []string `json:"required_damage_tags,omitempty"`
	RequiredSpawnAbilityTags []string `json:"required_spawn_ability_tags,omitempty"`
	RequiresSourceTags       bool     `json:"requires_source_tag_filters,omitempty"`
	RequiredDamageEffectID   string   `json:"required_damage_effect_id,omitempty"`
	SourceCounterLimit       int      `json:"source_counter_limit,omitempty"`
	RemoveOnLeaveBattle      bool     `json:"remove_on_leave_battle,omitempty"`
}
type BuffCalculation struct {
	Kind             string    `json:"kind"`
	Source           string    `json:"source,omitempty"`
	SourcePropertyID string    `json:"source_property_id,omitempty"`
	Values           []float64 `json:"values_by_level,omitempty"`
	Ratios           []float64 `json:"ratios_by_level,omitempty"`
	PerCount         []float64 `json:"values_per_count_by_level,omitempty"`
	Caps             []float64 `json:"caps_by_level,omitempty"`
	Counter          *Input    `json:"counter,omitempty"`
	CapPolicy        string    `json:"cap_policy,omitempty"`
}
type Stacking struct {
	GroupID            string `json:"group_id"`
	Limit              int    `json:"effect_stack_limit"`
	SameCasterPolicy   string `json:"same_caster_policy"`
	MixedSourcesPolicy string `json:"mixed_sources_policy"`
	ValuesPerStack     bool   `json:"values_per_stack"`
}
type Buff struct {
	Trigger            string          `json:"trigger,omitempty"`
	ID                 string          `json:"id"`
	CharacterID        int             `json:"character_id"`
	AbilityID          string          `json:"ability_id"`
	LevelSource        string          `json:"skill_level_source"`
	Target             Target          `json:"target"`
	Calculation        BuffCalculation `json:"calculation"`
	Recipients         Recipients      `json:"recipients"`
	DurationPolicy     string          `json:"duration_policy"`
	Durations          []float64       `json:"duration_seconds_by_level"`
	ActiveWindows      []float64       `json:"active_window_seconds_by_level"`
	Conditions         Conditions      `json:"conditions"`
	Requirements       []string        `json:"application_requirements"`
	DamageRequirements []string        `json:"damage_application_requirements"`
	Supersedes         string          `json:"supersedes_effect_id"`
	Combination        json.RawMessage `json:"combination"`
	Projection         json.RawMessage `json:"application_projection"`
	Lifecycle          json.RawMessage `json:"lifecycle"`
	SourceTags         json.RawMessage `json:"source_tag_requirements"`
	TargetTags         json.RawMessage `json:"target_tag_requirements"`
	LevelIndependent   bool            `json:"level_independent"`
	Stacking           Stacking        `json:"stacking"`
	ModifierScope      string          `json:"modifier_scope,omitempty"`
	DamageEffectIDs    []string        `json:"applies_to_damage_effect_ids,omitempty"`
	Assumptions        []string        `json:"assumptions"`
	// Keep the full condition contract for diagnostics, including native predicates.
	ConditionDetails json.RawMessage `json:"condition_details,omitempty"`
}

func (b *Buff) UnmarshalJSON(raw []byte) error {
	type plain Buff
	var value plain
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	var fields struct {
		Conditions json.RawMessage `json:"conditions"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	*b = Buff(value)
	b.ConditionDetails = fields.Conditions
	return nil
}

type BuffCatalog struct {
	SchemaVersion        int                   `json:"schema_version"`
	Effects              []Buff                `json:"effects"`
	AttributeExtractions []AttributeExtraction `json:"attribute_extraction_effects"`
}
type ScenarioParameter struct {
	Value            json.RawMessage `json:"value"`
	Values           []float64       `json:"values_by_source_level,omitempty"`
	LevelIndependent bool            `json:"level_independent"`
	LevelBinding     *struct {
		AbilityID string `json:"ability_id"`
		Status    string `json:"status"`
	} `json:"level_binding,omitempty"`
	SourceStatus string `json:"source_status"`
}
type ActorSelection struct {
	Kind                string   `json:"kind"`
	Context             string   `json:"selection_context_actor"`
	CharacterParameters []string `json:"character_id_parameters"`
}
type Scenario struct {
	ID                           string                       `json:"id"`
	CharacterID                  int                          `json:"character_id"`
	AbilityID                    string                       `json:"ability_id"`
	Kind                         string                       `json:"kind"`
	DamageEffectID               *string                      `json:"damage_effect_id"`
	CandidateDamageEffectID      string                       `json:"candidate_damage_effect_id,omitempty"`
	ResultStage                  string                       `json:"result_stage"`
	Parameters                   map[string]ScenarioParameter `json:"parameters"`
	Inputs                       map[string]Input             `json:"inputs"`
	Calculation                  json.RawMessage              `json:"calculation"`
	Conditions                   json.RawMessage              `json:"conditions"`
	Assumptions                  []string                     `json:"assumptions"`
	Status                       string                       `json:"status"`
	NativeExecutionSupported     bool                         `json:"native_execution_supported"`
	AutomaticActivationSupported bool                         `json:"automatic_activation_supported"`
	SourceActorSelection         *ActorSelection              `json:"source_actor_selection,omitempty"`
	Recipients                   *Recipients                  `json:"recipients,omitempty"`
	SharedGroup                  string                       `json:"shared_application_group,omitempty"`
}
type ReactionCatalog struct {
	SchemaVersion       int                 `json:"schema_version"`
	CharacterExtensions []ReactionExtension `json:"character_extensions"`
	DamageScenarios     []Scenario          `json:"damage_scenarios"`
	SupportScenarios    []Scenario          `json:"support_scenarios"`
	Summons             SummonCatalog       `json:"summons"`
}

// Extensions remain diagnostic inputs, not automatically active reaction bonuses.
type ReactionExtension struct {
	ID                           string             `json:"id"`
	CharacterID                  int                `json:"character_id"`
	AbilityID                    string             `json:"ability_id"`
	NumericParameters            map[string]float64 `json:"numeric_parameters"`
	AutomaticActivationSupported bool               `json:"automatic_activation_supported"`
	Status                       string             `json:"status"`
}
type SummonCatalog struct {
	Profiles         map[string]json.RawMessage `json:"profiles"`
	SelectedVariants []json.RawMessage          `json:"selected_variants"`
	SpawnPaths       []json.RawMessage          `json:"spawn_paths"`
}
type Modifier struct {
	ID            string    `json:"id"`
	CharacterID   string    `json:"character_id"`
	EffectID      string    `json:"effect_id"`
	PropertyID    string    `json:"property_id"`
	Operation     string    `json:"operation"`
	ValuesByLevel []float64 `json:"values_by_level"`
	DisplayUnit   struct {
		Percent *bool `json:"percent"`
	} `json:"display_unit"`
	PublishedIDs                  []string        `json:"published_ids"`
	Status                        string          `json:"status"`
	SourceKind                    string          `json:"source_kind"`
	Magnitude                     json.RawMessage `json:"magnitude"`
	Transform                     json.RawMessage `json:"magnitude_transform"`
	ExpressionProjection          json.RawMessage `json:"expression_projection"`
	ApplicationPolicy             json.RawMessage `json:"application_policy"`
	AutomaticApplicationSupported bool            `json:"automatic_application_supported"`
}
type ModifierCatalog struct {
	SchemaVersion int        `json:"schema_version"`
	Entries       []Modifier `json:"entries"`
}
type Catalog struct {
	Skills    SkillCatalog
	Buffs     BuffCatalog
	Reactions ReactionCatalog
	Modifiers ModifierCatalog
}

func read(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
func Load(dataDir string) (Catalog, error) {
	layout := datafiles.New(dataDir)
	var c Catalog
	for _, item := range []struct {
		name  string
		value any
	}{
		{"skill_values", &c.Skills}, {"skill_buffs", &c.Buffs}, {"reaction_values", &c.Reactions}, {"modifier_values", &c.Modifiers},
	} {
		if err := read(layout.Game("combat", item.name+".json"), item.value); err != nil {
			return c, err
		}
	}
	if c.Skills.SchemaVersion != 1 || c.Buffs.SchemaVersion != 1 || c.Reactions.SchemaVersion != 1 || c.Modifiers.SchemaVersion != 1 {
		return c, fmt.Errorf("unsupported numerical combat schema")
	}
	return c, nil
}

func LoadBuffs(dataDir string) (BuffCatalog, error) {
	var catalog BuffCatalog
	if err := read(datafiles.New(dataDir).Game("combat", "skill_buffs.json"), &catalog); err != nil {
		return catalog, err
	}
	if catalog.SchemaVersion != 1 {
		return catalog, fmt.Errorf("unsupported skill buff schema")
	}
	return catalog, nil
}

func LoadModifiers(dataDir string) (ModifierCatalog, error) {
	var catalog ModifierCatalog
	if err := read(datafiles.New(dataDir).Game("combat", "modifier_values.json"), &catalog); err != nil {
		return catalog, err
	}
	if catalog.SchemaVersion != 1 {
		return catalog, fmt.Errorf("unsupported modifier value schema")
	}
	return catalog, nil
}
