package unreal

import "testing"

func FuzzDecodeLZ4(f *testing.F) {
	f.Add([]byte{0x10, 'A', 1, 0})
	f.Add([]byte{0xff, 0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32768 {
			t.Skip()
		}
		decoded, ok := DecodeLZ4BlockLimited(data, 65536)
		if ok && len(decoded) > 65536 {
			t.Fatal("output budget exceeded")
		}
	})
}

func TestLZ4RejectsOutputBeyondBudget(t *testing.T) {
	for _, data := range [][]byte{{0x50, 'h', 'e', 'l', 'l', 'o'}, {0x10, 'A', 1, 0}} {
		if _, ok := DecodeLZ4BlockLimited(data, 4); ok {
			t.Fatal("accepted oversized output")
		}
		if _, ok := DecodeLZ4BlockLimited(data, 5); !ok {
			t.Fatal("rejected exact output budget")
		}
	}
}
