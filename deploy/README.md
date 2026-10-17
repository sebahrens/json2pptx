# Documentation companion deployment

The Go MCP and llm-wiki MCP are separate processes with distinct responsibilities.
Wiki source is bundled with json2pptx; runtime pages, Git workspace, registry and
index live in writable companion state. No global wiki is registered.

## POSIX native install and stdio MCP

Install `pwc-llm-wiki 0.5.0` from a trusted source checkout with
`cargo install --path . --locked`, then:

```sh
make install-wiki PREFIX="$HOME/.local"
json2pptx-wiki init
json2pptx-wiki search 'column widths' --format json
json2pptx-wiki serve
```

Merge `mcp.wiki.example.json` into your client's MCP configuration beside the
json2pptx entry. The launcher requires a POSIX shell and Git; Windows clients
can invoke it through Bash or use the container companion. From a repository
checkout use `scripts/wiki.sh` directly.
Set `PWC_LLM_WIKI_BIN` for a binary outside PATH and `JSON2PPTX_WIKI_STATE_DIR`
for a dedicated writable state directory. No embedding service is needed.

## Container companion

The companion Dockerfile builds the real engine with Cargo.lock and verifies
its reported version. Its named source context is pinned to upstream commit
`9e7a1bffb0bb4b63d9efc758ff6fc078f402af00` (Cargo package version `0.5.0`).
That commit is recorded in the inspected source archive's Git archive comment;
the archive SHA-256 was
`e94f26dd18b40c58869a93bcc911bd76d4180445b4b9d3623110ee54eb62b1c5`.
The official release lookup returned 404 in this environment, so this integration
does not invent a binary download URL or trust a floating release tag.

Use a trusted local checkout of that exact commit when the upstream repository
requires access credentials:

```sh
docker buildx build --load -t json2pptx-wiki:0.5.0 \
  --build-context llm_wiki=/path/to/trusted/llm-wiki \
  -f Dockerfile.wiki .
```

In a release archive, run the container build from its `wiki-source/` directory.
For a stdio container companion after building, use
`docker run --rm -i -v json2pptx-wiki-state:/data json2pptx-wiki:0.5.0`.
The HTTP Compose add-on is designed to be merged with an existing Go deployment.

Check `git -C /path/to/trusted/llm-wiki rev-parse HEAD` against the commit above
before substituting a local context. The builder checks the Cargo version,
uses `--locked`, and checks the built executable's version. Container builds
require access to the pinned base image and Cargo.lock dependencies. The local
Linux arm64 image build was verified against the checksum-matched source archive.

To add an HTTP companion to the existing deployment:

```sh
# Generate a dedicated bearer token and set it in your deployment environment.
# It must contain at least 32 letters/digits or ._~- characters.
export JSON2PPTX_WIKI_HTTP_TOKEN='replace-with-a-dedicated-token-of-at-least-32-characters'
docker compose -f docker-compose.yml -f deploy/docker-compose.wiki.yml up --build
```

If the upstream Git context requires credentials, use the trusted local build
above instead. With the existing Go deployment already running, start only the
companion from that local image, without rebuilding or pulling:

```sh
docker compose -f docker-compose.yml -f deploy/docker-compose.wiki.yml \
  up -d --no-build --pull never wiki
```

The Compose service uses the explicit `json2pptx-wiki:0.5.0` tag produced by the
local build. The default `up --build` command still builds from the pinned remote
source context.

The wiki endpoint is `http://localhost:8081/mcp`; supply
`Authorization: Bearer <your token>`. The host port binds only to loopback.
The engine requires authentication for the container's non-loopback listener;
the wrapper writes that runtime token into a private config file and never
prints it. The companion source lives at `/opt/json2pptx`; its named volume
`wiki-state` holds the independent runtime workspace and index at `/data`.
The Go service remains on its existing port and transport.

The source-build runtime intentionally reuses the pinned Rust image for Git and
runtime libraries; it favors a verifiable working build over a smaller unverified
release image. The built image passed stdio MCP discovery/search and authenticated
HTTP initialization/search; unauthenticated HTTP requests returned 401.

## Canonical source and local edits

`docs/wiki` is the versioned source. The wrapper seeds runtime content once into
`STATE/repository/wiki`, avoiding the installed engine's nested-root ingest bug.
Every launch validates then ingests all runtime pages with `auto_commit=false`.
MCP content writes stay in this runtime workspace. They never silently modify
packaged files or the checkout; copy reviewed changes back into source explicitly.
The wrapper warns when the bundled source fingerprint changes, preserving runtime
files. For a new documentation release, choose a new state directory/volume and retain
or export local changes from the old one before switching. Do not expect a restart
to overwrite maintained runtime pages.

See `docs/wiki/wiki-engine-and-deployment.md` for exploration tools and limitations.

## Focused verification

```sh
PWC_LLM_WIKI_BIN=/path/to/pwc-llm-wiki python3 scripts/test-wiki-companion.py --http
shellcheck scripts/wiki.sh
docker compose -f docker-compose.yml -f deploy/docker-compose.wiki.yml config --quiet
```

The smoke test runs the installed engine, initializes stdio MCP, lists real tools,
and calls stats, search, sections, content read, config, schema and ingest. It also
checks restart persistence, interrupted seeds, changed-bundle warnings, double-install
removal of obsolete packaged files, and absence of commits/writeback. With `--http`
it also checks unauthorized rejection, initialization and authenticated search
through `/mcp`, including a `localhost:port` Host header. Run one companion
process per state directory; use separate directories for independent deployments.
An interrupted seed remains in a private sibling temporary directory until cleanup;
only a complete seed is renamed into the runtime repository. Concurrent bootstrap
attempts fail clearly without overwriting data. After a forcibly killed initializer,
choose a fresh state directory or inspect/remove its stale `.seed-lock` only when
no initializer is running. Existing runtime pages are never removed automatically.

Linux and Windows release archives both contain `wiki-source/` with pages, schemas,
configuration, deployment guidance and companion build/launcher files. The POSIX
archive installer installs the launcher and replaces the owned packaged source
bundle, removing obsolete files. The Windows installer copies documentation source
only; it does not claim that a shell script is a native Windows executable.
Packaged source replacement never touches the independent runtime state directory.
