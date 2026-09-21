package main

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"strconv"

	scannerunreal "nte-optimizer/internal/scanner/unreal"
)

type equipmentCatalog struct {
	Items      map[string]equipmentDefinition `json:"items"`
	Attributes map[string]struct{}            `json:"attributes"`
	Curves     map[string][][2]float32        `json:"curves"`
	Shapes     map[string]equipmentShape      `json:"shapes"`
	Suits      map[string]equipmentSuit       `json:"suits"`
}
type equipmentSuit struct {
	NameEN  string                `json:"name_en"`
	Effects []equipmentSuitEffect `json:"effects"`
}
type equipmentSuitEffect struct {
	Count  int    `json:"count"`
	TextEN string `json:"text_en"`
}
type equipmentShape struct {
	Cells []equipmentGridCell `json:"cells"`
}
type equipmentGridCell struct {
	Row    int32 `json:"row"`
	Column int32 `json:"column"`
}
type equipmentDefinition struct {
	Kind      string  `json:"kind"`
	Quality   string  `json:"quality"`
	Grid      *uint32 `json:"grid"`
	MaxLevel  uint32  `json:"max_level"`
	MainCount int     `json:"main_count"`
	SubCount  int     `json:"sub_count"`
	Geometry  string  `json:"geometry"`
	Suit      string  `json:"suit"`
}
type itemNetID struct {
	Solt   uint32 `json:"solt"`
	Serial uint32 `json:"serial"`
}
type inventoryStat struct {
	Property string  `json:"property"`
	Value    float32 `json:"value"`
}
type inventoryItem struct {
	ID                itemNetID          `json:"id"`
	ItemID            string             `json:"itemId"`
	Name              string             `json:"name,omitempty"`
	Kind              string             `json:"kind"`
	Level             uint32             `json:"level"`
	MainStats         []inventoryStat    `json:"mainStats"`
	SubStats          []inventoryStat    `json:"subStats"`
	Locked            bool               `json:"locked"`
	Discarded         bool               `json:"discarded"`
	CharacterNetID    *itemNetID         `json:"characterNetId,omitempty"`
	EquippedPlacement *equipmentGridCell `json:"equippedPlacement,omitempty"`
}

func loadEquipmentCatalog(path string) (*equipmentCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c equipmentCatalog
	e = json.Unmarshal(b, &c)
	return &c, e
}

type invBits struct {
	data           []byte
	bitLen, cursor int
}

func (r *invBits) bits(n int) (uint64, bool) {
	if n < 0 || n > 64 || r.cursor+n > r.bitLen {
		return 0, false
	}
	var v uint64
	for i := 0; i < n; i++ {
		v |= uint64((r.data[(r.cursor+i)/8]>>((r.cursor+i)%8))&1) << i
	}
	r.cursor += n
	return v, true
}
func (r *invBits) u16() (uint16, bool)   { v, o := r.bits(16); return uint16(v), o }
func (r *invBits) u32() (uint32, bool)   { v, o := r.bits(32); return uint32(v), o }
func (r *invBits) i32() (int32, bool)    { v, o := r.u32(); return int32(v), o }
func (r *invBits) i64() (int64, bool)    { v, o := r.bits(64); return int64(v), o }
func (r *invBits) boolean() (bool, bool) { v, o := r.bits(1); return v != 0, o }
func (r *invBits) f32() (float32, bool)  { v, o := r.u32(); return math.Float32frombits(v), o }
func (r *invBits) bytes(n int) ([]byte, bool) {
	if n < 0 || r.cursor+n*8 > r.bitLen {
		return nil, false
	}
	b := make([]byte, n)
	for i := range b {
		v, o := r.bits(8)
		if !o {
			return nil, false
		}
		b[i] = byte(v)
	}
	return b, true
}
func (r *invBits) fstring(max int) (string, bool) {
	n, o := r.i32()
	if !o {
		return "", false
	}
	if n == 0 {
		return "", true
	}
	if n < 0 || int(n) > max {
		return "", false
	}
	b, o := r.bytes(int(n))
	if !o || len(b) == 0 || b[len(b)-1] != 0 {
		return "", false
	}
	for _, x := range b[:len(b)-1] {
		if x == 0 {
			return "", false
		}
	}
	return string(b[:len(b)-1]), true
}
func (r *invBits) dynamicName() (string, bool) {
	hard, o := r.boolean()
	if !o || hard {
		return "", false
	}
	s, o := r.fstring(128)
	if !o || s == "" {
		return "", false
	}
	n, o := r.u32()
	return s, o && n == 0
}
func (r *invBits) netID() (itemNetID, bool) {
	a, o := r.u32()
	if !o {
		return itemNetID{}, false
	}
	b, o := r.u32()
	return itemNetID{a, b}, o
}
func validNetID(v itemNetID) bool {
	return v.Solt != 0 && v.Serial != 0 && v.Solt != math.MaxUint32 && v.Serial != math.MaxUint32
}
func mainStatValue(c *equipmentCatalog, d equipmentDefinition, p string, level uint32) (float32, bool) {
	q := map[string]string{"blue": "ITEM_QUALITY_BLUE", "purple": "ITEM_QUALITY_PURPLE", "orange": "ITEM_QUALITY_ORANGE"}[d.Quality]
	if q == "" {
		return 0, false
	}
	var k string
	if d.Kind == "module" {
		if d.Grid == nil {
			return 0, false
		}
		k = p + "_" + strconv.Itoa(int(*d.Grid)) + "_" + q
	} else {
		k = p + "_Core_" + q
	}
	a, o := c.Curves[k]
	if !o || len(a) == 0 {
		return 0, false
	}
	x := float32(level)
	if x <= a[0][0] {
		return a[0][1], true
	}
	for i := 1; i < len(a); i++ {
		if x <= a[i][0] {
			p, q := a[i-1], a[i]
			if q[0] == p[0] {
				return q[1], true
			}
			return p[1] + (x-p[0])*(q[1]-p[1])/(q[0]-p[0]), true
		}
	}
	return a[len(a)-1][1], true
}

func parseInventoryItemAt(data []byte, bitLen, off int, c *equipmentCatalog) (inventoryItem, int, bool) {
	r := invBits{data, bitLen, off}
	itemID, o := r.dynamicName()
	if !o {
		return inventoryItem{}, 0, false
	}
	def, o := c.Items[itemID]
	if !o {
		return inventoryItem{}, 0, false
	}
	id, o := r.netID()
	if !o || !validNetID(id) {
		return inventoryItem{}, 0, false
	}
	one, o := r.i64()
	if !o || one != 1 {
		return inventoryItem{}, 0, false
	}
	if _, o = r.i32(); !o {
		return inventoryItem{}, 0, false
	}
	created, o := r.i64()
	if !o || created < 0 {
		return inventoryItem{}, 0, false
	}
	for _, want := range []uint16{0, 0, 1} {
		v, o := r.u16()
		if !o || v != want {
			return inventoryItem{}, 0, false
		}
	}
	lv, o := r.i32()
	if !o || lv < 0 || uint32(lv) > def.MaxLevel {
		return inventoryItem{}, 0, false
	}
	ch, o := r.netID()
	if !o {
		return inventoryItem{}, 0, false
	}
	var chp *itemNetID
	if ch.Solt != 0 || ch.Serial != 0 {
		if !validNetID(ch) {
			return inventoryItem{}, 0, false
		}
		z := ch
		chp = &z
	}
	dur, o := r.i32()
	if !o || dur < 0 {
		return inventoryItem{}, 0, false
	}
	locked, o := r.boolean()
	if !o {
		return inventoryItem{}, 0, false
	}
	discarded, o := r.boolean()
	if !o {
		return inventoryItem{}, 0, false
	}
	z, o := r.u16()
	if !o || z != 0 {
		return inventoryItem{}, 0, false
	}
	cnt, o := r.u16()
	if !o || int(cnt) != def.MainCount {
		return inventoryItem{}, 0, false
	}
	it := inventoryItem{ID: id, ItemID: itemID, Kind: def.Kind, Level: uint32(lv), Locked: locked, Discarded: discarded, CharacterNetID: chp}
	for i := 0; i < def.MainCount; i++ {
		p, o := r.dynamicName()
		if !o {
			return inventoryItem{}, 0, false
		}
		if _, o = c.Attributes[p]; !o {
			return inventoryItem{}, 0, false
		}
		v, o := mainStatValue(c, def, p, uint32(lv))
		if !o {
			return inventoryItem{}, 0, false
		}
		it.MainStats = append(it.MainStats, inventoryStat{p, v})
	}
	cnt, o = r.u16()
	if !o || int(cnt) != def.SubCount {
		return inventoryItem{}, 0, false
	}
	for i := 0; i < def.SubCount; i++ {
		p, o := r.dynamicName()
		if !o {
			return inventoryItem{}, 0, false
		}
		if _, o = c.Attributes[p]; !o {
			return inventoryItem{}, 0, false
		}
		v, o := r.f32()
		if !o || v <= 0 || v > 1e6 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return inventoryItem{}, 0, false
		}
		it.SubStats = append(it.SubStats, inventoryStat{p, v})
	}
	timer, o := r.f32()
	if !o || timer < 0 || math.IsNaN(float64(timer)) {
		return inventoryItem{}, 0, false
	}
	if _, o = r.boolean(); !o {
		return inventoryItem{}, 0, false
	}
	tail, o := r.i32()
	if !o || (tail != 0 && tail != 1) {
		return inventoryItem{}, 0, false
	}
	if _, o = r.fstring(16 * 1024); !o {
		return inventoryItem{}, 0, false
	}
	return it, r.cursor, true
}
func parseInventoryItems(data []byte, bitLen int, c *equipmentCatalog) []inventoryItem {
	if bitLen > len(data)*8 {
		return nil
	}
	m := map[itemNetID]inventoryItem{}
	for off := 0; off < bitLen; {
		it, next, ok := parseInventoryItemAt(data, bitLen, off, c)
		if ok {
			m[it.ID] = it
			if len(m) >= 4096 {
				break
			}
			if next > off {
				off = next
				continue
			}
		}
		off++
	}
	out := make([]inventoryItem, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID.Solt == out[j].ID.Solt {
			return out[i].ID.Serial < out[j].ID.Serial
		}
		return out[i].ID.Solt < out[j].ID.Solt
	})
	return out
}

type placementCandidate struct {
	itemID  string
	cells   map[equipmentGridCell]bool
	first   *equipmentGridCell
	invalid bool
}

func parseCompactPlacements(data []byte, bitLen int, known map[itemNetID]inventoryItem, c *equipmentCatalog) map[itemNetID]equipmentGridCell {
	groups := map[itemNetID]*placementCandidate{}
	for off := 0; off < bitLen; {
		r := invBits{data: data, bitLen: bitLen, cursor: off}
		one, o := r.i32()
		if !o || one != 1 {
			off++
			continue
		}
		itemID, o := r.dynamicName()
		if !o {
			off++
			continue
		}
		def, o := c.Items[itemID]
		if !o || def.Kind != "module" {
			off++
			continue
		}
		id, o := r.netID()
		if !o || !validNetID(id) {
			off++
			continue
		}
		first, o := r.boolean()
		if !o {
			off++
			continue
		}
		row, o := r.i32()
		if !o {
			off++
			continue
		}
		col, o := r.i32()
		if !o {
			off++
			continue
		}
		nf, o := r.i32()
		if !o || row < 0 || row >= 7 || col < 0 || col >= 7 || nf != 0 {
			off++
			continue
		}
		if it, o := known[id]; o && it.ItemID != itemID {
			off++
			continue
		}
		g := groups[id]
		if g == nil {
			g = &placementCandidate{itemID: itemID, cells: map[equipmentGridCell]bool{}}
			groups[id] = g
		}
		cell := equipmentGridCell{row, col}
		if g.itemID != itemID || g.cells[cell] {
			g.invalid = true
		}
		g.cells[cell] = true
		if first {
			if g.first != nil {
				g.invalid = true
			}
			x := cell
			g.first = &x
		}
		if r.cursor > off {
			off = r.cursor
		} else {
			off++
		}
	}
	out := map[itemNetID]equipmentGridCell{}
	for id, g := range groups {
		if g.invalid || g.first == nil {
			continue
		}
		def := c.Items[g.itemID]
		shape, o := c.Shapes[def.Geometry]
		if !o || def.Grid == nil || len(g.cells) != int(*def.Grid) || len(shape.Cells) != len(g.cells) {
			continue
		}
		valid := true
		for _, rel := range shape.Cells {
			if !g.cells[equipmentGridCell{g.first.Row + rel.Row, g.first.Column + rel.Column}] {
				valid = false
				break
			}
		}
		if valid {
			out[id] = *g.first
		}
	}
	return out
}

type invBunch struct {
	channel, seq uint16
	flags        uint8
	data         []byte
	bits         int
}
type invFragment struct {
	b     invBunch
	order int
}
type invStream struct {
	data []byte
	bits int
}

func bitValue(data []byte, off, n int) (uint64, bool) {
	if n < 0 || n > 64 || off < 0 || off+n > len(data)*8 {
		return 0, false
	}
	var v uint64
	for i := 0; i < n; i++ {
		v |= uint64((data[(off+i)/8]>>((off+i)%8))&1) << i
	}
	return v, true
}
func extractBitRange(data []byte, off, n int) ([]byte, bool) {
	if off < 0 || n < 0 || off+n > len(data)*8 {
		return nil, false
	}
	out := make([]byte, (n+7)/8)
	for i := 0; i < n; i++ {
		if data[(off+i)/8]&(1<<((off+i)%8)) != 0 {
			out[i/8] |= 1 << (i % 8)
		}
	}
	return out, true
}
func transportPayload(data []byte) (uint16, []byte, int, bool) {
	end := scannerunreal.TerminatedBits(data) - 1
	if end < 72 {
		return 0, nil, 0, false
	}
	sig, _ := bitValue(data, 3, 3)
	if sig == 7 {
		return 0, nil, 0, false
	}
	mode, _ := bitValue(data, 6, 2)
	if mode != 0 {
		return 0, nil, 0, false
	}
	pid, _ := bitValue(data, 24, 14)
	p, n := extractBitRange(data, 72, end-72)
	return uint16(pid), p, end - 72, n
}
func inventoryBunchAt(payload []byte, pbits, off int, exact bool) (invBunch, bool) {
	if off+48 > pbits {
		return invBunch{}, false
	}
	f, _ := bitValue(payload, off+23, 12)
	desc := uint8(f >> 4)
	flags := uint8(f & 15)
	if desc != 0xcc || (flags != 8 && flags != 9 && flags != 12 && flags != 13) {
		return invBunch{}, false
	}
	n, _ := bitValue(payload, off+35, 13)
	stop := off + 48 + int(n)
	if n == 0 || stop > pbits {
		return invBunch{}, false
	}
	if exact && (stop+1 != pbits || func() bool { x, _ := bitValue(payload, stop, 1); return x != 1 }()) {
		return invBunch{}, false
	}
	prefix, _ := bitValue(payload, off, 13)
	seq, _ := bitValue(payload, off+13, 10)
	d, o := extractBitRange(payload, off+48, int(n))
	return invBunch{uint16(prefix) & 0x3ff, uint16(seq), flags, d, int(n)}, o
}
func inventoryBunches(payload []byte, pbits int, known map[uint16]bool) []invBunch {
	if pbits < 49 {
		return nil
	}
	type located struct {
		off int
		b   invBunch
	}
	var exact []located
	for off := 0; off <= pbits-48; off++ {
		if b, o := inventoryBunchAt(payload, pbits, off, true); o {
			known[b.channel] = true
			exact = append(exact, located{off, b})
		}
	}
	seen := map[[2]uint16]bool{}
	all := append([]located(nil), exact...)
	for _, x := range exact {
		seen[[2]uint16{x.b.channel, x.b.seq}] = true
	}
	if len(known) > 0 {
		for off := 0; off <= pbits-48; off++ {
			if b, o := inventoryBunchAt(payload, pbits, off, false); o && known[b.channel] && !seen[[2]uint16{b.channel, b.seq}] {
				seen[[2]uint16{b.channel, b.seq}] = true
				all = append(all, located{off, b})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].off < all[j].off })
	out := make([]invBunch, 0, len(all))
	for _, x := range all {
		out = append(out, x.b)
	}
	return out
}
func reassembleInventoryStreams(packets [][]byte) []invStream {
	known := map[uint16]bool{}
	frags := map[[2]uint16]invFragment{}
	var out []invStream
	for order, raw := range packets {
		_, p, pbits, o := transportPayload(raw)
		if !o {
			continue
		}
		for _, b := range inventoryBunches(p, pbits, known) {
			if b.flags == 13 {
				out = append(out, invStream{b.data, b.bits})
				continue
			}
			frags[[2]uint16{b.channel, b.seq}] = invFragment{b, order}
		}
		var starts [][2]uint16
		for k, v := range frags {
			if v.b.flags == 9 {
				starts = append(starts, k)
			}
		}
		for _, start := range starts {
			var d []byte
			bits := 0
			seq := start[1]
			var used [][2]uint16
			complete := false
			min, max := order, 0
			for i := 0; i < 1024; i++ {
				k := [2]uint16{start[0], seq}
				v, ok := frags[k]
				if !ok {
					break
				}
				if v.order < min {
					min = v.order
				}
				if v.order > max {
					max = v.order
				}
				if max-min > 96 {
					break
				}
				if (i == 0 && v.b.flags != 9) || (i > 0 && v.b.flags != 8 && v.b.flags != 12) {
					break
				}
				d, bits = scannerunreal.AppendBits(d, bits, v.b.data, v.b.bits)
				used = append(used, k)
				if v.b.flags == 12 {
					complete = true
					break
				}
				seq = (seq + 1) & 0x3ff
			}
			if complete {
				out = append(out, invStream{d, bits})
				for _, k := range used {
					delete(frags, k)
				}
			}
		}
		for k, v := range frags {
			if order-v.order > 96 {
				delete(frags, k)
			}
		}
	}
	return out
}
