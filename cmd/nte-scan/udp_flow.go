package main

import (
	"encoding/binary"
	"net"
)

type flowKey struct {
	src   string
	sport uint16
	dst   string
	dport uint16
}

type timedUDPPacket struct {
	ts   float64
	data []byte
}

func selectPrimaryUDPFlow(packets []packet, port uint16) (flowKey, []timedUDPPacket, int) {
	flows := map[flowKey][]timedUDPPacket{}
	for _, packet := range packets {
		data := packet.data
		if len(data) < 42 || binary.BigEndian.Uint16(data[12:14]) != 0x0800 {
			continue
		}
		ipOffset := 14
		headerLength := int(data[ipOffset]&15) * 4
		if headerLength < 20 || len(data) < ipOffset+headerLength+8 || data[ipOffset+9] != 17 {
			continue
		}
		udpOffset := ipOffset + headerLength
		sourcePort := binary.BigEndian.Uint16(data[udpOffset : udpOffset+2])
		destinationPort := binary.BigEndian.Uint16(data[udpOffset+2 : udpOffset+4])
		if port != 0 && sourcePort != port && destinationPort != port {
			continue
		}
		if port == 0 && (sourcePort == 53 || destinationPort == 53 || sourcePort == 443 || destinationPort == 443 || sourcePort == 5353 || destinationPort == 5353) {
			continue
		}
		end := udpOffset + int(binary.BigEndian.Uint16(data[udpOffset+4:udpOffset+6]))
		if end > len(data) {
			end = len(data)
		}
		if udpOffset+8 > end {
			continue
		}
		key := flowKey{
			src:   net.IP(data[ipOffset+12 : ipOffset+16]).String(),
			sport: sourcePort,
			dst:   net.IP(data[ipOffset+16 : ipOffset+20]).String(),
			dport: destinationPort,
		}
		if isPrivateIPv4(key.src) && isPrivateIPv4(key.dst) {
			continue
		}
		flows[key] = append(flows[key], timedUDPPacket{ts: packet.ts, data: append([]byte(nil), data[udpOffset+8:end]...)})
	}
	var selectedKey flowKey
	var selected []timedUDPPacket
	selectedBytes := 0
	for key, flow := range flows {
		payloadBytes := 0
		for _, packet := range flow {
			payloadBytes += len(packet.data)
		}
		if payloadBytes > selectedBytes {
			selectedBytes = payloadBytes
			selectedKey = key
			selected = flow
		}
	}
	return selectedKey, selected, selectedBytes
}

func measureUDPFlow(packets []timedUDPPacket) (largest, peakBytes int, peakAt float64) {
	if len(packets) == 0 {
		return 0, 0, 0
	}
	base := packets[0].ts
	for index, packet := range packets {
		largest = max(largest, len(packet.data))
		burst := 0
		for next := index; next < len(packets) && packets[next].ts-packet.ts <= .1; next++ {
			burst += len(packets[next].data)
		}
		if burst > peakBytes {
			peakBytes = burst
			peakAt = packet.ts - base
		}
	}
	return largest, peakBytes, peakAt
}
