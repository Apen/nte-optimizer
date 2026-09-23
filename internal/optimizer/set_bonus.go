package optimizer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

type SetBonus struct {
	Count            int        `json:"count"`
	Score            float64    `json:"score"`
	Stats            []nte.Stat `json:"stats,omitempty"`
	ConditionalStats []nte.Stat `json:"conditional_stats,omitempty"`
}

type SetDefinition struct {
	ID                 string     `json:"id"`
	InventorySetID     string     `json:"inventory_set_id"`
	RequiredGeometries []string   `json:"required_geometries"`
	Bonuses            []SetBonus `json:"bonuses"`
}

type SetCatalog struct {
	Definitions map[string]SetDefinition
}

type setFile struct {
	SchemaVersion int             `json:"schema_version"`
	Sets          []SetDefinition `json:"sets"`
}

func LoadSetCatalog(path string) (SetCatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return SetCatalog{}, fmt.Errorf("read set catalog %s: %w", path, err)
	}
	var file setFile
	if err := json.Unmarshal(b, &file); err != nil {
		return SetCatalog{}, fmt.Errorf("decode set catalog %s: %w", path, err)
	}
	if file.SchemaVersion != 1 {
		return SetCatalog{}, fmt.Errorf("set catalog schema_version must be 1")
	}
	catalog := SetCatalog{Definitions: make(map[string]SetDefinition, len(file.Sets))}
	for _, definition := range file.Sets {
		if strings.EqualFold(definition.InventorySetID, definition.ID) {
			definition.InventorySetID = definition.ID
		}
		if definition.ID == "" || definition.InventorySetID == "" || len(definition.RequiredGeometries) == 0 {
			return SetCatalog{}, fmt.Errorf("invalid set definition %q", definition.ID)
		}
		if _, exists := catalog.Definitions[definition.ID]; exists {
			return SetCatalog{}, fmt.Errorf("duplicate set definition %q", definition.ID)
		}
		catalog.Definitions[definition.ID] = definition
	}
	return catalog, nil
}

type BonusResult struct {
	Score          float64
	SetBonusScore  float64
	CartridgeScore float64
	SetID          string
	CartridgeID    string
	MatchedCount   int
}

type GlobalBonusEvaluator interface {
	Evaluate([]Placement) BonusResult
	UpperBound() float64
}

type NoBonusEvaluator struct{}

func (NoBonusEvaluator) Evaluate([]Placement) BonusResult { return BonusResult{} }
func (NoBonusEvaluator) UpperBound() float64              { return 0 }

type CartridgeSetEvaluator struct {
	exact      bool
	sets       []weightedSet
	modules    map[string]nte.Module
	upperBound float64
	objectives []ObjectiveGoal
	character  scoring.Character
	additional map[string][]nte.Stat
}

// Exact mode retains every cartridge: raw cartridge score is not an
// admissible dominance test for nonlinear objectives or strict maxima.
func NewExactObjectiveEvaluator(catalog SetCatalog, cartridges []nte.Cartridge, preferences []scoring.SetPreference, modules []Candidate, character scoring.Character, refs scoring.References, goals []ObjectiveGoal, additional map[string][]nte.Stat) CartridgeSetEvaluator {
	e := NewObjectiveEvaluator(catalog, cartridges, preferences, modules, character, refs, goals, additional)
	e.exact = true
	e.character = character
	e.sets = nil
	keys := make([]string, 0, len(catalog.Definitions))
	for key := range catalog.Definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		definition := catalog.Definitions[key]
		priority := 1.0
		for _, preference := range preferences {
			if preference.SetID == key && preference.Priority > 0 {
				priority = preference.Priority
			}
		}
		for _, cartridge := range cartridges {
			if cartridge.SetID == definition.InventorySetID {
				e.sets = append(e.sets, weightedSet{definition: definition, priority: priority, cartridge: cartridge, cartridgeScore: scoring.ScoreCartridgeDetailed(cartridge, character, refs).Total})
			}
		}
	}
	e.upperBound = math.Inf(1)
	return e
}

func exceedsMaximum(values map[string]float64, goals []ObjectiveGoal) bool {
	for _, goal := range goals {
		if exceedsLimit(values[goal.PropertyID], goal.Maximum) {
			return true
		}
	}
	return false
}

func belowToleranceFloor(value float64, goal ObjectiveGoal) bool {
	if !goal.StrictMinimum {
		return false
	}
	if goal.StrictFloor > 0 {
		return value < goal.StrictFloor-1e-12*math.Max(1, math.Abs(goal.StrictFloor))
	}
	if goal.Minimum <= 0 {
		return false
	}
	tolerance := goal.Tolerance
	if tolerance < 0 {
		tolerance = 0
	} else if tolerance > 1 {
		tolerance = 1
	}
	floor := goal.Minimum * (1 - tolerance)
	return value < floor-1e-12*math.Max(1, math.Abs(floor))
}

// Ignore only binary summation noise at a strict decimal boundary, not the
// user-configured target tolerance (which never relaxes a maximum).
func exceedsLimit(value, maximum float64) bool {
	return maximum > 0 && value > maximum+1e-12*math.Max(1, math.Abs(maximum))
}

type ObjectiveGoal struct {
	SourceWeights  bool
	PropertyID     string
	Minimum        float64
	Maximum        float64
	Tolerance      float64
	Importance     float64
	StrictMinimum  bool
	StrictFloor    float64
	ExplicitWeight bool
	ScoreScale     float64
}

type weightedSet struct {
	definition     SetDefinition
	priority       float64
	cartridge      nte.Cartridge
	cartridgeScore float64
}

func NewCartridgeSetEvaluator(catalog SetCatalog, cartridges []nte.Cartridge, preferences []scoring.SetPreference, modules []Candidate, character scoring.Character, refs scoring.References) CartridgeSetEvaluator {
	available := map[string]nte.Cartridge{}
	cartridgeScores := map[string]float64{}
	for _, cartridge := range cartridges {
		score := scoring.ScoreCartridgeDetailed(cartridge, character, refs).Total
		if _, ok := available[cartridge.SetID]; !ok || score > cartridgeScores[cartridge.SetID] {
			available[cartridge.SetID], cartridgeScores[cartridge.SetID] = cartridge, score
		}
	}
	evaluator := CartridgeSetEvaluator{modules: make(map[string]nte.Module, len(modules))}
	for _, module := range modules {
		evaluator.modules[module.Module.LocalID] = module.Module
	}
	preferenceBySet := make(map[string]float64, len(preferences))
	for _, preference := range preferences {
		preferenceBySet[preference.SetID] = preference.Priority
	}
	// Recommendations are preferences, not constraints: every owned cartridge
	// participates in the search. A configured priority may still bias raw-score
	// mode, while the default recommendation priority of 1 is neutral.
	for setID, definition := range catalog.Definitions {
		cartridge, owned := available[definition.InventorySetID]
		if !owned {
			continue
		}
		priority := preferenceBySet[setID]
		if priority <= 0 {
			priority = 1
		}
		weighted := weightedSet{definition: definition, priority: priority, cartridge: cartridge, cartridgeScore: cartridgeScores[definition.InventorySetID]}
		evaluator.sets = append(evaluator.sets, weighted)
		maxBonus := scoreForMatched(definition.Bonuses, len(definition.RequiredGeometries))*priority + weighted.cartridgeScore
		if maxBonus > evaluator.upperBound {
			evaluator.upperBound = maxBonus
		}
	}
	return evaluator
}

func NewObjectiveEvaluator(catalog SetCatalog, cartridges []nte.Cartridge, preferences []scoring.SetPreference, modules []Candidate, character scoring.Character, refs scoring.References, goals []ObjectiveGoal, additional map[string][]nte.Stat) CartridgeSetEvaluator {
	evaluator := NewCartridgeSetEvaluator(catalog, cartridges, preferences, modules, character, refs)
	evaluator.objectives = goals
	evaluator.character = character
	evaluator.additional = additional
	if len(goals) > 0 {
		objectiveBound := 0.0
		for _, goal := range goals {
			if goal.Minimum > 0 {
				objectiveBound += objectiveMaximum(goal)
			}
		}
		evaluator.upperBound = 0
		for _, set := range evaluator.sets {
			bonus := scoreForMatched(set.definition.Bonuses, len(set.definition.RequiredGeometries)) * set.priority
			evaluator.upperBound = math.Max(evaluator.upperBound, (bonus+set.cartridgeScore)*EquipmentTieBreakScale+objectiveBound)
		}
		evaluator.upperBound = math.Nextafter(evaluator.upperBound, math.Inf(1))
	}
	return evaluator
}

func (e CartridgeSetEvaluator) Evaluate(placements []Placement) BonusResult {
	best := BonusResult{}
	if e.exact {
		best.Score = math.Inf(-1)
	}
	for _, set := range e.sets {
		required := make(map[string]bool, len(set.definition.RequiredGeometries))
		for _, geometry := range set.definition.RequiredGeometries {
			required[geometry] = true
		}
		matched := map[string]bool{}
		for _, placement := range placements {
			module, ok := e.modules[placement.ModuleID]
			if ok && required[module.Geometry] {
				matched[module.Geometry] = true
			}
		}
		setScore := scoreForMatched(set.definition.Bonuses, len(matched)) * set.priority
		if e.exact && len(matched) != len(set.definition.RequiredGeometries) {
			continue
		}
		score := setScore + set.cartridgeScore
		if len(e.objectives) > 0 {
			score *= EquipmentTieBreakScale
			selected := make([]nte.Module, 0, len(placements))
			for _, placement := range placements {
				if module, ok := e.modules[placement.ModuleID]; ok {
					selected = append(selected, module)
				}
			}
			summary := BuildStatSummary(e.character, selected, &set.cartridge, set.definition, len(matched), e.additional)
			if e.exact && (exceedsMaximum(summary.Derived, e.objectives) || belowAnyToleranceFloor(summary.Derived, e.objectives)) {
				continue
			}
			utility := ObjectiveScore(summary.Derived, e.objectives)
			if e.sourceWeighted() {
				utility, _ = SourceObjectiveScore(summary.Unconditional, selected, &set.cartridge, e.character, e.objectives)
			}
			score += utility
		}
		if score > best.Score {
			best = BonusResult{Score: score, SetBonusScore: setScore, CartridgeScore: set.cartridgeScore, SetID: set.definition.ID, CartridgeID: set.cartridge.LocalID, MatchedCount: len(matched)}
		}
	}
	return best
}

func belowAnyToleranceFloor(values map[string]float64, goals []ObjectiveGoal) bool {
	for _, goal := range goals {
		if belowToleranceFloor(values[goal.PropertyID], goal) {
			return true
		}
	}
	return false
}

func ObjectiveScore(values map[string]float64, goals []ObjectiveGoal) float64 {
	score := 0.0
	for _, goal := range goals {
		score += objectiveUtility(values[goal.PropertyID], goal)
	}
	return score
}

func objectiveUtility(value float64, goal ObjectiveGoal) float64 {
	if goal.Minimum <= 0 {
		return 0
	}
	importance := objectiveImportance(goal)
	if importance == 0 {
		return 0
	}
	if goal.ScoreScale > 0 {
		if exceedsLimit(value, goal.Maximum) {
			excess := (value - goal.Maximum) / goal.Maximum
			return -(10000 + excess*10000) * importance
		}
		capped := math.Min(math.Max(value, 0), goal.Minimum)
		return 10 * importance * capped / (goal.ScoreScale + capped)
	}
	ratio := value / goal.Minimum
	if math.Abs(ratio-1) <= 1e-12 {
		ratio = 1
	}
	if exceedsLimit(value, goal.Maximum) {
		excess := (value - goal.Maximum) / goal.Maximum
		return -(10000 + excess*10000) * importance
	}
	if ratio < 0 {
		ratio = 0
	}
	utility := 0.0
	switch {
	case ratio < 1:
		// Targets and weights define ranking. Tolerance and strictness only
		// define feasibility, so toggling a constraint cannot change the score
		// or candidate preselection of the same build. The fourth-power curve
		// keeps a meaningful incentive to approach the requested target.
		utility = math.Pow(ratio, 4)
	default:
		// Values above the target remain useful, with diminishing returns.
		utility = 1 + minFloat(ratio-1, .25)*.2
	}
	return utility * importance * 10
}

func objectiveImportance(goal ObjectiveGoal) float64 {
	if goal.Importance <= 0 && !goal.ExplicitWeight {
		return 1
	}
	return math.Max(goal.Importance, 0)
}

func objectiveMaximum(goal ObjectiveGoal) float64 {
	importance := objectiveImportance(goal)
	if goal.ScoreScale > 0 {
		return 10 * importance * goal.Minimum / (goal.ScoreScale + goal.Minimum)
	}
	return 10.5 * importance
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (e CartridgeSetEvaluator) UpperBound() float64 { return e.upperBound }

func scoreForMatched(bonuses []SetBonus, matched int) float64 {
	result := 0.0
	for _, bonus := range bonuses {
		if matched >= bonus.Count && bonus.Score > result {
			result = bonus.Score
		}
	}
	return result
}
