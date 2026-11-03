package damage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPublishedComboHitsMatchActionEvidence(t *testing.T) {
	dir := filepath.Join("..", "..", "data", "game", "combat")
	catalog, err := ReadCatalog(filepath.Join(dir, "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "action_evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Actions []struct {
			ID          string `json:"id"`
			CharacterID int    `json:"character_id"`
			Effects     []struct {
				ID              int `json:"effect_id"`
				HitCount        int `json:"hit_count"`
				MaxTriggerCount int `json:"max_trigger_count"`
			} `json:"effects"`
		} `json:"actions"`
		Unresolved []struct {
			Effects []struct {
				ID   int    `json:"numeric_id"`
				Name string `json:"name"`
			} `json:"effects"`
		} `json:"unresolved"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	effectNames := map[int]string{}
	for _, group := range evidence.Unresolved {
		for _, effect := range group.Effects {
			if effect.ID != 0 {
				effectNames[effect.ID] = effect.Name
			}
		}
	}
	wantGroups := map[string]string{
		"shinku_tidal_assault_stage_3": "GA_Shinku_Melee_Rage",
		"blackbird_pyreflame":          "GA_BlackBird_Evolve_Melee",
	}
	for _, action := range evidence.Actions {
		group, ok := wantGroups[action.ID]
		if !ok {
			continue
		}
		delete(wantGroups, action.ID)
		if len(action.Effects) == 0 {
			t.Fatalf("%s: no effect evidence", action.ID)
		}
		characterID := strconv.Itoa(action.CharacterID)
		instances := map[string]TieredInstance{}
		for _, instance := range catalog.Characters[characterID].Instances {
			instances[instance.ID] = instance
		}
		for _, effect := range action.Effects {
			name := effectNames[effect.ID]
			instance, ok := instances[name]
			if !ok || instance.OmitFromActionPreview || instance.ActionID != group {
				t.Errorf("%s: effect %d (%s) must be included in %s", action.ID, effect.ID, name, group)
				continue
			}
			// The action preview assumes the target stays eligible for the entire combo.
			want := effect.HitCount
			if want == 0 {
				want = effect.MaxTriggerCount
			}
			if got := max(1, instance.Hits); want == 0 || got != want {
				t.Errorf("%s: effect %d has %d hits, evidence has %d", action.ID, effect.ID, got, want)
			}
		}
	}
	for id := range wantGroups {
		t.Errorf("missing evidence for %s", id)
	}
}
