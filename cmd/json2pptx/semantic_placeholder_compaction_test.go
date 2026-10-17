package main

import (
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

func markerDiagnostic(path, marker string) semanticDiagnostic {
	d := diagnostics.Diagnostic{Code: diagnostics.CodeSemanticWeakContent, Severity: diagnostics.SeverityError, Path: path,
		Message: `this is placeholder copy from "recommend_visual recipes", not content; replace it with real text`}
	if marker != "" {
		d.Details = map[string]any{semantic.PlaceholderDetail: marker}
	}
	return semanticDiagFromCompile(d)
}

func TestPlaceholderCompactionKeepsMarkerFamilies(t *testing.T) {
	in := []semanticDiagnostic{
		markerDiagnostic("slides[0].title", placeholder.NameRecipeTitle),
		markerDiagnostic("slides[0].source", placeholder.NameRecipeSampleSource),
		markerDiagnostic("slides[0].insights[0]", placeholder.NameRecipeCopy),
		markerDiagnostic("slides[0].insights[1]", ""),
	}
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}} {
		var ordered []semanticDiagnostic
		for _, i := range order {
			ordered = append(ordered, in[i])
		}
		got := collapseDiagnostics(ordered)
		if len(got) != len(ordered) {
			t.Fatalf("different or absent marker identities merged: %+v", got)
		}
		for i, d := range got {
			if d.SemanticPath != ordered[i].SemanticPath || !reflect.DeepEqual(d.Evidence, ordered[i].Evidence) || len(d.members) != 0 || !d.Blocking {
				t.Errorf("marker provenance/address/blocking changed: %+v", d)
			}
		}
	}
}

func TestPlaceholderCompactionKeepsSameFamilyAddresses(t *testing.T) {
	for _, marker := range []string{placeholder.NameRecipeCopy, ""} {
		t.Run(marker, func(t *testing.T) {
			first := markerDiagnostic("slides[0].insights[0]", marker)
			second := markerDiagnostic("slides[0].insights[1]", marker)
			// A compiler source may carry the marker before an adapter copies it
			// to Evidence; that representation must have the same identity.
			second.Evidence = nil
			got := collapseDiagnostics([]semanticDiagnostic{first, second, first})
			if len(got) != 1 || len(got[0].members) != 2 {
				t.Fatalf("same family did not compact/deduplicate: %+v", got)
			}
			doc := newSpecDoc("deck.json", []byte(`{"slides":[{"kind":"chart_insight","insights":["First","Second"]}]}`))
			shapeRenderDiagnostics(got, doc)
			want := []string{"/slides/0/insights/0", "/slides/0/insights/1"}
			if !reflect.DeepEqual(got[0].memberPaths, want) || !got[0].Blocking {
				t.Errorf("same-family member addresses or blocking lost: %+v", got[0])
			}
			again := collapseDiagnostics(got)
			if len(again) != 1 || len(again[0].members) != 2 || again[0].Message != got[0].Message {
				t.Errorf("same-family compaction is not idempotent: %+v", again)
			}
		})
	}
}

func TestPlaceholderRawDeduplicationKeepsMarkerFacts(t *testing.T) {
	base := *markerDiagnostic("/slides/0/title", placeholder.NameRecipeTitle).diag
	other := *markerDiagnostic("/slides/0/title", placeholder.NameRecipeCopy).diag
	absent := *markerDiagnostic("/slides/0/title", "").diag
	richer := base
	richer.Fix = &diagnostics.Fix{Kind: "rewrite_field"}
	got := dedupeDiagnostics([]diagnostics.Diagnostic{base, other, absent, richer})
	if len(got) != 3 || got[0].Fix == nil || placeholderMarkerIdentity(got[0].Details) != placeholder.NameRecipeTitle || placeholderMarkerIdentity(got[1].Details) != placeholder.NameRecipeCopy {
		t.Fatalf("raw deduplication lost marker facts or richer same-family edit: %+v", got)
	}
	// Unrelated facts are not new identity classes for unmarked findings.
	absent.Details = map[string]any{"measured": 12}
	plain := absent
	plain.Details = nil
	if got := dedupeDiagnostics([]diagnostics.Diagnostic{absent, plain}); len(got) != 1 {
		t.Fatalf("unrelated evidence changed unmarked deduplication: %+v", got)
	}
}
