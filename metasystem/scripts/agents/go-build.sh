#!/usr/bin/env bash
# The one fenced, pinned, stamped engine build (go-production-grade Phase
# 0a): the gate and adoption both build through here, so no second unfenced
# path can swap bin/metasystem under a live gate run. CGO is pinned off so
# the binary's link portability is deliberate, and the commit stamp makes
# operational artifacts self-attest. The fence exempts this process's own
# chain, so the gate calling this script never blocks itself.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"

# --out PATH is the PROOF build: compile the engine to PATH and leave
# bin/metasystem untouched. The landing boundary uses it because a
# supervision-armed checkout fingerprints the live binary — a commit-time
# swap under an armed watch is exactly what the fingerprint refuses
# (found live: the benchmark kit's wrapped-commit probe rebuilt the
# provisioned target's engine and its preflight then refused).
proof_out=
if [[ "${1:-}" == --out ]]; then
  [[ $# -ge 2 && -n "$2" ]] || { echo "go-build: --out needs a path" >&2; exit 2; }
  proof_out=$2
  shift 2
fi

command -v go >/dev/null 2>&1 \
  || { echo "go-build: no go toolchain on PATH; the engine cannot be built" >&2; exit 1; }

if [[ -z "$proof_out" && "${METASYSTEM_ALLOW_CONCURRENT_GATE:-0}" != 1 && -x "$root/bin/metasystem" ]]; then
  fence_rc=0
  "$root/bin/metasystem" gate fence --root "$root" --self-pid $$ || fence_rc=$?
  if [[ "$fence_rc" == 1 ]]; then
    echo "go-build: a live gate run owns this checkout; rebuilding now would swap its binary mid-run (METASYSTEM_ALLOW_CONCURRENT_GATE=1 overrides)" >&2
    exit 1
  fi
fi

# The stamp is the enclosing commit only while the compiled ENGINE surface
# matches HEAD. A changed or untracked engine path makes the default stamp a
# development stamp, which automatic enrollment refuses. The witness path
# overrides the default with the judged engine-input digest. VCS stamping is
# pinned OFF either way: the explicit stamp is the attestation, and implicit
# repository metadata made equal source build unequal binaries.
if [[ -n "${METASYSTEM_BUILD_STAMP:-}" ]]; then
  commit=$METASYSTEM_BUILD_STAMP
else
  commit=$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)
  if [[ "$commit" != unknown ]]; then
    engine_prefix=$(git -C "$root" rev-parse --show-prefix)
    engine_prefix=${engine_prefix%/}
    engine_changes=$(mktemp "${TMPDIR:-/tmp}/metasystem-engine-changes.XXXXXX")
    {
      git -C "$root" diff --name-only --no-renames -z HEAD --
      git -C "$root" ls-files --others --exclude-standard --full-name -z
    } | go run ./cmd/metasystem behavior-surface select --projection ENGINE --prefix "$engine_prefix" --nul >"$engine_changes" || {
      engine_change_rc=$?
      rm -f "$engine_changes"
      echo "go-build: cannot classify the working tree against the compiled ENGINE policy" >&2
      exit "$engine_change_rc"
    }
    if [[ -s "$engine_changes" ]]; then
      commit="dev-$commit-dirty"
    fi
    rm -f "$engine_changes"
  fi
fi
mkdir -p bin
# Build beside the target and rename over it: go build refuses to overwrite
# a non-object file (exactly the stale/foreign case this script exists to
# replace), and the atomic rename never leaves a half-written binary where
# a live process might exec it.
if [[ -n "$proof_out" ]]; then
  CGO_ENABLED=0 go build -buildvcs=false \
    -ldflags "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=$commit" \
    -o "$proof_out" ./cmd/metasystem \
    || { echo "go-build: build failed" >&2; exit 1; }
  echo "go-build: proof engine @ $commit (CGO_ENABLED=0); bin/metasystem untouched"
  exit 0
fi
staging="bin/.metasystem.build.$$"
trap 'rm -f "$staging"' EXIT
CGO_ENABLED=0 go build -buildvcs=false \
  -ldflags "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=$commit" \
  -o "$staging" ./cmd/metasystem \
  || { echo "go-build: build failed" >&2; exit 1; }
mv -f "$staging" bin/metasystem
echo "go-build: bin/metasystem @ $commit (CGO_ENABLED=0)"
