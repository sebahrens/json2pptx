package main

import (
	"regexp"
	"strings"
)

var headingNumberWord = regexp.MustCompile(`^[+−-]?[$€£]?[0-9]+(?:[.,][0-9]+)*$`)
var headingUnits = map[string]bool{"%": true, "percent": true, "pts": true, "points": true, "pp": true, "eur": true, "usd": true, "gbp": true, "million": true, "billion": true, "m": true, "bn": true, "weeks": true, "days": true, "hours": true, "months": true, "years": true, "kg": true, "km": true, "mw": true, "gw": true}

func headingOrphanReason(m *geomMeasurer, p geomParagraph, width float64) string {
	if m == nil || m.family == nil || width <= 0 || p.text == "" {
		return ""
	}
	var lines [][]string
	for _, part := range strings.Split(p.text, "\n") {
		words := strings.FieldsFunc(part, func(r rune) bool { return r == ' ' || r == '\t' || r == '\r' })
		var line []string
		for _, word := range words {
			trial := append(append([]string(nil), line...), word)
			if len(line) > 0 && m.widthPt(strings.Join(trial, " "), p.sizePt, p.bold) > width {
				lines = append(lines, line)
				line = nil
			}
			line = append(line, word)
		}
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	if len(lines) < 2 {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		previous, next := lines[i-1], lines[i]
		if headingNumberWord.MatchString(previous[len(previous)-1]) && headingUnits[strings.ToLower(strings.Trim(next[0], ".,:;()"))] {
			return "A heading line break separates a number from its unit"
		}
	}
	if len(strings.Fields(strings.Join(lines[len(lines)-1], " "))) == 1 {
		return "A heading ends with one word on its final line"
	}
	return ""
}
