# running notes (e-revise)
- n=1 list; n=2 get_started brief (12KB), n=3 get_started revise (9KB).
- F: get_started(revise).fast_path.steps[*].args_template shows {"spec":"<DeckSpec>","strict":"warn"} even though when_to_call says "Send deck_id + patch" -> template contradicts prose.
- F: get_started lists tools not in default profile: get_capabilities, validate_input, preview_presentation_plan, repair_slide, generate_presentation, inspect_slide_images, analyze_deck_rhythm (falls_back_to + sequence). Default profile has 12 tools.
- Delight: revise fast_path prose is very explicit about deck_id + patch, JSON pointer, changed_slides -> slide_indices.
- n=4 was a bridge harness problem (@file) - not product.
- n=5 plan_deck(deckspec, budget 8, template modern-yellow): 3KB. Draft = structure form: cover + 2 sections (exec_summary, kpi_snapshot | roadmap) + closing next_steps = 5 authored slots. No slot for risk; no slot for ask-of-investors (closing next_steps has no facts); "main risk is permitting delay" and "ask is introductions..." vanish: not in any slots[].facts AND unplaced_facts == []. "8 slides" routed as a FACT to the kpi slot. kpi slot labelled slot:"problem" guidance "What is wrong or at stake" for good-news numbers. No budget_note explaining 5 vs 8. slots[].slide_index 0..4 (authoring index, not rendered index once agenda/dividers expand). slots[].path is dotted "structure.sections[0].slides[1]" while patch wants JSON Pointer.
- n=7 validate of UNMODIFIED plan draft (670B req, 8.3KB resp): 3 errors (kpis/phases/actions required), 8 warnings. deck_id returned even for invalid spec (good). where.slide is SECTION-RELATIVE in structure form: {"slide":0} for both structure.sections[0].slides[0] and structure.sections[1].slides[0] -> ambiguous address; cover/closing have no where at all.
- next_tool_call patch paths ARE JSON pointers (/structure/sections/0/slides/0/title) while evidence.path is dotted (structure.sections[0].slides[0].title): two notations in one finding.
- n=6 list_slide_kinds default: 22.9KB; n=8 item_schema for 7 kinds: 23.8KB. Slide schemas have NO id/handle/name field (additionalProperties:false) -> slides only addressable by position.
- Authoring decision: plan draft kept title/executive_summary/roadmap/next_steps kinds; dropped structure form (agenda+2 dividers would eat 3 of 8 slides), replaced kpi_snapshot by stat + chart_insight + stat (brief wants headline numbers), added risk table. I placed risk right after runway (before roadmap) - my own choice.
## Baseline
- n=9 validate v1 (3.8KB req, 3.1KB resp) ok, deck_id deck_8878...; n=10 render via deck_id+patch (233B req, 7.0KB resp) -> first valid render on attempt 1. changed_slides [1,5] and next_tool_call narrowed thumbnails to [1,5] although NOTHING had been rendered/seen yet (changed relative to stored spec, not to last seen revision).
- n=11 full thumbnails 268KB, 2.25s, 8 jpeg 667x375. All 8 slides clean.
## Turn 1 (slide 3 -> chart)
- n=12 render_deck_spec deck_id+patch replace /slides/2 (727B req; 7.0KB resp, ~6KB of it unchanged quality_summary+explanation_summary). changed_slides [2]. n=13 thumbs slide_indices [2] 36.6KB 1.5s. Looked right first time.
- user says "slide 3" (1-based), pointer /slides/2 (0-based); response mixes quality_summary.slide_scores[].slide_number (1-based) with explanation_summary.slides[].index + changed_slides (0-based).
- No read-back tool for the stored spec in default profile; explanation_summary.slides[] (index, kind, title) is the only confirmation of what sits where.
## Turn 2 (insert competitor matrix after roadmap) - 4 render calls, 2 failed
- n=15 add /slides/7 option_matrix (905B req) -> isError, TEXT_BELOW_READABLE_MIN: evidence.text "Tidewater Carbon", role card-title, semantic_path only "slides[7]", recommended_edit shorten_text, next_tool_call = add /slides/7/layout "content" (would degrade matrix to bullets). Response still 7.8KB incl. full explanation_summary. changed_slides [7,8] -> the patch WAS stored although render failed (not stated explicitly).
- n=16 shortened name to "Tidewater" -> SAME error, evidence.text now "Tidewater" at same 11.04pt. Shortening the named text did nothing: wrong lever.
- n=17 describe_finding (1.5KB) generic; n=18 compositions (only table-highlight | content bullets). hint refers to explain_deck_spec which is not in default profile.
- n=19 validate deck_id only: reported 3 errors (render reported 1), paths in 2 notations in one response: "/slides/7/pattern/rows/2/cells/0/shape/text" and "slides[7].options[1].name"; params cell_max_chars 60 for 9-char "Tidewater".
- n=20 guessed: remove /slides/7/options/0/detail -> render OK. n=21 validate now "no issues" (so the 3 errors for other rows vanished after touching row 0 only).
- n=23 restored name "Tidewater Carbon" (redo caused by misleading diagnostic) -> OK. Real cause: detail (37 chars; schema says <=80) on the highlighted row that also carries highlight_label badge.
- n=25 catalog example for option_matrix validates clean on modern-yellow (detail 26 chars).
- n=22 thumbs [7] 42KB; n=24 thumbs [7] density 110: 104KB. Visual: column header font sizes inconsistent (Cost per tonne/Energy use larger than Deployment speed/Commercial traction); score says 100.
- Index shift: ask slide moved from 7 -> 8 silently; changed_slides on the failed call said [7,8] (8 = shifted, content unchanged).
## Turn 3 (move risk before ask + shorten title)
- n=26 tried op "move" -> clean INPUT.UNKNOWN_ENUM "use replace, add or remove" (1.4KB). No move op: a move = remove + add with the WHOLE slide resent (n=27, 752B req) from my own memory, because there is no read-back of the stored spec (and my local file lacked the earlier-patched source field).
- changed_slides [5,6,7]: positional diff; 5 and 6 only shifted (page number changed), 7 is the real edit. No distinction moved vs edited. n=28 thumbs [5,6,7] 108KB for 1 real change.
## Turn 4 (ARR 9.4 -> 9.6 everywhere)
- No search/find tool, no read-back. Located occurrences from my own memory of the authored spec; explanation_summary only echoes titles (+takeaways), not body fields. 6 replace ops in one patch (n=29, 748B req, 7.6KB resp): 2 titles, 1 support, 1 insight, 1 takeaway, chart values array; derived 54% -> 57% also by hand.
- Verified no leftovers only by unzipping the pptx and grepping (outside the product). changed_slides [1,2] correct. n=30 thumbs [1,2] 77KB.
## Turn 5 (burn chart + roadmap share a slide)
- n=31 regions item_schema 19KB for one kind. No copy op / no reference to existing slide content: resent the burn chart data and the three milestones in full (1134B req). n=32 remove /slides/3 + replace /slides/4 (index of the second op is AFTER the first op's shift - had to do the arithmetic myself). Rendered first time.
- changed_slides [3,4,5,6,7] = 5 of 8; only slide 4 is new content, the rest are renumber-only (page number). n=33 thumbs for 5 slides 185KB.
- Observed: thumbnail content_hash for the ask slide (now back at index 7, page 8) equals its v1 hash 0a85a55a1582 -> hashes are content-addressed across revisions, so an agent can self-detect "already seen", but the product does not say so.
- New warning SEMANTIC_RHYTHM_DENSITY at slides[4] (3 consecutive dense slides) caused by the merge.
## Turn 6 (template -> abstract)
- n=34 one patch replace /meta/template (159B req, 8.7KB resp). changed_slides all 8. New info findings surfaced by the new template (SUBTITLE_WRAPS slides[0], contrast_predicted). n=35 full thumbs 306KB 1.5s. All 8 fine.
## Turn 7 (undo last change to title slide - there was none)
- No history/undo/revert/revision tool or parameter (grep of tools/list; n=36,n=37 UNKNOWN_PARAMETER for history/undo - clean rejections listing accepted args).
- What helped: (a) my OWN log of changed_slides per call - slide 0 appears only in the template switch; (b) thumbnail content_hash: v6 slide 0 hash == v1 hash fc938fa4 (n=39, 21KB, 0.01s cache hit) so pixels identical through v1..v6; (c) no-op patch of original title/subtitle (n=38) returned NO changed_slides key at all (absent, not []).
- Product keeps no per-slide change log; an agent that did not save every response cannot answer. No way to restore a previous revision; "undo" = hand-author the inverse patch.
