package pptxread

import "encoding/xml"

// slideDocument is a minimal representation of a slide XML file for reading.
type slideDocument struct {
	XMLName xml.Name        `xml:"sld"`
	CSld    commonSlideData `xml:"cSld"`
}

// notesDocument is a minimal representation of a notes slide XML file.
type notesDocument struct {
	XMLName xml.Name        `xml:"notes"`
	CSld    commonSlideData `xml:"cSld"`
}

type commonSlideData struct {
	SpTree shapeTree `xml:"spTree"`
}

type shapeTree struct {
	Shapes        []shapeElement        `xml:"sp"`
	GraphicFrames []graphicFrameElement `xml:"graphicFrame"`
	// Groups holds p:grpSp children. Native diagrams (swot, pestel, bmc)
	// render as groups, so without them their text was invisible to readers
	// (go-slide-creator-s1uvj.27).
	Groups []groupShapeElement `xml:"grpSp"`
	// Pictures holds p:pic children: images plus every SVG chart, diagram
	// and icon the engine embeds (go-slide-creator-csclk.25).
	Pictures []pictureElement `xml:"pic"`
	// Connectors holds p:cxnSp children (lines and arrows).
	Connectors []connectorElement `xml:"cxnSp"`
	// AltContent holds mc:AlternateContent wrappers; their mc:Choice (or,
	// failing that, mc:Fallback) content is part of the slide.
	AltContent []alternateContent `xml:"AlternateContent"`
}

// alternateContent represents <mc:AlternateContent>: alternative renditions
// of the same content, of which a reader shows one.
type alternateContent struct {
	Choices  []shapeTree `xml:"Choice"`
	Fallback *shapeTree  `xml:"Fallback"`
}

// pictureElement represents a <p:pic>.
type pictureElement struct {
	NvPicPr  nvPicPr         `xml:"nvPicPr"`
	BlipFill blipFillElement `xml:"blipFill"`
	SpPr     shapeProperties `xml:"spPr"`
}

type nvPicPr struct {
	CNvPr cnvPr `xml:"cNvPr"`
}

type blipFillElement struct {
	Blip *blipElement `xml:"blip"`
}

type blipElement struct {
	Embed  string       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
	Link   string       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr"`
	ExtLst *blipExtList `xml:"extLst"`
}

type blipExtList struct {
	Exts []blipExt `xml:"ext"`
}

type blipExt struct {
	SVGBlip *svgBlipElement `xml:"svgBlip"`
}

type svgBlipElement struct {
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
}

// connectorElement represents a <p:cxnSp>.
type connectorElement struct {
	NvCxnSpPr nvCxnSpPr       `xml:"nvCxnSpPr"`
	SpPr      shapeProperties `xml:"spPr"`
}

type nvCxnSpPr struct {
	CNvPr cnvPr `xml:"cNvPr"`
}

// hyperlinkElement represents <a:hlinkClick>; its r:id names the target.
type hyperlinkElement struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// groupShapeElement represents a <p:grpSp>: a nested shape tree whose
// children are positioned in the group's child coordinate space.
type groupShapeElement struct {
	GrpSpPr groupShapeProperties `xml:"grpSpPr"`
	shapeTree
}

type groupShapeProperties struct {
	Xfrm *groupXfrmElement `xml:"xfrm"`
}

type groupXfrmElement struct {
	Off   offsetElement `xml:"off"`
	Ext   extentElement `xml:"ext"`
	ChOff offsetElement `xml:"chOff"`
	ChExt extentElement `xml:"chExt"`
}

// shapeElement represents a <p:sp> element.
type shapeElement struct {
	NvSpPr nvSpPr          `xml:"nvSpPr"`
	SpPr   shapeProperties `xml:"spPr"`
	TxBody *textBody       `xml:"txBody"`
}

type nvSpPr struct {
	CNvPr cnvPr `xml:"cNvPr"`
	NvPr  nvPr  `xml:"nvPr"`
}

type cnvPr struct {
	ID         uint32            `xml:"id,attr"`
	Name       string            `xml:"name,attr"`
	Descr      string            `xml:"descr,attr"`
	HlinkClick *hyperlinkElement `xml:"hlinkClick"`
}

type nvPr struct {
	Placeholder *placeholderRef `xml:"ph"`
}

type placeholderRef struct {
	Type  string `xml:"type,attr,omitempty"`
	Index string `xml:"idx,attr,omitempty"`
}

type shapeProperties struct {
	Xfrm     *xfrmElement    `xml:"xfrm"`
	PrstGeom *presetGeometry `xml:"prstGeom"`
}

type xfrmElement struct {
	Off offsetElement `xml:"off"`
	Ext extentElement `xml:"ext"`
}

type offsetElement struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

type extentElement struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

type presetGeometry struct {
	Prst string `xml:"prst,attr"`
}

type textBody struct {
	Paragraphs []paragraph `xml:"p"`
}

// paragraph keeps a:r text runs and a:fld field runs (slide numbers, dates)
// in document order; a field's cached text is part of what the slide shows.
type paragraph struct {
	Runs []run
}

// UnmarshalXML collects <a:r> and <a:fld> children in order.
func (p *paragraph) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" || t.Name.Local == "fld" {
				var r run
				if err := d.DecodeElement(&r, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, r)
				continue
			}
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

type run struct {
	Text string         `xml:"t"`
	RPr  *runProperties `xml:"rPr"`
}

type runProperties struct {
	HlinkClick *hyperlinkElement `xml:"hlinkClick"`
}

// graphicFrameElement represents a <p:graphicFrame> for tables/charts.
type graphicFrameElement struct {
	NvGraphicFramePr nvGraphicFramePr `xml:"nvGraphicFramePr"`
	Xfrm             *xfrmElement     `xml:"xfrm"`
	Graphic          graphicElement   `xml:"graphic"`
}

type nvGraphicFramePr struct {
	CNvPr cnvPr `xml:"cNvPr"`
}

type graphicElement struct {
	GraphicData graphicData `xml:"graphicData"`
}

type graphicData struct {
	Table *tableXML `xml:"tbl"`
}

type tableXML struct {
	Rows []tableRow `xml:"tr"`
}

type tableRow struct {
	Cells []tableCell `xml:"tc"`
}

type tableCell struct {
	TxBody *textBody `xml:"txBody"`
}
