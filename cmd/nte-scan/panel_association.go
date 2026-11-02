package main

import "math"

// HP is supporting evidence, not an entity identifier. Until the actor-to-roster
// ownership contract is verified, reject collisions and conflicting panels.
func attachCharacterPanels(characters []characterItem, messages []completedMessage) {
	owners := map[uint32]int{}
	for _, character := range characters {
		owners[math.Float32bits(character.MaxHP)]++
	}
	for i := range characters {
		characters[i].PanelStats = nil
		if owners[math.Float32bits(characters[i].MaxHP)] != 1 {
			continue
		}
		var observed *characterPanelStats
		ambiguous := false
		for _, message := range messages {
			stats, ok := parseCharacterPanelStats(message.data, message.bits, characters[i].MaxHP)
			if !ok {
				continue
			}
			if observed != nil && *observed != stats {
				ambiguous = true
				break
			}
			copy := stats
			observed = &copy
		}
		if !ambiguous {
			characters[i].PanelStats = observed
		}
	}
}
