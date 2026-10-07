#!/usr/bin/env bash
# Says which groups of CI jobs a change needs, so a push that did not touch a
# job's inputs does not run it.
#
#   scripts/ci_changed_paths.sh <base-sha> <head-sha>
#
# Prints three lines, each "name=true|false" (the form $GITHUB_OUTPUT takes):
#
#   go       Go sources, modules, lint config or the build / CI scripts changed:
#            the lint and vulnerability jobs.
#   svggen   The svggen module changed: its own test and lint jobs.
#   build    Anything a rendering or test job reads changed: every other job.
#            A path this script does not know counts here, so a new kind of
#            file runs the suite until someone decides it is inert.
#
# Inert paths change none of them: the bead export and the two shard-balancing
# tables, which only decide which shard a test runs in.
#
# Everything is true when the range cannot be read (a first push, a force
# push whose base is gone, a shallow clone): running too much is the safe
# failure.
set -euo pipefail

base="${1:-}"
head="${2:-HEAD}"

all() { printf 'go=true\nsvggen=true\nbuild=true\n'; }

case "$base" in
  ""|0000000000000000000000000000000000000000) all; exit 0 ;;
esac
if ! files="$(git diff --name-only "$base" "$head" 2>/dev/null)"; then
  all; exit 0
fi

go=false svggen=false build=false
while IFS= read -r f; do
  [ -n "$f" ] || continue
  case "$f" in
    beads/*|.beads/*|scripts/ci_test_durations.txt|scripts/ci_test_affinity.txt|LICENSE|.gitignore)
      continue ;;
  esac
  build=true
  case "$f" in
    .github/workflows/ci.yml|scripts/ci_changed_paths.sh|go.mod|go.sum|go.work|Makefile|.golangci*|.golangci-version)
      go=true svggen=true ;;
  esac
  case "$f" in
    svggen/*) svggen=true go=true ;;
    *.go|*/go.mod|*/go.sum|scripts/*) go=true ;;
  esac
done <<<"$files"

printf 'go=%s\nsvggen=%s\nbuild=%s\n' "$go" "$svggen" "$build"
