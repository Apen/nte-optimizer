package unreal

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestReplicationParsersRejectTruncatedPayloads(t *testing.T) {
	if got := ParsePackageExports([]byte{0}, 1); got != nil {
		t.Fatalf("ParsePackageExports() = %#v, want nil", got)
	}
	if got := ParseContentBlocks([]byte{0}, 8, 0); got != nil {
		t.Fatalf("ParseContentBlocks() = %#v, want nil", got)
	}
	if got := ParseRPCs([]byte{0}, 8, 425); len(got) != 0 {
		t.Fatalf("ParseRPCs() = %#v, want empty", got)
	}
	if _, _, ok := ParseHottaContainer([]byte{0}, 8); ok {
		t.Fatal("ParseHottaContainer() unexpectedly accepted truncated data")
	}
}

func TestContainsPackedGUIDAtArbitraryBitOffset(t *testing.T) {
	// GUID 128 uses the bit-packed bytes 0x01, 0x02. Shift them by three bits
	// to exercise the bit-level search rather than a byte-aligned substring.
	data := []byte{0x08, 0x10, 0x00}
	if !ContainsPackedGUID(data, 19, 128) {
		t.Fatal("ContainsPackedGUID() did not find shifted GUID")
	}
	if ContainsPackedGUID(data, 19, 129) {
		t.Fatal("ContainsPackedGUID() matched another GUID")
	}
}

func TestExtractAtkAdd(t *testing.T) {
	payload := make([]byte, 7+4+4+18)
	copy(payload, []byte("AtkAdd\x00"))
	binary.LittleEndian.PutUint32(payload[11:15], 18)
	payload[15+13] = 8
	payload[15+15] = 42
	if got := ExtractAtkAdd(payload); !reflect.DeepEqual(got, []int{21}) {
		t.Fatalf("ExtractAtkAdd() = %v, want [21]", got)
	}
}
