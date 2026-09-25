package generator

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// Notes-master support (go-slide-creator-s1uvj.28).
//
// ECMA-376 requires every notesSlide part to carry an implicit relationship to
// a notesMaster part. Notes slides used to be written with only the back
// reference to their slide. prepareNotesMaster resolves the template's notes
// master when it has one; otherwise it synthesizes a minimal notes master with
// its own theme part, content-type overrides, a presentation relationship and
// a p:notesMasterIdLst entry.

const (
	contentTypeNotesMaster = "application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"
	contentTypeTheme       = "application/vnd.openxmlformats-officedocument.theme+xml"
	relTypeTheme           = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"

	synthNotesMasterPath = "ppt/notesMasters/notesMaster1.xml"
	synthNotesMasterRels = "ppt/notesMasters/_rels/notesMaster1.xml.rels"
)

var themePartNum = regexp.MustCompile(`^ppt/theme/theme(\d+)\.xml$`)

// notesMasterState records the notes master every notes slide relates to.
type notesMasterState struct {
	// part is the notes master's package path, e.g. ppt/notesMasters/notesMaster1.xml.
	part string
	// synthesized is set when the template had no notes master and one was
	// generated; the fields below are only meaningful then.
	synthesized bool
	themePart   string
	presRelID   string
}

// prepareNotesMaster decides which notes master the deck's notes slides
// relate to, synthesizing one when the template has none. It must run after
// prepareSlides (so presentation.xml and the slide rIds exist) and after
// applyThemeOverrideToThemeParts (so the synthesized theme copies the patched
// theme), and before writeTemplateFiles.
func (ctx *singlePassContext) prepareNotesMaster() error {
	ctx.notesMaster = nil
	if len(ctx.slideNotes) == 0 || ctx.templateIndex == nil {
		return nil
	}

	presRels, err := ctx.templatePresentationRels()
	if err != nil {
		return err
	}
	for _, rel := range presRels.Relationships {
		if rel.Type == pptx.RelTypeNotesMaster {
			part := resolvePresentationRelTarget(rel.Target)
			if _, err := utils.ReadFileFromZipIndex(ctx.templateIndex, part); err == nil {
				ctx.notesMaster = &notesMasterState{part: part}
				return nil
			}
		}
	}

	return ctx.synthesizeNotesMaster(presRels)
}

func (ctx *singlePassContext) templatePresentationRels() (*pptx.RelationshipsXML, error) {
	data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, PathPresentationRels)
	if err != nil {
		return nil, fmt.Errorf("notes master: read %s: %w", PathPresentationRels, err)
	}
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		return nil, fmt.Errorf("notes master: parse %s: %w", PathPresentationRels, err)
	}
	return &rels, nil
}

// resolvePresentationRelTarget turns a presentation.xml.rels target into a
// package path.
func resolvePresentationRelTarget(target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join("ppt", target))
}

func (ctx *singlePassContext) synthesizeNotesMaster(presRels *pptx.RelationshipsXML) error {
	// The notes master gets its own copy of the deck theme: a theme part is
	// owned by exactly one master.
	var sourceTheme string
	maxTheme := 0
	for _, rel := range presRels.Relationships {
		if rel.Type == relTypeTheme && sourceTheme == "" {
			sourceTheme = resolvePresentationRelTarget(rel.Target)
		}
	}
	for _, f := range ctx.templateReader.File {
		if m := themePartNum.FindStringSubmatch(f.Name); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > maxTheme {
				maxTheme = n
			}
		}
	}
	for p := range ctx.syntheticFiles {
		if m := themePartNum.FindStringSubmatch(p); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > maxTheme {
				maxTheme = n
			}
		}
	}
	if sourceTheme == "" {
		sourceTheme = "ppt/theme/theme1.xml"
	}
	themeData, ok := ctx.syntheticFiles[sourceTheme]
	if !ok {
		var err error
		themeData, err = utils.ReadFileFromZipIndex(ctx.templateIndex, sourceTheme)
		if err != nil {
			return fmt.Errorf("notes master: read theme %s: %w", sourceTheme, err)
		}
	}
	themePart := fmt.Sprintf("ppt/theme/theme%d.xml", maxTheme+1)

	// Pick an rId above every presentation relationship, template and new slides.
	maxRel := 0
	consider := func(id string) {
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "rId")); err == nil && strings.HasPrefix(id, "rId") && n > maxRel {
			maxRel = n
		}
	}
	for _, rel := range presRels.Relationships {
		consider(rel.ID)
	}
	for _, id := range ctx.slideRelIDs {
		consider(id)
	}
	presRelID := fmt.Sprintf("rId%d", maxRel+1)

	presXML, ok := ctx.modifiedFiles[PathPresentationXML]
	if !ok {
		var err error
		presXML, err = utils.ReadFileFromZipIndex(ctx.templateIndex, PathPresentationXML)
		if err != nil {
			return fmt.Errorf("notes master: read %s: %w", PathPresentationXML, err)
		}
	}
	updated, err := insertNotesMasterIDList(string(presXML), presRelID)
	if err != nil {
		return err
	}

	masterRels, err := xml.Marshal(pptx.RelationshipsXML{
		XMLName: xml.Name{Space: pptx.NsPackageRels, Local: "Relationships"},
		Xmlns:   pptx.NsPackageRels,
		Relationships: []pptx.RelationshipXML{{
			ID:     "rId1",
			Type:   relTypeTheme,
			Target: "../theme/" + path.Base(themePart),
		}},
	})
	if err != nil {
		return fmt.Errorf("notes master: marshal rels: %w", err)
	}

	if ctx.syntheticFiles == nil {
		ctx.syntheticFiles = make(map[string][]byte)
	}
	ctx.modifiedFiles[PathPresentationXML] = []byte(updated)
	ctx.syntheticFiles[synthNotesMasterPath] = []byte(minimalNotesMasterXML)
	ctx.syntheticFiles[synthNotesMasterRels] = append([]byte(xml.Header), masterRels...)
	ctx.syntheticFiles[themePart] = themeData
	ctx.notesMaster = &notesMasterState{
		part:        synthNotesMasterPath,
		synthesized: true,
		themePart:   themePart,
		presRelID:   presRelID,
	}
	slog.Debug("synthesized notes master", slog.String("theme", themePart), slog.String("rel_id", presRelID))
	return nil
}

// insertNotesMasterIDList adds <p:notesMasterIdLst> directly after
// <p:sldMasterIdLst>, where CT_Presentation's sequence requires it.
func insertNotesMasterIDList(presXML, relID string) (string, error) {
	if strings.Contains(presXML, "notesMasterIdLst") {
		return presXML, nil
	}
	entry := `<p:notesMasterIdLst><p:notesMasterId r:id="` + relID + `"/></p:notesMasterIdLst>`
	const closing = "</p:sldMasterIdLst>"
	idx := strings.Index(presXML, closing)
	if idx < 0 {
		return "", fmt.Errorf("notes master: presentation.xml has no %s", closing)
	}
	idx += len(closing)
	return presXML[:idx] + entry + presXML[idx:], nil
}

// notesMasterRelTarget is the notes master's target relative to ppt/notesSlides/.
func (ctx *singlePassContext) notesMasterRelTarget() string {
	if ctx.notesMaster == nil {
		return ""
	}
	return "../" + strings.TrimPrefix(ctx.notesMaster.part, "ppt/")
}

// addNotesMasterContentTypes registers the synthesized notes master and its
// theme in [Content_Types].xml.
func addNotesMasterContentTypes(ctData []byte, nm *notesMasterState) ([]byte, error) {
	if nm == nil || !nm.synthesized {
		return ctData, nil
	}
	var contentTypes pptx.ContentTypesXML
	if err := xml.Unmarshal(ctData, &contentTypes); err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml for notes master: %w", err)
	}
	existing := make(map[string]bool, len(contentTypes.Overrides))
	for _, ovr := range contentTypes.Overrides {
		existing[ovr.PartName] = true
	}
	for _, o := range []pptx.ContentTypeOverride{
		{PartName: "/" + nm.part, ContentType: contentTypeNotesMaster},
		{PartName: "/" + nm.themePart, ContentType: contentTypeTheme},
	} {
		if !existing[o.PartName] {
			contentTypes.Overrides = append(contentTypes.Overrides, o)
		}
	}
	out, err := xml.Marshal(contentTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal [Content_Types].xml with notes master: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

// minimalNotesMasterXML is a schema-valid notes master for the default
// 6858000 x 9144000 notes page: a slide-image and a notes-body placeholder
// plus the required colour map.
const minimalNotesMasterXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notesMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr><p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image Placeholder 1"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg" idx="2"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="685800" y="1143000"/><a:ext cx="5486400" cy="3086100"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln w="12700"><a:solidFill><a:prstClr val="black"/></a:solidFill></a:ln></p:spPr></p:sp><p:sp><p:nvSpPr><p:cNvPr id="3" name="Notes Placeholder 2"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" sz="quarter" idx="3"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="685800" y="4400550"/><a:ext cx="5486400" cy="3600450"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0"/><a:lstStyle/><a:p><a:pPr lvl="0"/><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master text styles</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:notesStyle><a:lvl1pPr marL="0" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1200" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl1pPr></p:notesStyle></p:notesMaster>`
