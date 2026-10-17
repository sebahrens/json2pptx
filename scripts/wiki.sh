#!/bin/sh
# Dedicated llm-wiki companion: never reads/writes the user's global registry.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
source_root=${JSON2PPTX_WIKI_REPO:-"$script_dir/.."}
if [ ! -f "$source_root/wiki.toml" ]; then
    source_root="$script_dir/../share/json2pptx"
fi
source_root=$(CDPATH='' cd -- "$source_root" && pwd)
engine=${PWC_LLM_WIKI_BIN:-pwc-llm-wiki}
if ! command -v "$engine" >/dev/null 2>&1; then
    echo 'Install pwc-llm-wiki 0.5.0, or set PWC_LLM_WIKI_BIN to its executable.' >&2
    exit 1
fi
if [ "$("$engine" --version)" != 'pwc-llm-wiki 0.5.0' ]; then
    echo 'This wiki integration requires pwc-llm-wiki 0.5.0.' >&2
    exit 1
fi
state_dir=${JSON2PPTX_WIKI_STATE_DIR:-"${XDG_STATE_HOME:-$HOME/.local/state}/json2pptx/wiki"}
mkdir -p "$state_dir"
state_dir=$(CDPATH='' cd -- "$state_dir" && pwd)
repo_root="$state_dir/repository"
source_fingerprint=$(
    cd "$source_root"
    { cksum wiki.toml; find schemas docs/wiki -type f -print | LC_ALL=C sort | while IFS= read -r file; do cksum "$file"; done; } | cksum | awk '{print $1 ":" $2}'
)
# Seed in a sibling directory and publish only a complete workspace. A lock
# prevents concurrent initializers from nesting/overwriting the other's tree.
if [ ! -f "$repo_root/.json2pptx-wiki-ready" ]; then
    seed_lock="$state_dir/.seed-lock"
    if ! mkdir "$seed_lock" 2>/dev/null; then
        echo 'Wiki initialization is already in progress; retry after it completes. Existing state was retained.' >&2
        exit 1
    fi
    seed_dir=""
    cleanup_seed() {
        if [ -n "$seed_dir" ]; then rm -rf "$seed_dir"; fi
        rmdir "$seed_lock" 2>/dev/null || true
    }
    trap cleanup_seed EXIT HUP INT TERM
    if [ ! -f "$repo_root/.json2pptx-wiki-ready" ]; then
        if [ -e "$repo_root" ]; then
            echo 'Existing unrecognized wiki workspace retained; choose a fresh JSON2PPTX_WIKI_STATE_DIR.' >&2
            exit 1
        fi
        seed_dir=$(mktemp -d "$state_dir/.repository.XXXXXX")
        sed 's|^wiki_root = .*|wiki_root = "wiki"|' "$source_root/wiki.toml" > "$seed_dir/wiki.toml"
        cp -R "$source_root/schemas" "$seed_dir/schemas"
        cp -R "$source_root/docs/wiki" "$seed_dir/wiki"
        git init -q "$seed_dir"
        printf '%s\n' "$source_fingerprint" > "$seed_dir/.json2pptx-wiki-source-fingerprint"
        printf '%s\n' 'json2pptx llm-wiki runtime snapshot 0.5.0' > "$seed_dir/.json2pptx-wiki-ready"
        mv "$seed_dir" "$repo_root"
        seed_dir=""
    fi
    cleanup_seed
    trap - EXIT HUP INT TERM
fi
if [ ! -d "$repo_root/wiki" ]; then
    echo 'Incompatible existing wiki state; choose a fresh JSON2PPTX_WIKI_STATE_DIR. Existing files were retained.' >&2
    exit 1
fi
previous_fingerprint=$(cat "$repo_root/.json2pptx-wiki-source-fingerprint" 2>/dev/null || true)
if [ "$previous_fingerprint" != "$source_fingerprint" ]; then
    echo 'Bundled wiki source changed. Runtime edits were retained; use a fresh JSON2PPTX_WIKI_STATE_DIR to adopt the new bundle, then merge reviewed local edits.' >&2
fi
config="$state_dir/config.toml"
# This config is owned solely by this wrapper. The runtime registry contains
# resolved absolute paths; only portable relative configuration is committed.
umask 077
cat > "$config" <<'CONFIG'
[ingest]
auto_commit = false
[logging]
log_path = ""
CONFIG
if [ -n "${JSON2PPTX_WIKI_HTTP_BIND:-}" ]; then
    if [ "$JSON2PPTX_WIKI_HTTP_BIND" != '0.0.0.0' ]; then
        echo 'JSON2PPTX_WIKI_HTTP_BIND supports only the explicit container bind 0.0.0.0.' >&2
        exit 1
    fi
    if ! printf '%s' "${JSON2PPTX_WIKI_HTTP_TOKEN:-}" | grep -Eq '^[A-Za-z0-9._~-]{32,}$'; then
        echo 'Set JSON2PPTX_WIKI_HTTP_TOKEN to at least 32 letters, digits, or ._~- characters.' >&2
        exit 1
    fi
    printf '\n[serve]\nhttp_bind = "0.0.0.0"\nhttp_allowed_hosts = ["localhost", "127.0.0.1", "::1", "wiki"]\nhttp_token = "%s"\n' "$JSON2PPTX_WIKI_HTTP_TOKEN" >> "$config"
fi

"$engine" --config "$config" spaces register "$repo_root" --name json2pptx --wiki-root wiki >&2
# Validate before indexing, including uncommitted pages. Both operations use
# real engine commands and inherit auto_commit=false from the runtime wiki.
"$engine" --config "$config" --wiki json2pptx ingest . --full --dry-run --format json >&2
"$engine" --config "$config" --wiki json2pptx ingest . --full --format json >&2
if [ "${1:-}" = init ]; then exit 0; fi
if [ "$#" -eq 0 ]; then set -- serve; fi
cd "$repo_root"
exec "$engine" --config "$config" --wiki json2pptx "$@"
