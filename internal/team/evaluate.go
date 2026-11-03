package team

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

type Member struct {
	CharacterID int
	Arc         decoded.Weapon
}

type Contribution struct {
	RuleID      string
	SourceIDs   []int
	RecipientID int
	Stat        nte.Stat
}

type Unresolved struct {
	RuleID      string
	RecipientID int
	Reason      string
}

type Result struct {
	Contributions []Contribution
	Unresolved    []Unresolved
}

type rule struct {
	ID             string
	Status         string
	ExecutionBasis string `json:"execution_basis"`
	Parameter      struct {
		NameID     string `json:"name_id"`
		RankSource string `json:"rank_source"`
	}
	Target struct {
		PropertyID string `json:"property_id"`
		Operation  string
	}
	Recipients struct {
		Scope         string
		IncludeWearer bool   `json:"include_wearer"`
		Filter        string `json:"esper_type_filter"`
	}
	Conditions struct {
		OffField         bool `json:"wearer_off_field"`
		RequiresSkill    bool `json:"requires_skill"`
		RequiresUltimate bool `json:"requires_ultimate"`
		RequiresDamage   bool `json:"requires_damage"`
		Matching         *struct {
			Minimum       int
			IncludeWearer bool   `json:"include_wearer"`
			Filter        string `json:"esper_type_filter"`
		} `json:"matching_team_members"`
	}
	Stacking struct {
		Group string `json:"group_id"`
		Equal string `json:"equal_rank_multiple_sources"`
		Mixed string `json:"mixed_rank_multiple_sources"`
	}
}

type arc struct {
	Effects map[string]decoded.ForkEffect `json:"effects_by_star"`
	Rules   []rule                        `json:"team_effects"`
}

type Catalog struct {
	arcs  map[string]arc
	types map[int]string
}

func Load(arcsPath, charactersPath string) (Catalog, error) {
	var arcs struct{ Forks map[string]arc }
	var characters struct {
		Characters map[int]struct {
			EsperType string `json:"esper_type"`
		}
	}
	for _, input := range []struct {
		path   string
		target any
	}{{arcsPath, &arcs}, {charactersPath, &characters}} {
		data, err := os.ReadFile(input.path)
		if err != nil {
			return Catalog{}, err
		}
		if err = json.Unmarshal(data, input.target); err != nil {
			return Catalog{}, fmt.Errorf("read team catalog %s: %w", input.path, err)
		}
	}
	catalog := Catalog{arcs: arcs.Forks, types: make(map[int]string, len(characters.Characters))}
	for id, character := range characters.Characters {
		catalog.types[id] = character.EsperType
	}
	return catalog, nil
}

type candidate struct {
	rule   rule
	source int
	rank   int
	stat   nte.Stat
}

type group struct {
	id        string
	recipient int
}

func (c Catalog) Evaluate(members []Member, activeID int) (Result, error) {
	var result Result
	if len(members) == 0 && activeID == 0 {
		return result, nil
	}
	if len(members) > 4 {
		return result, fmt.Errorf("a team can contain at most four members")
	}
	members = slices.Clone(members)
	slices.SortFunc(members, func(a, b Member) int { return a.CharacterID - b.CharacterID })
	ids := map[int]bool{}
	instances := map[decoded.NetID]bool{}
	for _, member := range members {
		id := member.CharacterID
		if id <= 0 || ids[id] {
			return result, fmt.Errorf("invalid or duplicate team member %d", id)
		}
		ids[id] = true
		if c.types[id] == "" {
			return result, fmt.Errorf("missing Esper type for team member %d", id)
		}
		if member.Arc.ForkID != "" {
			if _, ok := c.arcs[member.Arc.ForkID]; !ok {
				return result, fmt.Errorf("unknown team Arc %q", member.Arc.ForkID)
			}
			if member.Arc.ID != (decoded.NetID{}) {
				if instances[member.Arc.ID] {
					return result, fmt.Errorf("duplicate team Arc instance")
				}
				instances[member.Arc.ID] = true
			}
		}
	}
	if !ids[activeID] {
		return result, fmt.Errorf("active character must be a team member")
	}
	groups := map[group][]candidate{}
	var order []group
	for _, wearer := range members {
		definition := c.arcs[wearer.Arc.ForkID]
		for _, r := range definition.Rules {
			if !r.supported() {
				result.Unresolved = append(result.Unresolved, Unresolved{RuleID: r.ID, Reason: "unsupported_rule"})
				continue
			}
			if r.Conditions.OffField && wearer.CharacterID == activeID {
				continue
			}
			if matching := r.Conditions.Matching; matching != nil {
				count := 0
				for _, member := range members {
					if member.CharacterID == wearer.CharacterID && !matching.IncludeWearer {
						continue
					}
					if c.types[member.CharacterID] == c.types[wearer.CharacterID] {
						count++
					}
				}
				if count < matching.Minimum {
					continue
				}
			}
			stat := resolveStat(definition, r, wearer.Arc.Star)
			for _, recipient := range members {
				id := recipient.CharacterID
				if id == wearer.CharacterID && !r.Recipients.IncludeWearer {
					continue
				}
				if r.Recipients.Scope == "active_character" && id != activeID {
					continue
				}
				if r.Recipients.Filter == "same_as_wearer" && c.types[id] != c.types[wearer.CharacterID] {
					continue
				}
				key := group{r.Stacking.Group, id}
				if _, exists := groups[key]; !exists {
					order = append(order, key)
				}
				groups[key] = append(groups[key], candidate{r, wearer.CharacterID, wearer.Arc.Star, stat})
			}
		}
	}
	for _, key := range order {
		candidates := groups[key]
		first := candidates[0]
		sources := make([]int, 0, len(candidates))
		reason := ""
		for _, item := range candidates {
			sources = append(sources, item.source)
			switch {
			case item.rank != first.rank:
				reason = "mixed_rank_priority_unknown"
			case reason != "":
			case len(candidates) > 1 && (item.rule.Stacking.Equal != "apply_once" || item.rule.ID != first.rule.ID):
				reason = "multiple_sources_unsupported"
			case item.stat.PropertyID == "":
				reason = "missing_rank_parameter"
			}
		}
		if reason != "" {
			result.Unresolved = append(result.Unresolved, Unresolved{first.rule.ID, key.recipient, reason})
			continue
		}
		result.Contributions = append(result.Contributions, Contribution{first.rule.ID, sources, key.recipient, first.stat})
	}
	return result, nil
}

func (r rule) supported() bool {
	if r.Status != "supported" || r.ExecutionBasis != "hardcoded_description_static_snapshot" || r.Target.Operation != "add" || r.Target.PropertyID == "" || r.Parameter.RankSource != "wearer_arc_rank" || r.Stacking.Group == "" {
		return false
	}
	if r.Conditions.RequiresSkill || r.Conditions.RequiresUltimate || r.Conditions.RequiresDamage {
		return false
	}
	if r.Recipients.Scope != "active_character" && r.Recipients.Scope != "team" {
		return false
	}
	if r.Recipients.Filter != "any" && r.Recipients.Filter != "same_as_wearer" {
		return false
	}
	if r.Stacking.Mixed != "unsupported" || (r.Stacking.Equal != "apply_once" && r.Stacking.Equal != "unsupported") {
		return false
	}
	matching := r.Conditions.Matching
	return matching == nil || (matching.Minimum > 0 && matching.Filter == "same_as_wearer")
}

func resolveStat(definition arc, r rule, rank int) nte.Stat {
	effect := definition.Effects[strconv.Itoa(rank)]
	for _, parameter := range effect.OrderedParameters {
		if parameter.NameID == r.Parameter.NameID {
			return nte.Stat{PropertyID: r.Target.PropertyID, Value: parameter.Value, Percent: parameter.IsPercent}
		}
	}
	return nte.Stat{}
}
