#!/usr/bin/env bash
# Runs the cmd/json2pptx short race suite as N parallel shards and merges
# their coverage profiles (go-slide-creator-zpgmp, go-slide-creator-q7cpq).
#
# The package's tests run one after another, so on a multi-core runner a
# single `go test` leaves most cores idle while the suite takes about an hour
# under -race. The test binary is built once and each shard is one run of it
# over its share of the top-level tests; a test belongs to exactly one shard.
#
# Shards are balanced by measured duration, not by name:
#
#   scripts/ci_test_durations.txt  "seconds TestName" per line, from a CI run.
#       Refresh it from a job log with scripts/ci_test_durations.sh (see that
#       script). A test the file does not list weighs CI_TEST_DEFAULT_WEIGHT
#       seconds, so adding a test never needs the file edited; a stale file
#       only balances worse.
#   scripts/ci_test_affinity.txt   one group of test names per line. A group
#       shares an expensive once-per-binary fixture, so it is placed in one
#       shard as a unit and the fixture is computed once.
#
# Groups and single tests are placed longest first, each on the shard that is
# lightest so far.
#
# Every shard's wall time is printed. A shard over SHARD_BUDGET_SECONDS is a
# warning, and a failure when SHARD_BUDGET_ENFORCE=1 (CI sets it): the step
# then fails with the slowest tests named while it still has room under its
# hard time limit, instead of timing out some weeks later.
#
# Usage: scripts/ci_test_cmd_shards.sh [shards] [coverprofile]
#
# Environment:
#   SHARD_TIMEOUT            go test timeout of one shard (default 47m)
#   SHARD_BUDGET_SECONDS     wall time one shard may take (default 1800)
#   SHARD_BUDGET_ENFORCE     1 fails the run over budget (default 0: warn)
#   CI_TEST_DURATIONS        durations file (default scripts/ci_test_durations.txt)
#   CI_TEST_AFFINITY         affinity file (default scripts/ci_test_affinity.txt)
#   CI_TEST_DEFAULT_WEIGHT   seconds assumed for an unlisted test (default 0.1,
#                            about what the two thousand tests under a second take)
#   SHARD_PLAN_ONLY          1 prints the assignment and exits without running
#   SHARD_TESTS              a regular expression: run only the matching tests
#                            (for trying the script; CI runs them all)
set -euo pipefail
export LC_ALL=C # one sort order on every machine: the assignment is deterministic

SHARDS="${1:-3}"
PROFILE="${2:-coverage-cmd.out}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG_DIR="$ROOT/cmd/json2pptx"
TIMEOUT="${SHARD_TIMEOUT:-47m}"
BUDGET="${SHARD_BUDGET_SECONDS:-1800}"
ENFORCE="${SHARD_BUDGET_ENFORCE:-0}"
DURATIONS="${CI_TEST_DURATIONS:-$ROOT/scripts/ci_test_durations.txt}"
AFFINITY="${CI_TEST_AFFINITY:-$ROOT/scripts/ci_test_affinity.txt}"
DEFAULT_WEIGHT="${CI_TEST_DEFAULT_WEIGHT:-0.1}"
LOGDIR="$(mktemp -d)"
BIN="$LOGDIR/json2pptx.test"

case "$PROFILE" in
  /*) ;;
  *) PROFILE="$PWD/$PROFILE" ;;
esac
for f in "$DURATIONS" "$AFFINITY"; do
  if [ ! -f "$f" ]; then
    echo "missing $f" >&2
    exit 1
  fi
done

# One race + coverage build for every shard (and for the test listing).
(cd "$ROOT" && go test -c -race -covermode=atomic -o "$BIN" ./cmd/json2pptx)

mkdir "$LOGDIR/listcov" # where the listing's (empty) coverage counters go
(cd "$PKG_DIR" && GOCOVERDIR="$LOGDIR/listcov" "$BIN" -test.short -test.list '^Test') | grep '^Test' | sort >"$LOGDIR/tests.txt"
total="$(wc -l <"$LOGDIR/tests.txt" | tr -d ' ')"
if [ "$total" -eq 0 ]; then
  echo "no tests listed in ./cmd/json2pptx" >&2
  exit 1
fi

# An affinity group that names a test the package no longer has would stop
# keeping its fixture in one shard without anyone deciding so.
missing="$(grep -v '^[[:space:]]*#' "$AFFINITY" | tr -s '[:space:]' '\n' | grep . | sort -u | comm -23 - "$LOGDIR/tests.txt" || true)"
if [ -n "$missing" ]; then
  echo "$AFFINITY names tests that ./cmd/json2pptx does not have:" >&2
  # shellcheck disable=SC2086 # one name per line
  printf '  %s\n' $missing >&2
  echo "rename them in the affinity file (or drop the group if the fixture is no longer shared)." >&2
  exit 1
fi

# A local trial of the script itself can run a part of the suite.
if [ -n "${SHARD_TESTS:-}" ]; then
  grep -E "$SHARD_TESTS" "$LOGDIR/tests.txt" >"$LOGDIR/tests-kept.txt" || true
  mv "$LOGDIR/tests-kept.txt" "$LOGDIR/tests.txt"
  total="$(wc -l <"$LOGDIR/tests.txt" | tr -d ' ')"
  if [ "$total" -eq 0 ]; then
    echo "SHARD_TESTS=$SHARD_TESTS matches no test" >&2
    exit 1
  fi
fi

# Items (a group or a single test) with their weight, heaviest first, then
# each onto the lightest shard. Output: "<shard> <test>" per test.
awk -v def="$DEFAULT_WEIGHT" '
  FILENAME == ARGV[1] { if ($0 !~ /^[[:space:]]*#/ && NF >= 2) w[$2] = $1; next }
  FILENAME == ARGV[2] {
    if ($0 ~ /^[[:space:]]*#/ || NF == 0) next
    groups++
    for (i = 1; i <= NF; i++) group[$i] = groups
    next
  }
  {
    weight = ($1 in w) ? w[$1] : def
    if ($1 in group) {
      g = group[$1]
      gw[g] += weight
      gm[g] = (g in gm) ? gm[g] " " $1 : $1
    } else {
      printf "%.2f\t%s\n", weight, $1
    }
  }
  END { for (g in gm) printf "%.2f\t%s\n", gw[g], gm[g] }
' "$DURATIONS" "$AFFINITY" "$LOGDIR/tests.txt" |
  sort -t "$(printf '\t')" -k1,1nr -k2,2 |
  awk -F '\t' -v shards="$SHARDS" -v plan="$LOGDIR/plan.txt" '
    BEGIN { for (s = 0; s < shards; s++) load[s] = 0 }
    {
      best = 0
      for (s = 1; s < shards; s++) if (load[s] < load[best]) best = s
      load[best] += $1
      n = split($2, names, " ")
      count[best] += n
      for (i = 1; i <= n; i++) print best, names[i]
    }
    END { for (s = 0; s < shards; s++) printf "%d %d %.0f\n", s, count[s], load[s] >plan }
  ' >"$LOGDIR/assign.txt"

echo "$total top-level tests in $SHARDS shards, balanced by $(basename "$DURATIONS")"
while read -r s n predicted; do
  echo "  shard $s: $n tests, predicted ${predicted}s"
done <"$LOGDIR/plan.txt"
if [ "${SHARD_PLAN_ONLY:-0}" = "1" ]; then
  cat "$LOGDIR/assign.txt"
  exit 0
fi

pids=()
for ((s = 0; s < SHARDS; s++)); do
  pattern="^($(awk -v s="$s" '$1 == s { print $2 }' "$LOGDIR/assign.txt" | paste -s -d '|' -))\$"
  (
    cd "$PKG_DIR"
    started="$(date +%s)"
    rc=0
    # As `go test` runs a coverage binary: one counter directory for the test
    # process and, through GOCOVERDIR, for the CLI runs it starts by executing
    # itself; the profile is written from all of it.
    mkdir "$LOGDIR/gocover-$s"
    GOCOVERDIR="$LOGDIR/gocover-$s" "$BIN" -test.short -test.v -test.paniconexit0 -test.run "$pattern" \
      -test.gocoverdir="$LOGDIR/gocover-$s" \
      -test.coverprofile="$LOGDIR/cover-$s.out" -test.timeout="$TIMEOUT" \
      >"$LOGDIR/shard-$s.log" 2>&1 || rc=$?
    echo "$(($(date +%s) - started))" >"$LOGDIR/seconds-$s"
    exit "$rc"
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

# What each shard took, against what the durations file predicted.
over=""
sum=0
slowest=0
while read -r s n predicted; do
  took="$(cat "$LOGDIR/seconds-$s")"
  coverage="$(grep -h '^coverage:' "$LOGDIR/shard-$s.log" | tail -1 || true)"
  echo "shard $s: $n tests, predicted ${predicted}s, took ${took}s; $coverage"
  sum=$((sum + took))
  if [ "$took" -gt "$slowest" ]; then slowest="$took"; fi
  if [ "$took" -gt "$BUDGET" ]; then over="$over $s"; fi
done <"$LOGDIR/plan.txt"
echo "slowest shard ${slowest}s of a ${BUDGET}s budget; ${sum} shard-seconds in all"
echo "slowest tests (seconds, as scripts/ci_test_durations.sh weighs them):"
for ((s = 0; s < SHARDS; s++)); do
  echo "::group::shard $s"
  cat "$LOGDIR/shard-$s.log"
  echo "::endgroup::"
done | "$ROOT/scripts/ci_test_durations.sh" | awk '!/^#/ && ++n <= 15 { print "  " $0 }'

if [ -n "$over" ]; then
  msg="cmd/json2pptx shard(s)$over took more than ${BUDGET}s (SHARD_BUDGET_SECONDS); the slowest took ${slowest}s. The CI step is killed at its time limit, so act now: cut what the slowest tests above iterate over in -short mode (two templates, not all of them), and refresh scripts/ci_test_durations.txt with scripts/ci_test_durations.sh so the shards rebalance."
  if [ "$ENFORCE" = "1" ]; then
    echo "::error::$msg"
    exit 1
  fi
  echo "::warning::$msg (not enforced: SHARD_BUDGET_ENFORCE is not 1)"
fi
