package pptx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image/png"
	"math"
	"path"
	"strings"
)

// svgFallbackPic holds only the picture fields needed to measure the raster
// fallback at the physical size at which the slide displays it.
type svgFallbackPic struct {
	BlipFill struct {
		Blip struct {
			Embed  string `xml:"embed,attr"`
			ExtLst struct {
				Ext []struct {
					SVGBlip *struct{} `xml:"svgBlip"`
				} `xml:"ext"`
			} `xml:"extLst"`
		} `xml:"blip"`
	} `xml:"blipFill"`
	SpPr struct {
		Xfrm struct {
			Ext struct {
				CX int64 `xml:"cx,attr"`
				CY int64 `xml:"cy,attr"`
			} `xml:"ext"`
		} `xml:"xfrm"`
	} `xml:"spPr"`
}

func (pic svgFallbackPic) hasSVG() bool {
	for _, ext := range pic.BlipFill.Blip.ExtLst.Ext {
		if ext.SVGBlip != nil {
			return true
		}
	}
	return false
}

// fallbackDPIFindings warns only about SVG pictures whose PNG fallback would
// be visibly undersampled in SVG-unaware viewers. It is advisory, not an OPC
// integrity check: a low-resolution raster remains a valid package part.
func (ov *OutputValidator) fallbackDPIFindings() []Finding {
	var findings []Finding
	for _, slidePath := range ov.pkg.Entries() {
		slideIdx := slideIndexFromPart(slidePath)
		if slideIdx < 0 {
			continue
		}
		findings = append(findings, ov.slideFallbackDPIFindings(slidePath, slideIdx)...)
	}
	return findings
}

func (ov *OutputValidator) slideFallbackDPIFindings(slidePath string, slideIdx int) []Finding {
	var findings []Finding
	slideXML, err := ov.pkg.ReadEntry(slidePath)
	if err != nil {
		return nil // structural validator reports unreadable parts
	}
	relsXML, err := ov.pkg.ReadEntry(GetRelsPath(slidePath))
	if err != nil {
		return nil
	}
	rels, err := ParseRelationships(relsXML)
	if err != nil {
		return nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(slideXML))
	for {
		token, err := decoder.Token()
		if err != nil {
			break // malformed XML is reported by the existing validator
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "pic" {
			continue
		}
		var pic svgFallbackPic
		if err := decoder.DecodeElement(&pic, &start); err != nil || !pic.hasSVG() {
			continue
		}
		mediaPath, dpi, ok := ov.svgFallbackDPI(slidePath, rels, pic)
		if !ok || dpi >= 96 {
			continue
		}
		findings = append(findings, Finding{
			Code: "OOXML_LOW_FALLBACK_DPI", Severity: SeverityWarning,
			Path: slidePath, Message: fmt.Sprintf("SVG picture's PNG fallback %s is %.1f DPI at its displayed size; use at least 96 DPI", mediaPath, dpi),
			Phase: "ooxml", Validator: "raster_fallback", SlideIndex: slideIdx,
			SourcePath: sourcePathFromSlideIndex(slideIdx), Scope: RepairScopeGenerator,
		})
	}
	return findings
}

func (ov *OutputValidator) svgFallbackDPI(slidePath string, rels *Relationships, pic svgFallbackPic) (string, float64, bool) {
	if pic.SpPr.Xfrm.Ext.CX <= 0 || pic.SpPr.Xfrm.Ext.CY <= 0 {
		return "", 0, false
	}
	rel := rels.Get(pic.BlipFill.Blip.Embed)
	if rel == nil || rel.Type != RelTypeImage || rel.TargetMode == "External" {
		return "", 0, false
	}
	mediaPath := path.Clean(path.Join(path.Dir(slidePath), rel.Target))
	if strings.HasPrefix(rel.Target, "/") {
		mediaPath = strings.TrimPrefix(path.Clean(rel.Target), "/")
	}
	if !strings.EqualFold(path.Ext(mediaPath), ".png") {
		return "", 0, false
	}
	pngData, err := ov.pkg.ReadEntry(mediaPath)
	if err != nil {
		return "", 0, false
	}
	image, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return "", 0, false
	}
	const emuPerInch = 914400.0
	dpi := math.Min(float64(image.Width)*emuPerInch/float64(pic.SpPr.Xfrm.Ext.CX),
		float64(image.Height)*emuPerInch/float64(pic.SpPr.Xfrm.Ext.CY))
	return mediaPath, dpi, true
}
