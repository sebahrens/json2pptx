package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

func TestResolvedGridRolesUseWrittenParagraphOrder(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		override   tokens.TextRole
		want       []tokens.TextRole
	}{
		{"blank paragraph retained", `{"paragraphs":[{"content":"48%","size":32},{"content":""},{"content":"KPI caption","size":12}]}`, "", []tokens.TextRole{tokens.TextRoleKPIValue, "", tokens.TextRoleCaption}},
		{"inline emphasis does not turn prose into a title", `{"content":"This required body paragraph has <b>emphasis</b> but is not a bold card title."}`, "", []tokens.TextRole{tokens.TextRoleCardBody}},
		{"bold card title", `{"content":"Card title","bold":true}`, "", []tokens.TextRole{tokens.TextRoleCardTitle}},
		{"lone axis number is a caption, not a KPI", `{"content":"1","size":24,"bold":true}`, "", []tokens.TextRole{tokens.TextRoleCaption}},
		{"figure with caption stays KPI", `{"paragraphs":[{"content":"4","size":36,"bold":true},{"content":"new markets","size":14}]}`, "", []tokens.TextRole{tokens.TextRoleKPIValue, tokens.TextRoleCaption}},
		{"lone metric stays KPI", `{"content":"$4.2M","size":36,"bold":true}`, "", []tokens.TextRole{tokens.TextRoleKPIValue}},
		{"pull quote override", `{"content":"A quote is prose","size":36,"italic":true}`, tokens.TextRoleBody, []tokens.TextRole{tokens.TextRoleBody}},
		{"invalid source not measured", `123`, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cell := shapegrid.ResolvedCell{ShapeSpec: &shapegrid.ShapeSpec{Text: json.RawMessage(tc.text)}}
			got := resolvedCellTextRoles(cell, tc.override)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("roles=%v want %v", got, tc.want)
			}
			if string(cell.ShapeSpec.Text) != tc.text {
				t.Fatal("source text mutated")
			}
		})
	}
	if got := resolvedCellTextRoles(shapegrid.ResolvedCell{}, ""); got != nil {
		t.Fatalf("empty cell roles: %v", got)
	}
}

func TestConvertGridPreservesRoleMetadataOnNestedCells(t *testing.T) {
	var slide SlideInput
	if err := json.Unmarshal([]byte(`{"layout_id":"blank","shape_grid":{"columns":[100],"rows":[{"cells":[{"grid":{"columns":[100],"rows":[{"cells":[{"shape":{"geometry":"rect","text":{"paragraphs":[{"content":""},{"content":"This nested body paragraph must retain its role and its exact source text.","size":13}]}}}]}]}}]}]}}`), &slide); err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(slide)
	if err != nil {
		t.Fatal(err)
	}
	specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, nil, 12192000, 6858000, nil, nil, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || len(specs[0].GridTextRoles) != 1 {
		t.Fatalf("nested role metadata lost: %+v", specs)
	}
	for _, roles := range specs[0].GridTextRoles {
		if !reflect.DeepEqual(roles, []tokens.TextRole{"", tokens.TextRoleCardBody}) {
			t.Fatalf("nested roles reordered: %v", roles)
		}
	}
	after, err := json.Marshal(slide)
	if err != nil || string(before) != string(after) {
		t.Fatal("source slide changed")
	}
}
