package optimizer

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/scoring"
)

func readSanitizedOptimizerInventory(t testing.TB) nte.Inventory {
	t.Helper()
	data, err := os.ReadFile("../testdata/optimizer/sanitized_inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory nte.Inventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatalf("decode sanitized optimizer inventory: %v", err)
	}
	return inventory
}

func sanitizedCandidates(inventory nte.Inventory) []Candidate {
	profile := scoring.Character{Weights: map[string]float64{
		"CritBase": 1, "CritDamageBase": .8, "AtkUp": .6, "UnbalIntensityBase": .4,
	}}
	references := scoring.References{
		"CritBase": .064, "CritDamageBase": .128, "AtkUp": .12, "UnbalIntensityBase": 12,
	}
	candidates := make([]Candidate, 0, len(inventory.Modules))
	for _, module := range inventory.Modules {
		candidates = append(candidates, Candidate{
			Module: module,
			Score:  scoring.Score(module, profile, references),
		})
	}
	return candidates
}

func TestSelectCandidatesKeepsGeometryAndSetLeadersFromSanitizedInventory(t *testing.T) {
	inventory := readSanitizedOptimizerInventory(t)
	candidates := sanitizedCandidates(inventory)
	selected := SelectCandidates(candidates, 1, 2)
	selectedIDs := make(map[string]bool, len(selected))
	for _, candidate := range selected {
		selectedIDs[candidate.Module.LocalID] = true
	}

	byGeometry := map[string][]Candidate{}
	bySet := map[string][]Candidate{}
	for _, candidate := range candidates {
		byGeometry[candidate.Module.Geometry] = append(byGeometry[candidate.Module.Geometry], candidate)
		bySet[candidate.Module.SetID] = append(bySet[candidate.Module.SetID], candidate)
	}
	for geometry, group := range byGeometry {
		sort.Slice(group, func(i, j int) bool { return betterScoredCandidate(group[i], group[j]) })
		if !selectedIDs[group[0].Module.LocalID] {
			t.Errorf("geometry leader %q for %s was dropped", group[0].Module.LocalID, geometry)
		}
	}
	for setID, group := range bySet {
		sort.Slice(group, func(i, j int) bool { return betterScoredCandidate(group[i], group[j]) })
		for _, candidate := range group[:min(2, len(group))] {
			if !selectedIDs[candidate.Module.LocalID] {
				t.Errorf("set leader %q for %s was dropped", candidate.Module.LocalID, setID)
			}
		}
	}
	if len(selected) >= len(candidates) {
		t.Fatalf("fixture did not exercise candidate reduction: selected=%d eligible=%d", len(selected), len(candidates))
	}

	if got := SelectCandidates(candidates, 0, 0); len(got) != 0 {
		t.Fatalf("zero geometry and set limits selected %d candidates", len(got))
	}
}

func TestSelectObjectiveCandidatesKeepsGeneralistsAndStatSpecialists(t *testing.T) {
	inventory := readSanitizedOptimizerInventory(t)
	candidates := sanitizedCandidates(inventory)
	for index := range candidates {
		candidates[index].Priority = candidates[index].Score
		candidates[index].ObjectiveValues = map[string]float64{}
		for _, stat := range candidates[index].Module.SubStats {
			candidates[index].ObjectiveValues[stat.PropertyID] = stat.Value
		}
	}
	goals := []ObjectiveGoal{{PropertyID: "CritBase"}, {PropertyID: "UnbalIntensityBase"}}
	selected := SelectObjectiveCandidates(candidates, goals, 0, 0)
	selectedIDs := make(map[string]bool, len(selected))
	for _, candidate := range selected {
		selectedIDs[candidate.Module.LocalID] = true
	}
	byGeometry := map[string][]Candidate{}
	for _, candidate := range candidates {
		byGeometry[candidate.Module.Geometry] = append(byGeometry[candidate.Module.Geometry], candidate)
	}
	for geometry, group := range byGeometry {
		generalist := group[0]
		for _, candidate := range group[1:] {
			if betterObjectiveCandidate(candidate, generalist) {
				generalist = candidate
			}
		}
		if !selectedIDs[generalist.Module.LocalID] {
			t.Errorf("generalist %q for %s was dropped", generalist.Module.LocalID, geometry)
		}
		for _, goal := range goals {
			specialist := group[0]
			for _, candidate := range group[1:] {
				left := candidate.ObjectiveValues[goal.PropertyID]
				right := specialist.ObjectiveValues[goal.PropertyID]
				if left > right || (left == right && candidate.Priority > specialist.Priority) {
					specialist = candidate
				}
			}
			if !selectedIDs[specialist.Module.LocalID] {
				t.Errorf("%s specialist %q for %s was dropped", goal.PropertyID, specialist.Module.LocalID, geometry)
			}
		}
	}
	if len(selected) >= len(candidates) {
		t.Fatalf("fixture did not exercise objective candidate reduction: selected=%d eligible=%d", len(selected), len(candidates))
	}
	wantOrder := make([]string, 0, len(selectedIDs))
	for _, candidate := range candidates {
		if selectedIDs[candidate.Module.LocalID] {
			wantOrder = append(wantOrder, candidate.Module.LocalID)
		}
	}
	for index, candidate := range selected {
		if candidate.Module.LocalID != wantOrder[index] {
			t.Fatalf("objective selection changed input order: got %q at %d, want %q", candidate.Module.LocalID, index, wantOrder[index])
		}
	}
}

func betterScoredCandidate(left, right Candidate) bool {
	if left.Score == right.Score {
		return left.Module.LocalID < right.Module.LocalID
	}
	return left.Score > right.Score
}

func betterObjectiveCandidate(left, right Candidate) bool {
	if left.Priority == right.Priority {
		if left.Score == right.Score {
			return left.Module.LocalID < right.Module.LocalID
		}
		return left.Score > right.Score
	}
	return left.Priority > right.Priority
}
