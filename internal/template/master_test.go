package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestParseSlideMasterPositions(t *testing.T) {
	// Minimal slide master XML with two placeholders
	masterXML := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="2" name="Title Placeholder"/>
          <p:cNvSpPr/>
          <p:nvPr>
            <p:ph type="title"/>
          </p:nvPr>
        </p:nvSpPr>
        <p:spPr>
          <a:xfrm>
            <a:off x="457200" y="274638"/>
            <a:ext cx="8229600" cy="1143000"/>
          </a:xfrm>
        </p:spPr>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="3" name="Body Placeholder"/>
          <p:cNvSpPr/>
          <p:nvPr>
            <p:ph type="body" idx="1"/>
          </p:nvPr>
        </p:nvSpPr>
        <p:spPr>
          <a:xfrm>
            <a:off x="457200" y="1600200"/>
            <a:ext cx="8229600" cy="4525963"/>
          </a:xfrm>
        </p:spPr>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="4" name="No Placeholder"/>
          <p:cNvSpPr/>
          <p:nvPr/>
        </p:nvSpPr>
        <p:spPr/>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sldMaster>`)

	positions, err := ParseSlideMasterPositions(masterXML)
	if err != nil {
		t.Fatalf("ParseSlideMasterPositions failed: %v", err)
	}

	// Check we got positions for the two placeholders
	// Title should have keys: "type:title"
	// Body should have keys: "type:body", "type:body:idx:1", "idx:1"
	if len(positions) == 0 {
		t.Fatal("expected some positions, got none")
	}

	// Verify title placeholder position
	titlePos, ok := positions["type:title"]
	if !ok {
		t.Error("expected position for type:title")
	} else {
		if titlePos.OffsetX != 457200 {
			t.Errorf("title OffsetX = %d, want 457200", titlePos.OffsetX)
		}
		if titlePos.OffsetY != 274638 {
			t.Errorf("title OffsetY = %d, want 274638", titlePos.OffsetY)
		}
		if titlePos.ExtentCX != 8229600 {
			t.Errorf("title ExtentCX = %d, want 8229600", titlePos.ExtentCX)
		}
		if titlePos.ExtentCY != 1143000 {
			t.Errorf("title ExtentCY = %d, want 1143000", titlePos.ExtentCY)
		}
	}

	// Verify body placeholder position via multiple keys
	bodyPos, ok := positions["type:body"]
	if !ok {
		t.Error("expected position for type:body")
	} else {
		if bodyPos.OffsetX != 457200 || bodyPos.OffsetY != 1600200 {
			t.Errorf("body position = (%d, %d), want (457200, 1600200)",
				bodyPos.OffsetX, bodyPos.OffsetY)
		}
	}

	// Verify index-based key exists
	if _, ok := positions["idx:1"]; !ok {
		t.Error("expected position for idx:1")
	}

	// Verify combined key exists
	if _, ok := positions["type:body:idx:1"]; !ok {
		t.Error("expected position for type:body:idx:1")
	}
}

func TestParseSlideMasterPositions_EmptyMaster(t *testing.T) {
	masterXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree/>
  </p:cSld>
</p:sldMaster>`)

	positions, err := ParseSlideMasterPositions(masterXML)
	if err != nil {
		t.Fatalf("ParseSlideMasterPositions failed: %v", err)
	}

	if len(positions) != 0 {
		t.Errorf("expected empty positions map, got %d entries", len(positions))
	}
}

func TestParseSlideMasterPositions_InvalidXML(t *testing.T) {
	masterXML := []byte(`not valid xml`)

	_, err := ParseSlideMasterPositions(masterXML)
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestMasterTransform_ToBoundingBox(t *testing.T) {
	transform := &MasterTransform{
		OffsetX:  100,
		OffsetY:  200,
		ExtentCX: 300,
		ExtentCY: 400,
	}

	box := transform.ToBoundingBox()

	if box.X != 100 {
		t.Errorf("X = %d, want 100", box.X)
	}
	if box.Y != 200 {
		t.Errorf("Y = %d, want 200", box.Y)
	}
	if box.Width != 300 {
		t.Errorf("Width = %d, want 300", box.Width)
	}
	if box.Height != 400 {
		t.Errorf("Height = %d, want 400", box.Height)
	}
}

func TestLookupMasterPosition(t *testing.T) {
	// Use realistic EMU values: body height must be >= minMasterBodyHeight (1828800)
	// for the untypified body fallback to work.
	masterPositions := map[string]*MasterTransform{
		"type:title":      {OffsetX: 100, OffsetY: 200, ExtentCX: 8229600, ExtentCY: 1143000},
		"type:body":       {OffsetX: 500, OffsetY: 600, ExtentCX: 8229600, ExtentCY: 4525963},
		"type:body:idx:1": {OffsetX: 500, OffsetY: 600, ExtentCX: 8229600, ExtentCY: 4525963},
		"idx:1":           {OffsetX: 500, OffsetY: 600, ExtentCX: 8229600, ExtentCY: 4525963},
	}

	tests := []struct {
		name        string
		placeholder *placeholderXML
		wantFound   bool
		wantX       int64
	}{
		{
			name:        "type only - title",
			placeholder: &placeholderXML{Type: "title"},
			wantFound:   true,
			wantX:       100,
		},
		{
			name:        "type only - body",
			placeholder: &placeholderXML{Type: "body"},
			wantFound:   true,
			wantX:       500,
		},
		{
			name:        "index only",
			placeholder: &placeholderXML{Index: intPtr(1)},
			wantFound:   true,
			wantX:       500,
		},
		{
			name:        "type and index",
			placeholder: &placeholderXML{Type: "body", Index: intPtr(1)},
			wantFound:   true,
			wantX:       500,
		},
		{
			name:        "unknown type",
			placeholder: &placeholderXML{Type: "unknown"},
			wantFound:   false,
		},
		{
			name:        "nil placeholder",
			placeholder: nil,
			wantFound:   false,
		},
		{
			name:        "empty placeholder",
			placeholder: &placeholderXML{},
			wantFound:   false,
		},
		{
			// OOXML fallback: untypified placeholder with high idx falls back to master body.
			// Regression test: Layout7 has <ph idx="12"/> with no type,
			// which should inherit the master's body placeholder bounds.
			name:        "untypified high idx falls back to body",
			placeholder: &placeholderXML{Index: intPtr(12)},
			wantFound:   true,
			wantX:       500, // matches the "type:body" entry
		},
		{
			// Untypified placeholder with idx matching an existing entry should use exact match first
			name:        "untypified idx 1 uses exact match before fallback",
			placeholder: &placeholderXML{Index: intPtr(1)},
			wantFound:   true,
			wantX:       500, // matches "idx:1" exactly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupMasterPosition(masterPositions, tt.placeholder)
			if tt.wantFound {
				if result == nil {
					t.Error("expected to find position, got nil")
				} else if result.OffsetX != tt.wantX {
					t.Errorf("OffsetX = %d, want %d", result.OffsetX, tt.wantX)
				}
			} else {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
			}
		})
	}
}

// TestLookupMasterPosition_TinyBodyFallbackRejected verifies that the untypified
// body fallback in LookupMasterPosition rejects master body placeholders that are
// too small (height < 2 inches / 1828800 EMU). Without this guard, a master with
// a small subtitle-sized body placeholder would propagate its tiny bounds to all
// untypified content placeholders, causing charts and diagrams to render as thumbnails.
// Regression test: SVG diagram thumbnailing with non-standard slide dimensions.
func TestLookupMasterPosition_TinyBodyFallbackRejected(t *testing.T) {
	tinyMaster := map[string]*MasterTransform{
		"type:body": {OffsetX: 500, OffsetY: 600, ExtentCX: 8229600, ExtentCY: 547908}, // ~0.6 inches — too short
	}

	// Untypified placeholder with high idx — should NOT fall back to tiny body
	ph := &placeholderXML{Index: intPtr(12)}
	result := LookupMasterPosition(tinyMaster, ph)
	if result != nil {
		t.Errorf("expected nil for tiny body fallback, got %+v", result)
	}

	// Same placeholder with adequate body height — should succeed
	adequateMaster := map[string]*MasterTransform{
		"type:body": {OffsetX: 500, OffsetY: 600, ExtentCX: 8229600, ExtentCY: 4525963}, // ~5 inches
	}
	result = LookupMasterPosition(adequateMaster, ph)
	if result == nil {
		t.Error("expected non-nil for adequate body fallback")
	}

	// Typed placeholder should always work regardless of height
	// (the height guard only applies to the untypified fallback path)
	typedPH := &placeholderXML{Type: "body"}
	result = LookupMasterPosition(tinyMaster, typedPH)
	if result == nil {
		t.Error("expected non-nil for typed body lookup (guard should not apply)")
	}
}

func TestLookupMasterPosition_NilMap(t *testing.T) {
	result := LookupMasterPosition(nil, &placeholderXML{Type: "title"})
	if result != nil {
		t.Errorf("expected nil for nil map, got %+v", result)
	}
}

func TestResolvePlaceholderBounds(t *testing.T) {
	masterPositions := map[string]*MasterTransform{
		"type:title": {OffsetX: 100, OffsetY: 200, ExtentCX: 300, ExtentCY: 400},
	}

	tests := []struct {
		name            string
		shapeTransform  *transformXML
		placeholder     *placeholderXML
		masterPositions map[string]*MasterTransform
		want            types.BoundingBox
	}{
		{
			name: "shape has own transform - use it",
			shapeTransform: &transformXML{
				Offset:  &offsetXML{X: 1000, Y: 2000},
				Extents: &extentsXML{CX: 3000, CY: 4000},
			},
			placeholder:     &placeholderXML{Type: "title"},
			masterPositions: masterPositions,
			want:            types.BoundingBox{X: 1000, Y: 2000, Width: 3000, Height: 4000},
		},
		{
			name:            "no shape transform - resolve from master",
			shapeTransform:  nil,
			placeholder:     &placeholderXML{Type: "title"},
			masterPositions: masterPositions,
			want:            types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 400},
		},
		{
			name:            "no shape transform, no master - empty bounds",
			shapeTransform:  nil,
			placeholder:     &placeholderXML{Type: "unknown"},
			masterPositions: masterPositions,
			want:            types.BoundingBox{},
		},
		{
			name:            "no shape transform, nil master positions - empty bounds",
			shapeTransform:  nil,
			placeholder:     &placeholderXML{Type: "title"},
			masterPositions: nil,
			want:            types.BoundingBox{},
		},
		{
			name: "shape transform with partial data",
			shapeTransform: &transformXML{
				Offset: &offsetXML{X: 500, Y: 600},
				// No extents
			},
			placeholder:     &placeholderXML{Type: "title"},
			masterPositions: masterPositions,
			want:            types.BoundingBox{X: 500, Y: 600, Width: 0, Height: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePlaceholderBounds(tt.shapeTransform, tt.placeholder, tt.masterPositions, "TestLayout", 0)
			if got != tt.want {
				t.Errorf("ResolvePlaceholderBounds() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveRelativePath(t *testing.T) {
	tests := []struct {
		name         string
		basePath     string
		relativePath string
		want         string
	}{
		{
			name:         "simple relative",
			basePath:     "ppt/slideLayouts",
			relativePath: "../slideMasters/slideMaster1.xml",
			want:         "ppt/slideMasters/slideMaster1.xml",
		},
		{
			name:         "same directory",
			basePath:     "ppt/slideLayouts",
			relativePath: "slideLayout2.xml",
			want:         "ppt/slideLayouts/slideLayout2.xml",
		},
		{
			name:         "two levels up",
			basePath:     "ppt/slideLayouts/nested",
			relativePath: "../../theme/theme1.xml",
			want:         "ppt/theme/theme1.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveRelativePath(tt.basePath, tt.relativePath)
			if got != tt.want {
				t.Errorf("ResolveRelativePath(%q, %q) = %q, want %q",
					tt.basePath, tt.relativePath, got, tt.want)
			}
		})
	}
}

// intPtr is a helper to create an *int for tests
func intPtr(i int) *int {
	return &i
}
