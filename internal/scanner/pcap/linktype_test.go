package pcap

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func syntheticIPv4UDP() []byte {
	ip := make([]byte, 28)
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], 28)
	ip[8] = 64
	ip[9] = 17 // UDP
	copy(ip[12:16], []byte{192, 0, 2, 1})
	copy(ip[16:20], []byte{198, 51, 100, 2})
	binary.BigEndian.PutUint16(ip[20:22], 50000)
	binary.BigEndian.PutUint16(ip[22:24], 7777)
	binary.BigEndian.PutUint16(ip[24:26], 8)
	return ip
}

var snapIPv4 = []byte{0xaa, 0xaa, 0x03, 0, 0, 0, 0x08, 0x00}

func ieee80211Frame(fc0, fc1 byte, extra int) []byte {
	h := make([]byte, 24+extra)
	h[0], h[1] = fc0, fc1
	for i := 0; i < 6; i++ {
		h[4+i], h[10+i], h[16+i] = 0x11, 0x22, 0x33
	}
	return append(append(h, snapIPv4...), syntheticIPv4UDP()...)
}

func assertEthernetIPv4UDP(t *testing.T, frame []byte) {
	t.Helper()
	if len(frame) != 14+28 || binary.BigEndian.Uint16(frame[12:14]) != 0x0800 || frame[23] != 17 {
		t.Fatalf("not an Ethernet IPv4/UDP frame: % x", frame)
	}
	if !bytes.Equal(frame[14:], syntheticIPv4UDP()) {
		t.Fatal("payload changed")
	}
}

func TestNormalizeKeepsEthernet(t *testing.T) {
	eth := append(make([]byte, 12), 0x08, 0x00)
	eth = append(eth, syntheticIPv4UDP()...)
	out, kind := normalizeFrame(linkTypeEthernet, eth, nil)
	if kind != frameEthernet || &out[0] != &eth[0] {
		t.Fatalf("kind=%s", kind)
	}
}

func TestNormalize80211Data(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fc0, fc1 byte
		extra    int
	}{
		{"plain data from AP", 0x08, 0x02, 0},
		{"QoS data to AP", 0x88, 0x01, 2},
		{"QoS + HT control", 0x88, 0x81, 6},
		{"4-address QoS", 0x88, 0x03, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, lt := range []uint16{linkTypeIEEE80211, linkTypeEthernet} {
				out, kind := normalizeFrame(lt, ieee80211Frame(tc.fc0, tc.fc1, tc.extra), nil)
				if kind != frameIEEE80211 {
					t.Fatalf("link type %d: kind=%s", lt, kind)
				}
				assertEthernetIPv4UDP(t, out)
			}
		})
	}
}

func TestNormalizeRefusesProtectedAndNullFrames(t *testing.T) {
	for _, f := range [][]byte{ieee80211Frame(0x08, 0x42, 0), ieee80211Frame(0x48, 0x01, 0)} {
		if _, kind := normalizeFrame(linkTypeIEEE80211, f, nil); kind != frameUnrecognised {
			t.Fatalf("kind=%s", kind)
		}
	}
}

func TestNormalizeRadiotap(t *testing.T) {
	rt := []byte{0, 0, 8, 0, 0, 0, 0, 0} // version, pad, length 8, no fields
	out, kind := normalizeFrame(linkTypeRadiotap, append(rt, ieee80211Frame(0x08, 0x02, 0)...), nil)
	if kind != frameRadiotap {
		t.Fatalf("kind=%s", kind)
	}
	assertEthernetIPv4UDP(t, out)
}

func TestNormalizeRawIP(t *testing.T) {
	for _, lt := range []uint16{linkTypeRaw, linkTypeIPv4, linkTypeEthernet} {
		out, kind := normalizeFrame(lt, syntheticIPv4UDP(), nil)
		if kind != frameRawIP {
			t.Fatalf("link type %d: kind=%s", lt, kind)
		}
		assertEthernetIPv4UDP(t, out)
	}
	v6 := make([]byte, 48)
	v6[0], v6[6] = 0x60, 17
	out, kind := normalizeFrame(linkTypeIPv6, v6, nil)
	if kind != frameRawIP || binary.BigEndian.Uint16(out[12:14]) != 0x86dd {
		t.Fatalf("ipv6: kind=%s", kind)
	}
}

func TestNormalizeLeavesTruncatedFrames(t *testing.T) {
	for _, tc := range []struct {
		linkType uint16
		data     []byte
	}{
		{linkTypeIEEE80211, ieee80211Frame(0x08, 0x02, 0)[:28]},
		{linkTypeIEEE80211, ieee80211Frame(0x88, 0x03, 8)[:34]},
		{linkTypeRadiotap, []byte{0, 0, 64, 0, 0, 0, 0, 0}},
		{linkTypeRadiotap, []byte{0, 0}},
		{linkTypeRaw, nil},
	} {
		if out, kind := normalizeFrame(tc.linkType, tc.data, nil); kind != frameUnrecognised || !bytes.Equal(out, tc.data) {
			t.Fatalf("link type %d, %d bytes: kind=%s", tc.linkType, len(tc.data), kind)
		}
	}
}

func TestNormalizeLeavesGarbage(t *testing.T) {
	junk := bytes.Repeat([]byte{0x01}, 40)
	if out, kind := normalizeFrame(linkTypeEthernet, junk, nil); kind != frameUnrecognised || !bytes.Equal(out, junk) {
		t.Fatalf("kind=%s", kind)
	}
}

func TestCountFrameReportsFormatsOnly(t *testing.T) {
	var s SelectionSummary
	out, kind := normalizeFrame(linkTypeIEEE80211, ieee80211Frame(0x08, 0x02, 0), nil)
	s.countFrame(linkTypeIEEE80211, kind, out)
	want := map[string]int{"link_type_ieee80211": 1, "frame_ieee80211": 1, "ipv4": 1, "udp_ipv4": 1}
	for k, v := range want {
		if s.Formats[k] != v {
			t.Fatalf("formats = %v", s.Formats)
		}
	}
	if len(s.Formats) != len(want) {
		t.Fatalf("formats = %v", s.Formats)
	}
}

func TestReadFileSelectedConvertsWiFiFramesLabelledAsEthernet(t *testing.T) {
	sectionBody := make([]byte, 16)
	binary.LittleEndian.PutUint32(sectionBody[0:4], 0x1a2b3c4d)
	interfaceBody := make([]byte, 8)
	binary.LittleEndian.PutUint16(interfaceBody[0:2], linkTypeEthernet)
	capture := append(pcapngBlock(0x0a0d0d0a, sectionBody), pcapngBlock(1, interfaceBody)...)
	capture = append(capture, enhancedPacketBlock(1_000, ieee80211Frame(0x88, 0x02, 2))...)
	path := filepath.Join(t.TempDir(), "wifi.pcapng")
	if err := os.WriteFile(path, capture, 0o600); err != nil {
		t.Fatal(err)
	}

	packets, summary, err := ReadFileSelected(path, func(data []byte) bool {
		return len(data) >= 14 && binary.BigEndian.Uint16(data[12:14]) == 0x0800
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 1 || summary.Formats["frame_ieee80211"] != 1 || summary.Formats["link_type_ethernet"] != 1 {
		t.Fatalf("packets = %d, formats = %v", len(packets), summary.Formats)
	}
	assertEthernetIPv4UDP(t, packets[0].Data)
}
