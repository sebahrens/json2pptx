# Visual Inspection Criteria

After running `TEST_MODE=all ./scripts/e2e_visual_test.sh`, review slide images in `test_output/<template>/<template>-slide-*.jpg` against `test_output/<template>/test_content.md`.

## Quality Checklist

**Basic Quality:**
1. No Missing Content — all input text/bullets present
2. Readable Text — font size appropriate, no truncation
3. Proper Alignment — content centered/aligned correctly
4. Theme Consistency — colors/fonts match template
5. No Overflow — content fits within slide bounds

**Charts & Diagrams:**
6. Chart Accuracy — data values/labels match input
7. Chart Aspect Ratios — pie/donut circular, bar charts readable
8. SVG/Diagram Rendering — no artifacts, correct theme colors
9. Diagram Labels — readable, no overlapping text

**Multi-Content & Slots:**
10. Slot Routing — `::slot1::`/`::slot2::` in correct placeholders
11. Two-Column Balance — columns equal width, content balanced
12. Text + Chart Combos — both text AND chart visible side-by-side
13. No Empty Placeholders — every slot renders visibly

**Tables:**
14. Table Structure — rows/columns aligned, headers styled
15. Table Data — all cells populated

**Complex Slide Verification:**
16. Multi-Content Presence — both text AND visual elements present on same slide (no missing half)
17. Aspect Ratio Integrity — circular charts (pie/donut) maintain 1:1 ratio even in half-width containers
18. Visual Balance — content distributed evenly between columns (no huge whitespace on one side)
19. Diagram Readability — labels on diagrams are legible at rendered size (not too small to read)
20. Data Integrity — chart data values match the input specification exactly (correct numbers, correct labels)
21. Overlap Check — no text overlapping with chart/diagram boundaries
22. Placeholder Utilization — all available placeholders in the layout are used (no empty placeholders visible)
23. Consulting Standard — slide quality acceptable for professional consulting presentation (McKinsey/BCG level)

## Issue Creation

For any defect:
```bash
bd create --title="Visual: [description]" --type=bug --priority=2 \
  --description="Template: [name], Slide: [number], Issue: [details]"
```

Do NOT fix visual bugs in the current iteration. Create beads and exit so the next iteration gets fresh context.
