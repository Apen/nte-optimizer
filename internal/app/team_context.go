package app

import (
	"fmt"
	"maps"
	"slices"
	"strconv"

	"nte-optimizer/internal/apperrors"
	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
	"nte-optimizer/internal/team"
)

type TeamAlly struct {
	CharacterID int    `json:"character_id"`
	ArcID       string `json:"arc_id,omitempty"`
}

type teamScenario struct {
	catalog team.Catalog
	members []team.Member
	allies  []TeamAlly
	gear    map[int]teamGear
}

func (s *OptimizerService) prepareTeam(inv nte.Inventory, state *decoded.State, activeID int, currentArc *decoded.Weapon) (*teamScenario, error) {
	if s.WeightOverrides == nil || len(s.WeightOverrides.TeamAllies) == 0 {
		return nil, nil
	}
	allies := s.WeightOverrides.TeamAllies
	invalid := func(reason string) (*teamScenario, error) {
		return nil, apperrors.New("invalid_team", fmt.Errorf("%s", reason))
	}
	if state == nil || len(allies) > 3 || !slices.ContainsFunc(state.Characters, func(c decoded.Character) bool { return c.CharacterID == activeID }) {
		return invalid("team requires an imported active character and at most three allies")
	}
	layout := datafiles.New(s.DataDir)
	charactersPath := layout.DecodeCatalog("characters.json")
	catalog, err := team.Load(layout.Arcs(), charactersPath)
	if err != nil {
		return nil, err
	}
	compatibility, err := decoded.LoadArcCompatibilityCatalog(charactersPath, layout.Arcs())
	if err != nil {
		return nil, err
	}
	scenario := &teamScenario{catalog: catalog, members: []team.Member{{CharacterID: activeID}}}
	scenario.gear = make(map[int]teamGear, len(allies))
	selectedCharacters := map[int]bool{activeID: true}
	selectedArcs := map[string]bool{}
	reserved := maps.Clone(s.ReservedArcIDs)
	if reserved == nil {
		reserved = map[string]bool{}
	}
	for _, ally := range allies {
		index := slices.IndexFunc(state.Characters, func(c decoded.Character) bool { return c.CharacterID == ally.CharacterID })
		if selectedCharacters[ally.CharacterID] || index < 0 {
			return invalid("duplicate or unavailable ally")
		}
		selectedCharacters[ally.CharacterID] = true
		member := team.Member{CharacterID: ally.CharacterID}
		gear := resolveTeamGear(state.Characters[index], s.teamBuilds[strconv.Itoa(ally.CharacterID)], inv, state.Weapons)
		scenario.gear[ally.CharacterID] = gear
		ally.ArcID = ""
		if gear.arc != nil {
			ally.ArcID = gear.arc.ID.Key()
			if selectedArcs[ally.ArcID] || (currentArc != nil && currentArc.ID.Key() == ally.ArcID) {
				return invalid("an Arc instance cannot be used by multiple team members")
			}
			member.Arc = *gear.arc
			if !slices.Contains(compatibility.CompatibleCharacterIDs(member.Arc.ForkID), ally.CharacterID) {
				return invalid("ally Arc is incompatible")
			}
			selectedArcs[ally.ArcID] = true
			reserved[ally.ArcID] = true
		}
		scenario.allies = append(scenario.allies, ally)
		scenario.members = append(scenario.members, member)
	}
	s.ReservedArcIDs = reserved
	return scenario, nil
}

func (s teamScenario) evaluate(activeArc *decoded.Weapon) (team.Result, error) {
	members := slices.Clone(s.members)
	if activeArc != nil {
		members[0].Arc = *activeArc
	}
	return s.catalog.Evaluate(members, members[0].CharacterID)
}

func (s teamScenario) apply(activeArc *decoded.Weapon, sources map[string][]nte.Stat) ([]string, error) {
	activeID := s.members[0].CharacterID
	result, err := s.evaluate(activeArc)
	if err != nil {
		return nil, apperrors.New("invalid_team", err)
	}
	for _, contribution := range result.Contributions {
		if contribution.RecipientID == activeID {
			sources["team"] = append(sources["team"], contribution.Stat)
		}
	}
	var warnings []string
	for _, issue := range result.Unresolved {
		if (issue.RecipientID == 0 || issue.RecipientID == activeID) && !slices.Contains(warnings, issue.Reason) {
			warnings = append(warnings, issue.Reason)
		}
	}
	return warnings, nil
}
