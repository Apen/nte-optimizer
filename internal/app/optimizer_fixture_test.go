package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

func loadSanitizedInventoryFixture(t testing.TB) nte.Inventory {
	t.Helper()
	data, err := os.ReadFile("../testdata/optimizer/sanitized_inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory nte.Inventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatalf("decode sanitized inventory fixture: %v", err)
	}
	return inventory
}

func TestSanitizedInventoryFixtureMatchesRepresentativeShape(t *testing.T) {
	inventory := loadSanitizedInventoryFixture(t)
	if inventory.SchemaVersion != 1 || len(inventory.Modules) != 24 || len(inventory.Cartridges) != 4 {
		t.Fatalf("unexpected fixture dimensions: schema=%d modules=%d cartridges=%d", inventory.SchemaVersion, len(inventory.Modules), len(inventory.Cartridges))
	}
	areas := map[int]int{}
	geometries := map[string]bool{}
	equipped := 0
	for _, module := range inventory.Modules {
		areas[module.Area]++
		geometries[module.Geometry] = true
		if module.EquippedCharacterID != 0 {
			equipped++
		}
		if len(module.MainStats) != 2 || len(module.SubStats) != 4 {
			t.Errorf("synthetic module %q has %d main stats and %d substats", module.LocalID, len(module.MainStats), len(module.SubStats))
		}
		if len(module.GameItemID) != 0 || len(module.SetName) != 0 {
			t.Errorf("fixture module %q contains display or imported identifiers", module.LocalID)
		}
		if !strings.HasPrefix(module.LocalID, "synthetic-") {
			t.Errorf("fixture module ID %q is not synthetic", module.LocalID)
		}
	}
	if areas[2] != 4 || areas[3] != 12 || areas[4] != 8 || len(geometries) != 12 || equipped != 2 {
		t.Fatalf("unexpected fixture distribution: areas=%v geometries=%d equipped=%d", areas, len(geometries), equipped)
	}
	for _, cartridge := range inventory.Cartridges {
		if len(cartridge.GameItemID) != 0 || len(cartridge.SetName) != 0 || !strings.HasPrefix(cartridge.LocalID, "synthetic-") {
			t.Errorf("fixture cartridge %q contains imported or personal identifiers", cartridge.LocalID)
		}
	}
}

func TestSanitizedInventoryCandidatePreparationAndSelection(t *testing.T) {
	inventory := loadSanitizedInventoryFixture(t)
	service := OptimizerService{
		ExcludedModuleIDs:    map[string]bool{"synthetic-03-h3-d": true},
		ReservedModuleIDs:    map[string]bool{"synthetic-04-tv-b": true},
		ReservedCartridgeIDs: map[string]bool{"synthetic-cartridge-a1": true},
	}
	profile := scoring.Character{CharacterID: 41, Weights: map[string]float64{"CritBase": 1}}
	refs := scoring.References{"CritBase": .064}
	pool := service.prepareModuleCandidates(inventory.Modules, profile, refs, nil, nil, nil, false)
	if len(pool.eligible) != 21 || len(pool.current) != 1 || pool.excludedEquipped != 2 {
		t.Fatalf("unexpected sanitized candidate pool: eligible=%d current=%d excluded=%d", len(pool.eligible), len(pool.current), pool.excludedEquipped)
	}
	config := optimizerDataConfig{}
	config.Optimize.TopPerGeometry = 1
	config.Optimize.TopPerSet = 2
	selection, err := service.selectModuleCandidates(pool, config, searchPlan{requestedMode: "score", solverMode: "score", approximate: true})
	if err != nil {
		t.Fatal(err)
	}
	selectedIDs := candidateIDs(selection.selected)
	if selectedIDs["synthetic-03-h3-d"] || selectedIDs["synthetic-04-tv-b"] {
		t.Fatalf("excluded or reserved equipment was selected: %#v", selection.selected)
	}
	if !selectedIDs["synthetic-02-h2-a"] {
		t.Fatal("low-scoring currently equipped module was not restored after preselection")
	}
	if len(selection.selected) >= len(pool.eligible) {
		t.Fatalf("sanitized fixture did not reduce the search pool: selected=%d eligible=%d", len(selection.selected), len(pool.eligible))
	}

	sets := optimizer.SetCatalog{Definitions: map[string]optimizer.SetDefinition{
		"synthetic-a": {ID: "synthetic-a", InventorySetID: "synthetic-a", RequiredGeometries: []string{"H_2"}},
		"synthetic-b": {ID: "synthetic-b", InventorySetID: "synthetic-b", RequiredGeometries: []string{"H_3"}},
	}}
	cartridges, err := service.prepareCartridges(inventory.Cartridges, profile, sets, false, searchPlan{requestedMode: "fast", solverMode: "objective", approximate: true}, len(pool.eligible))
	if err != nil {
		t.Fatal(err)
	}
	if len(cartridges.available) != 2 || cartridges.excludedEquipped != 2 {
		t.Fatalf("unexpected sanitized cartridge pool: available=%d excluded=%d", len(cartridges.available), cartridges.excludedEquipped)
	}
}
