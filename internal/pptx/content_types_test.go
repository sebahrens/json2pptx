package pptx

import (
	"strings"
	"testing"
)

func TestNewContentTypes(t *testing.T) {
	ct := NewContentTypes()
	if len(ct.defaults) != 0 {
		t.Errorf("expected 0 defaults, got %d", len(ct.defaults))
	}
	if len(ct.overrides) != 0 {
		t.Errorf("expected 0 overrides, got %d", len(ct.overrides))
	}
}

func TestParseContentTypes(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="png" ContentType="image/png"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`)

	ct, err := ParseContentTypes(xmlData)
	if err != nil {
		t.Fatalf("ParseContentTypes failed: %v", err)
	}

	if !ct.HasDefault("png") {
		t.Error("expected default for png")
	}

	if ct.Default("png") != "image/png" {
		t.Errorf("expected image/png, got %s", ct.Default("png"))
	}

	if !ct.HasOverride("/ppt/presentation.xml") {
		t.Error("expected override for presentation.xml")
	}

	if ct.Override("/ppt/slides/slide1.xml") != ContentTypeSlide {
		t.Errorf("expected slide content type, got %s", ct.Override("/ppt/slides/slide1.xml"))
	}
}

func TestParseContentTypes_Invalid(t *testing.T) {
	xmlData := []byte(`not valid xml`)
	_, err := ParseContentTypes(xmlData)
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestContentTypes_SetDefault(t *testing.T) {
	ct := NewContentTypes()

	ct.SetDefault("svg", ContentTypeSVG)

	if !ct.HasDefault("svg") {
		t.Error("expected svg default to be set")
	}
	if ct.Default("svg") != ContentTypeSVG {
		t.Errorf("expected %s, got %s", ContentTypeSVG, ct.Default("svg"))
	}
}

func TestContentTypes_SetDefault_NormalizesExtension(t *testing.T) {
	ct := NewContentTypes()

	ct.SetDefault(".SVG", ContentTypeSVG)

	// Should find it with normalized form
	if !ct.HasDefault("svg") {
		t.Error("expected svg default to be set (lowercase)")
	}
	if !ct.HasDefault(".svg") {
		t.Error("expected svg default to be found with dot prefix")
	}
	if !ct.HasDefault("SVG") {
		t.Error("expected svg default to be found with uppercase")
	}
}

func TestContentTypes_EnsureDefault(t *testing.T) {
	ct := NewContentTypes()

	// First call should set the default
	ct.EnsureDefault("svg", ContentTypeSVG)
	if ct.Default("svg") != ContentTypeSVG {
		t.Errorf("expected %s, got %s", ContentTypeSVG, ct.Default("svg"))
	}

	// Second call with different value should be a no-op
	ct.EnsureDefault("svg", "different/type")
	if ct.Default("svg") != ContentTypeSVG {
		t.Errorf("expected original value, got %s", ct.Default("svg"))
	}
}

func TestContentTypes_EnsureDefaultForExtension(t *testing.T) {
	ct := NewContentTypes()

	// Known extension
	ok := ct.EnsureDefaultForExtension("svg")
	if !ok {
		t.Error("expected true for known extension")
	}
	if ct.Default("svg") != ContentTypeSVG {
		t.Errorf("expected %s, got %s", ContentTypeSVG, ct.Default("svg"))
	}

	// Unknown extension
	ok = ct.EnsureDefaultForExtension("xyz")
	if ok {
		t.Error("expected false for unknown extension")
	}
}

func TestContentTypes_EnsureSVG(t *testing.T) {
	ct := NewContentTypes()

	ct.EnsureSVG()

	if !ct.HasDefault("svg") {
		t.Error("expected svg default")
	}
	if ct.Default("svg") != ContentTypeSVG {
		t.Errorf("expected %s, got %s", ContentTypeSVG, ct.Default("svg"))
	}
}

func TestContentTypes_EnsurePNG(t *testing.T) {
	ct := NewContentTypes()

	ct.EnsurePNG()

	if !ct.HasDefault("png") {
		t.Error("expected png default")
	}
	if ct.Default("png") != ContentTypePNG {
		t.Errorf("expected %s, got %s", ContentTypePNG, ct.Default("png"))
	}
}

func TestContentTypes_Override(t *testing.T) {
	ct := NewContentTypes()

	ct.SetOverride("/ppt/slides/slide1.xml", ContentTypeSlide)

	if !ct.HasOverride("/ppt/slides/slide1.xml") {
		t.Error("expected override to exist")
	}

	if ct.Override("/ppt/slides/slide1.xml") != ContentTypeSlide {
		t.Errorf("expected %s, got %s", ContentTypeSlide, ct.Override("/ppt/slides/slide1.xml"))
	}
}

func TestContentTypes_RemoveOverride(t *testing.T) {
	ct := NewContentTypes()
	ct.SetOverride("/ppt/slides/slide1.xml", ContentTypeSlide)

	removed := ct.RemoveOverride("/ppt/slides/slide1.xml")
	if !removed {
		t.Error("expected RemoveOverride to return true")
	}

	if ct.HasOverride("/ppt/slides/slide1.xml") {
		t.Error("expected override to be removed")
	}

	// Remove non-existent
	removed = ct.RemoveOverride("/ppt/slides/slide1.xml")
	if removed {
		t.Error("expected RemoveOverride to return false for non-existent")
	}
}

func TestContentTypes_GetContentType(t *testing.T) {
	ct := NewContentTypes()
	ct.SetDefault("png", ContentTypePNG)
	ct.SetOverride("/ppt/media/special.png", "special/type")

	// Override takes precedence
	if ct.ContentType("/ppt/media/special.png") != "special/type" {
		t.Errorf("expected override, got %s", ct.ContentType("/ppt/media/special.png"))
	}

	// Fall back to default for regular files
	if ct.ContentType("/ppt/media/normal.png") != ContentTypePNG {
		t.Errorf("expected default, got %s", ct.ContentType("/ppt/media/normal.png"))
	}

	// Unknown extension/part
	if ct.ContentType("/ppt/media/unknown.xyz") != "" {
		t.Errorf("expected empty for unknown, got %s", ct.ContentType("/ppt/media/unknown.xyz"))
	}
}

func TestContentTypes_AllDefaults(t *testing.T) {
	ct := NewContentTypes()
	ct.SetDefault("png", ContentTypePNG)
	ct.SetDefault("svg", ContentTypeSVG)

	all := ct.AllDefaults()
	if len(all) != 2 {
		t.Errorf("expected 2 defaults, got %d", len(all))
	}

	// Modifying returned map shouldn't affect original
	all["xyz"] = "test"
	if ct.HasDefault("xyz") {
		t.Error("AllDefaults should return a copy")
	}
}

func TestContentTypes_AllOverrides(t *testing.T) {
	ct := NewContentTypes()
	ct.SetOverride("/ppt/a.xml", "type/a")
	ct.SetOverride("/ppt/b.xml", "type/b")

	all := ct.AllOverrides()
	if len(all) != 2 {
		t.Errorf("expected 2 overrides, got %d", len(all))
	}

	// Modifying returned map shouldn't affect original
	all["/ppt/c.xml"] = "test"
	if ct.HasOverride("/ppt/c.xml") {
		t.Error("AllOverrides should return a copy")
	}
}

func TestContentTypes_Marshal(t *testing.T) {
	ct := NewContentTypes()
	ct.SetDefault("png", ContentTypePNG)
	ct.SetDefault("svg", ContentTypeSVG)
	ct.SetOverride("/ppt/presentation.xml", ContentTypePresentationMain)

	data, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, "<?xml") {
		t.Error("expected XML declaration")
	}
	if !strings.Contains(xml, "Types") {
		t.Error("expected Types element")
	}
	if !strings.Contains(xml, "image/png") {
		t.Error("expected png content type")
	}
	if !strings.Contains(xml, "image/svg+xml") {
		t.Error("expected svg content type")
	}
}

func TestContentTypes_Marshal_Sorted(t *testing.T) {
	ct := NewContentTypes()
	// Add in non-alphabetical order
	ct.SetDefault("z", "type/z")
	ct.SetDefault("a", "type/a")
	ct.SetDefault("m", "type/m")

	ct.SetOverride("/z.xml", "type/z")
	ct.SetOverride("/a.xml", "type/a")
	ct.SetOverride("/m.xml", "type/m")

	data, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	xml := string(data)

	// Check extension order
	posA := strings.Index(xml, `Extension="a"`)
	posM := strings.Index(xml, `Extension="m"`)
	posZ := strings.Index(xml, `Extension="z"`)
	if posA > posM || posM > posZ {
		t.Errorf("expected extensions in order: a=%d, m=%d, z=%d", posA, posM, posZ)
	}

	// Check part name order
	posPartA := strings.Index(xml, `PartName="/a.xml"`)
	posPartM := strings.Index(xml, `PartName="/m.xml"`)
	posPartZ := strings.Index(xml, `PartName="/z.xml"`)
	if posPartA > posPartM || posPartM > posPartZ {
		t.Errorf("expected parts in order: a=%d, m=%d, z=%d", posPartA, posPartM, posPartZ)
	}
}

func TestContentTypes_Clone(t *testing.T) {
	ct := NewContentTypes()
	ct.SetDefault("png", ContentTypePNG)
	ct.SetOverride("/ppt/a.xml", "type/a")

	clone := ct.Clone()

	// Modify original
	ct.SetDefault("svg", ContentTypeSVG)
	ct.SetOverride("/ppt/b.xml", "type/b")

	// Clone should not be affected
	if clone.HasDefault("svg") {
		t.Error("clone should not have svg default")
	}
	if clone.HasOverride("/ppt/b.xml") {
		t.Error("clone should not have b.xml override")
	}

	// Modify clone
	clone.SetDefault("gif", ContentTypeGIF)

	// Original should not be affected
	if ct.HasDefault("gif") {
		t.Error("original should not be affected by clone modification")
	}
}

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"png", "png"},
		{"PNG", "png"},
		{".png", "png"},
		{".PNG", "png"},
		{"", ""},
		{".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeExtension(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeExtension(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtensionForContentType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    string
	}{
		{ContentTypePNG, "png"},
		{ContentTypeSVG, "svg"},
		{ContentTypeJPEG, "jpeg"}, // or "jpg" - either is valid since both map to same type
		{"unknown/type", ""},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := ExtensionForContentType(tt.contentType)
			// For JPEG, either jpeg or jpg is acceptable
			if tt.contentType == ContentTypeJPEG {
				if result != "jpeg" && result != "jpg" {
					t.Errorf("ExtensionForContentType(%q) = %q, want jpeg or jpg", tt.contentType, result)
				}
			} else if result != tt.expected {
				t.Errorf("ExtensionForContentType(%q) = %q, want %q", tt.contentType, result, tt.expected)
			}
		})
	}
}

func TestContentTypes_RoundTrip(t *testing.T) {
	ct := NewContentTypes()
	ct.SetDefault("png", ContentTypePNG)
	ct.SetDefault("svg", ContentTypeSVG)
	ct.SetOverride("/ppt/presentation.xml", ContentTypePresentationMain)
	ct.SetOverride("/ppt/slides/slide1.xml", ContentTypeSlide)

	// Marshal
	data, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Parse
	ct2, err := ParseContentTypes(data)
	if err != nil {
		t.Fatalf("ParseContentTypes failed: %v", err)
	}

	// Verify defaults
	if ct2.Default("png") != ContentTypePNG {
		t.Errorf("png default mismatch: got %s", ct2.Default("png"))
	}
	if ct2.Default("svg") != ContentTypeSVG {
		t.Errorf("svg default mismatch: got %s", ct2.Default("svg"))
	}

	// Verify overrides
	if ct2.Override("/ppt/presentation.xml") != ContentTypePresentationMain {
		t.Errorf("presentation override mismatch: got %s", ct2.Override("/ppt/presentation.xml"))
	}
	if ct2.Override("/ppt/slides/slide1.xml") != ContentTypeSlide {
		t.Errorf("slide override mismatch: got %s", ct2.Override("/ppt/slides/slide1.xml"))
	}
}

func BenchmarkContentTypes_EnsureDefault(b *testing.B) {
	ct := NewContentTypes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct.EnsureDefault("svg", ContentTypeSVG)
	}
}

func BenchmarkContentTypes_ParseMarshal(b *testing.B) {
	ct := NewContentTypes()
	for i := 0; i < 20; i++ {
		ct.SetDefault(string(rune('a'+i)), "type/"+string(rune('a'+i)))
		ct.SetOverride("/part"+string(rune('a'+i))+".xml", "type/"+string(rune('a'+i)))
	}
	data, _ := ct.Marshal()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, _ := ParseContentTypes(data)
		_, _ = parsed.Marshal()
	}
}
