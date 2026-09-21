package unreal

import "testing"

func TestBitReaderReadsLeastSignificantBitFirst(t *testing.T) {
	reader := bitReader{data: []byte{0b10110110}, end: 8}
	first, ok := reader.bits(4)
	if !ok || first != 0b0110 {
		t.Fatalf("first nibble = %04b, %v; want 0110, true", first, ok)
	}
	second, ok := reader.bits(4)
	if !ok || second != 0b1011 {
		t.Fatalf("second nibble = %04b, %v; want 1011, true", second, ok)
	}
	if _, ok := reader.bit(); ok {
		t.Fatal("read past end unexpectedly succeeded")
	}
}

func TestTerminatedBits(t *testing.T) {
	tests := []struct {
		data []byte
		want int
	}{{nil, 0}, {[]byte{0}, 0}, {[]byte{1}, 1}, {[]byte{0x80}, 8}, {[]byte{0x04, 0}, 3}}
	for _, test := range tests {
		if got := TerminatedBits(test.data); got != test.want {
			t.Fatalf("TerminatedBits(%v) = %d, want %d", test.data, got, test.want)
		}
	}
}

func TestAppendBitsAcrossByteBoundary(t *testing.T) {
	got, bits := AppendBits([]byte{0b00000101}, 3, []byte{0b00000111}, 3)
	if bits != 6 || len(got) != 1 || got[0] != 0b00111101 {
		t.Fatalf("AppendBits() = %08b, %d", got, bits)
	}
}
