package pptx

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestGenerateConnector_AllConnectorTypes(t *testing.T) {
	t.Parallel()

	geoms := []PresetGeometry{
		GeomStraightConnector1,
		GeomBentConnector2, GeomBentConnector3, GeomBentConnector4, GeomBentConnector5,
		GeomCurvedConnector2, GeomCurvedConnector3, GeomCurvedConnector4, GeomCurvedConnector5,
	}

	for _, geom := range geoms {
		t.Run(string(geom), func(t *testing.T) {
			cxn, err := GenerateConnector(ConnectorOptions{
				ID:       1,
				Bounds:   RectFromInches(1, 1, 3, 2),
				Geometry: geom,
			})
			if err != nil {
				t.Fatalf("GenerateConnector(%s) failed: %v", geom, err)
			}

			s := string(cxn)

			if !strings.Contains(s, `prst="`+string(geom)+`"`) {
				t.Errorf("missing prst=%q", geom)
			}

			for _, tag := range []string{"<p:cxnSp>", "</p:cxnSp>", "<p:nvCxnSpPr>", "<p:spPr>", "<a:prstGeom", "<a:avLst/>"} {
				if !strings.Contains(s, tag) {
					t.Errorf("missing %s", tag)
				}
			}

			// Connectors must NOT have a txBody
			if strings.Contains(s, `<p:txBody>`) || strings.Contains(s, `<a:txBody>`) {
				t.Error("connectors must not contain txBody")
			}

			// Must have a line element
			if !strings.Contains(s, `<a:ln`) {
				t.Error("missing line element")
			}

			var v interface{}
			if err := xml.Unmarshal(cxn, &v); err != nil {
				t.Errorf("invalid XML: %v\n%s", err, s)
			}
		})
	}
}

func TestGenerateConnector_WithArrowheads(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       10,
		Bounds:   RectFromInches(1, 1, 4, 0),
		Geometry: GeomStraightConnector1,
		Line:     SolidLinePoints(2.0, "FF0000"),
		HeadEnd:  &ArrowHead{Type: "triangle", W: "med", Len: "med"},
		TailEnd:  &ArrowHead{Type: "stealth", W: "lg", Len: "sm"},
	})
	if err != nil {
		t.Fatalf("GenerateConnector failed: %v", err)
	}

	s := string(cxn)

	if !strings.Contains(s, `<a:headEnd type="triangle" w="med" len="med"/>`) {
		t.Error("missing headEnd arrowhead")
	}
	if !strings.Contains(s, `<a:tailEnd type="stealth" w="lg" len="sm"/>`) {
		t.Error("missing tailEnd arrowhead")
	}

	var v interface{}
	if err := xml.Unmarshal(cxn, &v); err != nil {
		t.Errorf("invalid XML: %v", err)
	}
}

func TestGenerateConnector_WithArrowheadMinimal(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       5,
		Bounds:   RectFromInches(0, 0, 2, 2),
		Geometry: GeomBentConnector3,
		TailEnd:  &ArrowHead{Type: "triangle"},
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(cxn)

	// Minimal arrowhead: only type, no w or len
	if !strings.Contains(s, `<a:tailEnd type="triangle"/>`) {
		t.Error("missing minimal tailEnd arrowhead")
	}
	// No headEnd should appear
	if strings.Contains(s, `<a:headEnd`) {
		t.Error("should not have headEnd when not specified")
	}
}

func TestGenerateConnector_WithConnectionRefs(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       20,
		Bounds:   RectFromInches(2, 2, 3, 0),
		Geometry: GeomStraightConnector1,
		StartConn: &ConnectionRef{
			ShapeID: 2,
			SiteIdx: 1, // right
		},
		EndConn: &ConnectionRef{
			ShapeID: 3,
			SiteIdx: 3, // left
		},
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(cxn)

	if !strings.Contains(s, `<a:stCxn id="2" idx="1"/>`) {
		t.Error("missing stCxn element")
	}
	if !strings.Contains(s, `<a:endCxn id="3" idx="3"/>`) {
		t.Error("missing endCxn element")
	}
	if !strings.Contains(s, `<p:cNvCxnSpPr>`) {
		t.Error("cNvCxnSpPr should not be self-closing with connections")
	}

	var v interface{}
	if err := xml.Unmarshal(cxn, &v); err != nil {
		t.Errorf("invalid XML: %v", err)
	}
}

func TestGenerateConnector_NoConnectionRefs(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 2, 2),
		Geometry: GeomStraightConnector1,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(cxn)

	if !strings.Contains(s, `<p:cNvCxnSpPr/>`) {
		t.Error("cNvCxnSpPr should be self-closing without connections")
	}
}

func TestGenerateConnector_DefaultName(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       42,
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomStraightConnector1,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if !strings.Contains(string(cxn), `name="Connector 42"`) {
		t.Error("expected default name 'Connector 42'")
	}
}

func TestGenerateConnector_CustomName(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       1,
		Name:     "My Link",
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomBentConnector3,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if !strings.Contains(string(cxn), `name="My Link"`) {
		t.Error("missing custom name")
	}
}

func TestGenerateConnector_FlipHV(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 3, 2),
		Geometry: GeomStraightConnector1,
		FlipH:    true,
		FlipV:    true,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(cxn)
	if !strings.Contains(s, `flipH="1"`) {
		t.Error("missing flipH")
	}
	if !strings.Contains(s, `flipV="1"`) {
		t.Error("missing flipV")
	}
}

func TestGenerateConnector_DefaultLine(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 2, 0),
		Geometry: GeomStraightConnector1,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(cxn)
	// Default should be 1pt (12700 EMU) black
	if !strings.Contains(s, `w="12700"`) {
		t.Error("expected default 1pt line width")
	}
	if !strings.Contains(s, `srgbClr val="000000"`) {
		t.Error("expected default black line color")
	}
}

func TestGenerateConnector_CustomLine(t *testing.T) {
	t.Parallel()

	cxn, err := GenerateConnector(ConnectorOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 2, 0),
		Geometry: GeomStraightConnector1,
		Line:     SolidLinePoints(3.0, "0070C0"),
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(cxn)
	if !strings.Contains(s, `srgbClr val="0070C0"`) {
		t.Error("missing custom line color")
	}
}

func TestGenerateConnector_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts ConnectorOptions
	}{
		{
			name: "missing ID",
			opts: ConnectorOptions{Geometry: GeomStraightConnector1, Bounds: RectFromInches(0, 0, 1, 1)},
		},
		{
			name: "missing geometry",
			opts: ConnectorOptions{ID: 1, Bounds: RectFromInches(0, 0, 1, 1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateConnector(tt.opts)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestRouteBetween_TargetToRight(t *testing.T) {
	t.Parallel()

	source := ShapeOptions{
		ID: 1, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 100, CX: 200, CY: 100},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomRect,
		Bounds: RectEmu{X: 500, Y: 100, CX: 200, CY: 100},
	}

	bounds, startSite, endSite := RouteBetween(source, target)

	if startSite != 1 {
		t.Errorf("startSite = %d, want 1 (right)", startSite)
	}
	if endSite != 3 {
		t.Errorf("endSite = %d, want 3 (left)", endSite)
	}
	// Bounds should span from right edge of source to left edge of target
	if bounds.X != 300 { // source.X + source.CX
		t.Errorf("bounds.X = %d, want 300", bounds.X)
	}
	if bounds.CX <= 0 {
		t.Errorf("bounds.CX = %d, want > 0", bounds.CX)
	}
}

func TestRouteBetween_TargetBelow(t *testing.T) {
	t.Parallel()

	source := ShapeOptions{
		ID: 1, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 100, CX: 200, CY: 100},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 500, CX: 200, CY: 100},
	}

	bounds, startSite, endSite := RouteBetween(source, target)

	if startSite != 2 {
		t.Errorf("startSite = %d, want 2 (bottom)", startSite)
	}
	if endSite != 0 {
		t.Errorf("endSite = %d, want 0 (top)", endSite)
	}
	// Should span from bottom of source to top of target
	if bounds.Y != 200 { // source.Y + source.CY
		t.Errorf("bounds.Y = %d, want 200", bounds.Y)
	}
}

func TestRouteBetween_TargetToLeft(t *testing.T) {
	t.Parallel()

	source := ShapeOptions{
		ID: 1, Geometry: GeomRect,
		Bounds: RectEmu{X: 500, Y: 100, CX: 200, CY: 100},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 100, CX: 200, CY: 100},
	}

	_, startSite, endSite := RouteBetween(source, target)

	if startSite != 3 {
		t.Errorf("startSite = %d, want 3 (left)", startSite)
	}
	if endSite != 1 {
		t.Errorf("endSite = %d, want 1 (right)", endSite)
	}
}

func TestRouteBetween_TargetAbove(t *testing.T) {
	t.Parallel()

	source := ShapeOptions{
		ID: 1, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 500, CX: 200, CY: 100},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 100, CX: 200, CY: 100},
	}

	_, startSite, endSite := RouteBetween(source, target)

	if startSite != 0 {
		t.Errorf("startSite = %d, want 0 (top)", startSite)
	}
	if endSite != 2 {
		t.Errorf("endSite = %d, want 2 (bottom)", endSite)
	}
}

func TestRouteBetween_HomePlateOffset(t *testing.T) {
	t.Parallel()

	// HomePlate shapes should have connector bounds offset past the tip
	source := ShapeOptions{
		ID: 1, Geometry: GeomHomePlate,
		Bounds: RectEmu{X: 100, Y: 100, CX: 2000, CY: 500},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomHomePlate,
		Bounds: RectEmu{X: 2500, Y: 100, CX: 2000, CY: 500},
	}

	bounds, startSite, endSite := RouteBetween(source, target)

	if startSite != 1 {
		t.Errorf("startSite = %d, want 1 (right)", startSite)
	}
	if endSite != 3 {
		t.Errorf("endSite = %d, want 3 (left)", endSite)
	}

	// Source right edge is 2100. With 15% tip offset (300 EMU), connector starts at 2400.
	srcRightEdge := int64(100 + 2000)        // 2100
	tipOffset := int64(2000 * 15 / 100)      // 300
	expectedStartX := srcRightEdge + tipOffset // 2400
	if bounds.X != expectedStartX {
		t.Errorf("bounds.X = %d, want %d (right edge + tip offset)", bounds.X, expectedStartX)
	}

	// Target left edge is 2500. With no left-tip offset for homePlate, connector ends at 2500.
	if bounds.X+bounds.CX != 2500 {
		t.Errorf("bounds right edge = %d, want 2500 (target left edge)", bounds.X+bounds.CX)
	}

	// Connector width should be gap minus tip offset
	expectedCX := int64(2500) - expectedStartX // 100
	if bounds.CX != expectedCX {
		t.Errorf("bounds.CX = %d, want %d", bounds.CX, expectedCX)
	}
}

func TestRouteBetween_RectNoOffset(t *testing.T) {
	t.Parallel()

	// Rectangular shapes should have no tip offset
	source := ShapeOptions{
		ID: 1, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 100, CX: 2000, CY: 500},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomRect,
		Bounds: RectEmu{X: 2500, Y: 100, CX: 2000, CY: 500},
	}

	bounds, _, _ := RouteBetween(source, target)

	// No offset: connector starts at source right edge (2100)
	if bounds.X != 2100 {
		t.Errorf("bounds.X = %d, want 2100 (no offset for rect)", bounds.X)
	}
	if bounds.CX != 400 {
		t.Errorf("bounds.CX = %d, want 400 (full gap width)", bounds.CX)
	}
}

func TestRouteBetween_DiagonalPreferHorizontal(t *testing.T) {
	t.Parallel()

	// When horizontal distance > vertical distance, should use right/left
	source := ShapeOptions{
		ID: 1, Geometry: GeomRect,
		Bounds: RectEmu{X: 100, Y: 100, CX: 200, CY: 100},
	}
	target := ShapeOptions{
		ID: 2, Geometry: GeomRect,
		Bounds: RectEmu{X: 600, Y: 200, CX: 200, CY: 100},
	}

	_, startSite, endSite := RouteBetween(source, target)

	if startSite != 1 {
		t.Errorf("startSite = %d, want 1 (right)", startSite)
	}
	if endSite != 3 {
		t.Errorf("endSite = %d, want 3 (left)", endSite)
	}
}
