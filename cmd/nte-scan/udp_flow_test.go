package main

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestSelectPrimaryUDPFlowFiltersNoiseAndChoosesLargest(t *testing.T) {
	packets := []packet{
		{ts: 1, data: udpFrame(t, "10.0.0.2", "8.8.8.8", 51000, 53, []byte("ignored dns"))},
		{ts: 2, data: udpFrame(t, "10.0.0.2", "20.1.1.1", 51000, 30031, []byte("small"))},
		{ts: 3, data: udpFrame(t, "10.0.0.2", "20.1.1.2", 51001, 30032, []byte("largest payload"))},
	}
	key, selected, payloadBytes := selectPrimaryUDPFlow(packets, 0)
	if key.dport != 30032 || len(selected) != 1 || payloadBytes != len("largest payload") {
		t.Fatalf("unexpected selected flow: %#v, packets=%d bytes=%d", key, len(selected), payloadBytes)
	}
}

func TestMeasureUDPFlow(t *testing.T) {
	packets := []timedUDPPacket{{ts: 10, data: make([]byte, 4)}, {ts: 10.05, data: make([]byte, 7)}, {ts: 10.2, data: make([]byte, 2)}}
	largest, peak, at := measureUDPFlow(packets)
	if largest != 7 || peak != 11 || at != 0 {
		t.Fatalf("measureUDPFlow() = largest %d, peak %d at %f", largest, peak, at)
	}
}

func TestScannerPacketSelectionPreservesSupportedTransports(t *testing.T) {
	udp := udpFrame(t, "10.0.0.2", "20.1.1.1", 51000, 30031, []byte("synthetic"))
	if !scannerPacket(udp) {
		t.Fatal("game UDP filtered out")
	}
	tcp := append([]byte(nil), udp...)
	tcp[23] = 6
	if !scannerPacket(tcp) {
		t.Fatal("game TCP filtered out")
	}
	binary.BigEndian.PutUint16(tcp[34:36], 443)
	binary.BigEndian.PutUint16(tcp[36:38], 51000)
	if scannerPacket(tcp) {
		t.Fatal("unrelated HTTPS retained")
	}
	if scannerPacket([]byte{1, 2, 3}) {
		t.Fatal("truncated frame retained")
	}
}

func udpFrame(t *testing.T, source, destination string, sourcePort, destinationPort uint16, payload []byte) []byte {
	t.Helper()
	frame := make([]byte, 14+20+8+len(payload))
	binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	ip := frame[14:]
	ip[0], ip[9] = 0x45, 17
	copy(ip[12:16], net.ParseIP(source).To4())
	copy(ip[16:20], net.ParseIP(destination).To4())
	udp := ip[20:]
	binary.BigEndian.PutUint16(udp[0:2], sourcePort)
	binary.BigEndian.PutUint16(udp[2:4], destinationPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	copy(udp[8:], payload)
	return frame
}
