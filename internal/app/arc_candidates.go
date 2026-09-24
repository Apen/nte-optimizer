package app

import (
	"fmt"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

const NoArcForkSelection = "__none__"

type arcCandidateSet struct {
	option     *optimizer.ArcOption
	weapons    map[string]decoded.Weapon
	notes      map[string]string
	additional map[string]map[string][]nte.Stat
}

func (s OptimizerService) prepareArcSelection(state *decoded.State, profile scoring.Character, current *decoded.Weapon, currentAdditional map[string][]nte.Stat, selectedForkID, language string, scoreScale float64, refs scoring.References) (arcCandidateSet, error) {
	base := cloneStatSources(currentAdditional)
	delete(base, "weapon")
	delete(base, "weapon_permanent")
	delete(base, "weapon_conditional")
	result := arcCandidateSet{
		weapons:    map[string]decoded.Weapon{},
		notes:      map[string]string{},
		additional: map[string]map[string][]nte.Stat{"": base},
	}
	if selectedForkID == NoArcForkSelection {
		return result, nil
	}

	explicitSelection := selectedForkID != ""
	if selectedForkID == "" && current != nil {
		selectedForkID = current.ForkID
	}
	if selectedForkID == "" {
		return result, nil
	}
	if state == nil {
		if explicitSelection {
			return arcCandidateSet{}, fmt.Errorf("selected Arc %q is not present in the imported account data", selectedForkID)
		}
		return result, nil
	}

	layout := datafiles.New(s.DataDir)
	compatibility, err := decoded.LoadArcCompatibilityCatalog(layout.DecodeCatalog("characters.json"), layout.Arcs())
	if err != nil {
		return arcCandidateSet{}, err
	}
	compatible := false
	for _, characterID := range compatibility.CompatibleCharacterIDs(selectedForkID) {
		if characterID == profile.CharacterID {
			compatible = true
			break
		}
	}
	if !compatible {
		if explicitSelection {
			return arcCandidateSet{}, fmt.Errorf("Arc %q is not compatible with character %d", selectedForkID, profile.CharacterID)
		}
		return result, nil
	}

	var weapon *decoded.Weapon
	for index := range state.Weapons {
		candidate := &state.Weapons[index]
		if candidate.ForkID != selectedForkID || (candidate.ID.Slot == 0 && candidate.ID.Serial == 0) {
			continue
		}
		if weapon == nil || betterArcCopy(*candidate, *weapon) {
			weapon = candidate
		}
	}
	if weapon == nil {
		if explicitSelection {
			return arcCandidateSet{}, fmt.Errorf("Arc %q is not owned in the imported account data", selectedForkID)
		}
		return result, nil
	}

	forkCatalog, err := decoded.LoadForkCatalog(layout.Arcs())
	if err != nil {
		return arcCandidateSet{}, fmt.Errorf("load Arc stat catalog: %w", err)
	}
	localizedNames, err := ntelocale.Load(s.DataDir, language)
	if err != nil {
		return arcCandidateSet{}, fmt.Errorf("load Arc names: %w", err)
	}
	panel, permanent, conditional, note := forkCatalog.Stats(*weapon)
	additional := cloneStatSources(base)
	additional["weapon"] = panel
	additional["weapon_permanent"] = permanent
	additional["weapon_conditional"] = conditional
	allAlwaysOn := append(append([]nte.Stat(nil), panel...), permanent...)
	mainScore := scoring.ScoreStatsDetailed(allAlwaysOn, nil, profile, refs).Total
	subScore := scoring.ScoreStatsDetailed(nil, allAlwaysOn, profile, refs).Total
	weapon.Name = cleanArcName(localizedNames.ItemName(weapon.ForkID, weapon.Name))
	id := weapon.ID.Key()
	result.option = &optimizer.ArcOption{ID: id, Additional: additional, Score: max(mainScore, subScore) * scoreScale}
	result.weapons[id] = *weapon
	result.notes[id] = note
	result.additional[id] = additional
	return result, nil
}

func betterArcCopy(candidate, current decoded.Weapon) bool {
	if candidate.Level != current.Level {
		return candidate.Level > current.Level
	}
	if candidate.Star != current.Star {
		return candidate.Star > current.Star
	}
	if candidate.Breakthrough != current.Breakthrough {
		return candidate.Breakthrough > current.Breakthrough
	}
	return candidate.ID.Key() < current.ID.Key()
}

func cloneStatSources(source map[string][]nte.Stat) map[string][]nte.Stat {
	result := make(map[string][]nte.Stat, len(source))
	for key, stats := range source {
		result[key] = append([]nte.Stat(nil), stats...)
	}
	return result
}
