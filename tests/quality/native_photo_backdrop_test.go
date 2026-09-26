package quality

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Explicit opt-in is not an image classifier. Reject our required evidence even
// when renamed, so this photographic experiment cannot silently dim that input.
func validateNativePhotoSource(path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read photographic source: %w", err)
	}
	required, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), "tests", "quality", "evidence", "connectors", "midnight-blue", "powerpoint-slide-4.png"))
	if err != nil {
		return fmt.Errorf("read required screenshot guard: %w", err)
	}
	if bytes.Equal(source, required) {
		return fmt.Errorf("photo-only backdrop must not dim the required screenshot, including renamed copies")
	}
	return nil
}

// This is a source-aware authoring alternative, not a native image-placeholder
// renderer repair. Original native-photo and required-screenshot cases remain
// immutable in separate evidence sets and still require their own review.
func authorNativePhotoBackdrop(probe *nativeProbe) error {
	var source string
	var text []generator.ContentItem
	for _, content := range probe.Slide.Content {
		if content.Type != generator.ContentImage {
			text = append(text, content)
			continue
		}
		image, ok := content.Value.(generator.ImageContent)
		if !ok || image.Path == "" || source != "" {
			return fmt.Errorf("photo backdrop requires exactly one explicit source image")
		}
		source = image.Path
	}
	if source == "" {
		return fmt.Errorf("photo backdrop source missing")
	}
	probe.Slide.Content = text
	probe.Slide.Background = &generator.BackgroundImage{Path: source, Fit: "cover", Overlay: &generator.BackgroundOverlay{Color: "dk1", Alpha: 0.6}}
	// A p:bg image is not a p:pic. Verify its resolved media bytes separately,
	// without changing native-picture expectations in the original probes.
	probe.ExpectedPictures = 0
	probe.ExpectedBackgroundSource = source
	probe.NotApplicable = append(probe.NotApplicable, "Photo-only authoring repair: image is a scrimmed slide background, not an authored native picture placeholder; original native-picture cases are retained separately")
	return nil
}

func TestNativePhotoBackdropPreservesExactTextAndSource(t *testing.T) {
	title := generator.ContentItem{PlaceholderID: "title", Type: generator.ContentText, Value: "Quarterly operating review and service delivery priorities"}
	subtitle := generator.ContentItem{PlaceholderID: "subtitle", Type: generator.ContentText, Value: "Internal review of service delivery, ownership and reporting requirements"}
	probe := nativeProbe{Slide: generator.SlideSpec{LayoutID: "slideLayout6", Content: []generator.ContentItem{
		title, {PlaceholderID: "image", Type: generator.ContentImage, Value: generator.ImageContent{Path: "original-photo.png", Fit: "contain"}}, subtitle,
	}}, ExpectedPictures: 1}
	if err := authorNativePhotoBackdrop(&probe); err != nil {
		t.Fatal(err)
	}
	if probe.Slide.LayoutID != "slideLayout6" || !reflect.DeepEqual(probe.Slide.Content, []generator.ContentItem{title, subtitle}) {
		t.Fatal("authoring alternative changed native layout or exact title/subtitle")
	}
	background := probe.Slide.Background
	if background == nil || background.Path != "original-photo.png" || background.Fit != "cover" || background.Overlay == nil || background.Overlay.Color != "dk1" || background.Overlay.Alpha != 0.6 || probe.ExpectedPictures != 0 || probe.ExpectedBackgroundSource != "original-photo.png" {
		t.Fatalf("source/overlay/picture expectation changed: %+v", probe)
	}
}

func checkNativeBackgroundSource(z *zip.ReadCloser, slide int, source string) error {
	read := func(name string) ([]byte, error) {
		f, err := z.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	body, err := read(fmt.Sprintf("ppt/slides/slide%d.xml", slide))
	if err != nil {
		return err
	}
	var document struct {
		Background struct {
			Properties struct {
				Fill struct {
					Blip struct {
						Embed string `xml:"embed,attr"`
					} `xml:"blip"`
				} `xml:"blipFill"`
			} `xml:"bgPr"`
		} `xml:"cSld>bg"`
	}
	if err := xml.Unmarshal(body, &document); err != nil {
		return err
	}
	id := document.Background.Properties.Fill.Blip.Embed
	if id == "" {
		return fmt.Errorf("expected photographic background image missing")
	}
	rels, err := read(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slide))
	if err != nil {
		return err
	}
	var relationships struct {
		Entries []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
			Type   string `xml:"Type,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(rels, &relationships); err != nil {
		return err
	}
	for _, rel := range relationships.Entries {
		if rel.ID != id {
			continue
		}
		if rel.Mode != "" || rel.Type != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" {
			return fmt.Errorf("background must reference embedded image media")
		}
		media, err := read(filepath.ToSlash(filepath.Clean(filepath.Join("ppt/slides", rel.Target))))
		if err != nil {
			return err
		}
		expected, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if !bytes.Equal(media, expected) {
			return fmt.Errorf("background source image bytes changed")
		}
		return nil
	}
	return fmt.Errorf("background image relationship missing")
}

func TestNativePhotoBackgroundPackageRequiresExactEmbeddedSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "original.png")
	if err := os.WriteFile(source, []byte("original-source-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	const slide = `<sld><cSld><bg><bgPr><blipFill><blip embed="photo"/></blipFill></bgPr></bg></cSld></sld>`
	const rel = `<Relationships><Relationship Id="photo" Target="../media/photo.png" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/></Relationships>`
	for _, tc := range []struct {
		name, body, relationships, media string
		wantError                        bool
	}{
		{"exact source", slide, rel, "original-source-bytes", false},
		{"missing background", `<sld><cSld/></sld>`, rel, "original-source-bytes", true},
		{"missing relationship", slide, `<Relationships/>`, "original-source-bytes", true},
		{"external relationship", slide, `<Relationships><Relationship Id="photo" Target="../media/photo.png" TargetMode="External"/></Relationships>`, "original-source-bytes", true},
		{"wrong relation type", slide, `<Relationships><Relationship Id="photo" Target="../media/photo.png" Type="layout"/></Relationships>`, "original-source-bytes", true},
		{"modified source", slide, rel, "modified", true},
		{"missing media", slide, rel, "", true},
		{"malformed XML", `<sld`, rel, "original-source-bytes", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			parts := map[string]string{"ppt/slides/slide1.xml": tc.body, "ppt/slides/_rels/slide1.xml.rels": tc.relationships}
			if tc.media != "" {
				parts["ppt/media/photo.png"] = tc.media
			}
			for name, body := range parts {
				part, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(part, body); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			deck := filepath.Join(t.TempDir(), "deck.pptx")
			if err := os.WriteFile(deck, archive.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(deck)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			if err := checkNativeBackgroundSource(z, 1, source); (err != nil) != tc.wantError {
				t.Fatalf("got %v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

func TestNativePhotoSourceRejectsRequiredScreenshotCopies(t *testing.T) {
	t.Setenv("NATIVE_LAYOUT_IMAGE_SOURCE", "")
	source := nativeReferenceImage()
	if err := validateNativePhotoSource(source); err == nil {
		t.Fatal("required screenshot accepted as photographic background")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "renamed-photo.png")
	if err := os.WriteFile(copyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateNativePhotoSource(copyPath); err == nil {
		t.Fatal("renamed screenshot bypassed guard")
	}
	if err := validateNativePhotoSource(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := os.WriteFile(copyPath, []byte("different-source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateNativePhotoSource(copyPath); err != nil {
		t.Fatal(err)
	}
}

func TestNativePhotoBackdropRejectsAmbiguousSourcesWithoutMutation(t *testing.T) {
	image := generator.ContentItem{Type: generator.ContentImage, Value: generator.ImageContent{Path: "photo.png"}}
	for _, content := range [][]generator.ContentItem{nil, {{Type: generator.ContentImage, Value: "invalid"}}, {image, image}} {
		probe := nativeProbe{Slide: generator.SlideSpec{Content: content}}
		before := probe
		if err := authorNativePhotoBackdrop(&probe); err == nil || !reflect.DeepEqual(probe, before) {
			t.Fatal("missing/invalid/multiple source should fail without mutating the probe")
		}
	}
}
