#!/usr/bin/env bash
# The Go engine gate (plans/go-migration.md): gofmt, vet, the race-detector
# unit suite, and the build — run AHEAD of the shell fixtures so a broken
# binary fails fast and the fixtures that drive it have something to drive.
# Sourced by validate-metasystem.sh; also runnable standalone.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"

if ! command -v go >/dev/null 2>&1; then
  echo "go gate: no go toolchain on PATH; the Go engine cannot be built or tested" >&2
  exit 1
fi

# gofmt is a hard gate: unformatted code is a review-noise source and the
# engineering standard requires it.
unformatted=$(gofmt -l internal cmd 2>/dev/null || true)
if [[ -n "$unformatted" ]]; then
  echo "go gate: gofmt would change these files:" >&2
  printf '  %s\n' $unformatted >&2
  exit 1
fi

go vet ./... || { echo "go gate: go vet failed" >&2; exit 1; }

# The domain packages carry a coverage floor (the engineering standard);
# the cmd package is thin wiring the fixtures exercise, so it is built and
# vetted but not floored here.
go test -race -cover ./internal/... || { echo "go gate: unit tests failed" >&2; exit 1; }

# Build the binary the shell fixtures and wrappers exec, stamped with its
# source commit so its artifacts self-attest (GO-MIG-R4-009).
commit=$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)
mkdir -p bin
go build -ldflags "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=$commit" \
  -o bin/metasystem ./cmd/metasystem \
  || { echo "go gate: build failed" >&2; exit 1; }

echo "go gate: PASSED (gofmt, vet, race tests, build @ $commit)"
