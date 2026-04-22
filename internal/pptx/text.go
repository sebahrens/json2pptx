package pptx

import (
	"bytes"
	"fmt"
)

// TextBody represents a DrawingML text body (a:txBody).
type TextBody struct {
	Wrap      string      // Text wrapping: "square", "none"
	Anchor    string      // Vertical anchor: "t", "ctr", "b"
	AnchorCtr bool        // Center text horizontally in shape
	Vert      string      // Vertical text direction: "vert", "vert270", "wordArtVert", etc.
	Insets    [4]int64    // Padding [left, top, right, bottom] in EMU
	AutoFit   string      // Auto-fit mode: "noAutofit", "normAutofit", "spAutoFit"
	Paragraphs []Paragraph
}

// Paragraph represents a DrawingML paragraph (a:p).
type Paragraph struct {
	Align      string     // Alignment: "l", "ctr", "r", "just"
	Runs       []Run      // Text runs in this paragraph
	Bullet     *BulletDef // Optional bullet definition
	NoBullet   bool       // Emit <a:buNone/> (explicitly suppress bullets)
	MarginL    int64      // Left margin in EMU
	Indent     int64      // First-line indent in EMU (negative for hanging indent)
	SpaceAfter int        // Space after paragraph in hundredths of a point (e.g. 600 = 6pt)
}

// Run represents a DrawingML text run (a:r) or field (a:fld).
type Run struct {
	Text       string
	FontSize   int    // Font size in hundredths of a point (e.g. 1200 = 12pt)
	Bold       bool
	Italic     bool
	Underline  bool
	Dirty      bool // Emit dirty="0" (marks text as spell-check clean)
	Color      Fill   // Text color fill
	Lang       string // Language tag (e.g. "en-US")
	FontFamily string // Font typeface (e.g. "+mn-lt" for theme minor font, "Arial")
	FieldType  string // If set, renders as <a:fld type="..."> instead of <a:r> (e.g. "slidenum")
	FieldID    string // UUID for field identification (required when FieldType is set)
}

// BulletDef defines bullet formatting for a paragraph.
type BulletDef struct {
	Char  string // Bullet character (e.g. "•", "–")
	Font  string // Bullet font family
	Color Fill   // Bullet color
}

// WriteTo writes the TextBody's DrawingML XML (a:txBody) into buf.
func (tb TextBody) WriteTo(buf *bytes.Buffer) {
	buf.WriteString(`<p:txBody>`)
	tb.writeBodyPr(buf)
	buf.WriteString(`<a:lstStyle/>`)
	for _, p := range tb.Paragraphs {
		p.WriteXML(buf)
	}
	if len(tb.Paragraphs) == 0 {
		// At least one paragraph required
		buf.WriteString(`<a:p/>`)
	}
	buf.WriteString(`</p:txBody>`)
}

func (tb TextBody) writeBodyPr(buf *bytes.Buffer) {
	buf.WriteString(`<a:bodyPr`)
	if tb.Wrap != "" {
		fmt.Fprintf(buf, ` wrap="%s"`, tb.Wrap)
	}
	if tb.Anchor != "" {
		fmt.Fprintf(buf, ` anchor="%s"`, tb.Anchor)
	}
	if tb.AnchorCtr {
		buf.WriteString(` anchorCtr="1"`)
	}
	if tb.Vert != "" {
		fmt.Fprintf(buf, ` vert="%s"`, tb.Vert)
	}
	if tb.Insets != [4]int64{} {
		fmt.Fprintf(buf, ` lIns="%d" tIns="%d" rIns="%d" bIns="%d"`,
			tb.Insets[0], tb.Insets[1], tb.Insets[2], tb.Insets[3])
	}
	buf.WriteString(`>`)

	// Auto-fit
	switch tb.AutoFit {
	case "normAutofit":
		buf.WriteString(`<a:normAutofit/>`)
	case "spAutoFit":
		buf.WriteString(`<a:spAutoFit/>`)
	case "noAutofit":
		buf.WriteString(`<a:noAutofit/>`)
	}

	buf.WriteString(`</a:bodyPr>`)
}

// WriteXML writes the paragraph's DrawingML XML (a:p) into buf.
func (p Paragraph) WriteXML(buf *bytes.Buffer) {
	buf.WriteString(`<a:p>`)

	// Paragraph properties
	hasPPr := p.Align != "" || p.MarginL != 0 || p.Indent != 0 || p.Bullet != nil || p.NoBullet || p.SpaceAfter > 0
	if hasPPr {
		buf.WriteString(`<a:pPr`)
		if p.Align != "" {
			fmt.Fprintf(buf, ` algn="%s"`, p.Align)
		}
		if p.MarginL != 0 {
			fmt.Fprintf(buf, ` marL="%d"`, p.MarginL)
		}
		if p.Indent != 0 {
			fmt.Fprintf(buf, ` indent="%d"`, p.Indent)
		}
		hasChildren := p.Bullet != nil || p.NoBullet || p.SpaceAfter > 0
		if hasChildren {
			buf.WriteString(`>`)
			if p.SpaceAfter > 0 {
				fmt.Fprintf(buf, `<a:spcAft><a:spcPts val="%d"/></a:spcAft>`, p.SpaceAfter)
			}
			if p.Bullet != nil {
				p.Bullet.marshalXML(buf)
			}
			if p.NoBullet {
				buf.WriteString(`<a:buNone/>`)
			}
			buf.WriteString(`</a:pPr>`)
		} else {
			buf.WriteString(`/>`)
		}
	}

	// Runs
	for _, r := range p.Runs {
		r.marshalXML(buf)
	}

	buf.WriteString(`</a:p>`)
}

func (b BulletDef) marshalXML(buf *bytes.Buffer) {
	if !b.Color.IsZero() {
		buf.WriteString(`<a:buClr>`)
		b.Color.WriteColorTo(buf)
		buf.WriteString(`</a:buClr>`)
	}
	if b.Font != "" {
		fmt.Fprintf(buf, `<a:buFont typeface="%s"/>`, escapeXMLAttr(b.Font))
	}
	if b.Char != "" {
		fmt.Fprintf(buf, `<a:buChar char="%s"/>`, escapeXMLAttr(b.Char))
	}
}

func (r Run) marshalXML(buf *bytes.Buffer) {
	// Use <a:fld> for field elements (e.g. slidenum), <a:r> for normal runs
	isField := r.FieldType != ""
	if isField {
		fmt.Fprintf(buf, `<a:fld id="%s" type="%s">`, escapeXMLAttr(r.FieldID), escapeXMLAttr(r.FieldType))
	} else {
		buf.WriteString(`<a:r>`)
	}

	// Run properties
	hasRPr := r.FontSize > 0 || r.Bold || r.Italic || r.Underline || r.Dirty || !r.Color.IsZero() || r.Lang != "" || r.FontFamily != ""
	if hasRPr {
		buf.WriteString(`<a:rPr`)
		if r.Lang != "" {
			fmt.Fprintf(buf, ` lang="%s"`, r.Lang)
		}
		if r.FontSize > 0 {
			fmt.Fprintf(buf, ` sz="%d"`, r.FontSize)
		}
		if r.Bold {
			buf.WriteString(` b="1"`)
		}
		if r.Italic {
			buf.WriteString(` i="1"`)
		}
		if r.Underline {
			buf.WriteString(` u="sng"`)
		}
		if r.Dirty {
			buf.WriteString(` dirty="0"`)
		}
		hasChildren := !r.Color.IsZero() || r.FontFamily != ""
		if hasChildren {
			buf.WriteString(`>`)
			if !r.Color.IsZero() {
				r.Color.WriteTo(buf)
			}
			if r.FontFamily != "" {
				fmt.Fprintf(buf, `<a:latin typeface="%s"/>`, escapeXMLAttr(r.FontFamily))
			}
			buf.WriteString(`</a:rPr>`)
		} else {
			buf.WriteString(`/>`)
		}
	}

	buf.WriteString(`<a:t>`)
	buf.WriteString(escapeXMLText(r.Text))
	buf.WriteString(`</a:t>`)

	if isField {
		buf.WriteString(`</a:fld>`)
	} else {
		buf.WriteString(`</a:r>`)
	}
}

// escapeXMLText escapes special characters for XML text content.
func escapeXMLText(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
