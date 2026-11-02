package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type characterDefinition struct {
	Codename string `json:"codename"`
}
type characterCatalog struct {
	Characters map[string]characterDefinition `json:"characters"`
}
type characterItem struct {
	BondStageFlags                []bool                  `json:"bondStageFlags,omitempty"`
	NetID                         itemNetID               `json:"netId"`
	CharacterID                   uint32                  `json:"characterId"`
	Name                          string                  `json:"name"`
	Codename                      string                  `json:"codename"`
	Level                         int32                   `json:"level"`
	BreakthroughLevel             int32                   `json:"breakthroughLevel"`
	AwakenLevel                   int32                   `json:"awakenLevel"`
	BondLevel                     *int                    `json:"bondLevel,omitempty"`
	ForkNetID                     *itemNetID              `json:"forkNetId,omitempty"`
	SkillLevels                   []characterSkillLevel   `json:"skillLevels"`
	MaxHP                         float32                 `json:"maxHp"`
	SavedHealthRatio              float32                 `json:"savedHealthRatio"`
	PanelStats                    *characterPanelStats    `json:"panelStats,omitempty"`
	ObservedLoadedAtLogin         bool                    `json:"observedLoadedAtLogin,omitempty"`
	ObservedActiveAwakeningLevels []int32                 `json:"observedActiveAwakeningLevels,omitempty"`
	ObservedActiveAwakeningBuffs  []string                `json:"observedActiveAwakeningBuffs,omitempty"`
	ObservedActiveEquipmentBuffs  []observedEquipmentBuff `json:"observedActiveEquipmentBuffs,omitempty"`
	ObservedActiveWeaponBuffs     []observedWeaponBuff    `json:"observedActiveWeaponBuffs,omitempty"`
}

type characterPanelStats struct {
	BaseHP               float32 `json:"baseHp"`
	BaseAttack           float32 `json:"baseAttack"`
	BaseDefense          float32 `json:"baseDefense"`
	MaxHP                float32 `json:"maxHp"`
	Attack               float32 `json:"attack"`
	Defense              float32 `json:"defense"`
	Endurance            float32 `json:"endurance"`
	CritRate             float32 `json:"critRate"`
	CritDamage           float32 `json:"critDamage"`
	ChargeEfficiency     float32 `json:"chargeEfficiency"`
	CycleIntensity       float32 `json:"cycleIntensity"`
	HasCycleIntensity    bool    `json:"-"`
	BreakIntensity       float32 `json:"breakIntensity"`
	UniversalDMGBonus    float32 `json:"universalDamageBonus"`
	ElementalDMGBonus    float32 `json:"elementalDamageBonus"`
	HasElementalDMGBonus bool    `json:"-"`
}

func bitFloat32(data []byte, bitLen, off int) (float32, bool) {
	if off < 0 || off+32 > bitLen {
		return 0, false
	}
	var raw uint32
	for bit := 0; bit < 32; bit++ {
		raw |= uint32((data[(off+bit)/8]>>uint((off+bit)%8))&1) << uint(bit)
	}
	return math.Float32frombits(raw), true
}

func sameFloat(a, b float32) bool { return math.Float32bits(a) == math.Float32bits(b) }

// parseCharacterPanelStats recognizes the replicated HTCharacterAttributeSet.
// The observed attributes use 8-bit encoded field handles followed by float32
// values. Base/current members can be independently omitted, so values must be
// read by handle rather than by their position in the serialized sequence.
func parseCharacterPanelStats(data []byte, bitLen int, maxHP float32) (characterPanelStats, bool) {
	if bitLen < 0 || bitLen > len(data)*8 {
		return characterPanelStats{}, false
	}
	for totalOff := 0; totalOff+200 <= bitLen; totalOff++ {
		v0, _ := bitFloat32(data, bitLen, totalOff)
		if !sameFloat(v0, maxHP) {
			continue
		}
		v1, _ := bitFloat32(data, bitLen, totalOff+40)
		v2, _ := bitFloat32(data, bitLen, totalOff+80)
		if !sameFloat(v1, maxHP) || !sameFloat(v2, maxHP) {
			continue
		}
		finalAttack, _ := bitFloat32(data, bitLen, totalOff+120)
		endurance, _ := bitFloat32(data, bitLen, totalOff+160)
		if !validPanelFloat(finalAttack, 1, 100000) || !validPanelFloat(endurance, 1, 10000) {
			continue
		}
		for off := totalOff - 400; off >= max(8, totalOff-6000); off-- {
			stats, ok := readPanelAttributeBlock(data, bitLen, off, totalOff)
			if !ok {
				continue
			}
			stats.MaxHP = maxHP
			stats.Attack = finalAttack
			stats.Endurance = endurance
			return stats, true
		}
	}
	return characterPanelStats{}, false
}

func panelFieldHandle(data []byte, bitLen, off int) (byte, bool) {
	if off < 0 || off+8 > bitLen {
		return 0, false
	}
	var value byte
	for bit := 0; bit < 8; bit++ {
		value |= ((data[(off+bit)/8] >> uint((off+bit)%8)) & 1) << uint(bit)
	}
	return value, true
}

func validPanelFloat(value, minimum, maximum float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0) && value >= minimum && value <= maximum
}

func readPanelAttributeBlock(data []byte, bitLen, start, end int) (characterPanelStats, bool) {
	first, ok := panelFieldHandle(data, bitLen, start-8)
	if !ok || first != 2 {
		return characterPanelStats{}, false
	}
	var values [256]float32
	var present [256]bool
	previous := byte(0)
	for off := start; off+32 <= min(bitLen, end); off += 40 {
		handle, ok := panelFieldHandle(data, bitLen, off-8)
		if !ok || handle == 0 || handle&1 != 0 || handle <= previous {
			break
		}
		value, ok := bitFloat32(data, bitLen, off)
		if !ok || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return characterPanelStats{}, false
		}
		values[handle] = value
		present[handle] = true
		previous = handle
	}
	// Encoded field handles are protocol identifiers, not offsets. In the
	// observed attribute schema each pair is base/current: HP 2/4, cycle 30/32,
	// ATK 42/44, CRIT 46/48, CRIT DMG 62/64, charge 70/72, general DMG 82/84,
	// DEF 194/196, DEF percent 198/200 and flat DEF 202/204.
	for _, handle := range []byte{2, 4, 42, 44, 48, 64, 72, 84, 194, 196} {
		if !present[handle] {
			return characterPanelStats{}, false
		}
	}
	if !sameFloat(values[2], values[4]) || !sameFloat(values[42], values[44]) || !sameFloat(values[194], values[196]) {
		return characterPanelStats{}, false
	}
	if !validPanelFloat(values[2], 1, 1000000) || !validPanelFloat(values[44], 1, 100000) || !validPanelFloat(values[196], 1, 100000) || !validPanelFloat(values[48], 0, 5) || !validPanelFloat(values[64], .1, 10) || !validPanelFloat(values[72], .1, 10) || !validPanelFloat(values[84], 0, 10) || !validPanelFloat(values[200], 0, 10) || !validPanelFloat(values[204], 0, 100000) || !validPanelFloat(values[32], 0, 100000) {
		return characterPanelStats{}, false
	}
	if (present[30] && !present[32]) || (present[198] && !present[200]) || (present[202] && !present[204]) {
		return characterPanelStats{}, false
	}
	stats := characterPanelStats{BaseHP: values[4], BaseAttack: values[44], BaseDefense: values[196], CritRate: values[48], CritDamage: values[64], ChargeEfficiency: values[72], CycleIntensity: values[32], UniversalDMGBonus: values[84], Defense: values[196] + float32(math.Floor(float64(values[196]*values[200]+values[204])+1e-6))}
	stats.HasCycleIntensity = present[32]
	// Multiple elemental attributes can be present simultaneously. The current
	// scalar export cannot identify which applies to the character, so never
	// pick one by its serialized position. Report a scalar only when the
	// observed current elemental values agree.
	elementalSet := false
	// These three current-value field handles are directly observed in the
	// captured elemental block. Other unknown fields are not interpreted.
	for _, handle := range []byte{108, 132, 140} {
		if !present[handle] {
			continue
		}
		value := values[handle]
		if !validPanelFloat(value, 0, 10) {
			return characterPanelStats{}, false
		}
		if elementalSet && !sameFloat(stats.ElementalDMGBonus, value) {
			stats.ElementalDMGBonus = 0
			stats.HasElementalDMGBonus = false
			break
		}
		stats.ElementalDMGBonus = value
		stats.HasElementalDMGBonus = true
		elementalSet = true
	}
	return stats, true
}
func attachEquipmentPanelStats(characters []characterItem, items []inventoryItem) {
	byNetID := make(map[itemNetID]*characterPanelStats, len(characters))
	for i := range characters {
		if characters[i].PanelStats != nil {
			byNetID[characters[i].NetID] = characters[i].PanelStats
		}
	}
	for _, item := range items {
		if item.CharacterNetID == nil {
			continue
		}
		stats := byNetID[*item.CharacterNetID]
		if stats == nil {
			continue
		}
		subStats := item.SubStats
		if item.Kind == "module" && item.Level == 0 {
			subStats = nil
		}
		for _, stat := range append(append([]inventoryStat(nil), item.MainStats...), subStats...) {
			if stat.Property == "UnbalIntensityBase" {
				stats.BreakIntensity += stat.Value
			}
		}
	}
}

type observedEquipmentBuff struct {
	EquipmentID uint32 `json:"equipmentId"`
	Buff        string `json:"buff"`
	SetID       string `json:"setId,omitempty"`
	SetName     string `json:"setName,omitempty"`
	Pieces      int    `json:"pieces,omitempty"`
	Effect      string `json:"effect,omitempty"`
}
type observedWeaponBuff struct {
	ForkID string `json:"forkId"`
	Buff   string `json:"buff"`
	Star   int    `json:"star"`
}

var activeAwakeningExport = regexp.MustCompile(`/Ability_(\d{3})_[^/]+/Upgrade/Level([1-6])/`)
var loadedCharacterExport = regexp.MustCompile(`(?i)/Character/Player/player_(\d{3})_[^/]+$`)
var activeEquipmentBuffExport = regexp.MustCompile(`/Equipment/Equipment_(\d+)/Buff_Equipment_([^/]+)$`)
var activeWeaponBuffExport = regexp.MustCompile(`/Abilities/(?:Buff/)?Fork/(Fork_[^/]+)/(Buff_Fork_[^/]+_(?:Lv(\d+)|(\d+)_([0-9]+)))$`)

// attachObservedActiveAwakenings associates instantiated awakening buff paths
// with their character. This is capture-dependent: an absent path does not mean
// that the character has no selected awakening effect.
func attachObservedActiveAwakenings(characters []characterItem, exportNames []string, catalog *equipmentCatalog) {
	bySuffix := make(map[uint32]map[int32]bool)
	awakeningBuffs := make(map[uint32][]string)
	loaded := make(map[uint32]bool)
	equipmentBuffs := make(map[uint32][]observedEquipmentBuff)
	weaponBuffs := make(map[uint32][]observedWeaponBuff)
	var currentCharacter uint32
	for _, name := range exportNames {
		if match := loadedCharacterExport.FindStringSubmatch(name); match != nil {
			if id, err := strconv.ParseUint(match[1], 10, 32); err == nil {
				currentCharacter = uint32(id)
				loaded[currentCharacter] = true
			}
		}
		if match := activeEquipmentBuffExport.FindStringSubmatch(name); match != nil && currentCharacter != 0 {
			if strings.HasSuffix(match[2], "_Effect") {
				continue
			}
			if id, err := strconv.ParseUint(match[1], 10, 32); err == nil {
				candidate := observedEquipmentBuff{EquipmentID: uint32(id), Buff: match[2]}
				enrichObservedEquipmentBuff(&candidate, catalog)
				seen := false
				for _, existing := range equipmentBuffs[currentCharacter] {
					seen = seen || existing == candidate
				}
				if !seen {
					equipmentBuffs[currentCharacter] = append(equipmentBuffs[currentCharacter], candidate)
				}
			}
		}
		if match := activeWeaponBuffExport.FindStringSubmatch(name); match != nil && currentCharacter != 0 {
			starText := match[3]
			if starText == "" {
				starText = match[5]
			}
			if star, err := strconv.Atoi(starText); err == nil {
				candidate := observedWeaponBuff{ForkID: strings.ToLower(match[1]), Buff: match[2], Star: star}
				seen := false
				for _, existing := range weaponBuffs[currentCharacter] {
					seen = seen || existing == candidate
				}
				if !seen {
					weaponBuffs[currentCharacter] = append(weaponBuffs[currentCharacter], candidate)
				}
			}
		}
		match := activeAwakeningExport.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		id, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil {
			continue
		}
		level, err := strconv.ParseInt(match[2], 10, 32)
		if err != nil {
			continue
		}
		if bySuffix[uint32(id)] == nil {
			bySuffix[uint32(id)] = make(map[int32]bool)
		}
		suffix := uint32(id)
		bySuffix[suffix][int32(level)] = true
		buffName := name[strings.LastIndex(name, "/")+1:]
		seenBuff := false
		for _, existing := range awakeningBuffs[suffix] {
			seenBuff = seenBuff || existing == buffName
		}
		if !seenBuff {
			awakeningBuffs[suffix] = append(awakeningBuffs[suffix], buffName)
		}
	}
	for i := range characters {
		suffix := characters[i].CharacterID % 1000
		characters[i].ObservedLoadedAtLogin = loaded[suffix]
		characters[i].ObservedActiveAwakeningBuffs = awakeningBuffs[suffix]
		sort.Strings(characters[i].ObservedActiveAwakeningBuffs)
		characters[i].ObservedActiveEquipmentBuffs = equipmentBuffs[suffix]
		characters[i].ObservedActiveWeaponBuffs = weaponBuffs[suffix]
		levels := bySuffix[suffix]
		for level := range levels {
			characters[i].ObservedActiveAwakeningLevels = append(characters[i].ObservedActiveAwakeningLevels, level)
		}
		sort.Slice(characters[i].ObservedActiveAwakeningLevels, func(a, b int) bool {
			return characters[i].ObservedActiveAwakeningLevels[a] < characters[i].ObservedActiveAwakeningLevels[b]
		})
	}
}

func enrichObservedEquipmentBuff(buff *observedEquipmentBuff, catalog *equipmentCatalog) {
	if catalog == nil {
		return
	}
	parts := strings.Split(buff.Buff, "_")
	if len(parts) < 2 {
		return
	}
	pieces, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return
	}
	prefix := strings.Join(parts[:len(parts)-1], "_")
	for strings.HasSuffix(prefix, "2") {
		prefix = strings.TrimSuffix(prefix, "2")
	}
	for itemID, definition := range catalog.Items {
		if definition.Kind != "core" || !strings.HasPrefix(itemID, prefix+"_") || definition.Suit == "" {
			continue
		}
		suit, ok := catalog.Suits[definition.Suit]
		if !ok {
			continue
		}
		buff.SetID = definition.Suit
		buff.SetName = suit.NameEN
		buff.Pieces = pieces
		for _, effect := range suit.Effects {
			if effect.Count == pieces {
				buff.Effect = effect.TextEN
				break
			}
		}
		return
	}
}

type characterSkillLevel struct {
	AbilityID string `json:"abilityId"`
	Category  string `json:"category"`
	Level     int32  `json:"level"`
}
type characterProbe struct {
	NetID          itemNetID             `json:"netId"`
	CharacterID    uint32                `json:"characterId"`
	Name           string                `json:"name"`
	Level          int32                 `json:"level"`
	Breakthrough   int32                 `json:"breakthroughLevel"`
	CurrentAwaken  int32                 `json:"decodedAwakenLevel"`
	RecordStartBit int                   `json:"recordStartBit"`
	AfterAwakenBit int                   `json:"afterAwakenBit"`
	RecordEndBit   int                   `json:"recordEndBit"`
	UnknownFloats  []float32             `json:"unknownFloats"`
	UnknownFlag    bool                  `json:"unknownFlag"`
	UnknownInt32   int32                 `json:"unknownInt32"`
	UnknownUint16  uint16                `json:"unknownUint16"`
	TailBits       int                   `json:"tailBits"`
	TailHex        string                `json:"tailHex,omitempty"`
	ForkNetID      *itemNetID            `json:"forkNetId,omitempty"`
	SkillLevels    []characterSkillLevel `json:"skillLevels"`
}

func (r *invBits) packed() (uint32, bool) {
	var value uint32
	for shift := 0; shift <= 28; shift += 7 {
		more, ok := r.bits(1)
		if !ok {
			return 0, false
		}
		part, ok := r.bits(7)
		if !ok {
			return 0, false
		}
		value |= uint32(part) << uint(shift)
		if more == 0 {
			return value, true
		}
	}
	return 0, false
}

func (r *invBits) nameRef() (string, bool) {
	hard, ok := r.boolean()
	if !ok {
		return "", false
	}
	if hard {
		v, ok := r.packed()
		if !ok {
			return "", false
		}
		return fmt.Sprintf("#%d", v), true
	}
	s, ok := r.fstring(128)
	if !ok || s == "" {
		return "", false
	}
	n, ok := r.u32()
	return s, ok && n == 0
}

func loadCharacterCatalog(path string) (*characterCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c characterCatalog
	e = json.Unmarshal(b, &c)
	return &c, e
}
func parseCharacterAt(data []byte, bitLen, off int, c *characterCatalog) (characterItem, int, bool) {
	p, item, end, ok := parseCharacterProbeAt(data, bitLen, off, c)
	_ = p
	return item, end, ok
}

func parseCharacterProbeAt(data []byte, bitLen, off int, c *characterCatalog) (characterProbe, characterItem, int, bool) {
	r := invBits{data: data, bitLen: bitLen, cursor: off}
	s, o := r.dynamicName()
	if !o {
		return characterProbe{}, characterItem{}, 0, false
	}
	id64, e := strconv.ParseUint(s, 10, 32)
	if e != nil || id64 == 0 {
		return characterProbe{}, characterItem{}, 0, false
	}
	def, o := c.Characters[s]
	if !o {
		return characterProbe{}, characterItem{}, 0, false
	}
	net, o := r.netID()
	if !o || !validNetID(net) {
		return characterProbe{}, characterItem{}, 0, false
	}
	one, o := r.i64()
	if !o || one != 1 {
		return characterProbe{}, characterItem{}, 0, false
	}
	if _, o = r.i32(); !o {
		return characterProbe{}, characterItem{}, 0, false
	}
	created, o := r.i64()
	if !o || created < 0 {
		return characterProbe{}, characterItem{}, 0, false
	}
	// Current inventory records include an additional 64-bit header field.
	if _, o = r.i64(); !o {
		return characterProbe{}, characterItem{}, 0, false
	}
	arr, o := r.u16()
	if !o || arr != 1 {
		return characterProbe{}, characterItem{}, 0, false
	}
	level, o := r.i32()
	if !o || level < 1 || level > 100 {
		return characterProbe{}, characterItem{}, 0, false
	}
	br, o := r.i32()
	if !o || br < 0 || br > 100 {
		return characterProbe{}, characterItem{}, 0, false
	}
	fork, o := r.netID()
	if !o || ((fork.Solt != 0 || fork.Serial != 0) && !validNetID(fork)) {
		return characterProbe{}, characterItem{}, 0, false
	}
	aw, o := r.i32()
	if !o || aw < 0 || aw > 100 {
		return characterProbe{}, characterItem{}, 0, false
	}
	afterAwaken := r.cursor
	unknownFloats := make([]float32, 0, 7)
	for i := 0; i < 7; i++ {
		v, o := r.f32()
		if !o || v < 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return characterProbe{}, characterItem{}, 0, false
		}
		unknownFloats = append(unknownFloats, v)
	}
	unknownFlag, o := r.boolean()
	if !o {
		return characterProbe{}, characterItem{}, 0, false
	}
	n, o := r.i32()
	if !o || n < 0 {
		return characterProbe{}, characterItem{}, 0, false
	}
	n16, o := r.u16()
	if !o || n16 > 64 {
		return characterProbe{}, characterItem{}, 0, false
	}
	skills := make([]characterSkillLevel, 0, n16)
	for i := uint16(0); i < n16; i++ {
		ability, ok := r.nameRef()
		if !ok {
			return characterProbe{}, characterItem{}, 0, false
		}
		category, ok := r.nameRef()
		if !ok {
			return characterProbe{}, characterItem{}, 0, false
		}
		skillLevel, ok := r.i32()
		if !ok || skillLevel < 1 || skillLevel > 10 {
			return characterProbe{}, characterItem{}, 0, false
		}
		skills = append(skills, characterSkillLevel{AbilityID: ability, Category: category, Level: skillLevel})
	}
	var fp *itemNetID
	if fork.Solt != 0 || fork.Serial != 0 {
		x := fork
		fp = &x
	}
	item := characterItem{NetID: net, CharacterID: uint32(id64), Name: def.Codename, Codename: def.Codename, Level: level, BreakthroughLevel: br, AwakenLevel: aw, ForkNetID: fp, SkillLevels: skills, MaxHP: unknownFloats[1], SavedHealthRatio: unknownFloats[0]}
	tailBits := bitLen - r.cursor
	if tailBits > 8192 {
		tailBits = 8192
	}
	var tailHex string
	if tailBits > 0 {
		if tail, ok := extractBitRange(data, r.cursor, tailBits); ok {
			tailHex = hex.EncodeToString(tail)
		}
	}
	probe := characterProbe{NetID: net, CharacterID: uint32(id64), Name: def.Codename, Level: level, Breakthrough: br, CurrentAwaken: aw, RecordStartBit: off, AfterAwakenBit: afterAwaken, RecordEndBit: r.cursor, UnknownFloats: unknownFloats, UnknownFlag: unknownFlag, UnknownInt32: n, UnknownUint16: n16, TailBits: tailBits, TailHex: tailHex, ForkNetID: fp, SkillLevels: skills}
	return probe, item, r.cursor, true
}

func parseCharacterProbes(data []byte, bitLen int, c *characterCatalog) []characterProbe {
	m := map[itemNetID]characterProbe{}
	for off := 0; off < bitLen; off++ {
		p, _, _, ok := parseCharacterProbeAt(data, bitLen, off, c)
		if ok {
			m[p.NetID] = p
		}
	}
	out := make([]characterProbe, 0, len(m))
	for _, p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CharacterID < out[j].CharacterID })
	return out
}
func parseCharacters(data []byte, bitLen int, c *characterCatalog) []characterItem {
	m := map[itemNetID]characterItem{}
	for off := 0; off < bitLen; off++ {
		v, _, o := parseCharacterAt(data, bitLen, off, c)
		if o {
			m[v.NetID] = v
		}
	}
	out := make([]characterItem, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CharacterID == out[j].CharacterID {
			return out[i].NetID.Solt < out[j].NetID.Solt
		}
		return out[i].CharacterID < out[j].CharacterID
	})
	return out
}
