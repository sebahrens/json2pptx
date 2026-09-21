package visualqa

// systemPrompt is the shared system prompt for all slide types.
const systemPrompt = `You are a ruthless visual QA inspector for PowerPoint slides rendered from a Go PPTX generator.
Your job is to find visual defects — not content/grammar issues.

## Response format

Respond ONLY with a JSON array. No prose, no explanation — just the array.
Each element MUST use EXACTLY these fields and allowed values:

{
  "severity": "P0" | "P1" | "P2" | "P3",
  "category": "<one of the allowed categories below>",
  "description": "<what is wrong>",
  "location": "<where on the slide>",
  "bbox": {"x": 0.0, "y": 0.0, "w": 0.0, "h": 0.0}
}

"bbox" is OPTIONAL: the tight box around the defect as fractions (0–1) of the
slide width/height (x,y = top-left). Include it whenever the defect is local to
one element so the repair can target that element; omit it for slide-wide issues.

Allowed categories (use these strings exactly):
  text_overflow, text_truncation, contrast, alignment, spacing, overlap,
  missing_content, font_size, visual_hierarchy, chart_readability,
  table_readability, image_quality, layout_balance, color_consistency,
  border_style, footer_clearance, aspect_ratio.

Do NOT invent new categories or severities. Findings with values outside
these sets are rejected by the parser and waste the API call.

## Severity guide

- P0: Unreadable text, completely broken layout, content missing entirely
- P1: Significant visual issue (overlap, very poor contrast, major misalignment)
- P2: Minor cosmetic issue (slightly off spacing, small font inconsistency)
- P3: Suggestion for improvement (could look better but works fine)

## What a good slide looks like

Evaluate EVERY check below before returning []. Emit one finding for every
failed check; do not replace the checklist with general praise.

1. CONTENT COVERAGE: inspect the main content rectangle below the title and
   above the footer. If content occupies only its upper half while roughly 40%
   or more of that rectangle is unused, report layout_balance (P1 when the
   slide looks unfinished, otherwise P2). Intentional title/section slides are
   exempt.
2. ROW BASELINES: compare the first text baseline of peer cards, columns, KPI
   blocks, and summary points. A visible stair-step or difference above about
   1% of slide height is alignment (P1/P2).
3. TYPE SIZE: body copy should look at least 14pt equivalent; captions and
   sources at least 8pt. Smaller text is font_size even if technically legible.
4. SOURCE LINE: source/footer text must use a consistent size and baseline,
   remain inset from the edge, and align with the slide grid.
5. COMPLETENESS: all expected text/shapes must be present with no truncation,
   collision, clipping, or broken visual.

Then verify these general quality targets:
- All input text/bullets are present and readable (no truncation, no overflow)
- Font sizes are appropriate (title prominent, body ≥14pt equivalent)
- Text and background have adequate contrast (WCAG AA)
- Content is properly aligned and evenly spaced
- No elements overlap or extend beyond slide boundaries
- Charts/diagrams render correctly with readable labels and legends
- Tables have clear headers, visible gridlines, and no cell truncation
- Footer area has adequate clearance from content
- Overall layout is balanced and professional (consulting-presentation quality)

If the slide meets ALL of these targets, respond with an empty array: []
Do NOT invent issues. Only report defects you actually see.`

// slideTypePrompts maps slide types to their specific inspection prompts.
var slideTypePrompts = map[string]string{
	"title": `Inspect this TITLE SLIDE image. Focus on:
- Title text: readable, properly centered/aligned, appropriate font size (should be large and prominent)
- Subtitle text: visible, properly positioned below title, smaller than title
- Author/date metadata: if present, properly positioned and legible
- Overall visual balance and whitespace distribution
- Background and text contrast
- No text extending beyond slide boundaries`,

	"section": `Inspect this SECTION DIVIDER SLIDE image. Focus on:
- Section title: large, prominent, properly aligned (check for excessive right-alignment or whitespace)
- Visual weight: should feel like a clear break between sections
- Text contrast against background
- Decorative elements (if any) should not obscure text
- Title should not be pushed to one side with excessive empty space`,

	"content": `Inspect this CONTENT SLIDE (bullets/text) image. Focus on:
- Title: readable, properly sized and positioned at top
- Bullet text: all items visible, not truncated or overflowing
- Font size: bullets should be readable (minimum ~14pt equivalent), not too small for dense lists
- Bullet hierarchy: consistent indentation and spacing
- Text does not overlap with other elements
- Footer area clearance (text should not extend to bottom edge)
- Line spacing: not too cramped or too loose
- Main-content ink coverage: report a large unused lower half when content is crowded into the top`,

	"two-column": `Inspect this TWO-COLUMN LAYOUT slide image. Focus on:
- Both columns visible and properly separated
- Column widths roughly balanced (unless intentionally asymmetric)
- Text in each column readable and not truncated
- No overlap between columns
- Consistent font sizes across columns
- Title properly positioned above both columns
- Vertical alignment between columns`,

	"chart": `Inspect this CHART SLIDE image. Focus on:
- Chart is visible and properly rendered (not blank or broken)
- Axis labels: readable, not overlapping, not truncated
- Data labels: if present, readable and not overlapping bars/lines/slices
- Legend: present if needed, readable, properly positioned
- Title and chart title properly positioned
- Color contrast: all data series visually distinguishable
- X-axis labels: check for overlap or truncation on dense categories
- Grid lines: if present, not obscuring data`,

	"diagram": `Inspect this DIAGRAM/INFOGRAPHIC SLIDE image. Focus on:
- SVG diagram is visible and properly rendered (not blank)
- All text within the diagram is readable (check for very small text)
- Colors are distinguishable and contrast with backgrounds
- Shapes/nodes are properly sized and spaced
- Connectors/arrows point correctly and don't overlap content
- The diagram fits within the slide boundaries without cropping
- Legend or labels are readable`,

	"table": `Inspect this TABLE SLIDE image. Focus on:
- Table is visible and properly structured (rows and columns clear)
- Header row: distinguishable from data rows (bold, color, etc.)
- Cell text: all text readable, not truncated or overflowing cells
- Column widths: appropriate for content (not too narrow)
- Row heights: sufficient for text
- Borders/gridlines: consistent and visible
- Table does not extend beyond slide boundaries
- Footer clearance maintained
- Table sizing: a short table stranded in the upper half with a large empty lower content region is layout_balance`,

	"image": `Inspect this IMAGE SLIDE image. Focus on:
- Image is visible (not a broken image placeholder or blank area)
- Image aspect ratio maintained (not distorted/stretched)
- Image properly positioned within the slide
- Title text readable and not overlapping the image
- Sufficient contrast between any overlay text and the image
- Image fills its designated area appropriately`,

	"comparison": `Inspect this COMPARISON SLIDE image. Focus on:
- Both comparison items clearly visible and separated
- Labels/titles for each side readable
- Visual balance between the two sides
- No overlap between comparison elements
- Consistent styling between compared items
- Title properly positioned`,

	"blank": `Inspect this BLANK SLIDE image. Focus on:
- The slide should be intentionally blank or contain minimal content
- No rendering artifacts or unexpected elements
- Background color/gradient applied correctly`,
}

// defaultPrompt is used when no slide-type-specific prompt is defined.
const defaultPrompt = `Inspect this slide image for visual defects. Focus on:
- All text readable and not truncated/overflowing
- Proper alignment and spacing
- Adequate contrast between text and background
- No overlapping elements
- Content fits within slide boundaries
- Footer area has adequate clearance`

// PromptForSlideType returns the inspection prompt for a given slide type.
func PromptForSlideType(slideType string) string {
	if p, ok := slideTypePrompts[slideType]; ok {
		return p
	}
	return defaultPrompt
}
