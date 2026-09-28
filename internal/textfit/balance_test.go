package textfit

import (
	"strings"
	"testing"
)

// monoLines wraps text greedily with every rune (space included) 10000 EMU wide.
func monoLines(text string, width int64) int {
	const runeEMU = 10000
	lines, cur := 1, int64(0)
	for _, w := range strings.Fields(text) {
		need := int64(len([]rune(w))) * runeEMU
		if cur > 0 {
			need += runeEMU
		}
		if cur > 0 && cur+need > width {
			lines++
			cur = int64(len([]rune(w))) * runeEMU
			continue
		}
		cur += need
	}
	return lines
}

func TestBalancedMarginEMU(t *testing.T) {
	const text = "Governance and data readiness now gate value" // 44 runes
	// 40 runes wide: "Governance and data readiness now gate" / "value".
	width := int64(40 * 10000)
	margin := BalancedMarginEMU(monoLines, text, width)
	if margin <= 0 {
		t.Fatal("widow was not balanced")
	}
	if got := monoLines(text, width-margin); got != 2 {
		t.Errorf("balanced text wraps to %d lines, want 2", got)
	}
	if monoLines("Governance and data readiness now gate", width-margin) != 2 {
		t.Error("last line still holds one word")
	}
	// No widow: the last line already has several words.
	if m := BalancedMarginEMU(monoLines, text, int64(30*10000)); m != 0 {
		t.Errorf("no-widow text got margin %d", m)
	}
	// One line: nothing to balance.
	if m := BalancedMarginEMU(monoLines, text, int64(60*10000)); m != 0 {
		t.Errorf("one-line text got margin %d", m)
	}
}
