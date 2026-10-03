package generator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
)

// Abstract's divider titles split ordinary words — BOTTLENEC / K AND,
// COMMITTIN / G TO SCALE — at 43.5pt: the longest-word guard measured the
// Tenorite title in its Liberation Sans stand-in with a 3% margin
// (go-slide-creator-akues).
var abstractDividerTitles = map[string]string{
	"Diagnose the bottleneck and choose the model": "BOTTLENECK",
	"Prove the model before committing to scale":   "COMMITTING",
}

func generateAbstractDivider(t *testing.T, title string) (string, []patterns.FitFinding) {
	t.Helper()
	tpl := "../../templates/abstract.pptx"
	if _, err := os.Stat(tpl); err != nil {
		t.Skipf("abstract template not found: %v", err)
	}
	out := filepath.Join(t.TempDir(), "divider.pptx")
	result, _, err := generateSinglePass(context.Background(), GenerationRequest{
		TemplatePath: tpl, OutputPath: out, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentSectionTitle, Value: title},
			{PlaceholderID: "section_number", Type: ContentSectionTitle, Value: "01"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, result.FitFindings
}

func TestAbstractDividerKeepsLongestWordWhole(t *testing.T) {
	sizeRE := regexp.MustCompile(`<a:rPr[^>]*\bsz="(\d+)"`)
	for title, word := range abstractDividerTitles {
		t.Run(word, func(t *testing.T) {
			out, findings := generateAbstractDivider(t, title)
			for _, f := range findings {
				if f.Code == patterns.ErrCodeTitleTruncated {
					t.Fatalf("divider title refused, want it to fit at >= 28pt: %+v", f)
				}
			}
			slide := readZipFileString(t, out, "ppt/slides/slide1.xml")
			titleXML := slide[:strings.Index(slide, `name="Section Number"`)]
			m := sizeRE.FindStringSubmatch(titleXML)
			if m == nil {
				t.Fatal("divider title has no explicit run size")
			}
			sz, _ := strconv.Atoi(m[1])
			if sz < SectionTitleMinHPt {
				t.Fatalf("divider title at %d hpt, below the %d floor", sz, SectionTitleMinHPt)
			}
			// The line width the box gives its text (default insets, no
			// margins) must hold the all-caps, tracked word with the
			// renderer slack the shape writer leaves a host-dependent face.
			const boxW, insets = 4179570, 2 * 91440
			need, ok := pptx.WordLineNeedEMU(word, "Tenorite", float64(sz)/100, false, 150)
			if !ok {
				t.Skip("no measurement font")
			}
			if need > boxW-insets {
				t.Errorf("%s needs %d EMU at %d hpt, the line holds %d", word, need, sz, boxW-insets)
			}
		})
	}
}

// TestAbstractDividerRenderedLineBoundaries renders the divider and checks
// that the rendered lines keep BOTTLENECK and COMMITTING whole, rather than
// trusting the written font size.
func TestAbstractDividerRenderedLineBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("renders with LibreOffice")
	}
	soffice, err := exec.LookPath("soffice")
	if err != nil {
		t.Skip("soffice not installed")
	}
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("pdftotext not installed")
	}
	for title, word := range abstractDividerTitles {
		t.Run(word, func(t *testing.T) {
			out, _ := generateAbstractDivider(t, title)
			dir := filepath.Dir(out)
			cmd := exec.Command(soffice, "-env:UserInstallation=file://"+filepath.Join(dir, "lo"), "--headless", "--convert-to", "pdf", "--outdir", dir, out) // #nosec G204 -- soffice from PATH converting a test-owned temp file.
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Skipf("soffice failed: %v: %s", err, b)
			}
			text, err := exec.Command(pdftotext, "-layout", filepath.Join(dir, "divider.pdf"), "-").Output() // #nosec G204 -- pdftotext from PATH reading a test-owned temp file.
			if err != nil {
				t.Fatal(err)
			}
			whole := false
			for _, line := range strings.Split(string(text), "\n") {
				for _, w := range strings.Fields(line) {
					if strings.EqualFold(w, word) {
						whole = true
					}
				}
			}
			if !whole {
				t.Errorf("rendered divider lines do not keep %s on one line:\n%s", word, text)
			}
		})
	}
}

func TestDetectSectionTitleFloorRefusesUnbreakableWord(t *testing.T) {
	in := TitleFitInput{
		WidthEMU: 4179570, HeightEMU: 3377354, FontName: "Tenorite",
		Style: template.InheritedTextStyle{SizeHPt: 5400, CapsAll: true, SpcHPt: 150},
	}
	in.Title = "Diagnose the bottleneck and choose the model"
	if f := DetectSectionTitleFloor("/slides/0/content/0", in); f != nil {
		t.Fatalf("ordinary divider words fit at 28pt: %+v", f)
	}
	in.Title = "The Donaudampfschifffahrtsgesellschaft plan"
	f := DetectSectionTitleFloor("/slides/0/content/0", in)
	if f == nil || f.Code != patterns.ErrCodeTitleTruncated || f.Action != "refuse" {
		t.Fatalf("a word wider than the divider at 28pt must be refused: %+v", f)
	}
	if !strings.Contains(f.Message, "DONAUDAMPFSCHIFFFAHRTSGESELLSCHAFT") {
		t.Errorf("refusal must name the word that breaks: %q", f.Message)
	}
	if got := f.Fix.Params["max_chars"]; got != len("The") {
		t.Errorf("max_chars = %v, want the text before the word (3)", got)
	}
}
