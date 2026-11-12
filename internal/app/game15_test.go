package app

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

func TestGame15CharacterFactsAndDamageUseOwnEffectsWithAssumedLevelReviews(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	panel := map[string]float64{"AtkFinal": 1700, "HPFinal": 23000, "DefFinal": 800, "CritBase": .5, "CritDamageBase": 1.5}
	for _, tc := range []struct {
		id       int
		codename string
		count    int
	}{{1074, "Exe", 18}, {1090, "Elyms", 35}} {
		character := decoded.Character{CharacterID: tc.id, Level: 80, BreakthroughLevel: 6, Skills: []decoded.CharacterSkill{
			{AbilityID: "GA_" + tc.codename + "_Melee", Level: 3}, {AbilityID: "GA_" + tc.codename + "_Skill", Level: 5}, {AbilityID: "GA_" + tc.codename + "_UltraSkill", Level: 8}, {AbilityID: "GA_" + tc.codename + "_QTE", Level: 11},
		}}
		facts, err := service.characterInputs(character)
		if err != nil || facts.BaseStats["AtkBase"] <= 0 || facts.ConsoleTrait == nil {
			t.Fatalf("character %d facts: %+v %v", tc.id, facts, err)
		}
		for _, language := range []string{"en", "fr"} {
			labels, err := service.loadLocale(language)
			if err != nil || !strings.Contains(labels.CharacterName(tc.id, ""), tc.codename) {
				t.Fatalf("%s character %d locale: %v", language, tc.id, err)
			}
			batch := service.prepareDamageBatch(&character, nil, language)
			analysis := batch.evaluate(optimizer.StatSummary{Derived: panel})
			if analysis.Status != "atomic_preview" || len(analysis.Build) != tc.count {
				t.Fatalf("character %d output: %s/%d", tc.id, analysis.Status, len(analysis.Build))
			}
			for _, result := range analysis.Build {
				if !strings.HasPrefix(result.InstanceID, "GE_Player_"+tc.codename+"_") {
					t.Fatal("foreign sheet effect entered damage output")
				}
			}
			covered := 0
			for ability, p := range analysis.SkillLevelProjections {
				if p.Status != "assumed" || p.Review == nil || p.Review.NativeExecutionCertified || p.Review.Status != "assumed" || p.Review.SkillLevelSource != p.SourceAbilityID || p.Review.AbilityID != ability || len(p.Review.Evidence) == 0 {
					t.Fatalf("lost assumed level review for %s: %+v", ability, p)
				}
				for _, id := range p.Review.ExpectedEffectIDs {
					found := false
					for _, result := range analysis.Build {
						if result.InstanceID == id {
							found = true
							covered++
							level := 3
							if strings.HasSuffix(p.SourceAbilityID, "UltraSkill") {
								level = 8
							}
							for _, entry := range batch.catalog.Characters[p.Review.CharacterID].Instances {
								if entry.ID == id && result.Factors.Coefficient != entry.Coefficients[level-1] {
									t.Fatal("selected wrong imported level")
								}
							}
						}
					}
					if !found {
						t.Fatalf("projected effect missing: %s", id)
					}
				}
			}
			want := 3
			if tc.id == 1090 {
				want = 12
			}
			if covered != want {
				t.Fatalf("projection coverage %d, want %d", covered, want)
			}
		}
		file, err := os.Open(filepath.Join("..", "..", "assets", "game_ui", "characters", strconv.Itoa(tc.id)+".png"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = png.DecodeConfig(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestElymsAllySummaryShowsConditionalStackAmountWithoutActivatingIt(t *testing.T) {
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	for _, language := range []string{"en", "fr"} {
		labels, err := service.loadLocale(language)
		if err != nil {
			t.Fatal(err)
		}
		for _, stage := range []int{1, 2, 4, 6} {
			state := decoded.State{Characters: []decoded.Character{{CharacterID: 1090, Level: 80, BreakthroughLevel: stage}}}
			scene := &teamScenario{allies: []TeamAlly{{CharacterID: 1090}}, gear: map[int]teamGear{}}
			rows, err := service.allyBuffSummaries(&state, scene, labels)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if stage >= 2 {
				want = 1
			}
			if len(rows) != want {
				t.Fatalf("stage %d: %+v", stage, rows)
			}
			for _, row := range rows {
				if row.ID != "elyms_charge_team_cosmos_damage" || row.Value == nil || *row.Value != .0375 || !row.PerStack || row.Included || row.Condition != labels.UI["ally_buff_condition_active_character_gains_ultimate_energy"] {
					t.Fatalf("invalid conditional summary: %+v", row)
				}
			}
		}
	}
}

func TestGame15WorkspacePreservesImportedCharactersWithoutInventingProfiles(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	state := decoded.State{Characters: []decoded.Character{{CharacterID: 1074, Level: 80}, {CharacterID: 1090, Level: 80}}}
	if err := writeAccountData(dir, nte.Inventory{}, state); err != nil {
		t.Fatal(err)
	}
	workspace, err := LoadBuildWorkspace(dir, filepath.Join("..", "..", "data"), "en")
	if err != nil || len(workspace.Characters) != 2 {
		t.Fatalf("new character workspace: %+v %v", workspace, err)
	}
	for _, c := range workspace.Characters {
		if c.Name == "" || c.Build != nil {
			t.Fatalf("invalid imported character: %+v", c)
		}
	}
}
