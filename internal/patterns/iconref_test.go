package patterns

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestIconRef_UnmarshalJSON_StringShorthand(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  IconRef
	}{
		{"bundled_name", `"rocket"`, IconRef{Name: "rocket"}},
		{"qualified_name", `"filled:trending-up"`, IconRef{Name: "filled:trending-up"}},
		{"url_https", `"https://example.com/icon.svg"`, IconRef{URL: "https://example.com/icon.svg"}},
		{"url_http", `"http://example.com/icon.svg"`, IconRef{URL: "http://example.com/icon.svg"}},
		{"data_uri", `"data:image/svg+xml;base64,PHN2Zy8+"`, IconRef{URL: "data:image/svg+xml;base64,PHN2Zy8+"}},
		{"inline_svg", `"<svg xmlns=\"http://www.w3.org/2000/svg\"/>"`, IconRef{SVGData: `<svg xmlns="http://www.w3.org/2000/svg"/>`}},
		{"file_path", `"assets/logo.svg"`, IconRef{Path: "assets/logo.svg"}},
		{"unknown_falls_to_name", `"not-a-bundled-name"`, IconRef{Name: "not-a-bundled-name"}},
		{"empty", `""`, IconRef{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got IconRef
			if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestIconRef_UnmarshalJSON_ObjectForm(t *testing.T) {
	input := `{"path":"logo.svg","fill":"#FF0000","alt":"brand logo","position":"left"}`
	var got IconRef
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := IconRef{Path: "logo.svg", Fill: "#FF0000", Alt: "brand logo", Position: "left"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestIconRef_MarshalJSON_StringShortcut(t *testing.T) {
	// json.Marshal HTML-escapes <, >, & so svg_data round-trips with unicode
	// escape sequences. The test expectations match Go's default encoder output.
	cases := []struct {
		name string
		in   IconRef
		want string
	}{
		{"bundled_name_only", IconRef{Name: "rocket"}, `"rocket"`},
		{"name_with_fill_emits_object", IconRef{Name: "rocket", Fill: "#FF0000"}, `{"name":"rocket","fill":"#FF0000"}`},
		{"path_emits_object", IconRef{Path: "logo.svg"}, `{"path":"logo.svg"}`},
		{"url_emits_object", IconRef{URL: "https://x.io/i.svg"}, `{"url":"https://x.io/i.svg"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}

	t.Run("svg_data_emits_object", func(t *testing.T) {
		got, err := json.Marshal(IconRef{SVGData: "<svg/>"})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		// json.Marshal HTML-escapes < and >; check the round-trip yields the
		// original SVGData rather than the literal escaped form.
		var rt IconRef
		if err := json.Unmarshal(got, &rt); err != nil {
			t.Fatalf("re-unmarshal: %v\nbytes=%s", err, got)
		}
		if rt.SVGData != "<svg/>" {
			t.Errorf("round-trip lost svg_data: %+v", rt)
		}
	})
}

func TestIconRef_Resolve_AppliesDefaults(t *testing.T) {
	ref := IconRef{Name: "rocket"}
	out := ref.Resolve("accent2", "left")
	if out == nil {
		t.Fatal("Resolve returned nil for non-empty ref")
	}
	if out.Name != "rocket" || out.Fill != "accent2" || out.Position != "left" {
		t.Errorf("Resolve defaults not applied: %+v", out)
	}
}

func TestIconRef_Resolve_PreservesCallerOverrides(t *testing.T) {
	ref := IconRef{Path: "logo.svg", Fill: "#FF0000", Position: "top"}
	out := ref.Resolve("accent1", "left")
	if out.Fill != "#FF0000" {
		t.Errorf("caller fill %q overridden by default %q", out.Fill, "#FF0000")
	}
	if out.Position != "top" {
		t.Errorf("caller position %q overridden by default %q", out.Position, "top")
	}
	if out.Path != "logo.svg" {
		t.Errorf("path lost: %+v", out)
	}
}

func TestIconRef_Resolve_EmptyReturnsNil(t *testing.T) {
	var ref IconRef
	if out := ref.Resolve("accent1", "top"); out != nil {
		t.Errorf("expected nil for empty ref, got %+v", out)
	}
}

func TestValidateIconRef(t *testing.T) {
	cases := []struct {
		name      string
		ref       IconRef
		wantErr   string
		wantClean bool
	}{
		{"empty_ok", IconRef{}, "", true},
		{"bundled_known", IconRef{Name: "rocket"}, "", true},
		{"bundled_unknown", IconRef{Name: "not-a-real-icon"}, "must be a bundled icon name", false},
		{"path_svg_ok", IconRef{Path: "logo.svg"}, "", true},
		{"path_png_rejected", IconRef{Path: "logo.png"}, "must be a .svg file", false},
		{"url_accepted_without_extension_check", IconRef{URL: "https://x.io/i"}, "", true},
		{"svg_data_accepted", IconRef{SVGData: "<svg/>"}, "", true},
		{"two_sources_rejected", IconRef{Name: "rocket", Path: "logo.svg"}, "exactly one of name/path/url/svg_data", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateIconRef("test-pattern", "x.icon", tc.ref)
			if tc.wantClean {
				if len(errs) > 0 {
					t.Errorf("expected no errors, got %v", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			}
			joined := errs[0].Error()
			if !strings.Contains(joined, tc.wantErr) {
				t.Errorf("error %q does not contain %q", joined, tc.wantErr)
			}
		})
	}
}

func TestCardGridCell_AcceptsRichIcon(t *testing.T) {
	jsonInput := `{"header":"Brand","body":"Custom logo","icon":{"path":"logo.svg","fill":"#FF0000","alt":"company logo"}}`
	var cell CardGridCell
	if err := json.Unmarshal([]byte(jsonInput), &cell); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cell.Icon == nil {
		t.Fatal("expected Icon to be populated")
	}
	want := IconRef{Path: "logo.svg", Fill: "#FF0000", Alt: "company logo"}
	if !reflect.DeepEqual(*cell.Icon, want) {
		t.Errorf("got %+v, want %+v", *cell.Icon, want)
	}
}

func TestCardGridCell_AcceptsLegacyStringIcon(t *testing.T) {
	jsonInput := `{"header":"Launch","body":"text","icon":"rocket"}`
	var cell CardGridCell
	if err := json.Unmarshal([]byte(jsonInput), &cell); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cell.Icon == nil || cell.Icon.Name != "rocket" {
		t.Errorf("expected Icon.Name=rocket, got %+v", cell.Icon)
	}
}

func TestCardGrid_ExpandPassesThroughRichIcon(t *testing.T) {
	p := &cardGrid{}
	vals := &CardGridValues{
		Columns: 2,
		Rows:    1,
		Cells: []CardGridCell{
			{Header: "Brand", Body: "Custom", Icon: &IconRef{Path: "/abs/logo.svg", Fill: "#FF0000", Alt: "brand"}},
			{Header: "Other", Body: "Bundled", Icon: &IconRef{Name: "rocket"}},
		},
	}
	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != 2 {
		t.Fatalf("expected 1 row × 2 cells, got %d rows", len(grid.Rows))
	}
	c0 := grid.Rows[0].Cells[0]
	if c0.Shape == nil || c0.Shape.Icon == nil {
		t.Fatal("expected first cell to have Shape.Icon populated")
	}
	if c0.Shape.Icon.Path != "/abs/logo.svg" {
		t.Errorf("expected Path=/abs/logo.svg, got %q (full icon=%+v)", c0.Shape.Icon.Path, c0.Shape.Icon)
	}
	if c0.Shape.Icon.Name != "" {
		t.Errorf("expected Name=empty (path-based), got %q", c0.Shape.Icon.Name)
	}
	if c0.Shape.Icon.Fill != "#FF0000" {
		t.Errorf("expected caller-supplied Fill #FF0000, got %q", c0.Shape.Icon.Fill)
	}
	c1 := grid.Rows[0].Cells[1]
	if c1.Shape.Icon == nil || c1.Shape.Icon.Name != "rocket" {
		t.Errorf("expected second cell Icon.Name=rocket, got %+v", c1.Shape.Icon)
	}
}

func TestIconRefSchema_AcceptsBothForms(t *testing.T) {
	s := IconRefSchema("test")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `"oneOf"`) {
		t.Errorf("schema missing oneOf: %s", out)
	}
	if !strings.Contains(out, `"svg_data"`) {
		t.Errorf("schema missing svg_data property: %s", out)
	}
}
