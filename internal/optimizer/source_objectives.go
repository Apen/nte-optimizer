package optimizer

import (
	"math"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func (e CartridgeSetEvaluator) sourceWeighted() bool {
	for _, goal := range e.objectives {
		if goal.SourceWeights {
			return true
		}
	}
	return false
}

// SourceObjectiveScore shares target saturation across all sources, but only
// rewards equipment contributions using their own main/sub weight. Other
// sources help reach the target without inheriting an equipment weight.
func SourceObjectiveScore(total map[string]float64, modules []nte.Module, cartridge *nte.Cartridge, character scoring.Character, goals []ObjectiveGoal) (float64, []RankingContribution) {
	main, sub := map[string]float64{}, map[string]float64{}
	for _, module := range modules {
		addStats(main, module.MainStats)
		addStats(sub, module.SubStats)
	}
	if cartridge != nil {
		addStats(main, cartridge.MainStats)
		addStats(sub, cartridge.SubStats)
	}
	return sourceObjectiveValues(total, main, sub, character, goals)
}

func sourceObjectiveValues(total, main, sub map[string]float64, character scoring.Character, goals []ObjectiveGoal) (float64, []RankingContribution) {
	score := 0.0
	contributions := []RankingContribution{}
	for _, goal := range goals {
		if goal.Minimum <= 0 {
			continue
		}
		keys := objectiveInputs(goal.PropertyID)
		value := total[goal.PropertyID]
		if len(keys) == 3 {
			value = panelTotal(total[keys[0]], total[keys[1]], total[keys[2]])
		}
		if !goal.SourceWeights {
			points := objectiveUtility(value, goal)
			score += points
			contributions = append(contributions, RankingContribution{PropertyID: goal.PropertyID, Value: value, Target: goal.Minimum, Importance: goal.Importance, Points: points, ScoreScale: goal.ScoreScale})
			continue
		}
		neutral := goal
		neutral.Importance = 1
		utility := objectiveUtility(value, neutral)
		// Split multiplicative base/percentage interactions equally between inputs.
		// The unrounded total avoids assigning rounding residuals to either source.
		denominator := value
		if len(keys) == 3 {
			denominator = total[keys[0]]*(1+total[keys[1]]) + total[keys[2]]
		}
		for group, source := range []map[string]float64{main, sub} {
			weights := character.MainWeights
			name := "main"
			if group == 1 {
				weights = character.SubWeights
				name = "sub"
			}
			for _, key := range keys {
				amount := source[key]
				if len(keys) == 3 {
					switch key {
					case keys[0]:
						amount *= 1 + total[keys[1]] - .5*(main[keys[1]]+sub[keys[1]])
					case keys[1]:
						amount *= total[keys[0]] - .5*(main[keys[0]]+sub[keys[0]])
					}
				}
				weight := weights[key]
				points := 0.0
				if denominator > 0 {
					points = utility * math.Max(0, amount) / denominator * weight
				}
				if source[key] == 0 {
					continue
				}
				score += points
				contributions = append(contributions, RankingContribution{PropertyID: goal.PropertyID, Source: name, InputPropertyID: key, Value: source[key], Target: goal.Minimum, Importance: weight, Points: points, ScoreScale: goal.ScoreScale})
			}
		}
	}
	return score, contributions
}
