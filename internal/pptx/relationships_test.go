package pptx

import (
	"strings"
	"testing"
)

func TestNewRelationships(t *testing.T) {
	r := NewRelationships()
	if r.Count() != 0 {
		t.Errorf("expected 0 relationships, got %d", r.Count())
	}
}

func TestParseRelationships(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps" Target="presProps.xml"/>
</Relationships>`)

	r, err := ParseRelationships(xmlData)
	if err != nil {
		t.Fatalf("ParseRelationships failed: %v", err)
	}

	if r.Count() != 3 {
		t.Errorf("expected 3 relationships, got %d", r.Count())
	}

	// Check maxID was calculated correctly (highest is rId5)
	nextID := r.NextID()
	if nextID != "rId6" {
		t.Errorf("expected next ID rId6, got %s", nextID)
	}
}

func TestParseRelationships_Invalid(t *testing.T) {
	xmlData := []byte(`not valid xml`)
	_, err := ParseRelationships(xmlData)
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestRelationships_Add(t *testing.T) {
	r := NewRelationships()

	id1 := r.Add(RelTypeSlide, "slides/slide1.xml")
	if id1 != "rId1" {
		t.Errorf("expected rId1, got %s", id1)
	}

	id2 := r.Add(RelTypeSlide, "slides/slide2.xml")
	if id2 != "rId2" {
		t.Errorf("expected rId2, got %s", id2)
	}

	if r.Count() != 2 {
		t.Errorf("expected 2 relationships, got %d", r.Count())
	}
}

func TestRelationships_Add_Deduplication(t *testing.T) {
	r := NewRelationships()

	id1 := r.Add(RelTypeSlide, "slides/slide1.xml")
	id2 := r.Add(RelTypeSlide, "slides/slide1.xml") // Same target

	if id1 != id2 {
		t.Errorf("expected same ID for duplicate target, got %s and %s", id1, id2)
	}

	if r.Count() != 1 {
		t.Errorf("expected 1 relationship (deduplicated), got %d", r.Count())
	}
}

func TestRelationships_AddWithID(t *testing.T) {
	r := NewRelationships()

	err := r.AddWithID("rId5", RelTypeSlide, "slides/slide1.xml")
	if err != nil {
		t.Fatalf("AddWithID failed: %v", err)
	}

	// Next allocated ID should be rId6
	nextID := r.AllocID()
	if nextID != "rId6" {
		t.Errorf("expected rId6, got %s", nextID)
	}
}

func TestRelationships_AddWithID_Duplicate(t *testing.T) {
	r := NewRelationships()

	_ = r.AddWithID("rId1", RelTypeSlide, "slides/slide1.xml")
	err := r.AddWithID("rId1", RelTypeSlide, "slides/slide2.xml")
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestRelationships_AddExternal(t *testing.T) {
	r := NewRelationships()

	id := r.AddExternal(RelTypeHyperlink, "https://example.com")

	rel := r.Get(id)
	if rel == nil {
		t.Fatal("expected relationship to exist")
	}

	if rel.TargetMode != "External" {
		t.Errorf("expected TargetMode 'External', got %q", rel.TargetMode)
	}
}

func TestRelationships_Get(t *testing.T) {
	r := NewRelationships()
	id := r.Add(RelTypeSlide, "slides/slide1.xml")

	rel := r.Get(id)
	if rel == nil {
		t.Fatal("expected relationship to exist")
	}

	if rel.Target != "slides/slide1.xml" {
		t.Errorf("expected target 'slides/slide1.xml', got %q", rel.Target)
	}

	if rel.Type != RelTypeSlide {
		t.Errorf("expected type %q, got %q", RelTypeSlide, rel.Type)
	}
}

func TestRelationships_Get_NotFound(t *testing.T) {
	r := NewRelationships()
	rel := r.Get("rId99")
	if rel != nil {
		t.Error("expected nil for non-existent relationship")
	}
}

func TestRelationships_FindByTarget(t *testing.T) {
	r := NewRelationships()
	id := r.Add(RelTypeSlide, "slides/slide1.xml")

	found := r.FindByTarget("slides/slide1.xml")
	if found != id {
		t.Errorf("expected %s, got %s", id, found)
	}

	notFound := r.FindByTarget("nonexistent.xml")
	if notFound != "" {
		t.Errorf("expected empty string for non-existent target, got %s", notFound)
	}
}

func TestRelationships_FindByType(t *testing.T) {
	r := NewRelationships()
	r.Add(RelTypeSlide, "slides/slide1.xml")
	r.Add(RelTypeSlide, "slides/slide2.xml")
	r.Add(RelTypeTheme, "theme/theme1.xml")

	slides := r.FindByType(RelTypeSlide)
	if len(slides) != 2 {
		t.Errorf("expected 2 slide relationships, got %d", len(slides))
	}

	themes := r.FindByType(RelTypeTheme)
	if len(themes) != 1 {
		t.Errorf("expected 1 theme relationship, got %d", len(themes))
	}

	empty := r.FindByType("nonexistent")
	if len(empty) != 0 {
		t.Errorf("expected 0 relationships, got %d", len(empty))
	}
}

func TestRelationships_Remove(t *testing.T) {
	r := NewRelationships()
	id := r.Add(RelTypeSlide, "slides/slide1.xml")

	removed := r.Remove(id)
	if !removed {
		t.Error("expected Remove to return true")
	}

	if r.Count() != 0 {
		t.Errorf("expected 0 relationships after removal, got %d", r.Count())
	}

	if r.Get(id) != nil {
		t.Error("expected nil after removal")
	}

	if r.FindByTarget("slides/slide1.xml") != "" {
		t.Error("expected empty target lookup after removal")
	}
}

func TestRelationships_Remove_NotFound(t *testing.T) {
	r := NewRelationships()
	removed := r.Remove("rId99")
	if removed {
		t.Error("expected Remove to return false for non-existent ID")
	}
}

func TestRelationships_Remove_UpdatesIndices(t *testing.T) {
	r := NewRelationships()
	r.Add(RelTypeSlide, "slides/slide1.xml")
	id2 := r.Add(RelTypeSlide, "slides/slide2.xml")
	id3 := r.Add(RelTypeSlide, "slides/slide3.xml")

	// Remove middle relationship
	r.Remove(id2)

	// id3 should still be accessible
	rel := r.Get(id3)
	if rel == nil {
		t.Fatal("expected relationship to exist after removing another")
	}
	if rel.Target != "slides/slide3.xml" {
		t.Errorf("expected target 'slides/slide3.xml', got %q", rel.Target)
	}
}

func TestRelationships_All(t *testing.T) {
	r := NewRelationships()
	r.Add(RelTypeSlide, "slides/slide1.xml")
	r.Add(RelTypeSlide, "slides/slide2.xml")

	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2 relationships, got %d", len(all))
	}

	// Modifying the returned slice shouldn't affect the original
	all[0].Target = "modified"
	rel := r.Get("rId1")
	if rel.Target == "modified" {
		t.Error("All() should return a copy, not the original slice")
	}
}

func TestRelationships_Marshal(t *testing.T) {
	r := NewRelationships()
	r.Add(RelTypeSlide, "slides/slide1.xml")
	r.Add(RelTypeTheme, "theme/theme1.xml")

	data, err := r.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check basic structure
	xml := string(data)
	if !strings.Contains(xml, "<?xml") {
		t.Error("expected XML declaration")
	}
	if !strings.Contains(xml, "Relationships") {
		t.Error("expected Relationships element")
	}
	if !strings.Contains(xml, "slides/slide1.xml") {
		t.Error("expected slide1 target")
	}
}

func TestRelationships_Marshal_Sorted(t *testing.T) {
	r := NewRelationships()
	// Add out of order using specific IDs
	_ = r.AddWithID("rId3", RelTypeSlide, "slides/slide3.xml")
	_ = r.AddWithID("rId1", RelTypeSlide, "slides/slide1.xml")
	_ = r.AddWithID("rId2", RelTypeSlide, "slides/slide2.xml")

	data, err := r.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	xml := string(data)
	pos1 := strings.Index(xml, "rId1")
	pos2 := strings.Index(xml, "rId2")
	pos3 := strings.Index(xml, "rId3")

	if pos1 > pos2 || pos2 > pos3 {
		t.Errorf("expected rIds in order: pos1=%d, pos2=%d, pos3=%d", pos1, pos2, pos3)
	}
}

func TestRelationships_Clone(t *testing.T) {
	r := NewRelationships()
	r.Add(RelTypeSlide, "slides/slide1.xml")

	clone := r.Clone()

	// Modify original
	r.Add(RelTypeSlide, "slides/slide2.xml")

	// Clone should not be affected
	if clone.Count() != 1 {
		t.Errorf("clone should have 1 relationship, got %d", clone.Count())
	}

	// Modify clone
	clone.Add(RelTypeSlide, "slides/slide3.xml")

	// Original should not be affected
	if r.Count() != 2 {
		t.Errorf("original should have 2 relationships, got %d", r.Count())
	}
}

func TestGetRelsPath(t *testing.T) {
	tests := []struct {
		partPath string
		expected string
	}{
		{"ppt/slides/slide1.xml", "ppt/slides/_rels/slide1.xml.rels"},
		{"ppt/presentation.xml", "ppt/_rels/presentation.xml.rels"},
		{"[Content_Types].xml", "_rels/[Content_Types].xml.rels"},
		{"simple.xml", "_rels/simple.xml.rels"},
	}

	for _, tt := range tests {
		t.Run(tt.partPath, func(t *testing.T) {
			result := GetRelsPath(tt.partPath)
			if result != tt.expected {
				t.Errorf("GetRelsPath(%q) = %q, want %q", tt.partPath, result, tt.expected)
			}
		})
	}
}

func TestGetPartPath(t *testing.T) {
	tests := []struct {
		relsPath string
		expected string
	}{
		{"ppt/slides/_rels/slide1.xml.rels", "ppt/slides/slide1.xml"},
		{"ppt/_rels/presentation.xml.rels", "ppt/presentation.xml"},
		{"_rels/.rels", ""},
		{"notarels.xml", "notarels.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.relsPath, func(t *testing.T) {
			result := GetPartPath(tt.relsPath)
			if result != tt.expected {
				t.Errorf("GetPartPath(%q) = %q, want %q", tt.relsPath, result, tt.expected)
			}
		})
	}
}

func TestPackageRels(t *testing.T) {
	if PackageRels() != "_rels/.rels" {
		t.Errorf("expected '_rels/.rels', got %q", PackageRels())
	}
}

func TestPresentationRels(t *testing.T) {
	if PresentationRels() != "ppt/_rels/presentation.xml.rels" {
		t.Errorf("expected 'ppt/_rels/presentation.xml.rels', got %q", PresentationRels())
	}
}

func TestParseRelID(t *testing.T) {
	tests := []struct {
		id       string
		expected int
	}{
		{"rId1", 1},
		{"rId10", 10},
		{"rId999", 999},
		{"invalid", 0},
		{"rId", 0},
		{"RID1", 0}, // case-sensitive
		{"rIdABC", 0},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := parseRelID(tt.id)
			if result != tt.expected {
				t.Errorf("parseRelID(%q) = %d, want %d", tt.id, result, tt.expected)
			}
		})
	}
}

func TestRelationships_AllocID_Sequential(t *testing.T) {
	r := NewRelationships()

	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		ids[i] = r.AllocID()
	}

	expected := []string{"rId1", "rId2", "rId3", "rId4", "rId5", "rId6", "rId7", "rId8", "rId9", "rId10"}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("id[%d] = %s, want %s", i, id, expected[i])
		}
	}
}

func TestRelationships_NextID(t *testing.T) {
	r := NewRelationships()

	// NextID shouldn't actually allocate
	next1 := r.NextID()
	next2 := r.NextID()
	if next1 != next2 {
		t.Errorf("NextID should be consistent, got %s and %s", next1, next2)
	}

	// AllocID should change NextID
	r.AllocID()
	next3 := r.NextID()
	if next3 == next1 {
		t.Error("NextID should change after AllocID")
	}
}

// Benchmark relationship operations
func BenchmarkRelationships_Add(b *testing.B) {
	r := NewRelationships()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Add(RelTypeSlide, "slides/slide.xml")
	}
}

func BenchmarkRelationships_ParseMarshal(b *testing.B) {
	// Create a relationships file with many entries
	r := NewRelationships()
	for i := 1; i <= 100; i++ {
		r.Add(RelTypeSlide, "slides/slide.xml")
	}
	data, _ := r.Marshal()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, _ := ParseRelationships(data)
		_, _ = parsed.Marshal()
	}
}
