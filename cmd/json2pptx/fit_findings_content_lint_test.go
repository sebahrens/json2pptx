package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func bulletsPtr(b []string) *[]string                              { return &b }
func bodyAndBulletsPtr(v BodyAndBulletsInput) *BodyAndBulletsInput { return &v }
func bulletGroupsPtr(v BulletGroupsInput) *BulletGroupsInput       { return &v }

// findFinding returns the first finding with matching code, or nil if absent.
func findFinding(findings []patterns.FitFinding, code string) *patterns.FitFinding {
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}

func TestContentLint_HeadlineTooLong(t *testing.T) {
	// 20-word title — must exceed the 12-word headline budget.
	words := make([]string, 20)
	for i := range words {
		words[i] = "word"
	}
	twentyWordTitle := strings.Join(words, " ")
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr(twentyWordTitle)},
			},
		}},
	}

	findings := collectContentLintFindings(input)
	f := findFinding(findings, patterns.ErrCodeHeadlineTooLong)
	if f == nil {
		t.Fatalf("expected HEADLINE_TOO_LONG finding, got %+v", findings)
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review", f.Action)
	}
	if !strings.Contains(f.Message, "20 words") {
		t.Errorf("message should report measured word count, got %q", f.Message)
	}
	if !strings.Contains(f.Message, "12") {
		t.Errorf("message should mention the 12-word budget, got %q", f.Message)
	}
	if f.Fix == nil || f.Fix.Kind != "shorten_title" {
		t.Errorf("fix kind = %v, want shorten_title", f.Fix)
	}
}

func TestContentLint_HeadlineWithinBudget(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Q1 results: revenue up 22%, margin steady")},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	if findFinding(findings, patterns.ErrCodeHeadlineTooLong) != nil {
		t.Errorf("did not expect HEADLINE_TOO_LONG for 8-word title, got %+v", findings)
	}
}

func TestContentLint_BodyTooLong(t *testing.T) {
	// 100 words.
	words := make([]string, 100)
	for i := range words {
		words[i] = "word"
	}
	body := strings.Join(words, " ")

	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "text", TextValue: strPtr(body)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	f := findFinding(findings, patterns.ErrCodeBodyTooLong)
	if f == nil {
		t.Fatalf("expected BODY_TOO_LONG finding for 100-word body, got %+v", findings)
	}
	if !strings.Contains(f.Message, "100 words") {
		t.Errorf("message should report 100 words, got %q", f.Message)
	}
	if !strings.Contains(f.Message, "80") {
		t.Errorf("message should mention 80-word budget, got %q", f.Message)
	}
	if f.Fix == nil || f.Fix.Kind != "reduce_text" {
		t.Errorf("fix kind = %v, want reduce_text", f.Fix)
	}
}

func TestContentLint_BodyTooLong_BulletsAggregated(t *testing.T) {
	// 10 bullets × 10 words = 100 words across the list.
	bullet := strings.Repeat("word ", 10)
	bullet = strings.TrimSpace(bullet)
	bullets := make([]string, 10)
	for i := range bullets {
		bullets[i] = bullet
	}

	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "bullets", BulletsValue: bulletsPtr(bullets)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	if findFinding(findings, patterns.ErrCodeBodyTooLong) == nil {
		t.Errorf("expected BODY_TOO_LONG when bullets sum > 80 words, got %+v", findings)
	}
}

func TestContentLint_BulletNestingDeep_LeadingSpaces(t *testing.T) {
	// "    - sub" → 4 leading spaces → 2 indent units → renders at level 3
	// (base 1 + 2 indent units), which exceeds the budget.
	bullets := []string{
		"Top-level item",
		"  Second level",
		"    Third level",
	}
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "bullets", BulletsValue: bulletsPtr(bullets)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	f := findFinding(findings, patterns.ErrCodeBulletNestingDeep)
	if f == nil {
		t.Fatalf("expected BULLET_NESTING_DEEP for 3-level bullets, got %+v", findings)
	}
	if !strings.Contains(f.Message, "3 levels") {
		t.Errorf("message should report 3-level depth, got %q", f.Message)
	}
	if f.Fix == nil || f.Fix.Kind != "reduce_text" {
		t.Errorf("fix kind = %v, want reduce_text", f.Fix)
	}
}

func TestContentLint_BulletNesting_FlatStaysWithinBudget(t *testing.T) {
	bullets := []string{"Apple", "Banana", "Cherry"}
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "bullets", BulletsValue: bulletsPtr(bullets)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	if findFinding(findings, patterns.ErrCodeBulletNestingDeep) != nil {
		t.Errorf("did not expect BULLET_NESTING_DEEP for flat list, got %+v", findings)
	}
}

func TestContentLint_BulletNesting_BulletGroupsHeaderPlusIndent(t *testing.T) {
	// bullet_groups already places headers at level 1 and bullets at level 2.
	// An indented bullet inside a group renders at level 3 → trip the budget.
	bg := BulletGroupsInput{
		Groups: []BulletGroupInput{{
			Header:  "Group",
			Bullets: []string{"  Indented sub-bullet"},
		}},
	}
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "bullet_groups", BulletGroupsValue: bulletGroupsPtr(bg)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	if findFinding(findings, patterns.ErrCodeBulletNestingDeep) == nil {
		t.Errorf("expected BULLET_NESTING_DEEP for bullet_groups with indented bullet, got %+v", findings)
	}
}

func TestContentLint_BulletIndentDepth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"flat", 0},
		{"  two-space indent", 1},
		{"    four-space indent", 2},
		{"\ttab", 1},
		{"\t\tdouble-tab", 2},
		{" - one space then dash", 0}, // single space rounds down
	}
	for _, c := range cases {
		got := bulletIndentDepth(c.in)
		if got != c.want {
			t.Errorf("bulletIndentDepth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestContentLint_BodyAndBullets_PathTargetsPlaceholder(t *testing.T) {
	bab := BodyAndBulletsInput{
		Body:    strings.Repeat("word ", 50),
		Bullets: []string{strings.Repeat("word ", 40)},
	}
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "body_and_bullets", BodyAndBulletsValue: bodyAndBulletsPtr(bab)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	f := findFinding(findings, patterns.ErrCodeBodyTooLong)
	if f == nil {
		t.Fatalf("expected BODY_TOO_LONG, got %+v", findings)
	}
	if f.Path != "/slides/0/content/body" {
		t.Errorf("path = %q, want /slides/0/content/body", f.Path)
	}
}

func TestContentLint_NilInputSafe(t *testing.T) {
	if findings := collectContentLintFindings(nil); findings != nil {
		t.Errorf("expected nil findings for nil input, got %+v", findings)
	}
}

func TestContentLint_NonTitleTextSkipsHeadlineCheck(t *testing.T) {
	// A long text on a non-title placeholder should not fire HEADLINE_TOO_LONG.
	long := strings.Repeat("word ", 20)
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "text", TextValue: strPtr(long)},
			},
		}},
	}
	findings := collectContentLintFindings(input)
	if findFinding(findings, patterns.ErrCodeHeadlineTooLong) != nil {
		t.Errorf("HEADLINE_TOO_LONG should only fire on title placeholders, got %+v", findings)
	}
}

// go-slide-creator-uy5s: an image background is invisible to the contrast pass
// (it reads a solid fill), so text on a photo gets no verdict at all. Report it
// rather than let the template's dark title land on the dark half of a picture.
func TestBackgroundFindings_TextOverImageUnverified(t *testing.T) {
	deck := func(bg string) *PresentationInput {
		var in PresentationInput
		raw := `{"template":"t","slides":[{"slide_type":"title","background":` + bg + `,
			"content":[{"placeholder_id":"title","type":"text","text_value":"Hero"}]}]}`
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return &in
	}
	codes := func(in *PresentationInput) int {
		n := 0
		for _, f := range collectBackgroundFindings(in) {
			if f.Code == patterns.ErrCodeTextOverImageUnverified {
				n++
			}
		}
		return n
	}

	if got := codes(deck(`{"image":"hero.jpg"}`)); got != 1 {
		t.Errorf("a photo with no scrim under text must be reported, got %d findings", got)
	}
	if got := codes(deck(`{"url":"https://example.com/hero.jpg"}`)); got != 1 {
		t.Errorf("a URL photo is the same case, got %d findings", got)
	}
	// A scrim answers the question, so there is nothing to report.
	if got := codes(deck(`{"image":"hero.jpg","overlay":{"color":"dk1","alpha":0.45}}`)); got != 0 {
		t.Errorf("a scrimmed photo must not be reported, got %d findings", got)
	}
	// A solid colour is checkable, so it is not this finding's business.
	if got := codes(deck(`{"color":"dk2"}`)); got != 0 {
		t.Errorf("a solid background must not be reported, got %d findings", got)
	}

	// A slide that opted out of contrast checking has already said it knows.
	optOut := deck(`{"image":"hero.jpg"}`)
	no := false
	optOut.Slides[0].ContrastCheck = &no
	if got := codes(optOut); got != 0 {
		t.Errorf("contrast_check:false must silence it, got %d findings", got)
	}

	// A picture-only slide has no text to be illegible.
	pictureOnly := deck(`{"image":"hero.jpg"}`)
	pictureOnly.Slides[0].Content = nil
	if got := codes(pictureOnly); got != 0 {
		t.Errorf("a slide with no text must not be asked to dim its photo, got %d findings", got)
	}
}
