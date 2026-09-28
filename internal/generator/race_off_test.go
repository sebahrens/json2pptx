//go:build !race

package generator

// raceSlowdown scales wall-clock budgets in timing regression tests; 1
// without the race detector.
const raceSlowdown = 1
