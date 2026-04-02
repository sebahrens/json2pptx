package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestFindMissingCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		layouts []types.LayoutMetadata
		want    []string
	}{
		{
			name: "no layouts — all capabilities missing",
			want: []string{"two-column", "blank-title"},
		},
		{
			name: "has two-column and blank-title — nothing missing",
			layouts: []types.LayoutMetadata{
				{Tags: []string{"content", "two-column", "comparison"}},
				{Tags: []string{"blank-title", "virtual-base"}},
			},
			want: nil,
		},
		{
			name: "content only — two-column and blank-title missing",
			layouts: []types.LayoutMetadata{
				{Tags: []string{"content"}},
			},
			want: []string{"two-column", "blank-title"},
		},
		{
			name: "has blank-title — only two-column missing",
			layouts: []types.LayoutMetadata{
				{Tags: []string{"content"}},
				{Tags: []string{"blank-title"}},
			},
			want: []string{"two-column"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMissingCapabilities(tt.layouts)
			if len(got) != len(tt.want) {
				t.Errorf("findMissingCapabilities() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("findMissingCapabilities()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindBestContentLayoutIndex(t *testing.T) {
	tests := []struct {
		name    string
		layouts []types.LayoutMetadata
		want    int
	}{
		{
			name:    "no layouts",
			layouts: nil,
			want:    -1,
		},
		{
			name: "no content layouts",
			layouts: []types.LayoutMetadata{
				{Tags: []string{"title-slide"}},
			},
			want: -1,
		},
		{
			name: "single content layout",
			layouts: []types.LayoutMetadata{
				{
					Tags: []string{"content"},
					Placeholders: []types.PlaceholderInfo{
						{Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 1000, Height: 1000}},
					},
				},
			},
			want: 0,
		},
		{
			name: "picks largest body area",
			layouts: []types.LayoutMetadata{
				{
					Tags: []string{"content"},
					Placeholders: []types.PlaceholderInfo{
						{Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 1000, Height: 1000}},
					},
				},
				{
					Tags: []string{"content"},
					Placeholders: []types.PlaceholderInfo{
						{Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 2000, Height: 2000}},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBestContentLayoutIndex(tt.layouts)
			if got != tt.want {
				t.Errorf("findBestContentLayoutIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFilterLayoutsForCapabilities(t *testing.T) {
	layouts := []GeneratedLayout{
		{ID: "content-1", Name: "Single Content"},
		{ID: "content-2-50-50", Name: "Two Column (50/50)"},
		{ID: "content-2-60-40", Name: "Two Column (60/40)"},
		{ID: "content-2-40-60", Name: "Two Column (40/60)"},
		{ID: "content-2-70-30", Name: "Two Column (70/30)"},
		{ID: "content-3", Name: "Three Column"},
	}

	t.Run("two-column selects 50/50 plus asymmetric variants", func(t *testing.T) {
		got := filterLayoutsForCapabilities(layouts, []string{"two-column"})
		if len(got) != 3 {
			t.Fatalf("expected 3 layouts, got %d", len(got))
		}
		wantIDs := []string{"content-2-50-50", "content-2-60-40", "content-2-40-60"}
		for i, want := range wantIDs {
			if got[i].ID != want {
				t.Errorf("layout[%d] = %s, want %s", i, got[i].ID, want)
			}
		}
	})

	t.Run("no missing returns empty", func(t *testing.T) {
		got := filterLayoutsForCapabilities(layouts, nil)
		if len(got) != 0 {
			t.Errorf("expected 0 layouts, got %d", len(got))
		}
	})
}

func TestFindNextLayoutNum(t *testing.T) {
	tests := []struct {
		name    string
		layouts []types.LayoutMetadata
		want    int
	}{
		{
			name:    "no layouts — starts at 1",
			layouts: nil,
			want:    1,
		},
		{
			name: "sequential after highest existing",
			layouts: []types.LayoutMetadata{
				{ID: "slideLayout1"},
				{ID: "slideLayout5"},
			},
			want: 6,
		},
		{
			name: "high layout number — increments past it",
			layouts: []types.LayoutMetadata{
				{ID: "slideLayout100"},
			},
			want: 101,
		},
		{
			name: "template with 69 layouts",
			layouts: func() []types.LayoutMetadata {
				var layouts []types.LayoutMetadata
				for i := 1; i <= 69; i++ {
					layouts = append(layouts, types.LayoutMetadata{ID: fmt.Sprintf("slideLayout%d", i)})
				}
				return layouts
			}(),
			want: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findNextLayoutNum(tt.layouts)
			if got != tt.want {
				t.Errorf("findNextLayoutNum() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConvertGeneratedToMetadata(t *testing.T) {
	gl := GeneratedLayout{
		ID:   "content-2-50-50",
		Name: "Two Column (50/50)",
		Placeholders: []GeneratedPlaceholder{
			{
				ID:     "slot1",
				Index:  1,
				Type:   "body",
				Bounds: types.BoundingBox{X: 0, Y: 0, Width: 4000000, Height: 4000000},
				Style:  PlaceholderStyle{FontFamily: "Calibri", FontSize: 1800},
			},
			{
				ID:     "slot2",
				Index:  2,
				Type:   "body",
				Bounds: types.BoundingBox{X: 4200000, Y: 0, Width: 4000000, Height: 4000000},
				Style:  PlaceholderStyle{FontFamily: "Calibri", FontSize: 1800},
			},
		},
	}

	// Test without title placeholder
	metadata := convertGeneratedToMetadata(gl, "slideLayout99", 5, nil)

	// Check basic properties
	if metadata.ID != "slideLayout99" {
		t.Errorf("ID = %q, want %q", metadata.ID, "slideLayout99")
	}
	if metadata.Name != "Two Column (50/50)" {
		t.Errorf("Name = %q, want %q", metadata.Name, "Two Column (50/50)")
	}
	if metadata.Index != 5 {
		t.Errorf("Index = %d, want %d", metadata.Index, 5)
	}

	// Check placeholders (no title, so just 2 content placeholders)
	if len(metadata.Placeholders) != 2 {
		t.Fatalf("Placeholders count = %d, want 2", len(metadata.Placeholders))
	}

	// Check placeholder indices start at 10
	if metadata.Placeholders[0].Index != 10 {
		t.Errorf("Placeholder[0].Index = %d, want 10", metadata.Placeholders[0].Index)
	}
	if metadata.Placeholders[1].Index != 11 {
		t.Errorf("Placeholder[1].Index = %d, want 11", metadata.Placeholders[1].Index)
	}

	// Check font properties are propagated
	if metadata.Placeholders[0].FontFamily != "Calibri" {
		t.Errorf("Placeholder[0].FontFamily = %q, want %q", metadata.Placeholders[0].FontFamily, "Calibri")
	}
	if metadata.Placeholders[0].FontSize != 1800 {
		t.Errorf("Placeholder[0].FontSize = %d, want %d", metadata.Placeholders[0].FontSize, 1800)
	}

	// Test with title placeholder — should be prepended
	titlePH := &types.PlaceholderInfo{
		ID:       "Title 1",
		Type:     types.PlaceholderTitle,
		Index:    0,
		Bounds:   types.BoundingBox{X: 500000, Y: 100000, Width: 8000000, Height: 500000},
		MaxChars: 80,
	}
	metadataWithTitle := convertGeneratedToMetadata(gl, "slideLayout99", 5, titlePH)

	// Should have 3 placeholders: title + 2 content
	if len(metadataWithTitle.Placeholders) != 3 {
		t.Fatalf("Placeholders count with title = %d, want 3", len(metadataWithTitle.Placeholders))
	}
	if metadataWithTitle.Placeholders[0].Type != types.PlaceholderTitle {
		t.Errorf("Placeholder[0].Type = %q, want %q", metadataWithTitle.Placeholders[0].Type, types.PlaceholderTitle)
	}
	if metadataWithTitle.Placeholders[0].ID != "Title 1" {
		t.Errorf("Placeholder[0].ID = %q, want %q", metadataWithTitle.Placeholders[0].ID, "Title 1")
	}

	// Check tags include two-column (ClassifyLayout should detect side-by-side placeholders)
	hasTwoColumn := false
	for _, tag := range metadata.Tags {
		if tag == "two-column" {
			hasTwoColumn = true
		}
	}
	if !hasTwoColumn {
		t.Errorf("Tags = %v, want to contain 'two-column'", metadata.Tags)
	}
}

func TestGenerateLayoutXMLBytes(t *testing.T) {
	gl := GeneratedLayout{
		ID:   "content-2-50-50",
		Name: "Two Column (50/50)",
		Placeholders: []GeneratedPlaceholder{
			{
				ID:     "slot1",
				Index:  1,
				Type:   "body",
				Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 4000000, Height: 4000000},
				Style:  PlaceholderStyle{FontFamily: "Calibri", FontSize: 1800, MarginLeft: 91440},
			},
			{
				ID:     "slot2",
				Index:  2,
				Type:   "body",
				Bounds: types.BoundingBox{X: 4640000, Y: 1600200, Width: 4000000, Height: 4000000},
				Style:  PlaceholderStyle{FontFamily: "Calibri", FontSize: 1800, MarginLeft: 91440},
			},
		},
	}

	xmlBytes := GenerateLayoutXMLBytes(gl, 99, "Title 1")
	xmlStr := string(xmlBytes)

	// Check XML declaration
	if !strings.HasPrefix(xmlStr, `<?xml version="1.0"`) {
		t.Error("missing XML declaration")
	}

	// Check layout name
	if !strings.Contains(xmlStr, `name="Two Column (50/50)"`) {
		t.Error("missing layout name")
	}

	// Check placeholder indices are 10+
	if !strings.Contains(xmlStr, `idx="10"`) {
		t.Error("missing placeholder idx=10")
	}
	if !strings.Contains(xmlStr, `idx="11"`) {
		t.Error("missing placeholder idx=11")
	}

	// Check bodyPr has margins
	if !strings.Contains(xmlStr, `lIns="91440"`) {
		t.Error("missing bodyPr margin lIns")
	}

	// Check lstStyle has font
	if !strings.Contains(xmlStr, `typeface="Calibri"`) {
		t.Error("missing font typeface in lstStyle")
	}

	// Check title placeholder shape
	if !strings.Contains(xmlStr, `type="title"`) {
		t.Error("missing title placeholder in layout XML")
	}
	if !strings.Contains(xmlStr, `name="Title 1"`) {
		t.Error("missing title shape name in layout XML")
	}

	// Check slot markers
	if !strings.Contains(xmlStr, `::slot1::`) {
		t.Error("missing ::slot1:: marker")
	}
	if !strings.Contains(xmlStr, `::slot2::`) {
		t.Error("missing ::slot2:: marker")
	}
}

func TestGenerateLayoutRelsXMLBytes(t *testing.T) {
	t.Run("default master", func(t *testing.T) {
		xmlBytes := GenerateLayoutRelsXMLBytes("../slideMasters/slideMaster1.xml")
		xmlStr := string(xmlBytes)

		if !strings.HasPrefix(xmlStr, `<?xml version="1.0"`) {
			t.Error("missing XML declaration")
		}
		if !strings.Contains(xmlStr, `slideMaster1.xml`) {
			t.Error("missing slide master reference")
		}
	})

	t.Run("custom master", func(t *testing.T) {
		xmlBytes := GenerateLayoutRelsXMLBytes("../slideMasters/slideMaster3.xml")
		xmlStr := string(xmlBytes)

		if !strings.Contains(xmlStr, `slideMaster3.xml`) {
			t.Error("expected slideMaster3.xml reference")
		}
		if strings.Contains(xmlStr, `slideMaster1.xml`) {
			t.Error("should not contain slideMaster1.xml")
		}
	})
}

// TestSynthesizeIfNeeded_NativePreserved verifies AC17: templates with native
// two-column layouts don't get synthetic ones added.
func TestSynthesizeIfNeeded_NativePreserved(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "slideLayout1",
				Name: "Content",
				Tags: []string{"content"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 8000000, Height: 4000000}},
				},
			},
			{
				ID:   "slideLayout2",
				Name: "Two Content",
				Tags: []string{"content", "two-column", "comparison"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Width: 4000000, Height: 4000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4500000, Width: 4000000, Height: 4000000}},
				},
			},
			{
				ID:   "slideLayout3",
				Name: "Blank",
				Tags: []string{"blank-title"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 500000}},
				},
			},
		},
	}

	SynthesizeIfNeeded(nil, analysis)

	if analysis.Synthesis != nil {
		t.Error("expected nil Synthesis for template with native two-column and blank-title layouts")
	}
	if len(analysis.Layouts) != 3 {
		t.Errorf("layout count = %d, want 3 (no synthetic layouts added)", len(analysis.Layouts))
	}
}

// TestSynthesizeIfNeeded_RealTemplate verifies AC15/AC16: synthesis generates
// two-column layouts with styled XML for templates that lack them.
// Uses a real template but strips the two-column layout to simulate a template
// that lacks the capability (since all our test templates include two-column).
func TestSynthesizeIfNeeded_RealTemplate(t *testing.T) {
	templatePath := filepath.Join("testdata", "templates", "midnight-blue.pptx")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		templatePath = filepath.Join("..", "..", "testdata", "templates", "midnight-blue.pptx")
	}
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("midnight-blue.pptx not found in testdata/templates")
	}

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatalf("failed to parse layouts: %v", err)
	}

	// Strip any existing two-column and blank-title layouts to simulate a template without them.
	var filtered []types.LayoutMetadata
	for _, l := range layouts {
		skip := false
		for _, tag := range l.Tags {
			if tag == "two-column" || tag == "blank-title" {
				skip = true
			}
		}
		if !skip {
			filtered = append(filtered, l)
		}
	}

	// Ensure we have at least one content layout remaining
	hasContent := false
	for _, l := range filtered {
		for _, tag := range l.Tags {
			if tag == "content" {
				hasContent = true
			}
		}
	}
	if !hasContent {
		t.Skip("no content layout remaining after stripping two-column layouts")
	}

	originalCount := len(filtered)

	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Layouts:      filtered,
	}

	SynthesizeIfNeeded(reader, analysis)

	// AC15: SynthesisManifest should be populated
	if analysis.Synthesis == nil {
		t.Fatal("expected non-nil Synthesis for template without two-column layout")
	}
	if len(analysis.Synthesis.SyntheticFiles) == 0 {
		t.Fatal("expected SyntheticFiles to be populated")
	}

	// Should have layout XML + rels XML: 2 files per layout × (1 blank-title + 3 two-column variants) = 8
	if len(analysis.Synthesis.SyntheticFiles) != 8 {
		t.Errorf("SyntheticFiles count = %d, want 8", len(analysis.Synthesis.SyntheticFiles))
	}

	// Layouts should have four more entries (blank-title + 50/50, 60/40, 40/60)
	if len(analysis.Layouts) != originalCount+4 {
		t.Errorf("layout count = %d, want %d", len(analysis.Layouts), originalCount+4)
	}

	// The last new layout should have two-column tag
	newLayout := analysis.Layouts[len(analysis.Layouts)-1]
	hasTwoColumn := false
	for _, tag := range newLayout.Tags {
		if tag == "two-column" {
			hasTwoColumn = true
		}
	}
	if !hasTwoColumn {
		t.Errorf("synthetic layout tags = %v, expected to contain 'two-column'", newLayout.Tags)
	}

	// Check layout has 3 placeholders: title + 2 content slots
	if len(newLayout.Placeholders) != 3 {
		t.Errorf("synthetic layout placeholders = %d, want 3", len(newLayout.Placeholders))
	}

	// First placeholder should be the title from the base layout
	if len(newLayout.Placeholders) > 0 && newLayout.Placeholders[0].Type != types.PlaceholderTitle {
		t.Errorf("synthetic layout first placeholder type = %q, want %q",
			newLayout.Placeholders[0].Type, types.PlaceholderTitle)
	}

	// AC16: Check that the generated XML contains styled elements (two-column layouts).
	// Blank-title layouts have no content placeholders, so idx/lstStyle checks only
	// apply to two-column layouts.
	for path, data := range analysis.Synthesis.SyntheticFiles {
		if strings.HasSuffix(path, ".xml") && !strings.Contains(path, "_rels") {
			xmlStr := string(data)
			// All layouts should have bodyPr (may be empty or with margins)
			if !strings.Contains(xmlStr, "bodyPr") {
				t.Errorf("synthetic layout XML %s missing bodyPr element", path)
			}
			// Two-column layouts have content placeholders with idx >= 10.
			// Blank-title layouts have no content slots (only title + utility).
			isBlankTitle := strings.Contains(xmlStr, `name="Blank + Title"`)
			if !isBlankTitle && !strings.Contains(xmlStr, `idx="10"`) {
				t.Errorf("synthetic layout XML %s missing idx=10", path)
			}
			// Should have font information from the template
			if !strings.Contains(xmlStr, "typeface=") {
				t.Logf("Note: synthetic layout XML %s has no typeface (base layout may not specify font)", path)
			}
		}
	}
}

// TestSynthesizeBlankTitle verifies blank-title synthesis produces the correct layout.
func TestSynthesizeBlankTitle(t *testing.T) {
	titlePH := &types.PlaceholderInfo{
		ID:     "Title 1",
		Type:   types.PlaceholderTitle,
		Index:  0,
		Bounds: types.BoundingBox{X: 500000, Y: 100000, Width: 8000000, Height: 500000},
	}
	footerPHs := []types.PlaceholderInfo{
		{ID: "Footer Placeholder 4", Type: types.PlaceholderOther, Index: 11},
		{ID: "Slide Number Placeholder 5", Type: types.PlaceholderOther, Index: 12},
	}

	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "slideLayout1",
				Name: "Content",
				Tags: []string{"content"},
				Placeholders: []types.PlaceholderInfo{
					*titlePH,
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 8000000, Height: 4000000}},
					footerPHs[0],
					footerPHs[1],
				},
			},
			{
				ID:   "slideLayout2",
				Name: "Two Content",
				Tags: []string{"content", "two-column", "comparison"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Width: 4000000, Height: 4000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4500000, Width: 4000000, Height: 4000000}},
				},
			},
		},
	}

	SynthesizeIfNeeded(nil, analysis)

	if analysis.Synthesis == nil {
		t.Fatal("expected non-nil Synthesis")
	}

	// Should have 2 synthetic files (layout XML + rels)
	if len(analysis.Synthesis.SyntheticFiles) != 2 {
		t.Errorf("SyntheticFiles count = %d, want 2", len(analysis.Synthesis.SyntheticFiles))
	}

	// Should have 1 new layout added
	if len(analysis.Layouts) != 3 {
		t.Fatalf("layout count = %d, want 3", len(analysis.Layouts))
	}

	blankLayout := analysis.Layouts[2]

	// Check name
	if blankLayout.Name != "Blank + Title" {
		t.Errorf("Name = %q, want %q", blankLayout.Name, "Blank + Title")
	}

	// Check tags include blank-title and virtual-base
	hasBlankTitle := false
	hasVirtualBase := false
	for _, tag := range blankLayout.Tags {
		if tag == "blank-title" {
			hasBlankTitle = true
		}
		if tag == "virtual-base" {
			hasVirtualBase = true
		}
	}
	if !hasBlankTitle {
		t.Errorf("Tags = %v, want to contain 'blank-title'", blankLayout.Tags)
	}
	if !hasVirtualBase {
		t.Errorf("Tags = %v, want to contain 'virtual-base'", blankLayout.Tags)
	}

	// Check placeholders: title + 2 utility (footer, slide number)
	if len(blankLayout.Placeholders) != 3 {
		t.Fatalf("Placeholders count = %d, want 3", len(blankLayout.Placeholders))
	}
	if blankLayout.Placeholders[0].Type != types.PlaceholderTitle {
		t.Errorf("Placeholder[0].Type = %q, want %q", blankLayout.Placeholders[0].Type, types.PlaceholderTitle)
	}
	if blankLayout.Placeholders[1].Type != types.PlaceholderOther {
		t.Errorf("Placeholder[1].Type = %q, want %q", blankLayout.Placeholders[1].Type, types.PlaceholderOther)
	}
	if blankLayout.Placeholders[2].Type != types.PlaceholderOther {
		t.Errorf("Placeholder[2].Type = %q, want %q", blankLayout.Placeholders[2].Type, types.PlaceholderOther)
	}

	// Check generated XML
	for path, data := range analysis.Synthesis.SyntheticFiles {
		if strings.HasSuffix(path, ".xml") && !strings.Contains(path, "_rels") {
			xmlStr := string(data)
			// Should have title placeholder
			if !strings.Contains(xmlStr, `type="title"`) {
				t.Error("blank-title XML missing title placeholder")
			}
			if !strings.Contains(xmlStr, `name="title"`) {
				t.Error("blank-title XML missing normalized title shape name")
			}
			// Should have footer placeholder
			if !strings.Contains(xmlStr, `type="ftr"`) {
				t.Error("blank-title XML missing footer placeholder")
			}
			// Should have slide number placeholder
			if !strings.Contains(xmlStr, `type="sldNum"`) {
				t.Error("blank-title XML missing slide number placeholder")
			}
			// Should NOT have content placeholders (idx >= 10)
			if strings.Contains(xmlStr, `idx="10"`) {
				t.Error("blank-title XML should not have content placeholder idx=10")
			}
		}
	}
}

// TestGenerateBlankTitleLayoutXMLBytes verifies blank-title XML generation.
func TestGenerateBlankTitleLayoutXMLBytes(t *testing.T) {
	titlePH := &types.PlaceholderInfo{
		ID:     "Title 1",
		Type:   types.PlaceholderTitle,
		Bounds: types.BoundingBox{X: 500000, Y: 100000, Width: 8000000, Height: 500000},
	}
	footerPHs := []types.PlaceholderInfo{
		{ID: "Footer Placeholder 4", Type: types.PlaceholderOther, Index: 11},
		{ID: "Slide Number Placeholder 5", Type: types.PlaceholderOther, Index: 12},
	}

	xmlBytes := GenerateBlankTitleLayoutXMLBytes(titlePH, footerPHs, 99)
	xmlStr := string(xmlBytes)

	if !strings.HasPrefix(xmlStr, `<?xml version="1.0"`) {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(xmlStr, `name="Blank + Title"`) {
		t.Error("missing layout name")
	}
	if !strings.Contains(xmlStr, `type="title"`) {
		t.Error("missing title placeholder")
	}
	if !strings.Contains(xmlStr, `type="ftr"`) {
		t.Error("missing footer placeholder")
	}
	if !strings.Contains(xmlStr, `type="sldNum"`) {
		t.Error("missing slide number placeholder")
	}
	// Title should have explicit bounds
	if !strings.Contains(xmlStr, `x="500000"`) {
		t.Error("missing title X position")
	}
}

func TestFindBestContentLayoutIndex_ZeroBoundsFallback(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			ID:   "slideLayout1",
			Name: "Title",
			Tags: []string{"title"},
		},
		{
			ID:   "slideLayout2",
			Name: "Content",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				// Zero bounds — simulates unresolved master inheritance
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{}},
			},
		},
	}

	idx := findBestContentLayoutIndex(layouts)
	if idx != 1 {
		t.Errorf("findBestContentLayoutIndex = %d, want 1 (fallback to first content layout with body placeholder)", idx)
	}
}

func TestSynthesizeTwoColumn_ZeroBoundsProducesContent(t *testing.T) {
	// Simulate a template where the content layout has zero-area body bounds.
	// Synthesis should still produce non-empty two-column layouts using fallback bounds.
	analysis := &types.TemplateAnalysis{
		TemplatePath: "test-zero-bounds.pptx",
		Layouts: []types.LayoutMetadata{
			{
				ID:   "slideLayout1",
				Name: "Content",
				Tags: []string{"content"},
				Placeholders: []types.PlaceholderInfo{
					{
						ID:     "title",
						Type:   types.PlaceholderTitle,
						Bounds: types.BoundingBox{X: 457200, Y: 274638, Width: 10515600, Height: 1143000},
					},
					{
						ID:     "body",
						Type:   types.PlaceholderBody,
						Bounds: types.BoundingBox{}, // Zero bounds
					},
				},
			},
		},
	}

	SynthesizeIfNeeded(nil, analysis)

	if analysis.Synthesis == nil {
		t.Fatal("expected non-nil Synthesis even with zero-area body placeholder")
	}
	if len(analysis.Synthesis.SyntheticFiles) == 0 {
		t.Fatal("expected SyntheticFiles to be populated")
	}

	// Should have synthesized two-column layouts
	hasTwoColumn := false
	for _, layout := range analysis.Layouts {
		for _, tag := range layout.Tags {
			if tag == "two-column" {
				hasTwoColumn = true
				// Verify placeholders have non-zero bounds
				for _, ph := range layout.Placeholders {
					if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
						if ph.Bounds.Width == 0 || ph.Bounds.Height == 0 {
							t.Errorf("synthesized two-column placeholder %q has zero bounds", ph.ID)
						}
					}
				}
			}
		}
	}
	if !hasTwoColumn {
		t.Error("expected at least one two-column layout to be synthesized")
	}
}

func TestDefaultContentAreaFromTitle(t *testing.T) {
	t.Run("with title placeholder", func(t *testing.T) {
		titlePH := &types.PlaceholderInfo{
			Type:   types.PlaceholderTitle,
			Bounds: types.BoundingBox{X: 457200, Y: 274638, Width: 10515600, Height: 1143000},
		}
		area := defaultContentAreaFromTitle(titlePH)

		if area.X != 457200 {
			t.Errorf("X = %d, want 457200", area.X)
		}
		expectedY := int64(274638 + 1143000)
		if area.Y != expectedY {
			t.Errorf("Y = %d, want %d", area.Y, expectedY)
		}
		if area.Width != 10515600 {
			t.Errorf("Width = %d, want 10515600", area.Width)
		}
		if area.Height <= 0 {
			t.Errorf("Height = %d, want > 0", area.Height)
		}
	})

	t.Run("without title placeholder", func(t *testing.T) {
		area := defaultContentAreaFromTitle(nil)

		if area.Width <= 0 {
			t.Errorf("Width = %d, want > 0", area.Width)
		}
		if area.Height <= 0 {
			t.Errorf("Height = %d, want > 0", area.Height)
		}
	})
}
