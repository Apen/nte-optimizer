// Package datafiles centralizes the versioned data layout shared by the app,
// scanner and maintenance commands.
package datafiles

import "path/filepath"

// Layout resolves files below a data root without leaking directory knowledge
// into business packages.
type Layout struct {
	Root string
}

func New(root string) Layout { return Layout{Root: root} }

func (l Layout) Game(parts ...string) string {
	return filepath.Join(append([]string{l.Root, "game"}, parts...)...)
}

func (l Layout) Optimizer(parts ...string) string {
	return filepath.Join(append([]string{l.Root, "optimizer"}, parts...)...)
}

func (l Layout) Recommendation(parts ...string) string {
	return filepath.Join(append([]string{l.Root, "recommendations"}, parts...)...)
}

func (l Layout) Presentation(parts ...string) string {
	return filepath.Join(append([]string{l.Root, "presentation"}, parts...)...)
}

func (l Layout) CharacterBaseStats() string { return l.Game("characters", "base_stats.json") }
func (l Layout) Damage() string             { return l.Game("combat", "damage.json") }
func (l Layout) Arcs() string               { return l.Game("equipment", "arcs.json") }
func (l Layout) ConsoleTraits() string      { return l.Game("equipment", "console_traits.json") }
func (l Layout) Grids() string              { return l.Game("equipment", "grids.json") }
func (l Layout) Sets() string               { return l.Game("equipment", "sets.json") }
func (l Layout) Shapes() string             { return l.Game("equipment", "shapes.json") }
func (l Layout) References() string         { return l.Optimizer("references.json") }
func (l Layout) Config() string             { return l.Optimizer("config.json") }
func (l Layout) Target(id string) string    { return l.Recommendation("targets", id+".json") }
func (l Layout) TargetsDir() string         { return l.Recommendation("targets") }
func (l Layout) DecodeCatalog(name string) string {
	switch name {
	case "characters.json":
		return l.Game("characters", "decode.json")
	case "equipment.json":
		return l.Game("equipment", "decode.json")
	case "forks.json":
		return l.Game("forks", "decode.json")
	case "resources.json":
		return l.Game("resources", "decode.json")
	default:
		return l.Game(name)
	}
}
func (l Layout) Locale(language string) string {
	return l.Presentation(language + ".json")
}

func (l Layout) GameLocale(language string) string {
	return l.Game("locales", language+".json")
}
