package combatdata

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"nte-optimizer/internal/damage"
)

func akaneDisplayCoefficient(t *testing.T, c Catalog) Parameter {
	t.Helper()
	for _, ability := range c.Skills.Characters["1057"].Abilities {
		if ability.ID != "GA_Akane_Skill" {
			continue
		}
		for _, line := range ability.Lines {
			if line.ID != "1057__GA_Akane_Skill__4" {
				continue
			}
			if line.Status != "resolved" {
				t.Fatalf("display line numeric status = %q", line.Status)
			}
			for _, parameter := range line.Parameters {
				if parameter.Index == 0 {
					return parameter
				}
			}
		}
	}
	t.Fatal("missing Akane skill line 4, parameter 0")
	return Parameter{}
}

func TestPublishedSheetCoefficientsAgreeWithDamageCatalog(t *testing.T) {
	c := production(t)
	d, err := damage.ReadCatalog(filepath.Join("..", "..", "data", "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	unmatched := map[string]bool{}
	foreign := []string{}
	checked := 0
	displayOnly := 0
	for id, character := range c.Skills.Characters {
		effects := map[string]damage.TieredInstance{}
		for _, entry := range d.Characters[id].Instances {
			effects[entry.ID] = entry
		}
		for _, ability := range character.Abilities {
			for _, line := range ability.Lines {
				for _, p := range line.Parameters {
					if p.SourceKind == "display_damage_coefficient" {
						displayOnly++
						if p.DamageCalculation == nil || p.DamageCalculation.Supported || p.DamageCalculation.DamageEffectID != nil || p.DamageCalculation.Status != "display_only_no_confirmed_execution_binding" {
							t.Errorf("%s lost its display-only damage boundary", p.SourceID)
						}
						if _, exists := effects[p.SourceID]; exists {
							t.Errorf("%s display coefficient became an extra damage effect", p.SourceID)
						}
						continue
					}
					if p.SourceKind != "damage_effect" || p.Status == "unused_source_slot" {
						continue
					}
					entry, err := p.ResolveDamageEffect(id, d)
					if errors.Is(err, ErrForeignDamageEffect) {
						if entry.ID != "" {
							t.Fatal("foreign association returned an executable effect")
						}
						if id != "1074" {
							t.Fatalf("new cross-character sheet association: %s/%s: %v", id, line.ID, err)
						}
						foreign = append(foreign, fmt.Sprintf("%s:%d:%s", line.ID, p.Index, p.SourceID))
						continue
					}
					if err != nil {
						unmatched[p.SourceID] = true
						t.Errorf("%s/%s: %v", id, line.ID, err)
						continue
					}
					checked++
					if !reflect.DeepEqual(p.Values, entry.Coefficients) || p.Scaling != string(entry.Scaling) {
						t.Errorf("%s: sheet %s and damage catalog disagree on coefficients or scaling", id, line.ID)
					}
				}
			}
		}
	}
	if len(unmatched) != 0 {
		t.Fatalf("unexpected sheet effects without damage entries: %v", unmatched)
	}
	// Pin all 45 known raw-source associations by line, parameter and source ID.
	// Any added, removed or retargeted association requires another source review.
	slices.Sort(foreign)
	if len(foreign) != 45 || fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(foreign, "\n")))) != "8ee7adc30302a7e71709968ad0f1c8aa3fa3d1c08fd0121b99445f4f36c11c93" {
		t.Fatalf("review cross-character association coverage: %v", foreign)
	}
	if displayOnly != 1 {
		t.Fatalf("display-only coefficient coverage changed: %d", displayOnly)
	}
	if checked == 0 {
		t.Fatal("no sheet coefficients checked")
	}
	t.Logf("%d matching parameters; %d display-only coefficient and %d foreign effect bindings excluded", checked, displayOnly, len(foreign))
}

func TestAkaneDisplayCoefficientRemainsResolvedAndNonExecutable(t *testing.T) {
	c := production(t)
	p := akaneDisplayCoefficient(t, c)
	want := []float64{11.25, 12.15, 13.122, 14.172, 15.306, 16.53, 17.852, 19.281, 20.823, 22.489, 24.288, 26.231, 28.329}
	if p.SourceID != "GE_Player_Akane_UpgradeSkill_DisplayGE" || p.SourceKind != "display_damage_coefficient" || p.Status != "resolved" || p.Scaling != "attack" || !p.Percent {
		t.Fatalf("display-only numerical contract changed: %+v", p)
	}
	if !reflect.DeepEqual(p.Values, want) {
		t.Fatalf("display coefficient curve changed: %v", p.Values)
	}
	for i, value := range want {
		got, err := p.Resolve(i+1, 80, nil)
		if err != nil || got != value {
			t.Errorf("display coefficient at level %d: %v, %v", i+1, got, err)
		}
	}
	if p.DamageCalculation == nil || p.DamageCalculation.Supported || p.DamageCalculation.DamageEffectID != nil || p.DamageCalculation.Status != "display_only_no_confirmed_execution_binding" {
		t.Fatalf("display coefficient acquired an executable binding: %+v", p.DamageCalculation)
	}
	if p.DamageCalculation.Reason == "" || len(p.DamageCalculation.Provenance) == 0 {
		t.Fatal("display-only classification lost its explanation or source references")
	}
	d, err := damage.ReadCatalog(filepath.Join("..", "..", "data", "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, character := range d.Characters {
		for _, effect := range character.Instances {
			if effect.ID == p.SourceID {
				t.Fatal("display coefficient was published as an executable effect")
			}
		}
	}
}

func TestAkaneExecutableBaseAndConditionalCoefAddStaySeparate(t *testing.T) {
	c := production(t)
	d, err := damage.ReadCatalog(filepath.Join("..", "..", "data", "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantBase := []float64{10.125, 10.935, 11.81, 12.755, 13.775, 14.877, 16.067, 17.352, 18.741, 20.24, 21.859, 23.608, 25.496}
	var base damage.TieredInstance
	for _, effect := range d.Characters["1057"].Instances {
		if effect.ID == "GE_Player_Akane_UpgradeSkill_Damage" {
			base = effect
		}
	}
	if base.ID == "" || !reflect.DeepEqual(base.Coefficients, wantBase) {
		t.Fatalf("executable base was replaced or composed with a modifier: %v", base.Coefficients)
	}
	var modifier Modifier
	for _, entry := range c.Modifiers.Entries {
		if entry.ID == "GE_Player_Akane_UpgradeSkill_Damage__CustomApplicationModifierInfos_0_GameplayModifierInfos_0" {
			modifier = entry
		}
	}
	if modifier.ID == "" || modifier.EffectID != base.ID || modifier.PropertyID != "CoefAdd" || modifier.Operation != "EGameplayModOp::Additive" {
		t.Fatal("conditional CoefAdd lost its executable-effect binding")
	}
	wantAdd := []float64{1.125, 1.215, 1.3122, 1.4172, 1.5306, 1.653, 1.7852, 1.9281, 2.0823, 2.2489, 2.4288, 2.6231, 2.8329}
	key := func(effect string) string {
		raw, _ := json.Marshal(map[string]string{"op": "buff_active", "actor": "instigator_actor", "effect_id": effect})
		return string(raw)
	}
	for _, state := range []struct{ short, long bool }{{false, false}, {false, true}, {true, false}, {true, true}} {
		for level := 1; level <= 13; level++ {
			context := Context{Actors: map[string]Actor{"instigator_actor": {CharacterID: 1057, Levels: map[string]int{"GA_Akane_Skill": level}}}, Bindings: map[string]any{key("Buff_Akane057_Skill_short_C"): state.short, key("Buff_Akane057_Skill_Long_C"): state.long}}
			// Effect-spec wrapper level is separate from the imported skill level.
			got, err := modifier.Evaluate(context, 1)
			want := 0.0
			if state.short {
				want = wantAdd[level-1]
			}
			if err != nil || math.Abs(got-want) > 1e-10 {
				t.Errorf("level %d, short=%v long=%v: CoefAdd %v, want %v (%v)", level, state.short, state.long, got, want, err)
			}
		}
	}
}
