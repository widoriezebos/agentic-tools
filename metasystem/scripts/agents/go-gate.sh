#!/usr/bin/env bash
# The Go engine gate (plans/go-migration.md): gofmt, vet, the race-detector
# unit suite, and the build — run AHEAD of the shell fixtures so a broken
# binary fails fast and the fixtures that drive it have something to drive.
# Sourced by validate-metasystem.sh; also runnable standalone.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"

# A checkout without a Go module has not adopted the Go engine yet (adopt.sh
# ships it as a Phase 4 port, plans/go-migration.md). It runs pure
# shell/python, so the Go gate is a no-op there — SKIP, never fail. This is
# what keeps an adopted target's own suite green before the engine arrives.
# Identity, not existence: an adopted target may be an ordinary Go
# repository with a module of its own, and running the template's Go
# checks against a foreign module would fail its required validation.
if ! grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' "$root/go.mod"; then
  echo "go gate: not the metasystem source tree (adopted checkouts carry only the engine binary); skipped" >&2
  exit 0
fi

# From here the module exists, so a missing toolchain is a real failure: a
# committed engine that cannot be built must stop the gate.
if ! command -v go >/dev/null 2>&1; then
  echo "go gate: go.mod present but no go toolchain on PATH; the committed engine cannot be built" >&2
  exit 1
fi

# Rebuilding bin/metasystem while a FOREIGN gate run is live would swap the
# binary under that run mid-flight. The suite that sourced or spawned this
# gate is its own run — the fence exempts this process's chain — so only a
# standalone rebuild against someone else's live run is refused here.
# Only a REAL block (exit 1) refuses; a binary too old to know the verb
# (exit 2) must not stop the rebuild that would teach it.
if [[ "${METASYSTEM_ALLOW_CONCURRENT_GATE:-0}" != 1 && -x "$root/bin/metasystem" ]]; then
  gate_fence_rc=0
  "$root/bin/metasystem" gate fence --root "$root" --self-pid $$ || gate_fence_rc=$?
  if [[ "$gate_fence_rc" == 1 ]]; then
    echo "go gate: a live gate run owns this checkout; rebuilding now would swap its binary mid-run (METASYSTEM_ALLOW_CONCURRENT_GATE=1 overrides)" >&2
    exit 1
  fi
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
