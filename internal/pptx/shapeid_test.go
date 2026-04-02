package pptx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewShapeIDAllocator(t *testing.T) {
	tests := []struct {
		name      string
		slideXML  string
		wantMaxID uint32
	}{
		{
			name:      "empty slide",
			slideXML:  `<p:sld></p:sld>`,
			wantMaxID: 0,
		},
		{
			name: "single shape",
			slideXML: `<p:sld>
				<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/></p:nvSpPr></p:sp>
			</p:sld>`,
			wantMaxID: 2,
		},
		{
			name: "multiple shapes",
			slideXML: `<p:sld>
				<p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
				<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/></p:nvSpPr></p:sp>
				<p:sp><p:nvSpPr><p:cNvPr id="3" name="Subtitle"/></p:nvSpPr></p:sp>
				<p:pic><p:nvPicPr><p:cNvPr id="5" name="Picture"/></p:nvPicPr></p:pic>
			</p:sld>`,
			wantMaxID: 5,
		},
		{
			name: "gaps in IDs",
			slideXML: `<p:sld>
				<p:cNvPr id="1" name=""/>
				<p:cNvPr id="100" name=""/>
				<p:cNvPr id="50" name=""/>
			</p:sld>`,
			wantMaxID: 100,
		},
		{
			name: "with whitespace variations",
			slideXML: `<p:sld>
				<p:cNvPr  id="10" name=""/>
				<p:cNvPr	id="20" name=""/>
			</p:sld>`,
			wantMaxID: 20,
		},
		{
			name:      "no shapes",
			slideXML:  `<p:sld><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`,
			wantMaxID: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := NewShapeIDAllocator([]byte(tt.slideXML))
			if got := alloc.MaxID(); got != tt.wantMaxID {
				t.Errorf("MaxID() = %d, want %d", got, tt.wantMaxID)
			}
		})
	}
}

func TestShapeIDAllocator_Alloc(t *testing.T) {
	slideXML := `<p:sld>
		<p:cNvPr id="1" name=""/>
		<p:cNvPr id="3" name=""/>
		<p:cNvPr id="5" name=""/>
	</p:sld>`

	alloc := NewShapeIDAllocator([]byte(slideXML))

	// MaxID should be 5
	if got := alloc.MaxID(); got != 5 {
		t.Errorf("MaxID() = %d, want 5", got)
	}

	// First allocation should return 6
	if got := alloc.Alloc(); got != 6 {
		t.Errorf("Alloc() = %d, want 6", got)
	}

	// Second allocation should return 7
	if got := alloc.Alloc(); got != 7 {
		t.Errorf("Alloc() = %d, want 7", got)
	}

	// MaxID should now be 7
	if got := alloc.MaxID(); got != 7 {
		t.Errorf("MaxID() = %d, want 7", got)
	}
}

func TestShapeIDAllocator_AllocN(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte(`<p:cNvPr id="10" name=""/>`))

	// Allocate 3 IDs
	start := alloc.AllocN(3)
	if start != 11 {
		t.Errorf("AllocN(3) = %d, want 11", start)
	}

	// MaxID should now be 13
	if got := alloc.MaxID(); got != 13 {
		t.Errorf("MaxID() = %d, want 13", got)
	}

	// Next single allocation should be 14
	if got := alloc.Alloc(); got != 14 {
		t.Errorf("Alloc() = %d, want 14", got)
	}
}

func TestShapeIDAllocator_AllocN_EdgeCases(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte(`<p:cNvPr id="5" name=""/>`))

	// Allocate 0 - should still return next ID without allocating
	start := alloc.AllocN(0)
	if start != 6 {
		t.Errorf("AllocN(0) = %d, want 6", start)
	}
	// MaxID shouldn't have changed
	if got := alloc.MaxID(); got != 5 {
		t.Errorf("after AllocN(0), MaxID() = %d, want 5", got)
	}

	// Negative also returns next without allocating
	start = alloc.AllocN(-1)
	if start != 6 {
		t.Errorf("AllocN(-1) = %d, want 6", start)
	}
}

func TestShapeIDAllocator_NextID(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte(`<p:cNvPr id="10" name=""/>`))

	// NextID should preview without allocating
	if got := alloc.NextID(); got != 11 {
		t.Errorf("NextID() = %d, want 11", got)
	}

	// Calling NextID again should return the same value
	if got := alloc.NextID(); got != 11 {
		t.Errorf("NextID() second call = %d, want 11", got)
	}

	// After Alloc, NextID should advance
	alloc.Alloc()
	if got := alloc.NextID(); got != 12 {
		t.Errorf("NextID() after Alloc = %d, want 12", got)
	}
}

func TestShapeIDAllocator_SetMinID(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte(`<p:cNvPr id="5" name=""/>`))

	// Setting a lower minimum shouldn't change anything
	alloc.SetMinID(3)
	if got := alloc.NextID(); got != 6 {
		t.Errorf("after SetMinID(3), NextID() = %d, want 6", got)
	}

	// Setting a higher minimum should advance
	alloc.SetMinID(100)
	if got := alloc.NextID(); got != 100 {
		t.Errorf("after SetMinID(100), NextID() = %d, want 100", got)
	}

	if got := alloc.Alloc(); got != 100 {
		t.Errorf("after SetMinID(100), Alloc() = %d, want 100", got)
	}
}

func TestShapeIDAllocator_Clone(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte(`<p:cNvPr id="10" name=""/>`))
	alloc.Alloc() // 11

	clone := alloc.Clone()

	// Clone should have same state
	if got := clone.MaxID(); got != 11 {
		t.Errorf("clone.MaxID() = %d, want 11", got)
	}

	// Allocating from clone shouldn't affect original
	clone.Alloc() // 12
	if got := alloc.MaxID(); got != 11 {
		t.Errorf("original MaxID() after clone.Alloc() = %d, want 11", got)
	}
	if got := clone.MaxID(); got != 12 {
		t.Errorf("clone MaxID() after Alloc() = %d, want 12", got)
	}
}

func TestShapeIDAllocator_ScanXML(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte(`<p:cNvPr id="5" name=""/>`))

	// Scan additional XML
	alloc.ScanXML([]byte(`<p:cNvPr id="10" name=""/>`))

	// MaxID should be updated
	if got := alloc.MaxID(); got != 10 {
		t.Errorf("after ScanXML, MaxID() = %d, want 10", got)
	}

	// Scanning lower IDs shouldn't decrease max
	alloc.ScanXML([]byte(`<p:cNvPr id="2" name=""/>`))
	if got := alloc.MaxID(); got != 10 {
		t.Errorf("after scanning lower ID, MaxID() = %d, want 10", got)
	}
}

func TestShapeIDAllocator_EmptySlide(t *testing.T) {
	alloc := NewShapeIDAllocator([]byte{})

	// Empty input should start at 0, first alloc returns 1
	if got := alloc.MaxID(); got != 0 {
		t.Errorf("empty MaxID() = %d, want 0", got)
	}

	if got := alloc.Alloc(); got != 1 {
		t.Errorf("empty Alloc() = %d, want 1", got)
	}
}

// TestShapeIDAllocator_RealTemplates tests against actual PPTX templates.
func TestShapeIDAllocator_RealTemplates(t *testing.T) {
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

				for i := 0; i < enum.Count(); i++ {
					slidePath := enum.PartPath(i)
					alloc, err := NewShapeIDAllocatorForSlide(pkg, slidePath)
					if err != nil {
						t.Errorf("NewShapeIDAllocatorForSlide failed for %s: %v", slidePath, err)
						continue
					}

					// Verify we found at least the group shape (ID 1)
					if alloc.MaxID() < 1 {
						t.Errorf("slide %d: MaxID() = %d, expected at least 1", i, alloc.MaxID())
					}

					// Verify allocation returns a valid ID
					nextID := alloc.Alloc()
					if nextID <= alloc.MaxID()-1 {
						t.Errorf("slide %d: Alloc() = %d, but MaxID() was %d before", i, nextID, alloc.MaxID()-1)
					}

					t.Logf("slide %d (%s): max existing ID = %d", i, slidePath, alloc.MaxID()-1)
				}
			})
		}
	}
}
