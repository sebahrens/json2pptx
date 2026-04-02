package template

import (
	"encoding/xml"
	"log/slog"
)

// slideSizeXML matches <p:sldSz cx="..." cy="..."/> in presentation.xml.
type slideSizeXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

// presentationSizeXML is a minimal parse target for extracting slide dimensions.
type presentationSizeXML struct {
	XMLName   xml.Name      `xml:"presentation"`
	SlideSize *slideSizeXML `xml:"sldSz"`
}

// ParseSlideDimensions extracts slide width and height (EMU) from the template's
// presentation.xml. Returns (0, 0) if the dimensions cannot be determined.
func ParseSlideDimensions(reader *Reader) (width, height int64) {
	data, err := reader.ReadFile("ppt/presentation.xml")
	if err != nil {
		slog.Debug("could not read presentation.xml for slide dimensions", slog.String("error", err.Error()))
		return 0, 0
	}

	var pres presentationSizeXML
	if err := xml.Unmarshal(data, &pres); err != nil {
		slog.Debug("could not parse presentation.xml for slide dimensions", slog.String("error", err.Error()))
		return 0, 0
	}

	if pres.SlideSize == nil {
		return 0, 0
	}

	return pres.SlideSize.CX, pres.SlideSize.CY
}
