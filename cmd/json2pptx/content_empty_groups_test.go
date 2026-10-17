package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func TestBulletGroupsEmptyContentRefusalParity(t *testing.T) {
	for _, typed := range []bool{false, true} {
		field := "value"
		if typed {
			field = "bullet_groups_value"
		}
		for _, tc := range []struct {
			name  string
			value BulletGroupsInput
			valid bool
		}{
			{"empty", BulletGroupsInput{Groups: []BulletGroupInput{}}, false},
			{"whitespace", BulletGroupsInput{Body: " \n", TrailingBody: "\t", Groups: []BulletGroupInput{}}, false},
			{"body_only", BulletGroupsInput{Body: "Revenue grew by 18%.", Groups: []BulletGroupInput{}}, true},
			{"trailing_only", BulletGroupsInput{TrailingBody: "Approve the routing pilot.", Groups: []BulletGroupInput{}}, true},
			{"group", BulletGroupsInput{Groups: []BulletGroupInput{{Bullets: []string{"Launch the pilot"}}}}, true},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				content := ContentInput{PlaceholderID: "body", Type: "bullet_groups"}
				if typed {
					content.BulletGroupsValue = &tc.value
				} else {
					raw, err := json.Marshal(tc.value)
					if err != nil {
						t.Fatal(err)
					}
					content.Value = raw
				}
				wantPath := "/slides/0/content/1/" + field + "/groups"
				items := []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Pilot decision")}, content}
				t.Run("converter", func(t *testing.T) {
					_, err := convertPresentationContent(items, 1, "")
					if tc.valid {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					if err == nil || cliErrorPath(err) != wantPath {
						t.Fatalf("empty content error=%v path=%q, want %s", err, cliErrorPath(err), wantPath)
					}
				})
				t.Run("validate_input", func(t *testing.T) {
					deck := PresentationInput{Template: "midnight-blue", Slides: []SlideInput{{LayoutID: "content", Content: items}}}
					var presentation map[string]any
					raw, err := json.Marshal(deck)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &presentation); err != nil {
						t.Fatal(err)
					}
					result := mustCall(t, testMCPConfig(t).handleValidate, map[string]any{"presentation": presentation, "fit_report": false})
					var findings diagnostics.FindingEnvelope
					valid := false
					if result.IsError {
						structuredInto(t, result.StructuredContent, &findings)
					} else {
						var output dryRunOutput
						structuredInto(t, result.StructuredContent, &output)
						valid = output.Valid
						findings = output.Findings
					}
					if valid != tc.valid {
						t.Fatalf("valid=%v want %v: %s", valid, tc.valid, resultText(result))
					}

					if tc.valid {
						return
					}
					found := false
					for _, f := range findings.Findings {
						if f.Severity == diagnostics.SeverityError && f.Evidence["path"] == wantPath {
							found = true
						}
					}
					if !found {
						t.Fatalf("missing authored empty-content path %s: %s", wantPath, resultText(result))
					}
				})
			})
		}
	}
}
