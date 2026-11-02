package main

import (
	"context"
	"math"
)

// HP is supporting evidence, not an entity identifier. Until the actor-to-roster
// ownership contract is verified, reject collisions and conflicting panels.
func attachCharacterPanels(ctx context.Context, characters []characterItem, messages []completedMessage) error {
	owners := map[uint32]int{}
	for _, character := range characters {
		owners[math.Float32bits(character.MaxHP)]++
	}
	for i := range characters {
		if err := ctx.Err(); err != nil {
			return err
		}
		characters[i].PanelStats = nil
		if owners[math.Float32bits(characters[i].MaxHP)] != 1 {
			continue
		}
		var observed *characterPanelStats
		ambiguous := false
		for _, message := range messages {
			stats, ok := parseCharacterPanelStatsContext(ctx, message.data, message.bits, characters[i].MaxHP)
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
	return ctx.Err()
}
