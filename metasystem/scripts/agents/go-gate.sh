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
  # Three states, not two: Go SOURCE present without the metasystem module
  # line is a damaged template, and skipping would validate green with zero
  # Go checks — fail loudly instead.
  # No lexing: the discriminator is the presence of the engine's own
  # source files, which only the metasystem template carries — adopted
  # targets receive no Go source, and a foreign project would need both
  # exact paths to collide, where the failure is a loud refusal, not
  # silent breakage.
  if [[ -f "$root/internal/missionrunner/stoploss.go" || -f "$root/internal/mission/ledger.go" ]]; then
    echo "go gate: metasystem Go source present but go.mod does not declare the metasystem module — damaged template, refusing to skip" >&2
    exit 1
  fi
  echo "go gate: not the metasystem source tree (adopted checkouts carry only the engine binary); skipped" >&2
  exit 0
fi

# From here the module exists, so a missing toolchain is a real failure: a
# committed engine that cannot be built must stop the gate.
if ! command -v go >/dev/null 2>&1; then
  echo "go gate: go.mod present but no go toolchain on PATH; the committed engine cannot be built" >&2
  exit 1
fi

# ---- The boundary-scoped gate witness (D33) ----------------------------
# One validation run, one gate: the outer suite runs this gate inside an
# extracted git-archive-HEAD snapshot with METASYSTEM_GATE_WITNESS_WRITE
# set; nested delivery-contract runs hand the resulting witness back via
# METASYSTEM_GATE_WITNESS and skip re-proving byte-identical content.
# Everything here fails toward the full gate: any doubt, run it all.

# The digest covers what the gate consumes: the sorted (path, kind, mode,
# content) manifest of the engine inputs, the gate script itself, and the
# toolchain identity. git hash-object works outside a repository, so the
# same computation runs in snapshots and staged targets.
gate_input_digest() {
  {
    go version
    go env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN
    find cmd internal scripts/agents go.mod go.sum \( -type f -o -type l \) 2>/dev/null \
      | LC_ALL=C sort | while IFS= read -r entry; do
        if [[ -L "$entry" ]]; then
          printf '%s L - %s\n' "$entry" "$(readlink "$entry")"
        else
          mode=$(stat -f '%Lp' "$entry" 2>/dev/null || stat -c '%a' "$entry")
          printf '%s F %s %s\n' "$entry" "$mode" "$(git hash-object "$entry")"
        fi
      done
  } | { shasum -a 256 2>/dev/null || sha256sum; } | cut -d' ' -f1
}

# A tree where ./... could reach beyond the hashed closure refuses witness
# use in BOTH directions: go.work or vendor/ present, replace directives,
# or any package outside cmd/ and internal/ (and the root package dir).
witness_refusal() { # prints the reason and returns 0 when refused
  [[ ! -e go.work && ! -e "$root/../go.work" ]] || { echo "go.work present"; return 0; }
  [[ ! -d vendor ]] || { echo "vendor/ present"; return 0; }
  ! grep -q '^replace' go.mod || { echo "go.mod carries replace directives"; return 0; }
  local outside
  outside=$(go list -f '{{.Dir}}' ./... 2>/dev/null \
    | grep -v -e "^$root/cmd" -e "^$root/internal" -e "^$root\$" || true)
  [[ -z "$outside" ]] || { echo "packages outside the hashed closure: $outside"; return 0; }
  return 1
}

witness_fenced_off() { # seed and force runs never read or write witnesses
  [[ "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" == 1 || "${METASYSTEM_GATE_FORCE:-0}" == 1 ]]
}

# The accept decision, separated so fixtures can probe it without paying a
# full gate: exit 0 accept, 3 refuse (reason on stderr). Sanitization per
# the design: the state root and run id come from the controller's own
# environment, never from the witness; the witness must be a non-symlink
# regular file, 0600, under the 0700 controller state dir.
witness_acceptable() {
  local witness=${METASYSTEM_GATE_WITNESS:-} state_root=${METASYSTEM_GATE_WITNESS_ROOT:-} run=${METASYSTEM_GATE_WITNESS_RUN:-}
  [[ -n "$witness" && -n "$state_root" && -n "$run" ]] || { echo "witness handoff incomplete" >&2; return 3; }
  [[ "${METASYSTEM_DELIVERY_CONTRACT:-0}" == 1 ]] || { echo "witness honored only in delivery-contract runs" >&2; return 3; }
  ! witness_fenced_off || { echo "seed or force run; witness fenced off" >&2; return 3; }
  local canonical_root canonical_witness
  canonical_root=$(cd "$state_root" 2>/dev/null && pwd -P) || { echo "witness state root unreadable" >&2; return 3; }
  [[ ! -L "$witness" && -f "$witness" ]] || { echo "witness is not a plain regular file" >&2; return 3; }
  canonical_witness="$(cd "$(dirname "$witness")" 2>/dev/null && pwd -P)/$(basename "$witness")"
  [[ "$canonical_witness" == "$canonical_root"/* ]] || { echo "witness lies outside the controller state root" >&2; return 3; }
  local dir_mode file_mode
  dir_mode=$(stat -f '%Lp' "$canonical_root" 2>/dev/null || stat -c '%a' "$canonical_root")
  file_mode=$(stat -f '%Lp' "$canonical_witness" 2>/dev/null || stat -c '%a' "$canonical_witness")
  [[ "$dir_mode" == 700 && "$file_mode" == 600 ]] || { echo "witness permissions are not 0700/0600" >&2; return 3; }
  local recorded_run recorded_digest
  recorded_run=$(sed -n 's/.*"runId":"\([^"]*\)".*/\1/p' "$canonical_witness")
  recorded_digest=$(sed -n 's/.*"digest":"\([^"]*\)".*/\1/p' "$canonical_witness")
  [[ "$recorded_run" == "$run" && -n "$recorded_digest" ]] || { echo "witness run identity mismatch" >&2; return 3; }
  local refusal
  if refusal=$(witness_refusal); then echo "witness refused here: $refusal" >&2; return 3; fi
  local local_digest
  local_digest=$(gate_input_digest)
  [[ "$local_digest" == "$recorded_digest" ]] || { echo "witness digest mismatch (theirs ${recorded_digest:0:8}, ours ${local_digest:0:8})" >&2; return 3; }
  echo "$recorded_digest"
  return 0
}

if [[ "${1:-}" == --witness-check-only ]]; then
  digest=$(witness_acceptable) || exit 3
  echo "witness acceptable: ${digest:0:8}"
  exit 0
fi

if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]]; then
  if digest=$(witness_acceptable); then
    # Compilation and the self-gating proof stay real: the binary is
    # rebuilt from the accepted content and stamped with its digest.
    METASYSTEM_BUILD_STAMP="witness-${digest:0:12}" bash scripts/agents/go-build.sh \
      || { echo "go gate: build failed under an accepted witness" >&2; exit 1; }
    echo "go gate: PASSED (outer witness ${digest:0:8}, this boundary)"
    exit 0
  fi
  echo "go gate: witness not accepted; running the full gate" >&2
fi

# The snapshot-gated invocation computes its digest up front: the snapshot
# cannot change, and the build below stamps the binary with this digest
# (the snapshot has no git history to stamp from).
witness_digest=
if [[ -n "${METASYSTEM_GATE_WITNESS_WRITE:-}" ]] && ! witness_fenced_off && ! witness_refusal >/dev/null; then
  witness_digest=$(gate_input_digest)
  export METASYSTEM_BUILD_STAMP="witness-${witness_digest:0:12}"
  # A fresh snapshot has no bin/metasystem yet, and the binary-driven
  # fixtures (preflight and friends) SKIP without it — the gate builds
  # only after the tests, so a snapshot would deterministically measure
  # the skipped-fixture coverage (the 65.7% trap) and refuse its own
  # ratchet. Build first; the fence exempts this gate's own chain.
  bash scripts/agents/go-build.sh >/dev/null \
    || { echo "go gate: snapshot pre-build failed" >&2; exit 1; }
  # And compile the covered test binaries before the timed tests run:
  # -cover instrumentation is path-dependent, so a warm plain-race cache
  # still leaves a fresh snapshot's covered build cold, and compiling
  # thirty packages during the timing fixtures starves them.
  go test -race -cover -count=1 -run NoSuchTestEver ./internal/... >/dev/null 2>&1 || true
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
# engineering standard requires it. Its exit status is captured — a missing
# or crashing gofmt refuses the gate instead of passing silently
# (go-production-grade B8).
gofmt_rc=0
unformatted=$(gofmt -l internal cmd 2>&1) || gofmt_rc=$?
if [[ "$gofmt_rc" != 0 ]]; then
  echo "go gate: gofmt itself failed (status $gofmt_rc):" >&2
  printf '%s\n' "$unformatted" >&2
  exit 1
fi
if [[ -n "$unformatted" ]]; then
  echo "go gate: gofmt would change these files:" >&2
  printf '  %s\n' $unformatted >&2
  exit 1
fi

go vet ./... || { echo "go gate: go vet failed" >&2; exit 1; }

# The standing Linux signal (go-production-grade Phase 1, P3): a darwin-only
# regression is invisible until someone tries, so both Linux architectures
# cross-compile in every gate run. Seconds of cost, no runner needed (KI-10).
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./... \
  || { echo "go gate: linux/amd64 cross-build failed" >&2; exit 1; }
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./... \
  || { echo "go gate: linux/arm64 cross-build failed" >&2; exit 1; }

# staticcheck and govulncheck, pinned (go-production-grade Phase 0d, human
# decision 2026-08-11). Versions are frozen here; a network-unreachable tool
# run fails the gate loudly rather than skipping silently.
go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./... \
  || { echo "go gate: staticcheck 2025.1 refused (or could not run)" >&2; exit 1; }
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./... \
  || { echo "go gate: govulncheck v1.1.4 refused (or could not run)" >&2; exit 1; }

# The coverage floor is executable, not prose (plans/kill-shell.md, the
# production-grade 0c ratchet pulled forward): the test output feeds the
# audit verb, whose baseline only ever rises. The cmd package is exempt
# with its reason recorded in the baseline file. The consult runs through
# the freshly built temporary binary below, because THIS invocation's
# rebuild is what always-rebuild means.
coverage_log=$(mktemp)
go test -race -cover ./internal/... | tee "$coverage_log" || {
  # Evidence beats disk (the suite's own rule): a transient test failure
  # with its log deleted is undiagnosable — tonight's nested-gate flake
  # was exactly that. Keep the failing run's output where the suite keeps
  # its evidence.
  keep="artifacts/agents/gate-failures/$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  mkdir -p "$(dirname "$keep")"
  mv "$coverage_log" "$keep" 2>/dev/null || true
  echo "go gate: unit tests failed (output kept: $keep)" >&2
  exit 1
}

# cmd's own tests run too. The package is coverage-ratchet-exempt as thin
# wiring, but exempt-from-floors never meant exempt-from-running: a broken
# cmd test rode through this gate unseen on 2026-08-14 because the race run
# above scopes to ./internal/... (cli-10 follow-up).
go test -race ./cmd/... >/dev/null || {
  echo "go gate: cmd tests failed" >&2
  exit 1
}

# Build the binary the shell fixtures and wrappers exec, through the one
# shared fenced build (go-production-grade Phase 0a): stamped with its
# source commit so its artifacts self-attest (GO-MIG-R4-009), CGO pinned
# off. This gate run already holds the fence; the build's own consult is
# exempt inside this process chain.
commit=${METASYSTEM_BUILD_STAMP:-$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)}
bash scripts/agents/go-build.sh \
  || { rm -f "$coverage_log"; echo "go gate: build failed" >&2; exit 1; }

# The freshly built binary judges the coverage ratchet, joined against an
# independent package inventory so a testless package cannot hide — go test
# prints no coverage line for a package with no test files (B8). Floors are
# per platform: darwin floors are the committed measurements; a linux run
# uses its own baseline file, seeded at Phase 1 of the production-grade
# plan via METASYSTEM_COVERAGE_RATCHET_SEED=1 (every other check enforced,
# the floor join skipped, coverage collected for seeding).
pkg_list=$(mktemp)
go list ./internal/... >"$pkg_list" \
  || { rm -f "$coverage_log" "$pkg_list"; echo "go gate: go list failed; cannot join the coverage inventory" >&2; exit 1; }
ratchet_baseline=scripts/agents/coverage-ratchet.json
if [[ "$(uname -s)" == Linux ]]; then
  ratchet_baseline=scripts/agents/coverage-ratchet-linux.json
fi
if [[ "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" == 1 ]]; then
  echo "go gate: coverage ratchet in SEED mode; floors not enforced this run (bootstrap pass one)" >&2
elif [[ ! -f "$ratchet_baseline" ]]; then
  rm -f "$coverage_log" "$pkg_list"
  echo "go gate: no coverage baseline for this platform ($ratchet_baseline); run the two-pass seed bootstrap first" >&2
  exit 1
else
  bin/metasystem audit coverage-ratchet --baseline "$ratchet_baseline" --input "$coverage_log" --packages "$pkg_list" \
    || { rm -f "$coverage_log" "$pkg_list"; echo "go gate: coverage ratchet refused" >&2; exit 1; }
fi
rm -f "$coverage_log" "$pkg_list"

# The witness write (D33): only the controller's snapshot-gated invocation
# sets METASYSTEM_GATE_WITNESS_WRITE, and it runs this gate inside an
# extracted git-archive-HEAD tree, so the digest below describes exactly
# the bytes adoption stages. Seed and force runs never write.
if [[ -n "${METASYSTEM_GATE_WITNESS_WRITE:-}" ]] && ! witness_fenced_off; then
  if refusal=$(witness_refusal); then
    echo "go gate: witness not written ($refusal)" >&2
  else
    [[ -n "$witness_digest" ]] || witness_digest=$(gate_input_digest)
    umask 077
    printf '{"digest":"%s","runId":"%s","passedAt":"%s","goVersion":"%s","ratchetBaseline":"%s","summary":"full gate in HEAD snapshot"}\n' \
      "$witness_digest" "${METASYSTEM_GATE_WITNESS_RUN:-}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$(go version | tr -d '"')" "$ratchet_baseline" >"$METASYSTEM_GATE_WITNESS_WRITE"
    chmod 600 "$METASYSTEM_GATE_WITNESS_WRITE"
    echo "go gate: witness written (${witness_digest:0:8})"
  fi
fi

echo "go gate: PASSED (gofmt, vet, race tests, coverage ratchet, build @ $commit)"
