package damage

import "sort"

type Event struct {
	At       float64  `json:"at"`
	Instance Instance `json:"instance"`
	Stacks   int      `json:"stacks,omitempty"`
}

type Simulation struct {
	DurationSeconds float64              `json:"duration_seconds"`
	TotalExpected   float64              `json:"total_expected"`
	DPS             float64              `json:"dps"`
	ByCategory      map[Category]float64 `json:"by_category"`
	BySkill         map[string]float64   `json:"by_skill"`
	Results         []Result             `json:"results"`
	Warnings        []string             `json:"warnings,omitempty"`
}

// Simulate evaluates an explicit timeline. State application and consumption
// will be layered on this deterministic event core as character adapters are
// validated.
func Simulate(characterLevel int, stats Stats, enemy Enemy, duration float64, events []Event) Simulation {
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At < ordered[j].At })
	result := Simulation{
		DurationSeconds: duration,
		ByCategory:      map[Category]float64{},
		BySkill:         map[string]float64{},
	}
	for _, event := range ordered {
		if event.At < 0 || (duration > 0 && event.At > duration) {
			continue
		}
		var damage Result
		if event.Instance.Category == CategoryDOT {
			damage = CalculateDOT(characterLevel, stats, enemy, event.Instance, event.Stacks)
		} else {
			damage = Calculate(characterLevel, stats, enemy, event.Instance)
		}
		result.Results = append(result.Results, damage)
		result.TotalExpected += damage.Total
		result.ByCategory[damage.Category] += damage.Total
		result.BySkill[event.Instance.SkillID] += damage.Total
		result.Warnings = append(result.Warnings, damage.Warnings...)
	}
	if duration > 0 {
		result.DPS = result.TotalExpected / duration
	}
	return result
}
