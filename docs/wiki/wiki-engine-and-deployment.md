---
title: "Wiki engine and companion MCP"
type: documentation
summary: "Use the real llm-wiki engine to search and explore bundled json2pptx guidance"
status: active
last_updated: "2026-10-18"
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
[exploration hub](guides) provide entry points.

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
paths are committed. The wrapper disables automatic commits. On first launch or after runtime content
changes, it validates, ingests and lints the pages before serving them. Unchanged
launches reuse registration and the existing index. It never stages or commits the source checkout.

## Connect an agent

After `make install-wiki PREFIX=...`, add the companion configuration from
[`deploy/mcp.wiki.example.json`](https://github.com/sebahrens/json2pptx/blob/main/deploy/mcp.wiki.example.json) beside your
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
validation and `lint --severity error` must succeed before the command or server
starts. Content fingerprints cover configuration, schemas and all runtime pages,
so edits are detected even when timestamps are preserved. A missing index is
rebuilt; a failed validation or lint never marks the refresh complete.

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
than an unverified release download. See [`deploy/README.md`](https://github.com/sebahrens/json2pptx/blob/main/deploy/README.md)
for the build and MCP transport examples. Compose names the local image
`json2pptx-wiki:0.5.0`; after building it from a trusted local checkout, use
`up -d --no-build --pull never wiki` to start the companion beside an existing
Go deployment without contacting the upstream source or image registry.

## Align the binary, bundle and runtime snapshot

Deploy the Go MCP and `wiki-source/` from the same json2pptx release archive or
Git revision. Record that artifact revision alongside the Go MCP's live
`schema_version` from `get_capabilities` / `get_started`, and the companion's
`pwc-llm-wiki --version` (pinned to `0.5.0`). The engine version is separate
from the json2pptx schema version: updating the Go binary does not update an
already seeded documentation workspace.

After a release upgrade, seed a fresh `JSON2PPTX_WIKI_STATE_DIR` (or new
container volume), run strict ingestion and lint, and check search before
switching the MCP configuration to that snapshot. Preserve the prior workspace
for reviewed local edits. A changed-bundle warning means the runtime still
serves its existing snapshot; restarting alone does not adopt the new pages.
Use live tool schemas for automation while reconciling a version mismatch.

## Render cache location

The Go renderer stores reusable slide images under
`os.UserCacheDir()/json2pptx-render`; `JSON2PPTX_RENDER_CACHE_DIR` selects an
explicit cache directory for an isolated server. If the operating system has no
user cache directory, the renderer uses a private temporary directory. This is
separate from the companion wiki's state and search index.

The cache must be owned by the current user, private and not a symlink. On
Unix, cache directories use `0700` and artifact files use `0600`. An unsafe
cache location is not reused or written. Keep it writable by the service
identity; use the live `purge_render_cache` schema when clearing cached renders.
Do not point multiple service identities at a shared writable temporary cache.

## Verified capabilities and current limits

Search, typed listing, reading, stats, schema inspection, and stdio MCP discovery
are exercised against the actual engine. The Linux arm64 companion image was
built from the verified pinned source and passed stdio and authenticated HTTP
MCP search checks. Indexes and runtime configuration live
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

Links between wiki pages resolve inside the runtime snapshot. References to
source, examples and contributor documents use GitHub URLs so they also work
for an installed companion without the repository checkout. Those external
pages require network access and are not indexed by the wiki. Follow the live
Go MCP for current input contracts and inspect rendered slides before approval.
