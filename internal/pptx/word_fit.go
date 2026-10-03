package pptx

import (
	"encoding/xml"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Words a written shape breaks mid-word (go-slide-creator-v74wv).
//
// The writer keeps a shape's widest word on one line where it can: the inset
// clamp (EffectiveTextInsets) gives the word the margin back, and a
// normAutofit body above the readable minimum shrinks until it fits
// (longestWordScale). What neither can rescue — a word wider than the bare
// text rectangle at the floor — the renderer breaks mid-word ("Stra / teg /
// y" in a pyramid apex, "Inbound" in a narrow value-chain chevron), and
// nothing said so. UnfitWords reads the written shape XML and reports those
// words, measured exactly as the writer measured them, so one check covers
// every builder that writes through GenerateShape.

// UnfitWord is a word a written shape cannot hold on one line.
type UnfitWord struct {
	// ShapeID is the shape's cNvPr id.
	ShapeID string
	// Word is the widest word that breaks.
	Word string
	// NeedEMU is the word's width at its written (shrunk) size; AvailEMU is
	// the line width its paragraph has: the preset's text rectangle less the
	// written insets and the paragraph's side margins.
	NeedEMU, AvailEMU int64
}

var wordFitShapeRE = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)

// wordFitShape is the part of a written <p:sp> the word check reads. Local
// names only: the fragments carry unbound a: / p: prefixes.
type wordFitShape struct {
	NvSpPr struct {
		CNvPr struct {
			ID string `xml:"id,attr"`
		} `xml:"cNvPr"`
	} `xml:"nvSpPr"`
	SpPr struct {
		Xfrm struct {
			Ext struct {
				Cx int64 `xml:"cx,attr"`
				Cy int64 `xml:"cy,attr"`
			} `xml:"ext"`
		} `xml:"xfrm"`
		PrstGeom *struct {
			Prst string `xml:"prst,attr"`
			Gd   []struct {
				Name string `xml:"name,attr"`
				Fmla string `xml:"fmla,attr"`
			} `xml:"avLst>gd"`
		} `xml:"prstGeom"`
	} `xml:"spPr"`
	TxBody *struct {
		BodyPr struct {
			Wrap        string `xml:"wrap,attr"`
			Vert        string `xml:"vert,attr"`
			LIns        *int64 `xml:"lIns,attr"`
			RIns        *int64 `xml:"rIns,attr"`
			NormAutofit *struct {
				FontScale int `xml:"fontScale,attr"`
			} `xml:"normAutofit"`
		} `xml:"bodyPr"`
		P []struct {
			PPr struct {
				MarL int64 `xml:"marL,attr"`
				MarR int64 `xml:"marR,attr"`
			} `xml:"pPr"`
			R []struct {
				RPr struct {
					Sz int    `xml:"sz,attr"`
					B  string `xml:"b,attr"`
				} `xml:"rPr"`
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"txBody"`
}

// UnfitWords returns, for every preset-geometry shape in shapesXML (a slide,
// a group or a single <p:sp>), the widest word its written text rectangle
// cannot hold on one line at the written size and stored autofit shrink.
// Words are measured in fontName, the face the text renders in, when that
// face measures the same on every host (fontcache.HostIndependent: Calibri
// as Carlito, Arial, ...), and otherwise in the writer's Liberation Sans
// stand-in, so the result never depends on the host. Placeholders, unwrapped
// and rotated text, and single glyphs are not checked.
func UnfitWords(shapesXML, fontName string) []UnfitWord {
	face := measureFaceFor(fontName).name
	var out []UnfitWord
	for _, raw := range wordFitShapeRE.FindAllString(shapesXML, -1) {
		if strings.Contains(raw, "<p:ph") {
			continue
		}
		var sp wordFitShape
		if err := xml.Unmarshal([]byte(raw), &sp); err != nil {
			continue
		}
		if w, ok := sp.unfitWord(face); ok {
			out = append(out, w)
		}
	}
	return out
}

func (sp *wordFitShape) unfitWord(face string) (UnfitWord, bool) {
	tb := sp.TxBody
	if tb == nil || sp.SpPr.PrstGeom == nil || tb.BodyPr.Wrap == "none" ||
		(tb.BodyPr.Vert != "" && tb.BodyPr.Vert != "horz") {
		return UnfitWord{}, false
	}
	textW := sp.textWidthEMU()
	scale := 1.0
	if na := tb.BodyPr.NormAutofit; na != nil && na.FontScale > 0 {
		scale = float64(na.FontScale) / autofitScaleDenominator
	}
	best := UnfitWord{ShapeID: sp.NvSpPr.CNvPr.ID}
	worst := 0.0
	for _, p := range tb.P {
		avail := textW - max(p.PPr.MarL, 0) - max(p.PPr.MarR, 0)
		for _, r := range p.R {
			size := r.RPr.Sz
			if size <= 0 {
				size = runSizeHPt(Run{})
			}
			bold := r.RPr.B == "1" || r.RPr.B == "true"
			for _, word := range strings.FieldsFunc(r.T, unicode.IsSpace) {
				w, ok := wordOverflowEMU(word, face, float64(size)/100*scale, bold, avail)
				if !ok {
					continue
				}
				if ratio := float64(w) / float64(max(avail, 1)); ratio > worst {
					worst = ratio
					best.Word, best.NeedEMU, best.AvailEMU = word, w, max(avail, 0)
				}
			}
		}
	}
	return best, best.Word != ""
}

// textWidthEMU is the width the written shape leaves its text: the preset's
// text rectangle less the written left / right insets (the OOXML 0.1"
// default when none is written).
func (sp *wordFitShape) textWidthEMU() int64 {
	adj := int64(-1)
	for _, gd := range sp.SpPr.PrstGeom.Gd {
		if v, ok := strings.CutPrefix(gd.Fmla, "val "); ok && gd.Name == "adj" {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				adj = n
			}
		}
	}
	rectW, _ := PresetTextRectSize(sp.SpPr.PrstGeom.Prst, adj, RectEmu{CX: sp.SpPr.Xfrm.Ext.Cx, CY: sp.SpPr.Xfrm.Ext.Cy})
	body := sp.TxBody.BodyPr
	lIns, rIns := int64(autofitDefaultInsetLREMU), int64(autofitDefaultInsetLREMU)
	if body.LIns != nil {
		lIns = *body.LIns
	}
	if body.RIns != nil {
		rIns = *body.RIns
	}
	return rectW - lIns - rIns
}

// measureFaceFor is autofitMeasureFace for text set in the single face
// fontName.
func measureFaceFor(fontName string) autofitFace {
	return autofitMeasureFace(&TextBody{
		ThemeFonts: ThemeFonts{Minor: fontName},
		Paragraphs: []Paragraph{{Runs: []Run{{Text: "x"}}}},
	})
}

// WordLineNeedEMU is the line width word needs to stay whole when it renders
// in fontName at sizePt with spcHPt of letter spacing (hundredths of a point
// per glyph, not scaled with the font): its width measured as the writer
// measures it — in fontName when that face is host-independent, otherwise in
// the Liberation Sans stand-in — plus the tracking, with WordFitSlack
// (StandInWordFitSlack for a stand-in measure) of room, exactly the room
// EffectiveTextInsets leaves a clamped body's widest word. Callers that size
// text by its longest word (divider titles) use it so they keep words whole
// on the same terms. ok is false when no measurement font is available.
func WordLineNeedEMU(word, fontName string, sizePt float64, bold bool, spcHPt int) (int64, bool) {
	face := measureFaceFor(fontName)
	w, err := textfit.MeasureStyledLineWidth(word, face.name, sizePt, bold)
	if err != nil {
		return 0, false
	}
	w += runTrackingEMU(Run{Spacing: spcHPt}, word)
	slack := StandInWordFitSlack
	if face.exact {
		slack = WordFitSlack
	}
	return int64(math.Ceil(float64(w) * slack)), true
}

// wordOverflowEMU measures word at sizePt and reports its width when it is
// wider than avail. Single glyphs (arrows, bullets) cannot break mid-word.
func wordOverflowEMU(word, face string, sizePt float64, bold bool, avail int64) (int64, bool) {
	if len([]rune(word)) < 2 {
		return 0, false
	}
	w, err := textfit.MeasureStyledLineWidth(word, face, sizePt, bold)
	if err != nil || w <= avail {
		return 0, false
	}
	return w, true
}
