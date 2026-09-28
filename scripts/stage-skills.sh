#!/usr/bin/env bash
# Package canonical instructions and their repository-relative reference closure.
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
for source in "$repo_root"/skills/*/; do
  name=$(basename "$source")
  mkdir -p "$destination/$name"
  cp -R "$source". "$destination/$name/"
done
# Keep the reference snapshot repository-shaped so links within canonical docs
# continue to resolve. These are generated install resources, never new sources.
references="$destination/generate-deck/references/repository"
mkdir -p "$references/docs" "$references/examples" "$references/internal"
for doc in INPUT_FORMAT INPUT_FORMAT_ADVANCED FIT_FINDINGS SEMANTIC_COMPILER TEMPLATE_SPEC PATH_GRAMMAR PATTERNS TEMPLATE_ANALYSIS; do
  cp "$repo_root/docs/$doc.md" "$references/docs/"
done
cp -R "$repo_root/examples/semantic" "$references/examples/"
cp -R "$repo_root/internal/tokens" "$references/internal/"
cp -R "$repo_root/skills" "$references/"
# Portable source-aware examples are linked from the template guide. Keep their
# original relative image alongside them; do not ship the whole test corpus.
evidence_relative="tests/quality/evidence/connectors/midnight-blue"
mkdir -p "$references/$evidence_relative"
for resource in source-aware-evidence-route.json readable-source-companion-route.json powerpoint-slide-4.png; do
  cp "$repo_root/$evidence_relative/$resource" "$references/$evidence_relative/"
done
# Only installed entrypoint guides need rerouting; the reference snapshot keeps
# its original relative links. sed without -i works on macOS and Git Bash alike.
for source_guide in "$repo_root"/skills/*/*.md; do
  [ -f "$source_guide" ] || continue
  guide="$destination/$(basename "$(dirname "$source_guide")")/$(basename "$source_guide")"
  sed -e 's|](../../docs/|](../generate-deck/references/repository/docs/|g' \
      -e 's|](../../examples/|](../generate-deck/references/repository/examples/|g' \
      -e 's|](../../internal/|](../generate-deck/references/repository/internal/|g' \
      -e 's|](../../tests/|](../generate-deck/references/repository/tests/|g' \
      "$guide" > "$guide.install-tmp"
  mv "$guide.install-tmp" "$guide"
done
