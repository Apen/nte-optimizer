package optimizer

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Placement struct {
	ModuleID string  `json:"module_id"`
	X        int     `json:"x"`
	Y        int     `json:"y"`
	Rotation int     `json:"rotation"`
	Cells    []Point `json:"cells"`
}
