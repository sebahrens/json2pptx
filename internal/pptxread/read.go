// Package pptxread provides best-effort extraction of slide content from a PPTX file.
// It reads slide XML, resolves placeholders, extracts text runs, tables, and shape
// metadata to produce a structured JSON-friendly representation.
package pptxread

import (
	"encoding/xml"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
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
	Pictures     []Picture     `json:"pictures,omitempty"`
	Tables       []Table       `json:"tables,omitempty"`
	SpeakerNotes string        `json:"speaker_notes,omitempty"`
}

// Placeholder represents a populated placeholder on a slide.
type Placeholder struct {
	ID         string   `json:"id"`
	Type       string   `json:"type,omitempty"`
	Text       string   `json:"text"`
	Hyperlinks []string `json:"hyperlinks,omitempty"`
	Bounds     *Rect    `json:"bounds,omitempty"`
}

// Shape represents a non-placeholder shape on a slide.
type Shape struct {
	Name     string `json:"name,omitempty"`
	Geometry string `json:"geometry,omitempty"`
	// Connector marks a p:cxnSp line/arrow rather than a p:sp shape.
	Connector  bool     `json:"connector,omitempty"`
	Text       string   `json:"text,omitempty"`
	Hyperlinks []string `json:"hyperlinks,omitempty"`
	Bounds     *Rect    `json:"bounds,omitempty"`
}

// Picture represents a p:pic on a slide: a photo, or an SVG chart, diagram
// or icon the engine embedded (with its PNG fallback as Media).
type Picture struct {
	Name        string   `json:"name,omitempty"`
	AltText     string   `json:"alt_text,omitempty"`
	Media       string   `json:"media,omitempty"`        // package part path (or external URL) of the blip
	ContentType string   `json:"content_type,omitempty"` // content type of Media
	SVGMedia    string   `json:"svg_media,omitempty"`    // package part path of the svgBlip, when present
	Hyperlinks  []string `json:"hyperlinks,omitempty"`
	Bounds      *Rect    `json:"bounds,omitempty"`
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
		return nil, fmt.Errorf("read slide index %d: %w", info.Index, err)
	}

	slide := &Slide{Index: info.Index}

	// Resolve layout ID from slide relationships.
	slide.LayoutID = resolveLayoutID(pkg, info.PartPath)

	// Parse slide XML.
	var sld slideDocument
	if err := xml.Unmarshal(slideData, &sld); err != nil {
		return nil, fmt.Errorf("parse slide index %d XML: %w", info.Index, err)
	}

	collectShapeTreeCtx(slide, &sld.CSld.SpTree, identityTransform, newReadContext(pkg, info.PartPath))

	// Extract speaker notes.
	slide.SpeakerNotes = readSpeakerNotes(pkg, info.PartPath)

	return slide, nil
}

// childTransform maps a rectangle from a shape tree's coordinate space to
// slide coordinates. The top-level spTree uses the identity; each p:grpSp
// composes its chOff/chExt -> off/ext mapping on top of its parent's.
type childTransform func(r Rect) Rect

func identityTransform(r Rect) Rect { return r }

// groupTransform returns the transform for a group's children, composed with
// the parent transform. A group without a usable xfrm (zero child extent)
// passes coordinates through unscaled.
func groupTransform(parent childTransform, x *groupXfrmElement) childTransform {
	if x == nil {
		return parent
	}
	sx, sy := 1.0, 1.0
	if x.ChExt.CX != 0 {
		sx = float64(x.Ext.CX) / float64(x.ChExt.CX)
	}
	if x.ChExt.CY != 0 {
		sy = float64(x.Ext.CY) / float64(x.ChExt.CY)
	}
	return func(r Rect) Rect {
		return parent(Rect{
			X:      x.Off.X + int64(float64(r.X-x.ChOff.X)*sx),
			Y:      x.Off.Y + int64(float64(r.Y-x.ChOff.Y)*sy),
			Width:  int64(float64(r.Width) * sx),
			Height: int64(float64(r.Height) * sy),
		})
	}
}

// boundsIn converts an xfrm in a tree's coordinate space to slide bounds.
func boundsIn(xfrm *xfrmElement, tf childTransform) *Rect {
	r := xfrmToRect(xfrm)
	if r == nil {
		return nil
	}
	out := tf(*r)
	return &out
}

// readContext resolves relationship ids for one slide part. A nil context
// (tests on bare XML) leaves ids unresolved.
type readContext struct {
	partDir string
	rels    *pptx.Relationships
	ct      *pptx.ContentTypes
}

func newReadContext(pkg *pptx.Package, partPath string) *readContext {
	rc := &readContext{partDir: path.Dir(partPath)}
	if data, err := pkg.ReadEntry(pptx.GetRelsPath(partPath)); err == nil {
		rc.rels, _ = pptx.ParseRelationships(data)
	}
	if data, err := pkg.ReadEntry(pptx.ContentTypesPath); err == nil {
		rc.ct, _ = pptx.ParseContentTypes(data)
	}
	return rc
}

// target resolves a relationship id to an external URL or a package part
// path. It returns "" when the id is unknown.
func (rc *readContext) target(id string) string {
	if rc == nil || rc.rels == nil || id == "" {
		return ""
	}
	rel := rc.rels.Get(id)
	if rel == nil {
		return ""
	}
	if rel.TargetMode == "External" {
		return rel.Target
	}
	if strings.HasPrefix(rel.Target, "/") {
		return strings.TrimPrefix(rel.Target, "/")
	}
	return path.Clean(path.Join(rc.partDir, rel.Target))
}

func (rc *readContext) contentType(part string) string {
	if rc == nil || rc.ct == nil || part == "" || strings.Contains(part, "://") {
		return ""
	}
	return rc.ct.ContentType("/" + part)
}

// hyperlinks returns the distinct hyperlink targets of a shape: its own
// click link plus every run-level link in its text.
func (rc *readContext) hyperlinks(nv cnvPr, txBody *textBody) []string {
	var out []string
	add := func(h *hyperlinkElement) {
		if h == nil {
			return
		}
		t := rc.target(h.RID)
		if t == "" {
			return
		}
		for _, existing := range out {
			if existing == t {
				return
			}
		}
		out = append(out, t)
	}
	add(nv.HlinkClick)
	if txBody != nil {
		for _, p := range txBody.Paragraphs {
			for _, r := range p.Runs {
				if r.RPr != nil {
					add(r.RPr.HlinkClick)
				}
			}
		}
	}
	return out
}

// collectShapeTree appends the placeholders, shapes and tables of a shape
// tree to slide, recursing into p:grpSp groups so grouped text (native
// diagrams) is read and reported at slide coordinates
// (go-slide-creator-s1uvj.27).
func collectShapeTree(slide *Slide, tree *shapeTree, tf childTransform) {
	collectShapeTreeCtx(slide, tree, tf, nil)
}

// collectShapeTreeCtx is collectShapeTree with relationship resolution for
// hyperlinks and picture media. Besides p:sp, p:graphicFrame tables and
// p:grpSp groups it reads p:pic pictures, p:cxnSp connectors and the content
// of mc:AlternateContent (go-slide-creator-csclk.25).
func collectShapeTreeCtx(slide *Slide, tree *shapeTree, tf childTransform, rc *readContext) {
	// Extract shapes (sp elements).
	for _, sp := range tree.Shapes {
		ph := sp.NvSpPr.NvPr.Placeholder
		text := extractText(sp.TxBody)

		if ph != nil {
			// This is a placeholder.
			phID := placeholderID(sp.NvSpPr.CNvPr.Name, ph)
			p := Placeholder{
				ID:         phID,
				Type:       ph.Type,
				Text:       text,
				Hyperlinks: rc.hyperlinks(sp.NvSpPr.CNvPr, sp.TxBody),
			}
			p.Bounds = boundsIn(sp.SpPr.Xfrm, tf)
			slide.Placeholders = append(slide.Placeholders, p)
		} else if text != "" || sp.SpPr.PrstGeom != nil {
			// Non-placeholder shape with text or geometry.
			s := Shape{
				Name:       sp.NvSpPr.CNvPr.Name,
				Text:       text,
				Hyperlinks: rc.hyperlinks(sp.NvSpPr.CNvPr, sp.TxBody),
			}
			if sp.SpPr.PrstGeom != nil {
				s.Geometry = sp.SpPr.PrstGeom.Prst
			}
			s.Bounds = boundsIn(sp.SpPr.Xfrm, tf)
			slide.Shapes = append(slide.Shapes, s)
		}
	}

	// Extract tables (graphicFrame elements with tbl).
	for _, gf := range tree.GraphicFrames {
		tbl := gf.Graphic.GraphicData.Table
		if tbl == nil {
			continue
		}
		t := extractTable(gf.NvGraphicFramePr.CNvPr.Name, tbl, gf.Xfrm)
		t.Bounds = boundsIn(gf.Xfrm, tf)
		slide.Tables = append(slide.Tables, t)
	}

	for i := range tree.Pictures {
		slide.Pictures = append(slide.Pictures, readPicture(&tree.Pictures[i], tf, rc))
	}

	for _, cx := range tree.Connectors {
		s := Shape{
			Name:      cx.NvCxnSpPr.CNvPr.Name,
			Connector: true,
			Bounds:    boundsIn(cx.SpPr.Xfrm, tf),
		}
		if cx.SpPr.PrstGeom != nil {
			s.Geometry = cx.SpPr.PrstGeom.Prst
		}
		slide.Shapes = append(slide.Shapes, s)
	}

	for i := range tree.Groups {
		g := &tree.Groups[i]
		collectShapeTreeCtx(slide, &g.shapeTree, groupTransform(tf, g.GrpSpPr.Xfrm), rc)
	}

	// A reader shows one rendition of mc:AlternateContent: the first
	// mc:Choice, or mc:Fallback when the choice yields nothing readable.
	for i := range tree.AltContent {
		collectAlternateContent(slide, &tree.AltContent[i], tf, rc)
	}
}

// collectAlternateContent reads the first mc:Choice, or mc:Fallback when the
// choice yields nothing readable.
func collectAlternateContent(slide *Slide, ac *alternateContent, tf childTransform, rc *readContext) {
	if len(ac.Choices) > 0 {
		var probe Slide
		collectShapeTreeCtx(&probe, &ac.Choices[0], tf, rc)
		if hasReadableContent(&probe) || ac.Fallback == nil {
			appendSlideContent(slide, &probe)
			return
		}
	}
	if ac.Fallback != nil {
		collectShapeTreeCtx(slide, ac.Fallback, tf, rc)
	}
}

// readPicture extracts a picture's name, alt text, links, bounds and media
// (raster blip target plus any svgBlip extension).
func readPicture(pic *pictureElement, tf childTransform, rc *readContext) Picture {
	p := Picture{
		Name:       pic.NvPicPr.CNvPr.Name,
		AltText:    pic.NvPicPr.CNvPr.Descr,
		Hyperlinks: rc.hyperlinks(pic.NvPicPr.CNvPr, nil),
		Bounds:     boundsIn(pic.SpPr.Xfrm, tf),
	}
	b := pic.BlipFill.Blip
	if b == nil {
		return p
	}
	id := b.Embed
	if id == "" {
		id = b.Link
	}
	p.Media = rc.target(id)
	p.ContentType = rc.contentType(p.Media)
	if b.ExtLst != nil {
		for _, ext := range b.ExtLst.Exts {
			if ext.SVGBlip != nil {
				p.SVGMedia = rc.target(ext.SVGBlip.Embed)
			}
		}
	}
	return p
}

// hasReadableContent reports whether s holds any text or picture.
func hasReadableContent(s *Slide) bool {
	for _, p := range s.Placeholders {
		if p.Text != "" {
			return true
		}
	}
	for _, sh := range s.Shapes {
		if sh.Text != "" {
			return true
		}
	}
	return len(s.Pictures) > 0 || len(s.Tables) > 0
}

// appendSlideContent appends src's extracted items to dst.
func appendSlideContent(dst, src *Slide) {
	dst.Placeholders = append(dst.Placeholders, src.Placeholders...)
	dst.Shapes = append(dst.Shapes, src.Shapes...)
	dst.Pictures = append(dst.Pictures, src.Pictures...)
	dst.Tables = append(dst.Tables, src.Tables...)
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
			// Generated two-column slides keep distinct canonical shape names
			// even though both placeholders carry the OOXML type="body".
			// Preserve that distinction for read-back and round trips.
			name := strings.ToLower(strings.TrimSpace(shapeName))
			if suffix, ok := strings.CutPrefix(name, "body_"); ok {
				if n, err := strconv.Atoi(suffix); err == nil && n >= 2 {
					return name
				}
			}
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
