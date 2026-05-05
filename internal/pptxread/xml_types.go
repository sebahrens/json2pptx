package pptxread

import "encoding/xml"

// slideDocument is a minimal representation of a slide XML file for reading.
type slideDocument struct {
	XMLName xml.Name       `xml:"sld"`
	CSld    commonSlideData `xml:"cSld"`
}

// notesDocument is a minimal representation of a notes slide XML file.
type notesDocument struct {
	XMLName xml.Name       `xml:"notes"`
	CSld    commonSlideData `xml:"cSld"`
}

type commonSlideData struct {
	SpTree shapeTree `xml:"spTree"`
}

type shapeTree struct {
	Shapes        []shapeElement        `xml:"sp"`
	GraphicFrames []graphicFrameElement `xml:"graphicFrame"`
}

// shapeElement represents a <p:sp> element.
type shapeElement struct {
	NvSpPr nvSpPr         `xml:"nvSpPr"`
	SpPr   shapeProperties `xml:"spPr"`
	TxBody *textBody       `xml:"txBody"`
}

type nvSpPr struct {
	CNvPr cnvPr  `xml:"cNvPr"`
	NvPr  nvPr   `xml:"nvPr"`
}

type cnvPr struct {
	ID   uint32 `xml:"id,attr"`
	Name string `xml:"name,attr"`
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

type paragraph struct {
	Runs []run `xml:"r"`
}

type run struct {
	Text string `xml:"t"`
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
