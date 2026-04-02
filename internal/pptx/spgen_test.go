package pptx

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestGenerateShape_AllPhase1Geometries(t *testing.T) {
	t.Parallel()

	geoms := []PresetGeometry{
		GeomRect, GeomRoundRect, GeomRound1Rect, GeomRound2SameRect,
		GeomEllipse, GeomTriangle, GeomDiamond, GeomParallelogram,
		GeomTrapezoid, GeomHexagon, GeomOctagon, GeomChevron,
		GeomHomePlate, GeomPentagon, GeomPlus,
		GeomRightArrow, GeomLeftArrow, GeomUpArrow, GeomDownArrow,
		GeomDonut,
	}

	for _, geom := range geoms {
		t.Run(string(geom), func(t *testing.T) {
			sp, err := GenerateShape(ShapeOptions{
				ID:       1,
				Bounds:   RectFromInches(1, 1, 3, 2),
				Geometry: geom,
			})
			if err != nil {
				t.Fatalf("GenerateShape(%s) failed: %v", geom, err)
			}

			s := string(sp)

			// Must contain the geometry reference
			if !strings.Contains(s, `prst="`+string(geom)+`"`) {
				t.Errorf("missing prst=%q", geom)
			}

			// Must have essential structure
			for _, tag := range []string{"<p:sp>", "</p:sp>", "<p:nvSpPr>", "<p:spPr>", "<a:prstGeom", "<a:avLst/>"} {
				if !strings.Contains(s, tag) {
					t.Errorf("missing %s", tag)
				}
			}

			// Must be well-formed XML
			var v interface{}
			if err := xml.Unmarshal(sp, &v); err != nil {
				t.Errorf("invalid XML: %v\n%s", err, s)
			}
		})
	}
}

func TestGenerateShape_AllPhase2Geometries(t *testing.T) {
	t.Parallel()

	geoms := []PresetGeometry{
		// Flowchart shapes
		GeomFlowChartProcess, GeomFlowChartDecision, GeomFlowChartTerminator,
		GeomFlowChartDocument, GeomFlowChartMultidocument,
		GeomFlowChartInputOutput, GeomFlowChartPredefinedProcess,
		GeomFlowChartInternalStorage, GeomFlowChartPreparation,
		GeomFlowChartManualInput, GeomFlowChartManualOperation,
		GeomFlowChartConnector, GeomFlowChartOffpageConnector,
		GeomFlowChartAlternateProcess,
		// Additional arrows
		GeomLeftRightArrow, GeomUpDownArrow,
		GeomNotchedRightArrow, GeomStripedRightArrow,
		GeomCurvedRightArrow, GeomCurvedLeftArrow,
		GeomBentArrow, GeomBentUpArrow,
	}

	for _, geom := range geoms {
		t.Run(string(geom), func(t *testing.T) {
			sp, err := GenerateShape(ShapeOptions{
				ID:       1,
				Bounds:   RectFromInches(1, 1, 3, 2),
				Geometry: geom,
			})
			if err != nil {
				t.Fatalf("GenerateShape(%s) failed: %v", geom, err)
			}

			s := string(sp)

			// Must contain the geometry reference
			if !strings.Contains(s, `prst="`+string(geom)+`"`) {
				t.Errorf("missing prst=%q", geom)
			}

			// Must have essential structure
			for _, tag := range []string{"<p:sp>", "</p:sp>", "<p:nvSpPr>", "<p:spPr>", "<a:prstGeom", "<a:avLst/>"} {
				if !strings.Contains(s, tag) {
					t.Errorf("missing %s", tag)
				}
			}

			// Must be well-formed XML
			var v interface{}
			if err := xml.Unmarshal(sp, &v); err != nil {
				t.Errorf("invalid XML: %v\n%s", err, s)
			}
		})
	}
}

func TestGenerateShape_AllPhase4Geometries(t *testing.T) {
	t.Parallel()

	geoms := []PresetGeometry{
		GeomLine, GeomLineInv,
		GeomArc, GeomBlockArc, GeomChord,
		GeomLeftBracket, GeomRightBracket,
		GeomLeftBrace, GeomRightBrace,
		GeomBracketPair, GeomBracePair,
		GeomRightTriangle,
		GeomSnip1Rect, GeomSnip2SameRect,
		GeomSnip2DiagRect, GeomSnipRoundRect,
	}

	for _, geom := range geoms {
		t.Run(string(geom), func(t *testing.T) {
			sp, err := GenerateShape(ShapeOptions{
				ID:       1,
				Bounds:   RectFromInches(1, 1, 3, 2),
				Geometry: geom,
			})
			if err != nil {
				t.Fatalf("GenerateShape(%s) failed: %v", geom, err)
			}

			s := string(sp)

			// Must contain the geometry reference
			if !strings.Contains(s, `prst="`+string(geom)+`"`) {
				t.Errorf("missing prst=%q", geom)
			}

			// Must have essential structure
			for _, tag := range []string{"<p:sp>", "</p:sp>", "<p:nvSpPr>", "<p:spPr>", "<a:prstGeom", "<a:avLst/>"} {
				if !strings.Contains(s, tag) {
					t.Errorf("missing %s", tag)
				}
			}

			// Must be well-formed XML
			var v interface{}
			if err := xml.Unmarshal(sp, &v); err != nil {
				t.Errorf("invalid XML: %v\n%s", err, s)
			}
		})
	}
}

func TestGenerateShape_Phase2WithAdjustments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		geom PresetGeometry
		adjs []AdjustValue
	}{
		{
			name: "flowChartAlternateProcess with corner radius",
			geom: GeomFlowChartAlternateProcess,
			adjs: []AdjustValue{{Name: "adj", Value: 10000}},
		},
		{
			name: "leftRightArrow with head size",
			geom: GeomLeftRightArrow,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 50000},
				{Name: "adj2", Value: 50000},
			},
		},
		{
			name: "bentUpArrow with three adjustments",
			geom: GeomBentUpArrow,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 25000},
				{Name: "adj2", Value: 25000},
				{Name: "adj3", Value: 25000},
			},
		},
		{
			name: "bentArrow with four adjustments",
			geom: GeomBentArrow,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 25000},
				{Name: "adj2", Value: 20000},
				{Name: "adj3", Value: 25000},
				{Name: "adj4", Value: 43750},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := GenerateShape(ShapeOptions{
				ID:          1,
				Bounds:      RectFromInches(0, 0, 3, 2),
				Geometry:    tt.geom,
				Adjustments: tt.adjs,
			})
			if err != nil {
				t.Fatalf("GenerateShape(%s) failed: %v", tt.geom, err)
			}

			s := string(sp)

			if !strings.Contains(s, `<a:avLst>`) {
				t.Error("missing avLst opening tag")
			}

			for _, adj := range tt.adjs {
				if !strings.Contains(s, `name="`+adj.Name+`"`) {
					t.Errorf("missing adjustment name %q", adj.Name)
				}
			}

			// Must be well-formed XML
			var v interface{}
			if err := xml.Unmarshal(sp, &v); err != nil {
				t.Errorf("invalid XML: %v", err)
			}
		})
	}
}

func TestGenerateShape_Phase4ArcWithAdjustments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		geom PresetGeometry
		adjs []AdjustValue
	}{
		{
			name: "arc with custom angles",
			geom: GeomArc,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 16200000},
				{Name: "adj2", Value: 0},
			},
		},
		{
			name: "blockArc quarter ring",
			geom: GeomBlockArc,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 10800000},
				{Name: "adj2", Value: 0},
				{Name: "adj3", Value: 25000},
			},
		},
		{
			name: "chord with custom angles",
			geom: GeomChord,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 2700000},
				{Name: "adj2", Value: 16200000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := GenerateShape(ShapeOptions{
				ID:          1,
				Bounds:      RectFromInches(0, 0, 3, 3),
				Geometry:    tt.geom,
				Adjustments: tt.adjs,
			})
			if err != nil {
				t.Fatalf("GenerateShape(%s) failed: %v", tt.geom, err)
			}

			s := string(sp)

			if !strings.Contains(s, `<a:avLst>`) {
				t.Error("missing avLst opening tag")
			}

			for _, adj := range tt.adjs {
				if !strings.Contains(s, `name="`+adj.Name+`"`) {
					t.Errorf("missing adjustment name %q", adj.Name)
				}
			}

			// Must be well-formed XML
			var v interface{}
			if err := xml.Unmarshal(sp, &v); err != nil {
				t.Errorf("invalid XML: %v", err)
			}
		})
	}
}

func TestGenerateShape_Phase4BraceWithAdjustments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		geom PresetGeometry
		adjs []AdjustValue
	}{
		{
			name: "leftBrace tighter curve raised midpoint",
			geom: GeomLeftBrace,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 5000},
				{Name: "adj2", Value: 60000},
			},
		},
		{
			name: "rightBrace custom curvature and midpoint",
			geom: GeomRightBrace,
			adjs: []AdjustValue{
				{Name: "adj1", Value: 5000},
				{Name: "adj2", Value: 60000},
			},
		},
		{
			name: "bracketPair custom curvature",
			geom: GeomBracketPair,
			adjs: []AdjustValue{
				{Name: "adj", Value: 10000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := GenerateShape(ShapeOptions{
				ID:          1,
				Bounds:      RectFromInches(0, 0, 3, 3),
				Geometry:    tt.geom,
				Adjustments: tt.adjs,
			})
			if err != nil {
				t.Fatalf("GenerateShape(%s) failed: %v", tt.geom, err)
			}

			s := string(sp)

			if !strings.Contains(s, `<a:avLst>`) {
				t.Error("missing avLst opening tag")
			}

			for _, adj := range tt.adjs {
				if !strings.Contains(s, `name="`+adj.Name+`"`) {
					t.Errorf("missing adjustment name %q", adj.Name)
				}
			}

			// Must be well-formed XML
			var v interface{}
			if err := xml.Unmarshal(sp, &v); err != nil {
				t.Errorf("invalid XML: %v", err)
			}
		})
	}
}

func TestGenerateShape_SnipRoundRectWithAdjustments(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 3, 2),
		Geometry: GeomSnipRoundRect,
		Adjustments: []AdjustValue{
			{Name: "adj1", Value: 25000},
			{Name: "adj2", Value: 10000},
		},
	})
	if err != nil {
		t.Fatalf("GenerateShape(snipRoundRect) failed: %v", err)
	}

	s := string(sp)

	if !strings.Contains(s, `prst="snipRoundRect"`) {
		t.Error("missing prst=snipRoundRect")
	}
	if !strings.Contains(s, `<a:avLst>`) {
		t.Error("missing avLst opening tag")
	}
	if !strings.Contains(s, `name="adj1"`) {
		t.Error("missing adj1")
	}
	if !strings.Contains(s, `name="adj2"`) {
		t.Error("missing adj2")
	}
	if !strings.Contains(s, `fmla="val 25000"`) {
		t.Error("missing adj1 value")
	}
	if !strings.Contains(s, `fmla="val 10000"`) {
		t.Error("missing adj2 value")
	}

	var v interface{}
	if err := xml.Unmarshal(sp, &v); err != nil {
		t.Errorf("invalid XML: %v", err)
	}
}

func TestGenerateShape_WithAdjustments(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       5,
		Bounds:   RectFromInches(0, 0, 2, 2),
		Geometry: GeomRoundRect,
		Adjustments: []AdjustValue{
			{Name: "adj", Value: 25000},
		},
	})
	if err != nil {
		t.Fatalf("GenerateShape failed: %v", err)
	}

	s := string(sp)

	if !strings.Contains(s, `<a:avLst>`) {
		t.Error("missing avLst opening tag")
	}
	if !strings.Contains(s, `name="adj"`) {
		t.Error("missing adjustment name")
	}
	if !strings.Contains(s, `fmla="val 25000"`) {
		t.Error("missing adjustment value formula")
	}
}

func TestGenerateShape_WithTextBody(t *testing.T) {
	t.Parallel()

	text := &TextBody{
		Wrap:   "square",
		Anchor: "ctr",
		Paragraphs: []Paragraph{
			{
				Align: "ctr",
				Runs: []Run{
					{Text: "Hello World", FontSize: 1800, Bold: true},
				},
			},
		},
	}

	sp, err := GenerateShape(ShapeOptions{
		ID:       10,
		Bounds:   RectFromInches(1, 1, 4, 2),
		Geometry: GeomRect,
		Fill:     SolidFill("4472C4"),
		Text:     text,
	})
	if err != nil {
		t.Fatalf("GenerateShape failed: %v", err)
	}

	s := string(sp)

	if !strings.Contains(s, `<p:txBody>`) {
		t.Error("missing txBody")
	}
	if !strings.Contains(s, `Hello World`) {
		t.Error("missing text content")
	}
	if !strings.Contains(s, `sz="1800"`) {
		t.Error("missing font size")
	}
	if !strings.Contains(s, `b="1"`) {
		t.Error("missing bold")
	}

	// Verify well-formed
	var v interface{}
	if err := xml.Unmarshal(sp, &v); err != nil {
		t.Errorf("invalid XML: %v", err)
	}
}

func TestGenerateShape_WithRotationAndFlip(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       3,
		Bounds:   RectFromInches(2, 2, 3, 3),
		Geometry: GeomRightArrow,
		Rotation: 5400000, // 90 degrees
		FlipH:    true,
		FlipV:    true,
	})
	if err != nil {
		t.Fatalf("GenerateShape failed: %v", err)
	}

	s := string(sp)

	if !strings.Contains(s, `rot="5400000"`) {
		t.Error("missing rotation")
	}
	if !strings.Contains(s, `flipH="1"`) {
		t.Error("missing flipH")
	}
	if !strings.Contains(s, `flipV="1"`) {
		t.Error("missing flipV")
	}
}

func TestGenerateShape_WithFillAndLine(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       7,
		Bounds:   RectFromInches(1, 1, 2, 2),
		Geometry: GeomEllipse,
		Fill:     SchemeFill("accent1", LumMod(75000)),
		Line:     SolidLinePoints(2.0, "000000"),
	})
	if err != nil {
		t.Fatalf("GenerateShape failed: %v", err)
	}

	s := string(sp)

	// Fill
	if !strings.Contains(s, `<a:solidFill>`) {
		t.Error("missing solidFill")
	}
	if !strings.Contains(s, `schemeClr val="accent1"`) {
		t.Error("missing scheme color")
	}
	if !strings.Contains(s, `lumMod val="75000"`) {
		t.Error("missing lumMod")
	}

	// Line
	if !strings.Contains(s, `<a:ln`) {
		t.Error("missing line element")
	}
	if !strings.Contains(s, `srgbClr val="000000"`) {
		t.Error("missing line color")
	}
}

func TestGenerateShape_DefaultName(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       42,
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if !strings.Contains(string(sp), `name="Shape 42"`) {
		t.Error("expected default name 'Shape 42'")
	}
}

func TestGenerateShape_CustomNameAndDescription(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:          1,
		Name:        "My Shape",
		Description: "A blue rectangle",
		Bounds:      RectFromInches(0, 0, 1, 1),
		Geometry:    GeomRect,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(sp)
	if !strings.Contains(s, `name="My Shape"`) {
		t.Error("missing custom name")
	}
	if !strings.Contains(s, `descr="A blue rectangle"`) {
		t.Error("missing description")
	}
}

func TestGenerateShape_MinimalTxBody(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(sp)

	// Without Text option, a minimal txBody should be generated
	if !strings.Contains(s, `<a:bodyPr/>`) {
		t.Error("missing minimal bodyPr")
	}
	if !strings.Contains(s, `<a:lstStyle/>`) {
		t.Error("missing minimal lstStyle")
	}
	if !strings.Contains(s, `<a:p/>`) {
		t.Error("missing minimal paragraph")
	}
}

func TestGenerateShape_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts ShapeOptions
	}{
		{
			name: "missing ID",
			opts: ShapeOptions{Geometry: GeomRect, Bounds: RectFromInches(0, 0, 1, 1)},
		},
		{
			name: "missing geometry",
			opts: ShapeOptions{ID: 1, Bounds: RectFromInches(0, 0, 1, 1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateShape(tt.opts)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestGenerateShape_NoFillNoLine(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(sp)

	// Without explicit fill/line, those elements should not appear
	if strings.Contains(s, `<a:solidFill>`) {
		t.Error("should not have solidFill without explicit fill")
	}
	if strings.Contains(s, `<a:ln`) {
		t.Error("should not have line without explicit line")
	}
}

func TestGenerateShape_NoFill(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomRect,
		Fill:     NoFill(),
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if !strings.Contains(string(sp), `<a:noFill/>`) {
		t.Error("missing noFill element")
	}
}

func TestGenerateShape_SpecialCharsInName(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:          1,
		Name:        `Shape "A" & <B>`,
		Description: `Alt: "test" & <value>`,
		Bounds:      RectFromInches(0, 0, 1, 1),
		Geometry:    GeomRect,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(sp)
	if !strings.Contains(s, `&quot;`) {
		t.Error("quotes should be escaped")
	}
	if !strings.Contains(s, `&amp;`) {
		t.Error("ampersands should be escaped")
	}
	if !strings.Contains(s, `&lt;`) {
		t.Error("angle brackets should be escaped")
	}

	// Must still be well-formed
	var v interface{}
	if err := xml.Unmarshal(sp, &v); err != nil {
		t.Errorf("invalid XML: %v", err)
	}
}
