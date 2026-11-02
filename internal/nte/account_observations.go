package nte

// FurnitureLevel is an observed progression level, not confirmed activation.
type FurnitureLevel struct {
	ProgressionID string `json:"progression_id"`
	Level         int    `json:"level"`
}
