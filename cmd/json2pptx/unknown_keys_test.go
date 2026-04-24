package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestJsonFieldNames(t *testing.T) {
	type Example struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar,omitempty"`
		Baz bool   `json:"-"`
		Qux string // no json tag
	}
	names := jsonFieldNames(reflect.TypeOf(Example{}))
	want := []string{"bar", "foo"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("jsonFieldNames = %v, want %v", names, want)
	}
}

func TestCheckUnknownKeys_DetectsTypo(t *testing.T) {
	raw := json.RawMessage(`{"foo":"a","fooo":"b","bar":"c"}`)
	known := []string{"foo", "bar", "baz"}
	errs := checkUnknownKeys(raw, known, "root")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	ve := errs[0]
	if ve.Code != patterns.ErrCodeUnknownKey {
		t.Errorf("code = %q, want %q", ve.Code, patterns.ErrCodeUnknownKey)
	}
	if ve.Path != "root.fooo" {
		t.Errorf("path = %q, want %q", ve.Path, "root.fooo")
	}
	if !errors.Is(ve, patterns.ErrUnknownKey) {
		t.Error("expected errors.Is(ve, patterns.ErrUnknownKey)")
	}
	if ve.Fix == nil || ve.Fix.Kind != "rename_field" {
		t.Errorf("expected rename_field fix, got %+v", ve.Fix)
	}
	if ve.Fix.Params["to"] != "foo" {
		t.Errorf("expected fix.to = %q, got %q", "foo", ve.Fix.Params["to"])
	}
}

func TestCheckUnknownKeys_AllKnown(t *testing.T) {
	raw := json.RawMessage(`{"foo":"a","bar":"b"}`)
	known := []string{"foo", "bar", "baz"}
	errs := checkUnknownKeys(raw, known, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUnknownKeys_NoSuggestion(t *testing.T) {
	raw := json.RawMessage(`{"zzzzzzzzzzz":"x"}`)
	known := []string{"foo", "bar"}
	errs := checkUnknownKeys(raw, known, "")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Fix != nil {
		t.Errorf("expected nil Fix for distant unknown key, got %+v", errs[0].Fix)
	}
}

func TestCheckInputUnknownKeys_TopLevel(t *testing.T) {
	raw := json.RawMessage(`{
		"template": "midnight-blue",
		"slide_tpye": "wrong",
		"slides": []
	}`)
	errs := checkInputUnknownKeys(raw)
	found := false
	for _, ve := range errs {
		if ve.Path == "slide_tpye" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown_key error for 'slide_tpye', got %v", errs)
	}
}

func TestCheckInputUnknownKeys_SlideLevel(t *testing.T) {
	raw := json.RawMessage(`{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout1",
			"slide_tpye": "content",
			"content": []
		}]
	}`)
	errs := checkInputUnknownKeys(raw)
	found := false
	for _, ve := range errs {
		if ve.Path == "slides[0].slide_tpye" {
			found = true
			if ve.Fix == nil || ve.Fix.Params["to"] != "slide_type" {
				t.Errorf("expected did-you-mean 'slide_type', got fix %+v", ve.Fix)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected unknown_key for 'slides[0].slide_tpye', got %v", errs)
	}
}

func TestCheckInputUnknownKeys_ContentLevel(t *testing.T) {
	raw := json.RawMessage(`{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout1",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"placeholderId": "wrong_case"
			}]
		}]
	}`)
	errs := checkInputUnknownKeys(raw)
	found := false
	for _, ve := range errs {
		if ve.Path == "slides[0].content[0].placeholderId" {
			found = true
			if ve.Fix == nil || ve.Fix.Params["to"] != "placeholder_id" {
				t.Errorf("expected did-you-mean 'placeholder_id', got fix %+v", ve.Fix)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected unknown_key for 'slides[0].content[0].placeholderId', got %v", errs)
	}
}

func TestCheckInputUnknownKeys_BackgroundLevel(t *testing.T) {
	raw := json.RawMessage(`{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout1",
			"background": {"img": "photo.jpg"},
			"content": []
		}]
	}`)
	errs := checkInputUnknownKeys(raw)
	found := false
	for _, ve := range errs {
		if ve.Path == "slides[0].background.img" {
			found = true
			if ve.Fix != nil && ve.Fix.Params["to"] != "image" {
				t.Errorf("expected did-you-mean 'image', got fix %+v", ve.Fix)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected unknown_key for 'slides[0].background.img', got %v", errs)
	}
}

func TestCheckInputUnknownKeys_CleanInput(t *testing.T) {
	raw := json.RawMessage(`{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout1",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Hello"
			}]
		}]
	}`)
	errs := checkInputUnknownKeys(raw)
	if len(errs) != 0 {
		t.Errorf("expected no errors for clean input, got %d: %v", len(errs), errs)
	}
}
