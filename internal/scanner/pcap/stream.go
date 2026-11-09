package pcap

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
)

const MaxStreamCaptureBytes = 1 << 30
const maxStreamBlockBytes = 16 << 20
const maxSelectedPackets = 1_000_000

var ErrCaptureSourceBudgetExceeded = errors.New("capture exceeds streaming source budget")

// SelectionSummary describes all source packets, including those the caller's
// predicate excludes. Timestamps stay internal; shareable logs use durations.
type SelectionSummary struct {
	SourcePackets   int
	SelectedPackets int
	SelectedBytes   int64
	FirstTimestamp  float64
	LastTimestamp   float64
}

// ReadFileSelected streams a bounded source instead of allocating the complete
// PCAPNG. Only selected packet bytes are retained, under the decoder's existing
// byte budget and a separate record-count budget. The predicate must not retain
// packet slices: they point to a reusable block buffer.
func ReadFileSelected(path string, keep func([]byte) bool) ([]Packet, SelectionSummary, error) {
	var summary SelectionSummary
	file, err := os.Open(path)
	if err != nil {
		return nil, summary, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, summary, err
	}
	if !info.Mode().IsRegular() {
		return nil, summary, fmt.Errorf("capture is not a regular file")
	}
	if info.Size() > MaxStreamCaptureBytes {
		return nil, summary, ErrCaptureSourceBudgetExceeded
	}
	reader := bufio.NewReaderSize(io.LimitReader(file, MaxStreamCaptureBytes+1), 64<<10)
	var header [12]byte
	var order binary.ByteOrder = binary.LittleEndian
	var scales []float64
	var packets []Packet
	var block []byte
	var consumed int64
	for {
		_, err := io.ReadFull(reader, header[:])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, summary, fmt.Errorf("read PCAPNG header: %w", err)
		}
		section := bytes.Equal(header[:4], []byte{0x0a, 0x0d, 0x0d, 0x0a})
		if section {
			switch {
			case bytes.Equal(header[8:12], []byte{0x4d, 0x3c, 0x2b, 0x1a}):
				order = binary.LittleEndian
			case bytes.Equal(header[8:12], []byte{0x1a, 0x2b, 0x3c, 0x4d}):
				order = binary.BigEndian
			default:
				return nil, summary, fmt.Errorf("invalid PCAPNG byte order")
			}
			scales = nil
		}
		length := int(order.Uint32(header[4:8]))
		if length < 12 || length%4 != 0 || length > maxStreamBlockBytes {
			return nil, summary, fmt.Errorf("invalid bounded PCAPNG block length")
		}
		consumed += int64(length)
		if consumed > MaxStreamCaptureBytes {
			return nil, summary, ErrCaptureSourceBudgetExceeded
		}
		if cap(block) < length {
			block = make([]byte, length)
		} else {
			block = block[:length]
		}
		copy(block, header[:])
		if _, err := io.ReadFull(reader, block[12:]); err != nil {
			return nil, summary, fmt.Errorf("read PCAPNG block: %w", err)
		}
		if order.Uint32(block[length-4:]) != uint32(length) {
			return nil, summary, fmt.Errorf("invalid PCAPNG block trailer")
		}
		body := block[8 : length-4]
		switch order.Uint32(block[:4]) {
		case 1:
			if len(body) < 8 || len(scales) >= 65536 {
				return nil, summary, fmt.Errorf("invalid PCAPNG interface table")
			}
			scales = append(scales, interfaceTimestampScale(body, order))
		case 6:
			if len(body) < 20 {
				return nil, summary, fmt.Errorf("truncated enhanced packet block")
			}
			capturedLength := order.Uint32(body[12:16])
			if uint64(capturedLength) > uint64(len(body)-20) {
				return nil, summary, fmt.Errorf("truncated captured packet")
			}
			captured := int(capturedLength)
			iface := order.Uint32(body[:4])
			scale := 1e-6
			if uint64(iface) < uint64(len(scales)) {
				scale = scales[iface]
			}
			ticks := uint64(order.Uint32(body[4:8]))<<32 | uint64(order.Uint32(body[8:12]))
			ts := float64(ticks) * scale
			if summary.SourcePackets == 0 {
				summary.FirstTimestamp, summary.LastTimestamp = ts, ts
			}
			summary.FirstTimestamp = min(summary.FirstTimestamp, ts)
			summary.LastTimestamp = max(summary.LastTimestamp, ts)
			summary.SourcePackets++
			data := body[20 : 20+captured]
			if keep != nil && !keep(data) {
				continue
			}
			if summary.SelectedBytes+int64(captured) > maxCaptureBytes || summary.SelectedPackets >= maxSelectedPackets {
				return nil, summary, ErrCaptureBudgetExceeded
			}
			packets = append(packets, Packet{Timestamp: ts, Data: append([]byte(nil), data...)})
			summary.SelectedBytes += int64(captured)
			summary.SelectedPackets++
		}
	}
	sort.Slice(packets, func(i, j int) bool { return packets[i].Timestamp < packets[j].Timestamp })
	return packets, summary, nil
}
