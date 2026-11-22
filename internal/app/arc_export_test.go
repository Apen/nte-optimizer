package app

import (
	"context"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"nte-optimizer/internal/apperrors"
	"nte-optimizer/internal/damage"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
)

func loadArcExportFixture(t *testing.T) (OptimizerService, decoded.ForkCatalog, ntelocale.Catalog) {
	t.Helper()
	service := NewOptimizerService(filepath.Join("..", "..", "data"))
	arcs, err := service.loadArcs()
	if err != nil {
		t.Fatal(err)
	}
	labels, err := service.loadLocale("en")
	if err != nil {
		t.Fatal(err)
	}
	return service, arcs, labels
}

func publishedArcParam(t *testing.T, params []decoded.ForkEffectParameter, name string) decoded.ForkEffectParameter {
	t.Helper()
	for _, param := range params {
		if param.NameID == name {
			return param
		}
	}
	t.Fatalf("missing rank parameter %s", name)
	return decoded.ForkEffectParameter{}
}

func TestPublishedRavenousBladeUsesSelectedStacksAtEachBuildRank(t *testing.T) {
	const (
		zankouID = 1036
		fork     = "fork_DemonBlade"
		trigger  = "wearer_deals_incantation_damage"
		crit     = "CritDamageBase"
	)
	service, arcs, _ := loadArcExportFixture(t)
	rankOne := decoded.Weapon{ID: decoded.NetID{Slot: 1, Serial: 1}, ForkID: fork, Level: 80, Breakthrough: 6, Star: 1}
	rankFive := rankOne
	rankFive.ID.Slot, rankFive.Star = 2, 5
	state := decoded.State{
		Characters: []decoded.Character{{CharacterID: zankouID, Level: 80, BreakthroughLevel: 6, ForkNetID: &rankOne.ID,
			Skills: []decoded.CharacterSkill{{AbilityID: "GA_Zankou_Melee", Level: 10}}}},
		Weapons: []decoded.Weapon{rankOne, rankFive},
	}
	profiles, err := service.loadCharacters()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := service.loadOptimizerCatalog()
	if err != nil {
		t.Fatal(err)
	}
	setID := catalog.sets.Definitions[profiles["zankou"].PreferredSets[0].SetID].InventorySetID
	inv := loadSanitizedInventoryFixture(t)
	for index := range inv.Modules {
		inv.Modules[index].SetID = setID
		inv.Modules[index].EquippedCharacterID = zankouID
	}
	inv.Cartridges = []nte.Cartridge{{LocalID: "synthetic-cartridge", SetID: setID, Level: 20, EquippedCharacterID: zankouID,
		MainStats: []nte.Stat{{PropertyID: crit, Value: .6, Percent: true}}}}
	service.WeightOverrides = &WeightOverrides{MainStats: []string{crit}, Weights: map[string]float64{crit: 1}}
	key := ArcConditionKey{CharacterID: zankouID, ForkID: fork, Trigger: trigger}
	for _, stacks := range []int{0, 1, 4, 7, 8} {
		t.Run(strconv.Itoa(stacks), func(t *testing.T) {
			service.WeightOverrides.ArcConditions = nil
			if stacks > 0 {
				service.WeightOverrides.ArcConditions = []ArcConditionSelection{{ArcConditionKey: key, Stacks: stacks}}
			}
			result, err := service.optimizeTunedWithArc(context.Background(), inv, "zankou", &state, true, "en", "fast", nil, nil, fork)
			if stacks > 7 {
				if clientErr := apperrors.ForClient(err); clientErr == nil || clientErr.Error() != "nte:invalid_arc_conditions" {
					t.Fatalf("expected stack validation error, got %v", err)
				}
				return
			}
			if err != nil || result.CurrentStats == nil || result.Weapon == nil || result.Weapon.Star != 5 {
				t.Fatalf("missing current rank-one or proposed rank-five build: %v", err)
			}
			rankFiveParams := arcs.Forks[fork].EffectsByStar["5"].OrderedParameters
			duration := int(publishedArcParam(t, rankFiveParams, "buff_DemonBlade_CD").Value)
			if found := slices.ContainsFunc(result.BuffEffects, func(row TeamPreviewEffect) bool {
				return row.ID == "ravenous_blade_wearer_incantation_crit_damage" && row.Trigger == trigger && row.Star == 5 && row.Duration == duration
			}); found != (stacks > 0) {
				t.Errorf("unexpected Ravenous Blade preview row for %d stacks: %+v", stacks, result.BuffEffects)
			}
			for _, build := range []struct {
				rank  string
				stats optimizer.StatSummary
			}{{"1", *result.CurrentStats}, {"5", result.Stats}} {
				params := arcs.Forks[fork].EffectsByStar[build.rank].OrderedParameters
				perStack := publishedArcParam(t, params, "buff_DemonBlade_CritDamageUp").Value
				permanentRate := publishedArcParam(t, params, "buff_DemonBlade_Crit").Value
				want := build.stats.Derived[crit] + float64(stacks)*perStack
				if build.stats.Sources["weapon_conditional"][crit] != 0 || build.stats.Sources["weapon_permanent"]["CritBase"] != permanentRate {
					t.Errorf("rank %s: unexpected Arc stat sources: %+v", build.rank, build.stats.Sources)
				}
				for _, stats := range []map[string]float64{build.stats.DerivedConditional, build.stats.RankingStats()} {
					if math.Abs(stats[crit]-want) > 1e-9 {
						t.Errorf("rank %s: CRIT DMG = %g, want %g", build.rank, stats[crit], want)
					}
				}
			}
		})
	}
}

func TestPublishedArcTriggersKeepTheirExactRecipientsAndStats(t *testing.T) {
	service, arcs, labels := loadArcExportFixture(t)
	layout := datafiles.New(service.DataDir)
	compatibility, err := decoded.LoadArcCompatibilityCatalog(layout.DecodeCatalog("characters.json"), layout.Arcs())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, fork, trigger string
		wearer, recipient   int
		properties          map[string]string
	}{
		{"alternate wearer heal", "fork_Door", "healing", 1021, 1021, map[string]string{"the_wrong_gate_wearer_anima_damage": "DamageUpNatureBase"}},
		{"ally heal", "fork_Door", "healing", 1021, 1036, map[string]string{"the_wrong_gate_allies_damage": "DamageUpGeneralBase"}},
		{"wearer Ultimate", "fork_twinbirds", "ultimate", 1042, 1042, map[string]string{
			"crime_and_punishment_team_crit_damage": "CritDamageBase", "crime_and_punishment_wearer_crit_damage": "CritDamageBase",
		}},
		{"ally Ultimate", "fork_twinbirds", "ultimate", 1042, 1036, map[string]string{"crime_and_punishment_team_crit_damage": "CritDamageBase"}},
		{"Redirect or Ultimate", "fork_GoldWool", "redirect_skill_or_ultimate", 1033, 1033, map[string]string{"whats_desired_wearer_crit_damage": "CritDamageBase"}},
		{"Ultimate Cosmos and DEF Ignore", "fork_LunarPhase", "ultimate", 1076, 1076, map[string]string{
			"blushing_mirage_wearer_cosmos_damage": "DamageUpCosmosBase", "blushing_mirage_wearer_defense_ignore": "DefenseIgnoreBase",
		}},
		{"Redirect Cycle Intensity", "fork_worldrain", "redirect_skill", 1046, 1046, map[string]string{"rain_that_shook_the_world_wearer_cycle_intensity": "MagBase"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !slices.Contains(compatibility.CompatibleCharacterIDs(tc.fork), tc.wearer) {
				t.Fatalf("character %d cannot equip %s", tc.wearer, tc.fork)
			}
			for _, rank := range []int{1, 5} {
				weapon := decoded.Weapon{ForkID: tc.fork, Star: rank}
				activeArc := &weapon
				var team *teamScenario
				if tc.wearer != tc.recipient {
					activeArc = nil
					team = &teamScenario{
						allies: []TeamAlly{{CharacterID: tc.wearer}}, gear: map[int]teamGear{tc.wearer: {arc: &weapon}},
					}
				}
				effects, err := service.collectArcScenarioEffects(arcs, tc.recipient, activeArc, team)
				if err != nil {
					t.Fatal(err)
				}
				sources := map[string][]nte.Stat{}
				if _, err := applyArcScenario(arcs, tc.recipient, effects, nil, labels, sources); err != nil || len(sources[optimizer.ArcScenarioSource]) != 0 {
					t.Fatalf("rank %d: unconfirmed event added stats: %v (%v)", rank, sources, err)
				}
				key := ArcConditionKey{CharacterID: tc.wearer, ForkID: tc.fork, Trigger: tc.trigger}
				selected, err := arcConditionSelections(effects, []ArcConditionSelection{{ArcConditionKey: key, Stacks: 1}}, labels)
				if err != nil {
					t.Fatal(err)
				}
				result, err := applyArcScenario(arcs, tc.recipient, effects, selected, labels, sources)
				if err != nil {
					t.Fatal(err)
				}
				if len(sources[optimizer.ArcScenarioSource]) != len(tc.properties) {
					t.Fatalf("rank %d: unexpected scenario stats %v", rank, sources)
				}
				for id, property := range tc.properties {
					definition := arcs.Forks[tc.fork]
					index := slices.IndexFunc(definition.ConditionalEffects, func(effect decoded.ForkConditionalEffect) bool { return effect.ID == id })
					if index < 0 {
						t.Fatalf("missing published effect %s", id)
					}
					effect := definition.ConditionalEffects[index]
					params := definition.EffectsByStar[strconv.Itoa(rank)].OrderedParameters
					magnitude := publishedArcParam(t, params, effect.Parameter.NameID)
					duration := publishedArcParam(t, params, effect.DurationParameter)
					wantStat := nte.Stat{PropertyID: property, Value: magnitude.Value, Percent: magnitude.IsPercent}
					if !slices.ContainsFunc(result.rows, func(row TeamPreviewEffect) bool {
						return row.ID == id && row.Star == rank && row.CharacterID == tc.wearer && row.Trigger == tc.trigger &&
							row.Duration == int(duration.Value) && slices.Equal(row.ProjectedStats, []nte.Stat{wantStat})
					}) || !slices.Contains(sources[optimizer.ArcScenarioSource], wantStat) {
						t.Errorf("rank %d: missing %s projection %+v for recipient %d", rank, id, wantStat, tc.recipient)
					}
				}
			}
		})
	}

	weapon := decoded.Weapon{ForkID: "fork_twinbirds", Star: 1}
	_, permanent, conditional, _ := arcs.Stats(weapon)
	for _, property := range []string{"MagBase", "DamageUpPsycheBase"} {
		if !slices.ContainsFunc(permanent, func(stat nte.Stat) bool { return stat.PropertyID == property && stat.Value > 0 }) {
			t.Errorf("Crime and Punishment is missing permanent %s", property)
		}
	}
	if len(conditional) != 0 {
		t.Errorf("Crime and Punishment duplicated the event bonuses in legacy stats: %v", conditional)
	}
}

func TestPublishedGoodBoyArcConfirmsTheBaseOnce(t *testing.T) {
	const fork = "fork_mofeikesi"
	const first, second, recipient = 1003, 1008, 1036
	service, arcs, labels := loadArcExportFixture(t)
	params := arcs.Forks[fork].EffectsByStar["1"].OrderedParameters
	base := nte.Stat{PropertyID: "AtkUp", Value: publishedArcParam(t, params, "buff_mofeikesi_Atk").Value, Percent: true}
	control := nte.Stat{PropertyID: "AtkUp", Value: publishedArcParam(t, params, "buff_mofeikesi_Up").Value, Percent: true}
	both := []nte.Stat{base, control}
	selection := func(wearer int, trigger string) ArcConditionSelection {
		return ArcConditionSelection{ArcConditionKey: ArcConditionKey{CharacterID: wearer, ForkID: fork, Trigger: trigger}, Stacks: 1}
	}
	ultimate, controlled := selection(first, "ultimate"), selection(first, "ultimate_control")
	for _, tc := range []struct {
		name          string
		confirmations []ArcConditionSelection
		secondRank    int
		want          []nte.Stat
	}{
		{"no confirmation", nil, 1, []nte.Stat{}},
		{"Ultimate", []ArcConditionSelection{ultimate}, 1, []nte.Stat{base}},
		{"Ultimate control implies Ultimate", []ArcConditionSelection{controlled}, 1, both},
		{"both confirmations", []ArcConditionSelection{ultimate, controlled}, 1, both},
		{"identical copies", []ArcConditionSelection{controlled, selection(second, "ultimate_control")}, 1, both},
		{"mixed ranks", []ArcConditionSelection{controlled, selection(second, "ultimate_control")}, 5, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			team := &teamScenario{
				allies: []TeamAlly{{CharacterID: first}, {CharacterID: second}},
				gear: map[int]teamGear{
					first:  {arc: &decoded.Weapon{ForkID: fork, Star: 1}},
					second: {arc: &decoded.Weapon{ForkID: fork, Star: tc.secondRank}},
				},
			}
			effects, err := service.collectArcScenarioEffects(arcs, recipient, nil, team)
			if err != nil {
				t.Fatal(err)
			}
			selected, err := arcConditionSelections(effects, tc.confirmations, labels)
			if err != nil {
				t.Fatal(err)
			}
			sources := map[string][]nte.Stat{}
			_, err = applyArcScenario(arcs, recipient, effects, selected, labels, sources)
			if tc.want == nil {
				if err == nil || !strings.Contains(err.Error(), "unresolved combination of multiple sources") {
					t.Fatalf("expected unresolved mixed ranks, got %v", err)
				}
				return
			}
			if got := sources[optimizer.ArcScenarioSource]; err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("team bonuses = %+v, want %+v (%v)", got, tc.want, err)
			}
		})
	}
}

func TestPublishedDottedHorizonStacksApplyToTaggedWearerHits(t *testing.T) {
	const summon = "State.Damage.Summon"
	const nanally, exe = 1010, 1074
	const trigger = "wearer_deals_chaos_damage"
	service, arcs, labels := loadArcExportFixture(t)
	weapon := decoded.Weapon{ForkID: "fork_exe", Star: 5}
	effects, err := service.collectArcScenarioEffects(arcs, nanally, &weapon, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := ArcConditionKey{CharacterID: nanally, ForkID: weapon.ForkID, Trigger: trigger}
	selected, err := arcConditionSelections(effects, []ArcConditionSelection{{ArcConditionKey: key, Stacks: 10}}, labels)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]nte.Stat{}
	scenario, err := applyArcScenario(arcs, nanally, effects, selected, labels, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenario.tagged) != 1 || len(sources[optimizer.ArcScenarioSource]) != 0 {
		t.Fatalf("expected one hit-scoped bonus and no panel bonus, got %+v, %+v", scenario, sources)
	}

	damageCatalog, err := damage.ReadCatalog(filepath.Join(service.DataDir, "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	wearer := &decoded.Character{CharacterID: nanally, Level: 80}
	tagged := map[string]bool{}
	for _, hit := range damageCatalog.Characters["1010"].Instances {
		wearer.Skills = append(wearer.Skills, decoded.CharacterSkill{AbilityID: hit.AbilityID, Level: 10})
		tagged[hit.ID] = slices.Contains(hit.DamageTags, summon)
	}
	stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkBase": 100, "AtkFinal": 100}}
	plain := damageAnalysisWithArcBuffs(wearer, stats, nil, damageCatalog, nil, nil)
	buffed := damageAnalysisWithArcBuffs(wearer, stats, nil, damageCatalog, nil, scenario.tagged)
	plainByID := make(map[string]float64, len(plain.Build))
	for _, hit := range plain.Build {
		plainByID[hit.InstanceID] = hit.Factors.DamageZone
	}
	var matching, other int
	for _, hit := range buffed.Build {
		want, ok := plainByID[hit.InstanceID]
		if !ok {
			t.Fatalf("buffed hit %s has no baseline", hit.InstanceID)
		}
		if tagged[hit.InstanceID] {
			want += scenario.tagged[0].stat.Value
			matching++
		} else {
			other++
		}
		if math.Abs(hit.Factors.DamageZone-want) > 1e-9 {
			t.Errorf("%s damage multiplier = %g, want %g", hit.InstanceID, hit.Factors.DamageZone, want)
		}
	}
	if matching == 0 || other == 0 {
		t.Fatalf("fixture needs both eligible and ineligible hits, got %d and %d", matching, other)
	}

	team := &teamScenario{allies: []TeamAlly{{CharacterID: exe}}, gear: map[int]teamGear{exe: {arc: &weapon}}}
	effects, err = service.collectArcScenarioEffects(arcs, nanally, nil, team)
	if err != nil {
		t.Fatal(err)
	}
	key.CharacterID = exe
	if _, err := arcConditionSelections(effects, []ArcConditionSelection{{ArcConditionKey: key, Stacks: 10}}, labels); err == nil {
		t.Fatal("accepted a wearer's summon bonus for a teammate")
	}
}

func TestPublishedPrismRedirectBuffAppliesOnceToMatchingAkaneHits(t *testing.T) {
	const akane = 1057
	const melee, branch = "State.Damage.Melee", "State.Damage.Branch"
	service, arcs, labels := loadArcExportFixture(t)
	damageCatalog, err := damage.ReadCatalog(filepath.Join(service.DataDir, "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	character := &decoded.Character{CharacterID: akane, Level: 80}
	hits := damageCatalog.Characters["1057"].Instances
	for _, hit := range hits {
		character.Skills = append(character.Skills, decoded.CharacterSkill{AbilityID: hit.AbilityID, Level: 10})
	}
	published := arcs.Forks["fork_prism"]
	for _, star := range []int{1, 5} {
		rank := strconv.Itoa(star)
		t.Run(rank, func(t *testing.T) {
			weapon := decoded.Weapon{ForkID: "fork_prism", Star: star}
			effects, err := service.collectArcScenarioEffects(arcs, akane, &weapon, nil)
			if err != nil {
				t.Fatal(err)
			}
			i := slices.IndexFunc(effects, func(entry arcScenarioEffect) bool {
				return entry.effect.ID == "far_side_of_the_moon_wearer_basic_attack_damage"
			})
			if i < 0 {
				t.Fatal("published Prism effect is unavailable")
			}
			effect := effects[i].effect
			if effect.Scope != "wearer_filtered_damage" || effect.Trigger != "redirect_skill" ||
				len(effect.AnyDamageTags) != 2 || !slices.Contains(effect.AnyDamageTags, melee) || !slices.Contains(effect.AnyDamageTags, branch) {
				t.Fatalf("unexpected exported Prism rule %+v", effect)
			}
			paramIndex := slices.IndexFunc(published.EffectsByStar[rank].OrderedParameters, func(p decoded.ForkEffectParameter) bool {
				return p.NameID == effect.Parameter.NameID
			})
			if paramIndex < 0 {
				t.Fatal("missing Prism rank parameter")
			}
			bonus := published.EffectsByStar[rank].OrderedParameters[paramIndex].Value
			sources := map[string][]nte.Stat{}
			inactive, err := applyArcScenario(arcs, akane, effects, nil, labels, sources)
			if err != nil || slices.ContainsFunc(inactive.tagged, func(buff taggedArcBuff) bool { return !buff.alwaysActive }) {
				t.Fatalf("unconfirmed Prism effect was applied: %+v, %v", inactive, err)
			}
			selected, err := arcConditionSelections(effects, []ArcConditionSelection{{ArcConditionKey: effects[i].key, Stacks: 1}}, labels)
			if err != nil {
				t.Fatal(err)
			}
			active, err := applyArcScenario(arcs, akane, effects, selected, labels, sources)
			if err != nil || len(active.tagged) != len(inactive.tagged)+1 || len(sources[optimizer.ArcScenarioSource]) != 0 {
				t.Fatalf("expected only one scoped Prism effect: %+v, %+v, %v", active, sources, err)
			}
			stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkBase": 100, "AtkFinal": 100}}
			plain := damageAnalysisWithArcBuffs(character, stats, nil, damageCatalog, nil, inactive.tagged)
			buffed := damageAnalysisWithArcBuffs(character, stats, nil, damageCatalog, nil, active.tagged)
			deltas := make(map[string]float64, len(plain.Build))
			for _, hit := range plain.Build {
				deltas[hit.InstanceID] = -hit.Factors.DamageZone
			}
			for _, hit := range buffed.Build {
				deltas[hit.InstanceID] += hit.Factors.DamageZone
			}
			var eligible, ineligible int
			for _, hit := range hits {
				want := 0.0
				for _, tag := range hit.DamageTags {
					if tag == melee || strings.HasPrefix(tag, melee+".") || tag == branch || strings.HasPrefix(tag, branch+".") {
						want = bonus
						break
					}
				}
				delta, exists := deltas[hit.ID]
				if !exists || math.Abs(delta-want) > 1e-9 {
					t.Errorf("%s damage modifier = %g, want %g", hit.ID, delta, want)
				}
				if want > 0 {
					eligible++
				} else {
					ineligible++
				}
			}
			for _, id := range []string{"GE_Player_Akane_Branch_Damage", "GE_Player_Akane_Airattack_Damage"} {
				if math.Abs(deltas[id]-bonus) > 1e-9 {
					t.Errorf("%s matches both alternatives and should receive one bonus", id)
				}
			}
			if eligible == 0 || ineligible == 0 {
				t.Fatalf("missing coverage for eligible or ineligible Akane hits: %d, %d", eligible, ineligible)
			}
		})
	}
}

func TestPublishedEquippedArcBonusesAffectOnlyMatchingActionDamage(t *testing.T) {
	const redirect, ultimate = "State.Damage.Skill", "State.Damage.UltraSkill"
	service, arcs, labels := loadArcExportFixture(t)
	damageCatalog, err := damage.ReadCatalog(filepath.Join(service.DataDir, "game", "combat", "damage.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fork, property string
		characterID    int
		tagsByRule     map[string]string
	}{
		{"fork_prism", "DamageUpGeneralBase", 1057, map[string]string{
			"far_side_of_the_moon_wearer_redirect_damage": redirect,
		}},
		{"fork_worldrain", "DamageUpCosmosBase", 1046, map[string]string{
			"rain_that_shook_the_world_wearer_redirect_cosmos_damage": redirect,
			"rain_that_shook_the_world_wearer_ultimate_cosmos_damage": ultimate,
		}},
		{"fork_GoldRecord", "CritDamageBase", 1072, map[string]string{
			"voice_of_the_voyager_wearer_support_crit_damage":  "State.Damage.QTE",
			"voice_of_the_voyager_wearer_xiaozhen_crit_damage": "Ability.Player.Linko.XiaozhenDamage",
		}},
	} {
		t.Run(tc.fork, func(t *testing.T) {
			weapon := decoded.Weapon{ForkID: tc.fork, Star: 3}
			effects, err := service.collectArcScenarioEffects(arcs, tc.characterID, &weapon, nil)
			if err != nil {
				t.Fatal(err)
			}
			sources := map[string][]nte.Stat{}
			scenario, err := applyArcScenario(arcs, tc.characterID, effects, nil, labels, sources)
			if err != nil || len(scenario.tagged) != len(tc.tagsByRule) || len(sources[optimizer.ArcScenarioSource]) != 0 {
				t.Fatalf("expected only the equipped hit modifiers: %+v, %+v, %v", scenario, sources, err)
			}
			bonuses := map[string]float64{}
			for _, row := range scenario.rows {
				tag := tc.tagsByRule[row.ID]
				if tag == "" || row.Trigger != "equipped" || len(row.ProjectedStats) != 1 || row.ProjectedStats[0].PropertyID != tc.property {
					t.Fatalf("unexpected equipped action modifier: %+v", row)
				}
				bonuses[tag] = row.ProjectedStats[0].Value
			}
			if len(bonuses) != len(tc.tagsByRule) {
				t.Fatalf("missing equipped action modifiers: %v", bonuses)
			}
			character := &decoded.Character{CharacterID: tc.characterID, Level: 80}
			wantByHit := map[string]float64{}
			var eligible, ineligible int
			for _, hit := range damageCatalog.Characters[strconv.Itoa(tc.characterID)].Instances {
				character.Skills = append(character.Skills, decoded.CharacterSkill{AbilityID: hit.AbilityID, Level: 10})
				for nativeTag, bonus := range bonuses {
					if (tc.property != "DamageUpCosmosBase" || hit.Element == "DAMAGE_TYPE_COSMOS") && slices.ContainsFunc(hit.DamageTags, func(tag string) bool {
						return tag == nativeTag || strings.HasPrefix(tag, nativeTag+".")
					}) {
						wantByHit[hit.ID] += bonus
					}
				}
			}
			batch := damageBatch{character: character, catalog: damageCatalog}
			stats := optimizer.StatSummary{DerivedConditional: map[string]float64{"AtkBase": 100, "AtkFinal": 100}}
			modifier := func(hit damage.Result) float64 {
				if tc.property == "CritDamageBase" {
					return hit.Factors.CritDamage
				}
				return hit.Factors.DamageZone
			}
			plainByID := map[string]float64{}
			for _, hit := range batch.calculate(stats, false, nil).Build {
				plainByID[hit.InstanceID] = modifier(hit)
			}
			for _, hit := range batch.calculate(stats, false, scenario.tagged).Build {
				want := wantByHit[hit.InstanceID]
				delta := modifier(hit) - plainByID[hit.InstanceID]
				if math.Abs(delta-want) > 1e-9 {
					t.Errorf("%s equipped modifier = %g, want %g", hit.InstanceID, delta, want)
				}
				if want > 0 {
					eligible++
				} else {
					ineligible++
				}
			}
			if eligible == 0 || ineligible == 0 {
				t.Fatalf("missing eligible or ineligible hits: %d, %d", eligible, ineligible)
			}
		})
	}
}

func TestPublishedLastRoseSharesChaosThornStacksBetweenRedirectAndDOT(t *testing.T) {
	const lacrimosa = 1004
	service, arcs, labels := loadArcExportFixture(t)
	weapon := decoded.Weapon{ForkID: "fork_Rose", Star: 5}
	effects, err := service.collectArcScenarioEffects(arcs, lacrimosa, &weapon, nil)
	if err != nil {
		t.Fatal(err)
	}
	redirect := ArcConditionKey{CharacterID: lacrimosa, ForkID: weapon.ForkID, Trigger: "redirect_skill"}
	dot := ArcConditionKey{CharacterID: lacrimosa, ForkID: weapon.ForkID, Trigger: "wearer_deals_damage_over_time"}
	options := arcConditionOptions(effects, labels)
	redirectIndex := slices.IndexFunc(options, func(option ArcConditionOption) bool {
		return option.ArcConditionKey == redirect && option.MaxStacks == 1
	})
	dotIndex := slices.IndexFunc(options, func(option ArcConditionOption) bool {
		return option.ArcConditionKey == dot && option.MaxStacks == 10
	})
	if len(options) != 2 || redirectIndex < 0 || dotIndex < 0 {
		t.Fatalf("expected one Redirect checkbox and a DoT stack selector: %+v", options)
	}
	params := arcs.Forks[weapon.ForkID].EffectsByStar["5"].OrderedParameters
	perStack := publishedArcParam(t, params, "buff_Rose_CritDamageUp").Value
	duration := int(publishedArcParam(t, params, "buff_Rose_CD").Value)
	dotSelection := ArcConditionSelection{ArcConditionKey: dot, Stacks: 4}
	redirectSelection := ArcConditionSelection{ArcConditionKey: redirect, Stacks: 1}
	for _, tc := range []struct {
		name    string
		input   []ArcConditionSelection
		want    int
		trigger string
	}{
		{"inactive", nil, 0, ""},
		{"DoT stacks", []ArcConditionSelection{dotSelection}, 4, dot.Trigger},
		{"Redirect grant", []ArcConditionSelection{redirectSelection}, 10, redirect.Trigger},
		{"shared pool", []ArcConditionSelection{dotSelection, redirectSelection}, 10, redirect.Trigger},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := arcConditionSelections(effects, tc.input, labels)
			if err != nil {
				t.Fatal(err)
			}
			sources := map[string][]nte.Stat{}
			result, err := applyArcScenario(arcs, lacrimosa, effects, selected, labels, sources)
			if err != nil {
				t.Fatal(err)
			}
			stats := sources[optimizer.ArcScenarioSource]
			if tc.want == 0 {
				if len(stats) != 0 || len(result.rows) != 0 {
					t.Fatalf("unconfirmed stacking bonus entered the build: %+v", result)
				}
				return
			}
			if len(stats) != 1 || stats[0].PropertyID != "CritDamageBase" || math.Abs(stats[0].Value-float64(tc.want)*perStack) > 1e-9 {
				t.Fatalf("expected %d stacks of wearer CRIT DMG, got %+v", tc.want, stats)
			}
			if len(result.rows) != 1 || result.rows[0].Trigger != tc.trigger || result.rows[0].Duration != duration {
				t.Fatalf("expected single %s projection for %d s, got %+v", tc.trigger, duration, result.rows)
			}
		})
	}
}

func TestPublishedReadyReadySharesItsStackSelectionAcrossDamageFilters(t *testing.T) {
	const nanally, fork = 1010, "fork_TigerTally"
	service, arcs, labels := loadArcExportFixture(t)
	character := &decoded.Character{CharacterID: nanally, Level: 80}
	batch := service.prepareDamageBatch(character, nil, "en")
	for _, hit := range batch.catalog.Characters["1010"].Instances {
		character.Skills = append(character.Skills, decoded.CharacterSkill{AbilityID: hit.AbilityID, Level: 10})
	}
	panel := map[string]float64{"AtkBase": 100, "AtkFinal": 100}
	stats := optimizer.StatSummary{Derived: panel, DerivedConditional: panel}
	key := ArcConditionKey{CharacterID: nanally, ForkID: fork, Trigger: "redirect_skill_or_ultimate"}
	zonesByID := func(analysis *DamageAnalysis) map[string]float64 {
		zones := make(map[string]float64, len(analysis.Build))
		for _, hit := range analysis.Build {
			zones[hit.InstanceID] = hit.Factors.DamageZone
		}
		return zones
	}
	baseline := zonesByID(batch.calculate(stats, false, nil))
	for _, rank := range []int{1, 5} {
		weapon := decoded.Weapon{ForkID: fork, Star: rank}
		effects, err := service.collectArcScenarioEffects(arcs, nanally, &weapon, nil)
		if err != nil {
			t.Fatal(err)
		}
		var options []ArcConditionOption
		for _, option := range arcConditionOptions(effects, labels) {
			if option.ArcConditionKey == key {
				options = append(options, option)
			}
		}
		if len(options) != 1 || options[0].MaxStacks != 2 {
			t.Fatalf("rank %d: expected one shared two-stack option, got %+v", rank, options)
		}
		if _, err := arcConditionSelections(effects, []ArcConditionSelection{{ArcConditionKey: key, Stacks: 3}}, labels); err == nil {
			t.Fatalf("rank %d: accepted more than two Ready-Ready stacks", rank)
		}
		params := arcs.Forks[fork].EffectsByStar[strconv.Itoa(rank)].OrderedParameters
		perStack := publishedArcParam(t, params, "buff_TigerTally_NormalUp").Value
		for _, stacks := range []int{0, 1, 2} {
			var input []ArcConditionSelection
			if stacks > 0 {
				input = []ArcConditionSelection{{ArcConditionKey: key, Stacks: stacks}}
			}
			selected, err := arcConditionSelections(effects, input, labels)
			if err != nil {
				t.Fatal(err)
			}
			sources := map[string][]nte.Stat{}
			scenario, err := applyArcScenario(arcs, nanally, effects, selected, labels, sources)
			if err != nil || len(sources[optimizer.ArcScenarioSource]) != 0 {
				t.Fatalf("rank %d: expected hit-specific bonuses only, got %v (%v)", rank, sources, err)
			}
			plain := zonesByID(batch.calculate(stats, false, scenario.tagged))
			buffed := zonesByID(batch.calculate(stats, true, scenario.tagged))
			for id, eligible := range map[string]bool{
				"GE_Player_Nanally_Melee1_Damage":             true,
				"GE_Player_Nanally_PerfectEvadeAttack_Damage": true,
				"GE_Player_Nanally_Skill1_Damage":             false,
			} {
				want, ok := baseline[id]
				if !ok {
					t.Fatalf("missing test hit %s", id)
				}
				if plain[id] != want {
					t.Errorf("rank %d: hit %s gained an unconfirmed panel bonus", rank, id)
				}
				if eligible {
					want += float64(stacks) * perStack
				}
				if got := buffed[id]; math.Abs(got-want) > 1e-9 {
					t.Errorf("rank %d, %d stacks: %s damage multiplier = %g, want %g", rank, stacks, id, got, want)
				}
			}
		}
	}
}
