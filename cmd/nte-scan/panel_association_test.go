package main

import (
	"context"
	"errors"
	"testing"
)

// This fixture encodes invented identities and values, never captured bytes.
type panelFixture struct {
	id                      itemNetID
	actor, outer, reference uint32
	crit                    float32
	conflictingIdentity     bool
	finalHP                 float32
}

func fixturePacked(w *recordWriter, value uint32) {
	for {
		part := value & 127
		value >>= 7
		more := uint32(0)
		if value != 0 {
			more = 1
		}
		w.put(uint64(part<<1|more), 8)
		if more == 0 {
			return
		}
	}
}

func panelMessage(f panelFixture) completedMessage {
	if f.actor == 0 {
		f.actor = 20
	}
	if f.outer == 0 {
		f.outer = f.actor
	}
	if f.reference == 0 {
		f.reference = 22
	}
	if f.finalHP == 0 {
		f.finalHP = 16000
	}
	w := &recordWriter{}
	export := func(guid, outer uint32, name string) {
		fixturePacked(w, guid)
		w.put(1, 8)
		fixturePacked(w, outer)
		if outer != 0 {
			w.put(0, 8)
		}
		w.put(uint64(len(name)+1), 32)
		for _, b := range []byte(name) {
			w.put(uint64(b), 8)
		}
		w.put(0, 8)
	}
	w.put(0, 1)
	w.put(1, 32)
	export(11, 0, "Archetype")
	w.put(0, 1)
	w.put(2, 32)
	export(22, f.outer, "AttributeSet")
	export(24, f.actor, "AbilitySystemComponent")
	w.put(1, 16)
	fixturePacked(w, 11) // Must-map references.
	fixturePacked(w, f.actor)
	fixturePacked(w, 11)
	fixturePacked(w, 3)
	w.put(0, 13) // Invented spawn data; framing must not depend on its length.
	actor := &recordWriter{}
	identity := func(id itemNetID) {
		actor.put(198, 8)
		actor.put(uint64(id.Solt), 32)
		actor.put(200, 8)
		actor.put(uint64(id.Serial), 32)
	}
	identity(f.id)
	if f.conflictingIdentity {
		identity(itemNetID{99, 101})
	}
	attribute := &recordWriter{}
	attribute.put(0, 1)
	for _, field := range []struct {
		handle byte
		value  float32
	}{{2, 10000}, {4, 10000}, {42, 500}, {44, 500}, {48, f.crit}, {64, .8}, {72, 1}, {84, .2}, {194, 600}, {196, 600}} {
		attribute.put(uint64(field.handle), 8)
		attribute.putFloat(field.value)
	}
	attribute.put(0, 16)
	ability := &recordWriter{}
	fixturePacked(ability, f.reference)
	ability.put(0, 8)
	for i, handle := range []byte{36, 38, 40, 46, 56} {
		ability.put(uint64(handle), 8)
		ability.putFloat([]float32{f.finalHP, f.finalHP, f.finalHP, 1500, 200}[i])
	}
	block := func(guid uint32, payload *recordWriter) {
		w.put(1, 1)
		if guid == 0 {
			w.put(1, 1)
		} else {
			w.put(0, 1)
			fixturePacked(w, guid)
			w.put(1, 1)
		}
		fixturePacked(w, uint32(payload.bits))
		for bit := 0; bit < payload.bits; bit++ {
			w.bit(uint64(payload.data[bit/8] >> uint(bit%8)))
		}
	}
	block(0, actor)
	block(22, attribute)
	block(24, ability)
	return completedMessage{data: w.data, bits: w.bits, exports: true, channel: 5}
}

func TestPanelAssociationUsesIdentityWithEqualOrStaleHP(t *testing.T) {
	a, b := itemNetID{12, 34}, itemNetID{56, 78}
	characters := []characterItem{{NetID: a, MaxHP: 100}, {NetID: b, MaxHP: 100}}
	messages := []completedMessage{panelMessage(panelFixture{id: a, crit: .2}), panelMessage(panelFixture{id: b, actor: 30, crit: .7})}
	messages = append(messages, messages[0])
	if err := attachCharacterPanels(context.Background(), characters, messages); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float32{.2, .7} {
		stats := characters[i].PanelStats
		if stats == nil || stats.CritRate != want || stats.MaxHP != 16000 || stats.Attack != 1500 {
			t.Fatalf("character %d: %+v", i, stats)
		}
	}
}

func TestPanelAssociationRejectsUnprovenOwnership(t *testing.T) {
	id := itemNetID{12, 34}
	valid := panelMessage(panelFixture{id: id, crit: .2})
	for name, messages := range map[string][]completedMessage{
		"wrong outer":                {panelMessage(panelFixture{id: id, outer: 40})},
		"wrong attribute reference":  {panelMessage(panelFixture{id: id, reference: 26})},
		"unknown identity":           {panelMessage(panelFixture{id: itemNetID{98, 76}})},
		"conflicting actor identity": {panelMessage(panelFixture{id: id, conflictingIdentity: true})},
		"conflicting stats":          {valid, panelMessage(panelFixture{id: id, crit: .8})},
		"different actors":           {valid, panelMessage(panelFixture{id: id, actor: 40, crit: .2})},
		"bare stats":                 {{data: valid.data, bits: valid.bits}},
		"truncated":                  {{data: valid.data, bits: valid.bits - 1, exports: true}},
		"trailing foreign block":     {{data: append(append([]byte{}, valid.data...), 1), bits: valid.bits + 8, exports: true}},
	} {
		t.Run(name, func(t *testing.T) {
			characters := []characterItem{{NetID: id, MaxHP: 16000}}
			if err := attachCharacterPanels(context.Background(), characters, messages); err != nil {
				t.Fatal(err)
			}
			if characters[0].PanelStats != nil {
				t.Fatalf("accepted unsupported ownership: %+v", characters[0].PanelStats)
			}
		})
	}
	characters := []characterItem{{NetID: id}, {NetID: id}}
	if err := attachCharacterPanels(context.Background(), characters, []completedMessage{valid}); err != nil {
		t.Fatal(err)
	}
	if characters[0].PanelStats != nil || characters[1].PanelStats != nil {
		t.Fatal("duplicate roster identities accepted")
	}
}

func TestPanelAssociationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := attachCharacterPanels(ctx, nil, []completedMessage{{exports: true}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestPanelAssociationRejectsActorIdentityChanges(t *testing.T) {
	a, b := itemNetID{12, 34}, itemNetID{56, 78}
	characters := []characterItem{{NetID: a}, {NetID: b}}
	messages := []completedMessage{panelMessage(panelFixture{id: a}), panelMessage(panelFixture{id: b}), panelMessage(panelFixture{id: a})}
	if err := attachCharacterPanels(context.Background(), characters, messages); err != nil {
		t.Fatal(err)
	}
	if characters[0].PanelStats != nil || characters[1].PanelStats != nil {
		t.Fatal("actor GUID was attributed to conflicting roster identities")
	}
}

func TestPanelAssociationRejectsEveryTruncatedPrefix(t *testing.T) {
	id := itemNetID{12, 34}
	m := panelMessage(panelFixture{id: id})
	limit := m.bits
	for bits := 0; bits < limit; bits++ {
		m.bits = bits
		characters := []characterItem{{NetID: id}}
		if err := attachCharacterPanels(context.Background(), characters, []completedMessage{m}); err != nil {
			t.Fatal(err)
		}
		if characters[0].PanelStats != nil {
			t.Fatalf("accepted truncated prefix of %d bits", bits)
		}
	}
}

func FuzzActorSnapshotAssociation(f *testing.F) {
	m := panelMessage(panelFixture{id: itemNetID{12, 34}})
	f.Add(m.data, m.bits)
	f.Fuzz(func(t *testing.T, data []byte, bits int) {
		if len(data) > 65536 {
			t.Skip()
		}
		characters := []characterItem{{NetID: itemNetID{12, 34}}}
		if err := attachCharacterPanels(context.Background(), characters, []completedMessage{{data: data, bits: bits, exports: true}}); err != nil {
			t.Fatal(err)
		}
	})
}
