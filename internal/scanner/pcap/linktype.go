package pcap

import "encoding/binary"

// On Wi-Fi adapters pktmon can record native 802.11 frames while labelling the
// interface as Ethernet. The decoders expect Ethernet II, so 802.11 data,
// radiotap and raw IP frames are rewritten with an Ethernet header. Protected
// and unrecognised frames are left unchanged.

const (
	linkTypeEthernet  = 1
	linkTypeRaw       = 101
	linkTypeIEEE80211 = 105
	linkTypeRadiotap  = 127
	linkTypeIPv4      = 228
	linkTypeIPv6      = 229
)

func linkTypeName(linkType uint16) string {
	switch linkType {
	case linkTypeEthernet:
		return "ethernet"
	case linkTypeIEEE80211:
		return "ieee80211"
	case linkTypeRadiotap:
		return "radiotap"
	case linkTypeRaw, linkTypeIPv4, linkTypeIPv6:
		return "raw_ip"
	}
	return "other"
}

func plausibleEtherType(t uint16) bool {
	switch t {
	case 0x0800, 0x86dd, 0x0806, 0x8100, 0x88a8, 0x88cc, 0x888e:
		return true
	}
	return false
}

const (
	frameEthernet     = "ethernet"
	frameIEEE80211    = "ieee80211"
	frameRadiotap     = "radiotap"
	frameRawIP        = "raw_ip"
	frameUnrecognised = "unrecognised"
)

// normalizeFrame returns the frame as Ethernet II, using scratch for any copy,
// and the kind of frame it was recorded as.
func normalizeFrame(linkType uint16, data []byte, scratch []byte) (frame []byte, kind string) {
	switch linkType {
	case linkTypeRaw, linkTypeIPv4, linkTypeIPv6:
		if len(data) == 0 {
			return data, frameUnrecognised
		}
		etherType := uint16(0x0800)
		if linkType == linkTypeIPv6 || (linkType == linkTypeRaw && data[0]>>4 == 6) {
			etherType = 0x86dd
		}
		return withEthernetHeader(scratch, nil, nil, etherType, data), frameRawIP
	case linkTypeIEEE80211:
		if out, ok := ieee80211ToEthernet(data, scratch); ok {
			return out, frameIEEE80211
		}
	case linkTypeRadiotap:
		if len(data) >= 4 {
			n := int(binary.LittleEndian.Uint16(data[2:4]))
			if n >= 8 && n <= len(data) {
				if out, ok := ieee80211ToEthernet(data[n:], scratch); ok {
					return out, frameRadiotap
				}
			}
		}
	default:
		// Ethernet (or an unknown type): keep real Ethernet frames, but accept
		// 802.11 data frames that were labelled as Ethernet.
		if len(data) >= 14 && plausibleEtherType(binary.BigEndian.Uint16(data[12:14])) {
			return data, frameEthernet
		}
		if out, ok := ieee80211ToEthernet(data, scratch); ok {
			return out, frameIEEE80211
		}
		// Raw IP labelled as Ethernet: an IPv4/IPv6 header at offset 0.
		if len(data) >= 20 && (data[0] == 0x45 || data[0]>>4 == 6) {
			etherType := uint16(0x0800)
			if data[0]>>4 == 6 {
				etherType = 0x86dd
			}
			return withEthernetHeader(scratch, nil, nil, etherType, data), frameRawIP
		}
	}
	return data, frameUnrecognised
}

// ieee80211ToEthernet converts an unencrypted 802.11 data frame carrying an
// LLC/SNAP header into Ethernet II.
func ieee80211ToEthernet(d []byte, scratch []byte) ([]byte, bool) {
	if len(d) < 24 {
		return nil, false
	}
	fc0, fc1 := d[0], d[1]
	if (fc0>>2)&3 != 2 || fc1&0x40 != 0 { // not a data frame, or protected
		return nil, false
	}
	subtype := fc0 >> 4
	if subtype&4 != 0 { // null / no-data subtypes
		return nil, false
	}
	toDS, fromDS := fc1&1 != 0, fc1&2 != 0
	hdr := 24
	if toDS && fromDS {
		hdr += 6
	}
	if subtype&8 != 0 { // QoS
		hdr += 2
		if fc1&0x80 != 0 { // HT control
			hdr += 4
		}
	}
	if len(d) < hdr+8 {
		return nil, false
	}
	llc := d[hdr : hdr+8]
	if llc[0] != 0xaa || llc[1] != 0xaa || llc[2] != 0x03 || llc[3] != 0 || llc[4] != 0 || (llc[5] != 0 && llc[5] != 0xf8) {
		return nil, false
	}
	etherType := binary.BigEndian.Uint16(llc[6:8])
	if !plausibleEtherType(etherType) {
		return nil, false
	}
	addr1, addr2, addr3 := d[4:10], d[10:16], d[16:22]
	dst, src := addr1, addr2
	switch {
	case toDS && !fromDS:
		dst = addr3
	case !toDS && fromDS:
		src = addr3
	case toDS && fromDS:
		dst, src = addr3, d[24:30]
	}
	return withEthernetHeader(scratch, dst, src, etherType, d[hdr+8:]), true
}

func withEthernetHeader(scratch, dst, src []byte, etherType uint16, payload []byte) []byte {
	n := 14 + len(payload)
	if cap(scratch) < n {
		scratch = make([]byte, n)
	}
	out := scratch[:n]
	for i := 0; i < 12; i++ {
		out[i] = 0
	}
	copy(out[0:6], dst)
	copy(out[6:12], src)
	binary.BigEndian.PutUint16(out[12:14], etherType)
	copy(out[14:], payload)
	return out
}
