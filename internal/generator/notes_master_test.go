package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestGenerate_NotesSlidesRelateToNotesMaster is the regression test for
// go-slide-creator-s1uvj.28. ECMA-376 requires every notesSlide to carry a
// relationship to a notesMaster; notes slides were written with only the
// slide back-reference. Templates that ship a notes master (modern) must be
// related to it; templates without one (midnight-blue, forest-green,
// warm-coral) get a synthesized notes master with its own theme part,
// content-type overrides, a presentation relationship and a
// notesMasterIdLst entry.
func TestGenerate_NotesSlidesRelateToNotesMaster(t *testing.T) {
	for _, name := range []string{"midnight-blue.pptx", "modern.pptx", "warm-coral.pptx"} {
		t.Run(name, func(t *testing.T) {
			templatePath := filepath.Join("..", "..", "templates", name)
			if _, err := os.Stat(templatePath); err != nil {
				t.Skipf("template %s not found", name)
			}
			outputPath := filepath.Join(t.TempDir(), "out.pptx")
			req := GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            outputPath,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{
					{LayoutID: "slideLayout1", SpeakerNotes: "Talk track one.",
						Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "One"}}},
					{LayoutID: "slideLayout1", SpeakerNotes: "Talk track two.",
						Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Two"}}},
				},
			}
			if _, err := Generate(context.Background(), req); err != nil {
				t.Fatalf("Generate: %v", err)
			}

			files := readZipFiles(t, outputPath)
			masterPart := ""
			for i := 1; i <= 2; i++ {
				rels := parseRels(t, files, NotesSlideRelsPath(i))
				target := relTarget(rels, pptx.RelTypeNotesMaster)
				if target == "" {
					t.Fatalf("notesSlide%d has no notesMaster relationship", i)
				}
				part := path.Clean(path.Join("ppt/notesSlides", target))
				if _, ok := files[part]; !ok {
					t.Fatalf("notesSlide%d notesMaster target %s is missing", i, part)
				}
				masterPart = part
			}

			masterRels := parseRels(t, files, path.Join(path.Dir(masterPart), "_rels", path.Base(masterPart)+".rels"))
			themeTarget := relTarget(masterRels, "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme")
			if themeTarget == "" {
				t.Fatal("notes master has no theme relationship")
			}
			themePart := path.Clean(path.Join(path.Dir(masterPart), themeTarget))
			if _, ok := files[themePart]; !ok {
				t.Fatalf("notes master theme %s is missing", themePart)
			}

			presRels := parseRels(t, files, PathPresentationRels)
			var notesMasterRelID string
			for _, r := range presRels {
				if r.Type == pptx.RelTypeNotesMaster {
					notesMasterRelID = r.ID
					if path.Clean(path.Join("ppt", r.Target)) != masterPart {
						t.Errorf("presentation notesMaster rel targets %s, want %s", r.Target, masterPart)
					}
				}
			}
			if notesMasterRelID == "" {
				t.Fatal("presentation.xml.rels has no notesMaster relationship")
			}
			if !strings.Contains(string(files[PathPresentationXML]), `<p:notesMasterId r:id="`+notesMasterRelID+`"`) {
				t.Errorf("presentation.xml notesMasterIdLst does not reference %s", notesMasterRelID)
			}
			ct := string(files[PathContentTypes])
			if !strings.Contains(ct, `PartName="/`+masterPart+`"`) || !strings.Contains(ct, `PartName="/`+themePart+`"`) {
				t.Errorf("[Content_Types].xml lacks overrides for %s / %s", masterPart, themePart)
			}

			report, err := pptx.ValidateOutputFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range report.Blocking() {
				t.Errorf("blocking finding: %s", f.Error())
			}
		})
	}
}

type relEntry struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

func parseRels(t *testing.T, files map[string][]byte, name string) []relEntry {
	t.Helper()
	data, ok := files[name]
	if !ok {
		t.Fatalf("%s missing from output", name)
	}
	var rels struct {
		Rels []relEntry `xml:"Relationship"`
	}
	if err := xml.Unmarshal(data, &rels); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return rels.Rels
}

func relTarget(rels []relEntry, relType string) string {
	for _, r := range rels {
		if r.Type == relType {
			return r.Target
		}
	}
	return ""
}

func readZipFiles(t *testing.T, p string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := out[f.Name]; dup {
			t.Errorf("duplicate zip entry %s", f.Name)
		}
		out[f.Name] = data
	}
	return out
}
