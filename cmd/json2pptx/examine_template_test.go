package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

const examineSchemaPath = "../../docs/api/finding-envelope.schema.json"

// TestExamineTemplate_BundledTemplatesProduceValidReport runs the examination
// over every bundled template, asserts the directory tree is produced, and
// validates the report's findings envelope against the committed schema.
func TestExamineTemplate_BundledTemplatesProduceValidReport(t *testing.T) {
	schema := loadEnvelopeSchema(t)

	templates, err := filepath.Glob("../../templates/*.pptx")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("no bundled templates found")
	}

	for _, tpl := range templates {
		tpl := tpl
		t.Run(filepath.Base(tpl), func(t *testing.T) {
			reader, err := template.OpenTemplate(tpl)
			if err != nil {
				t.Fatalf("open template: %v", err)
			}
			defer func() { _ = reader.Close() }()

			report, err := examine.Examine(reader, examine.Options{TemplatePath: tpl})
			if err != nil {
				t.Fatalf("examine: %v", err)
			}

			// Validate the findings envelope against the committed schema.
			raw, err := json.Marshal(report.Findings)
			if err != nil {
				t.Fatalf("marshal findings: %v", err)
			}
			var findings any
			if err := json.Unmarshal(raw, &findings); err != nil {
				t.Fatalf("unmarshal findings: %v", err)
			}
			if errs := validateAgainstSchema(schema, schema, findings, "findings"); len(errs) > 0 {
				t.Fatalf("report.findings does not satisfy the envelope schema:\n%s", joinErrs(errs))
			}

			// Materialise the directory tree and assert the key artifacts exist.
			outDir := t.TempDir()
			if err := writeExamination(reader, report, tpl, outDir, false); err != nil {
				t.Fatalf("writeExamination: %v", err)
			}
			for _, f := range []string{"report.json", "report.md", "theme.json", "conformance.json", "canonical_roles.json"} {
				if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
					t.Errorf("missing %s: %v", f, err)
				}
			}
			if len(report.Layouts) == 0 {
				t.Fatal("expected at least one layout")
			}
			for i := range report.Layouts {
				base := filepath.Join(outDir, "layouts", report.Layouts[i].AssetBase)
				for _, ext := range []string{".json", ".xml", ".svg"} {
					if _, err := os.Stat(base + ext); err != nil {
						t.Errorf("missing %s%s: %v", report.Layouts[i].AssetBase, ext, err)
					}
				}
			}
		})
	}
}

// TestExamineTemplate_SyntheticMissingSectionDivider strips the section-divider
// layout from a bundled template and asserts examine reports the gap.
func TestExamineTemplate_SyntheticMissingSectionDivider(t *testing.T) {
	const src = "../../templates/midnight-blue.pptx"

	// Discover the section-divider layout's file so the test stays robust to
	// layout reordering across templates.
	reader, err := template.OpenTemplate(src)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		_ = reader.Close()
		t.Fatalf("parse layouts: %v", err)
	}
	_ = reader.Close()

	var sectionID string
	for _, l := range layouts {
		if template.EffectiveCanonicalType(&l) == types.CanonicalLayoutSectionDivider {
			sectionID = l.ID
			break
		}
	}
	if sectionID == "" {
		t.Fatal("midnight-blue should have a section-divider layout to strip")
	}

	exclude := map[string]bool{
		"ppt/slideLayouts/" + sectionID + ".xml":            true,
		"ppt/slideLayouts/_rels/" + sectionID + ".xml.rels": true,
	}
	stripped := copyZipExcluding(t, src, exclude)

	sr, err := template.OpenTemplate(stripped)
	if err != nil {
		t.Fatalf("open stripped template: %v", err)
	}
	defer func() { _ = sr.Close() }()

	report, err := examine.Examine(sr, examine.Options{TemplatePath: stripped})
	if err != nil {
		t.Fatalf("examine stripped: %v", err)
	}

	if report.CanonicalCoverage["section-divider"].Present {
		t.Errorf("section-divider should be absent after stripping; coverage=%+v", report.CanonicalCoverage["section-divider"])
	}

	found := false
	for _, f := range report.Findings.Findings {
		if f.Code == "TPL.LAYOUT.MISSING_ROLE" {
			if fam, _ := f.Evidence["family"].(string); fam == "section-divider" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a TPL.LAYOUT.MISSING_ROLE finding for section-divider; findings=%+v", report.Findings.Findings)
	}
}

// TestExamineGate_BundledTemplatesPass is the positive half of the template CI
// gate: every bundled template must pass every gate check with zero violations,
// so the gate added by go-slide-creator-vt0n never red-flags a template that is
// already conformant. If this fails, a shipped template regressed (empty tags,
// missing canonical family, mis-named title, or a section divider without a
// "Section Number" frame) — fix the template, not the gate.
func TestExamineGate_BundledTemplatesPass(t *testing.T) {
	templates, err := filepath.Glob("../../templates/*.pptx")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("no bundled templates found")
	}

	for _, tpl := range templates {
		tpl := tpl
		t.Run(filepath.Base(tpl), func(t *testing.T) {
			reader, err := template.OpenTemplate(tpl)
			if err != nil {
				t.Fatalf("open template: %v", err)
			}
			defer func() { _ = reader.Close() }()

			report, err := examine.Examine(reader, examine.Options{TemplatePath: tpl})
			if err != nil {
				t.Fatalf("examine: %v", err)
			}
			if v := examine.Gate(report); len(v) != 0 {
				t.Fatalf("bundled template %s failed the CI gate with %d violation(s): %+v",
					filepath.Base(tpl), len(v), v)
			}
		})
	}
}

// TestExamineGate_SyntheticBrokenTemplateFails is the negative half: a template
// missing a content-bearing canonical family must fail the gate with a precise
// GATE.CANONICAL_COVERAGE_INCOMPLETE violation naming the missing family. This
// is the acceptance check that "adding a synthetic broken template fails CI
// with a precise finding", exercised against a real PPTX (not just a synthetic
// report).
func TestExamineGate_SyntheticBrokenTemplateFails(t *testing.T) {
	const src = "../../templates/midnight-blue.pptx"

	reader, err := template.OpenTemplate(src)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		_ = reader.Close()
		t.Fatalf("parse layouts: %v", err)
	}
	_ = reader.Close()

	var sectionID string
	for _, l := range layouts {
		if template.EffectiveCanonicalType(&l) == types.CanonicalLayoutSectionDivider {
			sectionID = l.ID
			break
		}
	}
	if sectionID == "" {
		t.Fatal("midnight-blue should have a section-divider layout to strip")
	}

	stripped := copyZipExcluding(t, src, map[string]bool{
		"ppt/slideLayouts/" + sectionID + ".xml":            true,
		"ppt/slideLayouts/_rels/" + sectionID + ".xml.rels": true,
	})

	sr, err := template.OpenTemplate(stripped)
	if err != nil {
		t.Fatalf("open stripped template: %v", err)
	}
	defer func() { _ = sr.Close() }()

	report, err := examine.Examine(sr, examine.Options{TemplatePath: stripped})
	if err != nil {
		t.Fatalf("examine stripped: %v", err)
	}

	violations := examine.Gate(report)
	if len(violations) == 0 {
		t.Fatal("expected the gate to fail for a template missing its section divider")
	}
	found := false
	for _, v := range violations {
		if v.Code == examine.GateCodeCanonicalCoverage && contains(v.Message, "section-divider") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a %s violation naming section-divider; got %+v",
			examine.GateCodeCanonicalCoverage, violations)
	}
}

func TestPrettyXML_PreservesContentAndIsWellFormed(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?><a:root xmlns:a="urn:x"><a:child id="1"><a:t>hello world</a:t></a:child><a:empty/></a:root>`)
	out := prettyXML(raw)
	if !regexp.MustCompile(`<a:t>hello world</a:t>`).Match(out) {
		t.Errorf("pretty XML should keep simple text content inline:\n%s", out)
	}
	// Re-parse the pretty output to confirm it is still well-formed.
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("pretty XML is not well-formed: %v\n%s", err, out)
		}
	}
}

// --- helpers ---

// copyZipExcluding writes a copy of the zip at src omitting the named entries,
// returning the new path under the test's temp dir.
func copyZipExcluding(t *testing.T, src string, exclude map[string]bool) string {
	t.Helper()
	r, err := zip.OpenReader(src)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	dst := filepath.Join(t.TempDir(), "stripped.pptx")
	f, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create dst: %v", err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	for _, file := range r.File {
		if exclude[file.Name] {
			continue
		}
		hdr := file.FileHeader
		fw, err := w.CreateHeader(&hdr)
		if err != nil {
			t.Fatalf("create header %s: %v", file.Name, err)
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", file.Name, err)
		}
		if _, err := io.Copy(fw, rc); err != nil { //nolint:gosec // test fixture
			_ = rc.Close()
			t.Fatalf("copy entry %s: %v", file.Name, err)
		}
		_ = rc.Close()
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return dst
}

func loadEnvelopeSchema(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(examineSchemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}

func joinErrs(errs []string) string {
	out := ""
	for _, e := range errs {
		out += "  - " + e + "\n"
	}
	return out
}

// validateAgainstSchema is a compact JSON-schema validator supporting the
// constructs used by finding-envelope.schema.json: type, required,
// additionalProperties:false, properties, $ref (#/$defs/x), const, enum,
// pattern, and array items. root carries the $defs for $ref resolution.
func validateAgainstSchema(root, node map[string]any, val any, path string) []string {
	if ref, ok := node["$ref"].(string); ok {
		resolved := resolveRef(root, ref)
		if resolved == nil {
			return []string{fmt.Sprintf("%s: unresolved $ref %q", path, ref)}
		}
		return validateAgainstSchema(root, resolved, val, path)
	}

	var errs []string

	if c, ok := node["const"]; ok && !jsonEqual(c, val) {
		errs = append(errs, fmt.Sprintf("%s: const mismatch (want %v, got %v)", path, c, val))
	}
	if enum, ok := node["enum"].([]any); ok {
		member := false
		for _, e := range enum {
			if jsonEqual(e, val) {
				member = true
				break
			}
		}
		if !member {
			errs = append(errs, fmt.Sprintf("%s: value %v not in enum", path, val))
		}
	}

	typ, _ := node["type"].(string)
	switch typ {
	case "object":
		m, ok := val.(map[string]any)
		if !ok {
			return append(errs, fmt.Sprintf("%s: expected object, got %T", path, val))
		}
		props, _ := node["properties"].(map[string]any)
		if req, ok := node["required"].([]any); ok {
			for _, r := range req {
				key, _ := r.(string)
				if _, present := m[key]; !present {
					errs = append(errs, fmt.Sprintf("%s: missing required %q", path, key))
				}
			}
		}
		if ap, ok := node["additionalProperties"].(bool); ok && !ap {
			for k := range m {
				if _, allowed := props[k]; !allowed {
					errs = append(errs, fmt.Sprintf("%s: additional property %q not allowed", path, k))
				}
			}
		}
		for k, sub := range props {
			v, present := m[k]
			if !present {
				continue
			}
			subSchema, _ := sub.(map[string]any)
			errs = append(errs, validateAgainstSchema(root, subSchema, v, path+"."+k)...)
		}
	case "array":
		arr, ok := val.([]any)
		if !ok {
			return append(errs, fmt.Sprintf("%s: expected array, got %T", path, val))
		}
		if items, ok := node["items"].(map[string]any); ok {
			for i, v := range arr {
				errs = append(errs, validateAgainstSchema(root, items, v, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	case "string":
		s, ok := val.(string)
		if !ok {
			return append(errs, fmt.Sprintf("%s: expected string, got %T", path, val))
		}
		if pat, ok := node["pattern"].(string); ok {
			if !regexp.MustCompile(pat).MatchString(s) {
				errs = append(errs, fmt.Sprintf("%s: %q does not match pattern %q", path, s, pat))
			}
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			errs = append(errs, fmt.Sprintf("%s: expected boolean, got %T", path, val))
		}
	case "integer":
		if f, ok := val.(float64); !ok || f != float64(int64(f)) {
			errs = append(errs, fmt.Sprintf("%s: expected integer, got %v", path, val))
		}
	case "number":
		if _, ok := val.(float64); !ok {
			errs = append(errs, fmt.Sprintf("%s: expected number, got %T", path, val))
		}
	}
	return errs
}

func resolveRef(root map[string]any, ref string) map[string]any {
	const prefix = "#/$defs/"
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return nil
	}
	defs, _ := root["$defs"].(map[string]any)
	def, _ := defs[ref[len(prefix):]].(map[string]any)
	return def
}

func jsonEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
