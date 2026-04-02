package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// createTestPPTX creates a minimal PPTX for testing.
func createTestPPTX(t *testing.T, presentationXML, presentationRels string) *Package {
	t.Helper()

	// Create in-memory ZIP
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Add presentation.xml
	presWriter, err := zw.Create("ppt/presentation.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := presWriter.Write([]byte(presentationXML)); err != nil {
		t.Fatal(err)
	}

	// Add presentation.xml.rels
	relsWriter, err := zw.Create("ppt/_rels/presentation.xml.rels")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := relsWriter.Write([]byte(presentationRels)); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	pkg, err := OpenFromBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	return pkg
}

func TestNewSlideEnumerator(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
    <p:sldId id="257" r:id="rId3"/>
    <p:sldId id="258" r:id="rId4"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide3.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	enum, err := NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator failed: %v", err)
	}

	// Test Count
	if got := enum.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3", got)
	}

	// Test Slides order
	slides := enum.Slides()
	expected := []struct {
		index    int
		id       uint32
		rid      string
		partPath string
	}{
		{0, 256, "rId2", "ppt/slides/slide1.xml"},
		{1, 257, "rId3", "ppt/slides/slide2.xml"},
		{2, 258, "rId4", "ppt/slides/slide3.xml"},
	}

	for i, want := range expected {
		got := slides[i]
		if got.Index != want.index {
			t.Errorf("slides[%d].Index = %d, want %d", i, got.Index, want.index)
		}
		if got.ID != want.id {
			t.Errorf("slides[%d].ID = %d, want %d", i, got.ID, want.id)
		}
		if got.RID != want.rid {
			t.Errorf("slides[%d].RID = %q, want %q", i, got.RID, want.rid)
		}
		if got.PartPath != want.partPath {
			t.Errorf("slides[%d].PartPath = %q, want %q", i, got.PartPath, want.partPath)
		}
	}
}

func TestSlideEnumerator_GetByIndex(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
    <p:sldId id="257" r:id="rId3"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	enum, err := NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator failed: %v", err)
	}

	tests := []struct {
		name     string
		index    int
		wantNil  bool
		wantPath string
	}{
		{"first slide", 0, false, "ppt/slides/slide1.xml"},
		{"second slide", 1, false, "ppt/slides/slide2.xml"},
		{"out of range positive", 2, true, ""},
		{"out of range negative", -1, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := enum.ByIndex(tt.index)
			if tt.wantNil {
				if info != nil {
					t.Errorf("GetByIndex(%d) = %+v, want nil", tt.index, info)
				}
			} else {
				if info == nil {
					t.Errorf("GetByIndex(%d) = nil, want non-nil", tt.index)
				} else if info.PartPath != tt.wantPath {
					t.Errorf("GetByIndex(%d).PartPath = %q, want %q", tt.index, info.PartPath, tt.wantPath)
				}
			}
		})
	}
}

func TestSlideEnumerator_GetByRID(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
    <p:sldId id="257" r:id="rId3"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	enum, err := NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator failed: %v", err)
	}

	tests := []struct {
		name      string
		rid       string
		wantNil   bool
		wantIndex int
	}{
		{"rId2", "rId2", false, 0},
		{"rId3", "rId3", false, 1},
		{"not found", "rId99", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := enum.ByRID(tt.rid)
			if tt.wantNil {
				if info != nil {
					t.Errorf("GetByRID(%q) = %+v, want nil", tt.rid, info)
				}
			} else {
				if info == nil {
					t.Errorf("GetByRID(%q) = nil, want non-nil", tt.rid)
				} else if info.Index != tt.wantIndex {
					t.Errorf("GetByRID(%q).Index = %d, want %d", tt.rid, info.Index, tt.wantIndex)
				}
			}
		})
	}
}

func TestSlideEnumerator_GetPartPath(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	enum, err := NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator failed: %v", err)
	}

	if got := enum.PartPath(0); got != "ppt/slides/slide1.xml" {
		t.Errorf("GetPartPath(0) = %q, want %q", got, "ppt/slides/slide1.xml")
	}

	if got := enum.PartPath(1); got != "" {
		t.Errorf("GetPartPath(1) = %q, want empty string", got)
	}
}

func TestSlideEnumerator_GetRelsPath(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	enum, err := NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator failed: %v", err)
	}

	if got := enum.RelsPath(0); got != "ppt/slides/_rels/slide1.xml.rels" {
		t.Errorf("GetRelsPath(0) = %q, want %q", got, "ppt/slides/_rels/slide1.xml.rels")
	}

	if got := enum.RelsPath(1); got != "" {
		t.Errorf("GetRelsPath(1) = %q, want empty string", got)
	}
}

func TestResolveRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		relPath  string
		want     string
	}{
		{"simple relative", "ppt/presentation.xml", "slides/slide1.xml", "ppt/slides/slide1.xml"},
		{"parent reference", "ppt/slides/slide1.xml", "../media/image1.png", "ppt/media/image1.png"},
		{"absolute with leading slash", "ppt/presentation.xml", "/ppt/slides/slide1.xml", "ppt/slides/slide1.xml"},
		{"multiple parent refs", "ppt/slides/_rels/slide1.xml.rels", "../../media/image1.png", "ppt/media/image1.png"},
		{"no directory in base", "presentation.xml", "slides/slide1.xml", "slides/slide1.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRelativePath(tt.basePath, tt.relPath)
			if got != tt.want {
				t.Errorf("resolveRelativePath(%q, %q) = %q, want %q", tt.basePath, tt.relPath, got, tt.want)
			}
		})
	}
}

func TestNewSlideEnumerator_MissingPresentationXML(t *testing.T) {
	// Create empty package without presentation.xml
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	pkg, err := OpenFromBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewSlideEnumerator(pkg)
	if err == nil {
		t.Error("NewSlideEnumerator should fail with missing presentation.xml")
	}
}

func TestNewSlideEnumerator_MissingRelationship(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId99"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	_, err := NewSlideEnumerator(pkg)
	if err == nil {
		t.Error("NewSlideEnumerator should fail with missing relationship")
	}
}

func TestNewSlideEnumerator_WrongRelationshipType(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	_, err := NewSlideEnumerator(pkg)
	if err == nil {
		t.Error("NewSlideEnumerator should fail with wrong relationship type")
	}
}

func TestNewSlideEnumerator_EmptySlideList(t *testing.T) {
	presXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst/>
</p:presentation>`

	presRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
</Relationships>`

	pkg := createTestPPTX(t, presXML, presRels)
	enum, err := NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator failed: %v", err)
	}

	if got := enum.Count(); got != 0 {
		t.Errorf("Count() = %d, want 0", got)
	}

	if got := enum.Slides(); len(got) != 0 {
		t.Errorf("Slides() = %v, want empty slice", got)
	}
}

// TestSlideEnumerator_RealTemplates tests against actual PPTX templates.
func TestSlideEnumerator_RealTemplates(t *testing.T) {
	// Find template directory (go up from internal/pptx to project root)
	templateDir := filepath.Join("..", "..", "templates")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Skip("templates directory not found, skipping real template test")
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatalf("failed to read templates dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pptx" {
			path := filepath.Join(templateDir, entry.Name())
			t.Run(entry.Name(), func(t *testing.T) {
				pkg, closer, err := OpenFile(path)
				if err != nil {
					t.Fatalf("OpenFile failed: %v", err)
				}
				defer func() { _ = closer.Close() }()

				enum, err := NewSlideEnumerator(pkg)
				if err != nil {
					t.Fatalf("NewSlideEnumerator failed: %v", err)
				}

				// Basic validations
				if enum.Count() <= 0 {
					t.Error("expected at least one slide")
				}

				slides := enum.Slides()
				for i, slide := range slides {
					// Verify index matches position
					if slide.Index != i {
						t.Errorf("slide[%d].Index = %d, want %d", i, slide.Index, i)
					}

					// Verify part path is non-empty
					if slide.PartPath == "" {
						t.Errorf("slide[%d].PartPath is empty", i)
					}

					// Verify rels path is computed
					relsPath := enum.RelsPath(i)
					if relsPath == "" {
						t.Errorf("GetRelsPath(%d) is empty", i)
					}

					// Verify GetByIndex works
					byIdx := enum.ByIndex(i)
					if byIdx == nil {
						t.Errorf("GetByIndex(%d) returned nil", i)
					} else if byIdx.PartPath != slide.PartPath {
						t.Errorf("GetByIndex(%d).PartPath mismatch", i)
					}

					// Verify GetByRID works
					byRID := enum.ByRID(slide.RID)
					if byRID == nil {
						t.Errorf("GetByRID(%s) returned nil", slide.RID)
					} else if byRID.PartPath != slide.PartPath {
						t.Errorf("GetByRID(%s).PartPath mismatch", slide.RID)
					}

					// Verify the slide part actually exists in the package
					if !pkg.HasEntry(slide.PartPath) {
						t.Errorf("slide part %q not found in package", slide.PartPath)
					}
				}

				t.Logf("Successfully enumerated %d slides", enum.Count())
			})
		}
	}
}
