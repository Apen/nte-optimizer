package pcap

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func streamFixture(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.pcapng")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStreamingSelectionKeepsSourceTimingAndIndependentPacketBytes(t *testing.T) {
	data := append(enhancedPacketBlock(2_000_000, []byte("keep-second")), enhancedPacketBlock(0, []byte("discard"))...)
	data = append(data, enhancedPacketBlock(1_000_000, []byte("keep-first"))...)
	packets, summary, err := ReadFileSelected(streamFixture(t, data), func(b []byte) bool { return len(b) > 0 && b[0] == 'k' })
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 2 || string(packets[0].Data) != "keep-first" || string(packets[1].Data) != "keep-second" || summary.SourcePackets != 3 || summary.SelectedPackets != 2 || summary.FirstTimestamp != 0 || summary.LastTimestamp != 2 {
		t.Fatalf("selection lost timing or reused packet bytes: %+v %+v", packets, summary)
	}
	all, _, err := ReadFileSelected(streamFixture(t, data), nil)
	want, parseErr := Parse(data)
	if err != nil || parseErr != nil || !reflect.DeepEqual(all, want) {
		t.Fatalf("streamed packets differ from existing parser: %v, %v", err, parseErr)
	}
}

func TestStreamingSelectionRejectsMalformedOrOversizedBlocks(t *testing.T) {
	valid := enhancedPacketBlock(1, []byte("synthetic"))
	badTrailer := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badTrailer[len(badTrailer)-4:], 0)
	badLength := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badLength[4:8], maxStreamBlockBytes+4)
	badPacket := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badPacket[20:24], uint32(len(valid)))
	for _, data := range [][]byte{valid[:8], valid[:len(valid)-1], badTrailer, badLength, badPacket} {
		if _, _, err := ReadFileSelected(streamFixture(t, data), nil); err == nil {
			t.Fatal("invalid stream accepted")
		}
	}
	path := filepath.Join(t.TempDir(), "oversized-source.pcapng")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxStreamCaptureBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if packets, summary, err := ReadFileSelected(path, nil); !errors.Is(err, ErrCaptureSourceBudgetExceeded) || packets != nil || summary.SourcePackets != 0 {
		t.Fatalf("source budget not enforced before parsing: %+v, %v", summary, err)
	}
}

func TestStreamingSelectionReadsLargeSourceWithSmallRetainedSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large-synthetic.pcapng")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse synthetic packet blocks exercise a source above the old whole-file
	// limit without keeping its unselected packet bytes in memory.
	const size = maxStreamBlockBytes
	for i := 0; i < 17; i++ {
		header := make([]byte, 28)
		binary.LittleEndian.PutUint32(header[:4], 6)
		binary.LittleEndian.PutUint32(header[4:8], size)
		binary.LittleEndian.PutUint32(header[16:20], uint32(i*1_000_000))
		binary.LittleEndian.PutUint32(header[20:24], size-32)
		binary.LittleEndian.PutUint32(header[24:28], size-32)
		offset := int64(i * size)
		if _, err := f.WriteAt(header, offset); err != nil {
			f.Close()
			t.Fatal(err)
		}
		var trailer [4]byte
		binary.LittleEndian.PutUint32(trailer[:], size)
		if _, err := f.WriteAt(trailer[:], offset+size-4); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	if _, err := f.WriteAt(enhancedPacketBlock(17_000_000, []byte("keep")), 17*size); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); !errors.Is(err, ErrCaptureBudgetExceeded) {
		t.Fatalf("fixture does not exceed whole-file budget: %v", err)
	}
	packets, summary, err := ReadFileSelected(path, func(data []byte) bool { return len(data) < 1024 })
	if err != nil || len(packets) != 1 || string(packets[0].Data) != "keep" || summary.SourcePackets != 18 || summary.SelectedBytes != 4 || summary.LastTimestamp != 17 {
		t.Fatalf("bounded large-source selection failed: %+v, %v", summary, err)
	}
}

func TestStreamingSelectionResetsTimestampResolutionAtSectionBoundaries(t *testing.T) {
	block := func(order binary.ByteOrder, typ uint32, body []byte) []byte {
		out := make([]byte, len(body)+12)
		order.PutUint32(out[:4], typ)
		order.PutUint32(out[4:8], uint32(len(out)))
		copy(out[8:], body)
		order.PutUint32(out[len(out)-4:], uint32(len(out)))
		return out
	}
	var capture []byte
	for i, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		section := make([]byte, 16)
		order.PutUint32(section[:4], 0x1a2b3c4d)
		capture = append(capture, block(order, 0x0a0d0d0a, section)...)
		iface := make([]byte, 16)
		order.PutUint16(iface[8:10], 9)
		order.PutUint16(iface[10:12], 1)
		iface[12] = byte(7 - i)
		capture = append(capture, block(order, 1, iface)...)
		body := make([]byte, 24)
		order.PutUint32(body[8:12], 1_000_000)
		order.PutUint32(body[12:16], 1)
		order.PutUint32(body[16:20], 1)
		body[20] = byte(i)
		capture = append(capture, block(order, 6, body)...)
	}
	packets, _, err := ReadFileSelected(streamFixture(t, capture), nil)
	if err != nil || len(packets) != 2 || math.Abs(packets[0].Timestamp-.1) > 1e-9 || math.Abs(packets[1].Timestamp-1) > 1e-9 {
		t.Fatalf("section timestamp resolution leaked: %+v, %v", packets, err)
	}
}
