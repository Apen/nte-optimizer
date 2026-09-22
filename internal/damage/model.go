package damage

// Confidence describes how strongly a combat rule is supported. Unknown values
// are deliberately preserved instead of being silently interpreted as zero.
type Confidence string

const (
	ConfidenceGameData   Confidence = "game_data"
	ConfidenceMeasured   Confidence = "measured"
	ConfidenceDocumented Confidence = "documented"
	ConfidenceInferred   Confidence = "inferred"
	ConfidenceUnknown    Confidence = "unknown"
)

type Provenance struct {
	Confidence Confidence `json:"confidence"`
	Source     string     `json:"source"`
	SourceID   string     `json:"source_id,omitempty"`
	Version    string     `json:"version,omitempty"`
	ImportedAt string     `json:"imported_at,omitempty"`
}

type Category string

const (
	CategoryDirect   Category = "direct"
	CategoryDOT      Category = "dot"
	CategoryReaction Category = "reaction"
	CategoryBreak    Category = "break"
	CategorySpecial  Category = "special"
)

type ScalingStat string

const (
	ScalingAttack  ScalingStat = "attack"
	ScalingHP      ScalingStat = "hp"
	ScalingDefense ScalingStat = "defense"
)

type Stats struct {
	Attack             float64   `json:"attack"`
	HP                 float64   `json:"hp"`
	Defense            float64   `json:"defense"`
	CritRate           float64   `json:"crit_rate"`
	CritDamage         float64   `json:"crit_damage"`
	GeneralDamage      float64   `json:"general_damage"`
	ElementDamage      float64   `json:"element_damage"`
	SkillDamage        float64   `json:"skill_damage"`
	StateDamage        float64   `json:"state_damage"`
	Vulnerability      float64   `json:"vulnerability"`
	DefenseIgnore      float64   `json:"defense_ignore"`
	DefenseReduction   float64   `json:"defense_reduction"`
	ResistancePen      float64   `json:"resistance_penetration"`
	IndependentBonuses []float64 `json:"independent_bonuses,omitempty"`
	DOTFinalBonuses    []float64 `json:"dot_final_bonuses,omitempty"`
}

type Enemy struct {
	Level         int        `json:"level"`
	DefenseBase   *float64   `json:"defense_base,omitempty"`
	DefenseUp     float64    `json:"defense_up,omitempty"`
	DefenseAdd    float64    `json:"defense_add,omitempty"`
	Resistance    float64    `json:"resistance"`
	BreakLimit    float64    `json:"break_limit,omitempty"`
	Approximation string     `json:"approximation,omitempty"`
	Provenance    Provenance `json:"provenance"`
}

type TimingRule struct {
	IntervalSeconds float64 `json:"interval_seconds,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Ticks           int     `json:"ticks,omitempty"`
	MaxStacks       int     `json:"max_stacks,omitempty"`
}

type Instance struct {
	ID            string      `json:"id"`
	CharacterID   string      `json:"character_id,omitempty"`
	SkillID       string      `json:"skill_id,omitempty"`
	Name          string      `json:"name"`
	Category      Category    `json:"category"`
	Element       string      `json:"element,omitempty"`
	Scaling       ScalingStat `json:"scaling"`
	Coefficient   float64     `json:"coefficient"`
	Hits          int         `json:"hits,omitempty"`
	FixedCritRate *float64    `json:"fixed_crit_rate,omitempty"`
	Timing        TimingRule  `json:"timing,omitempty"`
	Provenance    Provenance  `json:"provenance"`
}

type Multipliers struct {
	ScalingStat       float64 `json:"scaling_stat"`
	Coefficient       float64 `json:"coefficient"`
	DamageZone        float64 `json:"damage_zone"`
	DefenseZone       float64 `json:"defense_zone"`
	ResistanceZone    float64 `json:"resistance_zone"`
	VulnerabilityZone float64 `json:"vulnerability_zone"`
	IndependentZone   float64 `json:"independent_zone"`
	DOTFinalZone      float64 `json:"dot_final_zone"`
	CritRate          float64 `json:"crit_rate"`
	CritDamage        float64 `json:"crit_damage"`
}

type Result struct {
	InstanceID string      `json:"instance_id"`
	SkillID    string      `json:"skill_id,omitempty"`
	Name       string      `json:"name"`
	Category   Category    `json:"category"`
	NonCrit    float64     `json:"non_crit"`
	Crit       float64     `json:"crit"`
	Expected   float64     `json:"expected"`
	Hits       int         `json:"hits"`
	Ticks      int         `json:"ticks,omitempty"`
	Stacks     int         `json:"stacks,omitempty"`
	Total      float64     `json:"total"`
	Factors    Multipliers `json:"factors"`
	Warnings   []string    `json:"warnings,omitempty"`
	Provenance Provenance  `json:"provenance"`
}
