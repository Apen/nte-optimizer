package damage

import "math"

// CalculateDOT expands an atomic DOT tick using an explicit stack count and
// timing rule. It intentionally does not guess refresh or snapshot behaviour.
func CalculateDOT(characterLevel int, stats Stats, enemy Enemy, instance Instance, stacks int) Result {
	result := Calculate(characterLevel, stats, enemy, instance)
	if stacks <= 0 {
		stacks = 1
	}
	if instance.Timing.MaxStacks > 0 && stacks > instance.Timing.MaxStacks {
		stacks = instance.Timing.MaxStacks
	}
	ticks := instance.Timing.Ticks
	if ticks <= 0 && instance.Timing.IntervalSeconds > 0 && instance.Timing.DurationSeconds > 0 {
		ticks = int(math.Floor(instance.Timing.DurationSeconds/instance.Timing.IntervalSeconds + 1e-9))
	}
	if ticks <= 0 {
		ticks = 1
	}
	result.Ticks = ticks
	result.Stacks = stacks
	result.Total = result.Expected * float64(result.Hits*ticks*stacks)
	return result
}
