package locale

import (
	"encoding/json"
	"fmt"
	"nte-optimizer/internal/datafiles"
	"os"
	"strconv"
	"strings"
)

var Supported = []string{"fr", "en"}

type Item struct {
	Name string `json:"name"`
}

type Catalog struct {
	Schema        string                        `json:"schema"`
	SchemaVersion int                           `json:"schema_version"`
	Locale        string                        `json:"locale"`
	Sources       map[string]PresentationSource `json:"sources,omitempty"`
	UI            map[string]string             `json:"ui,omitempty"`
	Stats         map[string]string             `json:"stats,omitempty"`
	Qualities     map[string]string             `json:"qualities,omitempty"`
	Geometries    map[string]string             `json:"geometries,omitempty"`
	StatSources   map[string]string             `json:"stat_sources,omitempty"`
	Damage        map[string]PresentationDamage `json:"damage,omitempty"`
	Abilities     map[string]string             `json:"abilities,omitempty"`
	Items         map[string]Item               `json:"items,omitempty"`
	Resources     map[string]string             `json:"resources,omitempty"`
	Sets          map[string]string             `json:"sets,omitempty"`
}

// PresentationCatalog contains labels owned by the application. Labels
// extracted from NTE are loaded independently from data/game/locales.
type PresentationCatalog struct {
	Schema        string                        `json:"schema"`
	SchemaVersion int                           `json:"schema_version"`
	Locale        string                        `json:"locale"`
	Sources       map[string]PresentationSource `json:"sources,omitempty"`
	UI            map[string]string             `json:"ui"`
	Stats         map[string]string             `json:"stats"`
	Qualities     map[string]string             `json:"qualities"`
	Geometries    map[string]string             `json:"geometries"`
	StatSources   map[string]string             `json:"stat_sources"`
	Damage        map[string]PresentationDamage `json:"damage"`
	Abilities     map[string]string             `json:"abilities,omitempty"`
}

type PresentationSource struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type PresentationDamage struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func Normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if cut := strings.IndexAny(value, "-_"); cut >= 0 {
		value = value[:cut]
	}
	for _, supported := range Supported {
		if value == supported {
			return value
		}
	}
	return "fr"
}

func Load(dataDir, language string) (Catalog, error) {
	language = Normalize(language)
	layout := datafiles.New(dataDir)
	b, err := os.ReadFile(layout.Locale(language))
	if err != nil {
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(b, &catalog); err != nil {
		return Catalog{}, err
	}
	if !((catalog.Schema == "nte-optimizer-locale" || catalog.Schema == "nte-map-locale") && catalog.SchemaVersion == 1 && catalog.Locale == language) {
		return Catalog{}, fmt.Errorf("unsupported or mismatched %s locale catalog", language)
	}
	game, err := loadGameLocale(layout.GameLocale(language), language)
	if err != nil {
		return Catalog{}, err
	}
	catalog.Items = game.itemLabels()
	catalog.Abilities = game.skillLabels()
	catalog.Resources = game.Tables["resources"]
	catalog.Sets = game.Tables["sets"]
	return catalog, nil
}

type gameLocale struct {
	SchemaVersion int                          `json:"schema_version"`
	Language      string                       `json:"language"`
	Tables        map[string]map[string]string `json:"tables"`
}

func loadGameLocale(path, language string) (gameLocale, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return gameLocale{}, fmt.Errorf("load game locale: %w", err)
	}
	var catalog gameLocale
	if err := json.Unmarshal(b, &catalog); err != nil {
		return gameLocale{}, fmt.Errorf("decode game locale: %w", err)
	}
	if catalog.SchemaVersion != 1 || catalog.Language != language || catalog.Tables == nil {
		return gameLocale{}, fmt.Errorf("unsupported or mismatched %s game locale", language)
	}
	return catalog, nil
}

func (c gameLocale) itemLabels() map[string]Item {
	items := map[string]Item{}
	for _, table := range []string{"characters", "forks"} {
		for id, name := range c.Tables[table] {
			items[id] = Item{Name: name}
		}
	}
	return items
}

func (c gameLocale) skillLabels() map[string]string {
	abilities := map[string]string{}
	for table, values := range c.Tables {
		if !strings.HasSuffix(strings.ToLower(table), "skilldes") {
			continue
		}
		for key, value := range values {
			if strings.HasSuffix(key, "_name") {
				key = strings.TrimSuffix(key, "_name")
				key = strings.ReplaceAll(key, "UtraSkill", "UltraSkill")
				abilities[key] = value
			}
		}
	}
	return abilities
}

func LoadPresentation(dataDir, language string) (PresentationCatalog, error) {
	catalog, err := Load(dataDir, language)
	if err != nil {
		return PresentationCatalog{}, err
	}
	return PresentationCatalog{
		Schema:        catalog.Schema,
		SchemaVersion: catalog.SchemaVersion,
		Locale:        catalog.Locale,
		Sources:       catalog.Sources,
		UI:            catalog.UI,
		Stats:         catalog.Stats,
		Qualities:     catalog.Qualities,
		Geometries:    catalog.Geometries,
		StatSources:   catalog.StatSources,
		Damage:        catalog.Damage,
		Abilities:     catalog.Abilities,
	}, nil
}

func (c Catalog) ItemName(id, fallback string) string {
	if item, ok := c.Items[id]; ok && strings.TrimSpace(item.Name) != "" {
		return strings.TrimSpace(item.Name)
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return id
}

func (c Catalog) ResourceName(id, fallback string) string {
	if name := strings.TrimSpace(c.Resources[id]); name != "" {
		return name
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return id
}

func (c Catalog) SetName(id, fallback string) string {
	if name := strings.TrimSpace(c.Sets[id]); name != "" {
		return name
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return id
}

// CharacterName resolves the numeric character ID and removes the catalog's
// display category ("Personnage·" / "Character·") from the UI-facing name.
func (c Catalog) CharacterName(id int, fallback string) string {
	name := c.ItemName(strconv.Itoa(id), fallback)
	if _, value, found := strings.Cut(name, "·"); found && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return name
}
