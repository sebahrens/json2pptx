package template

import (
	"encoding/xml"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// decorDocument reads only the visible, non-placeholder geometry needed for
// content exclusion. It deliberately ignores line-only shapes: a separator or
// guide line is not an opaque panel that needs the whole content column.
type decorDocument struct {
	Shapes   []decorShape   `xml:"cSld>spTree>sp"`
	Pictures []decorPicture `xml:"cSld>spTree>pic"`
}

type decorPicture struct {
	NV struct {
		CNV struct {
			Name string `xml:"name,attr"`
		} `xml:"cNvPr"`
		NVP struct {
			PH *struct{} `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvPicPr"`
	Properties struct {
		Xfrm *transformXML `xml:"xfrm"`
	} `xml:"spPr"`
}

type decorShape struct {
	NV struct {
		CNV struct {
			Name string `xml:"name,attr"`
		} `xml:"cNvPr"`
		NVP struct {
			PH *struct{} `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvSpPr"`
	Properties struct {
		Xfrm *struct {
			Off *struct {
				X int64 `xml:"x,attr"`
				Y int64 `xml:"y,attr"`
			} `xml:"off"`
			Ext *struct {
				CX int64 `xml:"cx,attr"`
				CY int64 `xml:"cy,attr"`
			} `xml:"ext"`
		} `xml:"xfrm"`
		Solid *struct {
			SRGB   *decorColor `xml:"srgbClr"`
			Scheme *decorColor `xml:"schemeClr"`
		} `xml:"solidFill"`
		Gradient *struct{} `xml:"gradFill"`
	} `xml:"spPr"`
}

type decorColor struct {
	Alpha *struct {
		Val int `xml:"val,attr"`
	} `xml:"alpha"`
}

func parseDecorRegions(data []byte, source string) ([]types.DecorRegion, error) {
	var doc decorDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s decorative shapes: %w", source, err)
	}
	regions := make([]types.DecorRegion, 0, len(doc.Shapes))
	for _, shape := range doc.Shapes {
		p := shape.Properties
		if shape.NV.NVP.PH != nil || (p.Solid == nil && p.Gradient == nil) || p.Xfrm == nil || p.Xfrm.Off == nil || p.Xfrm.Ext == nil {
			continue
		}
		if p.Solid != nil {
			if p.Solid.SRGB != nil && p.Solid.SRGB.Alpha != nil && p.Solid.SRGB.Alpha.Val < 100000 {
				continue
			}
			if p.Solid.Scheme != nil && p.Solid.Scheme.Alpha != nil && p.Solid.Scheme.Alpha.Val < 100000 {
				continue
			}
		}
		r := types.DecorRegion{Source: source, Name: shape.NV.CNV.Name, X: p.Xfrm.Off.X, Y: p.Xfrm.Off.Y, Width: p.Xfrm.Ext.CX, Height: p.Xfrm.Ext.CY}
		if r.Width > 0 && r.Height > 0 {
			regions = append(regions, r)
		}
	}
	for _, picture := range doc.Pictures {
		xfrm := picture.Properties.Xfrm
		if picture.NV.NVP.PH != nil || xfrm == nil || xfrm.Offset == nil || xfrm.Extents == nil {
			continue
		}
		r := types.DecorRegion{Source: source, Name: picture.NV.CNV.Name, X: xfrm.Offset.X, Y: xfrm.Offset.Y, Width: xfrm.Extents.CX, Height: xfrm.Extents.CY}
		if r.Width > 0 && r.Height > 0 {
			regions = append(regions, r)
		}
	}
	return regions, nil
}
