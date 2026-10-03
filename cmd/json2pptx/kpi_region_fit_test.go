package main

import (
	"archive/zip"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// kpiRegionSpec is a regions slide whose kpis row takes pct of the height
// above a chart, on the shortest shipped content area. With a heading the
// kpi-Nup sits one sub-grid deeper, under the heading row.
func kpiRegionSpec(pct int, heading string) map[string]any {
	kpis := map[string]any{"kind": "kpis", "size_pct": pct, "kpis": []any{
		map[string]any{"value": "$4.2M", "label": "ARR"},
		map[string]any{"value": "127%", "label": "Net revenue retention"},
		map[string]any{"value": "12d", "label": "Sales cycle"},
		map[string]any{"value": "98%", "label": "CSAT"},
	}}
	if heading != "" {
		kpis["heading"] = heading
	}
	return map[string]any{
		"meta": map[string]any{"template": "modern-template", "title": "KPI region", "source": "Finance ledger, FY26"},
		"slides": []any{map[string]any{
			"kind": "regions", "title": "Four KPIs over the revenue trend", "arrangement": "rows",
			"takeaway": "Growth held through the year.",
			"regions": []any{
				kpis,
				map[string]any{"kind": "chart", "size_pct": 100 - pct, "chart": map[string]any{
					"type": "bar_chart",
					"data": map[string]any{"categories": []any{"Q1", "Q2", "Q3", "Q4"}, "series": []any{map[string]any{"name": "Revenue", "values": []any{12, 14, 17, 21}}}},
				}},
			},
		}},
	}
}

// go-slide-creator-uj9zq: a kpis region too short for a value over its
// caption at the minimum sizes is a blocking finding at the region's path, at
// validate and at render. It used to render with the number drawn over its
// label and only an info-level fit_overflow.
func TestKPIRegionTooShortIsRefusedAtValidateAndRender(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, heading string
	}{{"bare", ""}, {"under a heading", "Run-rate"}} {
		t.Run(tc.name, func(t *testing.T) {
			mc := refusalTestConfig(t)
			spec := kpiRegionSpec(15, tc.heading)

			res, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
			if err != nil {
				t.Fatal(err)
			}
			var env deckSpecEnvelopeResponse
			structuredInto(t, res.StructuredContent, &env)
			if env.OK {
				t.Fatalf("validate approved a kpis region render refuses: %+v", env.Findings)
			}
			var found *diagnostics.Finding
			for i := range env.Findings {
				if env.Findings[i].Code == "FIT.fit_overflow" {
					found = &env.Findings[i]
				}
			}
			if found == nil || found.Severity != diagnostics.SeverityError {
				t.Fatalf("want an error-severity FIT.fit_overflow, got %+v", env.Findings)
			}
			if got := found.Evidence["path"]; got != "slides[0].regions[0]" {
				t.Errorf("evidence.path = %v, want the region slides[0].regions[0]", got)
			}
			if !strings.Contains(found.Message, "4 KPIs need") || !strings.Contains(found.Message, "size_pct") {
				t.Errorf("finding does not say what is short or what to change: %s", found.Message)
			}

			rres, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
			if err != nil {
				t.Fatal(err)
			}
			var render renderDeckSpecResponse
			structuredInto(t, rres.StructuredContent, &render)
			if render.OK || !rres.IsError {
				t.Fatalf("render should refuse the short kpis region: %+v", render)
			}
			var d *semanticDiagnostic
			for i := range render.Diagnostics {
				if render.Diagnostics[i].Code == "fit_overflow" {
					d = &render.Diagnostics[i]
				}
			}
			if d == nil {
				t.Fatalf("refusal left no diagnostic, only error %q", render.Error)
			}
			if d.Severity != "error" || d.Action != "refuse" {
				t.Errorf("severity/action = %s/%s, want error/refuse", d.Severity, d.Action)
			}
			if d.SemanticPath != "slides[0].regions[0]" {
				t.Errorf("semantic_path = %q, want slides[0].regions[0]", d.SemanticPath)
			}
			if !strings.HasPrefix(d.RawPath, "/slides/0/shape_grid/rows/0/cells/0/") || !strings.HasSuffix(d.RawPath, "/pattern/values") {
				t.Errorf("raw_path = %q, want the nested pattern's cell", d.RawPath)
			}
		})
	}
}

// A kpis region that fits — here by stepping down its type ladder at the 30%
// floor of the shortest shipped content area — validates, renders, and is
// written with no autofit shrink, so the value never touches its caption.
func TestKPIRegionThatFitsRendersUnshrunk(t *testing.T) {
	ctx := context.Background()
	for _, pct := range []int{30, 40, 50} {
		mc := refusalTestConfig(t)
		spec := kpiRegionSpec(pct, "")
		res, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
		if err != nil {
			t.Fatal(err)
		}
		var env deckSpecEnvelopeResponse
		structuredInto(t, res.StructuredContent, &env)
		for _, f := range env.Findings {
			if f.Severity == diagnostics.SeverityError {
				t.Errorf("%d%%: validate reports %s: %s", pct, f.Code, f.Message)
			}
		}
		rres, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
		if err != nil {
			t.Fatal(err)
		}
		var render renderDeckSpecResponse
		structuredInto(t, rres.StructuredContent, &render)
		if !render.OK {
			t.Fatalf("%d%%: render refused: %s", pct, render.Error)
		}
		for _, d := range render.Diagnostics {
			if d.Code == "fit_overflow" {
				t.Errorf("%d%%: render reports fit_overflow: %s", pct, d.Message)
			}
		}
		xml := readZipEntry(t, render.PptxPath, "ppt/slides/slide1.xml")
		if !strings.Contains(xml, "$4.2M") {
			t.Fatalf("%d%%: slide does not carry the KPI row", pct)
		}
		if strings.Contains(xml, "fontScale=") || strings.Contains(xml, "lnSpcReduction=") {
			t.Errorf("%d%%: a shape is written with an autofit shrink", pct)
		}
	}
}

func readZipEntry(t *testing.T, path, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	t.Fatalf("%s has no %s", path, name)
	return ""
}
