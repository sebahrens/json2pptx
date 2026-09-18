package portabilitycheck

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
)

// Package portabilitycheck asserts geometry for rendered decks on arbitrary templates
// (go-slide-creator-94sk). CheckDeckPortability opens a generated .pptx and the
// template it was generated from and reports every slide-level violation of the
// template-portability contract:
//
//   - every shape lies inside the slide canvas;
//   - the takeaway / source bands sit on the layout-derived chrome frame
//     (takeaway x == body column x) and overlap neither the title, the
//     footer placeholders, the rendered footers, nor a master logo;
//   - content shapes stay inside the safe area: clear of footer placeholders
//     and logos, and above the takeaway/source band when one is present.
//
// It is shared by the in-process fixture test (cmd/json2pptx) and the rendered
// CI matrix (tests/quality), so both enforce the same contract.

// Rect is an EMU rectangle.
type Rect struct{ X, Y, CX, CY int64 }

func (r Rect) String() string { return fmt.Sprintf("[x=%d y=%d w=%d h=%d]", r.X, r.Y, r.CX, r.CY) }

func (r Rect) overlaps(o Rect) bool {
	return r.CX > 0 && r.CY > 0 && o.CX > 0 && o.CY > 0 &&
		r.X < o.X+o.CX && o.X < r.X+r.CX && r.Y < o.Y+o.CY && o.Y < r.Y+r.CY
}

// DeckShape is one top-level shape on a generated slide.
type DeckShape struct {
	Name   string
	PhType string // placeholder type ("" for non-placeholders)
	Bounds Rect
}

// DeckSlide is a generated slide with its layout and top-level shapes.
type DeckSlide struct {
	Number   int
	LayoutID string
	Shapes   []DeckShape
}

// CheckDeckPortability returns the portability violations of deckPath against
// the template at templatePath. An error is returned only when a file cannot
// be read.
func CheckDeckPortability(deckPath, templatePath string) ([]string, error) {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	profile, err := template.BuildProfile(reader)
	if err != nil {
		return nil, err
	}
	logos, err := masterLogos(reader, profile)
	if err != nil {
		return nil, err
	}
	slides, err := ReadDeckSlides(deckPath)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range slides {
		out = append(out, checkSlide(s, profile, logos)...)
	}
	return out, nil
}

// slideShapes is a slide's shapes classified by portability role.
type slideShapes struct {
	takeaway, source, title *DeckShape
	rendered, content       []DeckShape // rendered footer text shapes; everything else
}

func classifyShapes(shapes []DeckShape) slideShapes {
	var c slideShapes
	for i := range shapes {
		sh := &shapes[i]
		switch {
		case sh.Name == "Takeaway" && sh.PhType == "":
			c.takeaway = sh
		case sh.Name == "Source Note" && sh.PhType == "":
			c.source = sh
		case sh.Name == "Footer Left" || sh.Name == "Footer Right":
			c.rendered = append(c.rendered, *sh)
		case sh.PhType == "title" || sh.PhType == "ctrTitle":
			c.title = sh
		case sh.PhType == "dt" || sh.PhType == "ftr" || sh.PhType == "sldNum":
		default:
			c.content = append(c.content, *sh)
		}
	}
	return c
}

type slideChecker struct {
	fails   []string
	prefix  string
	footers []Rect
	logos   []Rect
}

func (c *slideChecker) fail(format string, a ...any) {
	c.fails = append(c.fails, c.prefix+fmt.Sprintf(format, a...))
}

func (c *slideChecker) clearOf(what string, b Rect) {
	for _, f := range c.footers {
		if b.overlaps(f) {
			c.fail("%s %s overlaps footer placeholder %s", what, b, f)
		}
	}
	for _, l := range c.logos {
		if b.overlaps(l) {
			c.fail("%s %s overlaps the master logo %s", what, b, l)
		}
	}
}

func checkSlide(s DeckSlide, p *template.TemplateProfile, logos map[string][]Rect) []string {
	c := &slideChecker{prefix: fmt.Sprintf("slide %d (%s): ", s.Number, s.LayoutID)}
	if layout := p.Layout(s.LayoutID); layout != nil {
		for _, r := range layout.FooterRegions {
			if r.Y >= p.SlideHeight/2 && r.Y+r.Height <= p.SlideHeight {
				c.footers = append(c.footers, Rect{r.X, r.Y, r.Width, r.Height})
			}
		}
		c.logos = logos[layout.MasterPath]
	}
	for _, sh := range s.Shapes {
		b := sh.Bounds
		if b.CX > 0 && b.CY > 0 && (b.X < -1 || b.Y < -1 || b.X+b.CX > p.SlideWidth+1 || b.Y+b.CY > p.SlideHeight+1) {
			c.fail("shape %q %s lies outside the %dx%d canvas", sh.Name, b, p.SlideWidth, p.SlideHeight)
		}
	}

	shapes := classifyShapes(s.Shapes)
	frame := p.ChromeFrame(s.LayoutID, shapes.takeaway != nil, shapes.source != nil)
	if shapes.takeaway != nil {
		c.checkBand(shapes.takeaway, frame.Takeaway.X, shapes)
	}
	if shapes.source != nil {
		c.checkBand(shapes.source, frame.Source.X, shapes)
	}

	bandTop := int64(-1)
	if shapes.takeaway != nil {
		bandTop = shapes.takeaway.Bounds.Y
	} else if shapes.source != nil {
		bandTop = shapes.source.Bounds.Y
	}
	for _, ct := range shapes.content {
		b := ct.Bounds
		c.clearOf(fmt.Sprintf("content %q", ct.Name), b)
		if bandTop >= 0 && b.CY > 0 && b.Y < bandTop && b.Y+b.CY > bandTop+1 {
			c.fail("content %q %s runs into the takeaway/source band at y=%d", ct.Name, b, bandTop)
		}
	}
	return c.fails
}

// checkBand asserts a takeaway/source band sits on the frame's body column
// and clears the title, footer placeholders, rendered footers, and logos.
func (c *slideChecker) checkBand(band *DeckShape, wantX int64, shapes slideShapes) {
	b := band.Bounds
	if d := b.X - wantX; d < -1 || d > 1 {
		c.fail("%s x=%d, want layout body column x=%d", band.Name, b.X, wantX)
	}
	if shapes.title != nil && b.overlaps(shapes.title.Bounds) {
		c.fail("%s %s overlaps the title %s", band.Name, b, shapes.title.Bounds)
	}
	for _, f := range shapes.rendered {
		if b.overlaps(f.Bounds) {
			c.fail("%s %s overlaps %s %s", band.Name, b, f.Name, f.Bounds)
		}
	}
	c.clearOf(band.Name, b)
}

// masterLogos collects non-placeholder pictures on each slide master, keyed by
// master part path.
func masterLogos(reader *template.Reader, p *template.TemplateProfile) (map[string][]Rect, error) {
	out := map[string][]Rect{}
	seen := map[string]bool{}
	for _, l := range p.Layouts {
		if l.MasterPath == "" || seen[l.MasterPath] {
			continue
		}
		seen[l.MasterPath] = true
		data, err := reader.ReadFile(l.MasterPath)
		if err != nil {
			return nil, err
		}
		shapes, err := parseSpTree(data)
		if err != nil {
			return nil, err
		}
		for _, sh := range shapes {
			if sh.kind == "pic" && sh.PhType == "" && sh.Bounds.CX > 0 {
				out[l.MasterPath] = append(out[l.MasterPath], sh.Bounds)
			}
		}
	}
	return out, nil
}

var slidePartRe = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

// ReadDeckSlides reads every slide of a generated deck with its layout ID and
// top-level shape bounds, in slide-number order.
func ReadDeckSlides(deckPath string) ([]DeckSlide, error) {
	zr, err := zip.OpenReader(deckPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	var slides []DeckSlide
	for name, f := range files {
		m := slidePartRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		data, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		shapes, err := parseSpTree(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		s := DeckSlide{Number: n}
		for _, sh := range shapes {
			s.Shapes = append(s.Shapes, sh.DeckShape)
		}
		if rf, ok := files[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", n)]; ok {
			rels, err := readZipFile(rf)
			if err != nil {
				return nil, err
			}
			s.LayoutID = layoutFromRels(rels)
		}
		slides = append(slides, s)
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].Number < slides[j].Number })
	return slides, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func layoutFromRels(data []byte) string {
	var rels struct {
		Rel []struct {
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if xml.Unmarshal(data, &rels) != nil {
		return ""
	}
	for _, r := range rels.Rel {
		if strings.HasSuffix(r.Type, "/slideLayout") {
			return strings.TrimSuffix(path.Base(r.Target), ".xml")
		}
	}
	return ""
}

type treeShape struct {
	DeckShape
	kind string
}

// spTreeParser tracks position while streaming a shape tree.
type spTreeParser struct {
	out       []treeShape
	depth     int // depth relative to spTree (0 = outside, 1 = spTree itself)
	cur       *treeShape
	xfrmDepth int
	haveXfrm  bool
}

func (p *spTreeParser) start(t xml.StartElement) {
	if p.depth == 0 {
		if t.Name.Local == "spTree" {
			p.depth = 1
		}
		return
	}
	p.depth++
	if p.depth == 2 {
		p.cur, p.xfrmDepth, p.haveXfrm = nil, 0, false
		switch t.Name.Local {
		case "sp", "pic", "graphicFrame", "grpSp", "cxnSp":
			p.cur = &treeShape{kind: t.Name.Local}
		}
		return
	}
	if p.cur == nil {
		return
	}
	switch t.Name.Local {
	case "cNvPr":
		if p.cur.Name == "" {
			p.cur.Name = attr(t, "name")
		}
	case "ph":
		p.cur.PhType = attr(t, "type")
		if p.cur.PhType == "" {
			p.cur.PhType = "body"
		}
	case "xfrm":
		if !p.haveXfrm && p.xfrmDepth == 0 {
			p.xfrmDepth = p.depth
		}
	case "off", "ext":
		if p.xfrmDepth > 0 && p.depth == p.xfrmDepth+1 {
			p.readXfrmChild(t)
		}
	}
}

func (p *spTreeParser) readXfrmChild(t xml.StartElement) {
	num := func(name string) int64 {
		v, _ := strconv.ParseInt(attr(t, name), 10, 64)
		return v
	}
	if t.Name.Local == "off" {
		p.cur.Bounds.X, p.cur.Bounds.Y = num("x"), num("y")
		return
	}
	p.cur.Bounds.CX, p.cur.Bounds.CY = num("cx"), num("cy")
}

// end handles an end element; it reports true once the spTree has closed.
func (p *spTreeParser) end() bool {
	if p.depth == 0 {
		return false
	}
	if p.xfrmDepth > 0 && p.depth == p.xfrmDepth {
		p.xfrmDepth, p.haveXfrm = 0, true
	}
	if p.depth == 2 && p.cur != nil {
		p.out = append(p.out, *p.cur)
		p.cur = nil
	}
	p.depth--
	return p.depth == 0
}

// parseSpTree extracts the top-level children of <p:spTree> with their
// names, placeholder types, and the bounds of their first (outermost) xfrm.
func parseSpTree(data []byte) ([]treeShape, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	p := &spTreeParser{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return p.out, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			p.start(t)
		case xml.EndElement:
			if p.end() {
				return p.out, nil
			}
		}
	}
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
