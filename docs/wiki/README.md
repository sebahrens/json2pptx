# json2pptx wiki: building consulting-grade decks with an agent

This wiki is for an agent (or the person steering one) that has to turn a
brief into a deck a partner would present: a pitch or proposal, a deals
document, a risk-consulting proposal, a technology-and-data business case or
a risk-assurance readout. It sits beside the agent skill
([`skills/generate-deck/`](../../skills/generate-deck/SKILL.md), the contract
the engine enforces) and the contributor docs in [`docs/`](../) (how the engine
works). The wiki is the *how to use it well* layer: storyline, visual choice,
the MCP call path, and four worked decks that validate and render today.

Every YAML block in these pages validates against the current build
(`TestDocSemanticSnippetsValidateClean`), and the four playbook decks under
[`examples/semantic/playbooks/`](../../examples/semantic/playbooks/) render on
their template and on the local `p-style` template
(`TestBundledSemanticExamplesValidateClean`). If a page and the engine
disagree, the engine is right and the page has a bug: fix it in the same
change.

## Reading order

| Page | Read it when |
|---|---|
| [Agent journey through the MCP](agent-mcp-journey.md) | First. The exact call sequence from `get_started` to `submit_visual_review`, what each response carries, and the traps four product-only agent runs hit. |
| [Storyline and structure](storyline-and-structure.md) | Before writing a single slide. Ghost deck, action titles, pyramid / SCQA, executive summary, the closer, appendix. |
| [Split and complex layouts](split-and-complex-layouts.md) | When one slide has to carry two or three things: chart beside narrative, bridge beside implication, status boards, before/after, photo with callouts, architecture rails, long tables, raw composites. |
| [Visual vocabulary](visual-vocabulary.md) | When choosing how a message should look: message → kind / pattern, by domain. |
| [Quality and review](quality-and-review.md) | Before claiming a deck is done: the gates, the ten-point rubric, the second-template pass, what the tools cannot see. |
| [Troubleshooting](troubleshooting.md) | When a finding, a refusal or a render surprises you. |

## Playbooks (one worked deck each)

| Scenario | Page | Deck |
|---|---|---|
| Consulting pitch and deals (proposal, due diligence, IC paper) | [playbook-consulting-pitch-and-deals.md](playbook-consulting-pitch-and-deals.md) | [`deals-cdd-proposal.yaml`](../../examples/semantic/playbooks/deals-cdd-proposal.yaml) |
| Risk consulting (ERM uplift, regulatory remediation, risk appetite) | [playbook-risk-consulting.md](playbook-risk-consulting.md) | [`risk-erm-uplift-proposal.yaml`](../../examples/semantic/playbooks/risk-erm-uplift-proposal.yaml) |
| Technology and data (platform business case, architecture, migration) | [playbook-technology-and-data.md](playbook-technology-and-data.md) | [`tech-data-platform-business-case.yaml`](../../examples/semantic/playbooks/tech-data-platform-business-case.yaml) |
| Risk assurance (ITGC / SOX readout, internal audit, controls) | [playbook-risk-assurance.md](playbook-risk-assurance.md) | [`assurance-itgc-readout.yaml`](../../examples/semantic/playbooks/assurance-itgc-readout.yaml) |

Each playbook gives the audience and the ask, the storyline skeleton, a
slide-by-slide blueprint naming the DeckSpec kind for every slide, the domain's
own split and complex layouts, and the review points that matter for that
audience. The decks were authored by agents that saw only the product (no
source, no docs) in the journey harness under
[`tests/quality/journey/`](../../tests/quality/journey/) — personas `f` to `i` —
then tightened by hand.

## Three rules that outrank everything else here

1. **The storyline is the deliverable.** Write the action titles first and read
   them in sequence; if they do not argue the case on their own, no layout will.
2. **One slide, one message, one exhibit — unless several views prove the same
   title.** Then it is a split layout (`regions`, a raw `compose`), never two
   messages on one page.
3. **Images are truth.** `deterministic_ready: true` and a score of 100 mean the
   engine found nothing; they do not mean the slide looks right. Render every
   slide of the revision you ship and look at it, on the second template too.

## Render these pages' decks yourself

```bash
json2pptx semantic validate examples/semantic/playbooks/deals-cdd-proposal.yaml --templates-dir templates
json2pptx semantic render   examples/semantic/playbooks/deals-cdd-proposal.yaml --templates-dir templates --out /tmp/falcon.pptx
json2pptx render-thumbnails /tmp/falcon.pptx --out-dir /tmp/falcon-slides/
```

Through the MCP server (`json2pptx mcp`), the same deck is `render_deck_spec`
with the file's contents as `spec` and `base_dir` set to the repository root
(asset paths inside the playbooks are relative to the YAML file).
