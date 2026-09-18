# Template-portable slide quality: review and implementation design

Reviewed 2026-09-18 against commit `2c219bf`. Implementation is tracked in Beads epic `go-slide-creator-fh57`; this document is the technical rationale, not a second task-status tracker.

## Conclusion and scope

The agent is being asked to design visually while the default workflow primarily rewards schema validity, successful OOXML generation and text-fit checks. Worse, a reproducible layout-binding defect can destroy a sensible composition after the agent has chosen it. More elaborate prompting cannot repair that hidden mismatch.

The supported input is a user-provided PPTX with the seven canonical roles: title, one content, two content, section, blank, blank with title, closing. This is not a project to infer or manufacture missing layouts in arbitrary decks. Canonical roles alone do not guarantee usable geometry, available fonts, clear decorative intent or a particular aspect ratio. The contract must resolve these properties, diagnose ambiguity and report limitations instead of silently pretending every supplied template is visually supported.

The intended outcome is template-faithful, readable, fact-preserving slides with explicit evidence of rendered review. It is not a mathematical guarantee of attractive slides for every brief. Excessively dense content, an intrinsically unreadable template or missing rendering resources must produce a useful draft plus actionable limits, not a false approval.

## Evidence and corrections to the initial audit

### 1. A pipeline defect demonstrably collapses KPI layout

`internal/semantic/slides/kpi.go:49` emits a content slide containing a title and a KPI pattern without binding a layout. In `cmd/json2pptx/json_mode.go`, automatic layout selection runs before pattern expansion. `jsonSlideToDefinition` only infers diagram type from a pattern when slide_type is absent; semantic compilation explicitly sets content. The suitability check in `internal/layout/heuristic.go` therefore treats the title-only content definition as eligible for inappropriate layouts. Variety/heuristic scoring can select Section Divider.

`cmd/json2pptx/shape_grid.go:firstBodyOrContentBounds` accepts the first body/content placeholder, including a section-number frame. The selected section geometry becomes the cards' content area.

Reproduction:

```sh
go build -o /tmp/json2pptx-audit ./cmd/json2pptx
/tmp/json2pptx-audit semantic render --spec examples/semantic/qbr.yaml \
  --templates-dir templates --output /tmp/json2pptx-audit-qbr.pptx
```

At the reviewed baseline the six-slide deck generated successfully; slide 3 selected Section Divider. Rendering with the repository's pptx2jpg converter and LibreOffice showed the four KPI cards squeezed into the bottom-right corner, with values broken across lines and most of the slide empty.

Controlled intervention: compile the same example to raw JSON, set only slide 3's `layout_id` to `blank-title`, regenerate and render. The cards occupy the main canvas and the numbers remain on single lines. This confirms the binding/geometry path as the immediate cause, rather than merely guessing that the agent chose poor font sizes. It does not prove the corrected composition is aesthetically optimal: the cards remain very tall, for example.

Session artifacts (temporary, not durable repository fixtures):

- Original: `/tmp/json2pptx-audit-qbr.pptx`, `/tmp/json2pptx-audit-qbr-images/`.
- Controlled source/output: `/tmp/qbr-review-pinned.json`, `/tmp/qbr-review-pinned.pptx`, `/tmp/qbr-review-pinned-images/`.
- The two image sets were rendered at different pixel densities; the conclusion concerns relative layout and wrapping, not pixel-diff scores.

### 2. Several different scores are being mistaken for design quality

The example's reported `quality.score=1.0` comes from `computeQualityScore` in `cmd/json2pptx/json_mode.go:1718`, called by semantic rendering. It checks simple input properties such as title length and bullet counts. It is NOT the separate MCP `score_deck` score.

`cmd/json2pptx/mcp_score_deck.go` generates a PPTX and collects deterministic findings, then calculates 100 minus severity-weighted penalties. This is useful and includes generation-time effects; it does not inspect slide pixels. OOXML structural validation is another distinct assurance. None independently establishes readability, composition or visual hierarchy.

There are already safeguards for incomplete generation evidence and requested-versus-actual visual modes. Preserve and unify them rather than claiming this capability is wholly absent. The remaining problem is inconsistent meanings across entrypoints and a default finish condition that can accept an ugly deck.

### 3. Seven layouts are not a complete rendering contract

`internal/template/theme.go:ParseTheme` selects the first theme file. That is not sufficient for a template whose selected layouts belong to different masters/themes. Existing inheritance and font resolvers should be extended and tested through actual relationship chains, not replaced wholesale.

`internal/generator/takeaway_note.go` and `source_note.go` contain fixed positions and widths, including takeaway Y=6,200,000 EMU and source Y=6,607,200 EMU. A seven-layout template can have a different canvas. Template-relative pattern bounds do not by themselves fix independently generated chrome.

`docs/TEMPLATE_SPEC.md` also contains objectively inconsistent units: 9,144,000 × 6,858,000 is 4:3, not 16:9; font size parentheticals labeled half-points do not match their stated point sizes. Correct the specification as part of the contract work.

Multi-master rendering and all alternative aspect ratios were not exhaustively rendered in this review. These are source-supported portability risks requiring fixtures, not claims that every affected template has been observed failing.

### 4. Semantic compilation can discard information while staying valid

Inspect `internal/semantic/ir.go` and `internal/semantic/slides/{visual,chart,kpi}.go`. Kinds map to a small fixed pattern set. Process descriptions can disappear when labels are also supplied; an unsupported chart/insights combination can fall back to content without preserving the chart. Some KPI/over-capacity cases degrade to bullets. A valid fallback is not necessarily a faithful communication of the original intent.

The fix is not unlimited agent-generated geometry. It is a bounded set of explicit composition alternatives, source-linked diagnostics and fact-preserving splitting when a composition cannot hold the content.

### 5. Visual repair has a targeting and parameter gap

`internal/visualqa/repair_map.go` maps categories to fix kinds, but handlers require concrete inputs: cell_path/max_chars, rows/columns, layout_id or from/to colors. Pattern values may be arrays while grid repair expects a different representation. A repair name without an executable target is not a reliable improvement loop.

`cmd/json2pptx/mcp_visual_qa.go` also constructs content-type slide records and automatically acts on P0/P1 findings, leaving lower-severity design problems advisory. Advisory is appropriate in some cases, but completion must not imply that those problems have been fixed. Actual semantic roles and unresolved visual findings must survive into the final evidence.

### 6. The workflow teaches agents to stop before seeing their work

`skills/generate-deck/SKILL.md` contains deterministic completion instructions that discourage thumbnail inspection, alongside later guidance treating images as truth and optional visual inspection tools. This is a policy conflict for a visual task. Large reference material and many patterns also increase discovery burden, but instruction length alone is not the root cause.

A provider-backed visual path already exists. The missing default protocol should also allow the host agent or a human to inspect rendered images, without requiring another specific vendor account. No provider call should be enabled without configured consent.

### 7. Existing quality tests do not measure the full claim

`tests/quality/quality_test.go:274–340` ignores some command errors and decodes a legacy finding shape. `tests/quality/loop_driver.go:121` demonstrates the newer nested envelope. The old collector can therefore report empty or misleading metrics. Some regression handling logs rather than enforcing failure.

Fixtures exercise pre-authored inputs. They do not establish that a fresh agent can understand a brief, choose compositions, use a supplied template, inspect its work and deliver a good deck. Repairing the measurement harness is foundational, not optional cleanup.

## How a human designer's process differs

This is a design model to test, not a claim that user research was conducted.

| Human action | Current agent obstacle | Required system support |
| --- | --- | --- |
| Decide audience, viewing conditions and one message per slide | Semantic kind is treated as a complete design decision | Preserve intent and offer a few justified composition choices |
| Inspect template examples and master layouts | Names/placeholder IDs do not show visual character | Real template thumbnails and resolved style/geometry profile |
| Choose a compatible layout before arranging evidence | Later heuristics can override the intended canvas | One stable role binding shared by plan and render |
| Establish hierarchy and edit density | Fitting is rewarded even when text becomes too small | Viewing-mode readability policy and fact-preserving splitting |
| Look at actual slides and the whole deck | Deterministic scores can be the stopping condition | All-slide pixels plus contact-sheet review |
| Revise locally and compare the result | Repairs lack targets, provenance or usable parameters | Source-grounded edits with revision-bound evidence |

PowerPoint masters determine layout-specific arrangement and styling; copying a theme name is insufficient. See [Microsoft's explanation of slide masters](https://support.microsoft.com/en-us/powerpoint/training/what-is-a-slide-master-in-powerpoint). For live presentations, Microsoft's accessibility guidance recommends larger text, including 18pt or more, and sufficient whitespace. Treat that as a viewing-policy starting point, not a universal prohibition on smaller report labels: [PowerPoint accessibility guidance](https://support.microsoft.com/en-us/accessibility/powerpoint/make-your-powerpoint-presentations-accessible-to-people-with-disabilities).

Humans do not normally finish because an XML validator reports success. Equally, a vision model is not an infallible art director. The system needs complementary deterministic checks, pixel evidence and explicit judgment.

## Target architecture and responsibility boundaries

```text
Brief + semantic/raw source
    -> inspect supplied template -> resolved per-layout profile
    -> select intent/composition -> bind canonical role
    -> shared geometry + typography -> compile/generate
    -> structural/fit checks -> render every slide
    -> inspect slide images + deck overview
    -> source-grounded repair -> regenerate/reinspect changed revision
    -> deliver source, PPTX, evidence and unresolved limitations
```

The agent owns communication judgment: audience, message, evidence selection, composition choice and proposed copy changes. Deterministic code owns canonical binding, relationships, geometry, text policy enforcement, data preservation and artifact provenance. Pixel inspection owns observations about what actually rendered; it must not fabricate content changes.

Reuse existing template metadata, canonical resolvers, pattern registry, source maps, preview tools and repair machinery. Add fields/services only where multiple real consumers need the same contract. No new workspace database, general-purpose agent framework, UI or complete renderer rewrite is required.

### Template profile and layout frame

A template-content-hashed profile contains actual slide dimensions, canonical-role bindings, per-layout master/theme relationships, effective inherited transforms/styles and diagnostics. Optional explicit safe-area annotations are an escape hatch for ambiguous decorative intent, not a new mandatory eighth layout.

A single frame calculation feeds planning, fit validation and emission. It reserves title, content, takeaway, source and footer regions and respects foreground decorations. Full-slide backgrounds must not be mistaken for obstacles. Do not fill all whitespace simply because it exists; whitespace is a composition choice, whereas a tiny accidental content frame is a defect.

### Typography and content

Resolve typography roles from template styles plus viewing mode. Preserve title/body/caption hierarchy. Fit using the same font, weight, spacing and insets that are emitted, and disclose substitutions. No fixed minimum makes every template good: if template fidelity conflicts with readability, expose the conflict rather than silently violating either.

Prefer a different composition or split before shrinking or rewriting. Never silently remove chart data, metric units, qualifiers, negations, sources or process descriptions. Meaning-changing edits require review.

### Evidence and durable source

Keep authoring source as the editable source of truth; PPTX extraction is not a lossless replacement. A versioned filesystem manifest links stable slide IDs, source maps, template hash, compiled input, PPTX, render images, renderer/fonts and findings.

Separate schema validity, successful generation, fit checks, pixel rendering and visual inspection. A draft can be delivered without a renderer; a visually reviewed claim cannot. Review coverage binds to the exact current revision. Replacing a template at the same path invalidates its cached profile and dependent evidence.

### Inspection and repair

Support host-agent/manual inspection and configured vision adapters through one evidence contract. Inspect every slide and the deck overview; record readability, hierarchy, clipping, contrast, fidelity and content completeness separately. Provider-less image heuristics are not visual design approval.

Repairs carry target path/ID, revision preconditions, typed parameters and expected effect. If a proposal cannot be applied safely, return the reason. Re-render after changes and inspect the resulting revision. Bound iterations, preserve the best known state, and report unresolved defects instead of oscillating forever.

## Verification and rollout

First fix the known layout regression and the unreliable metrics. In parallel establish the template profile and source/evidence contract. Then unify geometry and typography, preserve semantic meaning, and connect inspection/repair. Only then switch onboarding and advertise a stronger completion guarantee.

Automated portability tests cover bundled templates plus independent seven-role fixtures: 4:3/16:9/ultrawide, reordered layouts, inherited geometry, multiple masters, dark backgrounds, logos, missing fonts, and dense/sparse/multilingual inputs. Assert semantic preservation, bounds and forbidden-region separation. Pixel regression jobs pin renderer/fonts and retain artifacts; tolerant image checks supplement rather than replace geometry/content assertions. Baseline changes require review.

A separate real-agent benchmark tests at least 12 briefs, 3 template families including a held-out template, and two repeated runs per configuration. Two reviewers blindly rate paired baseline/new outputs. Proposed release target: at least 80% usable without manual slide editing, zero lost critical facts, and clear paired improvement. This is a proposed acceptance bar, not an observed result. Record model, tool calls, iteration count, cost and reviewer disagreement. Do not claim the original “most of the time” frequency is statistically established by this one reproduced example.

### Adversarial checks on this plan

- Seven named layouts can still contain unusable geometry. Validate effective properties and return specific limitations; do not silently narrow support to bundled templates.
- The renderer can disagree with PowerPoint. Record renderer provenance and perform representative PowerPoint checks before claiming PowerPoint-specific fidelity.
- Pixel QA can miss incorrect numbers. Semantic preservation checks remain mandatory and independent.
- A profile/cache can become stale. Use content hashes, and test replacement at the same path.
- A stronger gate can make offline drafting unusable. Preserve explicit draft mode; only strengthen claims of visual approval.
- A larger pattern catalog can increase confusion. Start with a small, intent-filtered palette rather than more patterns.
- A perfect metric can coexist with ugly decks. Human-rated fresh-agent evaluation remains the release test.
- The simplest alternative—pin the correct layout—already fixes the immediate KPI defect. Ship that regression fix early rather than waiting for the full redesign.
- Vision-only repair and prompt-only redesign were considered; neither addresses wrong geometry, dropped content or stale evidence. Conversely, deterministic checks should not be discarded merely because they do not judge aesthetics.

## Implementation issue map

Each child Bead includes the what, exact starting code locations, implementation approach and verification criteria. Code and its tests are consolidated in the same deliverable. Dependencies encode interface prerequisites, not a requirement to serialize all work.

| Bead | Deliverable | Prerequisites |
| --- | --- | --- |
| `go-slide-creator-fh57.1` | Make quality regression collection fail closed and parse current envelopes | None |
| `go-slide-creator-fh57.2` | Bind semantic compositions to compatible canonical layouts before scoring | None |
| `go-slide-creator-fh57.3` | Resolve a validated per-layout template profile through actual master relationships | None |
| `go-slide-creator-fh57.4` | Use one template-relative safe-area model for content and slide chrome | `go-slide-creator-fh57.3`, `go-slide-creator-fh57.2` |
| `go-slide-creator-fh57.5` | Unify readable typography policy and effective text measurement | `go-slide-creator-fh57.3`, `go-slide-creator-fh57.4` |
| `go-slide-creator-fh57.6` | Preserve semantic content and expose bounded composition alternatives | `go-slide-creator-fh57.2` |
| `go-slide-creator-fh57.7` | Persist authoring source and revision-bound artifact manifests | None |
| `go-slide-creator-fh57.8` | Separate input scores, structural validity and pixel-reviewed quality | `go-slide-creator-fh57.7` |
| `go-slide-creator-fh57.9` | Make all-slide rendered inspection an available completion step | `go-slide-creator-fh57.8` |
| `go-slide-creator-fh57.10` | Generate executable source-grounded repairs with meaning safeguards | `go-slide-creator-fh57.6`, `go-slide-creator-fh57.7`, `go-slide-creator-fh57.9` |
| `go-slide-creator-fh57.11` | Expose template-specific visual examples and compact composition discovery | `go-slide-creator-fh57.3`, `go-slide-creator-fh57.4`, `go-slide-creator-fh57.5` |
| `go-slide-creator-fh57.12` | Unify agent onboarding and supported entrypoints around the visual workflow | `go-slide-creator-fh57.6`, `go-slide-creator-fh57.9`, `go-slide-creator-fh57.10`, `go-slide-creator-fh57.11` |
| `go-slide-creator-fh57.13` | Gate template portability with adversarial rendered regression fixtures | `go-slide-creator-fh57.1`, `go-slide-creator-fh57.4`, `go-slide-creator-fh57.5`, `go-slide-creator-fh57.6`, `go-slide-creator-fh57.8` |
| `go-slide-creator-fh57.14` | Measure end-to-end agent slide quality against a human-rated baseline | `go-slide-creator-fh57.12`, `go-slide-creator-fh57.13` |

The epic and implementation children remain open. This review produces the plan and backlog; it does not claim these changes are implemented or that the cross-template benchmark has passed.

A portable snapshot of this review's child issues is stored in SLIDE_QUALITY_BEADS_SNAPSHOT.json for recovery. Beads remains authoritative. At session close, bd dolt push auto-configured its remote from Git but failed with "no common ancestor"; neither local nor remote history was overwritten. Reconcile the histories before assuming the remote contains these new issues.
