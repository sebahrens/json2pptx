package generator

import (
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Two-column header lines (go-slide-creator-30471).
//
// A two-column comparison is often authored as two bullet lists whose first
// item names the column ("Open-weight models", "Frontier closed models").
// Rendered as ordinary bullets, the column names read as the first point of
// each list. When BOTH side-by-side columns open with such a line, that line
// renders as a column header: bold, no bullet, flush left, 6pt space after.
// Requiring both columns keeps a list that merely starts with a short point
// unchanged.

// columnHeaderBullets is a bullets value whose first line renders as a header.
type columnHeaderBullets struct {
	Header  string
	Bullets []string
}

// columnHeaderSpaceAfter is the gap between a column header and its bullets.
const columnHeaderSpaceAfter = `<a:spcAft><a:spcPts val="600"/></a:spcAft>`

// markColumnHeaders returns content with the body / body_2 bullet lists
// rewritten to columnHeaderBullets when the two placeholders sit side by side
// and each list opens with a header line. content itself is not modified.
func markColumnHeaders(shapes []shapeXML, content []ContentItem) []ContentItem {
	indices := buildPlaceholderMap(shapes)
	leftIdx, hasLeft := indices["body"]
	rightIdx, hasRight := indices["body_2"]
	if !hasLeft || !hasRight || !sideBySideBodies(&shapes[leftIdx], &shapes[rightIdx]) {
		return content
	}
	items := map[string]int{}
	for i, item := range content {
		id := strings.ToLower(item.PlaceholderID)
		if (id == "body" || id == "body_2") && item.Type == ContentBullets {
			if _, dup := items[id]; dup {
				return content
			}
			items[id] = i
		}
	}
	li, lok := items["body"]
	ri, rok := items["body_2"]
	if !lok || !rok {
		return content
	}
	left, lok := content[li].Value.([]string)
	right, rok := content[ri].Value.([]string)
	if !lok || !rok || !isColumnHeaderLine(left) || !isColumnHeaderLine(right) {
		return content
	}
	out := append([]ContentItem(nil), content...)
	out[li].Value = columnHeaderBullets{Header: strings.TrimSpace(left[0]), Bullets: left[1:]}
	out[ri].Value = columnHeaderBullets{Header: strings.TrimSpace(right[0]), Bullets: right[1:]}
	return out
}

// isColumnHeaderLine reports whether a bullet list opens with a header line: a
// short, unindented, unpunctuated label followed by at least two points that
// are longer on average. A trailing colon marks a header outright.
func isColumnHeaderLine(bullets []string) bool {
	if len(bullets) < 3 {
		return false
	}
	depth, first := pptx.BulletIndentDepth(bullets[0])
	first = strings.TrimSpace(first)
	if depth != 0 || first == "" || len([]rune(first)) > 48 {
		return false
	}
	if _, numbered := NumberedList(bullets[:2]); numbered {
		return false
	}
	words := len(strings.Fields(first))
	if strings.HasSuffix(first, ":") {
		return words <= 8
	}
	last := []rune(first)[len([]rune(first))-1]
	if words > 6 || unicode.IsPunct(last) && last != ')' {
		return false
	}
	restWords := 0
	for _, b := range bullets[1:] {
		restWords += len(strings.Fields(b))
	}
	mean := float64(restWords) / float64(len(bullets)-1)
	return mean >= 3 && float64(words) < mean
}

// columnHeaderParagraph renders a column header at the list's bullet level so
// it inherits the bullets' size and font, with the bullet glyph removed.
func columnHeaderParagraph(header string, templateStyles []bulletLevelStyle, bulletLevel int) paragraphXML {
	pProps, rProps := getBulletStyleForLevel(templateStyles, bulletLevel)
	props := *pProps
	zero := 0
	props.MarL, props.Indent = &zero, &zero
	props.Inner = columnHeaderSpaceAfter + `<a:buNone/>`
	runs := createFormattedRuns(header, rProps)
	for i := range runs {
		if runs[i].RunProperties == nil {
			runs[i].RunProperties = &runPropertiesXML{Lang: "en-US"}
		}
		runs[i].RunProperties.Bold = "1"
	}
	return paragraphXML{Properties: &props, Runs: runs}
}
