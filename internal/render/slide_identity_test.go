package render

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// identityDeck describes a minimal PPTX for the visible-slide key tests.
type identityDeck struct {
	slides     []string // slide XML bodies, in presentation order
	notes      map[int]string
	media      string // bytes of the one image slide 0 embeds
	layoutBody string
}

func (d identityDeck) write(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	add := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	const relNS = `xmlns="http://schemas.openxmlformats.org/package/2006/relationships"`
	const officeRel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

	var ids, presRels strings.Builder
	for i := range d.slides {
		fmt.Fprintf(&ids, `<p:sldId id="%d" r:id="rId%d"/>`, 256+i, 10+i)
		fmt.Fprintf(&presRels, `<Relationship Id="rId%d" Type="%s/slide" Target="slides/slide%d.xml"/>`, 10+i, officeRel, i+1)
	}
	notesMaster := ""
	if len(d.notes) > 0 {
		notesMaster = `<p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst>`
		fmt.Fprintf(&presRels, `<Relationship Id="rId2" Type="%s/notesMaster" Target="notesMasters/notesMaster1.xml"/>`, officeRel)
		add("ppt/notesMasters/notesMaster1.xml", `<p:notesMaster/>`)
	}
	add("ppt/presentation.xml", `<p:presentation><p:sldMasterIdLst><p:sldMasterId r:id="rId1"/></p:sldMasterIdLst>`+
		notesMaster+`<p:sldIdLst>`+ids.String()+`</p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/></p:presentation>`)
	add("ppt/_rels/presentation.xml.rels", `<Relationships `+relNS+`><Relationship Id="rId1" Type="`+officeRel+`/slideMaster" Target="slideMasters/slideMaster1.xml"/>`+presRels.String()+`</Relationships>`)

	// Master and layout reference each other, as in a real package.
	add("ppt/slideMasters/slideMaster1.xml", `<p:sldMaster/>`)
	add("ppt/slideMasters/_rels/slideMaster1.xml.rels", `<Relationships `+relNS+`><Relationship Id="rId1" Type="`+officeRel+`/slideLayout" Target="../slideLayouts/slideLayout1.xml"/><Relationship Id="rId2" Type="`+officeRel+`/theme" Target="../theme/theme1.xml"/></Relationships>`)
	add("ppt/theme/theme1.xml", `<a:theme/>`)
	add("ppt/slideLayouts/slideLayout1.xml", `<p:sldLayout>`+d.layoutBody+`</p:sldLayout>`)
	add("ppt/slideLayouts/_rels/slideLayout1.xml.rels", `<Relationships `+relNS+`><Relationship Id="rId1" Type="`+officeRel+`/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`)
	add("ppt/media/image1.png", d.media)

	for i, body := range d.slides {
		add(fmt.Sprintf("ppt/slides/slide%d.xml", i+1), `<p:sld>`+body+`</p:sld>`)
		rels := `<Relationship Id="rId1" Type="` + officeRel + `/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`
		if i == 0 {
			rels += `<Relationship Id="rId2" Type="` + officeRel + `/image" Target="../media/image1.png"/>`
		}
		if note, ok := d.notes[i]; ok {
			rels += fmt.Sprintf(`<Relationship Id="rId9" Type="%s/notesSlide" Target="../notesSlides/notesSlide%d.xml"/>`, officeRel, i+1)
			add(fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", i+1), `<p:notes>`+note+`</p:notes>`)
		}
		add(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1), `<Relationships `+relNS+`>`+rels+`</Relationships>`)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func identityKeys(t *testing.T, d identityDeck) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deck.pptx")
	d.write(t, path)
	keys, err := VisibleSlideKeys(path)
	if err != nil {
		t.Fatalf("VisibleSlideKeys: %v", err)
	}
	if len(keys) != len(d.slides) {
		t.Fatalf("got %d keys for %d slides", len(keys), len(d.slides))
	}
	return keys
}

// go-slide-creator-6ffgv: a slide's identity key covers what renders and
// nothing else, so a notes-only revision keeps every slide's key.
func TestVisibleSlideKeys(t *testing.T) {
	base := identityDeck{slides: []string{"<title/>", "<chart/>", "<closing/>"}, media: "PNG-A"}
	want := identityKeys(t, base)
	if want[0] == want[1] || want[1] == want[2] {
		t.Fatalf("different slides share a key: %v", want)
	}

	same := func(name string, d identityDeck, slides ...int) {
		t.Helper()
		got := identityKeys(t, d)
		for _, i := range slides {
			if got[i] != want[i] {
				t.Errorf("%s: slide %d key changed although it renders the same", name, i)
			}
		}
	}
	differs := func(name string, d identityDeck, slide int) {
		t.Helper()
		if got := identityKeys(t, d); got[slide] == want[slide] {
			t.Errorf("%s: slide %d key did not change although it renders differently", name, slide)
		}
	}

	withNotes := base
	withNotes.notes = map[int]string{0: "Welcome.", 1: "Stress the trend.", 2: "Thanks."}
	same("speaker notes added", withNotes, 0, 1, 2)

	appended := base
	appended.slides = append(append([]string{}, base.slides...), "<appendix/>")
	appended.notes = map[int]string{1: "note"}
	same("slide appended", appended, 0, 1, 2)

	edited := base
	edited.slides = []string{"<title/>", "<chart edited=\"1\"/>", "<closing/>"}
	differs("slide content edited", edited, 1)
	same("neighbour of an edited slide", edited, 0, 2)

	image := base
	image.media = "PNG-B"
	differs("embedded image replaced", image, 0)
	same("slide without the image", image, 1, 2)

	layout := base
	layout.layoutBody = "<logo/>"
	differs("layout changed", layout, 2)

	// A printed slide number makes position part of what the slide shows.
	numbered := identityDeck{slides: []string{`<a/>`, `<a:fld type="slidenum"/>`, `<a:fld type="slidenum"/>`}, media: "PNG-A"}
	keys := identityKeys(t, numbered)
	if keys[1] == keys[2] {
		t.Error("two slides that print different slide numbers share a key")
	}
}

// The first image rendered for a visible slide is the one every later
// conversion of that slide returns — LibreOffice does not render a slide the
// same way every time (go-slide-creator-6ffgv).
func TestStabilizeSlidePNGsReusesFirstRender(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := t.TempDir()

	v7 := filepath.Join(dir, "v7.pptx")
	identityDeck{slides: []string{"<title/>", "<chart/>"}, media: "PNG-A"}.write(t, v7)
	// v8: speaker notes on both slides, the chart slide edited, a slide added.
	v8 := filepath.Join(dir, "v8.pptx")
	identityDeck{
		slides: []string{"<title/>", "<chart edited=\"1\"/>", "<appendix/>"},
		notes:  map[int]string{0: "Welcome.", 1: "Stress the trend."},
		media:  "PNG-A",
	}.write(t, v8)

	fresh := func(name string, contents ...string) []string {
		t.Helper()
		out := make([]string, len(contents))
		for i, c := range contents {
			out[i] = filepath.Join(dir, fmt.Sprintf("%s-slide-%d.png", name, i))
			if err := os.WriteFile(out[i], []byte(c), 0644); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	first := fresh("v7", "title/sans", "chart/sans")
	stabilizeSlidePNGs(v7, first, 50, false)
	if read(first[0]) != "title/sans" || read(first[1]) != "chart/sans" {
		t.Fatal("a first render must keep its own images")
	}

	// The second conversion drew the same title slide with another fallback
	// font. It must come back as the image the first render produced.
	second := fresh("v8", "title/serif", "chart-edited/serif", "appendix/serif")
	stabilizeSlidePNGs(v8, second, 50, false)
	if got := read(second[0]); got != "title/sans" {
		t.Errorf("unchanged slide rendered as %q, want the first render's image", got)
	}
	if got := read(second[1]); got != "chart-edited/serif" {
		t.Errorf("edited slide rendered as %q, want its own fresh image", got)
	}
	if got := read(second[2]); got != "appendix/serif" {
		t.Errorf("new slide rendered as %q, want its own fresh image", got)
	}

	// Another density is another image.
	other := fresh("v8-d110", "title/110", "chart-edited/110", "appendix/110")
	stabilizeSlidePNGs(v8, other, 110, false)
	if got := read(other[0]); got != "title/110" {
		t.Errorf("density 110 reused a density 50 image: %q", got)
	}

	// force re-renders: the fresh image wins and becomes the stored one.
	forced := fresh("v7-forced", "title/forced", "chart/forced")
	stabilizeSlidePNGs(v7, forced, 50, true)
	if got := read(forced[0]); got != "title/forced" {
		t.Errorf("force returned %q, want the fresh image", got)
	}
	after := fresh("v8-again", "title/x", "chart-edited/x", "appendix/x")
	stabilizeSlidePNGs(v8, after, 50, false)
	if got := read(after[0]); got != "title/forced" {
		t.Errorf("after a forced render the stored image is %q, want the forced one", got)
	}
}
