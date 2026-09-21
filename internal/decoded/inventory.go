package decoded

import (
	"fmt"

	"nte-optimizer/internal/nte"
)

var supportedGeometries = map[string]bool{
	"H_2": true, "H_3": true, "H_4": true,
	"V_2": true, "V_3": true, "V_4": true,
	"Trap_4_H": true, "Trap_4_V": true,
	"L_3_TR": true, "L_3_BR": true, "L_3_BL": true, "L_3_TL": true,
}

func validateInventory(inventory nte.Inventory) []error {
	var errors []error
	ids := map[string]bool{}
	for index, module := range inventory.Modules {
		if module.LocalID == "" || ids[module.LocalID] {
			errors = append(errors, fmt.Errorf("module %d has missing or duplicate local_id", index))
		}
		ids[module.LocalID] = true
		if !supportedGeometries[module.Geometry] {
			errors = append(errors, fmt.Errorf("%s has unknown geometry %q", module.LocalID, module.Geometry))
		}
		if len(module.MainStats) == 0 {
			errors = append(errors, fmt.Errorf("%s has no main stat", module.LocalID))
		}
	}
	return errors
}
