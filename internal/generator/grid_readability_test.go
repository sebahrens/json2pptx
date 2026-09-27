package generator

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

func TestGridReadabilityRoleBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, scale, size string
		role              tokens.TextRole
		mode              tokens.ViewingMode
		refuses           bool
	}{
		{"body below present floor", "75000", "1200", tokens.TextRoleBody, tokens.ViewingModePresentation, true},
		{"body at present floor", "100000", "1200", tokens.TextRoleBody, tokens.ViewingModePresentation, false},
		{"body at report floor", "100000", "1000", tokens.TextRoleBody, tokens.ViewingModeReport, false},
		{"caption at present floor", "100000", "1000", tokens.TextRoleCaption, tokens.ViewingModePresentation, false},
		{"caption below present floor", "99000", "1000", tokens.TextRoleCaption, tokens.ViewingModePresentation, true},
		{"caption at report floor", "100000", "700", tokens.TextRoleCaption, tokens.ViewingModeReport, false},
		{"KPI below floor", "50000", "2400", tokens.TextRoleKPIValue, tokens.ViewingModePresentation, true},
		{"KPI at floor", "75000", "2400", tokens.TextRoleKPIValue, tokens.ViewingModePresentation, false},
		{"unknown shrink cannot fix small body", "", "1100", tokens.TextRoleBody, tokens.ViewingModePresentation, true},
		{"unknown shrink cannot convict readable body", "", "1200", tokens.TextRoleBody, tokens.ViewingModePresentation, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &singlePassContext{}
			ctx.viewingMode = tc.mode
			shapes := [][]byte{[]byte(slideWithAutofit(tc.scale, tc.size))}
			roles := map[uint32][]tokens.TextRole{1: {tc.role}}
			before, err := json.Marshal(roles)
			if err != nil {
				t.Fatal(err)
			}
			raw := bytes.Clone(shapes[0])
			ctx.reportGridReadability(shapes, roles, 2)
			if tc.refuses {
				if len(ctx.fitFindings) != 1 {
					t.Fatalf("missing role refusal: %+v", ctx.fitFindings)
				}
				f := ctx.fitFindings[0]
				if f.Action != "refuse" || f.Fix != nil || f.Path != "/slides/2/rendered_shapes/1/paragraphs/0" || strings.Contains(f.Message, "shorten") || !strings.Contains(f.Message, "preserve all source") {
					t.Fatalf("unsafe role refusal: %+v", f)
				}
			} else if len(ctx.fitFindings) != 0 {
				t.Fatalf("readable role refused: %+v", ctx.fitFindings)
			}
			after, err := json.Marshal(roles)
			if err != nil || !bytes.Equal(before, after) || !bytes.Equal(raw, shapes[0]) {
				t.Fatal("source roles or shape XML mutated")
			}
		})
	}
}

func TestGridReadabilityKeepsParagraphRolesAligned(t *testing.T) {
	ctx := &singlePassContext{}
	ctx.viewingMode = tokens.ViewingModePresentation
	raw := slideWithAutofit("80000", "2400", "1200", "2400")
	// The first and last runs are readable body/KPI, but the middle caption
	// falls to 9.6pt. Empty paragraphs must not move its caption role.
	raw = strings.Replace(raw, "</a:bodyPr>", "</a:bodyPr><a:p/>", 1)
	ctx.reportGridReadability([][]byte{[]byte(raw)}, map[uint32][]tokens.TextRole{1: {"", tokens.TextRoleKPIValue, tokens.TextRoleCaption, tokens.TextRoleBody}}, 0)
	if len(ctx.fitFindings) != 1 || ctx.fitFindings[0].Path != "/slides/0/rendered_shapes/1/paragraphs/2" || !strings.Contains(ctx.fitFindings[0].Message, "caption text renders at 9.6pt") {
		t.Fatalf("caption role slid onto another paragraph: %+v", ctx.fitFindings)
	}
}

func TestGridReadabilityDoesNotGuessUnknownRoles(t *testing.T) {
	raw := []byte(slideWithAutofit("75000", "1200"))
	for _, roles := range []map[uint32][]tokens.TextRole{nil, {2: {tokens.TextRoleBody}}, {1: {}}, {1: {""}}, {1: {tokens.TextRoleBody, tokens.TextRoleCaption}}} {
		ctx := &singlePassContext{}
		ctx.viewingMode = tokens.ViewingModePresentation
		ctx.reportGridReadability([][]byte{raw}, roles, 0)
		if len(ctx.fitFindings) != 0 {
			t.Fatalf("unknown/misaligned roles invented: %+v", ctx.fitFindings)
		}
	}
}

func TestGridReadabilityCountsOnlyPopulatedRunSizes(t *testing.T) {
	ctx := &singlePassContext{}
	ctx.viewingMode = tokens.ViewingModePresentation
	raw := slideWithAutofit("100000", "1200")
	raw = strings.Replace(raw, "</a:p>", `<a:r><a:rPr sz="100"/><a:t> </a:t></a:r><a:endParaRPr sz="100"/></a:p>`, 1)
	ctx.reportGridReadability([][]byte{[]byte(raw)}, map[uint32][]tokens.TextRole{1: {tokens.TextRoleBody}}, 0)
	if len(ctx.fitFindings) != 0 {
		t.Fatalf("empty styles refused readable body: %+v", ctx.fitFindings)
	}
	raw = strings.Replace(raw, "<a:t> </a:t>", "<a:t>Required child run</a:t>", 1)
	ctx.reportGridReadability([][]byte{[]byte(raw)}, map[uint32][]tokens.TextRole{1: {tokens.TextRoleBody}}, 0)
	if len(ctx.fitFindings) != 1 {
		t.Fatalf("small populated child run was ignored: %+v", ctx.fitFindings)
	}
}

func TestGridReadabilityRefusesFramesWithoutUsableTextArea(t *testing.T) {
	for _, tc := range []struct {
		name, extent, insets string
		refuses              bool
	}{
		{"default padding consumes frame", `<a:ext cx="1000000" cy="23812"/>`, "", true},
		{"explicit padding consumes frame", `<a:ext cx="1000000" cy="254000"/>`, `tIns="127000" bIns="127000"`, true},
		{"explicit zero padding fits small frame", `<a:ext cx="1000000" cy="254000"/>`, `lIns="0" tIns="0" rIns="0" bIns="0"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := slideWithAutofit("", "1200")
			raw = strings.Replace(raw, "<p:spPr/>", "<p:spPr><a:xfrm>"+tc.extent+"</a:xfrm></p:spPr>", 1)
			if tc.insets != "" {
				raw = strings.Replace(raw, "<a:bodyPr>", "<a:bodyPr "+tc.insets+">", 1)
			}
			ctx := &singlePassContext{}
			ctx.viewingMode = tokens.ViewingModePresentation
			ctx.reportGridReadability([][]byte{[]byte(raw)}, map[uint32][]tokens.TextRole{1: {tokens.TextRoleBody}}, 0)
			if tc.refuses {
				if len(ctx.fitFindings) != 1 || ctx.fitFindings[0].Action != "refuse" || ctx.fitFindings[0].Fix != nil || !strings.Contains(ctx.fitFindings[0].Message, "no usable area") {
					t.Fatalf("unusable frame published: %+v", ctx.fitFindings)
				}
			} else if len(ctx.fitFindings) != 0 {
				t.Fatalf("explicit zero padding replaced with defaults: %+v", ctx.fitFindings)
			}
		})
	}
}
