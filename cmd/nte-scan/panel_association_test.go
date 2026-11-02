package main

import (
	"context"
	"testing"
)

func TestPanelAssociationRejectsEqualHPOwners(t *testing.T) {
	fields := []struct {
		handle byte
		value  float32
	}{{2, 10000}, {4, 10000}, {42, 500}, {44, 500}, {48, .2}, {64, .8}, {72, 1}, {84, .2}, {194, 600}, {196, 600}}
	data := syntheticPanelFields(fields)
	messages := []completedMessage{{data: data, bits: len(data) * 8}}
	characters := []characterItem{{CharacterID: 1004, MaxHP: 16000}, {CharacterID: 1036, MaxHP: 16000}}
	attachCharacterPanels(context.Background(), characters, messages)
	if characters[0].PanelStats != nil || characters[1].PanelStats != nil {
		t.Fatal("same HP attributed a panel to multiple entities")
	}
	characters = characters[:1]
	attachCharacterPanels(context.Background(), characters, append(messages, messages[0]))
	if characters[0].PanelStats == nil {
		t.Fatal("identical repeated evidence should remain usable")
	}
	fields[4].value = .5
	other := syntheticPanelFields(fields)
	attachCharacterPanels(context.Background(), characters, append(messages, completedMessage{data: other, bits: len(other) * 8}))
	if characters[0].PanelStats != nil {
		t.Fatal("conflicting panels should be unavailable")
	}
}
