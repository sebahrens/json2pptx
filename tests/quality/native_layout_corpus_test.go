package quality

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// These are renderer probes, not agent-authored benchmark ratings. Every native
// layout receives representative and dense text. Evidence profiles target only
// appropriate native body slots; pictures fill every native image placeholder.
type nativeProbe struct {
	ID                       string              `json:"id"`
	LayoutID                 string              `json:"layout_id"`
	LayoutName               string              `json:"layout_name"`
	Profile                  string              `json:"profile"`
	Slide                    generator.SlideSpec `json:"input"`
	ExpectedText             []string            `json:"expected_text"`
	ExpectedPictures         int                 `json:"expected_pictures"`
	ExpectedBackgroundSource string              `json:"expected_background_source,omitempty"`
	ExpectedTables           int                 `json:"expected_tables"`
	ExpectedDiagramLabels    []string            `json:"expected_diagram_labels,omitempty"`
	NotApplicable            []string            `json:"not_applicable,omitempty"`
}

type nativeProbeEvidence struct {
	nativeProbe
	AuthoredInput    json.RawMessage           `json:"-"`
	SlideIndex       int                       `json:"slide_index"`
	OutputSlideIndex *int                      `json:"output_slide_index,omitempty"`
	GenerationStatus string                    `json:"generation_status"`
	Refusal          *patterns.ValidationError `json:"refusal,omitempty"`
	PNGPath          string                    `json:"png_path,omitempty"`
	PNGHash          string                    `json:"png_sha256,omitempty"`
	Failures         []string                  `json:"failures,omitempty"`
}

// Generation can mutate nested diagram/table pointers. Freeze the authored
// JSON before either publication attempt, and retain the effective input too.
func (e nativeProbeEvidence) MarshalJSON() ([]byte, error) {
	if len(e.AuthoredInput) == 0 {
		return nil, fmt.Errorf("probe %s has no pre-generation input snapshot", e.ID)
	}
	type evidenceAlias nativeProbeEvidence
	return json.Marshal(struct {
		evidenceAlias
		Input          json.RawMessage     `json:"input"`
		EffectiveInput generator.SlideSpec `json:"effective_input"`
	}{evidenceAlias(e), e.AuthoredInput, e.Slide})
}

func newNativeProbeEvidence(page nativeProbe, index int) (nativeProbeEvidence, error) {
	authored, err := json.Marshal(page.Slide)
	if err != nil {
		return nativeProbeEvidence{}, fmt.Errorf("snapshot authored probe %s: %w", page.ID, err)
	}
	return nativeProbeEvidence{nativeProbe: page, AuthoredInput: authored, SlideIndex: index}, nil
}

type nativeTemplateEvidence struct {
	Name          string                      `json:"name"`
	Path          string                      `json:"source_path"`
	Hash          string                      `json:"source_sha256"`
	NativeLayouts []string                    `json:"native_layout_ids"`
	DeckPath      string                      `json:"deck_path"`
	Generation    *generator.GenerationResult `json:"generation,omitempty"`
	Failures      []string                    `json:"failures,omitempty"`
	Probes        []nativeProbeEvidence       `json:"probes"`
}

func nativeCorpusPaths(t *testing.T) []string {
	t.Helper()
	paths := testutil.TestTemplatePaths()
	if dir := os.Getenv("NATIVE_LAYOUT_TEMPLATE_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("NATIVE_LAYOUT_TEMPLATE_DIR must be absolute")
		}
		expected := map[string]bool{}
		for _, path := range paths {
			expected[filepath.Base(path)] = true
		}
		var err error
		paths, err = filepath.Glob(filepath.Join(dir, "*.pptx"))
		if err != nil || len(paths) == 0 {
			t.Fatalf("explicit template corpus unavailable: %v", err)
		}
		for _, path := range paths {
			name := filepath.Base(path)
			if !expected[name] {
				t.Fatalf("explicit corpus has unexpected template %s", name)
			}
			delete(expected, name)
		}
		if len(expected) != 0 {
			t.Fatalf("explicit corpus omits local templates: %v", expected)
		}
	}
	fixtures, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "tests", "quality", "fixtures", "portability", "templates", "*.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, fixtures...)
	sort.Strings(paths)
	seen := map[string]string{}
	for _, path := range paths {
		name := filepath.Base(path)
		if previous, exists := seen[name]; exists {
			t.Fatalf("native corpus output name collision: %s and %s", previous, path)
		}
		seen[name] = path
	}
	return paths
}

func nativeReferenceImage() string {
	// Optional supplemental source-aware passes never replace the default
	// required screenshot cases. The output directory must still be new, and
	// each probe records the actual image path and the manifest its byte hash.
	if source := os.Getenv("NATIVE_LAYOUT_IMAGE_SOURCE"); source != "" {
		return source
	}
	// Existing real application screenshot, not a newly invented decorative asset.
	return filepath.Join(testutil.RepoRoot(), "tests", "quality", "evidence", "connectors", "midnight-blue", "powerpoint-slide-4.png")
}

// Commit IDs alone cannot identify an uncommitted renderer repair. Fingerprint
// all Go sources in the actual module tree, including ignored archive candidates;
// templates and the harness are hashed separately. Parent Git indexes do not
// inventory newly added files inside an ignored candidate tree.
func nativeEngineSourceHash(t *testing.T) string {
	t.Helper()
	hash, err := nativeEngineTreeHash(testutil.RepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

// Include helper files too: hashing only the entrypoint misses changes to
// publication classification and source-completeness assertions.
func nativeHarnessSourceHash(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "tests", "quality", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash.Write([]byte(filepath.Base(path) + "\x00"))
		hash.Write(body)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func makeNativeProbes(layout types.LayoutMetadata, imagePath string) []nativeProbe {
	profiles := []string{"representative", "dense"}
	evidenceBody := false
	for _, ph := range layout.Placeholders {
		if nativeEvidenceSlot(layout, ph) {
			evidenceBody = true
		}
	}
	if evidenceBody {
		profiles = append(profiles, "table", "chart", "table-stress")
	}
	probes := make([]nativeProbe, 0, len(profiles))
	for _, profile := range profiles {
		p := nativeProbe{ID: layout.ID + "-" + profile, LayoutID: layout.ID, LayoutName: layout.Name, Profile: profile, Slide: generator.SlideSpec{LayoutID: layout.ID}}
		bodyNumber := 0
		for _, ph := range layout.Placeholders {
			item := generator.ContentItem{PlaceholderID: ph.ID}
			if template.IsDisclosurePlaceholder(ph) {
				// Synthetic source text, not an invented real legal claim.
				value := "Illustrative disclosure for review"
				item.Type, item.Value = generator.ContentText, value
				p.ExpectedText = append(p.ExpectedText, value)
			} else if ph.Role == types.PlaceholderRoleSectionNumber || types.IsAutoFilledPlaceholder(ph.ID) {
				item.Type, item.Value = generator.ContentText, "07"
				p.ExpectedText = append(p.ExpectedText, "07")
			} else {
				switch ph.Type {
				case types.PlaceholderTitle:
					value := "Quarterly operating review"
					if layout.CanonicalType == types.CanonicalLayoutSectionDivider {
						value = "Service performance"
					}
					if profile == "dense" {
						value = "Quarterly operating review and service delivery priorities"
					}
					item.Type, item.Value = generator.ContentText, value
					p.ExpectedText = append(p.ExpectedText, value)
				case types.PlaceholderSubtitle:
					value := "Illustrative test data for review"
					if profile == "dense" {
						value = "Internal review of service delivery, ownership and reporting requirements"
					}
					item.Type, item.Value = generator.ContentText, value
					p.ExpectedText = append(p.ExpectedText, value)
				case types.PlaceholderImage:
					// Preserve the complete source in every pass. Default cases use
					// a required screenshot; supplemental passes may use a photo.
					item.Type, item.Value = generator.ContentImage, generator.ImageContent{Path: imagePath, Alt: "Native image probe " + ph.ID, Fit: "contain"}
					p.ExpectedPictures++
				case types.PlaceholderBody, types.PlaceholderContent:
					if layout.CanonicalType == types.CanonicalLayoutSectionDivider {
						item.Type, item.Value = generator.ContentText, "Delivery overview"
						p.ExpectedText = append(p.ExpectedText, "Delivery overview")
						break
					}
					bodyNumber++
					prefix := fmt.Sprintf("C%d", bodyNumber)
					if nativeEvidenceSlot(layout, ph) && (profile == "table" || profile == "table-stress") {
						rows := 6
						if profile == "table-stress" {
							rows = 24
						}
						table := &types.TableSpec{Headers: []string{"Case", "Owner", "Days"}, Alt: "Illustrative service cases", Style: types.TableStyle{UseTableStyle: true, StyleID: "@template-default", Borders: "all"}}
						for row := 0; row < rows; row++ {
							marker := fmt.Sprintf("%s-R%02d", prefix, row)
							table.Rows = append(table.Rows, []types.TableCell{{Content: marker}, {Content: fmt.Sprintf("Team %d", row+1)}, {Content: fmt.Sprintf("%d", row+2)}})
							p.ExpectedText = append(p.ExpectedText, marker)
						}
						item.Type, item.Value = generator.ContentTable, table
						p.ExpectedTables++
					} else if nativeEvidenceSlot(layout, ph) && profile == "chart" {
						labels := []string{"Q1", "Q2", "Q3", "Q4"}
						metric := prefix + " cases"
						item.Type, item.Value = generator.ContentDiagram, &types.DiagramSpec{
							Type: "bar_chart", Title: metric, Alt: "Illustrative quarterly service volume",
							Data: map[string]any{"categories": labels, "y_label": "Cases (count)", "series": []any{map[string]any{"name": metric, "values": []float64{12, 18, 15, 24}}}},
							// Exact labels make numeric fidelity observable in this QA probe;
							// they are not a universal authoring requirement for every chart.
							Style: &types.DiagramStyle{ShowValues: true},
						}
						p.ExpectedPictures++
						p.ExpectedDiagramLabels = append(p.ExpectedDiagramLabels, labels...)
						p.ExpectedDiagramLabels = append(p.ExpectedDiagramLabels, metric, "Cases (count)", "12", "18", "15", "24")
					} else {
						count := 4
						if profile == "dense" {
							count = 10
						}
						var bullets []string
						for row := 0; row < count; row++ {
							marker := fmt.Sprintf("%s-B%02d", prefix, row)
							text := marker + " Service owner confirms the reporting deadline"
							if profile == "dense" {
								text += " and documents unresolved handoffs before the monthly close. Café teams review résumé details and regional delivery constraints."
							}
							if row == 1 {
								text = "\t" + text
							}
							bullets = append(bullets, text)
							p.ExpectedText = append(p.ExpectedText, strings.TrimSpace(text))
						}
						item.Type, item.Value = generator.ContentBullets, bullets
					}
				default:
					continue
				}
			}
			p.Slide.Content = append(p.Slide.Content, item)
		}
		if !evidenceBody {
			p.NotApplicable = append(p.NotApplicable, "No sufficiently sized native body slot for table/chart evidence; native layout is still tested by both text profiles")
		}
		if p.ExpectedPictures == 0 {
			p.NotApplicable = append(p.NotApplicable, "No native image placeholder exercised in this profile")
		}
		probes = append(probes, p)
	}
	return probes
}

func nativeEvidenceSlot(layout types.LayoutMetadata, ph types.PlaceholderInfo) bool {
	return layout.CanonicalType != types.CanonicalLayoutSectionDivider && ph.Role != types.PlaceholderRoleSectionNumber && !template.IsDisclosurePlaceholder(ph) && !types.IsAutoFilledPlaceholder(ph.ID) && (ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent) && ph.Bounds.Width >= 1371600 && ph.Bounds.Height >= 914400
}

// Content presence is independent of the renderer's fit warnings: a diagnostic
// must not make a dropped row count as retained. Images/charts are reviewed in
// PNGs; picture count only establishes insertion, not label/crop correctness.
func checkNativeProbeSlide(body []byte, probe nativeProbe) []string {
	var failures []string
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var texts []string
	pictures, tables := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []string{"invalid slide XML: " + err.Error()}
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "pic":
			pictures++
		case "tbl":
			tables++
		case "t":
			var text string
			if err := decoder.DecodeElement(&text, &start); err != nil {
				return []string{err.Error()}
			}
			texts = append(texts, text)
		}
	}
	joined := strings.Join(texts, "")
	for _, text := range probe.ExpectedText {
		if !strings.Contains(joined, text) {
			failures = append(failures, "missing source text: "+text)
		}
	}
	if pictures < probe.ExpectedPictures {
		failures = append(failures, fmt.Sprintf("pictures=%d, expected >=%d", pictures, probe.ExpectedPictures))
	}
	if tables != probe.ExpectedTables {
		failures = append(failures, fmt.Sprintf("tables=%d, expected %d", tables, probe.ExpectedTables))
	}
	return failures
}

func TestNativeLayoutProbeInventory(t *testing.T) {
	imagePath := nativeReferenceImage()
	imageFile, err := os.Open(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = png.Decode(imageFile)
	_ = imageFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	paths := nativeCorpusPaths(t)
	if len(paths) < len(testutil.AllTestTemplateNames())+4 {
		t.Fatal("native portability templates excluded")
	}
	layoutCount, imageCount := 0, 0
	for _, path := range paths {
		r, err := template.OpenTemplate(path)
		if err != nil {
			t.Fatal(err)
		}
		layouts, err := template.ParseLayouts(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, layout := range layouts {
			layoutCount++
			probes := makeNativeProbes(layout, imagePath)
			if len(probes) < 2 || probes[0].Profile != "representative" || probes[1].Profile != "dense" {
				t.Fatalf("%s/%s missing text profiles", path, layout.ID)
			}
			for _, p := range probes {
				if seen[p.ID] {
					t.Fatal("duplicate native probe", p.ID)
				}
				seen[p.ID] = true
				if p.Slide.LayoutID != layout.ID {
					t.Fatal("native layout replaced by inferred role")
				}
			}
			for _, ph := range layout.Placeholders {
				if ph.Type == types.PlaceholderImage {
					imageCount++
					if probes[0].ExpectedPictures == 0 || probes[1].ExpectedPictures == 0 {
						t.Fatal("native image placeholder unexercised")
					}
				}
			}
		}
	}
	t.Logf("native inventory: %d templates, %d layouts, %d image placeholders", len(paths), layoutCount, imageCount)
}

func TestNativeProbeContentPresenceFailsOnOmittedRows(t *testing.T) {
	probe := nativeProbe{ExpectedText: []string{"C1-R00", "C1-R01"}, ExpectedTables: 1}
	body := []byte(`<p:sld xmlns:p="p" xmlns:a="a"><a:tbl><a:t>C1-R00</a:t></a:tbl></p:sld>`)
	failures := checkNativeProbeSlide(body, probe)
	if len(failures) != 1 || !strings.Contains(failures[0], "C1-R01") {
		t.Fatalf("omitted row accepted: %v", failures)
	}
}

func TestNativeProbeEvidenceProfilesPreserveDistinctColumns(t *testing.T) {
	layout := types.LayoutMetadata{ID: "slideLayout99", Placeholders: []types.PlaceholderInfo{
		{ID: "body", Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 2000000, Height: 2000000}},
		{ID: "body_2", Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 2000000, Height: 2000000}},
		{ID: "image", Type: types.PlaceholderImage},
	}}
	probes := makeNativeProbes(layout, "reference.png")
	if len(probes) != 5 {
		t.Fatalf("missing evidence profiles: %d", len(probes))
	}
	for _, p := range probes {
		if p.Slide.LayoutID != layout.ID || p.ExpectedPictures < 1 {
			t.Fatalf("native routing or image insertion lost: %+v", p)
		}
		switch p.Profile {
		case "representative", "dense":
			want := 8
			if p.Profile == "dense" {
				want = 20
			}
			if len(p.ExpectedText) != want || !strings.Contains(strings.Join(p.ExpectedText, " "), "C2-B00") {
				t.Fatalf("distinct complete bullet columns missing: %+v", p.ExpectedText)
			}
			var authored []string
			for _, item := range p.Slide.Content {
				if item.Type == generator.ContentBullets {
					for _, text := range item.Value.([]string) {
						authored = append(authored, strings.TrimSpace(text))
					}
				}
			}
			if !reflect.DeepEqual(p.ExpectedText, authored) {
				t.Fatalf("review expectations must contain every complete authored bullet: got %q, want %q", p.ExpectedText, authored)
			}
		case "table", "table-stress":
			want := 12
			if p.Profile == "table-stress" {
				want = 48
			}
			if p.ExpectedTables != 2 || len(p.ExpectedText) != want || !strings.Contains(strings.Join(p.ExpectedText, " "), "C2-R00") {
				t.Fatalf("distinct complete table rows missing: %+v", p)
			}
		case "chart":
			if p.ExpectedPictures != 3 || len(p.ExpectedDiagramLabels) != 20 {
				t.Fatalf("chart/image expectations lost: %+v", p)
			}
			charts := 0
			for _, item := range p.Slide.Content {
				if item.Type != generator.ContentDiagram {
					continue
				}
				charts++
				diagram := item.Value.(*types.DiagramSpec)
				metric := fmt.Sprintf("C%d cases", charts)
				if diagram.Title != metric || diagram.Data["y_label"] != "Cases (count)" || diagram.Style == nil || !diagram.Style.ShowValues {
					t.Fatalf("chart metric, units or exact value labels missing: %+v", diagram)
				}
				if got := diagram.Data["categories"].([]string); strings.Join(got, ",") != "Q1,Q2,Q3,Q4" {
					t.Fatalf("categories changed: %v", got)
				}
				series := diagram.Data["series"].([]any)[0].(map[string]any)
				values := series["values"].([]float64)
				if series["name"] != metric || fmt.Sprint(values) != "[12 18 15 24]" {
					t.Fatalf("source measure or values changed: %+v", series)
				}
			}
			if charts != 2 {
				t.Fatalf("distinct column charts lost: %d", charts)
			}
		default:
			t.Fatalf("unexpected profile %q", p.Profile)
		}
	}
}

// Opt-in because it retains several hundred full-size PNGs. Default test runs
// still verify the full source inventory and probe contracts, including p-style.
// Run with NATIVE_LAYOUT_CORPUS_OUT=/absolute/new/directory go test ./tests/quality
// -run TestNativeLayoutRenderedCorpus -count=1 -v -timeout=30m.
func TestNativeLayoutRenderedCorpus(t *testing.T) {
	out := os.Getenv("NATIVE_LAYOUT_CORPUS_OUT")
	if out == "" {
		t.Skip("set NATIVE_LAYOUT_CORPUS_OUT for retained rendered all-native-layout evidence")
	}
	if !filepath.IsAbs(out) {
		t.Fatal("output directory must be absolute")
	}
	if err := os.Mkdir(out, 0755); err != nil {
		t.Fatal("use a new output directory; historical evidence is never overwritten:", err)
	}
	if available, missing := render.DependencyStatus(); !available {
		t.Fatal("rendering required:", missing)
	}
	imagePath := nativeReferenceImage()
	mode := os.Getenv("NATIVE_LAYOUT_STRICT_FIT")
	if mode != "" && mode != "warn" && mode != "off" && mode != "strict" {
		t.Fatal("NATIVE_LAYOUT_STRICT_FIT must be omitted, warn, off or strict")
	}
	photoBackdrop := os.Getenv("NATIVE_LAYOUT_PHOTO_BACKDROP") == "1"
	tableContinuations := os.Getenv("NATIVE_LAYOUT_TABLE_CONTINUATIONS") == "1"
	bulletContinuations := os.Getenv("NATIVE_LAYOUT_BULLET_CONTINUATIONS") == "1"
	if photoBackdrop && os.Getenv("NATIVE_LAYOUT_IMAGE_SOURCE") == "" {
		t.Fatal("photo-only backdrop pass requires an explicitly supplied photographic source; never dim the default required screenshot")
	}
	if photoBackdrop {
		if err := validateNativePhotoSource(imagePath); err != nil {
			t.Fatal(err)
		}
	}
	imageHash, err := render.HashFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	engineHash := nativeEngineSourceHash(t)
	harnessHash := nativeHarnessSourceHash(t)
	renderer, err := render.OfficeCommand()
	if err != nil {
		t.Fatal(err)
	}
	renderer, err = exec.LookPath(renderer)
	if err != nil {
		t.Fatal(err)
	}
	rendererHash, err := render.HashFile(renderer)
	if err != nil {
		t.Fatal(err)
	}
	var evidence []nativeTemplateEvidence
	paths := nativeCorpusPaths(t)
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".pptx")
		e := nativeTemplateEvidence{Name: name, Path: path, DeckPath: filepath.Join(out, name+".pptx")}
		var err error
		e.Hash, err = render.HashFile(path)
		if err != nil {
			e.Failures = append(e.Failures, "source hash: "+err.Error())
			evidence = append(evidence, e)
			continue
		}
		r, err := template.OpenTemplate(path)
		if err != nil {
			e.Failures = append(e.Failures, err.Error())
			evidence = append(evidence, e)
			continue
		}
		layouts, err := template.ParseLayouts(r)
		_ = r.Close()
		if err != nil {
			e.Failures = append(e.Failures, err.Error())
			evidence = append(evidence, e)
			continue
		}
		var slides []generator.SlideSpec
		for _, layout := range layouts {
			e.NativeLayouts = append(e.NativeLayouts, layout.ID)
			for _, p := range makeNativeProbes(layout, imagePath) {
				if photoBackdrop && name == "modern" && (layout.ID == "slideLayout1" || layout.ID == "slideLayout6") {
					if err := authorNativePhotoBackdrop(&p); err != nil {
						t.Fatal(err)
					}
				}
				pages := []nativeProbe{p}
				if tableContinuations && p.Profile == "table-stress" {
					pages, err = authorNativeTableContinuations(p, 6)
					if err != nil {
						t.Fatal(err)
					}
				}
				if bulletContinuations && p.Profile == "dense" {
					pages, err = nativeBulletContinuations(p, 3)
					if err != nil {
						t.Fatal(err)
					}
				}
				for _, page := range pages {
					probe, snapshotErr := newNativeProbeEvidence(page, len(slides))
					if snapshotErr != nil {
						t.Fatal(snapshotErr)
					}
					e.Probes = append(e.Probes, probe)
					slides = append(slides, page.Slide)
				}
			}
		}
		// Preserve every original probe and its original index in the ledger.
		// Output indices refer only to independently source-complete probes.
		var accepted []generator.SlideSpec
		for i := range e.Probes {
			p := &e.Probes[i]
			status, loss, classifyErr := classifyNativePublication(t, path, p.nativeProbe, mode)
			p.GenerationStatus, p.Refusal = status, loss
			if classifyErr != nil {
				p.GenerationStatus = "generation_failed"
				p.Failures = append(p.Failures, classifyErr.Error())
				continue
			}
			if status == nativeRefused {
				continue
			}
		}
		acceptedIndices := nativeAcceptedProbeIndices(e.Probes)
		for _, i := range acceptedIndices {
			accepted = append(accepted, slides[i])
		}
		if len(accepted) == 0 {
			e.DeckPath = ""
			evidence = append(evidence, e)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		e.Generation, err = generator.Generate(ctx, generator.GenerationRequest{TemplatePath: path, OutputPath: e.DeckPath, StrictFit: mode, Slides: accepted, ExcludeTemplateSlides: true, AllowedImagePaths: []string{testutil.RepoRoot()}})
		cancel()
		if err != nil {
			e.Failures = append(e.Failures, "generation: "+err.Error())
			evidence = append(evidence, e)
			continue
		}
		if e.Generation.SlideCount != len(accepted) {
			e.Failures = append(e.Failures, "generation slide count differs from expected native probes")
		}
		for _, failure := range e.Generation.MediaFailures {
			if failure.SlideNum > 0 && failure.SlideNum <= len(acceptedIndices) {
				p := &e.Probes[acceptedIndices[failure.SlideNum-1]]
				p.Failures = append(p.Failures, "media: "+failure.Reason)
			} else {
				e.Failures = append(e.Failures, "unmapped media failure: "+failure.Reason)
			}
		}
		for _, failure := range e.Generation.ValidationErrors {
			e.Failures = append(e.Failures, "generation validation: "+failure.Error())
		}
		z, err := zip.OpenReader(e.DeckPath)
		if err != nil {
			e.Failures = append(e.Failures, "generated deck ZIP: "+err.Error())
			evidence = append(evidence, e)
			continue
		}
		parts := map[string]*zip.File{}
		for _, f := range z.File {
			parts[f.Name] = f
		}
		for i := range e.Probes {
			p := &e.Probes[i]
			if p.OutputSlideIndex == nil {
				continue
			}
			outputIndex := *p.OutputSlideIndex
			part := parts[fmt.Sprintf("ppt/slides/slide%d.xml", outputIndex+1)]
			if part == nil {
				p.Failures = append(p.Failures, "generated slide missing")
				continue
			}
			f, err := part.Open()
			if err != nil {
				p.Failures = append(p.Failures, err.Error())
				continue
			}
			body, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil {
				p.Failures = append(p.Failures, err.Error())
				continue
			}
			p.Failures = append(p.Failures, checkNativeProbeSlide(body, nativeCompleteSourceProbe(p.nativeProbe))...)
			if p.ExpectedBackgroundSource != "" {
				if err := checkNativeBackgroundSource(z, outputIndex+1, p.ExpectedBackgroundSource); err != nil {
					p.Failures = append(p.Failures, err.Error())
				}
			}
			rels := parts[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", outputIndex+1)]
			if rels == nil {
				p.Failures = append(p.Failures, "slide relationships missing")
			} else {
				f, err := rels.Open()
				if err != nil {
					p.Failures = append(p.Failures, err.Error())
				} else {
					body, err := io.ReadAll(f)
					_ = f.Close()
					if err != nil || !bytes.Contains(body, []byte("/"+p.LayoutID+".xml")) {
						p.Failures = append(p.Failures, "native layout relationship mismatch")
					}
				}
			}
		}
		_ = z.Close()
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
		deck, err := render.RenderDeckOptsContext(ctx, e.DeckPath, 96, 0, true)
		cancel()
		if err != nil {
			e.Failures = append(e.Failures, "render: "+err.Error())
		} else if deck.Truncated || len(deck.Slides) != len(acceptedIndices) {
			e.Failures = append(e.Failures, fmt.Sprintf("incomplete raster set: got %d, expected %d", len(deck.Slides), len(acceptedIndices)))
		} else {
			dir := filepath.Join(out, name)
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatal(err)
			}
			for i, img := range deck.Slides {
				p := &e.Probes[acceptedIndices[i]]
				var body []byte
				if img.Path != "" {
					body, err = os.ReadFile(img.Path)
				} else {
					body, err = base64.StdEncoding.DecodeString(img.PNG64)
				}
				if err != nil {
					p.Failures = append(p.Failures, err.Error())
					continue
				}
				if _, err = png.Decode(bytes.NewReader(body)); err != nil {
					p.Failures = append(p.Failures, "unreadable PNG: "+err.Error())
					continue
				}
				p.PNGPath = filepath.Join(dir, p.ID+".png")
				if err := os.WriteFile(p.PNGPath, body, 0644); err != nil {
					t.Fatal(err)
				}
				p.PNGHash, err = render.HashFile(p.PNGPath)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		evidence = append(evidence, e)
		t.Logf("%s: %d native layouts, %d retained probes", name, len(e.NativeLayouts), len(e.Probes))
	}
	commit, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	manifest := struct {
		SchemaVersion      int                      `json:"schema_version"`
		StrictFit          string                   `json:"strict_fit"`
		EngineCommit       string                   `json:"engine_commit"`
		EngineSourceHash   string                   `json:"engine_source_sha256"`
		HarnessHash        string                   `json:"harness_sha256"`
		ReferenceImageHash string                   `json:"reference_image_sha256"`
		Renderer           string                   `json:"renderer_entrypoint"`
		RendererHash       string                   `json:"renderer_entrypoint_sha256"`
		DPI                int                      `json:"dpi"`
		CreatedAt          string                   `json:"created_at_utc"`
		TemplateCount      int                      `json:"template_count"`
		Templates          []nativeTemplateEvidence `json:"templates"`
		Limits             []string                 `json:"limits"`
	}{
		SchemaVersion: 5, StrictFit: mode, EngineCommit: strings.TrimSpace(string(commit)),
		EngineSourceHash: engineHash, HarnessHash: harnessHash,
		ReferenceImageHash: imageHash, Renderer: renderer, RendererHash: rendererHash,
		DPI: 96, CreatedAt: time.Now().UTC().Format(time.RFC3339), TemplateCount: len(paths), Templates: evidence,
		Limits: []string{"Renderer probes, not new agent authoring or blind benchmark ratings", "Charts use the generator's current default rendering pipeline; picture insertion does not prove label readability or editability", "Only Western accented characters exercised; no CJK/RTL approval", "Generation failures retain explicit ledger entries; no visual score or approval for missing pages", "Independent visual inspection required; complete PNGs and marker presence are not visual approval"},
	}
	if tableContinuations {
		manifest.Limits = append(manifest.Limits, "Authoring alternative: native table-stress probes use coordinated six-row windows on all tables; original adverse evidence remains separate and is not approved")
	}
	if bulletContinuations {
		manifest.Limits = append(manifest.Limits, "Explicit production split_bullets repair: dense native bullet columns use coordinated three-item windows with intact nested groups; original unsplit dense evidence remains separate and is not approved")
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "manifest.json"), body, 0644); err != nil {
		t.Fatal(err)
	}
	if nativeEngineSourceHash(t) != engineHash {
		t.Error("engine source changed during render; evidence revision is not stable")
	}
	if nativeHarnessSourceHash(t) != harnessHash {
		t.Error("harness source changed during render; evidence revision is not stable")
	}
	for _, e := range evidence {
		for _, f := range e.Failures {
			t.Errorf("%s: %s", e.Name, f)
		}
		for _, p := range e.Probes {
			for _, f := range p.Failures {
				t.Errorf("%s/%s: %s", e.Name, p.ID, f)
			}
		}
	}
}
