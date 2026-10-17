---
title: "Wiki engine and companion MCP"
type: documentation
summary: "Use the real llm-wiki engine to search and explore bundled json2pptx guidance"
status: active
last_updated: "2026-10-17"
tags: [json2pptx, llm-wiki, mcp, deployment]
read_when:
  - "Finding authoring guidance through the companion wiki MCP"
  - "Deploying documentation beside the json2pptx MCP"
---

## What is deployed

The wiki is indexed by `pwc-llm-wiki 0.5.0`, the Rust engine from
[llm-wiki](https://github.com/pwc-ch-adv-ai/llm-wiki).
It is a separate companion MCP process. The json2pptx MCP remains responsible
for live schemas, validation, previews and rendering; the wiki explains how to
use those capabilities. Search results are documentation, not live schema proofs.

The repository's `wiki.toml` points to `docs/wiki`; `schemas/` contains the
engine's generated 0.5.0 frontmatter schemas and templates. Existing Markdown
paths and links are retained. [The overview](README.md) and
[exploration hub](guides/index.md) provide entry points.

## Start from a checkout

Install the exact engine version from a trusted 0.5.0 source checkout using
`cargo install --path . --locked` (Rust 1.95+). This repository never downloads
an unpinned engine or modifies your global wiki registry.

```sh
scripts/wiki.sh init
scripts/wiki.sh search 'regions' --top-k 5 --format json
scripts/wiki.sh list --type section --format json
scripts/wiki.sh content read slide-estate-and-segments
scripts/wiki.sh stats --format json
scripts/wiki.sh serve
```

`PWC_LLM_WIKI_BIN` selects the engine executable. `JSON2PPTX_WIKI_REPO` selects
the source root; its `wiki.toml`, `schemas/` and `docs/wiki/` must travel together.
`JSON2PPTX_WIKI_STATE_DIR` selects writable state, defaulting to
`$XDG_STATE_HOME/json2pptx/wiki` or `$HOME/.local/state/json2pptx/wiki`.
Runtime registration resolves paths on the current machine. No machine-specific
paths are committed. The wrapper disables automatic commits and validates/ingests
its runtime pages before each invocation. It never stages or commits the source checkout.

## Connect an agent

After `make install-wiki PREFIX=...`, add the companion configuration from
[`deploy/mcp.wiki.example.json`](../../deploy/mcp.wiki.example.json) beside your
existing json2pptx MCP entry. The command `json2pptx-wiki serve` uses stdio.
From a checkout, use `scripts/wiki.sh serve` instead.

Bootstrap with `wiki_config`, `wiki_schema`, `wiki_stats`, a `wiki_list` filtered
to `section`, then `wiki_content_read` on `README` or `guides`. Use
`wiki_search` for BM25 retrieval and `wiki_content_read` with backlinks to follow
related guidance. `wiki_graph`, `wiki_lint`, and `wiki_suggest` expose structure
and maintenance signals. These are engine tools; the Go MCP does not impersonate
them or maintain a second index.

## Packaging and state

`make install-wiki` installs the wrapper and wiki source together under the
chosen prefix. On its first invocation the wrapper copies the canonical pages,
schemas and configuration into `$JSON2PPTX_WIKI_STATE_DIR/repository`, changes
only that copy's `wiki_root` to the standard `wiki`, and runs `git init` there.
It creates no commit. The original packaged or checkout files remain unchanged.
Both `ingest --full --dry-run` and `ingest --full` run against this workspace;
validation must succeed before the command or server starts.

Runtime content edits persist in that workspace and affect the companion index;
they are not written back into `docs/wiki`. Review and copy intended changes
back into the source repository separately. A restart does not overwrite local
pages. After upgrading packaged documentation or editing canonical source pages,
choose a fresh state directory to seed a new snapshot; retain/export the old
workspace if it contains changes you want to keep. The wrapper detects a changed source bundle and warns without replacing runtime
pages. There is no hidden bidirectional synchronization. Config and search index are also stored under the writable state
directory, outside immutable source files.

The main Docker image ships the wiki source. The optional companion image is
built from an explicit trusted 0.5.0 source context with its Cargo.lock, rather
than an unverified release download. See [`deploy/README.md`](../../deploy/README.md)
for the build and MCP transport examples.

## Verified capabilities and current limits

Search, typed listing, reading, stats, schema inspection, and stdio MCP discovery
are exercised against the actual engine. Indexes and runtime configuration live
outside immutable packaged documentation. The engine provides lexical BM25,
optional fuzzy search and guarded regex; it does not require an embedding API.

The installed 0.5.0 engine cannot ingest a directly mounted nested `wiki_root`:
it derives the Git root from the content directory's immediate parent, so
`docs/wiki` makes it look for a repository in `docs/`. The wrapper avoids this
problem with the standard top-level runtime `wiki` directory. Its validation,
ingest, search and MCP tools use that working layout. Do not register the source
checkout directly and assume nested-root ingestion works. Content writes through
wiki MCP need `wiki_ingest` before search, as the engine's content workflow
requires; `serve --watch` can ingest external edits inside the runtime workspace.
Git history requires actual commits; snapshots start with none and automatic
commits stay disabled. Explicit `wiki_content_commit` remains an engine action.

Some Markdown links intentionally point outside the wiki to source, examples,
and contributor docs. They remain useful in a checkout; graph lint may report
external-to-wiki targets as unresolved, and a packaged source snapshot does not
promise that every contributor artifact is present. Follow the live Go MCP for
current input contracts and inspect rendered slides before approval.
