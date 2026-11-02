package housing

import (
	"path/filepath"
	"testing"
)

func TestLoadSpecialFurnitureModifierCatalog(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "data", "game", "housing", "special_furniture_modifiers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != 1 || len(catalog.Modifiers) != 30 {
		t.Fatalf("catalog schema/count = %d/%d, want 1/30", catalog.SchemaVersion, len(catalog.Modifiers))
	}
	byID := make(map[string]Modifier, len(catalog.Modifiers))
	for _, modifier := range catalog.Modifiers {
		byID[modifier.ID] = modifier
	}
	for id, expected := range map[string]struct {
		property string
		value    float64
	}{
		"yaodao_1":  {"AtkAdd", 4},
		"kaijia_2":  {"DefAdd", 9},
		"quantao_5": {"CritDamageBase", .024},
	} {
		modifier, ok := byID[id]
		if !ok || len(modifier.Stats) != 1 || modifier.Stats[0].PropertyID != expected.property || modifier.Stats[0].Value != expected.value {
			t.Errorf("modifier %s = %#v, want %s + %v", id, modifier, expected.property, expected.value)
			continue
		}
		if modifier.Stats[0].ModifierOp != "MODIFY_MODOP_ADDITIVE" || len(modifier.Conditions) != 0 {
			t.Errorf("modifier %s operation/conditions = %q/%v, want unconditional additive", id, modifier.Stats[0].ModifierOp, modifier.Conditions)
		}
		if (expected.property == "CritDamageBase") != modifier.Stats[0].Percent {
			t.Errorf("modifier %s percent = %t, want %t", id, modifier.Stats[0].Percent, expected.property == "CritDamageBase")
		}
	}
}

func TestFurnitureProgressionMetadataIsUnambiguous(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "data", "game", "housing", "special_furniture_modifiers.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, modifier := range catalog.Modifiers {
		if modifier.ProgressionID == "" || modifier.Level < 1 || modifier.Level > 10 {
			t.Fatalf("missing generated metadata: %v", modifier)
		}
	}
	catalog.Modifiers[1].ProgressionID = catalog.Modifiers[0].ProgressionID
	catalog.Modifiers[1].Level = catalog.Modifiers[0].Level
	if err := catalog.Validate(); err == nil {
		t.Fatal("accepted ambiguous progression level")
	}
}
