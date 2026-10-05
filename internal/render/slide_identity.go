package render

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Stable slide image identity (go-slide-creator-6ffgv).
//
// A slide's content_hash is the pixel hash of its rendered PNG, and the render
// cache was keyed by the PPTX file hash alone. So every revision of a deck was
// converted afresh — and LibreOffice does not convert the same slide the same
// way every time. Measured on the e-revise journey deck: its source line is
// italic in a font with no italic face, and across six conversions of one
// byte-identical PPTX LibreOffice substituted LiberationSans-Italic five times
// and LiberationSerif-Italic once. At the default 50 dpi the two differ by a
// few hundred pixels in that one line, so a notes-only edit "changed" the
// hash of five slides nobody had touched.
//
// The identity of a slide image therefore follows the slide's visible
// content: a digest of the slide part and everything it renders with (layout,
// master, theme, media, charts), excluding what never shows (speaker notes,
// comments, the slide's position in the deck unless it prints a slide-number
// field). The first image rendered for a digest is kept, and a later
// conversion of a slide with the same digest returns that image instead of
// its own — in any revision of any deck.

// VisibleSlideKeys returns, for each slide of a PPTX in presentation order, a
// digest of everything that determines how the slide renders.
func VisibleSlideKeys(pptxPath string) ([]string, error) {
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()

	pkg := &pptxParts{files: make(map[string]*zip.File, len(zr.File)), digests: map[string]string{}, visiting: map[string]bool{}}
	for _, f := range zr.File {
		pkg.files[f.Name] = f
	}

	const presentationPart = "ppt/presentation.xml"
	presentation, err := pkg.read(presentationPart)
	if err != nil {
		return nil, err
	}
	rels, err := pkg.relationships(presentationPart)
	if err != nil {
		return nil, err
	}
	targets := make(map[string]string, len(rels))
	for _, r := range rels {
		targets[r.ID] = r.Target
	}
	// Deck-wide settings every slide renders with: slide size, default text
	// styles. The slide list itself is positional, not visual.
	deckLevel := sha256.Sum256(slideListElements.ReplaceAll(presentation, nil))

	ids := slideIDRefs.FindAllSubmatch(presentation, -1)
	keys := make([]string, 0, len(ids))
	for i, m := range ids {
		target, ok := targets[string(m[1])]
		if !ok {
			return nil, fmt.Errorf("presentation.xml names slide relationship %q, which does not exist", m[1])
		}
		part := resolvePartTarget(presentationPart, target)
		digest, err := pkg.digest(part)
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		h.Write(deckLevel[:])
		h.Write([]byte(digest))
		// A slide-number field renders the slide's position, so two slides
		// that differ only in position look different when they print it.
		if content, err := pkg.read(part); err == nil && bytes.Contains(content, []byte(`type="slidenum"`)) {
			fmt.Fprintf(h, "position:%d", i)
		}
		keys = append(keys, hex.EncodeToString(h.Sum(nil)))
	}
	return keys, nil
}

var (
	// slideListElements are the presentation.xml lists of slides, notes and
	// handout masters: adding a slide or the first speaker note changes them
	// without changing how any existing slide renders.
	slideListElements = regexp.MustCompile(`(?s)<p:(sldIdLst|notesMasterIdLst|handoutMasterIdLst)\b.*?</p:(sldIdLst|notesMasterIdLst|handoutMasterIdLst)>`)
	slideIDRefs       = regexp.MustCompile(`<p:sldId\b[^>]*\br:id="([^"]+)"`)
)

// pptxParts digests the parts of an OPC package, following relationships.
type pptxParts struct {
	files    map[string]*zip.File
	digests  map[string]string
	visiting map[string]bool
}

type partRelationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

func (p *pptxParts) read(name string) ([]byte, error) {
	f, ok := p.files[name]
	if !ok {
		return nil, fmt.Errorf("pptx part %q is missing", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// relationships returns a part's relationships, sorted by id. A part without a
// .rels file has none.
func (p *pptxParts) relationships(part string) ([]partRelationship, error) {
	relsName := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	if _, ok := p.files[relsName]; !ok {
		return nil, nil
	}
	data, err := p.read(relsName)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Relationships []partRelationship `xml:"Relationship"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", relsName, err)
	}
	sort.Slice(doc.Relationships, func(i, j int) bool { return doc.Relationships[i].ID < doc.Relationships[j].ID })
	return doc.Relationships, nil
}

// invisibleRelationship reports relationships whose target never shows on a
// slide that renders with the part that holds them.
//
// A master's list of its layouts is one: a slide renders with its own layout
// and that layout's master, not with the master's other layouts. Following
// the list also closed a cycle (layout to master and back), and a layout
// first reached through its master was digested without the master or the
// theme, so a slide on it had one key on every template
// (go-slide-creator-o477e).
func invisibleRelationship(part, relType string) bool {
	for _, suffix := range []string{"/notesSlide", "/comments", "/commentAuthors", "/slide", "/tags"} {
		if strings.HasSuffix(relType, suffix) {
			return true
		}
	}
	return strings.HasSuffix(relType, "/slideLayout") && strings.HasPrefix(part, "ppt/slideMasters/")
}

// digest returns the digest of a part and of everything it renders with.
func (p *pptxParts) digest(part string) (string, error) {
	d, _, err := p.digestPart(part)
	return d, err
}

// digestPart is digest, and reports whether the digest stands for the part
// wherever it is reached from. A digest computed inside a relationship cycle
// names the part it closed on instead of that part's content, so it is not
// kept for a later reference from outside the cycle.
func (p *pptxParts) digestPart(part string) (digest string, whole bool, err error) {
	if d, ok := p.digests[part]; ok {
		return d, true, nil
	}
	if p.visiting[part] {
		return "cycle:" + part, false, nil
	}
	p.visiting[part] = true
	defer delete(p.visiting, part)
	whole = true

	content, err := p.read(part)
	if err != nil {
		return "", false, err
	}
	rels, err := p.relationships(part)
	if err != nil {
		return "", false, err
	}
	h := sha256.New()
	h.Write(content)
	for _, r := range rels {
		if invisibleRelationship(part, r.Type) {
			continue
		}
		fmt.Fprintf(h, "\x00%s\x00%s\x00", r.ID, r.Type)
		if r.TargetMode == "External" {
			h.Write([]byte(r.Target))
			continue
		}
		target := resolvePartTarget(part, r.Target)
		if _, ok := p.files[target]; !ok {
			h.Write([]byte("missing:" + target))
			continue
		}
		d, targetWhole, err := p.digestPart(target)
		if err != nil {
			return "", false, err
		}
		whole = whole && targetWhole
		h.Write([]byte(d))
	}
	digest = hex.EncodeToString(h.Sum(nil))
	if whole {
		p.digests[part] = digest
	}
	return digest, whole, nil
}

// resolvePartTarget resolves a relationship target against its source part.
func resolvePartTarget(source, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	return path.Clean(path.Join(path.Dir(source), target))
}

// visibleSlideStorePath is where the first image rendered for a visible-slide
// key at a density lives. It is a single file directly under the cache
// directory, so the sweep ages and evicts it on its own.
func visibleSlideStorePath(key string, density int) string {
	return filepath.Join(cacheDir(), fmt.Sprintf("visible-%s-d%d.png", key, density))
}

// stabilizeSlidePNGs makes freshly converted slide images agree with what was
// rendered before for the same visible content: a slide whose key already has
// a stored image at this density gets that image (written over the fresh
// file), and a slide seen for the first time has its image stored. force
// re-renders, so it replaces the stored images instead of reusing them.
//
// It is best-effort: a deck whose slides cannot be keyed, or a store that
// cannot be read or written, leaves the fresh images as they are.
func stabilizeSlidePNGs(pptxPath string, pngs []string, density int, force bool) {
	keys, err := VisibleSlideKeys(pptxPath)
	if err != nil || len(keys) != len(pngs) {
		return
	}
	if err := os.MkdirAll(cacheDir(), 0755); err != nil {
		return
	}
	now := time.Now()
	for i, png := range pngs {
		stored := visibleSlideStorePath(keys[i], density)
		if !force {
			if data, err := os.ReadFile(stored); err == nil && len(data) > 0 { //nolint:gosec // path is internal (cache dir + digest)
				if os.WriteFile(png, data, 0644) == nil { //nolint:gosec // png is the renderer's own temp file
					// Still in use: keep it from ageing out of the cache.
					_ = os.Chtimes(stored, now, now)
				}
				continue
			}
		}
		data, err := os.ReadFile(png) //nolint:gosec // png is the renderer's own temp file
		if err != nil || len(data) == 0 {
			continue
		}
		tmp, err := os.CreateTemp(cacheDir(), ".tmp-visible-")
		if err != nil {
			continue
		}
		_, werr := tmp.Write(data)
		cerr := tmp.Close()
		if werr != nil || cerr != nil || os.Rename(tmp.Name(), stored) != nil {
			_ = os.Remove(tmp.Name())
		}
	}
}
