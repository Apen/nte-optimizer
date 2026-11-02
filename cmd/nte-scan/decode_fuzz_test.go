package main

import (
	"context"
	"errors"
	"testing"
)

func FuzzSparsePanel(f *testing.F) {
	f.Add([]byte{0, 1, 2}, 24)
	f.Fuzz(func(t *testing.T, data []byte, bits int) {
		if len(data) > 8192 {
			t.Skip()
		}
		parseCharacterPanelStats(data, bits, 16000)
		namedBytePayloads(data, bits, "FurnitureActivateStates")
	})
}

func TestDecodeCancellationBeforeCatalogAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := decodeDomains(ctx, &udpReport{}, nil, nil, "missing", "missing", "missing", "missing", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
}
