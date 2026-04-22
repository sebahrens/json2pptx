package main

import (
	"encoding/json"
	"testing"
)

// --- TableCellInput custom UnmarshalJSON ---

func TestTableCellInput_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    TableCellInput
		wantErr bool
	}{
		{
			name:  "string shorthand",
			input: `"Hello"`,
			want:  TableCellInput{Content: "Hello", ColSpan: 1, RowSpan: 1},
		},
		{
			name:  "full object with col_span",
			input: `{"content":"Hello","col_span":2}`,
			want:  TableCellInput{Content: "Hello", ColSpan: 2, RowSpan: 1},
		},
		{
			name:  "full object with both spans",
			input: `{"content":"Cell","col_span":3,"row_span":2}`,
			want:  TableCellInput{Content: "Cell", ColSpan: 3, RowSpan: 2},
		},
		{
			name:  "object with zero spans defaults to 1",
			input: `{"content":"X"}`,
			want:  TableCellInput{Content: "X", ColSpan: 1, RowSpan: 1},
		},
		{
			name:  "empty string shorthand",
			input: `""`,
			want:  TableCellInput{Content: "", ColSpan: 1, RowSpan: 1},
		},
		{
			name:    "invalid JSON",
			input:   `{broken`,
			wantErr: true,
		},
		{
			name:    "number not accepted",
			input:   `42`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cell TableCellInput
			err := json.Unmarshal([]byte(tt.input), &cell)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cell.Content != tt.want.Content {
				t.Errorf("Content = %q, want %q", cell.Content, tt.want.Content)
			}
			if cell.ColSpan != tt.want.ColSpan {
				t.Errorf("ColSpan = %d, want %d", cell.ColSpan, tt.want.ColSpan)
			}
			if cell.RowSpan != tt.want.RowSpan {
				t.Errorf("RowSpan = %d, want %d", cell.RowSpan, tt.want.RowSpan)
			}
		})
	}
}

func TestTableCellInput_MixedRow(t *testing.T) {
	input := `["A", {"content":"B","col_span":3}]`
	var row []TableCellInput
	if err := json.Unmarshal([]byte(input), &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(row) != 2 {
		t.Fatalf("len(row) = %d, want 2", len(row))
	}
	if row[0].Content != "A" || row[0].ColSpan != 1 {
		t.Errorf("row[0] = %+v, want Content=A, ColSpan=1", row[0])
	}
	if row[1].Content != "B" || row[1].ColSpan != 3 {
		t.Errorf("row[1] = %+v, want Content=B, ColSpan=3", row[1])
	}
}

// --- TableInput.ToTableSpec() conversion ---

func TestTableInput_ToTableSpec(t *testing.T) {
	input := TableInput{
		Headers: []string{"Name", "Value"},
		Rows: [][]TableCellInput{
			{
				{Content: "A", ColSpan: 1, RowSpan: 1},
				{Content: "1", ColSpan: 1, RowSpan: 1},
			},
		},
		Style: &TableStyleInput{
			HeaderBackground: strPtr("accent2"),
			Borders:          "horizontal",
			Striped:          boolPtr(true),
		},
		ColumnAlignments: []string{"left", "right"},
	}

	spec := input.ToTableSpec()
	if spec == nil {
		t.Fatal("ToTableSpec returned nil")
	}

	// Headers
	if len(spec.Headers) != 2 || spec.Headers[0] != "Name" || spec.Headers[1] != "Value" {
		t.Errorf("Headers = %v, want [Name Value]", spec.Headers)
	}

	// Rows
	if len(spec.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(spec.Rows))
	}
	if spec.Rows[0][0].Content != "A" {
		t.Errorf("Rows[0][0].Content = %q, want A", spec.Rows[0][0].Content)
	}
	if spec.Rows[0][1].Content != "1" {
		t.Errorf("Rows[0][1].Content = %q, want 1", spec.Rows[0][1].Content)
	}

	// Style
	if spec.Style.HeaderBackground != "accent2" {
		t.Errorf("Style.HeaderBackground = %q, want accent2", spec.Style.HeaderBackground)
	}
	if spec.Style.Borders != "horizontal" {
		t.Errorf("Style.Borders = %q, want horizontal", spec.Style.Borders)
	}
	if spec.Style.Striped == nil || !*spec.Style.Striped {
		t.Error("Style.Striped should be true")
	}

	// Column alignments
	if len(spec.ColumnAlignments) != 2 || spec.ColumnAlignments[0] != "left" || spec.ColumnAlignments[1] != "right" {
		t.Errorf("ColumnAlignments = %v, want [left right]", spec.ColumnAlignments)
	}
}

func TestTableInput_ToTableSpec_DefaultStyle(t *testing.T) {
	input := TableInput{
		Headers: []string{"H1"},
		Rows: [][]TableCellInput{
			{{Content: "V1", ColSpan: 1, RowSpan: 1}},
		},
	}

	spec := input.ToTableSpec()
	// When no style is provided, DefaultTableStyle should be used (empty = no fill override)
	if spec.Style.HeaderBackground != "" {
		t.Errorf("default Style.HeaderBackground = %q, want empty", spec.Style.HeaderBackground)
	}
	if spec.Style.Borders != "all" {
		t.Errorf("default Style.Borders = %q, want all", spec.Style.Borders)
	}
}

func TestTableInput_ToTableSpec_Nil(t *testing.T) {
	var input *TableInput
	if spec := input.ToTableSpec(); spec != nil {
		t.Errorf("nil.ToTableSpec() = %v, want nil", spec)
	}
}

func TestTableInput_ToTableSpec_UseTableStyle(t *testing.T) {
	customStyleID := "{CUSTOM-GUID-1234}"
	input := TableInput{
		Headers: []string{"A", "B"},
		Rows: [][]TableCellInput{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: &TableStyleInput{
			UseTableStyle: true,
			StyleID:       customStyleID,
		},
	}

	spec := input.ToTableSpec()
	if !spec.Style.UseTableStyle {
		t.Error("Style.UseTableStyle = false, want true")
	}
	if spec.Style.StyleID != customStyleID {
		t.Errorf("Style.StyleID = %q, want %q", spec.Style.StyleID, customStyleID)
	}
}


func TestTableInput_ToTableSpec_UseTableStyleDefaultStyleID(t *testing.T) {
	input := TableInput{
		Headers: []string{"A"},
		Rows: [][]TableCellInput{
			{{Content: "1", ColSpan: 1, RowSpan: 1}},
		},
		Style: &TableStyleInput{
			UseTableStyle: true,
		},
	}

	spec := input.ToTableSpec()
	// When StyleID is not set, it should default to the standard table style GUID
	wantStyleID := "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	if spec.Style.StyleID != wantStyleID {
		t.Errorf("Style.StyleID = %q, want default %q", spec.Style.StyleID, wantStyleID)
	}
}

// --- ThemeInput.ToThemeOverride() ---

func TestThemeInput_ToThemeOverride(t *testing.T) {
	input := ThemeInput{
		Colors:    map[string]string{"accent1": "#FF0000", "dk1": "#000000"},
		TitleFont: "Arial",
		BodyFont:  "Georgia",
	}
	override := input.ToThemeOverride()
	if override == nil {
		t.Fatal("ToThemeOverride returned nil")
	}
	if override.Colors["accent1"] != "#FF0000" {
		t.Errorf("Colors[accent1] = %q, want #FF0000", override.Colors["accent1"])
	}
	if override.Colors["dk1"] != "#000000" {
		t.Errorf("Colors[dk1] = %q, want #000000", override.Colors["dk1"])
	}
	if override.TitleFont != "Arial" {
		t.Errorf("TitleFont = %q, want Arial", override.TitleFont)
	}
	if override.BodyFont != "Georgia" {
		t.Errorf("BodyFont = %q, want Georgia", override.BodyFont)
	}
}

func TestThemeInput_ToThemeOverride_Nil(t *testing.T) {
	var input *ThemeInput
	if override := input.ToThemeOverride(); override != nil {
		t.Errorf("nil.ToThemeOverride() = %v, want nil", override)
	}
}

func TestThemeInput_ToThemeOverride_Empty(t *testing.T) {
	input := ThemeInput{}
	override := input.ToThemeOverride()
	if override == nil {
		t.Fatal("ToThemeOverride returned nil for empty input")
	}
	if len(override.Colors) != 0 {
		t.Errorf("Colors = %v, want empty", override.Colors)
	}
}

// --- ContentInput backward compat: legacy Value json.RawMessage ---

func TestContentInput_LegacyValueFallback(t *testing.T) {
	t.Run("legacy text via Value", func(t *testing.T) {
		jsonStr := `{"placeholder_id":"p1","type":"text","value":"hello"}`
		var item ContentInput
		if err := json.Unmarshal([]byte(jsonStr), &item); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if item.TextValue != nil {
			t.Error("TextValue should be nil for legacy format")
		}
		if len(item.Value) == 0 {
			t.Error("Value should be set for legacy format")
		}
		// ResolveValue should still work
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue error: %v", err)
		}
		if val.(string) != "hello" {
			t.Errorf("ResolveValue = %v, want hello", val)
		}
	})

	t.Run("new text via text_value", func(t *testing.T) {
		jsonStr := `{"placeholder_id":"p1","type":"text","text_value":"hello"}`
		var item ContentInput
		if err := json.Unmarshal([]byte(jsonStr), &item); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if item.TextValue == nil || *item.TextValue != "hello" {
			t.Errorf("TextValue = %v, want 'hello'", item.TextValue)
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue error: %v", err)
		}
		if val.(string) != "hello" {
			t.Errorf("ResolveValue = %v, want hello", val)
		}
	})

	t.Run("typed field takes priority over legacy", func(t *testing.T) {
		jsonStr := `{"placeholder_id":"p1","type":"text","text_value":"typed","value":"legacy"}`
		var item ContentInput
		if err := json.Unmarshal([]byte(jsonStr), &item); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue error: %v", err)
		}
		if val.(string) != "typed" {
			t.Errorf("typed field should take priority, got %v", val)
		}
	})
}

// --- ContentInput.ResolveValue for all types ---

func TestContentInput_ResolveValue(t *testing.T) {
	t.Run("bullets via typed field", func(t *testing.T) {
		bullets := []string{"a", "b", "c"}
		item := ContentInput{
			PlaceholderID: "body",
			Type:          "bullets",
			BulletsValue:  &bullets,
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		bs := val.([]string)
		if len(bs) != 3 || bs[0] != "a" {
			t.Errorf("bullets = %v, want [a b c]", bs)
		}
	})

	t.Run("bullets via legacy Value", func(t *testing.T) {
		item := ContentInput{
			PlaceholderID: "body",
			Type:          "bullets",
			Value:         json.RawMessage(`["x","y"]`),
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		bs := val.([]string)
		if len(bs) != 2 || bs[0] != "x" {
			t.Errorf("bullets = %v, want [x y]", bs)
		}
	})

	t.Run("body_and_bullets via typed field", func(t *testing.T) {
		bab := &BodyAndBulletsInput{
			Body:         "intro",
			Bullets:      []string{"point1"},
			TrailingBody: "conclusion",
		}
		item := ContentInput{
			PlaceholderID:       "body",
			Type:                "body_and_bullets",
			BodyAndBulletsValue: bab,
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result := val.(*BodyAndBulletsInput)
		if result.Body != "intro" {
			t.Errorf("Body = %q, want intro", result.Body)
		}
		if result.TrailingBody != "conclusion" {
			t.Errorf("TrailingBody = %q, want conclusion", result.TrailingBody)
		}
	})

	t.Run("body_and_bullets via legacy Value", func(t *testing.T) {
		item := ContentInput{
			PlaceholderID: "body",
			Type:          "body_and_bullets",
			Value:         json.RawMessage(`{"body":"hello","bullets":["a","b"]}`),
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result := val.(*BodyAndBulletsInput)
		if result.Body != "hello" {
			t.Errorf("Body = %q, want hello", result.Body)
		}
	})

	t.Run("bullet_groups via typed field", func(t *testing.T) {
		bg := &BulletGroupsInput{
			Body: "overview",
			Groups: []BulletGroupInput{
				{Header: "Section A", Bullets: []string{"item1"}},
			},
		}
		item := ContentInput{
			PlaceholderID:     "body",
			Type:              "bullet_groups",
			BulletGroupsValue: bg,
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result := val.(*BulletGroupsInput)
		if len(result.Groups) != 1 || result.Groups[0].Header != "Section A" {
			t.Errorf("groups = %+v, want 1 group with header Section A", result.Groups)
		}
	})

	t.Run("table via typed field", func(t *testing.T) {
		table := &TableInput{
			Headers: []string{"A", "B"},
			Rows: [][]TableCellInput{
				{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
			},
		}
		item := ContentInput{
			PlaceholderID: "body",
			Type:          "table",
			TableValue:    table,
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result := val.(*TableInput)
		if len(result.Headers) != 2 {
			t.Errorf("headers = %v, want 2", result.Headers)
		}
	})

	t.Run("chart via legacy returns nil nil", func(t *testing.T) {
		item := ContentInput{
			PlaceholderID: "chart",
			Type:          "chart",
		}
		val, err := item.ResolveValue()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if val != nil {
			t.Errorf("expected nil for chart without typed field, got %v", val)
		}
	})

	t.Run("unknown type returns error", func(t *testing.T) {
		item := ContentInput{
			PlaceholderID: "p1",
			Type:          "video",
		}
		_, err := item.ResolveValue()
		if err == nil {
			t.Fatal("expected error for unknown type")
		}
	})

	t.Run("text with no value or typed field errors", func(t *testing.T) {
		item := ContentInput{
			PlaceholderID: "p1",
			Type:          "text",
		}
		_, err := item.ResolveValue()
		if err == nil {
			t.Fatal("expected error for text with no value")
		}
	})
}

// --- Full PresentationInput round-trip ---

func TestPresentationInput_FullRoundTrip(t *testing.T) {
	input := `{
		"template": "template_1",
		"theme_override": {
			"colors": {"accent1": "#336699"},
			"title_font": "Helvetica",
			"body_font": "Calibri"
		},
		"footer": {
			"enabled": true,
			"left_text": "Acme Corp"
		},
		"slides": [{
			"layout_id": "slideLayout1",
			"speaker_notes": "Talk about this",
			"source": "Annual Report 2025",
			"transition": "fade",
			"transition_speed": "fast",
			"build": "bullets",
			"content": [
				{"placeholder_id": "title", "type": "text", "text_value": "Hello World"},
				{"placeholder_id": "body", "type": "table", "table_value": {
					"headers": ["A", "B"],
					"rows": [["1", "2"], [{"content": "3", "col_span": 2}]]
				}},
				{"placeholder_id": "bullets", "type": "bullets", "bullets_value": ["first", "second"]},
				{"placeholder_id": "detail", "type": "body_and_bullets", "body_and_bullets_value": {
					"body": "intro text",
					"bullets": ["point 1", "point 2"],
					"trailing_body": "conclusion"
				}},
				{"placeholder_id": "groups", "type": "bullet_groups", "bullet_groups_value": {
					"body": "overview",
					"groups": [
						{"header": "Section A", "bullets": ["item 1"]},
						{"header": "Section B", "body": "sub-intro", "bullets": ["item 2", "item 3"]}
					],
					"trailing_body": "end note"
				}}
			]
		}]
	}`

	var p PresentationInput
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Top-level fields
	if p.Template != "template_1" {
		t.Errorf("Template = %q, want template_1", p.Template)
	}

	// Theme override
	if p.ThemeOverride == nil {
		t.Fatal("ThemeOverride is nil")
	}
	if p.ThemeOverride.Colors["accent1"] != "#336699" {
		t.Errorf("ThemeOverride.Colors[accent1] = %q, want #336699", p.ThemeOverride.Colors["accent1"])
	}
	if p.ThemeOverride.TitleFont != "Helvetica" {
		t.Errorf("ThemeOverride.TitleFont = %q, want Helvetica", p.ThemeOverride.TitleFont)
	}
	if p.ThemeOverride.BodyFont != "Calibri" {
		t.Errorf("ThemeOverride.BodyFont = %q, want Calibri", p.ThemeOverride.BodyFont)
	}

	// Footer
	if p.Footer == nil {
		t.Fatal("Footer is nil")
	}
	if !p.Footer.Enabled {
		t.Error("Footer.Enabled = false, want true")
	}
	if p.Footer.LeftText != "Acme Corp" {
		t.Errorf("Footer.LeftText = %q, want Acme Corp", p.Footer.LeftText)
	}

	// Slide
	if len(p.Slides) != 1 {
		t.Fatalf("len(Slides) = %d, want 1", len(p.Slides))
	}
	s := p.Slides[0]

	if s.LayoutID != "slideLayout1" {
		t.Errorf("LayoutID = %q, want slideLayout1", s.LayoutID)
	}
	if s.SpeakerNotes != "Talk about this" {
		t.Errorf("SpeakerNotes = %q, want 'Talk about this'", s.SpeakerNotes)
	}
	if s.Source != "Annual Report 2025" {
		t.Errorf("Source = %q, want 'Annual Report 2025'", s.Source)
	}
	if s.Transition != "fade" {
		t.Errorf("Transition = %q, want fade", s.Transition)
	}
	if s.TransitionSpeed != "fast" {
		t.Errorf("TransitionSpeed = %q, want fast", s.TransitionSpeed)
	}
	if s.Build != "bullets" {
		t.Errorf("Build = %q, want bullets", s.Build)
	}

	// Content items
	if len(s.Content) != 5 {
		t.Fatalf("len(Content) = %d, want 5", len(s.Content))
	}

	// text_value
	if s.Content[0].TextValue == nil || *s.Content[0].TextValue != "Hello World" {
		t.Errorf("Content[0].TextValue = %v, want 'Hello World'", s.Content[0].TextValue)
	}

	// table_value
	if s.Content[1].TableValue == nil {
		t.Fatal("Content[1].TableValue is nil")
	}
	if s.Content[1].TableValue.Headers[0] != "A" {
		t.Errorf("table headers[0] = %q, want A", s.Content[1].TableValue.Headers[0])
	}
	if len(s.Content[1].TableValue.Rows) != 2 {
		t.Fatalf("table rows = %d, want 2", len(s.Content[1].TableValue.Rows))
	}
	// First row: string shorthands "1", "2"
	if s.Content[1].TableValue.Rows[0][0].Content != "1" {
		t.Errorf("table[0][0] = %q, want 1", s.Content[1].TableValue.Rows[0][0].Content)
	}
	// Second row: mixed - "3" with col_span 2
	if s.Content[1].TableValue.Rows[1][0].ColSpan != 2 {
		t.Errorf("table[1][0].ColSpan = %d, want 2", s.Content[1].TableValue.Rows[1][0].ColSpan)
	}

	// bullets_value
	if s.Content[2].BulletsValue == nil {
		t.Fatal("Content[2].BulletsValue is nil")
	}
	bs := *s.Content[2].BulletsValue
	if len(bs) != 2 || bs[0] != "first" {
		t.Errorf("BulletsValue = %v, want [first second]", bs)
	}

	// body_and_bullets_value
	if s.Content[3].BodyAndBulletsValue == nil {
		t.Fatal("Content[3].BodyAndBulletsValue is nil")
	}
	bab := s.Content[3].BodyAndBulletsValue
	if bab.Body != "intro text" {
		t.Errorf("bab.Body = %q, want 'intro text'", bab.Body)
	}
	if bab.TrailingBody != "conclusion" {
		t.Errorf("bab.TrailingBody = %q, want conclusion", bab.TrailingBody)
	}

	// bullet_groups_value
	if s.Content[4].BulletGroupsValue == nil {
		t.Fatal("Content[4].BulletGroupsValue is nil")
	}
	bg := s.Content[4].BulletGroupsValue
	if bg.Body != "overview" {
		t.Errorf("bg.Body = %q, want overview", bg.Body)
	}
	if len(bg.Groups) != 2 {
		t.Fatalf("len(bg.Groups) = %d, want 2", len(bg.Groups))
	}
	if bg.Groups[0].Header != "Section A" {
		t.Errorf("bg.Groups[0].Header = %q, want Section A", bg.Groups[0].Header)
	}
	if bg.Groups[1].Body != "sub-intro" {
		t.Errorf("bg.Groups[1].Body = %q, want sub-intro", bg.Groups[1].Body)
	}
	if bg.TrailingBody != "end note" {
		t.Errorf("bg.TrailingBody = %q, want 'end note'", bg.TrailingBody)
	}
}

// --- PresentationInput to JSONInput compat ---

func TestPresentationInput_LegacyJSONInputStillWorks(t *testing.T) {
	// Verify that the old JSONInput struct still deserializes correctly
	input := `{
		"template": "template_2",
		"slides": [{
			"layout_id": "slideLayout3",
			"content": [
				{"placeholder_id": "title", "type": "text", "value": "Legacy Title"},
				{"placeholder_id": "body", "type": "bullets", "value": ["a", "b", "c"]}
			]
		}]
	}`

	var ji JSONInput
	if err := json.Unmarshal([]byte(input), &ji); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if ji.Template != "template_2" {
		t.Errorf("Template = %q, want template_2", ji.Template)
	}
	if len(ji.Slides) != 1 {
		t.Fatalf("len(Slides) = %d, want 1", len(ji.Slides))
	}
	if ji.Slides[0].LayoutID != "slideLayout3" {
		t.Errorf("LayoutID = %q, want slideLayout3", ji.Slides[0].LayoutID)
	}
	if len(ji.Slides[0].Content) != 2 {
		t.Fatalf("len(Content) = %d, want 2", len(ji.Slides[0].Content))
	}
	// Legacy text
	var text string
	if err := json.Unmarshal(ji.Slides[0].Content[0].Value, &text); err != nil {
		t.Fatalf("unmarshal text: %v", err)
	}
	if text != "Legacy Title" {
		t.Errorf("text = %q, want 'Legacy Title'", text)
	}
	// Legacy bullets
	var bullets []string
	if err := json.Unmarshal(ji.Slides[0].Content[1].Value, &bullets); err != nil {
		t.Fatalf("unmarshal bullets: %v", err)
	}
	if len(bullets) != 3 || bullets[0] != "a" {
		t.Errorf("bullets = %v, want [a b c]", bullets)
	}
}
