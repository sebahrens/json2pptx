#!/usr/bin/env bash
# Runs the cmd/json2pptx short race suite as N parallel shards and merges
# their coverage profiles (go-slide-creator-zpgmp).
#
# The package's tests run one after another, so on a multi-core runner a
# single `go test` leaves most cores idle while the suite takes about an hour
# under -race. Each shard is its own `go test -run` over a slice of the
# top-level tests; a test belongs to exactly one shard, chosen by its position
# in the sorted list, so adding a test never needs this file edited.
#
# Usage: scripts/ci_test_cmd_shards.sh [shards] [coverprofile]
set -euo pipefail

SHARDS="${1:-3}"
PROFILE="${2:-coverage-cmd.out}"
PKG=./cmd/json2pptx
TIMEOUT="${SHARD_TIMEOUT:-40m}"
LOGDIR="$(mktemp -d)"

TESTS=()
while IFS= read -r name; do
  TESTS+=("$name")
done < <(go test "$PKG" -short -list '^Test' | grep '^Test' | sort)
if [ "${#TESTS[@]}" -eq 0 ]; then
  echo "no tests listed in $PKG" >&2
  exit 1
fi
echo "${#TESTS[@]} top-level tests in $SHARDS shards"

pids=()
for ((s = 0; s < SHARDS; s++)); do
  names=()
  for ((i = s; i < ${#TESTS[@]}; i += SHARDS)); do
    names+=("${TESTS[i]}")
  done
  pattern="^($(IFS='|'; echo "${names[*]}"))\$"
  (
    go test "$PKG" -short -v -race -run "$pattern" \
      -coverprofile="$LOGDIR/cover-$s.out" -covermode=atomic -timeout="$TIMEOUT" \
      >"$LOGDIR/shard-$s.log" 2>&1
  ) &
  pids+=("$!")
done

failed=0
for ((s = 0; s < SHARDS; s++)); do
  if ! wait "${pids[s]}"; then
    failed=1
    echo "::group::shard $s FAILED"
  else
    echo "::group::shard $s"
  fi
  cat "$LOGDIR/shard-$s.log"
  echo "::endgroup::"
done

if [ "$failed" -ne 0 ]; then
  echo "--- failures ---"
  grep -hE '^(\s*--- FAIL|FAIL|panic:)' "$LOGDIR"/shard-*.log || true
  exit 1
fi

# One profile: the mode line once, then every shard's blocks. A block counted
# by several shards is summed by `go tool cover`.
echo "mode: atomic" >"$PROFILE"
for ((s = 0; s < SHARDS; s++)); do
  tail -n +2 "$LOGDIR/cover-$s.out" >>"$PROFILE"
done
grep -hE '^ok|^coverage:' "$LOGDIR"/shard-*.log || true
