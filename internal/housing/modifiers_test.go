package housing

import (
	"encoding/json"
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

func TestInferSharedModifiersMatchesPanelResidualAcrossLoadedCharacters(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "data", "game", "housing", "special_furniture_modifiers.json"))
	if err != nil {
		t.Fatal(err)
	}
	observations := []Observation{
		{CharacterID: 1, Loaded: true, Panel: map[string]float64{"AtkFinal": 1004, "DefFinal": 509, "CritDamageBase": .824}, Calculated: map[string]float64{"AtkFinal": 1000, "DefFinal": 500, "CritDamageBase": .8}},
		{CharacterID: 2, Loaded: true, Panel: map[string]float64{"AtkFinal": 1204.9, "DefFinal": 709, "CritDamageBase": 1.024}, Calculated: map[string]float64{"AtkFinal": 1200, "DefFinal": 700, "CritDamageBase": 1}},
		{CharacterID: 3, Loaded: true, Panel: map[string]float64{"AtkFinal": 1404.2, "DefFinal": 909, "CritDamageBase": 1.224}, Calculated: map[string]float64{"AtkFinal": 1400, "DefFinal": 900, "CritDamageBase": 1.2}},
		{CharacterID: 4, Loaded: true, Panel: map[string]float64{"AtkFinal": 1604.1, "DefFinal": 1109, "CritDamageBase": 1.424}, Calculated: map[string]float64{"AtkFinal": 1600, "DefFinal": 1100, "CritDamageBase": 1.4}},
		// A stale/unloaded character must not count as evidence.
		{CharacterID: 5, Loaded: false, Panel: map[string]float64{"AtkFinal": 1004, "DefFinal": 509, "CritDamageBase": .824}, Calculated: map[string]float64{"AtkFinal": 1000, "DefFinal": 500, "CritDamageBase": .8}},
	}

	got := InferShared(catalog, observations)
	if len(got) != 3 {
		t.Fatalf("inferred %d modifiers, want 3: %#v", len(got), got)
	}
	want := map[string]InferredModifier{
		"AtkFinal":       {ModifierID: "yaodao_1", PropertyID: "AtkAdd", PanelProperty: "AtkFinal", Value: 4, MatchingCharacters: 4, ObservedCharacters: 4},
		"DefFinal":       {ModifierID: "kaijia_2", PropertyID: "DefAdd", PanelProperty: "DefFinal", Value: 9, MatchingCharacters: 4, ObservedCharacters: 4},
		"CritDamageBase": {ModifierID: "quantao_5", PropertyID: "CritDamageBase", PanelProperty: "CritDamageBase", Value: .024, Percent: true, MatchingCharacters: 4, ObservedCharacters: 4},
	}
	for _, item := range got {
		if expected, ok := want[item.PanelProperty]; !ok || item != expected {
			t.Errorf("inferred candidate %#v, want %#v", item, expected)
		}
	}
}

func TestInferSharedDoesNotOverstateSingleCharacterOrAmbiguousResidual(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "data", "game", "housing", "special_furniture_modifiers.json"))
	if err != nil {
		t.Fatal(err)
	}
	single := []Observation{{Loaded: true, Panel: map[string]float64{"AtkFinal": 104}, Calculated: map[string]float64{"AtkFinal": 100}}}
	if got := InferShared(catalog, single); len(got) != 0 {
		t.Fatalf("single-character residual was presented as shared evidence: %#v", got)
	}

	ambiguousCatalog := Catalog{SchemaVersion: 1, Modifiers: []Modifier{
		{ID: "rank-a", Stats: []ModifierStat{{PropertyID: "AtkAdd", Value: 4, ModifierOp: "MODIFY_MODOP_ADDITIVE"}}},
		{ID: "rank-b", Stats: []ModifierStat{{PropertyID: "AtkAdd", Value: 6, ModifierOp: "MODIFY_MODOP_ADDITIVE"}}},
	}}
	ambiguous := []Observation{
		{Loaded: true, Panel: map[string]float64{"AtkFinal": 105}, Calculated: map[string]float64{"AtkFinal": 100}},
		{Loaded: true, Panel: map[string]float64{"AtkFinal": 205}, Calculated: map[string]float64{"AtkFinal": 200}},
		{Loaded: true, Panel: map[string]float64{"AtkFinal": 305}, Calculated: map[string]float64{"AtkFinal": 300}},
	}
	if got := InferShared(ambiguousCatalog, ambiguous); len(got) != 0 {
		t.Fatalf("ambiguous rank residual was presented as identified: %#v", got)
	}
}

func TestInferSharedRequiresUnconditionalAdditiveModifier(t *testing.T) {
	catalog := Catalog{SchemaVersion: 1, Modifiers: []Modifier{
		{ID: "conditional", Stats: []ModifierStat{{PropertyID: "AtkAdd", Value: 4, ModifierOp: "MODIFY_MODOP_ADDITIVE"}}, Conditions: []json.RawMessage{json.RawMessage(`{"condition":"example"}`)}},
		{ID: "multiplicative", Stats: []ModifierStat{{PropertyID: "AtkAdd", Value: 4, ModifierOp: "MODIFY_MODOP_MULTIPLY"}}},
	}}
	observations := []Observation{
		{Loaded: true, Panel: map[string]float64{"AtkFinal": 104}, Calculated: map[string]float64{"AtkFinal": 100}},
		{Loaded: true, Panel: map[string]float64{"AtkFinal": 204}, Calculated: map[string]float64{"AtkFinal": 200}},
	}
	if got := InferShared(catalog, observations); len(got) != 0 {
		t.Fatalf("unsupported effect was inferred: %#v", got)
	}
}
