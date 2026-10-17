#!/usr/bin/env bash
# Run canonical Linux byte-golden checks with the local render-toolchain image.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${DETERMINISM_IMAGE:-lo24-rt}"
ARCH="$(docker image inspect --format '{{.Architecture}}' "$IMAGE")"
SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"' EXIT
cd "$ROOT"
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go test -c -o "$SCRATCH/json2pptx.test" ./cmd/json2pptx
docker run --rm --entrypoint /runner/json2pptx.test \
  -e JSON2PPTX_SKIP_LOCAL_TEMPLATES=1 -e CI=1 \
  -v "$ROOT:$ROOT:ro" -v "$SCRATCH:/runner:ro" -w "$ROOT/cmd/json2pptx" \
  "$IMAGE" -test.run '^TestDeterministicCorpus$' -test.count=1 -test.timeout=15m -test.v
