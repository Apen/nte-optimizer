package combatdata

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

type Actor struct {
	CharacterID  int                `json:"character_id"`
	Level        int                `json:"level"`
	Awakening    int                `json:"awakening"`
	Breakthrough int                `json:"breakthrough"`
	Levels       map[string]int     `json:"levels"`
	Stats        map[string]float64 `json:"stats"`
}
type Context struct {
	Inputs     map[string]float64
	Parameters map[string]float64
	Actors     map[string]Actor
	// Bindings use the complete serialized query node, including actor, named
	// output and arguments. Equal method names need not imply equal queries.
	Bindings map[string]any
}

func Exact(values []float64, level int) (float64, error) {
	if level < 1 || level > len(values) {
		return 0, fmt.Errorf("unresolved level %d for %d values", level, len(values))
	}
	v := values[level-1]
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("non-finite catalog value")
	}
	return v, nil
}

func (p Parameter) Resolve(skillLevel, characterLevel int, inputs map[string]float64) (float64, error) {
	if p.Status == "unused_source_slot" {
		return 0, fmt.Errorf("unused source slot")
	}
	if p.SourceKind == "scenario_input_curve" {
		if p.Input == nil || p.Curve == nil {
			return 0, fmt.Errorf("missing scenario curve contract")
		}
		x, ok := inputs[p.Input.ID]
		if !ok {
			return 0, fmt.Errorf("missing input %s", p.Input.ID)
		}
		if err := p.Input.Validate(x); err != nil {
			return 0, err
		}
		if p.Curve.Interpolation != "ERichCurveInterpMode::RCIM_Constant" || len(p.Curve.Keys) == 0 {
			return 0, fmt.Errorf("unsupported scenario interpolation")
		}
		value := p.Curve.Keys[0].Value
		for _, key := range p.Curve.Keys {
			if key.Time <= x {
				value = key.Value
			}
		}
		if p.Multiplier != nil {
			value *= *p.Multiplier
		}
		return value, nil
	}
	if len(p.CharacterValues) > 0 {
		return Exact(p.CharacterValues, characterLevel)
	}
	// A constant display curve does not require inventing an imported level.
	if skillLevel == 0 && len(p.Values) > 0 {
		constant := true
		for _, value := range p.Values {
			if value != p.Values[0] {
				constant = false
			}
		}
		if constant {
			return Exact(p.Values, 1)
		}
	}
	return Exact(p.Values, skillLevel)
}

func (i Input) Validate(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || (i.Minimum != nil && value < *i.Minimum) || (i.Maximum != nil && value > *i.Maximum) || (i.Kind == "integer" && math.Trunc(value) != value) {
		return fmt.Errorf("input %s outside declared bounds", i.ID)
	}
	return nil
}

func Evaluate(raw json.RawMessage, c Context) (float64, error) {
	v, err := evaluate(raw, c, 0)
	if err != nil {
		return 0, err
	}
	n, ok := v.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("expression did not return a finite number")
	}
	return n, nil
}

func evaluate(raw json.RawMessage, c Context, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("expression depth exceeded")
	}
	var literal any
	if err := json.Unmarshal(raw, &literal); err != nil {
		return nil, err
	}
	if _, object := literal.(map[string]any); !object {
		return literal, nil
	}
	var node struct {
		Op        string            `json:"op"`
		ID        string            `json:"id"`
		Actor     string            `json:"actor"`
		AbilityID string            `json:"ability_id"`
		Values    []float64         `json:"values"`
		Args      []json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, err
	}
	if node.Op == "input" && node.Actor == "" {
		v, ok := c.Inputs[node.ID]
		if !ok {
			return nil, fmt.Errorf("missing input %s", node.ID)
		}
		return v, nil
	}
	if node.Op == "parameter" {
		v, ok := c.Parameters[node.ID]
		if !ok {
			return nil, fmt.Errorf("missing parameter %s", node.ID)
		}
		return v, nil
	}
	if node.Op == "skill_level_value" {
		actor, ok := c.Actors[node.Actor]
		if !ok {
			return nil, fmt.Errorf("missing actor %s", node.Actor)
		}
		return Exact(node.Values, actor.Levels[node.AbilityID])
	}
	switch node.Op {
	case "input", "buff_active", "buff_stacks", "actor_is_type", "damage_effect_has_all_tags", "damage_effect_class_id", "object_reference", "team_members", "scalable_float_reference":
		key, _ := json.Marshal(literal)
		if v, ok := c.Bindings[string(key)]; ok {
			return v, nil
		}
		return nil, fmt.Errorf("missing selected context: %s", key)
	case "class_reference":
		return node.ID, nil
	case "null_reference":
		return nil, nil
	}
	arg := func(i int) (any, error) {
		if i >= len(node.Args) {
			return nil, fmt.Errorf("missing %s argument %d", node.Op, i)
		}
		return evaluate(node.Args[i], c, depth+1)
	}
	if node.Op == "select" {
		if len(node.Args) != 3 {
			return nil, fmt.Errorf("select requires three arguments")
		}
		v, err := arg(0)
		if err != nil {
			return nil, err
		}
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("select condition is not boolean")
		}
		if b {
			return arg(1)
		}
		return arg(2)
	}
	if node.Op == "and" || node.Op == "or" || node.Op == "not" {
		if len(node.Args) == 0 || (node.Op == "not" && len(node.Args) != 1) {
			return nil, fmt.Errorf("invalid %s arguments", node.Op)
		}
		for i := range node.Args {
			v, err := arg(i)
			if err != nil {
				return nil, err
			}
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("boolean argument required")
			}
			if node.Op == "not" {
				return !b, nil
			}
			if node.Op == "and" && !b {
				return false, nil
			}
			if node.Op == "or" && b {
				return true, nil
			}
		}
		return node.Op == "and", nil
	}
	values := make([]any, len(node.Args))
	for i := range values {
		v, err := arg(i)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	if node.Op == "equal" {
		if len(values) != 2 {
			return nil, fmt.Errorf("equal requires two arguments")
		}
		return reflect.DeepEqual(values[0], values[1]), nil
	}
	if node.Op == "is_not_null" {
		if len(values) != 1 {
			return nil, fmt.Errorf("is_not_null requires one argument")
		}
		return values[0] != nil, nil
	}
	if node.Op == "count" {
		if len(values) != 1 {
			return nil, fmt.Errorf("count requires one argument")
		}
		a, ok := values[0].([]any)
		if !ok {
			return nil, fmt.Errorf("count requires array")
		}
		return float64(len(a)), nil
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("unsupported or empty operation %s", node.Op)
	}
	numbers := make([]float64, len(values))
	for i, v := range values {
		n, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("%s requires numeric arguments", node.Op)
		}
		numbers[i] = n
	}
	result := numbers[0]
	switch node.Op {
	case "add", "subtract", "multiply", "divide", "minimum", "maximum":
		if (node.Op == "subtract" || node.Op == "divide") && len(numbers) != 2 {
			return nil, fmt.Errorf("%s requires two arguments", node.Op)
		}
		for _, n := range numbers[1:] {
			switch node.Op {
			case "add":
				result += n
			case "subtract":
				result -= n
			case "multiply":
				result *= n
			case "divide":
				if n == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				result /= n
			case "minimum":
				result = math.Min(result, n)
			case "maximum":
				result = math.Max(result, n)
			}
		}
	case "clamp":
		if len(numbers) != 3 || numbers[1] > numbers[2] {
			return nil, fmt.Errorf("invalid clamp bounds")
		}
		result = math.Max(numbers[1], math.Min(numbers[2], result))
	case "floor", "truncate":
		if len(numbers) != 1 {
			return nil, fmt.Errorf("%s requires one argument", node.Op)
		}
		if node.Op == "floor" {
			result = math.Floor(result)
		} else {
			result = math.Trunc(result)
		}
	case "greater", "greater_equal", "less", "less_equal":
		if len(numbers) != 2 {
			return nil, fmt.Errorf("comparison requires two arguments")
		}
		switch node.Op {
		case "greater":
			return result > numbers[1], nil
		case "greater_equal":
			return result >= numbers[1], nil
		case "less":
			return result < numbers[1], nil
		default:
			return result <= numbers[1], nil
		}
	default:
		return nil, fmt.Errorf("unsupported operation %s", node.Op)
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil, fmt.Errorf("non-finite expression result")
	}
	return result, nil
}

// Scenario evaluates one selected result stage. It never appends damage to an
// action total or activates a related modifier from another catalog.
func (s Scenario) Evaluate(actor Actor, team map[int]Actor, inputs map[string]float64) (float64, error) {
	parameters := map[string]float64{}
	for id, p := range s.Parameters {
		var value float64
		if err := json.Unmarshal(p.Value, &value); err == nil {
			parameters[id] = value
			continue
		}
		if p.LevelBinding != nil {
			v, err := Exact(p.Values, actor.Levels[p.LevelBinding.AbilityID])
			if err != nil {
				return 0, err
			}
			parameters[id] = v
		}
	}
	selected := actor
	if s.SourceActorSelection != nil {
		if s.SourceActorSelection.Kind != "first_available_character" {
			return 0, fmt.Errorf("unsupported source actor selector")
		}
		found := false
		for _, id := range s.SourceActorSelection.CharacterParameters {
			var characterID string
			if err := json.Unmarshal(s.Parameters[id].Value, &characterID); err != nil {
				return 0, fmt.Errorf("invalid actor identity parameter %s", id)
			}
			n, err := strconv.Atoi(characterID)
			if err != nil {
				return 0, err
			}
			if a, ok := team[n]; ok {
				selected = a
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("no selected source actor available")
		}
	}
	values := map[string]float64{}
	for id, input := range s.Inputs {
		var v float64
		var ok bool
		switch input.Selection {
		case "builder_final_attack":
			v, ok = actor.Stats["AtkFinal"]
		case "builder_stat":
			v, ok = actor.Stats[input.StatID]
		case "source_actor_selection":
			v, ok = selected.Stats[input.StatID]
		case "manual":
			v, ok = inputs[id]
		default:
			return 0, fmt.Errorf("unsupported scenario input selection %s", input.Selection)
		}
		if !ok {
			return 0, fmt.Errorf("missing input %s", id)
		}
		if err := input.Validate(v); err != nil {
			return 0, err
		}
		values[id] = v
	}
	return Evaluate(s.Calculation, Context{Inputs: values, Parameters: parameters})
}

// EvaluateModifier keeps effect-spec levels independent of imported skill
// levels and applies the wrapper even when its coefficient is exactly zero.
func (m Modifier) Evaluate(c Context, effectLevel int) (float64, error) {
	if len(m.PublishedIDs) > 0 {
		return 0, fmt.Errorf("modifier already represented by published buff")
	}
	var p struct {
		Cases []struct {
			When  json.RawMessage `json:"when"`
			Value json.RawMessage `json:"value"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(m.ExpressionProjection, &p); err != nil {
		return 0, fmt.Errorf("missing or invalid expression projection")
	}
	for _, branch := range p.Cases {
		when, err := evaluate(branch.When, c, 0)
		if err != nil {
			return 0, err
		}
		active, ok := when.(bool)
		if !ok {
			return 0, fmt.Errorf("modifier condition is not boolean")
		}
		if !active {
			continue
		}
		base, err := Evaluate(branch.Value, c)
		if err != nil {
			return 0, err
		}
		var transform struct {
			Coefficient []float64 `json:"coefficient_by_effect_level"`
			Pre         []float64 `json:"pre_multiply_additive_by_effect_level"`
			Post        []float64 `json:"post_multiply_additive_by_effect_level"`
			Lookup      struct {
				Table json.RawMessage `json:"CurveTable"`
				Row   string          `json:"RowName"`
			} `json:"final_lookup"`
		}
		if err := json.Unmarshal(m.Transform, &transform); err != nil {
			return 0, fmt.Errorf("unresolved magnitude wrapper")
		}
		if (len(transform.Lookup.Table) > 0 && string(transform.Lookup.Table) != "null") || (transform.Lookup.Row != "None" && transform.Lookup.Row != "") {
			return 0, fmt.Errorf("unsupported final lookup curve")
		}
		coefficient, err := Exact(transform.Coefficient, effectLevel)
		if err != nil {
			return 0, err
		}
		pre, err := Exact(transform.Pre, effectLevel)
		if err != nil {
			return 0, err
		}
		post, err := Exact(transform.Post, effectLevel)
		if err != nil {
			return 0, err
		}
		result := coefficient*(base+pre) + post
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return 0, fmt.Errorf("non-finite magnitude result")
		}
		return result, nil
	}
	return 0, fmt.Errorf("no selected modifier branch")
}
