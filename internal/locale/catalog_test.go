package locale

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	for _, language := range []string{"fr", "en"} {
		catalog, err := Load(dataDir, language)
		if err != nil {
			t.Fatal(err)
		}
		if got := catalog.Abilities["GA_Zankou_Skill"]; strings.TrimSpace(got) == "" {
			t.Fatalf("%s ability label is empty", language)
		}
	}
}

func TestProductionCatalogLoadsEveryResourceLabelFromGameLocale(t *testing.T) {
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
			if label, exists := catalog.Resources[id]; !exists || strings.TrimSpace(label) == "" {
				t.Errorf("%s resource %s has no localized game label", language, id)
			}
			if _, duplicated := catalog.Items[id]; duplicated {
				t.Errorf("%s resource %s is duplicated in the character/arc item catalog", language, id)
			}
		}
	}
}

func TestProductionGameLocalesCoverEveryRuntimeCatalogID(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	gameDir := filepath.Join(dataDir, "game")
	expected := map[string][]string{
		"characters": objectKeys(t, filepath.Join(gameDir, "characters", "decode.json"), "characters"),
		"forks":      objectKeys(t, filepath.Join(gameDir, "forks", "decode.json"), "forks"),
		"resources":  objectKeys(t, filepath.Join(gameDir, "resources", "decode.json"), "items"),
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
