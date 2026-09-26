// split_slide.go implements the split_slide declarative envelope.
// A split_slide entry in the slides array expands into N regular slides
// by windowing table data across pages. This is a declarative primitive
// for the ONE pattern that's painful to express as sibling slides:
// large tables where each page needs identical chrome (title, headers, footer).
//
// Only "table.rows" splitting is supported. This is NOT a general workflow
// primitive — agents authoring heterogeneous multi-slide narratives must
// write sibling slides directly.
package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// SplitSlideInput and SplitConfig are defined in internal/deckinput; aliased
// here so package-main call sites (mcp_repair.go, tests) are unchanged. The
// expansion logic (expandSplitSlide) stays in this package and is registered
// with deckinput via init() so PresentationInput.UnmarshalJSON can expand
// split_slide entries.
type SplitSlideInput = deckinput.SplitSlideInput

// SplitConfig controls how the base slide's table data is windowed.
type SplitConfig = deckinput.SplitConfig

func init() {
	deckinput.SplitSlideExpander = expandSplitSlide
}

// expandSplitSlide validates and expands a SplitSlideInput into N regular SlideInputs.
func expandSplitSlide(s SplitSlideInput) ([]SlideInput, error) {
	tableIdx, _ := findTableContent(s.Base.Content)
	return expandSplitSlideAtTable(s, tableIdx)
}

// expandSplitSlideAtTable is shared by the declarative first-table envelope and
// path-targeted repair. A selected second table must not silently become first.
func expandSplitSlideAtTable(s SplitSlideInput, tableIdx int) ([]SlideInput, error) {
	if err := validateSplitSlide(s); err != nil {
		return nil, err
	}

	if tableIdx < 0 || tableIdx >= len(s.Base.Content) {
		return nil, fmt.Errorf("split_slide: selected table content index is invalid")
	}
	_, table := findTableContent(s.Base.Content[tableIdx : tableIdx+1])
	if table == nil {
		return nil, fmt.Errorf("split_slide: selected content item must contain a valid table")
	}

	rows := table.Rows
	groupSize := s.Split.GroupSize

	// group_size >= rows → one slide, no suffix
	if groupSize >= len(rows) {
		return []SlideInput{s.Base}, nil
	}

	if err := validateRowSpansAtBoundaries(rows, groupSize); err != nil {
		return nil, fmt.Errorf("split_slide: %w", err)
	}

	// Split rows into chunks
	var chunks [][][]TableCellInput
	for i := 0; i < len(rows); i += groupSize {
		end := i + groupSize
		if end > len(rows) {
			end = len(rows)
		}
		chunks = append(chunks, rows[i:end])
	}

	total := len(chunks)
	slides := make([]SlideInput, total)

	for i, chunk := range chunks {
		// Retain authored composition and internal layout/section context;
		// partial literals silently discarded fields whenever the schema grew.
		slide := s.Base

		// First page gets speaker notes and source
		if i > 0 {
			slide.SpeakerNotes = ""
			slide.Source = ""
			slide.SourceLink = nil
		}

		// Build content — replace table rows with this chunk
		slide.Content = make([]ContentInput, len(s.Base.Content))
		for j, ci := range s.Base.Content {
			if j == tableIdx {
				newTable := *table
				newTable.Rows = make([][]TableCellInput, len(chunk))
				for ri, row := range chunk {
					newTable.Rows[ri] = append([]TableCellInput(nil), row...)
				}
				if !s.Split.RepeatHeaders && i > 0 {
					newTable.Headers = nil
				}
				newCi := ci
				newCi.TableValue = &newTable
				newCi.Value = nil // clear legacy field
				slide.Content[j] = newCi
			} else {
				slide.Content[j] = ci
			}
		}

		// Apply title suffix to title content item
		if total > 1 && s.Split.TitleSuffix != "" {
			applySplitTitleSuffix(slide.Content, s.Split.TitleSuffix, i+1, total)
		}

		slides[i] = slide
	}

	return slides, nil
}

// validateSplitSlide checks split_slide constraints at parse time.
func validateSplitSlide(s SplitSlideInput) error {
	if s.Split.By != "table.rows" {
		return fmt.Errorf("split_slide: split.by must be 'table.rows', got %q", s.Split.By)
	}
	if s.Split.GroupSize <= 0 {
		return fmt.Errorf("split_slide: group_size must be > 0, got %d", s.Split.GroupSize)
	}
	tableIdx, _ := findTableContent(s.Base.Content)
	if tableIdx < 0 {
		return fmt.Errorf("split_slide: base must contain a table content item")
	}
	if !s.Split.RepeatHeaders {
		return fmt.Errorf("split_slide: repeat_headers=false is unsupported because PowerPoint tables require column headers; set repeat_headers=true")
	}

	// Reject if base contains both table and chart/diagram
	for _, ci := range s.Base.Content {
		if ci.Type == "chart" || ci.Type == "diagram" {
			return fmt.Errorf("split_slide: base cannot contain both a table and a %s", ci.Type)
		}
	}

	return nil
}

// findTableContent returns the index and pointer to the first table content item.
func findTableContent(content []ContentInput) (int, *TableInput) {
	for i := range content {
		if content[i].Type != "table" {
			continue
		}
		if content[i].TableValue != nil {
			return i, content[i].TableValue
		}
		// Try resolving from legacy Value field
		v, err := content[i].ResolveValue()
		if err == nil {
			if t, ok := v.(*TableInput); ok {
				return i, t
			}
		}
	}
	return -1, nil
}

// validateRowSpansAtBoundaries checks that no cell's row_span crosses a split boundary.
func validateRowSpansAtBoundaries(rows [][]TableCellInput, groupSize int) error {
	for boundary := groupSize; boundary < len(rows); boundary += groupSize {
		for r := 0; r < boundary; r++ {
			for _, cell := range rows[r] {
				if cell.RowSpan > 1 && r+cell.RowSpan > boundary {
					return fmt.Errorf(
						"row %d has a cell with row_span=%d that crosses split boundary at row %d",
						r+1, cell.RowSpan, boundary,
					)
				}
			}
		}
	}
	return nil
}

// applySplitTitleSuffix modifies the title content item with page/total substitution.
func applySplitTitleSuffix(content []ContentInput, suffix string, page, total int) {
	resolved := strings.ReplaceAll(suffix, "{page}", fmt.Sprintf("%d", page))
	resolved = strings.ReplaceAll(resolved, "{total}", fmt.Sprintf("%d", total))

	for i := range content {
		if content[i].PlaceholderID == "title" && content[i].Type == "text" && content[i].TextValue != nil {
			newVal := *content[i].TextValue + resolved
			content[i].TextValue = &newVal
			return
		}
	}
}
