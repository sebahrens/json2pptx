// Package testrand generates random PresentationInput JSON for E2E fuzz testing.
// All randomization is seed-based for 100% reproducible failures.
package testrand

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

// PresentationInput mirrors cmd/json2pptx.PresentationInput for JSON generation.
type PresentationInput struct {
	Template       string       `json:"template"`
	OutputFilename string       `json:"output_filename,omitempty"`
	Footer         *Footer      `json:"footer,omitempty"`
	ThemeOverride  *ThemeInput  `json:"theme_override,omitempty"`
	Slides         []SlideInput `json:"slides"`
}

// Footer mirrors JSONFooter.
type Footer struct {
	Enabled  bool   `json:"enabled"`
	LeftText string `json:"left_text,omitempty"`
}

// ThemeInput mirrors ThemeInput.
type ThemeInput struct {
	Colors    map[string]string `json:"colors,omitempty"`
	TitleFont string            `json:"title_font,omitempty"`
	BodyFont  string            `json:"body_font,omitempty"`
}

// SlideInput mirrors SlideInput.
type SlideInput struct {
	LayoutID     string         `json:"layout_id,omitempty"`
	SlideType    string         `json:"slide_type,omitempty"`
	Content      []ContentInput `json:"content"`
	SpeakerNotes string         `json:"speaker_notes,omitempty"`
	Source       string         `json:"source,omitempty"`
}

// ContentInput mirrors ContentInput with typed value fields.
type ContentInput struct {
	PlaceholderID string          `json:"placeholder_id"`
	Type          string          `json:"type"`
	TextValue     *string         `json:"text_value,omitempty"`
	BulletsValue  *[]string       `json:"bullets_value,omitempty"`
	BABValue      *BABInput       `json:"body_and_bullets_value,omitempty"`
	BGValue       *BGInput        `json:"bullet_groups_value,omitempty"`
	TableValue    *TableInput     `json:"table_value,omitempty"`
	ChartValue    *ChartInput     `json:"chart_value,omitempty"`
	DiagramValue  *DiagramInput   `json:"diagram_value,omitempty"`
	ImageValue    *ImageInput     `json:"image_value,omitempty"`
}

// BABInput = body_and_bullets.
type BABInput struct {
	Body         string   `json:"body"`
	Bullets      []string `json:"bullets"`
	TrailingBody string   `json:"trailing_body,omitempty"`
}

// BGInput = bullet_groups.
type BGInput struct {
	Body         string        `json:"body,omitempty"`
	Groups       []GroupInput  `json:"groups"`
	TrailingBody string        `json:"trailing_body,omitempty"`
}

// GroupInput is a single bullet group.
type GroupInput struct {
	Header  string   `json:"header,omitempty"`
	Body    string   `json:"body,omitempty"`
	Bullets []string `json:"bullets"`
}

// TableInput mirrors TableInput.
type TableInput struct {
	Headers          []string        `json:"headers"`
	Rows             [][]interface{} `json:"rows"`
	Style            *TableStyle     `json:"style,omitempty"`
	ColumnAlignments []string        `json:"column_alignments,omitempty"`
}

// TableStyle mirrors TableStyleInput.
type TableStyle struct {
	HeaderBackground string `json:"header_background,omitempty"`
	Borders          string `json:"borders,omitempty"`
	Striped          bool   `json:"striped,omitempty"`
}

// TableCellObj is used for cells with col_span or row_span.
type TableCellObj struct {
	Content string `json:"content"`
	ColSpan int    `json:"col_span,omitempty"`
	RowSpan int    `json:"row_span,omitempty"`
}

// ChartInput mirrors ChartSpec (subset).
type ChartInput struct {
	Type         string         `json:"type"`
	Title        string         `json:"title,omitempty"`
	Data         map[string]any `json:"data"`
	DataOrder    []string       `json:"data_order,omitempty"`
	SeriesLabels []string       `json:"series_labels,omitempty"`
	Style        *ChartStyleInput `json:"style,omitempty"`
}

// ChartStyleInput mirrors types.ChartStyle for JSON generation.
type ChartStyleInput struct {
	Colors     []string `json:"colors,omitempty"`
	FontFamily string   `json:"font_family,omitempty"`
	ShowLegend bool     `json:"show_legend,omitempty"`
	ShowValues bool     `json:"show_values,omitempty"`
}

// DiagramInput mirrors DiagramSpec (subset).
type DiagramInput struct {
	Type  string            `json:"type"`
	Title string            `json:"title,omitempty"`
	Data  map[string]any    `json:"data"`
	Style *DiagramStyleInput `json:"style,omitempty"`
}

// DiagramStyleInput mirrors types.DiagramStyle for JSON generation.
type DiagramStyleInput struct {
	Colors     []string `json:"colors,omitempty"`
	FontFamily string   `json:"font_family,omitempty"`
	ShowLegend bool     `json:"show_legend,omitempty"`
	ShowValues bool     `json:"show_values,omitempty"`
}

// ImageInput mirrors ImageInput.
type ImageInput struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

// templates available in the templates/ directory.
var templates = []string{
	"template_2",
	"forest-green",
	"midnight-blue",
	"warm-coral",
}

// slideTypes weighted by approximate real-world usage.
var slideTypes = []string{
	"content", "content", "content",
	"two-column", "two-column",
	"chart", "chart",
	"diagram",
	"title",
	"section",
	"image",
}

// chartTypes for random chart generation.
var chartTypes = []string{
	"bar", "line", "pie", "donut", "area", "radar",
	"stacked_bar", "grouped_bar", "stacked_area",
	"waterfall", "funnel", "gauge", "treemap", "scatter", "bubble",
}

// diagramTypes for random diagram generation.
var diagramTypes = []string{
	"timeline", "process_flow", "matrix_2x2", "pyramid", "swot", "venn",
	"org_chart", "gantt", "kpi_dashboard", "heatmap", "fishbone", "pestel",
	"business_model_canvas", "value_chain", "nine_box_talent",
	"porters_five_forces", "house_diagram", "funnel_chart", "panel_layout",
}

// panelLayoutAliases used to occasionally test alias resolution.
var panelLayoutAliases = []string{
	"panel_layout", "icon_columns", "icon_rows", "stat_cards",
	"panel", "icon_panel", "number_tiles", "callout_cards",
}

// panelLayoutModes for the three layout modes.
var panelLayoutModes = []string{"columns", "rows", "stat_cards"}

// iconSVGDirSubdirs lists subdirectories under svg/ to sample icons from.
// Varying sets test different SVG structures, path complexity, and sizes.
var iconSVGDirSubdirs = []string{
	"material", "bootstrap", "ionic", "oct", "foundation",
	"entypo", "open", "typcn", "maki", "simple",
}

// edgeCaseStrings for stress testing.
var edgeCaseStrings = []string{
	"",
	strings.Repeat("A", 2500),
	"日本語テスト 中文测试 한국어",
	"🚀 📊 💡 🎯 ✅ ❌",
	"<script>alert('xss')</script>",
	"Line 1\nLine 2\nLine 3",
	`He said "hello" & she said 'goodbye'`,
	"RTL: مرحبا بالعالم",
	"Tab\there\tand\tthere",
	"   Leading and trailing spaces   ",
}

var (
	tableHeaderBGs = []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "none"}
	tableBorders   = []string{"all", "horizontal", "outer", "none"}
	alignments     = []string{"left", "center", "right"}
)

// Generator produces random PresentationInput decks from a seed.
type Generator struct {
	rng      *rand.Rand
	Seed     uint64
	svgIcons []string // inline SVG strings loaded from svg/ directory
}

// New creates a Generator with the given seed.
func New(seed uint64) *Generator {
	return &Generator{
		rng:  rand.New(rand.NewPCG(seed, seed^0xDEADBEEF)), //nolint:gosec // intentionally using math/rand for reproducible test data
		Seed: seed,
	}
}

// WithSVGDir loads inline SVG icons from the given directory (e.g. "svg/")
// for use as random icon values in process_flow, timeline, and panel_layout.
// Samples up to maxIcons files across subdirectories for variety.
func (g *Generator) WithSVGDir(svgDir string, maxIcons int) *Generator {
	if maxIcons <= 0 {
		maxIcons = 40
	}
	var icons []string
	for _, subdir := range iconSVGDirSubdirs {
		dir := filepath.Join(svgDir, subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		// Collect .svg files
		var svgFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".svg") {
				svgFiles = append(svgFiles, filepath.Join(dir, e.Name()))
			}
		}
		// Sample up to maxIcons/len(subdirs) from each subdir
		perDir := maxIcons / len(iconSVGDirSubdirs)
		if perDir < 2 {
			perDir = 2
		}
		// Use deterministic sampling based on seed
		for i := 0; i < perDir && i < len(svgFiles); i++ {
			idx := int(g.Seed+uint64(i)) % len(svgFiles)
			data, err := os.ReadFile(svgFiles[idx])
			if err != nil || len(data) > 4096 {
				continue // skip large or unreadable files
			}
			icons = append(icons, string(data))
		}
	}
	g.svgIcons = icons
	return g
}

// randomIcon returns a random inline SVG icon string, or empty string if none loaded.
func (g *Generator) randomIcon() string {
	if len(g.svgIcons) == 0 {
		return ""
	}
	return g.svgIcons[g.rng.IntN(len(g.svgIcons))]
}

// Generate produces a random PresentationInput.
func (g *Generator) Generate() *PresentationInput {
	template := templates[g.rng.IntN(len(templates))]
	slideCount := 1 + g.rng.IntN(30) // 1-30 slides

	p := &PresentationInput{
		Template:       template,
		OutputFilename: fmt.Sprintf("random_seed_%d.pptx", g.Seed),
		Slides:         make([]SlideInput, 0, slideCount),
	}

	// 30% chance of footer
	if g.rng.IntN(10) < 3 {
		p.Footer = &Footer{
			Enabled:  true,
			LeftText: g.maybeEdge(g.randomSentence()),
		}
	}

	// 20% chance of theme override
	if g.rng.IntN(10) < 2 {
		p.ThemeOverride = g.randomTheme()
	}

	// First slide is always a title slide
	p.Slides = append(p.Slides, g.titleSlide())

	// Remaining slides are random content
	for i := 1; i < slideCount; i++ {
		p.Slides = append(p.Slides, g.randomSlide())
	}

	return p
}

// GenerateJSON produces the JSON bytes for a random deck.
func (g *Generator) GenerateJSON() ([]byte, error) {
	p := g.Generate()
	return json.MarshalIndent(p, "", "  ")
}

func (g *Generator) titleSlide() SlideInput {
	return SlideInput{
		SlideType: "title",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
			g.textContent("subtitle", g.maybeEdge(g.randomSentence())),
		},
	}
}

func (g *Generator) randomSlide() SlideInput {
	st := slideTypes[g.rng.IntN(len(slideTypes))]

	switch st {
	case "content":
		return g.contentSlide()
	case "two-column":
		return g.twoColumnSlide()
	case "chart":
		return g.chartSlide()
	case "diagram":
		return g.diagramSlide()
	case "title":
		return g.sectionSlide()
	case "section":
		return g.sectionSlide()
	case "image":
		return g.imageSlide()
	default:
		return g.contentSlide()
	}
}

func (g *Generator) contentSlide() SlideInput {
	s := SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
		},
	}

	// Pick a body content type
	switch g.rng.IntN(4) {
	case 0: // bullets
		s.Content = append(s.Content, g.bulletsContent("body"))
	case 1: // body_and_bullets
		s.Content = append(s.Content, g.babContent("body"))
	case 2: // bullet_groups
		s.Content = append(s.Content, g.bgContent("body"))
	case 3: // table
		s.Content = append(s.Content, g.tableContent("body"))
	}

	// 20% chance of speaker notes
	if g.rng.IntN(5) == 0 {
		s.SpeakerNotes = g.randomSentence()
	}

	return s
}

func (g *Generator) twoColumnSlide() SlideInput {
	s := SlideInput{
		SlideType: "two-column",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
		},
	}

	// Left slot content
	s.Content = append(s.Content, g.randomSlotContent("slot1"))
	// Right slot content
	s.Content = append(s.Content, g.randomSlotContent("slot2"))

	return s
}

func (g *Generator) randomSlotContent(pid string) ContentInput {
	switch g.rng.IntN(5) {
	case 0:
		return g.bulletsContent(pid)
	case 1:
		return g.babContent(pid)
	case 2:
		return g.chartContentItem(pid)
	case 3:
		return g.diagramContentItem(pid)
	default:
		return g.tableContent(pid)
	}
}

func (g *Generator) chartSlide() SlideInput {
	return SlideInput{
		SlideType: "chart",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
			g.chartContentItem("body"),
		},
	}
}

func (g *Generator) diagramSlide() SlideInput {
	return SlideInput{
		SlideType: "diagram",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
			g.diagramContentItem("body"),
		},
	}
}

func (g *Generator) sectionSlide() SlideInput {
	return SlideInput{
		SlideType: "section",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
		},
	}
}

func (g *Generator) imageSlide() SlideInput {
	return SlideInput{
		SlideType: "image",
		Content: []ContentInput{
			g.textContent("title", g.maybeEdge(g.randomTitle())),
			{
				PlaceholderID: "body",
				Type:          "image",
				ImageValue: &ImageInput{
					Path: fmt.Sprintf("/images/random_%d.png", g.rng.IntN(100)),
					Alt:  g.randomSentence(),
				},
			},
		},
	}
}

// Content generators

func (g *Generator) textContent(pid, text string) ContentInput {
	return ContentInput{
		PlaceholderID: pid,
		Type:          "text",
		TextValue:     &text,
	}
}

func (g *Generator) bulletsContent(pid string) ContentInput {
	count := 2 + g.rng.IntN(7) // 2-8 bullets
	bullets := make([]string, count)
	for i := range bullets {
		bullets[i] = g.maybeEdge(g.randomBullet())
	}
	return ContentInput{
		PlaceholderID: pid,
		Type:          "bullets",
		BulletsValue:  &bullets,
	}
}

func (g *Generator) babContent(pid string) ContentInput {
	count := 2 + g.rng.IntN(5) // 2-6 bullets
	bullets := make([]string, count)
	for i := range bullets {
		bullets[i] = g.maybeEdge(g.randomBullet())
	}
	bab := &BABInput{
		Body:    g.maybeEdge(g.randomSentence()),
		Bullets: bullets,
	}
	// 40% chance of trailing body
	if g.rng.IntN(5) < 2 {
		bab.TrailingBody = g.randomSentence()
	}
	return ContentInput{
		PlaceholderID: pid,
		Type:          "body_and_bullets",
		BABValue:      bab,
	}
}

func (g *Generator) bgContent(pid string) ContentInput {
	groupCount := 2 + g.rng.IntN(4) // 2-5 groups
	groups := make([]GroupInput, groupCount)
	for i := range groups {
		bulletCount := 1 + g.rng.IntN(4)
		bullets := make([]string, bulletCount)
		for j := range bullets {
			bullets[j] = g.maybeEdge(g.randomBullet())
		}
		groups[i] = GroupInput{
			Header:  g.maybeEdge(g.randomTitle()),
			Bullets: bullets,
		}
		// 30% chance of group body
		if g.rng.IntN(10) < 3 {
			groups[i].Body = g.randomSentence()
		}
	}
	bg := &BGInput{
		Groups: groups,
	}
	// 40% chance of leading body
	if g.rng.IntN(5) < 2 {
		bg.Body = g.randomSentence()
	}
	// 30% chance of trailing body
	if g.rng.IntN(10) < 3 {
		bg.TrailingBody = g.randomSentence()
	}
	return ContentInput{
		PlaceholderID: pid,
		Type:          "bullet_groups",
		BGValue:       bg,
	}
}

func (g *Generator) tableContent(pid string) ContentInput {
	cols := 2 + g.rng.IntN(6)  // 2-7 columns
	rows := 1 + g.rng.IntN(10) // 1-10 rows

	headers := make([]string, cols)
	for i := range headers {
		headers[i] = g.maybeEdge(g.randomWord())
	}

	// 25% chance of using cell spans in the table
	useSpans := cols >= 3 && g.rng.IntN(4) == 0

	tableRows := make([][]interface{}, rows)
	for i := range tableRows {
		row := make([]interface{}, cols)
		for j := range row {
			if useSpans && g.rng.IntN(5) == 0 && j < cols-1 {
				// Cell with col_span=2
				row[j] = TableCellObj{
					Content: g.maybeEdge(g.randomCellValue()),
					ColSpan: 2,
				}
				// Skip next column (spanned)
				j++
				if j < cols {
					row[j] = "" // placeholder for spanned cell
				}
			} else {
				row[j] = g.maybeEdge(g.randomCellValue())
			}
		}
		tableRows[i] = row
	}

	style := &TableStyle{
		HeaderBackground: tableHeaderBGs[g.rng.IntN(len(tableHeaderBGs))],
		Borders:          tableBorders[g.rng.IntN(len(tableBorders))],
		Striped:          g.rng.IntN(2) == 0,
	}

	colAligns := make([]string, cols)
	for i := range colAligns {
		colAligns[i] = alignments[g.rng.IntN(len(alignments))]
	}

	return ContentInput{
		PlaceholderID: pid,
		Type:          "table",
		TableValue: &TableInput{
			Headers:          headers,
			Rows:             tableRows,
			Style:            style,
			ColumnAlignments: colAligns,
		},
	}
}

func (g *Generator) chartContentItem(pid string) ContentInput {
	ct := chartTypes[g.rng.IntN(len(chartTypes))]
	chart := &ChartInput{
		Type:  ct,
		Title: g.randomTitle(),
		Data:  g.chartData(ct),
	}

	// For multi-series types, add data_order and series_labels
	switch ct {
	case "stacked_bar", "grouped_bar", "stacked_area":
		keys := make([]string, 0)
		for k := range chart.Data {
			keys = append(keys, k)
		}
		chart.DataOrder = keys
		chart.SeriesLabels = []string{"Series A", "Series B", "Series C"}
	default:
		// 40% chance of data_order for simple charts too
		if g.rng.IntN(5) < 2 {
			keys := make([]string, 0)
			for k := range chart.Data {
				keys = append(keys, k)
			}
			chart.DataOrder = keys
		}
	}

	// 25% chance of style overrides
	if g.rng.IntN(4) == 0 {
		chart.Style = g.randomChartStyle()
	}

	return ContentInput{
		PlaceholderID: pid,
		Type:          "chart",
		ChartValue:    chart,
	}
}

func (g *Generator) diagramContentItem(pid string) ContentInput {
	dt := diagramTypes[g.rng.IntN(len(diagramTypes))]
	// For panel_layout, 50% chance of using an alias instead
	displayType := dt
	if dt == "panel_layout" && g.rng.IntN(2) == 0 {
		displayType = panelLayoutAliases[g.rng.IntN(len(panelLayoutAliases))]
	}
	diagram := &DiagramInput{
		Type:  displayType,
		Title: g.randomTitle(),
		Data:  g.diagramData(dt),
	}
	// 25% chance of style overrides
	if g.rng.IntN(4) == 0 {
		diagram.Style = g.randomDiagramStyle()
	}
	return ContentInput{
		PlaceholderID: pid,
		Type:          "diagram",
		DiagramValue:  diagram,
	}
}

// chartData generates appropriate data for a chart type.
func (g *Generator) chartData(chartType string) map[string]any {
	catCount := 3 + g.rng.IntN(6) // 3-8 categories
	categories := g.randomCategories(catCount)

	switch chartType {
	case "pie", "donut":
		data := make(map[string]any, catCount)
		for _, c := range categories {
			data[c] = g.randomPercent()
		}
		return data

	case "gauge":
		return map[string]any{
			"Score": float64(g.rng.IntN(100)),
		}

	case "waterfall":
		data := make(map[string]any, catCount)
		for _, c := range categories {
			v := float64(g.rng.IntN(200) - 50) // -50 to 149
			data[c] = v
		}
		return data

	case "funnel":
		data := make(map[string]any, catCount)
		base := 1000.0
		for _, c := range categories {
			data[c] = base
			base *= 0.5 + float64(g.rng.IntN(40))/100.0 // 50%-90% drop-off
		}
		return data

	case "treemap":
		data := make(map[string]any, catCount)
		for _, c := range categories {
			data[c] = float64(10 + g.rng.IntN(90))
		}
		return data

	case "scatter":
		points := make([]map[string]any, catCount)
		for i := range points {
			points[i] = map[string]any{
				"x": float64(g.rng.IntN(100)),
				"y": float64(g.rng.IntN(100)),
			}
		}
		return map[string]any{"Series A": points}

	case "bubble":
		points := make([]map[string]any, catCount)
		for i := range points {
			points[i] = map[string]any{
				"x":    float64(g.rng.IntN(100)),
				"y":    float64(g.rng.IntN(100)),
				"size": float64(5 + g.rng.IntN(45)),
			}
		}
		return map[string]any{"Bubbles": points}

	case "stacked_bar", "grouped_bar", "stacked_area":
		data := make(map[string]any, catCount)
		seriesCount := 2 + g.rng.IntN(2) // 2-3 series
		for _, c := range categories {
			values := make([]any, seriesCount)
			for j := range values {
				values[j] = float64(10 + g.rng.IntN(90))
			}
			data[c] = values
		}
		return data

	default: // bar, line, area, radar
		data := make(map[string]any, catCount)
		for _, c := range categories {
			data[c] = float64(10 + g.rng.IntN(90))
		}
		return data
	}
}

// diagramData generates appropriate data for a diagram type.
func (g *Generator) diagramData(diagType string) map[string]any { //nolint:gocognit,gocyclo
	switch diagType {
	case "timeline":
		phaseCount := 2 + g.rng.IntN(5)
		phases := make([]map[string]any, phaseCount)
		year := 2025
		month := 1
		for i := range phases {
			startMonth := month
			endMonth := startMonth + 1 + g.rng.IntN(4)
			if endMonth > 12 {
				year++
				endMonth = endMonth - 12
			}
			phases[i] = map[string]any{
				"name":  fmt.Sprintf("Phase %d: %s", i+1, g.randomWord()),
				"start": fmt.Sprintf("%d-%02d", year, startMonth),
				"end":   fmt.Sprintf("%d-%02d", year, endMonth),
			}
			// 40% chance of icon on timeline phase
			if g.rng.IntN(5) < 2 {
				if icon := g.randomIcon(); icon != "" {
					phases[i]["icon"] = icon
				}
			}
			month = endMonth + 1
			if month > 12 {
				month = 1
				year++
			}
		}
		return map[string]any{"phases": phases}

	case "process_flow":
		stepCount := 3 + g.rng.IntN(5)
		steps := make([]map[string]any, stepCount)
		for i := range steps {
			steps[i] = map[string]any{
				"label": g.randomWord(),
			}
			if g.rng.IntN(3) == 0 {
				steps[i]["type"] = "decision"
			}
			// 30% chance of icon on process_flow step
			if g.rng.IntN(10) < 3 {
				if icon := g.randomIcon(); icon != "" {
					steps[i]["icon"] = icon
				}
			}
		}
		return map[string]any{"steps": steps}

	case "matrix_2x2":
		return map[string]any{
			"x_axis": g.randomWord(),
			"y_axis": g.randomWord(),
			"quadrants": []map[string]any{
				{"label": g.randomWord(), "items": []string{g.randomBullet()}},
				{"label": g.randomWord(), "items": []string{g.randomBullet()}},
				{"label": g.randomWord(), "items": []string{g.randomBullet()}},
				{"label": g.randomWord(), "items": []string{g.randomBullet()}},
			},
		}

	case "pyramid":
		levelCount := 3 + g.rng.IntN(8) // 3-10 levels
		levels := make([]map[string]any, levelCount)
		for i := range levels {
			levels[i] = map[string]any{"label": g.randomWord()}
		}
		return map[string]any{"levels": levels}

	case "swot":
		return map[string]any{
			"strengths":     []string{g.randomBullet(), g.randomBullet()},
			"weaknesses":    []string{g.randomBullet(), g.randomBullet()},
			"opportunities": []string{g.randomBullet(), g.randomBullet()},
			"threats":       []string{g.randomBullet(), g.randomBullet()},
		}

	case "venn":
		circleCount := 2 + g.rng.IntN(2)
		circles := make([]map[string]any, circleCount)
		for i := range circles {
			circles[i] = map[string]any{
				"label": g.randomWord(),
				"items": []string{g.randomBullet()},
			}
		}
		return map[string]any{"circles": circles}

	case "org_chart":
		return map[string]any{
			"name":  g.randomWord(),
			"title": "CEO",
			"children": []map[string]any{
				{"name": g.randomWord(), "title": "VP Engineering"},
				{"name": g.randomWord(), "title": "VP Sales"},
			},
		}

	case "gantt":
		taskCount := 3 + g.rng.IntN(5)
		tasks := make([]map[string]any, taskCount)
		for i := range tasks {
			tasks[i] = map[string]any{
				"name":     fmt.Sprintf("Task %d", i+1),
				"start":    fmt.Sprintf("2026-%02d-01", 1+i),
				"end":      fmt.Sprintf("2026-%02d-28", 1+i),
				"progress": float64(g.rng.IntN(100)),
			}
		}
		return map[string]any{"tasks": tasks}

	case "kpi_dashboard":
		kpiCount := 3 + g.rng.IntN(4)
		kpis := make([]map[string]any, kpiCount)
		for i := range kpis {
			kpis[i] = map[string]any{
				"label":  g.randomWord(),
				"value":  float64(g.rng.IntN(1000)),
				"target": float64(500 + g.rng.IntN(500)),
			}
		}
		return map[string]any{"kpis": kpis}

	case "heatmap":
		rows := 3 + g.rng.IntN(3)
		cols := 3 + g.rng.IntN(3)
		rowLabels := make([]string, rows)
		colLabels := make([]string, cols)
		values := make([][]float64, rows)
		for i := range rowLabels {
			rowLabels[i] = g.randomWord()
			values[i] = make([]float64, cols)
			for j := range values[i] {
				values[i][j] = float64(g.rng.IntN(100))
			}
		}
		for i := range colLabels {
			colLabels[i] = g.randomWord()
		}
		return map[string]any{
			"row_labels": rowLabels,
			"col_labels": colLabels,
			"values":     values,
		}

	case "fishbone":
		categoryCount := 4 + g.rng.IntN(3)
		categories := make([]map[string]any, categoryCount)
		for i := range categories {
			causeCount := 2 + g.rng.IntN(3)
			causes := make([]string, causeCount)
			for j := range causes {
				causes[j] = g.randomBullet()
			}
			categories[i] = map[string]any{
				"name":   g.randomWord(),
				"causes": causes,
			}
		}
		return map[string]any{
			"effect":     g.randomWord(),
			"categories": categories,
		}

	case "pestel":
		return map[string]any{
			"political":      []string{g.randomBullet(), g.randomBullet()},
			"economic":       []string{g.randomBullet(), g.randomBullet()},
			"social":         []string{g.randomBullet(), g.randomBullet()},
			"technological":  []string{g.randomBullet(), g.randomBullet()},
			"environmental":  []string{g.randomBullet(), g.randomBullet()},
			"legal":          []string{g.randomBullet(), g.randomBullet()},
		}

	case "business_model_canvas":
		return map[string]any{
			"key_partners":    []string{g.randomBullet()},
			"key_activities":  []string{g.randomBullet()},
			"key_resources":   []string{g.randomBullet()},
			"value_propositions": []string{g.randomBullet()},
			"customer_relationships": []string{g.randomBullet()},
			"channels":        []string{g.randomBullet()},
			"customer_segments": []string{g.randomBullet()},
			"cost_structure":  []string{g.randomBullet()},
			"revenue_streams": []string{g.randomBullet()},
		}

	case "value_chain":
		return map[string]any{
			"primary": []map[string]any{
				{"name": "Inbound Logistics", "items": []string{g.randomBullet()}},
				{"name": "Operations", "items": []string{g.randomBullet()}},
				{"name": "Outbound Logistics", "items": []string{g.randomBullet()}},
				{"name": "Marketing", "items": []string{g.randomBullet()}},
				{"name": "Service", "items": []string{g.randomBullet()}},
			},
			"support": []map[string]any{
				{"name": "Infrastructure", "items": []string{g.randomBullet()}},
				{"name": "HR", "items": []string{g.randomBullet()}},
			},
		}

	case "nine_box_talent":
		count := 4 + g.rng.IntN(6)
		people := make([]map[string]any, count)
		for i := range people {
			people[i] = map[string]any{
				"name":        g.randomWord(),
				"performance": float64(1 + g.rng.IntN(3)),
				"potential":   float64(1 + g.rng.IntN(3)),
			}
		}
		return map[string]any{"people": people}

	case "porters_five_forces":
		return map[string]any{
			"rivalry":        []string{g.randomBullet()},
			"new_entrants":   []string{g.randomBullet()},
			"substitutes":    []string{g.randomBullet()},
			"buyer_power":    []string{g.randomBullet()},
			"supplier_power": []string{g.randomBullet()},
		}

	case "house_diagram":
		return map[string]any{
			"roof":       g.randomWord(),
			"pillars":    []string{g.randomWord(), g.randomWord(), g.randomWord()},
			"foundation": g.randomWord(),
		}

	case "funnel_chart":
		stageCount := 3 + g.rng.IntN(4)
		stages := make([]map[string]any, stageCount)
		val := 1000.0
		for i := range stages {
			stages[i] = map[string]any{
				"label": g.randomWord(),
				"value": val,
			}
			val *= 0.6 + float64(g.rng.IntN(20))/100.0
		}
		return map[string]any{"values": stages}

	case "panel_layout":
		return g.panelLayoutData()

	default:
		return map[string]any{
			"items": []string{g.randomBullet(), g.randomBullet()},
		}
	}
}

// panelLayoutData generates data for panel_layout with a random layout mode.
func (g *Generator) panelLayoutData() map[string]any {
	mode := panelLayoutModes[g.rng.IntN(len(panelLayoutModes))]
	panelCount := 3 + g.rng.IntN(4) // 3-6 panels

	panels := make([]map[string]any, panelCount)
	for i := range panels {
		panel := map[string]any{
			"title": g.randomWord(),
		}
		// Icon: 70% chance
		if g.rng.IntN(10) < 7 {
			if icon := g.randomIcon(); icon != "" {
				panel["icon"] = icon
			}
		}
		switch mode {
		case "stat_cards":
			panel["value"] = fmt.Sprintf("%d%%", 10+g.rng.IntN(90))
			panel["body"] = g.randomBullet()
		case "rows":
			panel["body"] = g.randomSentence()
		default: // columns
			panel["body"] = g.randomBullet() + "\n" + g.randomBullet()
		}
		panels[i] = panel
	}

	data := map[string]any{
		"layout": mode,
		"panels": panels,
	}

	// 30% chance of callout (mainly for stat_cards)
	if g.rng.IntN(10) < 3 {
		callout := map[string]any{
			"text": g.randomSentence(),
		}
		if g.rng.IntN(2) == 0 {
			if icon := g.randomIcon(); icon != "" {
				callout["icon"] = icon
			}
		}
		data["callout"] = callout
	}

	// 25% chance of footnote
	if g.rng.IntN(4) == 0 {
		data["footnote"] = g.randomSentence()
	}

	return data
}

// Text generators

var nouns = []string{
	"Revenue", "Growth", "Strategy", "Market", "Performance", "Analysis",
	"Pipeline", "Forecast", "Budget", "Innovation", "Operations", "Roadmap",
	"Framework", "Metrics", "Benchmark", "Stakeholder", "Synergy", "Portfolio",
	"Transformation", "Optimization", "Initiative", "Assessment", "Compliance",
}

var adjectives = []string{
	"Strategic", "Annual", "Quarterly", "Global", "Regional", "Key",
	"Digital", "Operational", "Financial", "Comprehensive", "Critical",
	"Sustainable", "Competitive", "Integrated", "Advanced", "Emerging",
}

var verbs = []string{
	"increased", "decreased", "improved", "achieved", "delivered",
	"launched", "expanded", "optimized", "transformed", "exceeded",
}

func (g *Generator) randomWord() string {
	return nouns[g.rng.IntN(len(nouns))]
}

func (g *Generator) randomTitle() string {
	adj := adjectives[g.rng.IntN(len(adjectives))]
	noun := nouns[g.rng.IntN(len(nouns))]
	return adj + " " + noun
}

func (g *Generator) randomSentence() string {
	noun := nouns[g.rng.IntN(len(nouns))]
	verb := verbs[g.rng.IntN(len(verbs))]
	adj := adjectives[g.rng.IntN(len(adjectives))]
	return fmt.Sprintf("The %s %s %s targets by 15%%", noun, verb, adj)
}

func (g *Generator) randomBullet() string {
	noun := nouns[g.rng.IntN(len(nouns))]
	verb := verbs[g.rng.IntN(len(verbs))]
	pct := 5 + g.rng.IntN(50)
	return fmt.Sprintf("%s %s by %d%% year-over-year", noun, verb, pct)
}

func (g *Generator) randomCellValue() string {
	switch g.rng.IntN(3) {
	case 0:
		return fmt.Sprintf("$%d.%dM", g.rng.IntN(100), g.rng.IntN(10))
	case 1:
		return fmt.Sprintf("%d%%", g.rng.IntN(100))
	default:
		return g.randomWord()
	}
}

func (g *Generator) randomCategories(n int) []string {
	prefixes := []string{"Q1", "Q2", "Q3", "Q4", "Jan", "Feb", "Mar", "Apr",
		"May", "Jun", "North", "South", "East", "West", "APAC", "EMEA", "LATAM",
		"Product A", "Product B", "Product C", "Enterprise", "SMB", "Mid-Market"}
	cats := make([]string, n)
	used := make(map[string]bool)
	for i := range cats {
		c := prefixes[g.rng.IntN(len(prefixes))]
		if used[c] {
			// If collision, append a number
			c = fmt.Sprintf("%s %d", c, g.rng.IntN(100))
		}
		cats[i] = c
		used[c] = true
	}
	return cats
}

func (g *Generator) randomPercent() float64 {
	return float64(5+g.rng.IntN(40)) + float64(g.rng.IntN(100))/100.0
}

// maybeEdge has a 10% chance of replacing text with an edge case string.
func (g *Generator) maybeEdge(normal string) string {
	if g.rng.IntN(10) == 0 {
		return edgeCaseStrings[g.rng.IntN(len(edgeCaseStrings))]
	}
	return normal
}

func (g *Generator) randomChartStyle() *ChartStyleInput {
	s := &ChartStyleInput{}
	// 50% chance of custom colors
	if g.rng.IntN(2) == 0 {
		colorCount := 3 + g.rng.IntN(3)
		s.Colors = make([]string, colorCount)
		for i := range s.Colors {
			s.Colors[i] = fmt.Sprintf("#%02x%02x%02x", g.rng.IntN(256), g.rng.IntN(256), g.rng.IntN(256))
		}
	}
	// 30% chance of font_family
	if g.rng.IntN(10) < 3 {
		fonts := []string{"Arial", "Helvetica", "Georgia", "Verdana", "Calibri"}
		s.FontFamily = fonts[g.rng.IntN(len(fonts))]
	}
	s.ShowLegend = g.rng.IntN(2) == 0
	s.ShowValues = g.rng.IntN(2) == 0
	return s
}

func (g *Generator) randomDiagramStyle() *DiagramStyleInput {
	s := &DiagramStyleInput{}
	// 50% chance of custom colors
	if g.rng.IntN(2) == 0 {
		colorCount := 3 + g.rng.IntN(3)
		s.Colors = make([]string, colorCount)
		for i := range s.Colors {
			s.Colors[i] = fmt.Sprintf("#%02x%02x%02x", g.rng.IntN(256), g.rng.IntN(256), g.rng.IntN(256))
		}
	}
	// 30% chance of font_family
	if g.rng.IntN(10) < 3 {
		fonts := []string{"Arial", "Helvetica", "Georgia", "Verdana", "Calibri"}
		s.FontFamily = fonts[g.rng.IntN(len(fonts))]
	}
	s.ShowLegend = g.rng.IntN(2) == 0
	s.ShowValues = g.rng.IntN(2) == 0
	return s
}

func (g *Generator) randomTheme() *ThemeInput {
	colors := make(map[string]string)
	colorKeys := []string{"accent1", "accent2", "dk1", "lt1"}
	for _, k := range colorKeys {
		if g.rng.IntN(3) == 0 {
			colors[k] = fmt.Sprintf("#%02x%02x%02x", g.rng.IntN(256), g.rng.IntN(256), g.rng.IntN(256))
		}
	}
	fonts := []string{"Arial", "Helvetica", "Calibri", "Times New Roman", "Georgia", "Verdana"}
	return &ThemeInput{
		Colors:    colors,
		TitleFont: fonts[g.rng.IntN(len(fonts))],
		BodyFont:  fonts[g.rng.IntN(len(fonts))],
	}
}
