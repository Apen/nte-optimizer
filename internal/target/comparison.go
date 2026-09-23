package target

import (
	"encoding/json"
	"fmt"
	"os"
)

type GoalProgress struct {
	Goal
	Current  float64 `json:"current"`
	Missing  float64 `json:"missing"`
	Progress float64 `json:"progress"`
	Reached  bool    `json:"reached"`
}

func Read(path string) (BuildTarget, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return BuildTarget{}, err
	}
	var target BuildTarget
	if err := json.Unmarshal(b, &target); err != nil {
		return BuildTarget{}, fmt.Errorf("decode build target: %w", err)
	}
	if target.SchemaVersion != 2 || target.ID == "" || target.Character.CharacterID <= 0 || len(target.Goals) == 0 || len(target.Builds) == 0 {
		return BuildTarget{}, fmt.Errorf("invalid build target")
	}
	seenCharacters := map[int]bool{target.Character.CharacterID: true}
	for _, id := range target.Character.AlternateIDs {
		if id <= 0 || seenCharacters[id] {
			return BuildTarget{}, fmt.Errorf("invalid alternate character ID %d", id)
		}
		seenCharacters[id] = true
	}
	for _, build := range target.Builds {
		if build.ID == "" || len(build.Variants) == 0 {
			return BuildTarget{}, fmt.Errorf("invalid build target strategy %q", build.ID)
		}
	}
	return target, nil
}

func Compare(target BuildTarget, stats map[string]float64) []GoalProgress {
	result := make([]GoalProgress, 0, len(target.Goals))
	for _, goal := range target.Goals {
		current := stats[goal.PropertyID]
		missing := goal.Minimum - current
		if missing < 0 {
			missing = 0
		}
		progress := 0.0
		if goal.Minimum > 0 {
			progress = current / goal.Minimum
		}
		if progress > 1 {
			progress = 1
		}
		result = append(result, GoalProgress{Goal: goal, Current: current, Missing: missing, Progress: progress, Reached: current >= goal.Minimum})
	}
	return result
}
