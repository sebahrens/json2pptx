package diagnostics

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestFromValidationError(t *testing.T) {
	ve := &patterns.ValidationError{
		Pattern: "card-grid",
		Path:    "cells[2].header",
		Code:    "required",
		Message: "card-grid: cells[2].header is required",
		Fix:     &patterns.FixSuggestion{Kind: "provide_value", Params: map[string]any{"field": "header"}},
	}

	d := FromValidationError(ve)

	if d.Code != "required" {
		t.Errorf("Code = %q, want %q", d.Code, "required")
	}
	if d.Message != ve.Message {
		t.Errorf("Message = %q, want %q", d.Message, ve.Message)
	}
	if d.Path != "cells[2].header" {
		t.Errorf("Path = %q, want %q", d.Path, "cells[2].header")
	}
	if d.Severity != SeverityError {
		t.Errorf("Severity = %q, want %q", d.Severity, SeverityError)
	}
	if d.Fix == nil || d.Fix.Kind != "provide_value" {
		t.Errorf("Fix.Kind = %v, want provide_value", d.Fix)
	}
	if d.Details["pattern"] != "card-grid" {
		t.Errorf("Details[pattern] = %v, want card-grid", d.Details["pattern"])
	}
}

func TestFromValidationError_NoFix(t *testing.T) {
	ve := &patterns.ValidationError{
		Path:    "template",
		Code:    "required",
		Message: "template is required",
	}
	d := FromValidationError(ve)

	if d.Fix != nil {
		t.Errorf("Fix = %v, want nil", d.Fix)
	}
	if d.Details != nil {
		t.Errorf("Details = %v, want nil (no pattern)", d.Details)
	}
}

func TestFromFitFinding(t *testing.T) {
	tests := []struct {
		action   string
		wantSev  Severity
	}{
		{"refuse", SeverityError},
		{"shrink_or_split", SeverityWarning},
		{"review", SeverityInfo},
		{"info", SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			f := patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path:    "slides[0].content.body",
					Code:    "fit_overflow",
					Message: "text overflows",
				},
				Action:        tt.action,
				OverflowRatio: 1.25,
				Measured:      &patterns.Extent{WidthEMU: 100, HeightEMU: 200},
				Allowed:       &patterns.Extent{WidthEMU: 80, HeightEMU: 160},
			}
			d := FromFitFinding(f)

			if d.Severity != tt.wantSev {
				t.Errorf("Severity = %q, want %q", d.Severity, tt.wantSev)
			}
			if d.Details["action"] != tt.action {
				t.Errorf("Details[action] = %v, want %q", d.Details["action"], tt.action)
			}
			if d.Details["overflow_ratio"] != 1.25 {
				t.Errorf("Details[overflow_ratio] = %v, want 1.25", d.Details["overflow_ratio"])
			}
			if d.Details["measured"] == nil {
				t.Error("Details[measured] is nil, want non-nil")
			}
			if d.Details["allowed"] == nil {
				t.Error("Details[allowed] is nil, want non-nil")
			}
		})
	}
}

func TestFromJoinedError_Mixed(t *testing.T) {
	ve := &patterns.ValidationError{
		Pattern: "kpi-grid",
		Path:    "cells[0].value",
		Code:    "required",
		Message: "kpi-grid: cells[0].value is required",
	}
	plain := errors.New("something went wrong")
	joined := errors.Join(ve, plain)

	ds := FromJoinedError(joined, "GENERIC")

	if len(ds) != 2 {
		t.Fatalf("len = %d, want 2", len(ds))
	}

	// First: the ValidationError, gets structured conversion.
	if ds[0].Code != "required" {
		t.Errorf("ds[0].Code = %q, want required", ds[0].Code)
	}
	if ds[0].Details["pattern"] != "kpi-grid" {
		t.Errorf("ds[0].Details[pattern] = %v, want kpi-grid", ds[0].Details["pattern"])
	}

	// Second: plain error with fallback code.
	if ds[1].Code != "GENERIC" {
		t.Errorf("ds[1].Code = %q, want GENERIC", ds[1].Code)
	}
	if ds[1].Message != "something went wrong" {
		t.Errorf("ds[1].Message = %q", ds[1].Message)
	}
}

func TestFromJoinedError_Nil(t *testing.T) {
	ds := FromJoinedError(nil, "X")
	if ds != nil {
		t.Errorf("got %v, want nil", ds)
	}
}

func TestFromJoinedError_SingleError(t *testing.T) {
	err := errors.New("single error")
	ds := FromJoinedError(err, "SINGLE")

	if len(ds) != 1 {
		t.Fatalf("len = %d, want 1", len(ds))
	}
	if ds[0].Code != "SINGLE" {
		t.Errorf("Code = %q, want SINGLE", ds[0].Code)
	}
}

func TestJSONSerialization(t *testing.T) {
	d := Diagnostic{
		Code:     "required",
		Message:  "field is required",
		Path:     "slides[0].title",
		Severity: SeverityError,
		Fix: &Fix{
			Kind:   "provide_value",
			Params: map[string]any{"field": "title"},
		},
		Details: map[string]any{"pattern": "card-grid"},
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Diagnostic
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Code != d.Code {
		t.Errorf("Code = %q, want %q", decoded.Code, d.Code)
	}
	if decoded.Message != d.Message {
		t.Errorf("Message = %q, want %q", decoded.Message, d.Message)
	}
	if decoded.Path != d.Path {
		t.Errorf("Path = %q, want %q", decoded.Path, d.Path)
	}
	if decoded.Severity != d.Severity {
		t.Errorf("Severity = %q, want %q", decoded.Severity, d.Severity)
	}
	if decoded.Fix == nil || decoded.Fix.Kind != "provide_value" {
		t.Errorf("Fix round-trip failed: %v", decoded.Fix)
	}
}

func TestJSONSerialization_OmitsEmpty(t *testing.T) {
	d := Diagnostic{
		Code:     "warn",
		Message:  "something",
		Severity: SeverityWarning,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	s := string(data)
	if contains(s, "path") {
		t.Errorf("JSON contains 'path' but should omit empty: %s", s)
	}
	if contains(s, "fix") {
		t.Errorf("JSON contains 'fix' but should omit nil: %s", s)
	}
	if contains(s, "details") {
		t.Errorf("JSON contains 'details' but should omit nil: %s", s)
	}
}

func TestHasErrors(t *testing.T) {
	none := []Diagnostic{
		{Severity: SeverityWarning},
		{Severity: SeverityInfo},
	}
	if HasErrors(none) {
		t.Error("HasErrors(warnings+info) = true, want false")
	}

	some := []Diagnostic{
		{Severity: SeverityWarning},
		{Severity: SeverityError},
	}
	if !HasErrors(some) {
		t.Error("HasErrors(with error) = false, want true")
	}
}

func TestFilterBySeverity(t *testing.T) {
	ds := []Diagnostic{
		{Code: "a", Severity: SeverityError},
		{Code: "b", Severity: SeverityWarning},
		{Code: "c", Severity: SeverityError},
		{Code: "d", Severity: SeverityInfo},
	}

	errs := FilterBySeverity(ds, SeverityError)
	if len(errs) != 2 {
		t.Errorf("FilterBySeverity(error) = %d, want 2", len(errs))
	}

	infos := FilterBySeverity(ds, SeverityInfo)
	if len(infos) != 1 {
		t.Errorf("FilterBySeverity(info) = %d, want 1", len(infos))
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name string
		ds   []Diagnostic
		want string
	}{
		{"empty", nil, "no issues"},
		{"one error", []Diagnostic{{Severity: SeverityError}}, "1 error"},
		{"plural", []Diagnostic{
			{Severity: SeverityError},
			{Severity: SeverityError},
			{Severity: SeverityWarning},
		}, "2 errors, 1 warning"},
		{"all types", []Diagnostic{
			{Severity: SeverityError},
			{Severity: SeverityWarning},
			{Severity: SeverityWarning},
			{Severity: SeverityInfo},
		}, "1 error, 2 warnings, 1 info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Summary(tt.ds)
			if got != tt.want {
				t.Errorf("Summary = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFromValidationErrors_Nil(t *testing.T) {
	ds := FromValidationErrors(nil)
	if ds != nil {
		t.Errorf("got %v, want nil", ds)
	}
}

func TestFromFitFindings_Nil(t *testing.T) {
	ds := FromFitFindings(nil)
	if ds != nil {
		t.Errorf("got %v, want nil", ds)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
