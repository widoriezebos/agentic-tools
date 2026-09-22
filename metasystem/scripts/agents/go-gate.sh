#!/usr/bin/env bash
# The Go engine gate (records/misc/go-migration.md), stages ordered cheapest-first
# by measured cost: gofmt, vet, staticcheck, the linux cross-builds, and
# govulncheck — all seconds on a warm cache — run before the race-detector
# suites, the build, and the coverage ratchet, which own the minutes. The
# whole gate runs AHEAD of the shell fixtures so a broken binary fails fast
# and the fixtures that drive it have something to drive.
# Sourced by validate-metasystem.sh; also runnable standalone.
#
# Fast mode (go-gate.sh --fast) runs only the dependency and parallel ratchets and static
# stages — gofmt, vet, staticcheck, refusal register — plus the engine build:
# seconds end to end, for tight edit loops. It is not a landing gate: no race
# tests, no cross-builds, no govulncheck, no coverage ratchet, and it refuses the
# witness protocol outright. The full gate remains the landing requirement.
# The switch is the explicit flag only, never an environment variable: an
# exported variable outlives the edit loop it served and would silently
# weaken the very suites that make this gate a landing requirement.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"

# The fast switch is argument-only (see the header): every caller states
# fast mode at the call site or gets the full gate.
gate_fast=0
gate_proof_out=
gate_witness_check_only=0
gate_goal=
gate_cap_min=
while (($#)); do
  case "$1" in
    --fast) gate_fast=1; shift ;;
    --proof-out)
      [[ $# -ge 2 && -n "$2" ]] || { echo "go gate: --proof-out needs a path" >&2; exit 2; }
      gate_proof_out=$2; shift 2 ;;
    --witness-check-only) gate_witness_check_only=1; shift ;;
    --goal)
      [[ $# -ge 2 && -n "$2" ]] || { echo "go gate: --goal needs an accepted goal id" >&2; exit 2; }
      gate_goal=$2; shift 2 ;;
    --cap-min)
      [[ $# -ge 2 && "$2" =~ ^[1-9][0-9]*$ ]] || { echo "go gate: --cap-min needs positive minutes" >&2; exit 2; }
      gate_cap_min=$2; shift 2 ;;
    *) echo "go gate: unknown argument $1" >&2; exit 2 ;;
  esac
done
if [[ "$gate_witness_check_only" == 1 && ( "$gate_fast" == 1 || -n "$gate_proof_out" ) ]]; then
  echo "go gate: --witness-check-only is a probe and combines with no other mode" >&2
  exit 2
fi
if [[ -n "$gate_proof_out" && "$gate_fast" != 1 ]]; then
  echo "go gate: --proof-out is a fast-mode flag (the landing boundary's side-effect-free build proof)" >&2
  exit 2
fi
if [[ "$gate_fast" == 1 && ( -n "$gate_goal" || -n "$gate_cap_min" ) ]]; then
  echo "go gate: proof reservation flags apply only to the full gate" >&2
  exit 2
fi
gate_witness_reuse_out=${METASYSTEM_GATE_WITNESS_REUSE_OUT:-}
unset METASYSTEM_GATE_WITNESS_REUSE_OUT

# The full proof admits the environment it actually executes. Pin readonly
# module resolution before the outer proof reservation, not only inside a
# later frozen-snapshot branch, so coverage producers and retained consumers
# identify one real Go environment.
if [[ "$gate_fast" != 1 && "$gate_witness_check_only" != 1 ]]; then
  export GOFLAGS=-mod=readonly
fi

# A standalone full gate enters the same retained proof owner as validation
# and adoption. Only the Go owner may recognize a worker; ambient progress or
# locator strings do not bypass the wrapper.
proof_worker=0
proof_auth_bin=${METASYSTEM_PROOF_AUTH_BIN:-$root/bin/metasystem}
proof_auth_command=("$proof_auth_bin")
if [[ -x "$proof_auth_bin" ]] && "${proof_auth_command[@]}" proof-run worker-authorized --root "$root" >/dev/null 2>&1; then
  proof_worker=1
fi
# An enrolled engine may be older than the candidate source executing this
# admitted command group. Keep the actual detached root and ask that source's
# worker verifier to check the inherited attempt and live process custody.
if [[ "$proof_worker" != 1 && -n "${METASYSTEM_PROOF_ATTEMPT:-}" && -n "${METASYSTEM_PROOF_CONTROL_ROOT:-}" \
  && -f "$root/go.mod" ]] && grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' "$root/go.mod" \
  && command -v go >/dev/null 2>&1; then
  proof_auth_command=(env GOFLAGS=-mod=readonly GOWORK=off CGO_ENABLED=0 go run ./cmd/metasystem)
  if "${proof_auth_command[@]}" proof-run worker-authorized --root "$root" >/dev/null 2>&1; then
    proof_worker=1
  fi
fi
if [[ "${METASYSTEM_GO_GATE_RELAUNCHED:-0}" == 1 ]]; then
  if [[ "$proof_worker" != 1 ]]; then
    echo "go gate: relaunched child is not an authorized proof worker" >&2
    exit 1
  fi
  unset METASYSTEM_GO_GATE_RELAUNCHED
fi
if [[ "$gate_fast" != 1 && "$gate_witness_check_only" != 1 \
  && "$proof_worker" != 1 \
  && -f "$root/go.mod" ]] \
  && grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' "$root/go.mod"; then
  proof_progress="$root/artifacts/agents/supervision/suite-progress.jsonl"
  proof_log="$root/artifacts/agents/supervision/suite-logs/go-gate-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  proof_tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-go-gate.XXXXXX")
  proof_engine="$proof_tmp/metasystem"
  go build -o "$proof_engine" ./cmd/metasystem
  proof_launcher=("$proof_engine")
  proof_banner=$("${proof_launcher[@]}" proof-run banner --suite go-gate --root "$root" \
    --progress "$proof_progress" --log "$proof_log") \
    || { echo "go gate: could not prepare retained proof launch" >&2; exit 1; }
  proof_args=(proof-run launch --suite go-gate --root "$root" --conf "$root/metasystem.conf" \
    --progress "$proof_progress" --log "$proof_log" --banner "$proof_banner" --scope full --command-class go-gate)
  [[ -z "$gate_goal" ]] || proof_args+=(--goal "$gate_goal")
  [[ -z "$gate_cap_min" ]] || proof_args+=(--cap-min "$gate_cap_min")
  exec "${proof_launcher[@]}" "${proof_args[@]}" --tmp "$proof_tmp" -- \
    env METASYSTEM_GO_GATE_RELAUNCHED=1 METASYSTEM_PROOF_AUTH_BIN="$proof_engine" \
      bash "$root/scripts/agents/go-gate.sh"
fi

# Fast mode is an edit-loop tool, not a landing gate: it must neither
# consume nor produce a witness, so a witness handoff arriving alongside it
# is a contradiction to refuse loudly — before any branch, including the
# adopted-checkout skip, can swallow it.
if [[ "$gate_fast" == 1 && ( -n "${METASYSTEM_GATE_WITNESS:-}" || -n "${METASYSTEM_GATE_WITNESS_WRITE:-}" ) ]]; then
  echo "go gate: fast mode cannot join the witness protocol; run the full gate" >&2
  exit 1
fi

# A checkout without a Go module has not adopted the Go engine yet (adopt.sh
# ships it as a Phase 4 port, records/misc/go-migration.md). It runs pure
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

# A forbidden script dependency is cheaper and more urgent than every test in
# the edit-loop gate. Run the current source's audit before any test process;
# its status is the refusal status rather than a generic static-stage failure.
if [[ "$gate_fast" == 1 ]]; then
  gate_dependency_rc=0
  gate_dependency_out=$(go run ./cmd/metasystem audit dependency-ratchet --root "$root" 2>&1) \
    || gate_dependency_rc=$?
  if [[ "$gate_dependency_rc" != 0 ]]; then
    printf '%s\n' "$gate_dependency_out" >&2
    exit "$gate_dependency_rc"
  fi
  gate_parallel_rc=0
  gate_parallel_out=$(go run ./cmd/metasystem audit parallel-ratchet --root "$root" 2>&1) \
    || gate_parallel_rc=$?
  if [[ "$gate_parallel_rc" != 0 ]]; then
    printf '%s\n' "$gate_parallel_out" >&2
    exit "$gate_parallel_rc"
  fi
fi

# A STANDALONE go-gate run registers itself (goal-system GOAL-17: an
# unrecorded supported gate was invisible to the turn-end scan). Under the
# suite the parent already holds the serving root's marker; this adds one
# for this process, which is correct either way — markers are per-pid and
# die with their process. Skipped only where no binary exists yet (the
# bootstrap residual, bounded by this very gate's build step).
go_gate_marker=
consumer_freeze_snapshot=
consumer_live_root=$root
cleanup_consumer_freeze() {
  local snapshot=${consumer_freeze_snapshot:-}
  [[ -n "$snapshot" ]] || return 0
  consumer_freeze_snapshot=
  ( cd "$consumer_live_root" && go run ./cmd/metasystem gate witness-freeze --cleanup "$snapshot" )
}
go_gate_name=go-gate.sh
if [[ "$gate_fast" == 1 ]]; then
  go_gate_name="go-gate.sh --fast"
fi
if [[ -x "$root/bin/metasystem" ]]; then
  go_gate_marker=$("$root/bin/metasystem" gate register --root "$root" \
    --gate "$go_gate_name" --pid $$) || {
    echo "go gate: registration failed; refusing to run invisibly" >&2
    exit 1
  }
fi
trap '[[ -z "$go_gate_marker" ]] || rm -f "$go_gate_marker"; cleanup_consumer_freeze >/dev/null 2>&1 || true; [[ -z "${gate_build_scratch:-}" ]] || rm -f "$gate_build_scratch"' EXIT

# ---- The boundary-scoped gate witness (D33) ----------------------------
# One validation run, one gate: the outer suite runs this gate inside an
# set; descendant validations under that controller hand the resulting
# witness back and skip re-proving byte-identical ENGINE content.
# Everything here fails toward the full gate: any doubt, run it all.

# The engine built from the judged bytes owns every byte projection. Its JSON
# report names the policy version, projection, and endpoint; version equality
# is checked explicitly beside digest equality. Toolchain identity is an
# independent equality witness and is never mixed into a byte digest.
gate_surface_identity() { # ENGINE|PAYLOAD, endpoint, optional source path manifest -> version digest
  local report digest version manifest=${3:-} manifest_args=()
  [[ -z "$manifest" ]] || manifest_args=(--paths-from "$manifest")
  report=$(go run ./cmd/metasystem behavior-surface digest \
    --root "$root" --projection "$1" --endpoint "$2" "${manifest_args[@]+"${manifest_args[@]}"}") || return 1
  digest=$(printf '%s\n' "$report" | sed -n 's/.*"surfaceDigest":"\([a-f0-9]*\)".*/\1/p')
  version=$(printf '%s\n' "$report" | sed -n 's/.*"policyVersion":\([0-9][0-9]*\).*/\1/p')
  [[ "$digest" =~ ^[a-f0-9]{64}$ && "$version" =~ ^[1-9][0-9]*$ ]] || return 1
  printf '%s %s\n' "$version" "$digest"
}

gate_surface_digest() { # compatibility for the build stamp path
  local version digest
  read -r version digest < <(gate_surface_identity "$@") || return 1
  printf '%s\n' "$digest"
}

gate_toolchain_identity() {
  {
    go version
    go env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN
  } | { shasum -a 256 2>/dev/null || sha256sum; } | cut -d' ' -f1
}

frozen_toolchain_refusal() {
  [[ " ${GOFLAGS:-} " =~ [[:space:]]-(modfile|overlay)(=|[[:space:]]) ]] \
    && { echo "GOFLAGS may not contain -modfile or -overlay"; return 0; }
  return 1
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
  local consumer_scope=${METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE:-}
  [[ -n "$witness" && -n "$state_root" && -n "$run" ]] || { echo "witness handoff incomplete" >&2; return 3; }
  case "$consumer_scope" in
    ENGINE|DELIVERY) ;;
    *) echo "witness consumer scope must be explicitly ENGINE or DELIVERY" >&2; return 3 ;;
  esac
  ! witness_fenced_off || { echo "seed or force run; witness fenced off" >&2; return 3; }
  local canonical_root canonical_witness
  canonical_root=$(cd "$state_root" 2>/dev/null && pwd -P) || { echo "witness state root unreadable" >&2; return 3; }
  [[ ! -L "$witness" && -f "$witness" ]] || { echo "witness is not a plain regular file" >&2; return 3; }
  canonical_witness="$(cd "$(dirname "$witness")" 2>/dev/null && pwd -P)/$(basename "$witness")"
  [[ "$canonical_witness" == "$canonical_root"/* ]] || { echo "witness lies outside the controller state root" >&2; return 3; }
  local dir_mode file_mode
  dir_mode=$(stat -c '%a' "$canonical_root" 2>/dev/null || stat -f '%Lp' "$canonical_root")
  file_mode=$(stat -c '%a' "$canonical_witness" 2>/dev/null || stat -f '%Lp' "$canonical_witness")
  [[ "$dir_mode" == 700 && "$file_mode" == 600 ]] || { echo "witness permissions are not 0700/0600" >&2; return 3; }
  local recorded_run recorded_version recorded_digest recorded_payload recorded_toolchain recorded_payload_manifest recorded_manifest_digest
  local controller_pid controller_started_at controller_start_ticks controller_boot_id
  recorded_run=$(sed -n 's/.*"runId":"\([^"]*\)".*/\1/p' "$canonical_witness")
  recorded_version=$(sed -n 's/.*"policyVersion":\([0-9][0-9]*\).*/\1/p' "$canonical_witness")
  recorded_digest=$(sed -n 's/.*"engineDigest":"\([^"]*\)".*/\1/p' "$canonical_witness")
  recorded_payload=$(sed -n 's/.*"payloadDigest":"\([^"]*\)".*/\1/p' "$canonical_witness")
  recorded_toolchain=$(sed -n 's/.*"toolchainIdentity":"\([^"]*\)".*/\1/p' "$canonical_witness")
  recorded_payload_manifest=$(sed -n 's/.*"payloadManifest":"\([^"]*\)".*/\1/p' "$canonical_witness")
  recorded_manifest_digest=$(sed -n 's/.*"manifestDigest":"\([^"]*\)".*/\1/p' "$canonical_witness")
  controller_pid=$(sed -n 's/.*"controller":{"pid":\([0-9][0-9]*\).*/\1/p' "$canonical_witness")
  controller_started_at=$(sed -n 's/.*"controller":{"pid":[0-9][0-9]*,"startedAtSec":\([0-9][0-9]*\).*/\1/p' "$canonical_witness")
  controller_start_ticks=$(sed -n 's/.*"controller":{"pid":[0-9][0-9]*,"startedAtSec":[0-9][0-9]*,"startTicks":\([0-9][0-9]*\).*/\1/p' "$canonical_witness")
  controller_boot_id=$(sed -n 's/.*"controller":{"pid":[0-9][0-9]*,"startedAtSec":[0-9][0-9]*,"startTicks":[0-9][0-9]*,"bootId":"\([^"]*\)".*/\1/p' "$canonical_witness")
  [[ "$recorded_run" == "$run" && "$recorded_version" =~ ^[1-9][0-9]*$ \
    && "$recorded_digest" =~ ^[a-f0-9]{64}$ && "$recorded_toolchain" =~ ^[a-f0-9]{64}$ \
    && "$controller_pid" =~ ^[1-9][0-9]*$ && "$controller_started_at" =~ ^[1-9][0-9]*$ \
    && "$controller_start_ticks" =~ ^[0-9]+$ ]] \
    || { echo "witness run or projection identity is incomplete" >&2; return 3; }
  if (( controller_start_ticks > 0 )); then
    [[ -n "$controller_boot_id" ]] || { echo "witness controller pair identity is incomplete" >&2; return 3; }
  else
    [[ -z "$controller_boot_id" ]] || { echo "witness controller pair identity is malformed" >&2; return 3; }
  fi
  # This is an outsider boundary, not a same-user privilege boundary. A
  # same-user process can already replace the engine or this script; that
  # accepted risk gains no authority from the witness protocol.
  go run ./cmd/metasystem gate controller-descendant --consumer-pid $$ \
    --controller-pid "$controller_pid" --controller-started-at "$controller_started_at" \
    --controller-start-ticks "$controller_start_ticks" --controller-boot-id "$controller_boot_id" \
    >/dev/null 2>&1 \
    || { echo "witness consumer is not descended from its exact live controller identity" >&2; return 3; }
  local refusal
  if refusal=$(witness_refusal); then echo "witness refused here: $refusal" >&2; return 3; fi
  local local_version local_digest local_payload_version local_payload local_toolchain payload_manifest
  [[ "$recorded_manifest_digest" =~ ^[a-f0-9]{64}$ ]] \
    || { echo "witness has no compatible complete-project manifest identity" >&2; return 3; }
  go run ./cmd/metasystem gate witness-verify --root "$root" --witness "$canonical_witness" \
    >/dev/null 2>&1 \
    || { echo "full-tree manifest digest mismatch between witness and consumer" >&2; return 3; }

  read -r local_version local_digest < <(gate_surface_identity ENGINE "witness consumer $root") \
    || { echo "consumer ENGINE surface identity could not be computed" >&2; return 3; }
  local_toolchain=$(gate_toolchain_identity) || return 3
  [[ "$local_version" == "$recorded_version" ]] \
    || { echo "behavior-surface policy version mismatch between witness and consumer (theirs $recorded_version, ours ENGINE=$local_version)" >&2; return 3; }
  [[ "$local_digest" == "$recorded_digest" ]] \
    || { echo "ENGINE surface digest mismatch between witness and consumer (theirs ${recorded_digest:0:8}, ours ${local_digest:0:8})" >&2; return 3; }
  [[ "$local_toolchain" == "$recorded_toolchain" ]] \
    || { echo "toolchain identity mismatch between witness and consumer (theirs ${recorded_toolchain:0:8}, ours ${local_toolchain:0:8})" >&2; return 3; }
  if [[ "$consumer_scope" == DELIVERY ]]; then
    [[ "$recorded_payload" =~ ^[a-f0-9]{64}$ && -n "$recorded_payload_manifest" ]] \
      || { echo "witness PAYLOAD identity is incomplete" >&2; return 3; }
    [[ "$recorded_payload_manifest" != */* && "$recorded_payload_manifest" != .* ]] \
      || { echo "witness payload manifest name is unsafe" >&2; return 3; }
    payload_manifest="$canonical_root/$recorded_payload_manifest"
    [[ ! -L "$payload_manifest" && -f "$payload_manifest" ]] \
      || { echo "witness payload manifest is not a plain regular file" >&2; return 3; }
    local manifest_mode
    manifest_mode=$(stat -c '%a' "$payload_manifest" 2>/dev/null || stat -f '%Lp' "$payload_manifest")
    [[ "$manifest_mode" == 600 ]] || { echo "witness payload manifest permission is not 0600" >&2; return 3; }
    read -r local_payload_version local_payload < <(gate_surface_identity PAYLOAD "delivery consumer $root" "$payload_manifest") || return 3
    [[ "$local_payload_version" == "$recorded_version" ]] \
      || { echo "behavior-surface policy version mismatch between witness and delivery PAYLOAD (theirs $recorded_version, ours $local_payload_version)" >&2; return 3; }
    [[ "$local_payload" == "$recorded_payload" ]] \
      || { echo "PAYLOAD surface digest mismatch between witness and delivery consumer (theirs ${recorded_payload:0:8}, ours ${local_payload:0:8})" >&2; return 3; }
  fi
  echo "$recorded_digest"
  return 0
}

witness_engine_skip_authorized() {
  go run ./cmd/metasystem behavior-surface skip-allowed \
    --scope WITNESS --family witness-engine-gate >/dev/null 2>&1
}

# A witness consumer first freezes its own tree, then runs this same gate from
# the export. Every acceptance read, skip-path build, post-build recheck, and
# full fallback therefore sees frozen bytes. GOFLAGS is discarded and pinned
# to read-only module selection; GOMODCACHE remains inherited and shared because
# the frozen go.sum pins module content hashes while -mod=readonly forbids a
# dependency-set rewrite. Modfile and overlay flags would replace frozen inputs
# and are refused before the export is used.
if [[ -n "${METASYSTEM_GATE_WITNESS:-}" \
  && "${METASYSTEM_GATE_WITNESS_CONSUMER_EXPORT:-}" != "$root" ]]; then
  if frozen_refusal=$(frozen_toolchain_refusal); then
    echo "go gate: frozen witness consumer refused: $frozen_refusal" >&2
    exit 1
  fi
  # The consumer path ends by installing a fresh binary into the live
  # tree — the same mid-run swap the rebuild fence exists to prevent, so
  # the fence binds here exactly as it binds the plain rebuild.
  if [[ "${METASYSTEM_ALLOW_CONCURRENT_GATE:-0}" != 1 && -x "$root/bin/metasystem" ]]; then
    consumer_fence_rc=0
    "$root/bin/metasystem" gate fence --root "$root" --self-pid $$ || consumer_fence_rc=$?
    if [[ "$consumer_fence_rc" == 1 ]]; then
      echo "go gate: a live gate run owns this checkout; rebuilding now would swap its binary mid-run (METASYSTEM_ALLOW_CONCURRENT_GATE=1 overrides)" >&2
      exit 1
    fi
  fi
  consumer_freeze_output=
  if ! consumer_freeze_output=$(go run ./cmd/metasystem gate witness-freeze --root "$root"); then
    echo "go gate: witness consumer could not freeze its live tree; refusing to use it as a proof substrate" >&2
    exit 1
  fi
  IFS=$'\t' read -r consumer_manifest_digest consumer_export_root consumer_freeze_snapshot <<<"$consumer_freeze_output"
  if [[ ! "$consumer_manifest_digest" =~ ^[a-f0-9]{64}$ || ! -d "$consumer_export_root" || ! -d "$consumer_freeze_snapshot" ]]; then
    cleanup_consumer_freeze >/dev/null 2>&1 || true
    echo "go gate: witness consumer freeze returned an invalid digest or export path" >&2
    exit 1
  fi
  consumer_export_root=$(cd "$consumer_export_root" && pwd -P)
  consumer_rc=0
  if [[ "$gate_witness_check_only" == 1 ]]; then
    ( cd "$consumer_export_root" \
        && GOFLAGS=-mod=readonly METASYSTEM_GATE_FROZEN_TOOLCHAIN=1 \
           METASYSTEM_GATE_WITNESS_CONSUMER_EXPORT="$consumer_export_root" \
           bash scripts/agents/go-gate.sh --witness-check-only ) || consumer_rc=$?
  else
    ( cd "$consumer_export_root" \
        && GOFLAGS=-mod=readonly METASYSTEM_GATE_FROZEN_TOOLCHAIN=1 \
           METASYSTEM_GATE_WITNESS_CONSUMER_EXPORT="$consumer_export_root" \
           METASYSTEM_GATE_WITNESS_REUSE_OUT="$gate_witness_reuse_out" \
           bash scripts/agents/go-gate.sh ) || consumer_rc=$?
  fi
  if [[ "$consumer_rc" == 0 && "$gate_witness_check_only" != 1 ]]; then
    if [[ ! -x "$consumer_export_root/bin/metasystem" ]]; then
      echo "go gate: frozen witness consumer passed without producing its engine binary" >&2
      consumer_rc=1
    else
      mkdir -p "$consumer_live_root/bin" \
        && cp "$consumer_export_root/bin/metasystem" "$consumer_live_root/bin/.metasystem.witness.$$" \
        && mv -f "$consumer_live_root/bin/.metasystem.witness.$$" "$consumer_live_root/bin/metasystem" \
        || consumer_rc=1
    fi
  fi
  if ! cleanup_consumer_freeze; then
    echo "go gate: frozen witness consumer could not release its owned project snapshot" >&2
    consumer_rc=1
  fi
  exit "$consumer_rc"
fi

if [[ "${METASYSTEM_GATE_FROZEN_TOOLCHAIN:-0}" == 1 && "${GOFLAGS:-}" != -mod=readonly ]]; then
  echo "go gate: frozen proof tree requires GOFLAGS=-mod=readonly" >&2
  exit 1
fi

# The probe exit: parsed above with the other arguments — the strict
# loop landed after this handler once shadowed it (2026-08-24 red:
# the loop rejected the flag before this line could see it, but only
# when a witness was armed, so batteries stayed green until a fresh
# outer gate armed one).
if [[ "$gate_witness_check_only" == 1 ]]; then
  digest=$(witness_acceptable) || exit 3
  if [[ "${METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE:-}" == ENGINE ]]; then
    witness_engine_skip_authorized \
      || { echo "behavior-surface policy does not authorize the witness ENGINE gate" >&2; exit 3; }
  fi
  echo "witness acceptable: ${digest:0:8}"
  exit 0
fi

if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]]; then
  if [[ "${METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE:-}" == ENGINE ]] \
    && digest=$(witness_acceptable); then
    if witness_engine_skip_authorized; then
      # Compilation and the self-gating proof stay real: the binary is
      # rebuilt from the accepted content and stamped with its digest.
      METASYSTEM_BUILD_STAMP="witness-${digest:0:12}" bash scripts/agents/go-build.sh \
        || { echo "go gate: build failed under an accepted witness" >&2; exit 1; }
      post_digest=
      if post_digest=$(witness_acceptable) && [[ "$post_digest" == "$digest" ]]; then
        if [[ -n "$gate_witness_reuse_out" ]]; then
          printf 'reused\n' >"$gate_witness_reuse_out"
        fi
        echo "go gate: PASSED (outer witness ${digest:0:8}, this boundary)"
        exit 0
      fi
      echo "go gate: witness changed during the skip-path build; running the full gate" >&2
    else
      echo "go gate: witness acceptable, but the prospective behavior-surface policy does not authorize witness-engine-gate; running the full gate" >&2
    fi
  else
    echo "go gate: witness not accepted; running the full gate" >&2
  fi
fi

# Authenticate the worker before any gate stage can launch descendants. The
# producer decision and claim wait until the exact point this process starts
# the full measurement, so a fence or static refusal cannot consume the slot.
ratchet_baseline=scripts/agents/coverage-ratchet.json
if [[ "$(uname -s)" == Linux ]]; then
  ratchet_baseline=scripts/agents/coverage-ratchet-linux.json
fi
coverage_proof_handoff=0
coverage_proof_candidate=0
if [[ -n "${METASYSTEM_PROOF_CONTROL_ROOT:-}" || -n "${METASYSTEM_PROOF_ATTEMPT:-}" ]]; then
  [[ -n "${METASYSTEM_PROOF_CONTROL_ROOT:-}" ]] \
    || { echo "go gate: proof worker locator has no control root" >&2; exit 1; }
  [[ "${proof_auth_command[0]}" != "$proof_auth_bin" || -x "$proof_auth_bin" ]] \
    || { echo "go gate: proof worker has no authentication engine" >&2; exit 1; }
  proof_auth_error=$("${proof_auth_command[@]}" proof-run worker-authorized --root "$root" 2>&1) \
    || { echo "go gate: proof worker custody is not authenticated: $proof_auth_error" >&2; exit 1; }
  # A legacy launch has a live control-root/process binding but no admitted
  # attempt, so it runs every stage without retained coverage. An admitted
  # unseeded full gate defers the source-identity decision until measurement.
  if [[ -n "${METASYSTEM_PROOF_ATTEMPT:-}" \
    && "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" != 1 \
    && "${METASYSTEM_GATE_FORCE:-0}" != 1 ]]; then
    coverage_proof_candidate=1
  fi
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

printf 'go gate: effective Go: %s\n' "$(go version)"
printf 'go gate: GOTOOLCHAIN: %s\n' "$(go env GOTOOLCHAIN)"

# gofmt is a hard gate: unformatted code is a review-noise source and the
# engineering standard requires it. Its exit status is captured — a missing
# or crashing gofmt refuses the gate instead of passing silently
# (go-production-grade B8).
gofmt_rc=0
unformatted=$(gofmt -l internal cmd 2>&1) || gofmt_rc=$?
# Every static tool runs regardless of earlier reds and the verdicts land
# as ONE block (Ruling P / commit-gate-collect): a red gofmt no longer
# hides what vet and staticcheck would have said, so one gate run teaches
# everything it can. Exit stays nonzero on any red; no check weakens.
gate_static_reds=()
if [[ "$gofmt_rc" != 0 ]]; then
  gate_static_reds+=("gofmt itself failed (status $gofmt_rc): $unformatted")
elif [[ -n "$unformatted" ]]; then
  gate_static_reds+=("gofmt would change: $(printf '%s ' $unformatted)")
fi
gate_vet_out=$(go vet ./... 2>&1) || gate_static_reds+=("go vet failed:
$gate_vet_out")

# staticcheck, pinned to release 2026.2 (module v0.8.0; go-production-grade
# Phase 0d): the frozen version keeps every checkout judging by the same
# rules, and a tool run that cannot start fails the gate loudly rather than
# skipping silently. It rides the compile cache vet just filled, so its
# verdict lands seconds after vet's.
gate_sc_out=$(go run honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./... 2>&1) \
  || gate_static_reds+=("staticcheck 2026.2 (module v0.8.0) refused (or could not run):
$gate_sc_out")

gate_build_scratch=$(mktemp "${TMPDIR:-/tmp}/metasystem-gate-collect.XXXXXX")
if ! bash scripts/agents/go-build.sh --out "$gate_build_scratch" >/dev/null 2>&1; then
  gate_static_reds+=("build failed (go-build.sh)")
  rm -f "$gate_build_scratch"
  gate_build_scratch=
fi
if [[ -n "$gate_build_scratch" ]]; then
  METASYSTEM_RUN_OWNER=$("$gate_build_scratch" proc ref --pid $$) \
    || gate_static_reds+=("run owner export failed")
  export METASYSTEM_RUN_OWNER
fi
if [[ "$gate_fast" == 1 ]]; then
  gate_refusal_out=$(go test -count=1 ./internal/refusal 2>&1) \
    || gate_static_reds+=("refusal register failed:
$gate_refusal_out")
fi
gate_hook_start_scope=script-fixture
# wow.md marks both template and adopted installations. The audit source is a
# second positive signal so deleting the marker cannot downgrade a damaged
# installation to a fixture. This script is also copied alone into focused
# gate and witness fixtures; those partial trees own neither signal nor a
# SessionStart hook. A dangling signal still requires the audit.
if [[ -e "$root/wow.md" || -L "$root/wow.md" \
  || -e "$root/internal/audit/hookstartexits.go" || -L "$root/internal/audit/hookstartexits.go" ]]; then
  gate_hook_start_scope=installation
fi
if [[ -n "$gate_build_scratch" && "$gate_hook_start_scope" == installation ]]; then
  gate_hook_start_out=$("$gate_build_scratch" audit hook-start-exits --root "$root" 2>&1) \
    || gate_static_reds+=("SessionStart exit audit failed:
$gate_hook_start_out")
fi
if [[ "$gate_hook_start_scope" == script-fixture ]]; then
  echo "go gate: SessionStart exit audit not applicable to this script fixture"
fi

gate_stop_surface_applicable=0
if [[ "$gate_hook_start_scope" == installation ]] \
  && git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  gate_stop_surface_applicable=1
fi
if [[ -n "$gate_build_scratch" && "$gate_stop_surface_applicable" == 1 ]]; then
  if gate_stop_surface_out=$("$gate_build_scratch" audit stop-decision-surface --root "$root" 2>&1); then
    printf '%s\n' "$gate_stop_surface_out"
  else
    gate_static_reds+=("Stop decision surface audit failed:
$gate_stop_surface_out")
  fi
fi
if [[ "$gate_stop_surface_applicable" != 1 ]]; then
  echo "go gate: Stop decision surface audit not applicable to this tree"
fi

if (( ${#gate_static_reds[@]} )); then
  echo "go gate: ${#gate_static_reds[@]} static check(s) red — the complete block:" >&2
  for red in "${gate_static_reds[@]}"; do
    printf -- '--- %s\n' "$red" >&2
  done
  exit 1
fi

# Fast mode stops here: the static verdicts are in, and the build proves
# the engine still compiles while handing the edit loop a fresh binary.
if [[ "$gate_fast" == 1 ]]; then
  # Publish the exact engine already built by the collected static stage.
  # The temporary sibling plus comparison keeps publication atomic and
  # proves that no second compile or byte substitution occurred.
  gate_build_target=${gate_proof_out:-$root/bin/metasystem}
  mkdir -p "$(dirname "$gate_build_target")"
  gate_build_publish="$gate_build_target.gate.$$"
  cp "$gate_build_scratch" "$gate_build_publish" \
    || { rm -f "$gate_build_publish"; echo "go gate: publish static build failed" >&2; exit 1; }
  cmp -s "$gate_build_scratch" "$gate_build_publish" \
    || { rm -f "$gate_build_publish"; echo "go gate: published static build changed bytes" >&2; exit 1; }
  chmod 755 "$gate_build_publish"
  mv -f "$gate_build_publish" "$gate_build_target"
  rm -f "$gate_build_scratch"
  gate_build_scratch=
  if [[ "$gate_hook_start_scope" == installation ]]; then
    echo "go gate: fast mode passed (dependency ratchet, parallel ratchet, gofmt, vet, staticcheck, refusal register, SessionStart exit audit, Stop decision surface audit, build); the full gate remains the landing requirement"
  else
    echo "go gate: fast mode passed (dependency ratchet, parallel ratchet, gofmt, vet, staticcheck, refusal register, build); the full gate remains the landing requirement"
  fi
  exit 0
fi

# Full mode may require a later witness-specific build stamp, so the static
# proof artifact is not the final runtime binary. Keep it privately until the
# native stage, though: an enrolled authentication engine may lawfully predate
# the candidate's go-gate-tests verb. Authentication remains with that trusted
# engine; candidate-native behavior runs through this already-built artifact.

# The standing Linux signal (go-production-grade Phase 1, P3): a darwin-only
# regression is invisible until someone tries, so both Linux architectures
# cross-compile in every gate run. Seconds of cost, no runner needed (KI-10).
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./... \
  || { echo "go gate: linux/amd64 cross-build failed" >&2; exit 1; }
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./... \
  || { echo "go gate: linux/arm64 cross-build failed" >&2; exit 1; }

# govulncheck, pinned like staticcheck (Phase 0d) and last of the static
# stages: its cost belongs to the vulnerability-database fetch, which the
# network owns, so every deterministic check gets to fail first. Version
# pins must use parser and SSA tooling compatible with the repository compiler.
go run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./... \
  || { echo "go gate: govulncheck v1.2.0 refused (or could not run)" >&2; exit 1; }

# The snapshot-gated invocation (witness write) prepares here, after the
# static verdicts and just ahead of the tests this preparation serves: the
# digest describes bytes nothing above mutates, and the stamp exports
# before every build that consumes it.
witness_digest=
witness_payload_digest=
witness_policy_version=
witness_toolchain_identity=
witness_payload_manifest=
if [[ -n "${METASYSTEM_GATE_WITNESS_WRITE:-}" ]] && ! witness_fenced_off && ! witness_refusal >/dev/null; then
  witness_payload_manifest="${METASYSTEM_GATE_WITNESS_WRITE%.json}.payload-paths.nul"
  umask 077
  go run ./cmd/metasystem behavior-surface list --root "$root" --projection PAYLOAD --nul \
    >"$witness_payload_manifest"
  chmod 600 "$witness_payload_manifest"
  read -r witness_policy_version witness_digest < <(gate_surface_identity ENGINE "outer snapshot $root")
  read -r witness_payload_version witness_payload_digest < <(gate_surface_identity PAYLOAD "outer snapshot $root" "$witness_payload_manifest")
  [[ "$witness_payload_version" == "$witness_policy_version" ]] \
    || { echo "go gate: behavior-surface projections reported different policy versions" >&2; exit 1; }
  witness_toolchain_identity=$(gate_toolchain_identity)
  export METASYSTEM_BUILD_STAMP="witness-${witness_digest:0:12}"
  # A fresh snapshot has no bin/metasystem yet, and the binary-driven
  # fixtures (preflight and friends) SKIP without it — the gate installs
  # its binary only after the tests, so a snapshot would deterministically
  # measure the skipped-fixture coverage and refuse its own ratchet. Build
  # first; the fence exempts this gate's own chain.
  bash scripts/agents/go-build.sh >/dev/null \
    || { echo "go gate: snapshot pre-build failed" >&2; exit 1; }
fi

# The coverage floor is executable, not prose (records/kill-shell/kill-shell.md, the
# production-grade 0c ratchet pulled forward): the test output feeds the
# audit verb, whose baseline only ever rises. The cmd package is exempt
# with its reason recorded in the baseline file. The consult runs through
# the freshly built temporary binary below, because THIS invocation's
# rebuild is what always-rebuild means.
coverage_log=$(mktemp)
if (( coverage_proof_candidate )); then
  coverage_eligibility_rc=0
  "${proof_auth_command[@]}" proof-run coverage-eligible --root "$root" \
    --baseline "$ratchet_baseline" --producer-pid $$ >/dev/null 2>&1 \
    || coverage_eligibility_rc=$?
  case "$coverage_eligibility_rc" in
    0)
      "${proof_auth_command[@]}" proof-run coverage-begin --root "$root" \
        --baseline "$ratchet_baseline" --producer-pid $$ \
        || { rm -f "$coverage_log"; echo "go gate: could not claim the authenticated coverage producer slot" >&2; exit 1; }
      coverage_proof_handoff=1
      ;;
    3) ;;
    *) rm -f "$coverage_log"; echo "go gate: coverage producer eligibility could not be authenticated" >&2; exit 1 ;;
  esac
fi
# The Go adapter owns the complete internal and command package inventory,
# native process partitions, coverage merge, terminal checks, supervision,
# and bounded diagnostic reruns. The gate passes the allowance inherited
# from its admitted parent; a legacy standalone caller gets one worker.
gate_workers=${METASYSTEM_TEST_WORKERS:-1}
[[ "$gate_workers" =~ ^[1-9][0-9]*$ ]] \
  || { rm -f "$coverage_log"; echo "go gate: METASYSTEM_TEST_WORKERS must be a positive integer" >&2; exit 1; }
gate_native_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-gate-native.XXXXXX")
if ! METASYSTEM_TEST_WORKERS="$gate_workers" "$gate_build_scratch" proof-run go-gate-tests --root "$root" \
  --log-root "$gate_native_root" --workers "$gate_workers" >"$coverage_log" 2>&1; then
  keep="artifacts/agents/gate-failures/$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  mkdir -p "$(dirname "$keep")"
  # The witness owner removes its failed snapshot. Copying inside that tree
  # remains useful for local diagnosis, but the complete diagnostic must also
  # reach the retained parent log before any snapshot cleanup can erase it.
  cat "$coverage_log" >&2 || true
  mv "$coverage_log" "$keep" 2>/dev/null || true
  mv "$gate_native_root" "${keep%.log}.native" 2>/dev/null || true
  echo "go gate: native Go tests failed (output kept: $keep)" >&2
  exit 1
fi
cat "$coverage_log"
rm -rf "$gate_native_root"
rm -f "$gate_build_scratch"
gate_build_scratch=

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
if (( coverage_proof_handoff )); then
  bin/metasystem proof-run coverage-complete --root "$root" --baseline "$ratchet_baseline" \
    --input "$coverage_log" --packages "$pkg_list" --producer-pid $$ \
    || { rm -f "$coverage_log" "$pkg_list"; echo "go gate: authenticated coverage publication refused" >&2; exit 1; }
fi
rm -f "$coverage_log" "$pkg_list"

# The witness write (D33): only the controller's snapshot-gated invocation
# sets METASYSTEM_GATE_WITNESS_WRITE, and it runs this gate inside an
if [[ -n "${METASYSTEM_GATE_WITNESS_WRITE:-}" ]] && ! witness_fenced_off; then
  if refusal=$(witness_refusal); then
    echo "go gate: witness not written ($refusal)" >&2
  else
    witness_manifest_digest=${METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST:-}
    [[ "$witness_manifest_digest" =~ ^[a-f0-9]{64}$ ]] \
      || { echo "go gate: complete-project manifest identity is required for witness publication" >&2; exit 1; }
    go run ./cmd/metasystem gate witness-verify --root "$root" \
      --witness "$witness_manifest_digest" >/dev/null \
      || { echo "go gate: frozen export changed during the full proof; witness voided" >&2; exit 1; }
    if [[ -z "$witness_digest" ]]; then
      read -r witness_policy_version witness_digest < <(gate_surface_identity ENGINE "outer snapshot $root")
    fi
    if [[ -z "$witness_payload_manifest" ]]; then
      witness_payload_manifest="${METASYSTEM_GATE_WITNESS_WRITE%.json}.payload-paths.nul"
      umask 077
      go run ./cmd/metasystem behavior-surface list --root "$root" --projection PAYLOAD --nul \
        >"$witness_payload_manifest"
      chmod 600 "$witness_payload_manifest"
    fi
    if [[ -z "$witness_payload_digest" ]]; then
      read -r witness_payload_version witness_payload_digest < <(gate_surface_identity PAYLOAD "outer snapshot $root" "$witness_payload_manifest")
      [[ "$witness_payload_version" == "$witness_policy_version" ]] \
        || { echo "go gate: behavior-surface projections reported different policy versions" >&2; exit 1; }
    fi
    [[ -n "$witness_toolchain_identity" ]] || witness_toolchain_identity=$(gate_toolchain_identity)
    umask 077
    [[ "${METASYSTEM_GATE_WITNESS_CONTROLLER_PID:-}" =~ ^[1-9][0-9]*$ \
      && "${METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT:-}" =~ ^[1-9][0-9]*$ \
      && "${METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS:-}" =~ ^[0-9]+$ ]] \
      || { echo "go gate: witness controller identity is incomplete" >&2; exit 1; }
    if (( METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS > 0 )); then
      [[ -n "${METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID:-}" ]] \
        || { echo "go gate: witness controller pair identity is incomplete" >&2; exit 1; }
    else
      [[ -z "${METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID:-}" ]] \
        || { echo "go gate: witness controller pair identity is malformed" >&2; exit 1; }
    fi
    printf '{"policyVersion":%s,"engineDigest":"%s","manifestDigest":"%s","payloadDigest":"%s","payloadManifest":"%s","toolchainIdentity":"%s","runId":"%s","controller":{"pid":%s,"startedAtSec":%s,"startTicks":%s,"bootId":"%s"},"passedAt":"%s","goVersion":"%s","ratchetBaseline":"%s","summary":"full gate in frozen proof tree"}\n' \
      "$witness_policy_version" "$witness_digest" "$witness_manifest_digest" "$witness_payload_digest" "$(basename "$witness_payload_manifest")" "$witness_toolchain_identity" \
      "${METASYSTEM_GATE_WITNESS_RUN:-}" "${METASYSTEM_GATE_WITNESS_CONTROLLER_PID}" \
      "${METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT}" "${METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS}" \
      "${METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID:-}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$(go version | tr -d '"')" "$ratchet_baseline" >"$METASYSTEM_GATE_WITNESS_WRITE"
    chmod 600 "$METASYSTEM_GATE_WITNESS_WRITE"
    echo "go gate: witness written (${witness_digest:0:8})"
  fi
fi

echo "go gate: PASSED (gofmt, vet, race tests, coverage ratchet, build @ $commit)"
