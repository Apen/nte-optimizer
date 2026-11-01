package app

import (
	"fmt"
	"math"
	"strings"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/nte"
)

// simulateCartridgeMainStats projects only deterministic main stats. Unknown
// future substat rolls are never added, and the imported inventory stays intact.
func (s OptimizerService) simulateCartridgeMainStats(cartridges []nte.Cartridge) ([]nte.Cartridge, error) {
	var catalog struct {
		Items map[string]struct {
			Kind     string `json:"kind"`
			Quality  string `json:"quality"`
			MaxLevel int    `json:"max_level"`
		} `json:"items"`
		Curves map[string][][2]float64 `json:"curves"`
	}
	if err := readJSON(datafiles.New(s.DataDir).DecodeCatalog("equipment.json"), &catalog); err != nil {
		return nil, err
	}
	projected := append([]nte.Cartridge(nil), cartridges...)
	for index, cartridge := range projected {
		definition, ok := catalog.Items[cartridge.GameItemID]
		if !ok || definition.Kind != "core" || definition.MaxLevel <= 0 {
			return nil, fmt.Errorf("equipment/decode.json: missing core max_level for item %q", cartridge.GameItemID)
		}
		if cartridge.Level >= definition.MaxLevel {
			continue
		}
		if len(cartridge.MainStats) == 0 {
			return nil, fmt.Errorf("cannot simulate cartridge without a main stat")
		}
		cartridge.MainStats = append([]nte.Stat(nil), cartridge.MainStats...)
		for statIndex, stat := range cartridge.MainStats {
			key := stat.PropertyID + "_Core_ITEM_QUALITY_" + strings.ToUpper(definition.Quality)
			curve := catalog.Curves[key]
			found := false
			for _, point := range curve {
				if point[0] == float64(definition.MaxLevel) && !math.IsNaN(point[1]) && !math.IsInf(point[1], 0) && point[1] > 0 {
					cartridge.MainStats[statIndex].Value = point[1]
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("equipment/decode.json: missing level %d value in curves[%q]", definition.MaxLevel, key)
			}
		}
		cartridge.SimulatedMainStatLevel = definition.MaxLevel
		projected[index] = cartridge
	}
	return projected, nil
}
