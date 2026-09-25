package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseResourcesAcceptsEveryCatalogItemWithoutGroupFiltering(t *testing.T) {
	writer := resourceFixtureWriter{}
	writer.add("Annulith", itemNetID{Solt: 11, Serial: 101}, 7)
	writer.add("SyntheticEventToken", itemNetID{Solt: 12, Serial: 102}, 23)
	writer.add("UncataloguedItem", itemNetID{Solt: 13, Serial: 103}, 99)

	catalog := &resourceCatalog{Items: map[string]struct{}{
		"Annulith":            {},
		"SyntheticEventToken": {},
	}}
	got := parseResources(writer.data, writer.bitLen, catalog)
	want := []resourceItem{
		{ID: itemNetID{Solt: 11, Serial: 101}, ItemID: "Annulith", Quantity: 7},
		{ID: itemNetID{Solt: 12, Serial: 102}, ItemID: "SyntheticEventToken", Quantity: 23},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseResources() = %#v, want %#v", got, want)
	}
}

func TestProductionResourceCatalogDecodesNewAndUnusualResourceIDs(t *testing.T) {
	catalog, err := loadResourceCatalog(filepath.Join("..", "..", "data", "game", "resources", "decode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Items) < 200 {
		t.Fatalf("production resource catalog has %d IDs, want at least 200", len(catalog.Items))
	}
	wantIDs := []string{"Annulith", "Food_005", "food_008", "Vehicle007", "Fishbait01", "gold"}
	writer := resourceFixtureWriter{}
	for index, itemID := range wantIDs {
		if _, ok := catalog.Items[itemID]; !ok {
			t.Fatalf("production resource catalog does not contain %q", itemID)
		}
		writer.add(itemID, itemNetID{Solt: uint32(21 + index), Serial: uint32(31 + index)}, int64(index+1))
	}

	got := parseResources(writer.data, writer.bitLen, catalog)
	if len(got) != len(wantIDs) {
		t.Fatalf("decoded %d production resources, want %d: %#v", len(got), len(wantIDs), got)
	}
	decodedIDs := make(map[string]bool, len(got))
	for _, resource := range got {
		decodedIDs[resource.ItemID] = true
	}
	for _, itemID := range wantIDs {
		if !decodedIDs[itemID] {
			t.Errorf("production resource %q was not decoded", itemID)
		}
	}
}

// resourceFixtureWriter emits only the fields consumed by parseResourceAt.
// Records are bit-packed like the scanner's inventory stream and contain no
// account data, so they are safe to use as synthetic regression fixtures.
type resourceFixtureWriter struct {
	data   []byte
	bitLen int
}

func (w *resourceFixtureWriter) bits(value uint64, count int) {
	for bit := 0; bit < count; bit++ {
		byteIndex := w.bitLen / 8
		if byteIndex == len(w.data) {
			w.data = append(w.data, 0)
		}
		if value&(uint64(1)<<bit) != 0 {
			w.data[byteIndex] |= 1 << (w.bitLen % 8)
		}
		w.bitLen++
	}
}

func (w *resourceFixtureWriter) add(itemID string, id itemNetID, quantity int64) {
	w.bits(0, 1) // Dynamic name is not hard-coded.
	w.bits(uint64(len(itemID)+1), 32)
	for _, value := range append([]byte(itemID), 0) {
		w.bits(uint64(value), 8)
	}
	w.bits(0, 32) // Dynamic-name table index.
	w.bits(uint64(id.Solt), 32)
	w.bits(uint64(id.Serial), 32)
	w.bits(uint64(quantity), 64)
	w.bits(0, 32) // Reserved integer.
	w.bits(0, 64) // Creation timestamp.
}
