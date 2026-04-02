// mktemplate generates PPTX template files from predefined theme definitions.
//
// Usage:
//
//	go run ./cmd/mktemplate -name midnight-blue -out templates/midnight-blue.pptx
//	go run ./cmd/mktemplate -all -outdir templates/
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// templateDef defines a complete template theme.
type templateDef struct {
	Name        string
	DisplayName string
	Description string
	// Theme colors (RRGGBB without #)
	Dark1, Light1, Dark2, Light2 string
	Accent1, Accent2, Accent3   string
	Accent4, Accent5, Accent6   string
	Hlink, FolHlink              string
	// Fonts
	MajorFont string // Headings
	MinorFont string // Body
	// Master slide bar color (schemeClr value)
	BarSchemeClr string
	// Bullet character
	BulletChar string
}

var templates = []templateDef{
	{
		Name:         "midnight-blue",
		DisplayName:  "Midnight Blue",
		Description:  "Formal enterprise template with navy blue theme and conservative styling",
		Dark1:        "000000", Light1: "FFFFFF",
		Dark2:        "1B2A4A", Light2: "E8ECF1",
		Accent1:      "2E5090", Accent2: "D4463A",
		Accent3:      "E8A838", Accent4: "43A047",
		Accent5:      "5C6BC0", Accent6: "26A69A",
		Hlink:        "2E5090", FolHlink: "7986CB",
		MajorFont:    "Calibri", MinorFont: "Calibri",
		BarSchemeClr: "accent1",
		BulletChar:   "\u25A0",
	},
	{
		Name:         "forest-green",
		DisplayName:  "Forest Green",
		Description:  "Clean analytical template with green accent, suited for data-heavy presentations",
		Dark1:        "000000", Light1: "FFFFFF",
		Dark2:        "1A3C34", Light2: "EDF5F0",
		Accent1:      "2E7D32", Accent2: "FF8F00",
		Accent3:      "1565C0", Accent4: "6A1B9A",
		Accent5:      "00838F", Accent6: "C62828",
		Hlink:        "1565C0", FolHlink: "7986CB",
		MajorFont:    "Calibri", MinorFont: "Calibri",
		BarSchemeClr: "accent1",
		BulletChar:   "\u2022",
	},
	{
		Name:         "warm-coral",
		DisplayName:  "Warm Coral",
		Description:  "Modern creative template with warm coral tones for engaging visual presentations",
		Dark1:        "000000", Light1: "FFFFFF",
		Dark2:        "3E2723", Light2: "FBE9E7",
		Accent1:      "E64A19", Accent2: "5D4037",
		Accent3:      "FF8A65", Accent4: "0097A7",
		Accent5:      "7B1FA2", Accent6: "689F38",
		Hlink:        "0097A7", FolHlink: "7986CB",
		MajorFont:    "Gill Sans", MinorFont: "Calibri",
		BarSchemeClr: "accent1",
		BulletChar:   "\u2013",
	},
}

var deterministicTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	nameFlag := flag.String("name", "", "Template name to generate (e.g. midnight-blue)")
	allFlag := flag.Bool("all", false, "Generate all templates")
	outFlag := flag.String("out", "", "Output file path (for single template)")
	outdirFlag := flag.String("outdir", "templates", "Output directory (for -all)")
	flag.Parse()

	if !*allFlag && *nameFlag == "" {
		fmt.Println("Available templates:")
		for _, t := range templates {
			fmt.Printf("  %-20s %s\n", t.Name, t.Description)
		}
		fmt.Println("\nUsage: mktemplate -name <name> -out <path>")
		fmt.Println("       mktemplate -all -outdir <dir>")
		os.Exit(0)
	}

	if *allFlag {
		for _, t := range templates {
			outPath := filepath.Join(*outdirFlag, t.Name+".pptx")
			if err := generateTemplate(t, outPath); err != nil {
				log.Fatalf("Error generating %s: %v", t.Name, err)
			}
			fmt.Printf("Generated: %s\n", outPath)
		}
		return
	}

	// Single template
	var def *templateDef
	for i := range templates {
		if templates[i].Name == *nameFlag {
			def = &templates[i]
			break
		}
	}
	if def == nil {
		log.Fatalf("Unknown template: %s", *nameFlag)
	}

	outPath := *outFlag
	if outPath == "" {
		outPath = def.Name + ".pptx"
	}
	if err := generateTemplate(*def, outPath); err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("Generated: %s\n", outPath)
}

func generateTemplate(def templateDef, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Each entry uses deterministic timestamp
	add := func(name, content string) error {
		header := &zip.FileHeader{
			Name:     name,
			Method:   zip.Deflate,
			Modified: deterministicTime,
		}
		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write([]byte(content))
		return err
	}

	// --- Package relationships ---
	if err := add("[Content_Types].xml", contentTypes(def)); err != nil {
		return err
	}
	if err := add("_rels/.rels", packageRels()); err != nil {
		return err
	}

	// --- Presentation ---
	if err := add("ppt/presentation.xml", presentation(def)); err != nil {
		return err
	}
	if err := add("ppt/_rels/presentation.xml.rels", presentationRels(def)); err != nil {
		return err
	}

	// --- Theme ---
	if err := add("ppt/theme/theme1.xml", theme(def)); err != nil {
		return err
	}

	// --- Slide Master ---
	if err := add("ppt/slideMasters/slideMaster1.xml", slideMaster(def)); err != nil {
		return err
	}
	if err := add("ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRels(def)); err != nil {
		return err
	}

	// --- Slide Layouts ---
	layouts := []struct {
		idx      int
		name     string
		generate func(templateDef) string
	}{
		{1, "slideLayout1", titleSlideLayout},
		{2, "slideLayout2", contentLayout},
		{3, "slideLayout3", twoColumnLayout},
		{4, "slideLayout4", sectionLayout},
		{5, "slideLayout5", closingLayout},
		{6, "slideLayout6", blankLayout},
	}
	for _, l := range layouts {
		path := fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", l.idx)
		relsPath := fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", l.idx)
		if err := add(path, l.generate(def)); err != nil {
			return err
		}
		if err := add(relsPath, layoutRels()); err != nil {
			return err
		}
	}

	// --- Slides (4 required: title, content, section, closing) ---
	slideLayouts := []int{1, 2, 4, 5} // layout indices used by each slide
	for i, layoutIdx := range slideLayouts {
		slideNum := i + 1
		slidePath := fmt.Sprintf("ppt/slides/slide%d.xml", slideNum)
		slideRelsPath := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNum)
		if err := add(slidePath, slide(slideNum, layoutIdx)); err != nil {
			return err
		}
		if err := add(slideRelsPath, slideRels(layoutIdx)); err != nil {
			return err
		}
	}

	// --- Supporting files ---
	if err := add("ppt/presProps.xml", presProps()); err != nil {
		return err
	}
	if err := add("ppt/viewProps.xml", viewProps()); err != nil {
		return err
	}
	if err := add("ppt/tableStyles.xml", tableStyles()); err != nil {
		return err
	}
	if err := add("docProps/app.xml", appProps(def)); err != nil {
		return err
	}
	if err := add("docProps/core.xml", coreProps()); err != nil {
		return err
	}

	return nil
}

func contentTypes(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
		`<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>` +
		`<Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>` +
		`<Override PartName="/ppt/slides/slide3.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>` +
		`<Override PartName="/ppt/slides/slide4.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>` +
		`<Override PartName="/ppt/presProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"/>` +
		`<Override PartName="/ppt/viewProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"/>` +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		`<Override PartName="/ppt/tableStyles.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout3.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout4.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout5.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout6.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`
}

func packageRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func presentation(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" autoCompressPictures="0">` +
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
		`<p:sldIdLst>` +
		`<p:sldId id="256" r:id="rId2"/>` +
		`<p:sldId id="257" r:id="rId3"/>` +
		`<p:sldId id="258" r:id="rId4"/>` +
		`<p:sldId id="259" r:id="rId5"/>` +
		`</p:sldIdLst>` +
		`<p:sldSz cx="12192000" cy="6858000"/>` +
		`<p:notesSz cx="6858000" cy="9144000"/>` +
		defaultTextStyle() +
		`</p:presentation>`
}

func defaultTextStyle() string {
	levels := ""
	for i := 1; i <= 9; i++ {
		marL := (i - 1) * 457200
		levels += fmt.Sprintf(
			`<a:lvl%dpPr marL="%d" algn="l" defTabSz="457200" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">`+
				`<a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill>`+
				`<a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl%dpPr>`,
			i, marL, i)
	}
	return `<p:defaultTextStyle><a:defPPr><a:defRPr lang="en-US"/></a:defPPr>` + levels + `</p:defaultTextStyle>`
}

func presentationRels(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>` +
		`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide3.xml"/>` +
		`<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide4.xml"/>` +
		`<Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps" Target="presProps.xml"/>` +
		`<Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps" Target="viewProps.xml"/>` +
		`<Relationship Id="rId8" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>` +
		`<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles" Target="tableStyles.xml"/>` +
		`</Relationships>`
}

func theme(def templateDef) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="%s">`, xmlEsc(def.DisplayName)) +
		`<a:themeElements>` +
		fmt.Sprintf(`<a:clrScheme name="%s">`, xmlEsc(def.DisplayName)) +
		fmt.Sprintf(`<a:dk1><a:srgbClr val="%s"/></a:dk1>`, def.Dark1) +
		fmt.Sprintf(`<a:lt1><a:srgbClr val="%s"/></a:lt1>`, def.Light1) +
		fmt.Sprintf(`<a:dk2><a:srgbClr val="%s"/></a:dk2>`, def.Dark2) +
		fmt.Sprintf(`<a:lt2><a:srgbClr val="%s"/></a:lt2>`, def.Light2) +
		fmt.Sprintf(`<a:accent1><a:srgbClr val="%s"/></a:accent1>`, def.Accent1) +
		fmt.Sprintf(`<a:accent2><a:srgbClr val="%s"/></a:accent2>`, def.Accent2) +
		fmt.Sprintf(`<a:accent3><a:srgbClr val="%s"/></a:accent3>`, def.Accent3) +
		fmt.Sprintf(`<a:accent4><a:srgbClr val="%s"/></a:accent4>`, def.Accent4) +
		fmt.Sprintf(`<a:accent5><a:srgbClr val="%s"/></a:accent5>`, def.Accent5) +
		fmt.Sprintf(`<a:accent6><a:srgbClr val="%s"/></a:accent6>`, def.Accent6) +
		fmt.Sprintf(`<a:hlink><a:srgbClr val="%s"/></a:hlink>`, def.Hlink) +
		fmt.Sprintf(`<a:folHlink><a:srgbClr val="%s"/></a:folHlink>`, def.FolHlink) +
		`</a:clrScheme>` +
		fontScheme(def) +
		fmtScheme(def) +
		`</a:themeElements>` +
		`<a:objectDefaults/><a:extraClrSchemeLst/>` +
		`</a:theme>`
}

func fontScheme(def templateDef) string {
	return fmt.Sprintf(`<a:fontScheme name="%s">`, xmlEsc(def.DisplayName)) +
		fmt.Sprintf(`<a:majorFont><a:latin typeface="%s"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>`, xmlEsc(def.MajorFont)) +
		fmt.Sprintf(`<a:minorFont><a:latin typeface="%s"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>`, xmlEsc(def.MinorFont)) +
		`</a:fontScheme>`
}

func fmtScheme(def templateDef) string {
	return fmt.Sprintf(`<a:fmtScheme name="%s">`, xmlEsc(def.DisplayName)) +
		`<a:fillStyleLst>` +
		`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="67000"/><a:satMod val="105000"/><a:lumMod val="110000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="phClr"><a:tint val="73000"/><a:satMod val="103000"/><a:lumMod val="105000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="phClr"><a:tint val="81000"/><a:satMod val="109000"/><a:lumMod val="105000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="94000"/><a:satMod val="103000"/><a:lumMod val="102000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="phClr"><a:shade val="100000"/><a:satMod val="110000"/><a:lumMod val="100000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="78000"/><a:satMod val="120000"/><a:lumMod val="99000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`</a:fillStyleLst>` +
		`<a:lnStyleLst>` +
		`<a:ln w="6350" cap="flat" cmpd="sng" algn="in"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln>` +
		`<a:ln w="12700" cap="flat" cmpd="sng" algn="in"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln>` +
		`<a:ln w="19050" cap="flat" cmpd="sng" algn="in"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln>` +
		`</a:lnStyleLst>` +
		`<a:effectStyleLst>` +
		`<a:effectStyle><a:effectLst/></a:effectStyle>` +
		`<a:effectStyle><a:effectLst/></a:effectStyle>` +
		`<a:effectStyle><a:effectLst>` +
		`<a:outerShdw blurRad="57150" dist="19050" dir="5400000" algn="ctr" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="35000"/></a:srgbClr></a:outerShdw>` +
		`</a:effectLst></a:effectStyle>` +
		`</a:effectStyleLst>` +
		`<a:bgFillStyleLst>` +
		`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:solidFill><a:schemeClr val="phClr"><a:tint val="95000"/><a:satMod val="170000"/></a:schemeClr></a:solidFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="93000"/><a:shade val="98000"/><a:satMod val="150000"/><a:lumMod val="102000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="phClr"><a:tint val="98000"/><a:shade val="90000"/><a:satMod val="130000"/><a:lumMod val="103000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="63000"/><a:satMod val="120000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`</a:bgFillStyleLst>` +
		`</a:fmtScheme>`
}

func slideMaster(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld>` +
		`<p:bg><p:bgPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		// Title placeholder
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="838200" y="365125"/><a:ext cx="10515600" cy="1325563"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0" anchor="ctr"><a:normAutofit/></a:bodyPr>` +
		`<a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master title style</a:t></a:r></a:p></p:txBody></p:sp>` +
		// Body placeholder
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="838200" y="1825625"/><a:ext cx="10515600" cy="4351338"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0"><a:normAutofit/></a:bodyPr>` +
		`<a:lstStyle/><a:p><a:pPr lvl="0"/><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master text styles</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="1"/><a:r><a:rPr lang="en-US"/><a:t>Second level</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="2"/><a:r><a:rPr lang="en-US"/><a:t>Third level</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="3"/><a:r><a:rPr lang="en-US"/><a:t>Fourth level</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="4"/><a:r><a:rPr lang="en-US"/><a:t>Fifth level</a:t></a:r></a:p></p:txBody></p:sp>` +
		// Date placeholder
		`<p:sp><p:nvSpPr><p:cNvPr id="4" name="Date Placeholder 3"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="dt" sz="half" idx="2"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="838200" y="6356350"/><a:ext cx="2743200" cy="365125"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0" anchor="ctr"/>` +
		`<a:lstStyle><a:lvl1pPr algn="l"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:fld id="{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}" type="datetimeFigureOut"><a:rPr lang="en-US"/><a:t>1/1/2000</a:t></a:fld></a:p></p:txBody></p:sp>` +
		// Footer placeholder
		`<p:sp><p:nvSpPr><p:cNvPr id="5" name="Footer Placeholder 4"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="ftr" sz="quarter" idx="3"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="4038600" y="6356350"/><a:ext cx="4114800" cy="365125"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0" anchor="ctr"/>` +
		`<a:lstStyle><a:lvl1pPr algn="ctr"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp>` +
		// Slide number placeholder
		`<p:sp><p:nvSpPr><p:cNvPr id="6" name="Slide Number Placeholder 5"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="sldNum" sz="quarter" idx="4"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="8610600" y="6356350"/><a:ext cx="2743200" cy="365125"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0" anchor="ctr"/>` +
		`<a:lstStyle><a:lvl1pPr algn="r"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:fld id="{B2C3D4E5-F6A7-8901-BCDE-F12345678901}" type="slidenum"><a:rPr lang="en-US"/><a:t>‹#›</a:t></a:fld></a:p></p:txBody></p:sp>` +
		// Accent bar decorative shape
		fmt.Sprintf(
			`<p:sp><p:nvSpPr><p:cNvPr id="7" name="Accent Bar"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>`+
				`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="228600" cy="6858000"/></a:xfrm>`+
				`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+
				`<a:solidFill><a:schemeClr val="%s"/></a:solidFill>`+
				`<a:ln><a:noFill/></a:ln></p:spPr>`+
				`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp>`,
			def.BarSchemeClr) +
		`</p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:sldLayoutIdLst>` +
		`<p:sldLayoutId id="2147483649" r:id="rId1"/>` +
		`<p:sldLayoutId id="2147483650" r:id="rId2"/>` +
		`<p:sldLayoutId id="2147483651" r:id="rId3"/>` +
		`<p:sldLayoutId id="2147483652" r:id="rId4"/>` +
		`<p:sldLayoutId id="2147483653" r:id="rId5"/>` +
		`<p:sldLayoutId id="2147483654" r:id="rId6"/>` +
		`</p:sldLayoutIdLst>` +
		masterTextStyles(def) +
		`</p:sldMaster>`
}

func masterTextStyles(def templateDef) string {
	bulletChar := xmlEsc(def.BulletChar)
	return `<p:txStyles>` +
		`<p:titleStyle>` +
		`<a:lvl1pPr algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">` +
		`<a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:spcBef><a:spcPct val="0"/></a:spcBef><a:buNone/>` +
		`<a:defRPr sz="4400" kern="1200"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill>` +
		`<a:latin typeface="+mj-lt"/><a:ea typeface="+mj-ea"/><a:cs typeface="+mj-cs"/></a:defRPr></a:lvl1pPr>` +
		`</p:titleStyle>` +
		`<p:bodyStyle>` +
		bodyStyleLevels(bulletChar) +
		`</p:bodyStyle>` +
		`<p:otherStyle>` +
		otherStyleLevels() +
		`</p:otherStyle>` +
		`</p:txStyles>`
}

func bodyStyleLevels(bulletChar string) string {
	sizes := []int{2000, 1800, 1600, 1600, 1600, 1400, 1400, 1400, 1400}
	var b strings.Builder
	for i, sz := range sizes {
		marL := (i + 1) * 384048
		indent := -384048
		b.WriteString(fmt.Sprintf(
			`<a:lvl%dpPr marL="%d" indent="%d" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">`+
				`<a:lnSpc><a:spcPct val="94000"/></a:lnSpc><a:spcBef><a:spcPts val="800"/></a:spcBef><a:spcAft><a:spcPts val="200"/></a:spcAft>`+
				`<a:buChar char="%s"/>`+
				`<a:defRPr sz="%d" kern="1200"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill>`+
				`<a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl%dpPr>`,
			i+1, marL, indent, bulletChar, sz, i+1))
	}
	return b.String()
}

func otherStyleLevels() string {
	var b strings.Builder
	b.WriteString(`<a:defPPr><a:defRPr lang="en-US"/></a:defPPr>`)
	for i := 1; i <= 9; i++ {
		marL := (i - 1) * 457200
		b.WriteString(fmt.Sprintf(
			`<a:lvl%dpPr marL="%d" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">`+
				`<a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill>`+
				`<a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl%dpPr>`,
			i, marL, i))
	}
	return b.String()
}

func slideMasterRels(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout2.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout3.xml"/>` +
		`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout4.xml"/>` +
		`<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout5.xml"/>` +
		`<Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout6.xml"/>` +
		`<Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
		`</Relationships>`
}

// --- Slide Layouts ---

func titleSlideLayout(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="title" preserve="1">` +
		`<p:cSld name="Title Slide">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		// Center title
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="ctrTitle"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="1524000" y="1122363"/><a:ext cx="9144000" cy="2387600"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr anchor="b"><a:noAutofit/></a:bodyPr>` +
		`<a:lstStyle><a:lvl1pPr algn="ctr"><a:defRPr sz="6000"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master title style</a:t></a:r></a:p></p:txBody></p:sp>` +
		// Subtitle
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="subtitle"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="subTitle" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="1524000" y="3602038"/><a:ext cx="9144000" cy="1655762"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr><a:normAutofit/></a:bodyPr>` +
		`<a:lstStyle><a:lvl1pPr marL="0" indent="0" algn="ctr"><a:buNone/><a:defRPr sz="2400"/></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master subtitle style</a:t></a:r></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func contentLayout(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="obj" preserve="1">` +
		`<p:cSld name="One Content">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		// Title
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master title style</a:t></a:r></a:p></p:txBody></p:sp>` +
		// Content body
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:pPr lvl="0"/><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master text styles</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="1"/><a:r><a:rPr lang="en-US"/><a:t>Second level</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="2"/><a:r><a:rPr lang="en-US"/><a:t>Third level</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="3"/><a:r><a:rPr lang="en-US"/><a:t>Fourth level</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="4"/><a:r><a:rPr lang="en-US"/><a:t>Fifth level</a:t></a:r></a:p>` +
		`</p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func twoColumnLayout(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="twoObj" preserve="1">` +
		`<p:cSld name="Two Content">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		// Title
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master title style</a:t></a:r></a:p></p:txBody></p:sp>` +
		// Left content (idx=1)
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="body" sz="half" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="838200" y="1825625"/><a:ext cx="5181600" cy="4351338"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:pPr lvl="0"/><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master text styles</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="1"/><a:r><a:rPr lang="en-US"/><a:t>Second level</a:t></a:r></a:p>` +
		`</p:txBody></p:sp>` +
		// Right content (idx=2)
		`<p:sp><p:nvSpPr><p:cNvPr id="4" name="body_2"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="body" sz="half" idx="2"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="6172200" y="1825625"/><a:ext cx="5181600" cy="4351338"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:pPr lvl="0"/><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master text styles</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="1"/><a:r><a:rPr lang="en-US"/><a:t>Second level</a:t></a:r></a:p>` +
		`</p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func sectionLayout(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" preserve="1" userDrawn="1">` +
		`<p:cSld name="Section Divider">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		// Section title placeholder (left side)
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="838200" y="1268413"/><a:ext cx="6400800" cy="3417887"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr anchor="ctr"><a:normAutofit/></a:bodyPr>` +
		`<a:lstStyle><a:lvl1pPr><a:buNone/><a:defRPr sz="3600"/></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:r><a:rPr lang="en-US"/><a:t>Section Title</a:t></a:r></a:p></p:txBody></p:sp>` +
		// Decorative section number placeholder (right side)
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="body" sz="quarter" idx="1" hasCustomPrompt="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="7886700" y="1268413"/><a:ext cx="3182938" cy="3417887"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr marL="11113" indent="-11113"><a:buNone/><a:defRPr sz="9600"/></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:pPr lvl="0"/><a:r><a:rPr lang="en-US"/><a:t>#</a:t></a:r></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func closingLayout(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" preserve="1" userDrawn="1">` +
		`<p:cSld name="Closing">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		// Center title
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="ctrTitle"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="1485900" y="685800"/><a:ext cx="9486900" cy="1851025"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr anchor="b"><a:noAutofit/></a:bodyPr>` +
		`<a:lstStyle><a:lvl1pPr algn="ctr"><a:defRPr sz="6600"><a:solidFill><a:schemeClr val="dk2"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp>` +
		// Subtitle
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="subtitle"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="subTitle" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="1485900" y="2667000"/><a:ext cx="9486900" cy="1371600"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr><a:normAutofit/></a:bodyPr>` +
		`<a:lstStyle><a:lvl1pPr marL="0" indent="0" algn="ctr"><a:buNone/><a:defRPr sz="2400"/></a:lvl1pPr></a:lstStyle>` +
		`<a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func blankLayout(def templateDef) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">` +
		`<p:cSld name="Blank">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func layoutRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
		`</Relationships>`
}

// --- Slides ---

func slide(_, _ int) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sld>`
}

func slideRels(layoutIdx int) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout%d.xml"/>`+
		`</Relationships>`, layoutIdx)
}

// --- Supporting files ---

func presProps() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentationPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`
}

func viewProps() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:viewPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:normalViewPr><p:restoredLeft sz="15620"/><p:restoredTop sz="94660"/></p:normalViewPr>` +
		`<p:slideViewPr><p:cSldViewPr><p:cViewPr varScale="1"><p:scale><a:sx n="100" d="100"/><a:sy n="100" d="100"/></p:scale><p:origin x="0" y="0"/></p:cViewPr><p:guideLst/></p:cSldViewPr></p:slideViewPr>` +
		`<p:notesTextViewPr><p:cViewPr><p:scale><a:sx n="1" d="1"/><a:sy n="1" d="1"/></p:scale><p:origin x="0" y="0"/></p:cViewPr></p:notesTextViewPr>` +
		`<p:gridSpacing cx="72008" cy="72008"/>` +
		`</p:viewPr>`
}

func tableStyles() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"/>`
}

func appProps(def templateDef) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`+
		`<Template>%s</Template>`+
		`<TotalTime>0</TotalTime>`+
		`<Words>0</Words>`+
		`<Application>go-slide-creator</Application>`+
		`<PresentationFormat>Widescreen</PresentationFormat>`+
		`<Paragraphs>0</Paragraphs>`+
		`<Slides>4</Slides>`+
		`<Notes>0</Notes>`+
		`<HiddenSlides>0</HiddenSlides>`+
		`<MMClips>0</MMClips>`+
		`<ScaleCrop>false</ScaleCrop>`+
		`<LinksUpToDate>false</LinksUpToDate>`+
		`<SharedDoc>false</SharedDoc>`+
		`<HyperlinksChanged>false</HyperlinksChanged>`+
		`<AppVersion>16.0000</AppVersion>`+
		`</Properties>`, xmlEsc(def.DisplayName))
}

func coreProps() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title/>` +
		`<dc:creator>go-slide-creator</dc:creator>` +
		`<cp:lastModifiedBy>go-slide-creator</cp:lastModifiedBy>` +
		`<cp:revision>1</cp:revision>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">2000-01-01T00:00:00Z</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">2000-01-01T00:00:00Z</dcterms:modified>` +
		`</cp:coreProperties>`
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
