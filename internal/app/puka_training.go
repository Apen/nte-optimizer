package app

import (
	"fmt"

	"nte-optimizer/internal/nte"
)

func loadPukaTrainingRanks(path string) (map[uint32][]nte.Stat, error) {
	var catalog struct {
		SchemaVersion int                   `json:"schema_version"`
		Ranks         map[uint32][]nte.Stat `json:"ranks"`
	}
	if err := readJSON(path, &catalog); err != nil {
		return nil, err
	}
	if catalog.SchemaVersion != 1 {
		return nil, fmt.Errorf("account/puka_training.json: unsupported schema version %d", catalog.SchemaVersion)
	}
	if stats, ok := catalog.Ranks[0]; !ok || len(stats) != 0 || len(catalog.Ranks) < 2 {
		return nil, fmt.Errorf("account/puka_training.json: expected an empty rank 0 and at least one bonus rank")
	}
	for rank := uint32(1); int(rank) < len(catalog.Ranks); rank++ {
		stats := catalog.Ranks[rank]
		if len(stats) == 0 {
			return nil, fmt.Errorf("account/puka_training.json: missing stats for rank %d", rank)
		}
		seen := map[string]bool{}
		for _, stat := range stats {
			if stat.PropertyID == "" || !finiteNonNegative(stat.Value) || seen[stat.PropertyID] {
				return nil, fmt.Errorf("account/puka_training.json: invalid or duplicate property %q at rank %d", stat.PropertyID, rank)
			}
			seen[stat.PropertyID] = true
		}
	}
	return catalog.Ranks, nil
}
