package pptx

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestSlideIDXML_MarshalXML(t *testing.T) {
	tests := []struct {
		name    string
		input   SlideIDXML
		wantID  string // expected id attribute value
		wantRID string // expected r:id attribute value
	}{
		{
			name:    "basic slide ID",
			input:   SlideIDXML{ID: 256, RID: "rId2"},
			wantID:  "256",
			wantRID: "rId2",
		},
		{
			name:    "higher slide ID",
			input:   SlideIDXML{ID: 300, RID: "rId15"},
			wantID:  "300",
			wantRID: "rId15",
		},
		{
			name:    "zero slide ID",
			input:   SlideIDXML{ID: 0, RID: "rId1"},
			wantID:  "0",
			wantRID: "rId1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := xml.NewEncoder(&buf)

			err := enc.Encode(tt.input)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if err := enc.Flush(); err != nil {
				t.Fatalf("Flush failed: %v", err)
			}

			result := buf.String()

			// Check that the output contains sldId element
			if !strings.Contains(result, "<sldId") {
				t.Errorf("output should contain <sldId, got: %s", result)
			}

			// Check id attribute (unqualified)
			expectedIDAttr := `id="` + tt.wantID + `"`
			if !strings.Contains(result, expectedIDAttr) {
				t.Errorf("output should contain %s, got: %s", expectedIDAttr, result)
			}

			// Check that r:id or equivalent namespace-qualified id is present
			// The exact format depends on how Go's xml encoder handles namespaces
			if !strings.Contains(result, tt.wantRID) {
				t.Errorf("output should contain RID value %s, got: %s", tt.wantRID, result)
			}
		})
	}
}

func TestSlideIDXML_RoundTrip(t *testing.T) {
	// Test that we can unmarshal and then marshal back
	original := `<sldId xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" id="256" r:id="rId2"></sldId>`

	var slideID SlideIDXML
	if err := xml.Unmarshal([]byte(original), &slideID); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify unmarshal worked
	if slideID.ID != 256 {
		t.Errorf("ID = %d, want 256", slideID.ID)
	}
	if slideID.RID != "rId2" {
		t.Errorf("RID = %s, want rId2", slideID.RID)
	}

	// Now marshal it back
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.Encode(slideID); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	result := buf.String()

	// Verify the marshaled output contains correct values
	if !strings.Contains(result, "256") {
		t.Errorf("marshaled output should contain ID 256, got: %s", result)
	}
	if !strings.Contains(result, "rId2") {
		t.Errorf("marshaled output should contain RID rId2, got: %s", result)
	}
}

func TestSlideIDListXML_MarshalXML(t *testing.T) {
	// Test marshaling a list of slide IDs within the SlideIDListXML container
	list := SlideIDListXML{
		SlideIDs: []SlideIDXML{
			{ID: 256, RID: "rId2"},
			{ID: 257, RID: "rId3"},
			{ID: 258, RID: "rId4"},
		},
	}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.Encode(list); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	result := buf.String()

	// Verify all three slides are present
	if strings.Count(result, "<sldId") != 3 {
		t.Errorf("expected 3 sldId elements, got: %s", result)
	}

	// Verify IDs are present
	for _, id := range []string{"256", "257", "258"} {
		if !strings.Contains(result, id) {
			t.Errorf("expected ID %s in output, got: %s", id, result)
		}
	}

	// Verify RIDs are present
	for _, rid := range []string{"rId2", "rId3", "rId4"} {
		if !strings.Contains(result, rid) {
			t.Errorf("expected RID %s in output, got: %s", rid, result)
		}
	}
}

func TestSlideIDXML_MarshalXML_EmptyRID(t *testing.T) {
	// Edge case: empty RID (shouldn't happen in practice but should handle gracefully)
	slideID := SlideIDXML{ID: 256, RID: ""}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	err := enc.Encode(slideID)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "256") {
		t.Errorf("output should contain ID 256, got: %s", result)
	}
}
