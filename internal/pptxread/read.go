// Package pptxread provides best-effort extraction of slide content from a PPTX file.
// It reads slide XML, resolves placeholders, extracts text runs, tables, and shape
// metadata to produce a structured JSON-friendly representation.
package pptxread

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Presentation is the top-level result of reading a PPTX file.
type Presentation struct {
	SlideCount int     `json:"slide_count"`
	Slides     []Slide `json:"slides"`
}

// Slide represents a single slide extracted from the PPTX.
type Slide struct {
	Index        int           `json:"index"`
	LayoutID     string        `json:"layout_id,omitempty"`
	Placeholders []Placeholder `json:"placeholders,omitempty"`
	Shapes       []Shape       `json:"shapes,omitempty"`
	Tables       []Table       `json:"tables,omitempty"`
	SpeakerNotes string        `json:"speaker_notes,omitempty"`
}

// Placeholder represents a populated placeholder on a slide.
type Placeholder struct {
	ID     string `json:"id"`
	Type   string `json:"type,omitempty"`
	Text   string `json:"text"`
	Bounds *Rect  `json:"bounds,omitempty"`
}

// Shape represents a non-placeholder shape on a slide.
type Shape struct {
	Name     string `json:"name,omitempty"`
	Geometry string `json:"geometry,omitempty"`
	Text     string `json:"text,omitempty"`
	Bounds   *Rect  `json:"bounds,omitempty"`
}

// Table represents a table found on a slide.
type Table struct {
	Name    string     `json:"name,omitempty"`
	Rows    int        `json:"rows"`
	Cols    int        `json:"cols"`
	Headers []string   `json:"headers,omitempty"`
	Data    [][]string `json:"data,omitempty"`
	Bounds  *Rect      `json:"bounds,omitempty"`
}

// Rect represents position and size in EMU (English Metric Units).
type Rect struct {
	X      int64 `json:"x"`
	Y      int64 `json:"y"`
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

// ReadFile reads a PPTX file and returns the extracted presentation structure.
func ReadFile(path string) (*Presentation, error) {
	pkg, closer, err := pptx.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open pptx: %w", err)
	}
	defer closer.Close()

	return ReadPackage(pkg)
}

// ReadPackage reads slide content from an already-opened PPTX package.
func ReadPackage(pkg *pptx.Package) (*Presentation, error) {
	enum, err := pptx.NewSlideEnumerator(pkg)
	if err != nil {
		return nil, fmt.Errorf("enumerate slides: %w", err)
	}

	result := &Presentation{
		SlideCount: enum.Count(),
		Slides:     make([]Slide, 0, enum.Count()),
	}

	for _, info := range enum.Slides() {
		slide, err := readSlide(pkg, info)
		if err != nil {
			// Best-effort: include what we can, skip broken slides.
			result.Slides = append(result.Slides, Slide{Index: info.Index})
			continue
		}
		result.Slides = append(result.Slides, *slide)
	}

	return result, nil
}

// readSlide extracts content from a single slide.
func readSlide(pkg *pptx.Package, info pptx.SlideInfo) (*Slide, error) {
	slideData, err := pkg.ReadEntry(info.PartPath)
	if err != nil {
		return nil, fmt.Errorf("read slide %d: %w", info.Index, err)
	}

	slide := &Slide{Index: info.Index}

	// Resolve layout ID from slide relationships.
	slide.LayoutID = resolveLayoutID(pkg, info.PartPath)

	// Parse slide XML.
	var sld slideDocument
	if err := xml.Unmarshal(slideData, &sld); err != nil {
		return nil, fmt.Errorf("parse slide %d XML: %w", info.Index, err)
	}

	// Extract shapes (sp elements).
	for _, sp := range sld.CSld.SpTree.Shapes {
		ph := sp.NvSpPr.NvPr.Placeholder
		text := extractText(sp.TxBody)

		if ph != nil {
			// This is a placeholder.
			phID := placeholderID(sp.NvSpPr.CNvPr.Name, ph)
			p := Placeholder{
				ID:   phID,
				Type: ph.Type,
				Text: text,
			}
			if sp.SpPr.Xfrm != nil {
				p.Bounds = xfrmToRect(sp.SpPr.Xfrm)
			}
			slide.Placeholders = append(slide.Placeholders, p)
		} else if text != "" || sp.SpPr.PrstGeom != nil {
			// Non-placeholder shape with text or geometry.
			s := Shape{
				Name: sp.NvSpPr.CNvPr.Name,
				Text: text,
			}
			if sp.SpPr.PrstGeom != nil {
				s.Geometry = sp.SpPr.PrstGeom.Prst
			}
			if sp.SpPr.Xfrm != nil {
				s.Bounds = xfrmToRect(sp.SpPr.Xfrm)
			}
			slide.Shapes = append(slide.Shapes, s)
		}
	}

	// Extract tables (graphicFrame elements with tbl).
	for _, gf := range sld.CSld.SpTree.GraphicFrames {
		tbl := gf.Graphic.GraphicData.Table
		if tbl == nil {
			continue
		}
		t := extractTable(gf.NvGraphicFramePr.CNvPr.Name, tbl, gf.Xfrm)
		slide.Tables = append(slide.Tables, t)
	}

	// Extract speaker notes.
	slide.SpeakerNotes = readSpeakerNotes(pkg, info.PartPath)

	return slide, nil
}

// resolveLayoutID finds the layout filename referenced by a slide's .rels file.
func resolveLayoutID(pkg *pptx.Package, slidePartPath string) string {
	relsPath := pptx.GetRelsPath(slidePartPath)
	relsData, err := pkg.ReadEntry(relsPath)
	if err != nil {
		return ""
	}

	rels, err := pptx.ParseRelationships(relsData)
	if err != nil {
		return ""
	}

	layoutRels := rels.FindByType(pptx.RelTypeSlideLayout)
	if len(layoutRels) == 0 {
		return ""
	}

	// Extract layout filename without extension (e.g. "slideLayout2").
	target := layoutRels[0].Target
	base := filepath.Base(target)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// readSpeakerNotes extracts notes text from the notes slide if one exists.
func readSpeakerNotes(pkg *pptx.Package, slidePartPath string) string {
	relsPath := pptx.GetRelsPath(slidePartPath)
	relsData, err := pkg.ReadEntry(relsPath)
	if err != nil {
		return ""
	}

	rels, err := pptx.ParseRelationships(relsData)
	if err != nil {
		return ""
	}

	noteRels := rels.FindByType(pptx.RelTypeNotesSlide)
	if len(noteRels) == 0 {
		return ""
	}

	// Resolve relative path.
	slideDir := filepath.Dir(slidePartPath)
	notesPath := filepath.Join(slideDir, noteRels[0].Target)
	// Normalize path separators for ZIP (always forward slash).
	notesPath = filepath.ToSlash(notesPath)

	notesData, err := pkg.ReadEntry(notesPath)
	if err != nil {
		return ""
	}

	var notes notesDocument
	if err := xml.Unmarshal(notesData, &notes); err != nil {
		return ""
	}

	// Extract text from the notes body placeholder (type="body").
	for _, sp := range notes.CSld.SpTree.Shapes {
		if sp.NvSpPr.NvPr.Placeholder != nil && sp.NvSpPr.NvPr.Placeholder.Type == "body" {
			return extractText(sp.TxBody)
		}
	}

	return ""
}

// placeholderID returns a human-friendly ID for a placeholder.
// Prefers the placeholder type (title, body, etc.) and falls back to the shape name.
func placeholderID(shapeName string, ph *placeholderRef) string {
	if ph.Type != "" {
		switch ph.Type {
		case "title", "ctrTitle":
			return "title"
		case "subTitle":
			return "subtitle"
		case "body":
			return "body"
		case "dt":
			return "date"
		case "ftr":
			return "footer"
		case "sldNum":
			return "slide_number"
		default:
			return ph.Type
		}
	}
	// No type — use shape name normalized.
	return strings.ToLower(strings.ReplaceAll(shapeName, " ", "_"))
}

// extractText concatenates all paragraph text from a text body.
func extractText(txBody *textBody) string {
	if txBody == nil {
		return ""
	}

	var paragraphs []string
	for _, p := range txBody.Paragraphs {
		var runs []string
		for _, r := range p.Runs {
			if r.Text != "" {
				runs = append(runs, r.Text)
			}
		}
		if len(runs) > 0 {
			paragraphs = append(paragraphs, strings.Join(runs, ""))
		}
	}
	return strings.Join(paragraphs, "\n")
}

// extractTable converts a parsed table XML into a Table struct.
func extractTable(name string, tbl *tableXML, xfrm *xfrmElement) Table {
	t := Table{Name: name}
	if xfrm != nil {
		t.Bounds = xfrmToRect(xfrm)
	}

	if len(tbl.Rows) == 0 {
		return t
	}

	t.Rows = len(tbl.Rows)
	if len(tbl.Rows) > 0 {
		t.Cols = len(tbl.Rows[0].Cells)
	}

	// First row is headers.
	if len(tbl.Rows) > 0 {
		for _, cell := range tbl.Rows[0].Cells {
			t.Headers = append(t.Headers, extractCellText(cell))
		}
	}

	// Remaining rows are data.
	for _, row := range tbl.Rows[1:] {
		var rowData []string
		for _, cell := range row.Cells {
			rowData = append(rowData, extractCellText(cell))
		}
		t.Data = append(t.Data, rowData)
	}

	return t
}

// extractCellText gets text from a table cell's text body.
func extractCellText(cell tableCell) string {
	if cell.TxBody == nil {
		return ""
	}
	var parts []string
	for _, p := range cell.TxBody.Paragraphs {
		var runs []string
		for _, r := range p.Runs {
			if r.Text != "" {
				runs = append(runs, r.Text)
			}
		}
		if len(runs) > 0 {
			parts = append(parts, strings.Join(runs, ""))
		}
	}
	return strings.Join(parts, "\n")
}

// xfrmToRect converts an xfrm (transform) element to a Rect.
func xfrmToRect(xfrm *xfrmElement) *Rect {
	if xfrm == nil {
		return nil
	}
	return &Rect{
		X:      xfrm.Off.X,
		Y:      xfrm.Off.Y,
		Width:  xfrm.Ext.CX,
		Height: xfrm.Ext.CY,
	}
}
