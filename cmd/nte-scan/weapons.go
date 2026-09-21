package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

type forkDefinition struct {
	Quality         string `json:"quality"`
	MaxBreakthrough int32  `json:"max_breakthrough"`
	MaxStar         int32  `json:"max_star"`
}
type forkCatalog struct {
	Forks map[string]forkDefinition `json:"forks"`
}
type weaponItem struct {
	ID                  itemNetID `json:"id"`
	ForkID              string    `json:"forkId"`
	Name                string    `json:"name,omitempty"`
	Quality             string    `json:"quality"`
	Level               int32     `json:"level"`
	Breakthrough        int32     `json:"breakthrough"`
	Star                int32     `json:"star"`
	EquippedCharacterID *uint32   `json:"equippedCharacterId,omitempty"`
}

func loadForkCatalog(path string) (*forkCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c forkCatalog
	e = json.Unmarshal(b, &c)
	return &c, e
}
func parseWeaponAt(data []byte, bitLen, off int, c *forkCatalog) (weaponItem, int, bool) {
	r := invBits{data: data, bitLen: bitLen, cursor: off}
	forkID, o := r.dynamicName()
	if !o {
		return weaponItem{}, 0, false
	}
	def, canonical, o := lookupFork(c, forkID)
	if !o {
		return weaponItem{}, 0, false
	}
	id, o := r.netID()
	if !o || !validNetID(id) {
		return weaponItem{}, 0, false
	}
	one, o := r.i64()
	if !o || one != 1 {
		return weaponItem{}, 0, false
	}
	if _, o = r.i32(); !o {
		return weaponItem{}, 0, false
	}
	created, o := r.i64()
	if !o || created < 0 {
		return weaponItem{}, 0, false
	}
	zero, o := r.u16()
	if !o || zero != 0 {
		return weaponItem{}, 0, false
	}
	one16, o := r.u16()
	if !o || one16 != 1 {
		return weaponItem{}, 0, false
	}
	level, o := r.i32()
	if !o || level < 1 || level > 80 {
		return weaponItem{}, 0, false
	}
	br, o := r.i32()
	if !o || br < 0 || br > def.MaxBreakthrough {
		return weaponItem{}, 0, false
	}
	star, o := r.i32()
	if !o || star < 1 || star > def.MaxStar {
		return weaponItem{}, 0, false
	}
	return weaponItem{ID: id, ForkID: canonical, Quality: def.Quality, Level: level, Breakthrough: br, Star: star}, r.cursor, true
}
func lookupFork(c *forkCatalog, id string) (forkDefinition, string, bool) {
	if d, o := c.Forks[id]; o {
		return d, id, true
	}
	for k, d := range c.Forks {
		if strings.EqualFold(k, id) {
			return d, k, true
		}
	}
	return forkDefinition{}, "", false
}
func parseWeapons(data []byte, bitLen int, c *forkCatalog) []weaponItem {
	m := map[itemNetID]weaponItem{}
	for off := 0; off < bitLen; off++ {
		v, _, o := parseWeaponAt(data, bitLen, off, c)
		if o {
			m[v.ID] = v
		}
	}
	out := make([]weaponItem, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ForkID == out[j].ForkID {
			return out[i].ID.Solt < out[j].ID.Solt
		}
		return out[i].ForkID < out[j].ForkID
	})
	return out
}
