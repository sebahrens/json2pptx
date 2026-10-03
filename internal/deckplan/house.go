package deckplan

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// House pillar count (go-slide-creator-qad87).
//
// The strategy-house skeleton is the pattern's exemplar with its text blanked,
// so every drafted house had the exemplar's pillar count whatever the brief
// said: a brief naming four themes was handed three columns, and the agent
// filling them merged or dropped one. The plan now reads the count from the
// brief — an enumerated list of pillars, or a stated number — and drafts that
// many pillars.

const (
	houseMinPillars = 3
	houseMaxPillars = 5
)

// cueHouseList captures the list that follows a named set of pillars or
// themes: "pillars: trust, speed, scale and reach".
var cueHouseList = regexp.MustCompile(`(?i)\b(?:pillars|themes)\b\s*(?:are|of|:|—|–|-|\()\s*([^.;:()]+)`)

// cueHouseCount captures a stated count: "four strategic pillars".
var cueHouseCount = regexp.MustCompile(`(?i)\b(three|four|five|[3-5])\s+(?:[a-z-]+\s+){0,2}?(?:pillars|themes)\b`)

// houseListSplit separates the items of an enumerated list.
var houseListSplit = regexp.MustCompile(`(?i)\s*,\s*(?:and\s+)?|\s+and\s+|\s*&\s*|\s*/\s*`)

var houseCountWords = map[string]int{"three": 3, "3": 3, "four": 4, "4": 4, "five": 5, "5": 5}

// housePillarCount returns the number of pillars the brief names, or 0 when
// it names none or more than a house holds. An enumerated list wins over a
// stated number: the list is what will be written into the pillars.
func housePillarCount(brief string) int {
	if m := cueHouseList.FindStringSubmatch(brief); m != nil {
		n := 0
		for _, item := range houseListSplit.Split(strings.TrimSpace(m[1]), -1) {
			if strings.TrimSpace(item) != "" {
				n++
			}
		}
		if n >= houseMinPillars && n <= houseMaxPillars {
			return n
		}
	}
	if m := cueHouseCount.FindStringSubmatch(brief); m != nil {
		return houseCountWords[strings.ToLower(m[1])]
	}
	return 0
}

// resizeHousePillars returns a strategy-house skeleton with exactly n
// pillars: the exemplar's first n, or the last one repeated. The skeleton is
// returned unchanged when it has no pillars list.
func resizeHousePillars(skel json.RawMessage, n int) json.RawMessage {
	if n < houseMinPillars || n > houseMaxPillars {
		return skel
	}
	var slide map[string]any
	if err := json.Unmarshal(skel, &slide); err != nil {
		return skel
	}
	pattern, _ := slide["pattern"].(map[string]any)
	values, _ := pattern["values"].(map[string]any)
	pillars, _ := values["pillars"].([]any)
	if len(pillars) == 0 || len(pillars) == n {
		return skel
	}
	for len(pillars) < n {
		// Each pillar is its own object: copy through JSON.
		raw, err := json.Marshal(pillars[len(pillars)-1])
		if err != nil {
			return skel
		}
		var clone any
		if err := json.Unmarshal(raw, &clone); err != nil {
			return skel
		}
		pillars = append(pillars, clone)
	}
	values["pillars"] = pillars[:n]
	out, err := json.Marshal(slide)
	if err != nil {
		return skel
	}
	return out
}

// pillarCountGuidance is the sentence a DeckSpec pillars slot gains when the
// brief names its themes.
func pillarCountGuidance(brief string) string {
	n := housePillarCount(brief)
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" The brief names %d themes: write %d pillars, one per theme.", n, n)
}
