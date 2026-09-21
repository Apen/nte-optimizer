package pcap

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sort"
)

type Packet struct {
	Timestamp float64
	Data      []byte
}

type Flow struct {
	SourceAddress      string
	SourcePort         uint16
	DestinationAddress string
	DestinationPort    uint16
}

func (f Flow) String() string {
	return fmt.Sprintf("%s:%d -> %s:%d", f.SourceAddress, f.SourcePort, f.DestinationAddress, f.DestinationPort)
}

type Segment struct {
	Sequence uint32
	Data     []byte
}

func ReadFile(path string) ([]Packet, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(contents)
}

func Parse(contents []byte) ([]Packet, error) {
	var packets []Packet
	littleEndian := true
	timestampScales := []float64{}
	for offset := 0; offset+12 <= len(contents); {
		if bytes.Equal(contents[offset:offset+4], []byte{0x0a, 0x0d, 0x0d, 0x0a}) {
			littleEndian = bytes.Equal(contents[offset+8:offset+12], []byte{0x4d, 0x3c, 0x2b, 0x1a})
		}
		var byteOrder binary.ByteOrder = binary.BigEndian
		if littleEndian {
			byteOrder = binary.LittleEndian
		}
		blockType := byteOrder.Uint32(contents[offset : offset+4])
		blockLength := int(byteOrder.Uint32(contents[offset+4 : offset+8]))
		if blockLength < 12 || offset+blockLength > len(contents) {
			return nil, fmt.Errorf("invalid PCAPNG block at offset %d", offset)
		}
		body := contents[offset+8 : offset+blockLength-4]
		switch {
		case blockType == 1 && len(body) >= 8:
			timestampScales = append(timestampScales, interfaceTimestampScale(body, byteOrder))
		case blockType == 6 && len(body) >= 20:
			interfaceID := int(byteOrder.Uint32(body[0:4]))
			capturedLength := int(byteOrder.Uint32(body[12:16]))
			if 20+capturedLength <= len(body) {
				ticks := uint64(byteOrder.Uint32(body[4:8]))<<32 | uint64(byteOrder.Uint32(body[8:12]))
				scale := 1e-6
				if interfaceID < len(timestampScales) {
					scale = timestampScales[interfaceID]
				}
				data := append([]byte(nil), body[20:20+capturedLength]...)
				packets = append(packets, Packet{Timestamp: float64(ticks) * scale, Data: data})
			}
		}
		offset += blockLength
	}
	sort.Slice(packets, func(i, j int) bool { return packets[i].Timestamp < packets[j].Timestamp })
	return packets, nil
}

func Deduplicate(packets []Packet, windowSeconds float64) []Packet {
	lastSeen := map[[32]byte]float64{}
	unique := make([]Packet, 0, len(packets))
	for _, packet := range packets {
		hash := sha256.Sum256(packet.Data)
		if timestamp, exists := lastSeen[hash]; exists && packet.Timestamp-timestamp < windowSeconds {
			continue
		}
		lastSeen[hash] = packet.Timestamp
		unique = append(unique, packet)
	}
	return unique
}

func LargestInboundTCPFlow(packets []Packet, serverPort uint16) (Flow, []Segment) {
	flows := tcpFlows(packets, serverPort)
	var largestFlow Flow
	var largestSegments []Segment
	largestSize := 0
	for flow, segments := range flows {
		if flow.SourcePort != serverPort {
			continue
		}
		size := 0
		for _, segment := range segments {
			size += len(segment.Data)
		}
		if size > largestSize {
			largestSize = size
			largestFlow = flow
			largestSegments = segments
		}
	}
	return largestFlow, largestSegments
}

func Reassemble(segments []Segment) ([]byte, error) {
	if len(segments) == 0 {
		return nil, nil
	}
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].Sequence == segments[j].Sequence {
			return len(segments[i].Data) < len(segments[j].Data)
		}
		return segments[i].Sequence < segments[j].Sequence
	})
	start := segments[0].Sequence
	stream := []byte{}
	seen := map[string]bool{}
	for _, segment := range segments {
		key := fmt.Sprintf("%d:%s", segment.Sequence, hex.EncodeToString(segment.Data))
		if seen[key] {
			continue
		}
		seen[key] = true
		offset := int(uint32(segment.Sequence - start))
		if offset > len(stream) {
			return nil, fmt.Errorf("TCP gap of %d bytes", offset-len(stream))
		}
		skip := len(stream) - offset
		if skip < len(segment.Data) {
			stream = append(stream, segment.Data[skip:]...)
		}
	}
	return stream, nil
}

func tcpFlows(packets []Packet, port uint16) map[Flow][]Segment {
	flows := map[Flow][]Segment{}
	for _, packet := range packets {
		data := packet.Data
		if len(data) < 54 || binary.BigEndian.Uint16(data[12:14]) != 0x0800 {
			continue
		}
		ipOffset := 14
		ipHeaderLength := int(data[ipOffset]&15) * 4
		if data[ipOffset+9] != 6 || len(data) < ipOffset+ipHeaderLength+20 {
			continue
		}
		totalLength := int(binary.BigEndian.Uint16(data[ipOffset+2 : ipOffset+4]))
		tcpOffset := ipOffset + ipHeaderLength
		sourcePort := binary.BigEndian.Uint16(data[tcpOffset : tcpOffset+2])
		destinationPort := binary.BigEndian.Uint16(data[tcpOffset+2 : tcpOffset+4])
		if sourcePort != port && destinationPort != port {
			continue
		}
		tcpHeaderLength := int(data[tcpOffset+12]>>4) * 4
		end := ipOffset + totalLength
		if end > len(data) {
			end = len(data)
		}
		if tcpOffset+tcpHeaderLength >= end {
			continue
		}
		flow := Flow{
			SourceAddress:      net.IP(data[ipOffset+12 : ipOffset+16]).String(),
			SourcePort:         sourcePort,
			DestinationAddress: net.IP(data[ipOffset+16 : ipOffset+20]).String(),
			DestinationPort:    destinationPort,
		}
		sequence := binary.BigEndian.Uint32(data[tcpOffset+4 : tcpOffset+8])
		payload := append([]byte(nil), data[tcpOffset+tcpHeaderLength:end]...)
		flows[flow] = append(flows[flow], Segment{Sequence: sequence, Data: payload})
	}
	return flows
}

func interfaceTimestampScale(body []byte, byteOrder binary.ByteOrder) float64 {
	scale := 1e-6
	for offset := 8; offset+4 <= len(body); {
		code := byteOrder.Uint16(body[offset : offset+2])
		length := int(byteOrder.Uint16(body[offset+2 : offset+4]))
		if offset+4+length > len(body) {
			break
		}
		if code == 9 && length > 0 {
			resolution := body[offset+4]
			if resolution&128 != 0 {
				scale = 1 / float64(uint64(1)<<uint(resolution&127))
			} else {
				scale = 1
				for exponent := byte(0); exponent < resolution; exponent++ {
					scale /= 10
				}
			}
		}
		offset += 4 + (length+3)&^3
		if code == 0 {
			break
		}
	}
	return scale
}
