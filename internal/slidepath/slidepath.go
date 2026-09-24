// Package slidepath constructs and parses JSON Pointer (RFC 6901) paths for
// fit findings, validation errors, and repair_slide targeting. All paths are
// rooted at "slides" and use "/" as the separator with numeric array indices.
//
// Example paths:
//
//	/slides/0/content/body                          — placeholder by name
//	/slides/0/content/0/placeholder_id              — content by array index
//	/slides/0/shape_grid/rows/1/cells/2             — shape grid cell
//	/slides/0/shape_grid/rows/1/cells/2/shape/fill  — cell shape fill
//	/slides/0/content/0/rows/3/1                    — table cell
package slidepath

import (
	"fmt"
	"strconv"
	"strings"
)

// Slide returns "/slides/{idx}".
func Slide(idx int) string {
	return fmt.Sprintf("/slides/%d", idx)
}

// Content returns "/slides/{slideIdx}/content/{placeholderID}".
func Content(slideIdx int, placeholderID string) string {
	return fmt.Sprintf("/slides/%d/content/%s", slideIdx, placeholderID)
}

// ContentIndex returns "/slides/{slideIdx}/content/{contentIdx}".
func ContentIndex(slideIdx, contentIdx int) string {
	return fmt.Sprintf("/slides/%d/content/%d", slideIdx, contentIdx)
}

// ContentField returns "/slides/{slideIdx}/content/{contentIdx}/{field}".
func ContentField(slideIdx, contentIdx int, field string) string {
	return fmt.Sprintf("/slides/%d/content/%d/%s", slideIdx, contentIdx, field)
}

// SlideField returns "/slides/{slideIdx}/{field}".
func SlideField(slideIdx int, field string) string {
	return fmt.Sprintf("/slides/%d/%s", slideIdx, field)
}

// ShapeGrid returns "/slides/{slideIdx}/shape_grid".
func ShapeGrid(slideIdx int) string {
	return fmt.Sprintf("/slides/%d/shape_grid", slideIdx)
}

// GridCell returns "/slides/{slideIdx}/shape_grid/rows/{rowIdx}/cells/{cellIdx}".
func GridCell(slideIdx, rowIdx, cellIdx int) string {
	return fmt.Sprintf("/slides/%d/shape_grid/rows/%d/cells/%d", slideIdx, rowIdx, cellIdx)
}

// GridCellField returns "/slides/{slideIdx}/shape_grid/rows/{rowIdx}/cells/{cellIdx}/{field}".
// For example, field could be "table", "shape", or "shape/fill".
func GridCellField(slideIdx, rowIdx, cellIdx int, field string) string {
	return fmt.Sprintf("/slides/%d/shape_grid/rows/%d/cells/%d/%s", slideIdx, rowIdx, cellIdx, field)
}

// GridRow returns "/slides/{slideIdx}/shape_grid/rows/{rowIdx}".
func GridRow(slideIdx, rowIdx int) string {
	return fmt.Sprintf("/slides/%d/shape_grid/rows/%d", slideIdx, rowIdx)
}

// GridRowRange returns "/slides/{slideIdx}/shape_grid/rows/{startIdx}:{endIdx}".
func GridRowRange(slideIdx, startIdx, endIdx int) string {
	return fmt.Sprintf("/slides/%d/shape_grid/rows/%d:%d", slideIdx, startIdx, endIdx)
}

// TableHeader returns "{prefix}/headers/{headerIdx}".
func TableHeader(prefix string, headerIdx int) string {
	return fmt.Sprintf("%s/headers/%d", prefix, headerIdx)
}

// TableCell returns "{prefix}/rows/{rowIdx}/{cellIdx}".
func TableCell(prefix string, rowIdx, cellIdx int) string {
	return fmt.Sprintf("%s/rows/%d/%d", prefix, rowIdx, cellIdx)
}

// Join appends a suffix to a path with "/" separator.
func Join(prefix, suffix string) string {
	return prefix + "/" + suffix
}

// Field appends an svggen-style dotted/bracket field reference as JSON Pointer
// segments. An unindexed array wildcard (series[]) identifies only the array,
// so the result stops there rather than inventing an unaddressable element.
func Field(prefix, field string) string {
	if field == "" {
		return prefix
	}
	path := prefix
	for i := 0; i < len(field); {
		if field[i] == '.' {
			i++
			continue
		}
		if field[i] == '[' {
			end := strings.IndexByte(field[i:], ']')
			if end <= 1 {
				return path
			}
			index := field[i+1 : i+end]
			if _, err := strconv.Atoi(index); err != nil {
				return path
			}
			path += "/" + index
			i += end + 1
			continue
		}
		start := i
		for i < len(field) && field[i] != '.' && field[i] != '[' {
			i++
		}
		token := strings.ReplaceAll(strings.ReplaceAll(field[start:i], "~", "~0"), "/", "~1")
		path += "/" + token
	}
	return path
}

// SlideIndex extracts the slide index from a JSON Pointer path.
// Returns -1 if the path doesn't start with "/slides/{N}".
func SlideIndex(path string) int {
	if !strings.HasPrefix(path, "/slides/") {
		return -1
	}
	rest := path[len("/slides/"):]
	slash := strings.IndexByte(rest, '/')
	numStr := rest
	if slash >= 0 {
		numStr = rest[:slash]
	}
	idx, err := strconv.Atoi(numStr)
	if err != nil {
		return -1
	}
	return idx
}

// HasPrefix reports whether path starts with prefix. Both must be JSON Pointers.
func HasPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	if path[:len(prefix)] != prefix {
		return false
	}
	// Ensure the prefix ends at a segment boundary.
	if len(path) > len(prefix) && path[len(prefix)] != '/' {
		return false
	}
	return true
}

// ParseGridCell extracts slide, row, and cell indices from a grid cell path.
// Returns ok=false if the path doesn't match the expected format.
func ParseGridCell(path string) (slideIdx, rowIdx, cellIdx int, ok bool) {
	if n, _ := fmt.Sscanf(path, "/slides/%d/shape_grid/rows/%d/cells/%d", &slideIdx, &rowIdx, &cellIdx); n >= 3 {
		return slideIdx, rowIdx, cellIdx, true
	}
	// Findings on a pattern slide are rooted at /slides/N/pattern/... because the
	// deck has no shape_grid at that index (go-slide-creator-adur); the two
	// spellings address the same cell of the expanded grid.
	n, _ := fmt.Sscanf(path, "/slides/%d/pattern/rows/%d/cells/%d", &slideIdx, &rowIdx, &cellIdx)
	return slideIdx, rowIdx, cellIdx, n >= 3
}
