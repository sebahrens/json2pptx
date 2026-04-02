package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSlideTypesHandler(t *testing.T) {
	handler := SlideTypesHandler()

	req := httptest.NewRequest("GET", "/api/v1/slide-types", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v", contentType, "application/json")
	}

	// Decode response
	var response SlideTypesResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check that we have all expected slide types
	expectedTypes := types.SupportedSlideTypes()
	if len(response.SlideTypes) != len(expectedTypes) {
		t.Errorf("handler returned %d slide types, want %d", len(response.SlideTypes), len(expectedTypes))
	}

	// Verify specific types are present
	typeMap := make(map[types.SlideType]bool)
	for _, st := range response.SlideTypes {
		typeMap[st.Type] = true
	}

	requiredTypes := []types.SlideType{
		types.SlideTypeTitle,
		types.SlideTypeContent,
		types.SlideTypeTwoColumn,
		types.SlideTypeImage,
		types.SlideTypeChart,
		types.SlideTypeComparison,
		types.SlideTypeBlank,
		types.SlideTypeSection,
	}

	for _, rt := range requiredTypes {
		if !typeMap[rt] {
			t.Errorf("handler response missing required slide type: %v", rt)
		}
	}

	// Check that descriptions are not empty
	for _, st := range response.SlideTypes {
		if st.Description == "" {
			t.Errorf("slide type %v has empty description", st.Type)
		}
	}
}

func TestSlideTypesHandler_ResponseFormat(t *testing.T) {
	handler := SlideTypesHandler()

	req := httptest.NewRequest("GET", "/api/v1/slide-types", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify exact JSON format matches spec
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check that "slide_types" key exists
	slideTypes, ok := response["slide_types"]
	if !ok {
		t.Error("response missing 'slide_types' key")
		return
	}

	// Check it's an array
	stArray, ok := slideTypes.([]interface{})
	if !ok {
		t.Error("'slide_types' is not an array")
		return
	}

	// Check first element has expected fields
	if len(stArray) > 0 {
		first, ok := stArray[0].(map[string]interface{})
		if !ok {
			t.Error("slide type element is not an object")
			return
		}

		if _, ok := first["type"]; !ok {
			t.Error("slide type object missing 'type' field")
		}
		if _, ok := first["description"]; !ok {
			t.Error("slide type object missing 'description' field")
		}
	}
}
