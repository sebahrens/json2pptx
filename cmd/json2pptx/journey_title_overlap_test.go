package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression test for go-slide-creator-40to: the journey-maturity-model pattern
// must render in a region separate from the slide title.
//
// Before the dependency fixes (shape_grid raw-shape z-order, resolveVirtualLayout
// ContentZone derivation, explicit-layout ContentZone, and title font-size
// extraction) the pattern's resolved bounds intruded into the title band and the
// raw shapes — inserted at the end of the spTree — painted on top of the title
// placeholder, producing the "oversized title clipped and covered" defect on
// modern-template. This test asserts the integrated outcome end-to-end through
// runJSONMode across every bundled template:
//
//  1. the title placeholder's bottom edge sits at or above the topmost pattern
//     shape that horizontally overlaps it (no vertical intrusion into the title);
//  2. the title placeholder carries normAutofit so the (deliberately long) title
//     text shrinks to fit its box instead of overflowing down into the pattern;
//  3. the title shape appears after the pattern shapes in document order, so it
//     renders on top rather than being covered.
func TestJourneyMaturityModelTitleNoOverlap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-template overlap test in short mode")
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")

	templates := []string{"midnight-blue", "forest-green", "warm-coral", "modern-template"}
	for _, tmpl := range templates {
		tmpl := tmpl // capture for parallel subtest
		t.Run(tmpl, func(t *testing.T) {
			t.Parallel()

			outputDir := t.TempDir()
			outName := "journey_overlap.pptx"

			tmpJSON := filepath.Join(outputDir, "deck.json")
			if err := os.WriteFile(tmpJSON, journeyDeckJSON(t, tmpl, outName), 0o644); err != nil {
				t.Fatal(err)
			}

			resultPath := filepath.Join(outputDir, "result.json")
			if err := runJSONMode(tmpJSON, resultPath, templatesDir, outputDir, "", false, false, "", "off", false, "off", "", false); err != nil {
				t.Fatalf("runJSONMode failed: %v", err)
			}

			assertNoTitlePatternOverlap(t, filepath.Join(outputDir, outName))
		})
	}
}

// journeyDeckJSON builds a single-slide deck whose only content is a long title
// and a journey-maturity-model pattern. The long title is the exact condition
// that produced the original clipped/covered defect.
func journeyDeckJSON(t *testing.T, template, outputFilename string) []byte {
	t.Helper()
	deck := map[string]any{
		"template":        template,
		"output_filename": outputFilename,
		"slides": []any{
			map[string]any{
				"layout_id": "content",
				"content": []any{
					map[string]any{
						"placeholder_id": "title",
						"type":           "text",
						"text_value":     "Journey — journey-maturity-model pattern (current: Defined)",
					},
				},
				"pattern": map[string]any{
					"name": "journey-maturity-model",
					"values": map[string]any{
						"stages": []any{
							map[string]any{"label": "Initial", "description": "Ad hoc processes; no formal practices in place."},
							map[string]any{"label": "Developing", "description": "Repeatable practices; informal governance."},
							map[string]any{"label": "Defined", "description": "Standardised practices; documented playbooks.", "current": true},
							map[string]any{"label": "Managed", "description": "Quantitatively measured outcomes; continuous review."},
							map[string]any{"label": "Optimising", "description": "Continuous improvement embedded across the organisation."},
						},
					},
				},
			},
		},
	}
	data, err := json.Marshal(deck)
	if err != nil {
		t.Fatalf("marshal deck: %v", err)
	}
	return data
}

// --- minimal slide-geometry parser (local names; namespace-agnostic) ---

type jtOff struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}
type jtExt struct {
	Cx int64 `xml:"cx,attr"`
	Cy int64 `xml:"cy,attr"`
}
type jtXfrm struct {
	Off jtOff `xml:"off"`
	Ext jtExt `xml:"ext"`
}
type jtPh struct {
	Type string `xml:"type,attr"`
}
type jtShape struct {
	Ph     *jtPh   `xml:"nvSpPr>nvPr>ph"`
	Xfrm   *jtXfrm `xml:"spPr>xfrm"`
	BodyPr struct {
		Inner string `xml:",innerxml"`
	} `xml:"txBody>bodyPr"`
}
type jtSlide struct {
	SpTree struct {
		Sp    []jtShape `xml:"sp"`
		CxnSp []jtShape `xml:"cxnSp"`
	} `xml:"cSld>spTree"`
}

func assertNoTitlePatternOverlap(t *testing.T, pptxPath string) {
	t.Helper()

	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer func() { _ = zr.Close() }()

	// Locate the slide that carries the journey title.
	var slideXML []byte
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		if bytes.Contains(b, []byte("journey-maturity-model pattern")) {
			slideXML = b
			break
		}
	}
	if slideXML == nil {
		t.Fatal("could not find the journey-maturity-model slide in the generated PPTX")
	}

	var slide jtSlide
	if err := xml.Unmarshal(slideXML, &slide); err != nil {
		t.Fatalf("unmarshal slide xml: %v", err)
	}

	// Find the title placeholder (must have explicit geometry).
	var title *jtShape
	for i := range slide.SpTree.Sp {
		sp := &slide.SpTree.Sp[i]
		if sp.Ph != nil && (sp.Ph.Type == "title" || sp.Ph.Type == "ctrTitle") && sp.Xfrm != nil {
			title = sp
			break
		}
	}
	if title == nil {
		t.Fatal("no title placeholder with explicit geometry found on the journey slide")
	}

	titleTop := title.Xfrm.Off.Y
	titleBottom := title.Xfrm.Off.Y + title.Xfrm.Ext.Cy
	titleLeft := title.Xfrm.Off.X
	titleRight := title.Xfrm.Off.X + title.Xfrm.Ext.Cx

	// Walk every non-title shape with real geometry and check it does not start
	// above the title's bottom edge where it horizontally overlaps the title.
	pattern := make([]jtShape, 0, len(slide.SpTree.Sp)+len(slide.SpTree.CxnSp))
	pattern = append(pattern, slide.SpTree.Sp...)
	pattern = append(pattern, slide.SpTree.CxnSp...)

	topMost := int64(1<<62 - 1)
	overlap := false
	for i := range pattern {
		s := &pattern[i]
		if s.Xfrm == nil {
			continue
		}
		if s.Ph != nil && (s.Ph.Type == "title" || s.Ph.Type == "ctrTitle") {
			continue
		}
		if s.Xfrm.Ext.Cy <= 0 { // skip zero-height connectors / lines
			continue
		}
		x := s.Xfrm.Off.X
		right := s.Xfrm.Off.X + s.Xfrm.Ext.Cx
		// Horizontal overlap with the title band?
		if x >= titleRight || right <= titleLeft {
			continue
		}
		y := s.Xfrm.Off.Y
		if y < topMost {
			topMost = y
		}
		// Vertical intrusion: shape starts above the title's bottom edge.
		if y < titleBottom {
			overlap = true
			t.Errorf("pattern shape intrudes into title band: shape top=%d (%.3fin) < title bottom=%d (%.3fin)",
				y, inches(y), titleBottom, inches(titleBottom))
		}
	}

	if !overlap {
		t.Logf("OK: title [%.3f→%.3fin] clear of pattern (topmost pattern shape at %.3fin, gap %.3fin)",
			inches(titleTop), inches(titleBottom), inches(topMost), inches(topMost-titleBottom))
	}

	// The title must autofit so the long text shrinks inside its box rather than
	// overflowing downward into the pattern region.
	if !strings.Contains(title.BodyPr.Inner, "normAutofit") {
		t.Errorf("title placeholder is missing normAutofit; long titles could overflow into the pattern. bodyPr inner=%q", title.BodyPr.Inner)
	}

	// Z-order: the title text must appear after the pattern content in document
	// order so the native placeholder renders on top of the raw shapes. The
	// title's unique "current: Defined" marker must follow the last pattern
	// label ("Optimising").
	s := string(slideXML)
	titleMark := strings.Index(s, "current: Defined")
	patternMark := strings.LastIndex(s, "Optimising")
	if titleMark < 0 || patternMark < 0 {
		t.Fatalf("expected both title and pattern markers in slide XML (titleMark=%d patternMark=%d)", titleMark, patternMark)
	}
	if titleMark < patternMark {
		t.Errorf("title appears before pattern shapes in document order (titleMark=%d < patternMark=%d): title would render behind the pattern", titleMark, patternMark)
	}
}

func inches(emu int64) float64 { return float64(emu) / 914400.0 }
