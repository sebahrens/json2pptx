package svggen

import (
	"strings"
	"testing"
)

func flatOrgRequest(nodes ...map[string]any) *RequestEnvelope {
	raw := make([]any, len(nodes))
	for i, n := range nodes {
		raw[i] = n
	}
	return &RequestEnvelope{Type: "org_chart", Data: map[string]any{"nodes": raw}}
}

func orgNodesFinding(t *testing.T, req *RequestEnvelope) *Finding {
	t.Helper()
	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	for i := range findings {
		if findings[i].Code == FindingOrgChartNodesInvalid {
			return &findings[i]
		}
	}
	return nil
}

// go-slide-creator-oqmzr: orphan parent ids, duplicate ids and label-only
// nodes rendered with no finding; a self-parent recursed forever.
func TestOrgChart_InvalidNodesAreReported(t *testing.T) {
	cases := []struct {
		name  string
		req   *RequestEnvelope
		wants string
	}{
		{
			name: "orphan_parent",
			req: flatOrgRequest(
				map[string]any{"id": "ceo", "name": "Ada"},
				map[string]any{"id": "x", "name": "Bo", "parent": "missing"},
			),
			wants: `nodes[1].parent "missing" matches no node id`,
		},
		{
			name: "duplicate_id",
			req: flatOrgRequest(
				map[string]any{"id": "ceo", "name": "Ada"},
				map[string]any{"id": "ceo", "name": "Bo"},
			),
			wants: `nodes[1].id "ceo" duplicates nodes[0]`,
		},
		{
			name: "self_parent",
			req: flatOrgRequest(
				map[string]any{"id": "ceo", "name": "Ada"},
				map[string]any{"id": "me", "name": "Bo", "parent": "me"},
			),
			wants: "is its own parent",
		},
		{
			name: "cycle",
			req: flatOrgRequest(
				map[string]any{"id": "ceo", "name": "Ada"},
				map[string]any{"id": "a", "name": "Bo", "parent": "b"},
				map[string]any{"id": "b", "name": "Cy", "parent": "a"},
			),
			wants: "parent cycle",
		},
		{
			// A node written with "label" is now refused before render
			// (UNKNOWN_FIELD with did-you-mean "name", go-slide-creator-x9s5i);
			// a node with no text key at all still draws empty and is reported.
			name: "textless_nodes",
			req: flatOrgRequest(
				map[string]any{"id": "ceo", "name": "Ada"},
				map[string]any{"id": "vp", "parent": "ceo"},
				map[string]any{"id": "cfo", "parent": "ceo"},
			),
			wants: "2 node(s) have neither name nor title",
		},
		{
			name: "nested_empty_node",
			req: &RequestEnvelope{Type: "org_chart", Data: map[string]any{
				"root": map[string]any{"name": "Ada", "children": []any{map[string]any{"name": ""}}},
			}},
			wants: "1 node(s) have neither name nor title",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := orgNodesFinding(t, tc.req)
			if f == nil {
				t.Fatalf("no %s finding", FindingOrgChartNodesInvalid)
			}
			if !strings.Contains(f.Message, tc.wants) {
				t.Errorf("message %q does not contain %q", f.Message, tc.wants)
			}
			if f.Fix == nil || f.Fix.Kind != FixKindReplaceValue {
				t.Errorf("fix = %+v, want kind %s", f.Fix, FixKindReplaceValue)
			}
		})
	}
}

func TestOrgChart_ValidNodesEmitNoNodesFinding(t *testing.T) {
	req := flatOrgRequest(
		map[string]any{"id": "ceo", "name": "Ada", "title": "CEO"},
		map[string]any{"id": "vp", "name": "Bo", "title": "VP", "parent": "ceo"},
	)
	if f := orgNodesFinding(t, req); f != nil {
		t.Fatalf("unexpected finding: %+v", f)
	}
}

// Orphans used to be attached in map-iteration order, so two renders of the
// same input could differ.
func TestOrgChart_OrphanOrderIsDeterministic(t *testing.T) {
	build := func() *RequestEnvelope {
		return flatOrgRequest(
			map[string]any{"id": "ceo", "name": "Ada"},
			map[string]any{"id": "a", "name": "Orphan A", "parent": "x1"},
			map[string]any{"id": "b", "name": "Orphan B", "parent": "x2"},
			map[string]any{"id": "c", "name": "Orphan C", "parent": "x3"},
			map[string]any{"id": "d", "name": "Orphan D", "parent": "x4"},
		)
	}
	first, err := Render(build())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		doc, err := Render(build())
		if err != nil {
			t.Fatal(err)
		}
		if string(doc.Content) != string(first.Content) {
			t.Fatal("org chart with orphans renders non-deterministically")
		}
	}
}
