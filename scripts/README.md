# Scripts

Utility scripts for the Go Slide Creator project.

## Development Loop

### loop.sh

Automated development loop runner.

```bash
./scripts/loop.sh           # Run build loop
./scripts/loop.sh plan      # Run planning loop
./scripts/loop.sh 5         # Run build loop for max 5 iterations
./scripts/loop.sh plan 3    # Run planning loop for max 3 iterations
```

## Testing

### e2e_visual_test.sh

End-to-end visual testing against all templates (random / stress decks).

```bash
TEST_MODE=all ./scripts/e2e_visual_test.sh
```

### test_template_visual_qa.sh

Per-template visual QA driver. Renders the fixed reference deck at
`examples/template-qa-deck.json` (7 mandatory layout roles + 3 representative
patterns: shape_grid, comparison-2col, journey-maturity-model) against every
`templates/*.pptx`, converts to JPGs via `cmd/pptx2jpg`, and writes a
`REPORT.md` skeleton under `output/visual-qa/<template>/` that the
`slide-visual-qa` skill (Haiku subagent) fills in with per-slide findings.

```bash
./scripts/test_template_visual_qa.sh                      # all templates
TEMPLATE=midnight-blue ./scripts/test_template_visual_qa.sh   # one template
```

Output:

- `output/visual-qa/<template>/REPORT.md` — embedded screenshots + file:line
  refs into `deck.json`, placeholders for `slide-visual-qa` findings and
  `analyze_deck_rhythm` composition output, plus a maintainer-review
  checklist that must be ticked before any template ships.
- `output/visual-qa/<template>/template-check.json` — JSON conformance
  output consumed by the subagent's "Template Layout Review" pass.

### run_tests.sh

Run the Go test suite with coverage reporting.

## Utilities

### check-licenses.sh

Check third-party license compliance.

### check_lines.sh

Count lines in the beads issues file (for monitoring issue database size).
