package locale

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
)

func TestCatalogUsesStableFallbackForUnknownGameObject(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	for _, language := range []string{"fr", "en"} {
		catalog, err := Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		if got := catalog.CharacterName(999999, "Fallback"); got != "Fallback" {
			t.Fatalf("fallback: got %q", got)
		}
	}
}

func TestProductionCatalogLoadsSkillLabelsFromGameLocale(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	required := []string{
		"GA_Zankou_Skill",
		"GA_Mint019_UltraSkill",
		"GA_Female046_QTE",
		"GA_Female051_QTE",
		"GA_Jin_QTE",
		"GA_Radio072_Melee",
		"GA_Radio072_Skill",
		"GA_Radio072_UltraSkill",
		"GA_Radio072_QTE",
		"GA_Oneiroi_QTE",
		"GA_Shinku_Melee",
		"GA_Shinku_UltraSkill",
	}
	for _, language := range []string{"fr", "en"} {
		catalog, err := Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		for _, abilityID := range required {
			if got := catalog.Abilities[abilityID]; strings.TrimSpace(got) == "" {
				t.Errorf("%s ability label %s is empty", language, abilityID)
			}
		}
	}
}

func TestProductionCatalogLoadsAvailableResourceLabelsFromGameLocale(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	var decoded struct {
		Items map[string]json.RawMessage `json:"items"`
	}
	content, err := os.ReadFile(filepath.Join(dataDir, "game", "resources", "decode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"fr", "en"} {
		catalog, err := Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		for id := range decoded.Items {
			if label, exists := catalog.Resources[id]; exists && strings.TrimSpace(label) == "" {
				t.Errorf("%s resource %s has an empty localized game label", language, id)
			}
			if _, duplicated := catalog.Items[id]; duplicated {
				t.Errorf("%s resource %s is duplicated in the character/arc item catalog", language, id)
			}
		}
	}
}

func TestResourceNameFallsBackToIDWhenGameLabelIsUnavailable(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "data"), "en")
	if err != nil {
		t.Fatal(err)
	}
	if got := catalog.ResourceName("Fishbait01", ""); got != "Fishbait01" {
		t.Fatalf("ResourceName() = %q, want ID fallback %q", got, "Fishbait01")
	}
}

func TestProductionCatalogLoadsLocalizedConsoleTraitDescriptions(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	want := map[string]string{
		"en": "Increases CRIT DMG by 16% for each Type III Module equipped.",
		"fr": "Augmente DÉG CRIT de 16\u00a0% pour chaque module de type III équipé.",
	}
	var ids map[string]bool
	for _, language := range []string{"en", "fr"} {
		catalog, err := Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		if len(catalog.ConsoleTraitEffects) != 24 {
			t.Fatalf("%s console trait descriptions = %d, want 24", language, len(catalog.ConsoleTraitEffects))
		}
		if got := catalog.ConsoleTraitEffects["1036"]; got != want[language] {
			t.Errorf("%s Zankou console trait = %q, want %q", language, got, want[language])
		}
		if ids == nil {
			ids = make(map[string]bool, len(catalog.ConsoleTraitEffects))
			for id := range catalog.ConsoleTraitEffects {
				ids[id] = true
			}
		} else {
			for id := range catalog.ConsoleTraitEffects {
				if !ids[id] {
					t.Errorf("%s has unexpected console trait character ID %q", language, id)
				}
			}
		}
		for id, description := range catalog.ConsoleTraitEffects {
			if strings.TrimSpace(description) == "" {
				t.Errorf("%s console trait description for character %s is empty", language, id)
			}
		}

		presentation, err := LoadPresentation(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		if got := presentation.ConsoleTraitEffects["1036"]; got != want[language] {
			t.Errorf("%s presentation Zankou console trait = %q, want %q", language, got, want[language])
		}
	}
}

func TestProductionCatalogLoadsSpecialFurnitureProgressions(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	want := map[string]map[string]string{
		"en": {
			"SF_0011_lv10_desc":   "Increases deployed characters' ATK by <Grey>20</>.",
			"SF_0011_up_des_lv2":  "Deployed Characters ATK Bonus: 2 <img id=\"Image_up\"/> <Green>4</>",
			"SF_0012_lv10_desc":   "Increases deployed characters' CRIT DMG by <Grey>4%</>.",
			"SF_0012_up_des_lv10": "Deployed Characters CRIT DMG Bonus: 3.6% <img id=\"Image_up\"/> <Green>4%</>",
			"SF_0013_lv10_desc":   "Increases deployed characters' DEF by <Grey>30</>.",
			"SF_0013_up_des_lv10": "Deployed Character DEF Bonus: 27 <img id=\"Image_up\"/> <Green>30</>",
		},
		"fr": {
			"SF_0011_lv10_desc":   "Augmente l’ATQ des personnages déployés de <Grey>20</>.",
			"SF_0011_up_des_lv2":  "Bonus d’ATQ des personnages déployés : 2 <img id=\"Image_up\"/> <Green>4</>",
			"SF_0012_lv10_desc":   "Augmente les DÉG critiques des personnages déployés de <Grey>4 %</>.",
			"SF_0012_up_des_lv10": "Bonus de DÉG critiques des personnages déployés : 3,6 % <img id=\"Image_up\"/> <Green>4 %</>",
			"SF_0013_lv10_desc":   "Augmente la DÉF des personnages déployés de <Grey>30</>.",
			"SF_0013_up_des_lv10": "Bonus de DÉF des personnages déployés : 27 <img id=\"Image_up\"/> <Green>30</>",
		},
	}
	var expectedKeys map[string]bool
	for _, language := range []string{"en", "fr"} {
		catalog, err := Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		if len(catalog.Furniture) != 30 {
			t.Fatalf("%s special-furniture labels = %d, want 30", language, len(catalog.Furniture))
		}
		for key, value := range want[language] {
			if got := catalog.Furniture[key]; got != value {
				t.Errorf("%s furniture label %s = %q, want %q", language, key, got, value)
			}
		}
		for key, value := range catalog.Furniture {
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s furniture label %s is empty", language, key)
			}
		}
		if _, exists := catalog.Furniture["SF_0013_Level_Max_des"]; exists {
			t.Errorf("%s furniture catalog includes the known incorrect max-level DEF label", language)
		}
		if expectedKeys == nil {
			expectedKeys = make(map[string]bool, len(catalog.Furniture))
			for key := range catalog.Furniture {
				expectedKeys[key] = true
			}
		} else {
			for key := range catalog.Furniture {
				if !expectedKeys[key] {
					t.Errorf("%s furniture catalog has unexpected key %q", language, key)
				}
			}
		}

		presentation, err := LoadPresentation(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(presentation.Furniture, catalog.Furniture) {
			t.Errorf("%s presentation furniture labels do not match the loaded game locale", language)
		}
		payload, err := json.Marshal(presentation)
		if err != nil {
			t.Fatal(err)
		}
		var sections map[string]json.RawMessage
		if err := json.Unmarshal(payload, &sections); err != nil {
			t.Fatal(err)
		}
		var exposed map[string]string
		if err := json.Unmarshal(sections["furniture"], &exposed); err != nil {
			t.Fatalf("%s presentation furniture payload: %v", language, err)
		}
		if got := exposed["SF_0013_lv10_desc"]; got != want[language]["SF_0013_lv10_desc"] {
			t.Errorf("%s JSON furniture payload contains DEF progression %q, want %q", language, got, want[language]["SF_0013_lv10_desc"])
		}
	}
}

func TestProductionGameLocalesCoverEveryRuntimeCatalogID(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	gameDir := filepath.Join(dataDir, "game")
	expected := map[string][]string{
		"characters": objectKeys(t, filepath.Join(gameDir, "characters", "decode.json"), "characters"),
		"forks":      objectKeys(t, filepath.Join(gameDir, "forks", "decode.json"), "forks"),
		"sets":       arrayIDs(t, filepath.Join(gameDir, "equipment", "sets.json"), "sets"),
	}
	for _, language := range []string{"fr", "en"} {
		game, err := loadGameLocale(filepath.Join(gameDir, "locales", language+".json"), language)
		if err != nil {
			t.Fatal(err)
		}
		for table, ids := range expected {
			for _, id := range ids {
				if strings.TrimSpace(game.Tables[table][id]) == "" {
					t.Errorf("%s tables.%s is missing runtime id %q", language, table, id)
				}
			}
		}
		gameResources, err := loadGameLocale(filepath.Join(gameDir, "locales", language+".json"), language)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range objectKeys(t, filepath.Join(gameDir, "resources", "decode.json"), "items") {
			if label, exists := gameResources.Tables["resources"][id]; exists && strings.TrimSpace(label) == "" {
				t.Errorf("%s tables.resources contains an empty value for %q", language, id)
			}
		}
	}
}

func objectKeys(t *testing.T, path, property string) []string {
	t.Helper()
	var document map[string]json.RawMessage
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(document[property], &values); err != nil {
		t.Fatalf("decode %s.%s: %v", path, property, err)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func arrayIDs(t *testing.T, path, property string) []string {
	t.Helper()
	var document map[string]json.RawMessage
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	var values []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(document[property], &values); err != nil {
		t.Fatalf("decode %s.%s: %v", path, property, err)
	}
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return ids
}

func TestNormalizeFallsBackToFrench(t *testing.T) {
	if got := Normalize("en-US"); got != "en" {
		t.Fatalf("got %q", got)
	}
	if got := Normalize("de"); got != "fr" {
		t.Fatalf("got %q", got)
	}
}

func TestPresentationCatalogsHaveMatchingKeys(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dataDir := filepath.Join(filepath.Dir(filename), "..", "..", "data")
	fr, err := LoadPresentation(dataDir, "fr")
	if err != nil {
		t.Fatal(err)
	}
	en, err := LoadPresentation(dataDir, "en")
	if err != nil {
		t.Fatal(err)
	}
	compare := func(section string, left, right map[string]string) {
		t.Helper()
		for key := range left {
			if _, ok := right[key]; !ok {
				t.Errorf("%s key %q missing from en", section, key)
			}
		}
		for key := range right {
			if _, ok := left[key]; !ok {
				t.Errorf("%s key %q missing from fr", section, key)
			}
		}
	}
	compare("ui", fr.UI, en.UI)
	compare("stats", fr.Stats, en.Stats)
	compare("qualities", fr.Qualities, en.Qualities)
	compare("geometries", fr.Geometries, en.Geometries)
	compare("stat_sources", fr.StatSources, en.StatSources)
	compare("abilities", fr.Abilities, en.Abilities)
	compare("console_trait_effects", fr.ConsoleTraitEffects, en.ConsoleTraitEffects)
	compare("furniture", fr.Furniture, en.Furniture)
	for key := range fr.Damage {
		if _, ok := en.Damage[key]; !ok {
			t.Errorf("damage key %q missing from en", key)
		}
	}
	for key := range en.Damage {
		if _, ok := fr.Damage[key]; !ok {
			t.Errorf("damage key %q missing from fr", key)
		}
	}
}

func TestForkEffectParametersArePresentedInSourceOrder(t *testing.T) {
	catalog := decoded.ForkCatalog{Forks: map[string]decoded.ForkDefinition{
		"fork_DemonBlade": {EffectsByStar: map[string]decoded.ForkEffect{
			"1": {OrderedParameters: []decoded.ForkEffectParameter{
				{NameID: "buff_DemonBlade_Crit", Value: .16, IsPercent: true},
				{NameID: "buff_DemonBlade_CritDamageUp", Value: .09, IsPercent: true},
				{NameID: "buff_DemonBlade_CD", Value: 15},
			}},
		}},
	}}

	got := forkEffectParametersFromCatalog(catalog)["fork_DemonBlade_1"]
	want := []decoded.ForkEffectParameter{
		{NameID: "buff_DemonBlade_Crit", Value: .16, IsPercent: true},
		{NameID: "buff_DemonBlade_CritDamageUp", Value: .09, IsPercent: true},
		{NameID: "buff_DemonBlade_CD", Value: 15},
	}
	if len(got) != len(want) {
		t.Fatalf("presented parameters = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("presented parameter %d = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func TestProductionArcEffectParametersReachPresentationCatalogInOrder(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	presentation, err := LoadPresentation(dataDir, "fr")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]decoded.ForkEffectParameter{
		"fork_DemonBlade_1": {
			{NameID: "buff_DemonBlade_Crit", Value: .16, IsPercent: true},
			{NameID: "buff_DemonBlade_CritDamageUp", Value: .09, IsPercent: true},
			{NameID: "buff_DemonBlade_CD", Value: 15},
		},
		"fork_BlackBook_3": {
			{NameID: "buff_BlackBook2_Unbal", Value: 74},
			{NameID: "buff_BlackBook2_CD", Value: 20},
			{NameID: "buff_BlackBook2_CD2", Value: 5},
			{NameID: "buff_BlackBook2_DamageUpChaosBase", Value: .28, IsPercent: true},
			{NameID: "buff_BlackBook2_SkillDamage", Value: 2.6, IsPercent: true},
		},
		"fork_mamen_2": {
			{NameID: "buff_mamen_fons", Value: 100000},
			{NameID: "buff_mamen_CosmosUp", Value: .03, IsPercent: true},
		},
	}
	for key, expected := range want {
		if got := presentation.ForkEffectParameters[key]; !reflect.DeepEqual(got, expected) {
			t.Errorf("%s parameters = %#v, want %#v", key, got, expected)
		}
	}
}

func TestPresentationFilesContainOnlyApplicationLabels(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	for _, language := range []string{"fr", "en"} {
		content, err := os.ReadFile(filepath.Join(dataDir, "presentation", language+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var sections map[string]json.RawMessage
		if err := json.Unmarshal(content, &sections); err != nil {
			t.Fatal(err)
		}
		for _, section := range []string{"abilities", "banner_sources", "components", "items", "labels", "pools"} {
			if _, exists := sections[section]; exists {
				t.Errorf("%s presentation catalog still contains game section %q", language, section)
			}
		}
	}
}

func TestNormalizePresentationLocale(t *testing.T) {
	if got := Normalize("fr-FR"); got != "fr" {
		t.Fatalf("Normalize(fr-FR) = %q", got)
	}
	if got := Normalize("en_US"); got != "en" {
		t.Fatalf("Normalize(en_US) = %q", got)
	}
}
