package app

import (
	"fmt"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

const accountSnapshotSchemaVersion = 1

type accountSnapshot struct {
	SchemaVersion int           `json:"schema_version"`
	Inventory     nte.Inventory `json:"inventory"`
	State         decoded.State `json:"state"`
}

// loadAccountData reads the atomic account snapshot. The two legacy files are
// accepted together so existing workspaces migrate on their next successful
// scan, but a partial legacy pair is rejected.
func loadAccountData(projectDir string) (nte.Inventory, decoded.State, bool, error) {
	var snapshot accountSnapshot
	loaded, err := readOptionalJSON(workspaceFile(projectDir, "account_snapshot.json"), &snapshot)
	if err != nil {
		return nte.Inventory{}, decoded.State{}, false, err
	}
	if loaded {
		if snapshot.SchemaVersion != accountSnapshotSchemaVersion {
			return nte.Inventory{}, decoded.State{}, false, fmt.Errorf("unsupported account snapshot schema version %d", snapshot.SchemaVersion)
		}
		snapshot.Inventory.Normalize()
		return snapshot.Inventory, snapshot.State, true, nil
	}

	inventory, inventoryLoaded, err := readOptionalInventory(workspaceFile(projectDir, "inventory.json"))
	if err != nil {
		return nte.Inventory{}, decoded.State{}, false, err
	}
	var state decoded.State
	stateLoaded, err := readOptionalJSON(workspaceFile(projectDir, "decoded_state.json"), &state)
	if err != nil {
		return nte.Inventory{}, decoded.State{}, false, err
	}
	if inventoryLoaded != stateLoaded {
		return nte.Inventory{}, decoded.State{}, false, fmt.Errorf("incomplete legacy account workspace: inventory and decoded state must belong to the same import")
	}
	return inventory, state, inventoryLoaded, nil
}

func writeAccountData(projectDir string, inventory nte.Inventory, state decoded.State) error {
	return writeJSON(workspaceFile(projectDir, "account_snapshot.json"), accountSnapshot{
		SchemaVersion: accountSnapshotSchemaVersion,
		Inventory:     inventory,
		State:         state,
	})
}
