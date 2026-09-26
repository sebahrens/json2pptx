package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-s1uvj.26: a table row with fewer cells than headers
// validated as VALID but rendered fewer <a:tc> than <a:gridCol>, so generate
// failed with OOXML_INVALID_TABLE. Short rows are now padded with empty cells;
// rows wider than the header count fail validation and generation alike.
func TestRaggedTableRows_ValidateAndGenerateAgree(t *testing.T) {
	cases := []struct {
		name      string
		rows      string
		wantValid bool
	}{
		{"short row padded", `[["North America","$12.4M"],["Europe","$8.7M","+5%"]]`, true},
		{"short row after rowspan", `[[{"content":"NA","row_span":2},"$12.4M","+3%"],["$8.7M"]]`, true},
		{"wide row rejected", `[["North America","$12.4M","+3%","extra"],["Europe","$8.7M","+5%"]]`, false},
		{"col_span overflow rejected", `[[{"content":"North America","col_span":3},"$12.4M"]]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			deck := `{"template":"midnight-blue","output_filename":"ragged.pptx","slides":[{"slide_type":"content","content":[` +
				`{"placeholder_id":"title","type":"text","text_value":"Ragged table"},` +
				`{"placeholder_id":"body","type":"table","table_value":{"headers":["Region","Revenue","Growth"],"rows":` + tc.rows + `}}]}]}`
			inputPath := filepath.Join(dir, "input.json")
			if err := os.WriteFile(inputPath, []byte(deck), 0o600); err != nil {
				t.Fatal(err)
			}

			result := validateJSONFile(inputPath, testTemplatesDir, "", false, "warn")
			if result.Valid != tc.wantValid {
				t.Fatalf("validate Valid = %v, want %v; errors: %v", result.Valid, tc.wantValid, result.Errors)
			}
			if !tc.wantValid && !strings.Contains(strings.Join(result.Errors, "\n"), "headers define 3") {
				t.Errorf("validate error should name the header count, got: %v", result.Errors)
			}

			genErr := runJSONMode(inputPath, filepath.Join(dir, "result.json"), testTemplatesDir, dir,
				"", false, false, "midnight-blue", "off", false, "off", "", false)
			if !tc.wantValid {
				if genErr == nil {
					t.Fatal("generate succeeded for a row wider than the headers; validate rejected it")
				}
				return
			}
			if genErr != nil {
				t.Fatalf("generate: %v", genErr)
			}
			report, err := pptx.ValidateOutputFile(filepath.Join(dir, "ragged.pptx"))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range report.Findings {
				if f.Code == "OOXML_INVALID_TABLE" {
					t.Errorf("generated table is invalid: %s", f.Error())
				}
			}
		})
	}
}
