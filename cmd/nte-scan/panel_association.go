package main

import (
	"context"

	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

// Panels require matching roster identity and declared subobject ownership.
// HP and blueprint names are never identity keys.
func attachCharacterPanels(ctx context.Context, characters []characterItem, messages []completedMessage) error {
	indices := map[itemNetID]int{}
	ambiguous := map[itemNetID]bool{}
	for i := range characters {
		characters[i].PanelStats = nil
		id := characters[i].NetID
		if _, exists := indices[id]; exists {
			ambiguous[id] = true
		}
		indices[id] = i
	}
	actors := map[itemNetID]uint32{}
	actorOwners := map[uint32]itemNetID{}
	conflictingActors := map[uint32]bool{}
	for _, message := range messages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !message.exports {
			continue
		}
		snapshot, ok := scannerunreal.ParseActorSnapshot(ctx, message.data, message.bits)
		if ok {
			id, ok := panelActorIdentity(ctx, snapshot.Blocks[0])
			if !ok {
				continue
			}
			i, exists := indices[id]
			if !exists || ambiguous[id] {
				continue
			}
			stats, ok := ownedPanelStats(ctx, snapshot)
			if !ok {
				continue
			}
			if previous, exists := actorOwners[snapshot.GUID]; exists && previous != id {
				conflictingActors[snapshot.GUID] = true
				ambiguous[previous] = true
				characters[indices[previous]].PanelStats = nil
			}
			if conflictingActors[snapshot.GUID] {
				ambiguous[id] = true
				characters[i].PanelStats = nil
				continue
			}
			actorOwners[snapshot.GUID] = id
			if actor, exists := actors[id]; exists && (actor != snapshot.GUID || *characters[i].PanelStats != stats) {
				ambiguous[id] = true
				characters[i].PanelStats = nil
				continue
			}
			actors[id] = snapshot.GUID
			characters[i].PanelStats = &stats
		}
	}
	return ctx.Err()
}

func panelActorIdentity(ctx context.Context, block scannerunreal.ContentBlock) (itemNetID, bool) {
	var found itemNetID
	ok := false
	// Actor-layout encoded fields 198/200 carry the roster slot and serial.
	// They are separate from the AttributeSet's similarly numbered DEF fields.
	for off := 0; off+80 <= block.Bits; off++ {
		if off%256 == 0 && ctx.Err() != nil {
			return itemNetID{}, false
		}
		first, _ := panelFieldHandle(block.Data, block.Bits, off)
		second, _ := panelFieldHandle(block.Data, block.Bits, off+40)
		if first != 198 || second != 200 {
			continue
		}
		r := invBits{data: block.Data, bitLen: block.Bits, cursor: off + 8}
		slot, _ := r.u32()
		r.cursor += 8
		serial, _ := r.u32()
		id := itemNetID{slot, serial}
		if !validNetID(id) {
			continue
		}
		if ok && found != id {
			return itemNetID{}, false
		}
		found = id
		ok = true
	}
	return found, ok
}

func ownedPanelStats(ctx context.Context, snapshot scannerunreal.ActorSnapshot) (characterPanelStats, bool) {
	attribute := snapshot.Blocks[1]
	var abilityGUID uint32
	for _, entry := range snapshot.Exports {
		if entry.Path == "AbilitySystemComponent" && entry.Outer == snapshot.GUID {
			if abilityGUID != 0 && abilityGUID != entry.GUID {
				return characterPanelStats{}, false
			}
			abilityGUID = entry.GUID
		}
	}
	var ability *scannerunreal.ContentBlock
	for i := range snapshot.Blocks {
		if snapshot.Blocks[i].GUID == abilityGUID && snapshot.Blocks[i].HasReplication {
			if ability != nil {
				return characterPanelStats{}, false
			}
			ability = &snapshot.Blocks[i]
		}
	}
	if ability == nil {
		return characterPanelStats{}, false
	}
	var stats characterPanelStats
	found := false
	for off := 8; off+400 <= attribute.Bits; off++ {
		if off%256 == 0 && ctx.Err() != nil {
			return characterPanelStats{}, false
		}
		candidate, ok := readPanelAttributeBlock(attribute.Data, attribute.Bits, off, attribute.Bits)
		if !ok {
			continue
		}
		if found && stats != candidate {
			return characterPanelStats{}, false
		}
		stats = candidate
		found = true
	}
	if !found {
		return characterPanelStats{}, false
	}
	// Final ASC values explicitly reference this AttributeSet. Adjacent buffs
	// cannot supply another object's final HP, attack, or endurance.
	finalFound := false
	for off := 0; off+216 <= ability.Bits; off++ {
		if off%256 == 0 && ctx.Err() != nil {
			return characterPanelStats{}, false
		}
		r := invBits{data: ability.Data, bitLen: ability.Bits, cursor: off}
		guid, ok := r.packed()
		if !ok || guid != attribute.GUID {
			continue
		}
		marker, ok := r.bits(8)
		if !ok || marker != 0 {
			continue
		}
		var values [5]float32
		valid := true
		for i, handle := range []uint64{36, 38, 40, 46, 56} {
			field, ok := r.bits(8)
			if !ok || field != handle {
				valid = false
				break
			}
			value, ok := bitFloat32(ability.Data, ability.Bits, r.cursor)
			if !ok {
				valid = false
				break
			}
			r.cursor += 32
			values[i] = value
		}
		if !valid || !sameFloat(values[0], values[1]) || !sameFloat(values[0], values[2]) || !validPanelFloat(values[0], 1, 1000000) || !validPanelFloat(values[3], 1, 100000) || !validPanelFloat(values[4], 1, 10000) {
			continue
		}
		if finalFound && (!sameFloat(stats.MaxHP, values[0]) || !sameFloat(stats.Attack, values[3]) || !sameFloat(stats.Endurance, values[4])) {
			return characterPanelStats{}, false
		}
		stats.MaxHP = values[0]
		stats.Attack = values[3]
		stats.Endurance = values[4]
		finalFound = true
	}
	return stats, finalFound
}
