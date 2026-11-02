package unreal

import "context"

// ActorSnapshot is a complete initial actor message. Every subobject has a
// declared outer matching GUID. Unsupported or incomplete layouts are rejected.
type ActorSnapshot struct {
	GUID    uint32
	Exports []PackageExport
	Blocks  []ContentBlock
}

// ParseActorSnapshot locates bounded content-block framing after actor creation
// data. It does not guess the length of the variable spawn transform. Framing
// must consume the entire message and start with the actor and its AttributeSet.
// The caller must validate the actor's domain identity and reject ambiguity.
// Limits below are decoding budgets, not protocol maxima.
func ParseActorSnapshot(ctx context.Context, data []byte, bits int) (ActorSnapshot, bool) {
	if bits < 0 || bits > len(data)*8 || bits > 1<<20 || ctx.Err() != nil {
		return ActorSnapshot{}, false
	}
	r := bitReader{data: data, end: bits}
	var exports []PackageExport
	for section := 0; section < 64; section++ {
		if ctx.Err() != nil {
			return ActorSnapshot{}, false
		}
		entries, n := parsePackageExportsFrom(data, bits, r.pos)
		if len(entries) == 0 || n == 0 {
			break
		}
		exports = append(exports, entries...)
		if len(exports) > 4096 {
			return ActorSnapshot{}, false
		}
		r.pos += n
	}
	if len(exports) == 0 {
		return ActorSnapshot{}, false
	}
	count, ok := r.bits(16)
	if !ok || count > 4096 {
		return ActorSnapshot{}, false
	}
	for i := uint64(0); i < count; i++ {
		if _, ok := r.packed(); !ok {
			return ActorSnapshot{}, false
		}
	}
	actor, ok := r.packed()
	if !ok || actor == 0 || actor&1 != 0 {
		return ActorSnapshot{}, false
	}
	archetype, ok := r.packed()
	if !ok || archetype == 0 {
		return ActorSnapshot{}, false
	}
	if _, ok = r.packed(); !ok {
		return ActorSnapshot{}, false
	} // Level reference.
	byGUID := map[uint32]PackageExport{}
	for _, entry := range exports {
		if old, exists := byGUID[entry.GUID]; exists && old != entry {
			return ActorSnapshot{}, false
		}
		byGUID[entry.GUID] = entry
	}
	if _, exists := byGUID[archetype]; !exists {
		return ActorSnapshot{}, false
	}
	for start := r.pos; start < min(bits, r.pos+4096); start++ {
		if start%64 == 0 && ctx.Err() != nil {
			return ActorSnapshot{}, false
		}
		reader := bitReader{data: data, pos: start, end: bits}
		flags, ok := reader.bits(2)
		if !ok || flags != 3 {
			continue
		}
		n, ok := reader.packed()
		if !ok || n < 80 || n > 16384 || int(n) > bits-reader.pos {
			continue
		}
		actorStart := reader.pos
		reader.pos += int(n)
		var blocks []ContentBlock
		var positions []int
		seen := map[uint32]bool{}
		valid := true
		for reader.pos < bits && len(blocks) < 128 {
			flags, ok := reader.bits(2)
			if !ok || flags&2 != 0 {
				valid = false
				break
			}
			guid, ok := reader.packed()
			if !ok {
				valid = false
				break
			}
			entry, exists := byGUID[guid]
			if !exists || entry.Outer != actor || seen[guid] {
				valid = false
				break
			}
			seen[guid] = true
			stable, ok := reader.bit()
			if !ok || stable != 1 {
				valid = false
				break
			}
			length, ok := reader.packed()
			if !ok || int(length) > bits-reader.pos {
				valid = false
				break
			}
			if len(blocks) == 0 && (entry.Path != "AttributeSet" || flags&1 == 0) {
				valid = false
				break
			}
			positions = append(positions, reader.pos)
			reader.pos += int(length)
			blocks = append(blocks, ContentBlock{GUID: guid, Bits: int(length), HasReplication: flags&1 != 0})
		}
		if !valid || reader.pos != bits || len(blocks) < 2 {
			continue
		}
		// Materialize payloads only after the entire framing has been validated.
		for i := range blocks {
			blocks[i].Data, _ = sliceBits(data, bits, positions[i], blocks[i].Bits)
		}
		payload, _ := sliceBits(data, bits, actorStart, int(n))
		blocks = append([]ContentBlock{{Actor: true, HasReplication: true, Data: payload, Bits: int(n)}}, blocks...)
		// Never rescue a later framing after the first actor identity is rejected.
		return ActorSnapshot{GUID: actor, Exports: exports, Blocks: blocks}, true
	}
	return ActorSnapshot{}, false
}

func sliceBits(data []byte, bits, start, count int) ([]byte, bool) {
	if start < 0 || count < 0 || start > bits || count > bits-start {
		return nil, false
	}
	r := bitReader{data: data, pos: start, end: bits}
	return r.take(count)
}
