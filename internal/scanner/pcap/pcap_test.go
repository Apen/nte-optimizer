package pcap

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestDeduplicateUsesConfiguredTimeWindow(t *testing.T) {
	packets := []Packet{
		{Timestamp: 0, Data: []byte("same")},
		{Timestamp: .001, Data: []byte("same")},
		{Timestamp: .003, Data: []byte("same")},
		{Timestamp: .0035, Data: []byte("different")},
	}
	got := Deduplicate(packets, .002)
	if len(got) != 3 || got[0].Timestamp != 0 || got[1].Timestamp != .003 || got[2].Timestamp != .0035 {
		t.Fatalf("unexpected deduplicated packets: %#v", got)
	}
}

func TestReassembleHandlesOverlapDuplicatesAndGaps(t *testing.T) {
	got, err := Reassemble([]Segment{
		{Sequence: 102, Data: []byte("cde")},
		{Sequence: 100, Data: []byte("abc")},
		{Sequence: 100, Data: []byte("abc")},
	})
	if err != nil || string(got) != "abcde" {
		t.Fatalf("Reassemble overlap = %q, %v; want abcde", got, err)
	}
	if got, err := Reassemble(nil); err != nil || got != nil {
		t.Fatalf("Reassemble empty = %v, %v; want nil, nil", got, err)
	}
	if _, err := Reassemble([]Segment{{Sequence: 10, Data: []byte("abc")}, {Sequence: 16, Data: []byte("x")}}); err == nil {
		t.Fatal("TCP gap was not rejected")
	}
}

func TestLargestInboundTCPFlowSelectsServerTraffic(t *testing.T) {
	packets := []Packet{
		{Data: ethernetTCPPacket([4]byte{1, 2, 3, 4}, [4]byte{10, 0, 0, 1}, 30031, 50000, 10, []byte("largest"))},
		{Data: ethernetTCPPacket([4]byte{5, 6, 7, 8}, [4]byte{10, 0, 0, 1}, 30031, 50001, 20, []byte("small"))},
		{Data: ethernetTCPPacket([4]byte{10, 0, 0, 1}, [4]byte{1, 2, 3, 4}, 50000, 30031, 30, []byte("outbound-is-ignored"))},
	}
	flow, segments := LargestInboundTCPFlow(packets, 30031)
	if flow.SourceAddress != "1.2.3.4" || flow.SourcePort != 30031 || len(segments) != 1 {
		t.Fatalf("unexpected selected flow: %#v, %#v", flow, segments)
	}
	if string(segments[0].Data) != "largest" {
		t.Fatalf("payload = %q", segments[0].Data)
	}
}

func TestParseRejectsInvalidBlock(t *testing.T) {
	block := make([]byte, 12)
	binary.LittleEndian.PutUint32(block[4:8], 64)
	if _, err := Parse(block); err == nil {
		t.Fatal("invalid block was accepted")
	}
}

func TestParseEmptyCapture(t *testing.T) {
	packets, err := Parse(nil)
	if err != nil || len(packets) != 0 {
		t.Fatalf("Parse(nil) = %#v, %v", packets, err)
	}
}

func TestParseReadsAndSortsEnhancedPackets(t *testing.T) {
	sectionBody := make([]byte, 16)
	binary.LittleEndian.PutUint32(sectionBody[0:4], 0x1a2b3c4d)
	interfaceBody := make([]byte, 8)

	capture := append(pcapngBlock(0x0a0d0d0a, sectionBody), pcapngBlock(1, interfaceBody)...)
	capture = append(capture, enhancedPacketBlock(2_000, []byte("second"))...)
	capture = append(capture, enhancedPacketBlock(1_000, []byte("first"))...)

	packets, err := Parse(capture)
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 2 {
		t.Fatalf("packet count = %d, want 2", len(packets))
	}
	if packets[0].Timestamp != .001 || packets[1].Timestamp != .002 {
		t.Fatalf("timestamps = %v, %v", packets[0].Timestamp, packets[1].Timestamp)
	}
	if !reflect.DeepEqual(packets[0].Data, []byte("first")) || !reflect.DeepEqual(packets[1].Data, []byte("second")) {
		t.Fatalf("unexpected packet order: %q, %q", packets[0].Data, packets[1].Data)
	}
}

func enhancedPacketBlock(timestamp uint64, data []byte) []byte {
	paddedLength := (len(data) + 3) &^ 3
	body := make([]byte, 20+paddedLength)
	binary.LittleEndian.PutUint32(body[4:8], uint32(timestamp>>32))
	binary.LittleEndian.PutUint32(body[8:12], uint32(timestamp))
	binary.LittleEndian.PutUint32(body[12:16], uint32(len(data)))
	binary.LittleEndian.PutUint32(body[16:20], uint32(len(data)))
	copy(body[20:], data)
	return pcapngBlock(6, body)
}

func pcapngBlock(blockType uint32, body []byte) []byte {
	var block bytes.Buffer
	length := uint32(12 + len(body))
	_ = binary.Write(&block, binary.LittleEndian, blockType)
	_ = binary.Write(&block, binary.LittleEndian, length)
	_, _ = block.Write(body)
	_ = binary.Write(&block, binary.LittleEndian, length)
	return block.Bytes()
}

func ethernetTCPPacket(source, destination [4]byte, sourcePort, destinationPort uint16, sequence uint32, payload []byte) []byte {
	packet := make([]byte, 14+20+20+len(payload))
	binary.BigEndian.PutUint16(packet[12:14], 0x0800)
	ipOffset := 14
	packet[ipOffset] = 0x45
	binary.BigEndian.PutUint16(packet[ipOffset+2:ipOffset+4], uint16(20+20+len(payload)))
	packet[ipOffset+9] = 6
	copy(packet[ipOffset+12:ipOffset+16], source[:])
	copy(packet[ipOffset+16:ipOffset+20], destination[:])
	tcpOffset := ipOffset + 20
	binary.BigEndian.PutUint16(packet[tcpOffset:tcpOffset+2], sourcePort)
	binary.BigEndian.PutUint16(packet[tcpOffset+2:tcpOffset+4], destinationPort)
	binary.BigEndian.PutUint32(packet[tcpOffset+4:tcpOffset+8], sequence)
	packet[tcpOffset+12] = 5 << 4
	copy(packet[tcpOffset+20:], payload)
	return packet
}
