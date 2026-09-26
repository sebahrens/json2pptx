package quality

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

const nativeGenerated = "generated_unreviewed"
const nativeRefused = "source_loss_refused"

// Every original probe is attempted independently before accepted probes are
// assembled. A refusal is an explicit non-image outcome, never visual approval.
func classifyNativePublication(t *testing.T, path string, probe nativeProbe, mode string) (string, *patterns.ValidationError, error) {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "probe.pptx")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := generator.Generate(ctx, generator.GenerationRequest{TemplatePath: path, OutputPath: out, StrictFit: mode, Slides: []generator.SlideSpec{probe.Slide}, ExcludeTemplateSlides: true, AllowedImagePaths: []string{testutil.RepoRoot()}})
	if err != nil {
		var loss *patterns.ValidationError
		if result != nil || !errors.As(err, &loss) || loss == nil || loss.Path == "" || loss.Fix == nil || (loss.Code != patterns.ErrCodeTextTrimmed && loss.Code != patterns.ErrCodeReadabilityTrimmed && loss.Code != patterns.ErrCodeTableRowsTruncated) {
			return "", nil, fmt.Errorf("unexpected generation failure: %w", err)
		}
		files, readErr := os.ReadDir(dir)
		if readErr != nil || len(files) != 0 {
			return "", nil, fmt.Errorf("refusal leaked artifacts: files=%v error=%v", files, readErr)
		}
		return nativeRefused, loss, nil
	}
	if result == nil || result.SlideCount != 1 || len(result.MediaFailures) != 0 || len(result.ValidationErrors) != 0 {
		return "", nil, fmt.Errorf("incomplete generation: %+v", result)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		return "", nil, err
	}
	defer zr.Close()
	var body []byte
	for _, file := range zr.File {
		if file.Name != "ppt/slides/slide1.xml" {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return "", nil, err
		}
		body, err = io.ReadAll(r)
		closeErr := r.Close()
		if err != nil {
			return "", nil, err
		}
		if closeErr != nil {
			return "", nil, closeErr
		}
	}
	if len(body) == 0 {
		return "", nil, errors.New("generated slide missing")
	}
	if failures := checkNativeProbeSlide(body, nativeCompleteSourceProbe(probe)); len(failures) != 0 {
		return "", nil, fmt.Errorf("source incomplete: %v", failures)
	}
	return nativeGenerated, nil, nil
}

// Use the same complete-source contract for individual and assembled decks.
func nativeCompleteSourceProbe(probe nativeProbe) nativeProbe {
	complete := probe
	complete.ExpectedText = append([]string(nil), probe.ExpectedText...)
	for _, item := range probe.Slide.Content {
		switch item.Type {
		case generator.ContentText:
			complete.ExpectedText = append(complete.ExpectedText, item.Value.(string))
		case generator.ContentBullets:
			for _, text := range item.Value.([]string) {
				complete.ExpectedText = append(complete.ExpectedText, strings.TrimSpace(text))
			}
		case generator.ContentTable:
			table := item.Value.(*types.TableSpec)
			complete.ExpectedText = append(complete.ExpectedText, table.Headers...)
			for _, row := range table.Rows {
				for _, cell := range row {
					complete.ExpectedText = append(complete.ExpectedText, cell.Content)
				}
			}
		}
	}
	return complete
}

func nativeAcceptedProbeIndices(probes []nativeProbeEvidence) []int {
	var indices []int
	for i := range probes {
		probes[i].OutputSlideIndex = nil
		if probes[i].GenerationStatus != nativeGenerated || probes[i].Refusal != nil || len(probes[i].Failures) != 0 {
			continue
		}
		index := len(indices)
		probes[i].OutputSlideIndex = &index
		indices = append(indices, i)
	}
	return indices
}

func TestNativePublicationMixedLedgerMapping(t *testing.T) {
	probes := []nativeProbeEvidence{
		{GenerationStatus: nativeRefused, Refusal: &patterns.ValidationError{Code: patterns.ErrCodeTextTrimmed}},
		{GenerationStatus: nativeGenerated},
		{GenerationStatus: "generation_failed", Failures: []string{"failed"}},
		{GenerationStatus: nativeGenerated},
		{GenerationStatus: nativeGenerated, Failures: []string{"source missing"}},
	}
	indices := nativeAcceptedProbeIndices(probes)
	if len(indices) != 2 || indices[0] != 1 || indices[1] != 3 {
		t.Fatalf("wrong accepted identity mapping: %v", indices)
	}
	for i := range probes {
		if i == 1 || i == 3 {
			want := 0
			if i == 3 {
				want = 1
			}
			if probes[i].OutputSlideIndex == nil || *probes[i].OutputSlideIndex != want {
				t.Fatalf("wrong compact output index for original %d", i)
			}
		} else if probes[i].OutputSlideIndex != nil {
			t.Fatalf("refused/failed probe %d assigned an artifact", i)
		}
	}
	if probes[0].Refusal == nil || len(probes[2].Failures) != 1 || len(probes[4].Failures) != 1 {
		t.Fatal("original negative evidence was erased")
	}
}

func TestNativePublicationCompleteSourceRejectsMarkerOnly(t *testing.T) {
	probe := nativeProbe{ExpectedText: []string{"C1-B00"}, Slide: generator.SlideSpec{Content: []generator.ContentItem{
		{Type: generator.ContentBullets, Value: []string{"C1-B00 Required complete sentence"}},
	}}}
	before := append([]string(nil), probe.ExpectedText...)
	markerOnly := []byte(`<sld><t>C1-B00</t></sld>`)
	if failures := checkNativeProbeSlide(markerOnly, nativeCompleteSourceProbe(probe)); len(failures) == 0 {
		t.Fatal("marker-only evidence hid a shortened sentence")
	}
	complete := []byte(`<sld><t>C1-B00 Required complete sentence</t></sld>`)
	if failures := checkNativeProbeSlide(complete, nativeCompleteSourceProbe(probe)); len(failures) != 0 {
		t.Fatalf("complete sentence rejected: %v", failures)
	}
	if len(probe.ExpectedText) != len(before) || probe.ExpectedText[0] != before[0] {
		t.Fatal("complete-source check mutated original probe")
	}
}

func TestNativePublicationKeepsAcceptedAndRefusedEvidence(t *testing.T) {
	path := filepath.Join(testutil.TemplatesDir(), "abstract.pptx")
	rows := make([][]types.TableCell, 80)
	for i := range rows {
		rows[i] = []types.TableCell{{Content: fmt.Sprintf("REQUIRED-R%02d", i)}, {Content: "Owner"}, {Content: "Status"}}
	}
	for _, mode := range []string{"", "warn", "off", "strict"} {
		t.Run("mode="+mode, func(t *testing.T) {
			for _, adverse := range []bool{false, true} {
				probe := nativeProbe{ID: "unchanged-source", Slide: generator.SlideSpec{LayoutID: "slideLayout3"}}
				item := generator.ContentItem{PlaceholderID: "body", Type: generator.ContentText, Value: "Required complete source"}
				if adverse {
					item.Type, item.Value = generator.ContentTable, &types.TableSpec{Headers: []string{"Evidence", "Owner", "Status"}, Rows: rows}
				}
				probe.Slide.Content = []generator.ContentItem{item}
				status, loss, err := classifyNativePublication(t, path, probe, mode)
				if err != nil {
					t.Fatal(err)
				}
				if adverse {
					if status != nativeRefused || loss == nil || loss.Code != patterns.ErrCodeTableRowsTruncated || loss.Fix == nil {
						t.Fatalf("refused source not retained: %s %+v", status, loss)
					}
				} else if status != nativeGenerated || loss != nil {
					t.Fatalf("accepted source incorrectly withheld: %s %+v", status, loss)
				}
			}
		})
	}
}
