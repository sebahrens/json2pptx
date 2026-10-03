//go:build !unix

package render

// processAlive cannot probe a pid portably here, so every profile is treated
// as owned until it passes staleProfileOrphanAge.
func processAlive(int) bool { return true }
