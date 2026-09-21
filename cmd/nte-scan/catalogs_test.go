package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionDecodeCatalogsContainOnlyRuntimeData(t *testing.T) {
	root := filepath.Join("..", "..", "data", "game")
	charactersPath := filepath.Join(root, "characters", "decode.json")
	equipmentPath := filepath.Join(root, "equipment", "decode.json")
	forksPath := filepath.Join(root, "forks", "decode.json")
	resourcesPath := filepath.Join(root, "resources", "decode.json")

	characters, err := loadCharacterCatalog(charactersPath)
	if err != nil || len(characters.Characters) < 20 || !strings.EqualFold(characters.Characters["1036"].Codename, "Zankou") {
		t.Fatalf("invalid character decode catalog: count=%d err=%v", len(characters.Characters), err)
	}
	equipment, err := loadEquipmentCatalog(equipmentPath)
	if err != nil || len(equipment.Items) < 70 || len(equipment.Attributes) == 0 || len(equipment.Curves) == 0 || len(equipment.Shapes) == 0 || len(equipment.Suits) == 0 {
		t.Fatalf("invalid equipment decode catalog: %#v err=%v", equipment, err)
	}
	forks, err := loadForkCatalog(forksPath)
	demonBlade := forks.Forks["fork_DemonBlade"]
	if err != nil || len(forks.Forks) != 49 || demonBlade.Quality == "" || demonBlade.MaxBreakthrough <= 0 || demonBlade.MaxStar <= 0 {
		t.Fatalf("invalid fork decode catalog: count=%d err=%v", len(forks.Forks), err)
	}
	resources, err := loadResourceCatalog(resourcesPath)
	if err != nil || len(resources.Items) == 0 {
		t.Fatalf("invalid resource decode catalog: count=%d err=%v", len(resources.Items), err)
	}

	for _, path := range []string{charactersPath, equipmentPath, forksPath, resourcesPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, obsolete := range []string{"description_zh", "passives", "source_kind", "referenced_missing", "verified", "name_ja"} {
			if strings.Contains(text, `"`+obsolete+`"`) {
				t.Fatalf("%s still contains unused field %q", path, obsolete)
			}
		}
	}

	forkData, err := os.ReadFile(forksPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(forkData), `"name_zh"`) {
		t.Fatalf("%s still contains localized names unused by the decoder", forksPath)
	}

	characterData, err := os.ReadFile(charactersPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(characterData), `"name_en"`) {
		t.Fatalf("%s still contains display names duplicated by codename", charactersPath)
	}

	var equipmentRaw struct {
		Items map[string]map[string]json.RawMessage `json:"items"`
		Suits map[string]map[string]json.RawMessage `json:"suits"`
	}
	equipmentData, err := os.ReadFile(equipmentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(equipmentData, &equipmentRaw); err != nil {
		t.Fatal(err)
	}
	for itemID, definition := range equipmentRaw.Items {
		if _, exists := definition["name_en"]; exists {
			t.Fatalf("%s item %q still contains a display name unused by the decoder", equipmentPath, itemID)
		}
	}
	for suitID, definition := range equipmentRaw.Suits {
		if _, exists := definition["name_en"]; !exists {
			t.Fatalf("%s suit %q lost its displayed observed-buff name", equipmentPath, suitID)
		}
	}

	var resourcesRaw struct {
		Items map[string]map[string]json.RawMessage `json:"items"`
	}
	resourceData, err := os.ReadFile(resourcesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(resourceData, &resourcesRaw); err != nil {
		t.Fatal(err)
	}
	for itemID, definition := range resourcesRaw.Items {
		if len(definition) != 0 {
			t.Fatalf("%s resource %q contains fields unused by the decoder", resourcesPath, itemID)
		}
	}
}
