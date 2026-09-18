package main

// Portability fixture templates (go-slide-creator-94sk).
//
// The bundled templates are all single-master 16:9, so geometry code that
// quietly assumes a 12192000x6858000 canvas, one master, or empty side margins
// passes every bundled-template test. These variants take a generated theme
// and rewrite its package into the shapes real corporate templates come in:
//
//	4x3         9144000x6858000 canvas, all master/layout geometry scaled
//	21x9        16002000x6858000 ultrawide canvas, geometry scaled
//	two-master  a second slide master (own theme + 7 layouts) whose body column
//	            is inset and whose footer row sits higher than master 1's
//	side-logo   a logo picture on the master in the right-hand margin, level
//	            with the lower content / takeaway band
//
// Usage:
//
//	go run ./cmd/mktemplate -portability-fixtures tests/quality/fixtures/portability/templates
//	go run ./cmd/mktemplate -name midnight-blue -variant 4x3 -out /tmp/4x3.pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// portabilityVariants lists the fixture variants in generation order.
var portabilityVariants = []string{"4x3", "21x9", "two-master", "side-logo"}

// portabilityBase is the theme every fixture is derived from.
const portabilityBase = "midnight-blue"

// Side-logo geometry (16:9 canvas): a portrait logo in the right margin,
// right of the body column (which ends at x=11353800) and level with the
// lower content area where the takeaway band sits.
const (
	sideLogoX  = 11430000
	sideLogoY  = 4572000
	sideLogoCX = 640080
	sideLogoCY = 1600200
)

// generatePortabilityFixtures writes one fixture per variant into dir as
// portability-<variant>.pptx.
func generatePortabilityFixtures(dir string) error {
	def, ok := findTemplateDef(portabilityBase)
	if !ok {
		return fmt.Errorf("base template %q not defined", portabilityBase)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, v := range portabilityVariants {
		out := filepath.Join(dir, "portability-"+v+".pptx")
		if err := generateVariant(def, v, out); err != nil {
			return fmt.Errorf("%s: %w", v, err)
		}
		fmt.Printf("Generated: %s\n", out)
	}
	return nil
}

func findTemplateDef(name string) (templateDef, bool) {
	for _, t := range templates {
		if t.Name == name {
			return t, true
		}
	}
	return templateDef{}, false
}

// generateVariant builds the base template and rewrites its package parts for
// the requested variant.
func generateVariant(def templateDef, variant, outPath string) error {
	def.DisplayName = def.DisplayName + " (portability " + variant + ")"
	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".mktemplate-*.pptx")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := generateTemplate(def, tmpPath); err != nil {
		return err
	}
	parts, order, err := readZipParts(tmpPath)
	if err != nil {
		return err
	}
	switch variant {
	case "4x3":
		resizeCanvas(parts, 9144000, 6858000)
	case "21x9":
		resizeCanvas(parts, 16002000, 6858000)
	case "two-master":
		order = addSecondMaster(parts, order)
	case "side-logo":
		order, err = addSideLogo(parts, order)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown variant %q (want one of %s)", variant, strings.Join(portabilityVariants, ", "))
	}
	return writeZipParts(outPath, parts, order)
}

func readZipParts(path string) (map[string][]byte, []string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = zr.Close() }()
	parts := make(map[string][]byte, len(zr.File))
	order := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, nil, err
		}
		parts[f.Name] = data
		order = append(order, f.Name)
	}
	return parts, order, nil
}

func writeZipParts(path string, parts map[string][]byte, order []string) error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: deterministicTime})
		if err != nil {
			return err
		}
		if _, err := w.Write(parts[name]); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644) //nolint:gosec // template fixture output
}

var (
	offRe = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/>`)
	extRe = regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"/>`)
)

// transformXfrms rewrites every <a:off>/<a:ext> pair in an XML part through
// the affine map x' = dx + x*kx, y' = dy + y*ky (extents scale only).
func transformXfrms(data []byte, kx, ky float64, dx, dy int64) []byte {
	scale := func(v string, k float64, d int64) string {
		n, _ := strconv.ParseInt(v, 10, 64)
		return strconv.FormatInt(d+int64(float64(n)*k+0.5), 10)
	}
	data = offRe.ReplaceAllFunc(data, func(m []byte) []byte {
		g := offRe.FindSubmatch(m)
		x, y := string(g[1]), string(g[2])
		if x == "0" && y == "0" && dx == 0 && dy == 0 {
			return m
		}
		return []byte(fmt.Sprintf(`<a:off x="%s" y="%s"/>`, scale(x, kx, dx), scale(y, ky, dy)))
	})
	return extRe.ReplaceAllFunc(data, func(m []byte) []byte {
		g := extRe.FindSubmatch(m)
		return []byte(fmt.Sprintf(`<a:ext cx="%s" cy="%s"/>`, scale(string(g[1]), kx, 0), scale(string(g[2]), ky, 0)))
	})
}

// resizeCanvas sets the slide size and scales all master/layout geometry.
func resizeCanvas(parts map[string][]byte, cx, cy int64) {
	kx, ky := float64(cx)/12192000, float64(cy)/6858000
	for name, data := range parts {
		if isMasterOrLayoutXML(name) {
			parts[name] = transformXfrms(data, kx, ky, 0, 0)
		}
	}
	parts["ppt/presentation.xml"] = bytes.Replace(parts["ppt/presentation.xml"],
		[]byte(`<p:sldSz cx="12192000" cy="6858000"/>`),
		[]byte(fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/>`, cx, cy)), 1)
}

func isMasterOrLayoutXML(name string) bool {
	return (strings.HasPrefix(name, "ppt/slideMasters/") || strings.HasPrefix(name, "ppt/slideLayouts/")) &&
		strings.HasSuffix(name, ".xml") && !strings.Contains(name, "/_rels/")
}

// addSecondMaster clones master 1 (+ theme and its 7 layouts) as master 2 with
// an inset body column (1in left margin, 80% width) and a footer row lifted
// ~7% up the slide, then wires it into the presentation.
func addSecondMaster(parts map[string][]byte, order []string) []string {
	const layoutCount = 7
	const kx, ky = 0.8, 0.93
	const dx = 1219200

	master := transformXfrms(parts["ppt/slideMasters/slideMaster1.xml"], kx, ky, dx, 0)
	// Unique layout IDs for master 2's sldLayoutIdLst (master 2 itself takes
	// 2147483656; master 1 uses ...48 and its layouts ...49-...55).
	for i := layoutCount; i >= 1; i-- {
		master = bytes.Replace(master,
			[]byte(fmt.Sprintf(`<p:sldLayoutId id="%d" r:id="rId%d"/>`, 2147483648+i, i)),
			[]byte(fmt.Sprintf(`<p:sldLayoutId id="%d" r:id="rId%d"/>`, 2147483656+i, i)), 1)
	}
	parts["ppt/slideMasters/slideMaster2.xml"] = master
	masterRels := string(parts["ppt/slideMasters/_rels/slideMaster1.xml.rels"])
	for i := layoutCount; i >= 1; i-- {
		masterRels = strings.Replace(masterRels, fmt.Sprintf("slideLayout%d.xml", i), fmt.Sprintf("slideLayout%d.xml", i+layoutCount), 1)
	}
	masterRels = strings.Replace(masterRels, "../theme/theme1.xml", "../theme/theme2.xml", 1)
	parts["ppt/slideMasters/_rels/slideMaster2.xml.rels"] = []byte(masterRels)
	parts["ppt/theme/theme2.xml"] = parts["ppt/theme/theme1.xml"]
	added := []string{"ppt/slideMasters/slideMaster2.xml", "ppt/slideMasters/_rels/slideMaster2.xml.rels", "ppt/theme/theme2.xml"}

	var overrides strings.Builder
	overrides.WriteString(`<Override PartName="/ppt/slideMasters/slideMaster2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`)
	overrides.WriteString(`<Override PartName="/ppt/theme/theme2.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	for i := 1; i <= layoutCount; i++ {
		n := i + layoutCount
		layout := transformXfrms(parts[fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", i)], kx, ky, dx, 0)
		// Distinct layout names so the fixture's two One Content layouts are
		// addressable and reported separately.
		layout = regexp.MustCompile(`<p:cSld name="([^"]*)">`).ReplaceAll(layout, []byte(`<p:cSld name="$1 (Master 2)">`))
		parts[fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", n)] = layout
		parts[fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", n)] = bytes.Replace(
			parts[fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", i)],
			[]byte("slideMaster1.xml"), []byte("slideMaster2.xml"), 1)
		added = append(added, fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", n), fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", n))
		fmt.Fprintf(&overrides, `<Override PartName="/ppt/slideLayouts/slideLayout%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`, n)
	}
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"], []byte(`</Types>`), []byte(overrides.String()+`</Types>`), 1)
	parts["ppt/presentation.xml"] = bytes.Replace(parts["ppt/presentation.xml"],
		[]byte(`</p:sldMasterIdLst>`), []byte(`<p:sldMasterId id="2147483656" r:id="rId20"/></p:sldMasterIdLst>`), 1)
	parts["ppt/_rels/presentation.xml.rels"] = bytes.Replace(parts["ppt/_rels/presentation.xml.rels"],
		[]byte(`</Relationships>`),
		[]byte(`<Relationship Id="rId20" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster2.xml"/></Relationships>`), 1)
	return append(order, added...)
}

// addSideLogo places a logo picture on master 1 in the right-hand margin.
func addSideLogo(parts map[string][]byte, order []string) ([]string, error) {
	img := image.NewRGBA(image.Rect(0, 0, 28, 70))
	for y := 0; y < 70; y++ {
		for x := 0; x < 28; x++ {
			img.Set(x, y, color.RGBA{0x2E, 0x50, 0x90, 0xFF})
		}
	}
	var png1 bytes.Buffer
	if err := png.Encode(&png1, img); err != nil {
		return nil, err
	}
	parts["ppt/media/logo1.png"] = png1.Bytes()
	parts["ppt/slideMasters/_rels/slideMaster1.xml.rels"] = bytes.Replace(parts["ppt/slideMasters/_rels/slideMaster1.xml.rels"],
		[]byte(`</Relationships>`),
		[]byte(`<Relationship Id="rId20" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/logo1.png"/></Relationships>`), 1)
	pic := fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="20" name="Logo" descr="Company logo"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr userDrawn="1"/></p:nvPicPr>`+
		`<p:blipFill><a:blip r:embed="rId20"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		sideLogoX, sideLogoY, sideLogoCX, sideLogoCY)
	parts["ppt/slideMasters/slideMaster1.xml"] = bytes.Replace(parts["ppt/slideMasters/slideMaster1.xml"],
		[]byte(`</p:spTree>`), []byte(pic+`</p:spTree>`), 1)
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"],
		[]byte(`<Default Extension="json"`), []byte(`<Default Extension="png" ContentType="image/png"/><Default Extension="json"`), 1)
	return append(order, "ppt/media/logo1.png"), nil
}
