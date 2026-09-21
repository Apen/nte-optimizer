package unreal

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestExtractFramesExtractsPayload(t *testing.T) {
	frame := make([]byte, 19)
	binary.LittleEndian.PutUint32(frame[12:16], 3)
	copy(frame[16:], "abc")
	stream := make([]byte, 4+len(frame))
	binary.LittleEndian.PutUint32(stream[:4], uint32(len(frame)))
	copy(stream[4:], frame)

	got, err := ExtractFrames(stream)
	if err != nil || !reflect.DeepEqual(got, [][]byte{[]byte("abc")}) {
		t.Fatalf("ExtractFrames = %q, %v; want abc", got, err)
	}
	if _, err := ExtractFrames([]byte{20, 0, 0, 0, 1}); err == nil {
		t.Fatal("truncated frame was not rejected")
	}
}

func TestDecodeLZ4Block(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "literal", data: append([]byte{0x50}, []byte("hello")...), want: "hello"},
		{name: "match", data: []byte{0x10, 'A', 1, 0}, want: "AAAAA"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := DecodeLZ4Block(test.data)
			if !ok || string(got) != test.want {
				t.Fatalf("DecodeLZ4Block = %q, %v; want %q, true", got, ok, test.want)
			}
		})
	}
	if _, ok := DecodeLZ4Block([]byte{0x00, 0x00, 0x00}); ok {
		t.Fatal("empty padded block unexpectedly decoded")
	}
}
