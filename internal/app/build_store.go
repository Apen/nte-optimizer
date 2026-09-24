package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
)

const localBuildSchemaVersion = 1

type LocalBuild struct {
	CharacterID int                 `json:"character_id"`
	ProfileID   string              `json:"profile_id"`
	ModuleIDs   []string            `json:"module_ids"`
	CartridgeID string              `json:"cartridge_id,omitempty"`
	ArcID       string              `json:"arc_id,omitempty"`
	Stats       map[string]float64  `json:"stats,omitempty"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Result      *OptimizationResult `json:"result,omitempty"`
}

func SaveOptimizationResult(projectDir string, result OptimizationResult) (LocalBuildState, error) {
	if result.Character == nil {
		return LocalBuildState{}, fmt.Errorf("optimization result has no character")
	}
	moduleIDs := make([]string, 0, len(result.Modules))
	for _, module := range result.Modules {
		moduleIDs = append(moduleIDs, module.Module.LocalID)
	}
	cartridgeID := ""
	if result.Cartridge != nil {
		cartridgeID = result.Cartridge.LocalID
	}
	arcID := ""
	if result.Weapon != nil {
		arcID = result.Weapon.ID.Key()
	}
	builds, err := SaveLocalBuild(projectDir, result.ProfileID, result.Character.CharacterID, moduleIDs, cartridgeID, result.Stats.Derived, arcID)
	if err != nil {
		return LocalBuildState{}, err
	}
	key := strconv.Itoa(result.Character.CharacterID)
	build := builds.Builds[key]
	copy := result
	build.Result = &copy
	builds.Builds[key] = build
	return builds, writeJSON(workspaceFile(projectDir, "local_builds.json"), builds)
}

func SavedOptimizationResult(projectDir string, characterID int) (*OptimizationResult, error) {
	_, builds, err := loadBuildData(projectDir)
	if err != nil {
		return nil, err
	}
	build, ok := builds.Builds[strconv.Itoa(characterID)]
	if !ok {
		return nil, fmt.Errorf("no saved build for character %d", characterID)
	}
	if build.Result == nil {
		return nil, fmt.Errorf("saved build predates detailed snapshots; optimize and equip it again")
	}
	return build.Result, nil
}

type LocalBuildState struct {
	SchemaVersion int                   `json:"schema_version"`
	Priority      []int                 `json:"priority"`
	Builds        map[string]LocalBuild `json:"builds"`
}

type WorkspaceCharacter struct {
	CharacterID int         `json:"character_id"`
	Name        string      `json:"name"`
	Level       int         `json:"level"`
	AwakenLevel int         `json:"awaken_level"`
	Priority    int         `json:"priority"`
	Build       *LocalBuild `json:"build,omitempty"`
}

type BuildWorkspace struct {
	Characters []WorkspaceCharacter `json:"characters"`
}

func LoadBuildWorkspace(projectDir, dataDir, language string) (BuildWorkspace, error) {
	state, builds, err := loadBuildData(projectDir)
	if err != nil {
		return BuildWorkspace{}, err
	}
	builds.normalize(state.Characters)
	if len(state.Characters) == 0 {
		return BuildWorkspace{Characters: []WorkspaceCharacter{}}, nil
	}
	catalog, err := ntelocale.Load(dataDir, language)
	if err != nil {
		return BuildWorkspace{}, fmt.Errorf("load locale catalog: %w", err)
	}
	byID := make(map[int]decoded.Character, len(state.Characters))
	for _, character := range state.Characters {
		byID[character.CharacterID] = character
	}
	result := BuildWorkspace{Characters: make([]WorkspaceCharacter, 0, len(state.Characters))}
	for index, id := range builds.Priority {
		character, ok := byID[id]
		if !ok {
			continue
		}
		entry := WorkspaceCharacter{CharacterID: id, Name: cleanCharacterName(catalog.CharacterName(id, character.Name)), Level: character.Level, AwakenLevel: character.AwakenLevel, Priority: index + 1}
		if build, exists := builds.Builds[strconv.Itoa(id)]; exists {
			copy := build
			entry.Build = &copy
		}
		result.Characters = append(result.Characters, entry)
	}
	return result, nil
}

func cleanCharacterName(name string) string {
	return cleanCategorizedGameName(name)
}

func cleanArcName(name string) string {
	return cleanCategorizedGameName(name)
}

func cleanCategorizedGameName(name string) string {
	name = strings.TrimSpace(name)
	if _, value, found := strings.Cut(name, "·"); found && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return name
}

func SaveLocalBuild(projectDir, profileID string, characterID int, moduleIDs []string, cartridgeID string, stats map[string]float64, arcIDs ...string) (LocalBuildState, error) {
	state, builds, err := loadBuildData(projectDir)
	if err != nil {
		return LocalBuildState{}, err
	}
	builds.normalize(state.Characters)
	if !containsCharacter(state.Characters, characterID) {
		return LocalBuildState{}, fmt.Errorf("unknown account character %d", characterID)
	}
	inventory, account, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return LocalBuildState{}, err
	}
	if !loaded {
		return LocalBuildState{}, fmt.Errorf("no account has been imported")
	}
	availableModules := make(map[string]bool, len(inventory.Modules))
	for _, module := range inventory.Modules {
		availableModules[module.LocalID] = true
	}
	seen := map[string]bool{}
	for _, id := range moduleIDs {
		if !availableModules[id] || seen[id] {
			return LocalBuildState{}, fmt.Errorf("invalid or duplicate module %q", id)
		}
		seen[id] = true
	}
	if cartridgeID != "" {
		found := false
		for _, cartridge := range inventory.Cartridges {
			found = found || cartridge.LocalID == cartridgeID
		}
		if !found {
			return LocalBuildState{}, fmt.Errorf("unknown cartridge %q", cartridgeID)
		}
	}
	arcID := ""
	if len(arcIDs) > 0 {
		arcID = arcIDs[0]
	}
	if arcID != "" {
		found := false
		for _, weapon := range account.Weapons {
			found = found || weapon.ID.Key() == arcID
		}
		if !found {
			return LocalBuildState{}, fmt.Errorf("unknown Arc %q", arcID)
		}
	}
	builds.Builds[strconv.Itoa(characterID)] = LocalBuild{CharacterID: characterID, ProfileID: profileID, ModuleIDs: append([]string(nil), moduleIDs...), CartridgeID: cartridgeID, ArcID: arcID, Stats: stats, UpdatedAt: time.Now().UTC()}
	return builds, writeJSON(workspaceFile(projectDir, "local_builds.json"), builds)
}

func SaveCharacterPriority(projectDir string, priority []int) (LocalBuildState, error) {
	state, builds, err := loadBuildData(projectDir)
	if err != nil {
		return LocalBuildState{}, err
	}
	builds.normalize(state.Characters)
	if len(priority) != len(builds.Priority) {
		return LocalBuildState{}, fmt.Errorf("priority must contain every account character")
	}
	expected := map[int]bool{}
	for _, id := range builds.Priority {
		expected[id] = true
	}
	seen := map[int]bool{}
	for _, id := range priority {
		if !expected[id] || seen[id] {
			return LocalBuildState{}, fmt.Errorf("invalid character priority")
		}
		seen[id] = true
	}
	builds.Priority = append([]int(nil), priority...)
	return builds, writeJSON(workspaceFile(projectDir, "local_builds.json"), builds)
}

func HigherPriorityReservations(projectDir string, characterID int) (map[string]bool, map[string]bool, error) {
	modules, cartridges, _, err := HigherPriorityReservationsWithArcs(projectDir, characterID)
	return modules, cartridges, err
}

func HigherPriorityReservationsWithArcs(projectDir string, characterID int) (map[string]bool, map[string]bool, map[string]bool, error) {
	state, builds, err := loadBuildData(projectDir)
	if err != nil {
		return nil, nil, nil, err
	}
	builds.normalize(state.Characters)
	inventory, account, loaded, err := loadAccountData(projectDir)
	if err != nil {
		return nil, nil, nil, err
	}
	if !loaded {
		return nil, nil, nil, fmt.Errorf("no account has been imported")
	}
	modules, cartridges, arcs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, id := range builds.Priority {
		if id == characterID {
			return modules, cartridges, arcs, nil
		}
		// The imported account state is authoritative for equipment currently
		// worn by a higher-priority character, even when that character has no
		// locally saved build yet.
		for _, module := range inventory.Modules {
			if module.EquippedCharacterID == id && module.LocalID != "" {
				modules[module.LocalID] = true
			}
		}
		for _, cartridge := range inventory.Cartridges {
			if cartridge.EquippedCharacterID == id && cartridge.LocalID != "" {
				cartridges[cartridge.LocalID] = true
			}
		}
		for _, weapon := range account.Weapons {
			if weapon.EquippedCharacterID == id && (weapon.ID.Slot != 0 || weapon.ID.Serial != 0) {
				arcs[weapon.ID.Key()] = true
			}
		}
		for _, character := range account.Characters {
			if character.CharacterID == id && character.ForkNetID != nil && (character.ForkNetID.Slot != 0 || character.ForkNetID.Serial != 0) {
				arcs[character.ForkNetID.Key()] = true
			}
		}
		if build, ok := builds.Builds[strconv.Itoa(id)]; ok {
			for _, moduleID := range build.ModuleIDs {
				if moduleID != "" {
					modules[moduleID] = true
				}
			}
			if build.CartridgeID != "" {
				cartridges[build.CartridgeID] = true
			}
			if build.ArcID != "" {
				arcs[build.ArcID] = true
			}
		}
	}
	return modules, cartridges, arcs, nil
}

func loadBuildData(projectDir string) (decoded.State, LocalBuildState, error) {
	_, state, stateLoaded, err := loadAccountData(projectDir)
	if err != nil {
		return state, LocalBuildState{}, err
	}
	builds := LocalBuildState{SchemaVersion: localBuildSchemaVersion, Builds: map[string]LocalBuild{}}
	buildsLoaded, err := readOptionalJSON(workspaceFile(projectDir, "local_builds.json"), &builds)
	if err != nil {
		return state, LocalBuildState{}, err
	}
	if !stateLoaded && buildsLoaded {
		return state, LocalBuildState{}, fmt.Errorf("incomplete account workspace: local builds exist without decoded account state")
	}
	if builds.Builds == nil {
		builds.Builds = map[string]LocalBuild{}
	}
	return state, builds, nil
}

func (s *LocalBuildState) normalize(characters []decoded.Character) {
	known, included := map[int]bool{}, map[int]bool{}
	for _, character := range characters {
		known[character.CharacterID] = true
	}
	priority := make([]int, 0, len(characters))
	for _, id := range s.Priority {
		if known[id] && !included[id] {
			priority = append(priority, id)
			included[id] = true
		}
	}
	rest := make([]int, 0)
	for _, character := range characters {
		if !included[character.CharacterID] {
			rest = append(rest, character.CharacterID)
		}
	}
	sort.Ints(rest)
	s.Priority = append(priority, rest...)
	s.SchemaVersion = localBuildSchemaVersion
}

func containsCharacter(characters []decoded.Character, id int) bool {
	for _, character := range characters {
		if character.CharacterID == id {
			return true
		}
	}
	return false
}
