package app

import (
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"nte-optimizer/internal/combatdata"
	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/optimizer"
)

// Exercise the same batch that supplies the Damage tab, rather than a second
// skill-card calculation. Synthetic levels validate published bindings, not
// their equivalence to unobserved native execution.
func TestDamageTabConsumesPublishedInputsAcrossRoster(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	numerical, err := combatdata.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := damage.ReadCatalog(filepath.Join(dataDir, "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewOptimizerService(dataDir)
	panel := map[string]float64{"AtkFinal": 1700, "HPFinal": 23771, "DefFinal": 734, "CritBase": .67, "CritDamageBase": 1.8, "DamageUpGeneralBase": .2, "MagBase": 202}
	stats := optimizer.StatSummary{Derived: panel}
	covered, unresolved := 0, 0
	missingBindings := map[string]bool{}
	for id, characterCatalog := range catalog.Characters {
		characterID, err := strconv.Atoi(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, level := range []int{1, 8, 13} {
			levels := map[string]int{}
			for _, ability := range numerical.Skills.Characters[id].Abilities {
				if ability.Category == "active" {
					levels[ability.ID] = level
				}
			}
			for _, source := range characterCatalog.Rules.SkillLevelSources {
				levels[source] = level
			}
			character := &decoded.Character{CharacterID: characterID, Level: 80}
			for ability, level := range levels {
				character.Skills = append(character.Skills, decoded.CharacterSkill{AbilityID: ability, Level: level})
			}
			analysis := service.prepareDamageBatch(character, nil, "en").evaluate(stats)
			impacts := map[string]damage.Result{}
			for _, result := range analysis.Build {
				if _, exists := impacts[result.InstanceID]; exists {
					t.Fatalf("duplicate damage effect %s", result.InstanceID)
				}
				impacts[result.InstanceID] = result
			}
			for _, entry := range characterCatalog.Instances {
				skillLevel := levels[entry.AbilityID]
				if skillLevel == 0 {
					skillLevel = levels[characterCatalog.Rules.SkillLevelSources[entry.AbilityID]]
				}
				if skillLevel == 0 {
					if level == 1 {
						unresolved++
						missingBindings[entry.ID] = true
					}
					if _, exists := impacts[entry.ID]; exists {
						t.Fatalf("%s acquired an invented level", entry.ID)
					}
					continue
				}
				if level == 1 {
					covered++
				}
				impact, exists := impacts[entry.ID]
				if !exists {
					t.Fatalf("%s level %d absent from Damage output", entry.ID, skillLevel)
				}
				if skillLevel > len(entry.Coefficients) {
					t.Fatalf("%s missing coefficient at level %d", entry.ID, skillLevel)
				}
				if impact.Factors.Coefficient != entry.Coefficients[skillLevel-1] {
					t.Errorf("%s selected the wrong coefficient at level %d", entry.ID, skillLevel)
				}
				scaling := map[damage.ScalingStat]float64{damage.ScalingAttack: panel["AtkFinal"], damage.ScalingHP: panel["HPFinal"], damage.ScalingDefense: panel["DefFinal"]}
				if expected, ok := scaling[entry.Scaling]; !ok || impact.Factors.ScalingStat != expected {
					t.Errorf("%s uses the wrong %s scaling value", entry.ID, entry.Scaling)
				}
				if impact.Hits != max(1, entry.Hits) || math.Abs(impact.Total-impact.Expected*float64(impact.Hits)) > 1e-8 {
					t.Errorf("%s repeated hits were lost or counted twice", entry.ID)
				}
				if entry.FixedCritRate != nil && impact.Factors.CritRate != *entry.FixedCritRate {
					t.Errorf("%s fixed critical behavior changed", entry.ID)
				}
				if impact.OmitFromActionPreview != entry.OmitFromActionPreview || !reflect.DeepEqual(impact.ActionCondition, entry.ActionCondition) || impact.ActionID != entry.ActionID {
					t.Errorf("%s lost its action split, exclusion or condition", entry.ID)
				}
			}
		}
	}
	if covered == 0 {
		t.Fatal("no damage effects audited")
	}
	wantMissing := map[string]bool{
		"GE_Player_Lacrimosa_AnHunZhouTwo_Damage":   true,
		"GE_Player_Skia_GuangXiangFive_Damage":      true,
		"GE_Player_Hathor_GuangXiangFive_Damage":    true,
		"GE_Player_Daffodill_ExtraUnbalance_Damage": true,
	}
	if !reflect.DeepEqual(missingBindings, wantMissing) {
		t.Fatalf("review changed level-binding coverage: %v", missingBindings)
	}
	t.Logf("Damage output audited at levels 1, 8 and 13: %d effects with published level bindings; %d without bindings", covered, unresolved)
}

func TestAkaneDamageDoesNotSubstituteDisplayCoefficientOrActivateCoefAdd(t *testing.T) {
	catalog, err := damage.ReadCatalog(filepath.Join("..", "..", "data", "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	var base damage.TieredInstance
	for _, effect := range catalog.Characters["1057"].Instances {
		if effect.ID == "GE_Player_Akane_UpgradeSkill_Damage" {
			base = effect
		}
	}
	if len(base.Coefficients) != 13 {
		t.Fatal("missing executable Akane base curve")
	}
	stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkFinal": 1700, "CritBase": .67, "CritDamageBase": 1.8}}
	for level := 1; level <= 13; level++ {
		character := &decoded.Character{CharacterID: 1057, Level: 80, AwakenLevel: 6, Skills: []decoded.CharacterSkill{{AbilityID: "GA_Akane_Skill", Level: level}}}
		analysis := damageAnalysisFromCatalog(character, stats, nil, catalog, nil)
		found := 0
		for _, impact := range analysis.Build {
			if impact.InstanceID == "GE_Player_Akane_UpgradeSkill_DisplayGE" {
				t.Fatal("display-only row entered final damage output")
			}
			if impact.InstanceID != base.ID {
				continue
			}
			found++
			if impact.Factors.Coefficient != base.Coefficients[level-1] {
				t.Fatalf("level %d: executable base replaced or CoefAdd automatically applied: %v", level, impact.Factors.Coefficient)
			}
		}
		if found != 1 {
			t.Fatalf("level %d: executable effect count %d, want exactly one", level, found)
		}
	}
}
