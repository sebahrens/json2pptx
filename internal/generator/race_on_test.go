//go:build race

package generator

// raceSlowdown scales wall-clock budgets in timing regression tests: the race
// detector slows this CPU-bound code by roughly 5-10x.
const raceSlowdown = 10
