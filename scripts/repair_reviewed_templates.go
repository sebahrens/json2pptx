//go:build ignore

// Run with: go run scripts/repair_reviewed_templates.go
// Applies reviewed, narrowly scoped layout repairs. Unchanged ZIP entries and
// source preimages are preserved; unexpected sources fail before any mutation.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	for _, repair := range []func() error{
		func() error { return repairClosingRule("templates/modern-template.pptx") },
		func() error { return repairBusinessSubtitle("templates/business-template.pptx") },
		func() error { return repairModernSubtitle("templates/modern.pptx") },
		func() error { return repairModernSection("templates/modern-template.pptx") },
		func() error { return repairModernFooterAccent("templates/modern.pptx") },
		func() error { return repairModernBulletIndentation("templates/modern.pptx") },
	} {
		if err := repair(); err != nil {
			panic(err)
		}
	}
}

type reviewedPartRepair struct {
	old, replacement string
	guards           []string
}

// These local list styles override the master's nesting margins. Their parent
// and child text origins were identical despite correctly emitted paragraph
// levels. Keep hanging indents, glyphs, colors and typography unchanged.
func repairModernBulletIndentation(path string) error {
	known := map[string][2]string{
		"ppt/slideLayouts/slideLayout3.xml": {"044aa360163a84c079b2cb947a4b0867f9105f6afd8019f356c600fc05a0fcec", "6fd46f7550d00f099b8b4eae5eed6251343b42e10ba6f581d96065c9e7da161c"},
		"ppt/slideLayouts/slideLayout7.xml": {"0c8f36cdb9ca7620cd5fa4180611ae184faa15f1f6510743513d269c809f6a93", "34e8e22ba2bef917de01bfa694f9535dfe4f714d126dc6b63e6ac9df4f6c884c"},
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	repairs := map[string]reviewedPartRepair{}
	for _, entry := range z.File {
		hashes, selected := known[entry.Name]
		if !selected {
			continue
		}
		if _, seen := repairs[entry.Name]; seen {
			return fmt.Errorf("duplicate reviewed part %s", entry.Name)
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return err
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		if sum != hashes[0] && sum != hashes[1] {
			return fmt.Errorf("%s differs from reviewed bullet styles; refusing to patch", entry.Name)
		}
		before, after := string(data), string(data)
		for level := 2; level <= 5; level++ {
			old := fmt.Sprintf(`<a:lvl%dpPr marL="228600" indent="-228600">`, level)
			updated := fmt.Sprintf(`<a:lvl%dpPr marL="%d" indent="-228600">`, level, level*228600)
			before = strings.ReplaceAll(before, updated, old)
			after = strings.ReplaceAll(after, old, updated)
		}
		if fmt.Sprintf("%x", sha256.Sum256([]byte(before))) != hashes[0] || fmt.Sprintf("%x", sha256.Sum256([]byte(after))) != hashes[1] {
			return fmt.Errorf("%s bullet margin transformation differs from reviewed result", entry.Name)
		}
		repairs[entry.Name] = reviewedPartRepair{old: before, replacement: after}
	}
	if len(repairs) != len(known) {
		return fmt.Errorf("reviewed bullet layout missing")
	}
	if err := z.Close(); err != nil {
		return err
	}
	return repairReviewedParts(path, "modern-bullet-indentation-before.pptx", repairs)
}

// The original decorative rectangle is flush with the canvas bottom, not an
// image crop. Keep its exact gradient/style as a slim, full-width footer band
// rather than an isolated tall block; text and table regions are untouched.
func repairModernFooterAccent(path string) error {
	return repairReviewedParts(path, "modern-footer-accent-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout3.xml": {
			old:         `<a:off x="5291586" y="6303963"/><a:ext cx="4287186" cy="554037"/>`,
			replacement: `<a:off x="0" y="6781800"/><a:ext cx="12192000" cy="76200"/>`,
			guards:      []string{`name="Rectangle 7"`, `<a:gradFill flip="none" rotWithShape="1">`, `<a:schemeClr val="accent5"/>`, `<a:tileRect r="-100000" b="-100000"/>`},
		},
	})
}

// The original oversized, implicitly centered number frame overlaps the
// bottom-anchored title. Give each text role its own region, retaining title
// bottom edge, fonts, colors, artwork and a 0.5cm inter-frame clearance.
func repairModernSection(path string) error {
	const part = "ppt/slideLayouts/slideLayout2.xml"
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	var source []byte
	for _, entry := range r.File {
		if entry.Name != part {
			continue
		}
		if source != nil {
			_ = r.Close()
			return fmt.Errorf("duplicate reviewed part %s", part)
		}
		f, err := entry.Open()
		if err != nil {
			_ = r.Close()
			return err
		}
		source, err = io.ReadAll(f)
		closeErr := f.Close()
		if err != nil {
			_ = r.Close()
			return fmt.Errorf("read section layout: %w", err)
		}
		if closeErr != nil {
			_ = r.Close()
			return fmt.Errorf("close section layout: %w", closeErr)
		}
	}
	if err := r.Close(); err != nil {
		return err
	}
	if source == nil {
		return fmt.Errorf("reviewed part %s missing", part)
	}
	substitutions := [][2]string{
		{`<a:off x="1450428" y="990601"/><a:ext cx="9145991" cy="3630384"/>`, `<a:off x="1450428" y="2352408"/><a:ext cx="9145991" cy="2268577"/>`},
		{`<a:off x="7886700" y="572408"/><a:ext cx="3182938" cy="3417887"/>`, `<a:off x="7886700" y="572408"/><a:ext cx="3182938" cy="1600000"/>`},
		{`<p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr marL="11113" indent="-11113">`, `<p:txBody><a:bodyPr anchor="t"/><a:lstStyle><a:lvl1pPr marL="11113" indent="-11113">`},
	}
	original, repaired := true, true
	for _, pair := range substitutions {
		original = original && bytes.Count(source, []byte(pair[0])) == 1 && !bytes.Contains(source, []byte(pair[1]))
		repaired = repaired && bytes.Count(source, []byte(pair[1])) == 1 && !bytes.Contains(source, []byte(pair[0]))
	}
	guards := []string{`name="title"`, `name="Section Number"`, `<a:defRPr sz="6500">`, `<a:defRPr sz="9600">`, `<a:bodyPr anchor="b">`}
	for _, guard := range guards {
		if !bytes.Contains(source, []byte(guard)) {
			return fmt.Errorf("section layout missing reviewed marker %q", guard)
		}
	}
	if repaired {
		return nil
	}
	if !original {
		return fmt.Errorf("section layout differs from reviewed source; refusing partial or unexpected repair")
	}
	updated := append([]byte(nil), source...)
	for _, pair := range substitutions {
		updated = bytes.Replace(updated, []byte(pair[0]), []byte(pair[1]), 1)
	}
	return repairReviewedParts(path, "modern-template-section-before.pptx", map[string]reviewedPartRepair{
		part: {old: string(source), replacement: string(updated), guards: guards},
	})
}

func repairClosingRule(path string) error {
	return repairReviewedParts(path, "modern-template-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout5.xml": {
			old:         `<a:off x="4985657" y="3300000"/><a:ext cx="0" cy="1700000"/>`,
			replacement: `<a:off x="4985657" y="3300000"/><a:ext cx="0" cy="320000"/>`,
			guards:      []string{`name="Straight Connector 10"`, `<a:off x="4830857" y="3800000"/>`},
		},
	})
}

func repairBusinessSubtitle(path string) error {
	return repairReviewedParts(path, "business-template-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout3.xml": {
			old: `<a:schemeClr val="accent1"/>`, replacement: `<a:schemeClr val="dk2"/>`,
			guards: []string{`name="subtitle"`, `<p:ph type="subTitle" idx="1"/>`, `<a:defRPr sz="1600" b="0" i="0">`},
		},
	})
}

func repairModernSubtitle(path string) error {
	parts := map[string]reviewedPartRepair{}
	for _, id := range []string{"2", "4"} {
		parts["ppt/slideLayouts/slideLayout"+id+".xml"] = reviewedPartRepair{
			old:         `<a:lumMod val="97000"/><a:lumOff val="3000"/>`,
			replacement: `<a:lumMod val="50000"/><a:lumOff val="0"/>`,
			guards:      []string{`name="subtitle"`, `<a:schemeClr val="accent2">`, `<a:gradFill`},
		}
	}
	return repairReviewedParts(path, "modern-before.pptx", parts)
}

func repairReviewedParts(path, backupName string, repairs map[string]reviewedPartRepair) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	updates := map[string][]byte{}
	seen := map[string]bool{}
	for _, entry := range reader.File {
		repair, needed := repairs[entry.Name]
		if !needed {
			continue
		}
		if seen[entry.Name] {
			return fmt.Errorf("duplicate reviewed part %s", entry.Name)
		}
		seen[entry.Name] = true
		r, err := entry.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return err
		}
		for _, guard := range repair.guards {
			if !bytes.Contains(body, []byte(guard)) {
				return fmt.Errorf("%s missing reviewed marker %q", entry.Name, guard)
			}
		}
		if bytes.Count(body, []byte(repair.replacement)) == 1 && !bytes.Contains(body, []byte(repair.old)) {
			continue // Already repaired, without rewriting this part.
		}
		if bytes.Count(body, []byte(repair.old)) != 1 {
			return fmt.Errorf("%s differs from reviewed source; refusing to patch", entry.Name)
		}
		updates[entry.Name] = bytes.Replace(body, []byte(repair.old), []byte(repair.replacement), 1)
	}
	for part := range repairs {
		if !seen[part] {
			return fmt.Errorf("reviewed part %s missing", part)
		}
	}
	if len(updates) == 0 {
		return nil
	}
	backupDir := "output/template-repair-20260926"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}
	backupPath := filepath.Join(backupDir, backupName)
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("preserve template preimage: %w", err)
	}
	_, writeErr := backup.Write(source)
	closeErr := backup.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reviewed-template-*.pptx")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	w := zip.NewWriter(tmp)
	for _, entry := range reader.File {
		body, changed := updates[entry.Name]
		if !changed {
			if err := w.Copy(entry); err != nil {
				return err
			}
			continue
		}
		dst, err := w.CreateHeader(&entry.FileHeader)
		if err != nil {
			return err
		}
		if _, err := dst.Write(body); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), info.Mode()); err != nil {
		return err
	}
	if err := reader.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
