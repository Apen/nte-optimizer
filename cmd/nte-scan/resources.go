package main

import (
	"encoding/json"
	"os"
	"sort"
)

type resourceCatalog struct {
	Items map[string]struct{} `json:"items"`
}
type resourceItem struct {
	ID       itemNetID `json:"id"`
	ItemID   string    `json:"itemId"`
	Quantity int64     `json:"quantity"`
}

func loadResourceCatalog(path string) (*resourceCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c resourceCatalog
	e = json.Unmarshal(b, &c)
	return &c, e
}
func parseResourceAt(data []byte, bitLen, off int, c *resourceCatalog) (resourceItem, bool) {
	r := invBits{data: data, bitLen: bitLen, cursor: off}
	itemID, o := r.dynamicName()
	if !o {
		return resourceItem{}, false
	}
	_, o = c.Items[itemID]
	if !o {
		return resourceItem{}, false
	}
	id, o := r.netID()
	if !o || !validNetID(id) {
		return resourceItem{}, false
	}
	qty, o := r.i64()
	if !o || qty <= 0 || qty > 1_000_000_000_000 {
		return resourceItem{}, false
	}
	if _, o = r.i32(); !o {
		return resourceItem{}, false
	}
	created, o := r.i64()
	if !o || created < 0 {
		return resourceItem{}, false
	}
	return resourceItem{ID: id, ItemID: itemID, Quantity: qty}, true
}
func parseResources(data []byte, bitLen int, c *resourceCatalog) []resourceItem {
	m := map[itemNetID]resourceItem{}
	for off := 0; off < bitLen; off++ {
		v, o := parseResourceAt(data, bitLen, off, c)
		if o {
			m[v.ID] = v
		}
	}
	out := make([]resourceItem, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}
