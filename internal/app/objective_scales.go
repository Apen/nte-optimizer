package app

import (
	"fmt"
	"math"

	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

// ObjectiveScaleFactor is shared by every statistic and profile in the
// experimental search mode. References are independent of target and inventory.
const ObjectiveScaleFactor = 10.0

func applyObjectiveScales(goals []optimizer.ObjectiveGoal, refs scoring.References, baseStats map[string]float64) error {
	for i := range goals {
		reference := refs[goals[i].PropertyID]
		switch goals[i].PropertyID {
		case "AtkFinal":
			reference = math.Max(refs["AtkAdd"], baseStats["AtkBase"]*refs["AtkUp"])
		case "HPFinal":
			reference = math.Max(refs["HPMaxAdd"], baseStats["HPMaxBase"]*refs["HPMaxUp"])
		case "DefFinal":
			reference = math.Max(refs["DefAdd"], baseStats["DefBase"]*refs["DefUp"])
		}
		if reference <= 0 || math.IsNaN(reference) || math.IsInf(reference, 0) {
			return fmt.Errorf("no positive scoring reference for objective %s", goals[i].PropertyID)
		}
		goals[i].ScoreScale = ObjectiveScaleFactor * reference
	}
	return nil
}
