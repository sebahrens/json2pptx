#!/usr/bin/env bash
# Stage the agent skills and the repository files they link to.
#
# The staging itself is `json2pptx skill install`, run from this checkout: the
# four skills, the generate-deck/references/repository snapshot (so links
# resolve with no checkout around them) and a version stamp on every Markdown
# file. One implementation serves `make install-skill`, the install scripts,
# the dist archives and a binary installed on its own, so they cannot drift.
set -euo pipefail
if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: stage-skills.sh DESTINATION" >&2
  exit 2
fi
repo_root=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$1"
destination=$(cd "$1" && pwd)
case "$destination" in
  "$repo_root/skills"|"$repo_root/skills/"*)
    echo "Refusing to stage over canonical skill sources" >&2
    exit 2
    ;;
esac
cd "$repo_root"
go run ./cmd/json2pptx skill install --dest "$destination" >/dev/null
