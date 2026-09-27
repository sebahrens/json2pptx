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
	if len(os.Args) > 1 {
		if len(os.Args) != 3 {
			panic("usage: repair_reviewed_templates [--pstyle-title-grouping|--blue-reusable-title path.pptx]")
		}
		var repair func(string) error
		switch os.Args[1] {
		case "--pstyle-title-grouping":
			repair = repairPStyleTitleAnchor
		case "--blue-reusable-title":
			repair = repairBlueReusableTitle
		case "--yellow-closing-grouping":
			repair = repairYellowClosingGrouping
		case "--pstyle-column-hierarchy":
			repair = repairPStyleColumnHierarchy
		case "--blue-child-leading":
			repair = repairBlueChildLeading
		case "--business-column-leading":
			repair = repairBusinessColumnLeading
		default:
			panic("unknown reviewed repair")
		}
		if err := repair(os.Args[2]); err != nil {
			panic(err)
		}
		return
	}
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

func repairYellowClosingGrouping(path string) error {
	return repairReviewedParts(path, "yellow-closing-grouping-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout8.xml": {
			old:               `<a:off x="1524000" y="2286000"/><a:ext cx="9144000" cy="1600200"/>`,
			replacement:       `<a:off x="397665" y="2286000"/><a:ext cx="11396670" cy="1600200"/>`,
			guards:            []string{`name="Closing"`, `name="title"`, `name="subtitle"`, `sz="6000"`, `sz="2400"`, `<a:bodyPr anchor="b"/>`},
			secondOld:         `<a:off x="1524000" y="4000500"/><a:ext cx="9144000" cy="800100"/>`,
			secondReplacement: `<a:off x="397665" y="4000500"/><a:ext cx="11396670" cy="800100"/>`,
		},
	})
}

func repairPStyleColumnHierarchy(path string) error {
	const old = `<a:lvl1pPr><a:spcAft><a:spcPts val="1200"/></a:spcAft><a:defRPr sz="1500"/></a:lvl1pPr><a:lvl2pPr><a:defRPr sz="1200"/></a:lvl2pPr><a:lvl3pPr><a:defRPr sz="1200"/></a:lvl3pPr>`
	// This template's ordinary bullets start at native level2, with children at
	// level3. Raising only lvl1 disables the generator's floor override while
	// leaving the visible root at12pt. Repair the actual root/child styles.
	const replacement = `<a:lvl1pPr><a:spcAft><a:spcPts val="1200"/></a:spcAft><a:defRPr sz="2000"/></a:lvl1pPr><a:lvl2pPr><a:defRPr sz="2000"/></a:lvl2pPr><a:lvl3pPr><a:defRPr sz="2000"/></a:lvl3pPr><a:lvl4pPr><a:defRPr sz="1800"/></a:lvl4pPr>`
	// Both reviewed columns must change together; guard the entire part so an
	// unexpected third occurrence or partially repaired column is not accepted.
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, e := range z.File {
		if e.Name != "ppt/slideLayouts/slideLayout5.xml" {
			continue
		}
		r, err := e.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return err
		}
		oldCount, newCount := bytes.Count(data, []byte(old)), bytes.Count(data, []byte(replacement))
		if !((oldCount == 2 && newCount == 0) || (oldCount == 0 && newCount == 2)) {
			return fmt.Errorf("unexpected reviewed column typography")
		}
		before := strings.ReplaceAll(string(data), replacement, old)
		after := strings.ReplaceAll(before, old, replacement)
		return repairReviewedParts(path, "p-style-column-hierarchy-before.pptx", map[string]reviewedPartRepair{
			e.Name: {old: before, replacement: after, guards: []string{`name="Two Content"`, `name="body"`, `name="body_2"`, `cx="5583528" cy="4000502"`}},
		})
	}
	return fmt.Errorf("reviewed column layout missing")
}

// Blue One Content bullets start at native level2 with children at level3,
// which inherited the master's 90% leading under 140% parents. Give the child
// levels the parent leading so hierarchy reads by size and indent, not crowding.
func repairBlueChildLeading(path string) error {
	return repairReviewedParts(path, "blue-child-leading-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout2.xml": {
			old:         `<a:lvl3pPr><a:defRPr sz="1600"/></a:lvl3pPr><a:lvl4pPr><a:defRPr sz="1600"/></a:lvl4pPr><a:lvl5pPr><a:defRPr sz="1400"/></a:lvl5pPr>`,
			replacement: `<a:lvl3pPr><a:lnSpc><a:spcPct val="140000"/></a:lnSpc><a:defRPr sz="1600"/></a:lvl3pPr><a:lvl4pPr><a:lnSpc><a:spcPct val="140000"/></a:lnSpc><a:defRPr sz="1600"/></a:lvl4pPr><a:lvl5pPr><a:lnSpc><a:spcPct val="140000"/></a:lnSpc><a:defRPr sz="1400"/></a:lvl5pPr>`,
			guards:      []string{`name="body"`, `<a:lvl2pPr><a:lnSpc><a:spcPct val="140000"/></a:lnSpc><a:defRPr sz="2000"/></a:lvl2pPr>`},
		},
	})
}

// Business Two Content columns set four-line paragraphs at 110%, which read
// cramped on dense continuations. Open both columns to 120% together.
func repairBusinessColumnLeading(path string) error {
	const old = `<a:lvl1pPr><a:lnSpc><a:spcPct val="110000"/></a:lnSpc><a:defRPr sz="1800"/></a:lvl1pPr><a:lvl2pPr><a:lnSpc><a:spcPct val="110000"/></a:lnSpc><a:defRPr sz="1600"/></a:lvl2pPr>`
	const replacement = `<a:lvl1pPr><a:lnSpc><a:spcPct val="120000"/></a:lnSpc><a:defRPr sz="1800"/></a:lvl1pPr><a:lvl2pPr><a:lnSpc><a:spcPct val="120000"/></a:lnSpc><a:defRPr sz="1600"/></a:lvl2pPr>`
	return repairWholePart(path, "business-column-leading-before.pptx", "ppt/slideLayouts/slideLayout5.xml", old, replacement, 2,
		[]string{`name="Two Content"`, `name="body"`, `name="body_2"`})
}

// repairWholePart rewrites every occurrence of old in one part, requiring the
// part to hold exactly want occurrences of either old or replacement (never a
// partial mix), so all sibling placeholders change together.
func repairWholePart(path, backup, part, old, replacement string, want int, guards []string) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, e := range z.File {
		if e.Name != part {
			continue
		}
		r, err := e.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return err
		}
		oldCount, newCount := bytes.Count(data, []byte(old)), bytes.Count(data, []byte(replacement))
		if !((oldCount == want && newCount == 0) || (oldCount == 0 && newCount == want)) {
			return fmt.Errorf("unexpected reviewed typography in %s", part)
		}
		before := strings.ReplaceAll(string(data), replacement, old)
		after := strings.ReplaceAll(before, old, replacement)
		return repairReviewedParts(path, backup, map[string]reviewedPartRepair{
			part: {old: before, replacement: after, guards: guards},
		})
	}
	return fmt.Errorf("reviewed layout %s missing", part)
}

// A reusable content heading needs stronger hierarchy than ordinary body text.
// Retain its all-caps tracked style, position and width; reserve two lines for
// long titles instead of making the larger type clip in the old shallow frame.
func repairBlueReusableTitle(path string) error {
	return repairReviewedParts(path, "blue-reusable-title-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout7.xml": {
			old:               `<a:defRPr sz="2000" cap="all" spc="300" baseline="0"/>`,
			replacement:       `<a:defRPr sz="2800" cap="all" spc="300" baseline="0"/>`,
			guards:            []string{`type="titleOnly"`, `name="Blank + Title"`, `name="title"`, `<a:bodyPr anchor="t"/>`, `<p:ph type="title"/>`},
			secondOld:         `<a:off x="841248" y="841248"/><a:ext cx="10479024" cy="557784"/>`,
			secondReplacement: `<a:off x="841248" y="841248"/><a:ext cx="10479024" cy="1016000"/>`,
		},
	})
}

type reviewedPartRepair struct {
	old, replacement             string
	guards                       []string
	secondOld, secondReplacement string
}

// Keep title typography, height and subtitle frame, but widen the title into
// unused canvas space and anchor it toward its related subtitle. Short titles
// must not create a large empty gap or strand their last word on a new line.
// This private-template repair is explicitly invoked, never a built-in sweep.
func repairPStyleTitleAnchor(path string) error {
	guards := []string{`name="title"`, `name="subtitle"`,
		`<a:off x="401904" y="1963495"/>`,
		`<a:off x="401904" y="4000000"/><a:ext cx="5664555" cy="1100000"/>`,
		`<a:defRPr sz="4800" b="0"/>`}
	const oldFrame = `<a:off x="401904" y="1963495"/><a:ext cx="5664555" cy="1828800"/>`
	const newFrame = `<a:off x="401904" y="1963495"/><a:ext cx="8000000" cy="1828800"/>`
	return repairReviewedParts(path, "p-style-title-grouping-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout1.xml": {
			old: `<a:bodyPr anchor="t" anchorCtr="0"/>`, replacement: `<a:bodyPr anchor="b" anchorCtr="0"/>`, guards: guards,
			secondOld: oldFrame, secondReplacement: newFrame,
		},
		"ppt/slides/slide1.xml": {
			old: `<a:bodyPr wrap="square" anchor="t" anchorCtr="0">`, replacement: `<a:bodyPr wrap="square" anchor="b" anchorCtr="0">`, guards: guards,
			secondOld: oldFrame, secondReplacement: newFrame,
		},
	})
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
			if repair.secondOld != "" && (bytes.Count(body, []byte(repair.secondReplacement)) != 1 || bytes.Contains(body, []byte(repair.secondOld))) {
				return fmt.Errorf("%s has partial reviewed repair; refusing to patch", entry.Name)
			}
			continue // Already repaired, without rewriting this part.
		}
		if bytes.Count(body, []byte(repair.old)) != 1 {
			return fmt.Errorf("%s differs from reviewed source; refusing to patch", entry.Name)
		}
		updates[entry.Name] = bytes.Replace(body, []byte(repair.old), []byte(repair.replacement), 1)
		if repair.secondOld != "" {
			if bytes.Count(body, []byte(repair.secondOld)) != 1 || bytes.Contains(body, []byte(repair.secondReplacement)) {
				return fmt.Errorf("%s has unexpected or partial reviewed geometry; refusing to patch", entry.Name)
			}
			updates[entry.Name] = bytes.Replace(updates[entry.Name], []byte(repair.secondOld), []byte(repair.secondReplacement), 1)
		}
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
