---
name: template-deck
description: >
  Reference for choosing and using json2pptx PowerPoint templates: template
  selection, layout capabilities, layout-to-slide-type mapping, and portable
  placeholder IDs (title, subtitle, body, body_2). Use when picking a template,
  targeting a specific layout_id or placeholder, or checking what a template's
  layouts support. For the end-to-end deck workflow, use the generate-deck skill.
---

# Template Deck

Read [TEMPLATE_GUIDE.md](TEMPLATE_GUIDE.md) — the canonical reference for
*using* a template from JSON: discovery via `list_templates` and
`recommend_visual`, layout tags, the layout→slide-type mapping, and placeholder
naming.

Quick start:

1. Call `list_templates` to see the shipped templates, their role bindings, and
   layout thumbnails.
2. Call `recommend_visual` with the chosen `template` to rank layouts and
   patterns for each slide's intent.
3. Address placeholders with the portable IDs `title`, `subtitle`, `body`, and
   `body_2`; they resolve on every conforming template.

For the full plan → render → repair workflow, see the generate-deck skill
([../generate-deck/SKILL.md](../generate-deck/SKILL.md)).
