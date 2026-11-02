package decoded

import (
	"fmt"

	"nte-optimizer/internal/nte"
)

func validateInventory(inventory nte.Inventory, definitions nte.EquipmentDefinitions) []error {
	var errors []error
	ids := map[string]bool{}
	for index, module := range inventory.Modules {
		if module.LocalID == "" || ids[module.LocalID] {
			errors = append(errors, fmt.Errorf("module %d has missing or duplicate local_id", index))
		}
		ids[module.LocalID] = true
		if definition, ok := definitions[module.GameItemID]; !ok || definition.Geometry != module.Geometry {
			errors = append(errors, fmt.Errorf("%s has unknown geometry %q", module.LocalID, module.Geometry))
		}
		if len(module.MainStats) == 0 {
			errors = append(errors, fmt.Errorf("%s has no main stat", module.LocalID))
		}
	}
	return errors
}
