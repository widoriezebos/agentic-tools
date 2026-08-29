#!/usr/bin/env bash
set -euo pipefail

# Run classification belongs to the validation root. Capture inherited proof
# before any producer can arm a witness, then revoke the output capability so
# descendant validations cannot publish over the root's artifact.
witness_present_at_validation_entry=0
[[ -z "${METASYSTEM_GATE_WITNESS:-}" ]] || witness_present_at_validation_entry=1
witness_engine_reused=0
battery_run_class_out=${METASYSTEM_BATTERY_RUN_CLASS_OUT:-}
battery_run_class_writer=${METASYSTEM_BATTERY_ROOT_CLASS_WRITER:-0}
unset METASYSTEM_BATTERY_RUN_CLASS_OUT METASYSTEM_BATTERY_ROOT_CLASS_WRITER \
  METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE METASYSTEM_GATE_WITNESS_WRITE \
  METASYSTEM_GATE_WITNESS_CONTROLLER_PID METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT \
  METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID
if [[ -n "$battery_run_class_out" && "$battery_run_class_writer" != 1 ]]; then
  echo "battery run-class output is reserved for the isolated validation root" >&2
  exit 1
fi

usage() {
  echo "Usage: scripts/validate-metasystem.sh [--delegate-scope|--delivery-contract|--enumerate --report <path>]" >&2
}

delegate_scope=0
delivery_contract=0
delivery_reuse=0
enumeration_section=
enumerate_mode=0
case ${1:-} in
  '') ;;
  --delegate-scope) [[ $# -eq 1 ]] || { usage; exit 2; }; delegate_scope=1 ;;
  # The delivery contract (D33): a nested adopted validation proves the
  # payload is complete, wired, and self-gating; the engine-behavior
  # families are skipped only because the outer controller verified the
  # staged content is digest-identical to the tree its own full gate
  # proved. Its verdict line is its own — it can never be read as the
  # canonical suite's.
  --delivery-contract)
    [[ $# -eq 1 ]] || { usage; exit 2; }
    delivery_contract=1
    # Exported HERE, before anything runs: the gate hook executes long
    # before the later scope blocks, and a witness is honored only when
    # the gate can see it arrived under the contract.
    export METASYSTEM_DELIVERY_CONTRACT=1
    ;;
  --enumerate) enumerate_mode=1 ;;
  --enumeration-section)
    [[ "${METASYSTEM_ENUMERATION_DRIVER:-0}" == 1 && $# -eq 2 ]] \
      || { usage; exit 2; }
    enumeration_section=$2
    ;;
  -h|--help) usage; exit 0 ;;
  *) usage; exit 2 ;;
esac

section_selected() { # stable section identifier
  if [[ -z "$enumeration_section" || "$enumeration_section" == "$1" ]]; then
    suite_progress_enter_section "$1"
    return 0
  fi
  return 1
}

delegate_owed_sections=(
  "supervision and census fixtures"
  "supervisor fingerprint heal harness"
  "dispatcher, adapter selftest, and mission-runner process fixtures"
)
delegate_skipped_sections=()
delivery_skipped=()
delivery_contract_skip() { # family or section name; returns 0 = skip it
  (( delivery_contract && delivery_reuse )) || return 1
  local policy_engine=${engine:-$root/bin/metasystem}
  "$policy_engine" behavior-surface skip-allowed --scope DELIVERY --family "$1" >/dev/null 2>&1 || return 1
  delivery_skipped+=("$1")
  return 0
}
delegate_process_section() { # human-readable section name
  if (( delegate_scope )); then
    delegate_skipped_sections+=("$1")
    return 1
  fi
  return 0
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

# The entry process launches the suite into a separate process group beside
# its watchdog. A stopped suite therefore cannot stop its own custodian.
suite_progress_path="$root/artifacts/agents/supervision/suite-progress.jsonl"
suite_progress_worker=0
if [[ "${METASYSTEM_SUITE_PROGRESS_ACTIVE:-0}" == 1 \
  && "${METASYSTEM_SUITE_PROGRESS_ROOT:-}" == "$root" ]]; then
  suite_progress_worker=1
fi
if (( ! suite_progress_worker )); then
  suite_progress_run="$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM"
  suite_progress_tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-validate.XXXXXX")
  suite_progress_log="$root/artifacts/agents/supervision/suite-logs/validate-$suite_progress_run.log"
  suite_banner=$(go run ./cmd/metasystem proof-run banner \
    --suite validate-metasystem --root "$root" \
    --progress "$suite_progress_path" --log "$suite_progress_log")
  suite_depth=$(( ${METASYSTEM_SUITE_PROGRESS_DEPTH:--1} + 1 ))
  selector_args=(--selector "$root/scripts/agents/validate-section-selector.sh")
  if [[ -z "$enumeration_section" || "$enumeration_section" == engine-delivery-contract ]]; then
    selector_args+=(--twice engine-delivery-contract)
  fi
  if [[ -z "$enumeration_section" || "$enumeration_section" == runtime-contract-audits ]]; then
    selector_args+=(--twice runtime-contract-audits)
  fi
  [[ -z "$enumeration_section" ]] || selector_args+=(--selected "$enumeration_section")
  exec go run ./cmd/metasystem proof-run launch \
    --suite validate-metasystem --root "$root" --conf "$root/metasystem.conf" \
    --progress "$suite_progress_path" --log "$suite_progress_log" \
    --tmp "$suite_progress_tmp" --banner "$suite_banner" \
    "${selector_args[@]}" -- \
    env METASYSTEM_SUITE_PROGRESS_ACTIVE=1 \
      METASYSTEM_SUITE_PROGRESS_ROOT="$root" \
      METASYSTEM_SUITE_PROGRESS_DEPTH="$suite_depth" \
      METASYSTEM_SUITE_PROGRESS_TMP="$suite_progress_tmp" \
      METASYSTEM_SUITE_PROGRESS_LOG="$suite_progress_log" \
      bash "$root/scripts/validate-metasystem.sh" "$@"
fi

suite_progress_current=
suite_progress_append() { # section, start|end
  local section=$1 event=$2 at
  at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  printf '{"suite":"validate-metasystem","section":"%s","event":"%s","at":"%s","depth":%d}\n' \
    "$section" "$event" "$at" "${METASYSTEM_SUITE_PROGRESS_DEPTH:-0}" \
    >>"$suite_progress_path" \
    || { echo "suite progress: cannot append $event for section $section" >&2; exit 1; }
}
suite_progress_enter_section() { # selector section
  local section=$1
  if [[ -n "$suite_progress_current" ]]; then
    suite_progress_append "$suite_progress_current" end
  fi
  suite_progress_current=$section
  suite_progress_append "$suite_progress_current" start
}
suite_progress_finish() {
  if [[ -n "$suite_progress_current" ]]; then
    suite_progress_append "$suite_progress_current" end
    suite_progress_current=
  fi
}

if (( enumerate_mode )); then
  exec bash "$root/scripts/agents/enumerate-suite.sh" "${@:2}"
fi

source scripts/agents/checkout-execution-guard.sh
checkout_execution_guard_acquire "validate-metasystem.sh"
trap 'suite_progress_finish; checkout_execution_guard_release || true; if [[ -n "${METASYSTEM_SUITE_PROGRESS_TMP:-}" ]]; then rm -rf -- "$METASYSTEM_SUITE_PROGRESS_TMP" 2>/dev/null || true; fi' EXIT
if [[ -n "${METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE:-}" ]]; then
  checkout_execution_guard_fixture_wait
  checkout_execution_guard_release
  trap - EXIT
  exit 0
fi
# Captured AFTER the cd above: the sentinel must describe the suite's own
# root, never the caller's working directory — a nested adopted-copy run
# inherited the template's cwd and believed itself the template.
metasystem_here=$(pwd -P)

publish_battery_run_class() { # FULL|WITNESS-ASSISTED
  [[ -n "$battery_run_class_out" ]] || return 0
  case "$1" in FULL|WITNESS-ASSISTED) ;; *) return 1 ;; esac
  [[ "$battery_run_class_out" == /* && ! -e "$battery_run_class_out" \
    && ! -L "$battery_run_class_out" ]] || return 1
  local class_parent class_name class_stage
  class_parent=${battery_run_class_out%/*}
  class_name=${battery_run_class_out##*/}
  [[ -n "$class_name" && -d "$class_parent" && ! -L "$class_parent" ]] || return 1
  class_parent=$(cd "$class_parent" && pwd -P) || return 1
  class_stage=$class_parent/.${class_name}.stage.$$
  [[ ! -e "$class_stage" && ! -L "$class_stage" ]] || return 1
  umask 077
  printf '%s\n' "$1" >"$class_stage" || return 1
  chmod 600 "$class_stage" || return 1
  mv "$class_stage" "$battery_run_class_out"
}

# Disk-hygiene headroom guard (backlog item 19, slice 1): make a full
# disk NAME ITSELF before the suite assumes space. The ENOSPC incident
# that motivated the goal remounted the guest read-only and looked like
# a code failure; a loud named line here turns that into a diagnosis.
# Checks the filesystems the suite actually fills — the repo, TMPDIR,
# and the Go build cache. Non-fatal for now (the pressure-sweep recovery
# is a later slice); it warns loudly with the per-filesystem deficit.
headroom_floor="${METASYSTEM_HEADROOM_FLOOR_GB:-5}"
# The floor must be a plain non-negative number; anything else falls
# back to the default LOUDLY rather than feeding awk/the engine garbage.
if ! [[ "$headroom_floor" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  echo "HEADROOM WARNING: METASYSTEM_HEADROOM_FLOOR_GB='$headroom_floor' is not a non-negative number; using the default floor of 5" >&2
  headroom_floor=5
fi
headroom_bootstrap() {
  # Plain df arithmetic — the guard's degraded form for a checkout
  # with no engine (first build) or a STALE engine that predates the
  # verb (the engine cannot gate its own rebuild). Best-effort and
  # advisory, but every unmeasurable path is NAMED, never silent.
  local p out
  for p in "$root" "${TMPDIR:-/tmp}" "$(go env GOCACHE 2>/dev/null || true)"; do
    [[ -n "$p" ]] || continue
    out=$(df -Pk "$p" 2>/dev/null) || out=""
    if [[ -z "$out" ]]; then
      echo "HEADROOM WARNING: could not measure $p (df failed); a full disk will masquerade as a build failure" >&2
      continue
    fi
    printf '%s\n' "$out" | awk -v floor="$headroom_floor" -v p="$p" \
      'NR==2 && $4/1048576 < floor {
         printf "HEADROOM WARNING: %s has %.1fGiB free, below the %sGiB floor; a full disk will masquerade as a build failure\n", p, $4/1048576, floor > "/dev/stderr"
       }'
  done
}
if [[ -x "$root/bin/metasystem" ]]; then
  headroom_rc=0
  headroom_report=$("$root/bin/metasystem" janitor headroom \
    --path "$root" --path "${TMPDIR:-/tmp}" --path "$(go env GOCACHE 2>/dev/null || echo "$root")" \
    --floor-gb "$headroom_floor" 2>&1) || headroom_rc=$?
  case $headroom_rc in
    0) ;;
    3)
      # Below floor stays ADVISORY during the migration (the pressure
      # sweep is a later slice) — loud, named, non-fatal.
      echo "HEADROOM WARNING: a filesystem the suite fills is below its floor; a full disk will masquerade as a test failure:" >&2
      printf '%s\n' "$headroom_report" | grep '"belowFloor":true' >&2 || true ;;
    2)
      # A usage error from bin/metasystem here almost always means a
      # STALE binary that predates the verb or a flag change. Refusing
      # would permanently block the very rebuild that fixes it — so
      # degrade to the bootstrap check, loudly.
      echo "HEADROOM WARNING: bin/metasystem did not understand the headroom invocation (stale binary?); using the df bootstrap check instead" >&2
      headroom_bootstrap ;;
    *)
      # Exit 1 means the guard RAN and could not measure — an
      # unmeasurable filesystem is a refusal, not a pass.
      echo "metasystem validation failed: the headroom guard could not measure (exit $headroom_rc):" >&2
      printf '%s\n' "$headroom_report" >&2
      exit 1 ;;
  esac
else
  # First build: no engine yet, but the expensive build path is about
  # to run — the guard degrades to plain df arithmetic rather than
  # silently skipping (a clean checkout was the one shape that ran
  # with no check at all).
  headroom_bootstrap
fi

# Two concurrent suite runs trample each other's shared fixtures, so the
# suite refuses to start over a live gate run. The decision is the engine's
# (gate fence prunes dead markers and exempts this process's own chain);
# this is only the consult. On a first build there is no binary to ask.
# Only a REAL block (exit 1) refuses; a binary too old to know the verb
# (exit 2) must not brick the run that would rebuild it. The gate fence
# fixture below keeps that leniency honest: a usage bug fails there loudly.
if [[ "${METASYSTEM_ALLOW_CONCURRENT_GATE:-0}" != 1 && -x bin/metasystem ]]; then
  gate_fence_rc=0
  bin/metasystem gate fence --root "$root" --self-pid $$ || gate_fence_rc=$?
  if [[ "$gate_fence_rc" == 1 ]]; then
    echo "a gate run is already live in this checkout; refusing a concurrent suite (METASYSTEM_ALLOW_CONCURRENT_GATE=1 overrides)" >&2
    exit 1
  fi
fi

# A gate run is work in flight that no job record describes, so it says so
# itself rather than being guessed at from process command lines. The marker
# names THIS parent process by pid and start time and spans the WHOLE suite
# including the snapshot gate (goal-system GOAL-17: a snapshot-side
# registration marks the wrong root, so the serving root's liveness is the
# parent's job). Registration failure is FATAL — a gate that cannot record
# its liveness must not run invisibly. The one named residual: on a truly
# clean bootstrap no binary exists anywhere yet, bounded by the first build.
gate_run_marker=
if [[ -x bin/metasystem ]]; then
  gate_run_marker=$(bin/metasystem gate register --root "$root" \
    --gate validate-metasystem.sh --pid $$) || {
    echo "gate registration failed; refusing to run invisibly" >&2
    exit 1
  }
fi

# The Go engine gate runs first (records/misc/go-migration.md): gofmt, vet, the
# race unit suite, and the build. A broken binary fails here, before any
# fixture tries to drive it. Skipped only in delegate scope (no toolchain
# guarantee in a delegate sandbox, and it needs no process visibility).
# The Go engine section runs only where the engine is present (a go.mod
# in this checkout). An adopted target that has not yet received the Go
# engine (a Phase 4 port) runs pure shell/python, so the whole section is
# a no-op there — the go gate, the seam tripwire, and the owner-alone
# fixtures alike. It also needs process visibility, so it is out of
# delegate scope.
# The delivery mode is declared, never inferred (D33 closing the D17
# fail-open): wherever a metasystem.conf exists the key must too, and
# source delivery without the module is a damaged payload — a deleted
# go.mod must not read as "no engine expected".
if section_selected engine-delivery-contract && [[ -f metasystem.conf ]]; then
  engine_delivery=$(sed -n 's/^metasystem\.engine-delivery=//p' metasystem.conf | head -1)
  [[ -n "$engine_delivery" ]] \
    || { echo "metasystem.engine-delivery is required in metasystem.conf; a missing key reads as damage, not as a mode" >&2; exit 1; }
  if [[ "$engine_delivery" == source && ! -f go.mod ]]; then
    echo "metasystem.engine-delivery=source but go.mod is absent — the engine source did not ship" >&2
    exit 1
  fi
fi

metasystem_go_source=0
grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' go.mod && metasystem_go_source=1
if section_selected engine-delivery-contract && (( ! metasystem_go_source )) \
  && [[ -f internal/missionrunner/stoploss.go || -f internal/mission/ledger.go ]]; then
  echo "metasystem Go source present but go.mod does not declare the metasystem module — damaged template" >&2
  exit 1
fi

# The covenant evidence gate, EARLY (battery-wall-clock lever 3): the
# gate is a pure function of files and costs milliseconds, so a broken
# covenant/table pair refuses here in seconds instead of after the full
# engine gate. Exit 1 is a real refusal and fails the run; exit 2/127
# means the present binary predates the verb — deferred, because the
# LATE gate after the rebuild still enforces on the proven engine, and
# that late run remains the guarantee this early run only accelerates.
if section_selected covenant-evidence-pre-rebuild \
  && [[ -e covenant.json || -L covenant.json ]] && [[ -x bin/metasystem ]]; then
  early_evidence_rc=0
  bin/metasystem covenant evidence --root "$root" || early_evidence_rc=$?
  case "$early_evidence_rc" in
    0) echo "covenant evidence gate passed (pre-rebuild)" ;;
    2|127) echo "covenant evidence gate deferred: the present engine predates the verb; the post-rebuild gate judges" ;;
    *)
      echo "the covenant evidence gate refused (exit $early_evidence_rc); the table and the covenant disagree" >&2
      exit 1
      ;;
  esac
fi
if section_selected go-engine-gate && (( ! delegate_scope )) && (( metasystem_go_source )); then
  # The witness-producing gate (D33): when the gate-input roots are clean
  # against HEAD, the full gate runs inside an extracted HEAD snapshot —
  # the exact bytes adoption stages — and its witness is handed to the
  # nested delivery-contract runs this suite spawns. Dirty roots, seed,
  # force, or any refusal fall back to the plain worktree gate, no
  # witness, exactly as before. The machinery lives in the sourced
  # helper so standalone battery stages (adopt-fixtures) arm the same
  # witness instead of re-proving identical bytes per nested run. The
  # fallback is FORCED here: canonical validation always runs a real
  # gate when the witness cannot arm, immune to ambient state. The
  # narrow trap guards the window between arming and the full cleanup
  # trap far below — any early exit still takes the witness with it
  # (the later validation_cleanup trap replaces this one and repeats
  # the removal).
  trap '[[ -z "${witness_state:-}" ]] || rm -rf "$witness_state"; checkout_execution_guard_release || true' EXIT
  WITNESS_GATE_FALLBACK=plain source scripts/agents/witness-gate.sh
  if (( delivery_contract )); then
    # The delivery smoke (D33): the freshly stamped binary answers a
    # decision verb, and when the outer run's witness matches this tree
    # the binary's ldflags stamp must carry that digest — binary
    # identity is part of the equivalence, not an assumption.
    [[ "$(bin/metasystem json get --value '{"ok":1}' --field ok)" == 1 ]] \
      || { echo "delivery contract: the rebuilt binary did not answer" >&2; exit 1; }
    if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]] \
      && METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=DELIVERY \
        bash scripts/agents/go-gate.sh --witness-check-only >/dev/null 2>&1; then
      delivery_reuse=1
      delivery_stamp=$(go version -m bin/metasystem | sed -n 's/.*BuildStamp=\(witness-[a-f0-9]*\).*/\1/p' | head -1)
      delivery_recorded=$(sed -n 's/.*"engineDigest":"\([a-f0-9]*\)".*/\1/p' "$METASYSTEM_GATE_WITNESS")
      [[ -n "$delivery_stamp" && "$delivery_stamp" == "witness-${delivery_recorded:0:12}" ]] \
        || { echo "delivery contract: binary stamp ${delivery_stamp:-absent} does not match the witness digest" >&2; exit 1; }
    else
      echo "delivery contract: payload or toolchain equality was not proven; every validation family will run" >&2
    fi
  fi
fi

validation_run_class=FULL
if (( witness_present_at_validation_entry && witness_engine_reused )); then
  validation_run_class=WITNESS-ASSISTED
fi
publish_battery_run_class "$validation_run_class" \
  || { echo "validation root could not publish its run class" >&2; exit 1; }

# The engine-seam tripwire and the Go-vs-python census conformance
# harnesses (signature, fingerprint, run) retired with the migration:
# the python reference no longer exists to diff against, and the Go
# packages carry their own unit coverage under the go gate above.
# Owner-alone Go supervision fixtures drive the running binary.
if section_selected supervision-go-fixtures && (( ! delegate_scope )) \
  && (( metasystem_go_source )); then
  delivery_contract_skip supervision-go-fixtures \
    || bash scripts/agents/supervision-go-fixtures.sh
fi

# The gate fence, live: this suite's own marker never blocks (this shell
# is the registered run's chain), a foreign live run blocks both the
# fence and a standalone go-gate rebuild, and a dead run stops blocking.
if section_selected gate-fence-fixtures && (( ! delegate_scope )) \
  && (( metasystem_go_source )); then
  if ! delivery_contract_skip gate-fence-fixtures; then
  bin/metasystem gate fence --root "$root" --self-pid $$ \
    || { echo "the suite's own gate marker blocked its fence" >&2; exit 1; }
  sleep 60 & gate_fence_foreign=$!
  bin/metasystem gate register --root "$root" --gate fence-fixture --pid "$gate_fence_foreign" >/dev/null
  if bin/metasystem gate fence --root "$root" --self-pid $$ 2>/dev/null; then
    echo "a foreign live gate run did not block the fence" >&2; exit 1
  fi
  gate_fence_err=$(mktemp)
  if bash scripts/agents/go-gate.sh 2>"$gate_fence_err"; then
    echo "go-gate rebuilt over a foreign live gate run" >&2; exit 1
  fi
  grep -q "swap its binary mid-run" "$gate_fence_err" \
    || { echo "go-gate refusal did not come from the rebuild fence" >&2; exit 1; }
  rm -f "$gate_fence_err"
  kill "$gate_fence_foreign" 2>/dev/null || true; wait "$gate_fence_foreign" 2>/dev/null || true
  bin/metasystem gate fence --root "$root" --self-pid $$ \
    || { echo "a dead foreign gate run kept blocking the fence" >&2; exit 1; }
  echo "gate fence fixtures passed"
  fi
  delivery_contract_skip checkout-execution-guard-fixtures \
    || bash scripts/agents/checkout-execution-guard-fixtures.sh
fi

# The covenant evidence gate (counselor slice one): where an app has
# declared its covenant, every requirement must be backed by the
# evidence table and every declared dependency must be present. This
# guarantees validation/CI refusal, not mission admission — the wall
# does not bind the evidence table. Runs after the engine rebuild so
# the judging binary is the tree's own; absent covenant, pre-inception
# repositories skip silently.
# The presence test must see ANY directory entry without following it
# (-e follows symlinks and calls a dangling one absent): a directory,
# FIFO, or symlink at the covenant's home is the engine's to refuse by
# name, never validation's to skip silently.
if section_selected covenant-evidence-post-rebuild \
  && [[ -e covenant.json || -L covenant.json ]]; then
  if [[ -x bin/metasystem ]]; then
    bin/metasystem covenant evidence --root "$root" \
      || { echo "the covenant evidence gate refused; the table and the covenant disagree" >&2; exit 1; }
    echo "covenant evidence gate passed"
  else
    echo "covenant.json present but bin/metasystem is not executable — the evidence gate cannot run" >&2
    exit 1
  fi
fi


# python3 remains a suite-host dependency for exactly one fixture: the
# dispatch fixtures' TTY escalation driver needs a real pty, which shell
# cannot open. Say so up front, loudly, instead of dying mid-suite with a
# cryptic 127. The PRODUCT does not need python at all.
if section_selected suite-host-prerequisites; then
  command -v python3 >/dev/null 2>&1 \
    || { echo "validate-metasystem: python3 is required by the TTY escalation fixture (the metasystem itself does not need it)" >&2; exit 1; }
fi
source scripts/agents/fixture-budget.sh

# The engine does the structural JSON work below; helpers use the absolute
# path so they survive the fixture subshells that change directories.
engine="$root/bin/metasystem"

# Canonical (engine-rendered) compact form of a JSON literal, so equality
# checks compare one encoder's bytes against the same encoder's bytes.
canonical_json() { # JSON text
  "$engine" json get --value "{\"root\":$1}" --field root
}

# Print one top-level element per line from the engine's compact rendering
# of a JSON array (or one "key":value member per line from an object). The
# walk is depth- and string-aware, so elements may nest objects and arrays
# — the flat-object splitter in supervision-fixtures.sh cannot. Compact
# rendering escapes control characters, so no element carries a newline.
json_elements() { # compact JSON array or object
  printf '%s' "$1" | awk '
    {
      n = length($0)
      first = substr($0, 1, 1)
      if (n < 2 || (first != "[" && first != "{")) exit 1
      depth = 0; instring = 0; escaped = 0; start = 2
      for (i = 2; i < n; i++) {
        ch = substr($0, i, 1)
        if (instring) {
          if (escaped) escaped = 0
          else if (ch == "\\") escaped = 1
          else if (ch == "\"") instring = 0
        } else if (ch == "\"") instring = 1
        else if (ch == "{" || ch == "[") depth++
        else if (ch == "}" || ch == "]") depth--
        else if (ch == "," && depth == 0) {
          print substr($0, start, i - start)
          start = i + 1
        }
      }
      if (n > 2) print substr($0, start, n - start)
    }'
}

# The key names of a JSON object, one per line, in the engine's canonical
# (byte-sorted) order.
json_object_keys() { # compact JSON object
  local member
  while IFS= read -r member; do
    member=${member#\"}
    printf '%s\n' "${member%%\":*}"
  done < <(json_elements "$1")
}

# Atomically replace one top-level field of a JSON object file, leaving
# every other field exactly as the file's parser sees it. `json set`
# covers string and integer fields; this covers null and the object and
# array fields it cannot spell. The whole file and the field are rendered
# by the same engine encoder, so the needle below is byte-exact; a failed
# splice or an unparseable result refuses instead of writing. (Copied from
# supervision-fixtures.sh, extended two ways: string-valued fields print
# bare, so those retry with the quoted spelling; and a read-back proves
# the edit landed on the top-level field, not a nested lookalike.)
json_replace_field() { # file, top-level field, replacement JSON value
  local file=$1 field=$2 new=$3 compact old out canonical staged
  compact=$("$engine" json get --value "{\"root\":$(cat "$file")}" --field root) \
    || { echo "json_replace_field: $file did not parse" >&2; return 1; }
  old=$("$engine" json get --file "$file" --field "$field") \
    || { echo "json_replace_field: $file has no $field" >&2; return 1; }
  out=${compact/"\"$field\":$old"/"\"$field\":$new"}
  if [[ "$out" == "$compact" ]]; then
    out=${compact/"\"$field\":\"$old\""/"\"$field\":$new"}
  fi
  "$engine" util json-validate --value "$out" \
    || { echo "json_replace_field: editing $field left $file unparseable" >&2; return 1; }
  canonical=$("$engine" json get --value "{\"root\":$new}" --field root) \
    || { echo "json_replace_field: replacement for $field is not JSON: $new" >&2; return 1; }
  [[ "$("$engine" json get --value "$out" --field "$field")" == "$canonical" ]] \
    || { echo "json_replace_field: could not locate $field in $file" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.replace.XXXXXX") || return 1
  printf '%s\n' "$out" >"$staged"
  mv "$staged" "$file"
}

# Atomically add one top-level field a JSON object file does not have yet.
# `json set` spells only string and integer values; this inserts the null,
# object, and array values it cannot. The file is re-rendered by the engine
# encoder first so the splice point is exact, and a read-back proves the
# new field landed before the stage replaces the file.
json_insert_field() { # file, new top-level field, JSON value
  local file=$1 field=$2 new=$3 compact out canonical staged
  compact=$("$engine" json get --value "{\"root\":$(cat "$file")}" --field root) \
    || { echo "json_insert_field: $file did not parse" >&2; return 1; }
  if "$engine" json get --file "$file" --field "$field" >/dev/null 2>&1; then
    echo "json_insert_field: $file already has $field" >&2; return 1
  fi
  if [[ "$compact" == "{}" ]]; then
    out="{\"$field\":$new}"
  else
    out="{\"$field\":$new,${compact#\{}"
  fi
  "$engine" util json-validate --value "$out" \
    || { echo "json_insert_field: inserting $field left $file unparseable" >&2; return 1; }
  canonical=$("$engine" json get --value "{\"root\":$new}" --field root) \
    || { echo "json_insert_field: value for $field is not JSON: $new" >&2; return 1; }
  [[ "$("$engine" json get --value "$out" --field "$field")" == "$canonical" ]] \
    || { echo "json_insert_field: $field did not land in $file" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.insert.XXXXXX") || return 1
  printf '%s\n' "$out" >"$staged"
  mv "$staged" "$file"
}

# Atomically remove one top-level field that must exist: the callers model
# a document that HAD the field, so a silent no-op would leave the fixture
# proving nothing.
json_remove_field() { # file, top-level field the file must have
  local file=$1 field=$2 staged
  "$engine" json get --file "$file" --field "$field" >/dev/null \
    || { echo "json_remove_field: $file has no $field" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.remove.XXXXXX") || return 1
  "$engine" json strip --file "$file" --key "$field" >"$staged" \
    || { rm -f "$staged"; return 1; }
  mv "$staged" "$file"
}
if (( delegate_scope )); then
  # Load calibration is itself a real census. Delegate validation uses the
  # policy's minimum scale so no process enumeration occurs before the
  # process-sensitive sections are skipped.
  : "${METASYSTEM_FIXTURE_CAP_SCALE:=3}"
  export METASYSTEM_FIXTURE_CAP_SCALE
fi
harness_fixture_budget_init "$root"
fixture_minimum_cap_min=$(harness_fixture_semantic_cap minimum-minutes)
fixture_mission_job_cap_min=$(harness_fixture_semantic_cap mission-job-minutes)
fixture_dispatch_envelope_cap_min=$(harness_fixture_semantic_cap dispatch-envelope-minutes)
fixture_dispatch_over_envelope_cap_min=$(harness_fixture_semantic_cap dispatch-over-envelope-minutes)
fixture_watcher_config_cap_min=$(harness_fixture_semantic_cap watcher-config-minutes)
fixture_watcher_nonfiring_cap_min=$(harness_fixture_semantic_cap watcher-nonfiring-minutes)
fixture_watcher_firing_cap_min=$(harness_fixture_semantic_cap watcher-firing-minutes)

if section_selected metasystem-audit; then
  scripts/audit-metasystem.sh .
fi

# The gate's own integrity (go-production-grade B8): a gofmt that cannot run
# must refuse the gate, not pass it silently. The shim exits before any
# expensive stage, so this replay is cheap; the nested gate needs the
# concurrency waiver because this suite's own run holds the fence.
# Under the delivery contract the gate is a digest check plus rebuild —
# it never runs gofmt, so this tripwire's subject does not exist there;
# the outer run's full gate keeps it (D33).
if section_selected gate-fail-open-tripwire \
  && [[ -f go.mod ]] && command -v go >/dev/null 2>&1 \
  && ! delivery_contract_skip gate-fail-open-tripwire; then
  gofmt_shim_dir=$(mktemp -d)
  printf '#!/usr/bin/env bash\necho "shim: gofmt is broken" >&2\nexit 7\n' >"$gofmt_shim_dir/gofmt"
  chmod +x "$gofmt_shim_dir/gofmt"
  if METASYSTEM_ALLOW_CONCURRENT_GATE=1 PATH="$gofmt_shim_dir:$PATH" \
      bash scripts/agents/go-gate.sh >"$gofmt_shim_dir/out" 2>&1; then
    echo "go gate passed with a broken gofmt; the fail-open hole is back" >&2
    exit 1
  fi
  grep -q "gofmt itself failed" "$gofmt_shim_dir/out" \
    || { echo "go gate refused a broken gofmt without naming it" >&2; cat "$gofmt_shim_dir/out" >&2; exit 1; }
  rm -rf "$gofmt_shim_dir"
fi

# The template distinction is shared by later fixture sections. Computing it
# has no side effects; the checks that consume it remain separately enumerable.
template_mode=0
[[ "${metasystem_here##*/}" == metasystem && -f "${metasystem_here%/*}/development/metasystem-design.md" ]] && template_mode=1
if (( delivery_contract )); then
  # A delivery run is never the orchestrating template — wherever it
  # runs, adoption fixtures and the other template-only blocks belong to
  # the FULL suite that spawned it. (The contract env exports happen at
  # flag parse, ahead of the gate hook.)
  template_mode=0
fi

if section_selected static-contract-audits; then
# Validate every skill present, including project-added and moved optional
# skills, so this script holds in adopted repositories as well as the template.
# A skill directory without a SKILL.md is invisible to the find, so check for
# hollow directories explicitly first.
for dir in skills optional-skills; do
  [[ -d "$dir" ]] || continue
  for d in "$dir"/*/; do
    [[ -d "$d" ]] || continue
    [[ -f "${d}SKILL.md" ]] || { echo "skill directory without SKILL.md: ${d%/}" >&2; exit 1; }
  done
  while IFS= read -r skill_md; do
    scripts/validate-skill.sh "$(dirname "$skill_md")"
  done < <(find "$dir" -name SKILL.md | sort)
done

# Core assets are required everywhere. The full seven-skill set with every
# per-runtime profile is required only in the template repository, detected by
# its own directory name plus the development docs beside it: the metasystem
# never ships anything referencing outward, so the sibling test is guarded by
# the name test and adopted repositories never touch a parent path.
# Adopted repositories may prune unused skills, and
# each skill present is still validated by the loop above. In adopted mode,
# any profile a remaining skill does provide must be registered without drift;
# project-added skills are not required to invent profiles they never shipped.
if (( ! template_mode )); then
  scripts/metasystem-config.sh validate
fi

for link in \
  docs/project-rules.md \
  docs/orchestration.md \
  docs/collaboration.md \
  docs/design/design-principles.md \
  docs/design/design-obligation-gate.md \
  docs/examples/design-obligation-matrix.md \
  docs/examples/step-back-ledger.md \
  .gitattributes \
  memory/instruction-ledger.md \
  scripts/refactor-baseline.sh \
  scripts/frontier.sh \
  scripts/receipt.sh \
  scripts/adopt.sh \
  scripts/enforcement/github-actions-metasystem.yml \
  scripts/enforcement/claude-code-hooks.json \
  scripts/enforcement/codex-hooks.json \
  scripts/enforcement/devin-hooks.json \
  scripts/assert-stop-loss.sh \
  scripts/assert-mission.sh \
  docs/examples/mission-contract.md \
  docs/examples/mission-cron.example \
  docs/project-adaptation.md \
  docs/metasystem-reconciliation.md \
  docs/working-modes.md \
  docs/working-with-agents.md \
  plans/README.md; do
  [[ -e "$link" ]] || { echo "missing routed asset: $link" >&2; exit 1; }
done

# The agent protocol is runtime-neutral and ships in template and adopted
# repositories. Keep the six dispatchable roles in lockstep across their
# preamble, return schema, and capability declaration.
for link in \
  scripts/agents/templates/brief.md \
  scripts/agents/templates/follow-up.md \
  scripts/agents/templates/host-turn-instruction.md \
  scripts/agents/roles/orchestrator.md \
  scripts/agents/schemas/orchestrator.schema.json \
  scripts/agents/permissions/none.json \
  scripts/agents/permissions/workspace.json \
  metasystem.conf \
  scripts/metasystem-config.sh \
  scripts/agents/dispatch.sh \
  scripts/agents/checkout-execution-guard.sh \
  scripts/agents/commit.sh \
  scripts/agents/land.sh \
  scripts/agents/second-session.sh \
  scripts/agents/arm-supervision.sh \
  scripts/agents/fixture-budget.sh \
  scripts/agents/enumerate-suite.sh \
  scripts/agents/validate-section-selector.sh \
  scripts/agents/enumerate-suite-fixtures.sh \
  scripts/agents/fingerprint-harness.sh \
  scripts/agents/supervision-hook.sh \
  scripts/agents/supervision-hook-fixtures.sh \
  scripts/agents/supervision-fixtures.sh \
  scripts/agents/telemetry-census-fixtures.sh \
  scripts/agents/return-schema-fixtures.sh \
  scripts/agents/config-identity-fixtures.sh \
  scripts/agents/authority-regression-fixtures.sh \
  scripts/agents/pre-commit-guard-fixtures.sh \
  scripts/agents/static-reproof-fixtures.sh \
  scripts/agents/milestone-battery.sh \
  scripts/agents/battery.conf.local.template \
  scripts/agents/gate-run-freeze-fixtures.sh \
  scripts/agents/witness-gate-fixtures.sh \
  scripts/agents/suite-progress-fixtures.sh \
  scripts/agents/land-fixtures.sh \
  scripts/agents/checkout-execution-guard-fixtures.sh \
  scripts/agents/record-protocol-fixtures.sh \
  scripts/agents/evidence-segment-fixtures.sh \
  scripts/agents/second-session-fixtures.sh \
  scripts/agents/lease-succession-fixtures.sh \
  scripts/agents/flight-recorder-fixtures.sh \
  scripts/agents/acp-fixtures.sh \
  scripts/agents/mission-fixtures.sh \
  scripts/agents/delegate-caps-fixtures.sh \
  scripts/agents/adapter-deadline-fixtures.sh \
  scripts/adopt-fixtures.sh \
  scripts/agents/dispatch-fixtures.sh \
  scripts/agents/mission-runner.sh \
  scripts/agents/hosts/host-common.sh \
  scripts/agents/hosts/claude.sh \
  scripts/agents/hosts/codex.sh \
  scripts/agents/hosts/devin.sh \
  scripts/agents/hosts/fake.sh \
  scripts/agents/schemas/mission-state.schema.json \
  scripts/agents/schemas/wall-evidence.schema.json \
  scripts/agents/adapters/fake.sh \
  scripts/agents/adapters/runtime-common.sh \
  scripts/agents/adapters/codex-config-filter.v1.json \
  scripts/agents/adapters/claude-config-filter.v1.json \
  scripts/agents/adapters/devin-config-filter.v1.json \
  scripts/agents/assert-conformance.sh \
  scripts/agents/conformance-fixtures.sh \
  scripts/agents/instruction-bearing-paths.txt \
  scripts/assert-critique-closed.sh \
  scripts/assert-return-complete.sh \
  scripts/assert-turn-prompt.sh \
  scripts/agents/check-preamble-quotes.sh; do
  [[ -e "$link" ]] || { echo "missing agent protocol asset: $link" >&2; exit 1; }
done
fi

# Section 3.11 and retained watch-list round S4 have one bounded fixture suite.
# Process-owning groups run serially and use separate temporary repositories,
# so their supervisors and dispatch jobs cannot share lifecycle state. They
# name S4-1 through S4-10 at their owning checks and contain no uncapped
# process wait (IL-1).
if section_selected supervision-and-census-fixtures \
  && delegate_process_section "supervision and census fixtures" \
  && ! delivery_contract_skip "supervision and census fixtures"; then
  scripts/agents/supervision-hook-fixtures.sh
  scripts/agents/supervision-fixtures.sh
fi
if section_selected supervisor-fingerprint-heal-harness \
  && delegate_process_section "supervisor fingerprint heal harness" \
  && ! delivery_contract_skip "supervisor fingerprint heal harness"; then
  scripts/agents/fingerprint-harness.sh --iterations 2
fi
if section_selected mission-fixtures && ! delivery_contract_skip mission-fixtures; then
  scripts/agents/mission-fixtures.sh
fi

# Real runtime selftests spend model calls and remain manual acceptance steps.
# Validation covers only their static adapter contract.
# The external-dependency ratchet (os-dependency-reduction): an
# undeclared interpreter in metasystem scripts refuses here.
if section_selected shell-and-dependency-audits; then
bash scripts/agents/dependency-ratchet.sh --self-test >/dev/null
bash scripts/agents/dependency-ratchet.sh >/dev/null
bash -n scripts/agents/dependency-ratchet.sh
bash -n scripts/agents/arm-supervision.sh
bash -n scripts/agents/fixture-budget.sh
bash -n scripts/agents/enumerate-suite.sh
bash -n scripts/agents/validate-section-selector.sh
bash -n scripts/agents/enumerate-suite-fixtures.sh
bash -n scripts/agents/witness-gate.sh
bash -n scripts/agents/fingerprint-harness.sh
bash -n scripts/agents/supervision-hook.sh
bash -n scripts/agents/supervision-hook-fixtures.sh
bash -n scripts/agents/supervision-fixtures.sh
bash -n scripts/agents/telemetry-census-fixtures.sh
bash -n scripts/agents/return-schema-fixtures.sh
bash -n scripts/agents/config-identity-fixtures.sh
bash -n scripts/agents/record-protocol-fixtures.sh
bash -n scripts/agents/evidence-segment-fixtures.sh
bash -n scripts/agents/second-session-fixtures.sh
bash -n scripts/agents/lease-succession-fixtures.sh
bash -n scripts/agents/flight-recorder-fixtures.sh
bash -n scripts/agents/acp-fixtures.sh
bash -n scripts/agents/emit-event.sh
bash -n scripts/agents/pre-commit-guard-fixtures.sh
bash -n scripts/agents/static-reproof-fixtures.sh
bash -n scripts/agents/milestone-battery.sh
bash -n scripts/agents/gate-run-freeze-fixtures.sh
bash -n scripts/agents/witness-gate-fixtures.sh
bash -n scripts/agents/suite-progress-fixtures.sh
bash -n scripts/agents/land.sh
bash -n scripts/agents/land-fixtures.sh
bash -n scripts/agents/checkout-execution-guard.sh
bash -n scripts/agents/checkout-execution-guard-fixtures.sh
bash -n scripts/agents/mission-fixtures.sh
bash -n scripts/agents/delegate-caps-fixtures.sh
bash -n scripts/agents/adapter-deadline-fixtures.sh
bash -n scripts/adopt-fixtures.sh
bash -n scripts/agents/dispatch-fixtures.sh
bash -n scripts/agents/mission-runner.sh
bash -n scripts/agents/conformance-fixtures.sh
bash -n scripts/agents/goal-cli-fixtures.sh
bash -n scripts/agents/hosts/claude.sh
bash -n scripts/agents/hosts/codex.sh
bash -n scripts/agents/hosts/devin.sh
bash -n scripts/agents/hosts/fake.sh
bash -n scripts/assert-mission.sh
bash -n scripts/assert-return-complete.sh
bash -n scripts/assert-turn-prompt.sh
bash -n scripts/watch-background-jobs.sh
bash -n scripts/agents/dispatch.sh
bash -n scripts/agents/adapters/runtime-common.sh
fi
if section_selected conformance-fixtures; then
  delivery_contract_skip conformance-fixtures || bash scripts/agents/conformance-fixtures.sh
fi
if section_selected goal-cli-fixtures; then
  delivery_contract_skip goal-cli-fixtures || bash scripts/agents/goal-cli-fixtures.sh
fi
if section_selected telemetry-census-fixtures; then
  delivery_contract_skip telemetry-census-fixtures || bash scripts/agents/telemetry-census-fixtures.sh
fi
if section_selected return-schema-fixtures; then
  delivery_contract_skip return-schema-fixtures || bash scripts/agents/return-schema-fixtures.sh
fi
if section_selected config-identity-fixtures; then
  bash scripts/agents/config-identity-fixtures.sh
fi
# worktree-lease-fixtures.py retired with the python lease helper: it
# monkeypatched that module's internals (started_at, live, classify, ...),
# which cannot be expressed against the Go engine and is owned by
# internal/lease's unit tests under the go gate. The cross-process
# behavioral coverage it also carried (succession, lock contention)
# lives in scripts/agents/lease-succession-fixtures.sh below.
if section_selected authority-regression-fixtures; then
  delivery_contract_skip authority-regression-fixtures || bash scripts/agents/authority-regression-fixtures.sh
fi
if section_selected pre-commit-guard-fixtures; then
  delivery_contract_skip pre-commit-guard-fixtures || bash scripts/agents/pre-commit-guard-fixtures.sh
fi
if section_selected static-reproof-fixtures; then
  delivery_contract_skip static-reproof-fixtures || bash scripts/agents/static-reproof-fixtures.sh
fi
# PROJECT-DECLARED extra suites (born from the bm-2d rep-1 lesson: a
# sibling artifact's checks lived in no battery, and engine drift landed
# green three times in one weekend). The metasystem names nothing beyond
# itself — the audit's outside-reference fence stands — but a project may
# DECLARE companion suites in its own configuration, and a declaration is
# a promise: a declared suite that is missing or red refuses the run.
extra_suites=$(scripts/metasystem-config.sh get --key validate.extra-suites --default "" 2>/dev/null || true)
if section_selected project-extra-suites && [[ -n "$extra_suites" ]]; then
  for extra in $extra_suites; do
    if [[ ! -x "$extra" ]]; then
      echo "declared extra suite is missing or not executable: $extra (validate.extra-suites is a promise; clean the key or restore the suite)" >&2
      exit 1
    fi
    delivery_contract_skip project-extra-suites || bash "$extra"       || { echo "declared extra suite failed: $extra" >&2; exit 1; }
  done
fi
if section_selected record-protocol-fixtures; then
  delivery_contract_skip record-protocol-fixtures || bash scripts/agents/record-protocol-fixtures.sh
fi
if section_selected evidence-segment-fixtures; then
  delivery_contract_skip evidence-segment-fixtures || bash scripts/agents/evidence-segment-fixtures.sh
fi
if section_selected second-session-fixtures; then
  bash scripts/agents/second-session-fixtures.sh
fi
if section_selected lease-succession-fixtures; then
  delivery_contract_skip lease-succession-fixtures || bash scripts/agents/lease-succession-fixtures.sh
fi
if section_selected flight-recorder-fixtures; then
  delivery_contract_skip flight-recorder-fixtures || bash scripts/agents/flight-recorder-fixtures.sh
fi
if section_selected acp-fixtures; then
  delivery_contract_skip acp-fixtures || bash scripts/agents/acp-fixtures.sh
fi
if section_selected delegate-caps-fixtures; then
  delivery_contract_skip delegate-caps-fixtures || bash scripts/agents/delegate-caps-fixtures.sh
fi
if section_selected adapter-deadline-fixtures; then
  delivery_contract_skip adapter-deadline-fixtures || bash scripts/agents/adapter-deadline-fixtures.sh
fi
if section_selected enumeration-mode-fixtures; then
  enumeration_fixture_output=$(bash scripts/agents/enumerate-suite-fixtures.sh 2>&1) \
    || { printf '%s\n' "$enumeration_fixture_output" >&2; exit 1; }
fi
if section_selected runtime-contract-audits; then
[[ $(grep -Ec '^# Example model\.tier\.[123]=' metasystem.conf) -eq 3 ]] \
  || { echo "template demotion fixture: model tiers are not three commented examples" >&2; exit 1; }
[[ $(grep -Ec '^# Example mode\.[a-z0-9-]+\.role\.' metasystem.conf) -eq 3 ]] \
  || { echo "template demotion fixture: mode role overrides are not three commented examples" >&2; exit 1; }
if grep -Eq '^(model\.tier\.[1-9][0-9]*|mode\.[a-z0-9-]+\.role\.)' metasystem.conf; then
  echo "template demotion fixture: an optional tier or mode role key is still active" >&2
  exit 1
fi
for enforcement_source in scripts/enforcement/claude-code-hooks.json \
  scripts/enforcement/codex-hooks.json scripts/enforcement/devin-hooks.json; do
  enforcement_hooks=$("$engine" json get --file "$enforcement_source" --field hooks) \
    || { echo "$enforcement_source has no hooks object" >&2; exit 1; }
  for lifecycle_event in SessionStart Stop SessionEnd; do
    "$engine" json get --value "$enforcement_hooks" --field "$lifecycle_event" >/dev/null \
      || { echo "$enforcement_source hooks lack the $lifecycle_event lifecycle event" >&2; exit 1; }
  done
  [[ "$enforcement_hooks" == *supervision-hook.sh* ]] \
    || { echo "$enforcement_source hooks never invoke supervision-hook.sh" >&2; exit 1; }
done
# The common-lifecycle source-shape rows iterate the DECLARED
# population (agnosticism B1, ric critique r4-4: independent of the
# static enforcement map; fake's standalone shape is excluded by
# declaration, not by name).
common_lifecycle_population=$("$root/bin/metasystem" runtime list --with-common-lifecycle) \
  || { echo "the common-lifecycle population query refused" >&2; exit 1; }
[[ -n "$common_lifecycle_population" ]] \
  || { echo "the common-lifecycle population is empty" >&2; exit 1; }
while IFS= read -r runtime; do
  adapter="scripts/agents/adapters/$runtime.sh"
  [[ -f "$adapter" ]] || { echo "missing $runtime runtime adapter: $adapter" >&2; exit 1; }
  [[ -x "$adapter" ]] || { echo "$runtime runtime adapter is not executable: $adapter" >&2; exit 1; }
  bash -n "$adapter"
  adapter_usage=$($adapter --help 2>&1)
  for verb in identity config-identity signature enforcement-map contract probe dispatch follow-up cancel selftest; do
    grep -Fq "adapters/$runtime.sh $verb" <<<"$adapter_usage" \
      || { echo "$runtime adapter usage does not advertise $verb" >&2; exit 1; }
  done
  grep -Fq "adapter_common_init $runtime" "$adapter" \
    || { echo "$runtime adapter does not bind its snapshot runtime identity" >&2; exit 1; }
  grep -Fq "write_capability_snapshot $runtime \"\$version\" \"\$hash\"" "$adapter" \
    || { echo "$runtime adapter does not write its named capability snapshot" >&2; exit 1; }
done <<<"$common_lifecycle_population"
# EVERY declared adapter — fake included — proves its contract through
# the real snapshot construction path with dummy facts (ric critique
# r4-9's deterministic-construction resolution; no schema field, no
# live cutover).
adapter_population=$("$root/bin/metasystem" runtime list --with-adapter) \
  || { echo "the adapter population query refused" >&2; exit 1; }
[[ -n "$adapter_population" ]] \
  || { echo "the adapter population is empty" >&2; exit 1; }
while IFS= read -r runtime; do
  contract_json=$("scripts/agents/adapters/$runtime.sh" contract)
  contract_runtime=$("$root/bin/metasystem" json get --value "$contract_json" --field runtime)
  [[ "$contract_runtime" == "$runtime" ]] \
    || { echo "$runtime adapter contract snapshot carries wrong identity: $contract_runtime" >&2; exit 1; }
  # The FULL production snapshot shape, not a fragment (finding 12):
  # every required top-level member and the complete enforcement
  # object must decode.
  for contract_field in cliVersion configHash capabilities permissions envelopeEnforcement.writeRoots envelopeEnforcement.readRoots envelopeEnforcement.network; do
    "$root/bin/metasystem" json get --value "$contract_json" --field "$contract_field" >/dev/null \
      || { echo "$runtime adapter contract snapshot lacks $contract_field" >&2; exit 1; }
  done
done <<<"$adapter_population"
# The envelope-enforcement compare is GENERIC (agnosticism B1, ric
# critique r6-8): the registry's declared map and the adapter's
# side-effect-free enforcement-map verb are both decoded and
# canonicalized by the engine before comparison, for every runtime
# declaring a static map. Devin's all-notEnforced row is the measured
# truth (O-9/O-10 in records/misc/devin-support.md, demonstrated 2026-08-08):
# its declaration lives in internal/runtimes with that provenance, and
# changing it back requires evidence that enforcement returned.
enforcement_population=$("$root/bin/metasystem" runtime list --with-adapter) \
  || { echo "the enforcement population query refused" >&2; exit 1; }
enforcement_compared=0
while IFS= read -r rt_name; do
  registry_map=$("$root/bin/metasystem" runtime enforcement-map "$rt_name" 2>/dev/null) || continue
  adapter_map=$("scripts/agents/adapters/$rt_name.sh" enforcement-map)
  adapter_writeroots=$("$root/bin/metasystem" json get --value "$adapter_map" --field writeRoots)
  adapter_readroots=$("$root/bin/metasystem" json get --value "$adapter_map" --field readRoots)
  adapter_network=$("$root/bin/metasystem" json get --value "$adapter_map" --field network)
  registry_writeroots=$("$root/bin/metasystem" json get --value "$registry_map" --field writeRoots)
  registry_readroots=$("$root/bin/metasystem" json get --value "$registry_map" --field readRoots)
  registry_network=$("$root/bin/metasystem" json get --value "$registry_map" --field network)
  [[ "$adapter_writeroots" == "$registry_writeroots" && "$adapter_readroots" == "$registry_readroots" && "$adapter_network" == "$registry_network" ]] \
    || { echo "$rt_name adapter envelope enforcement drifted from the registry declaration" >&2; exit 1; }
  # Field-count equality kills extra members (finding 11): three keys
  # on both sides, no fourth passenger.
  adapter_keys=$(printf '%s' "$adapter_map" | tr -cd ':' | wc -c | tr -d ' ')
  registry_keys=$(printf '%s' "$registry_map" | tr -cd ':' | wc -c | tr -d ' ')
  [[ "$adapter_keys" == 3 && "$registry_keys" == 3 ]] \
    || { echo "$rt_name enforcement map carries unexpected members (adapter=$adapter_keys registry=$registry_keys)" >&2; exit 1; }
  enforcement_compared=$((enforcement_compared + 1))
done <<<"$enforcement_population"
(( enforcement_compared >= 3 )) \
  || { echo "only $enforcement_compared static enforcement maps compared — the population went missing" >&2; exit 1; }
# The host contract loop rides the DECLARED population (B1 critique
# finding 10: the hardcoded three already omitted devin).
host_population=$("$root/bin/metasystem" runtime list --with-host) \
  || { echo "the host population query refused" >&2; exit 1; }
[[ -n "$host_population" ]] \
  || { echo "the host population is empty" >&2; exit 1; }
while IFS= read -r runtime; do
  host="scripts/agents/hosts/$runtime.sh"
  [[ -x "$host" ]] || { echo "$runtime host adapter is missing or not executable: $host" >&2; exit 1; }
  grep -Fq 'start-turn' <<<"$($host --help 2>&1)" \
    || { echo "$runtime host adapter does not advertise start-turn" >&2; exit 1; }
done <<<"$host_population"
# The capability snapshot naming contract is pinned BEHAVIORALLY:
# TestSnapshotNameGrammar (internal/adapter) under the go gate, plus the
# fake-probe sequence fixture below — never by grepping Go source text
# (script-validate-9/D34: a reflow with zero behavior change failed the
# whole suite while a real regression was already caught behaviorally).
for role in design-critic implementer code-critic verifier investigator behavior-judge; do
  for suffix in md requirements.json; do
    [[ -f "scripts/agents/roles/$role.$suffix" ]] \
      || { echo "missing $role role asset: scripts/agents/roles/$role.$suffix" >&2; exit 1; }
  done
  [[ -f "scripts/agents/schemas/$role.schema.json" ]] \
    || { echo "missing $role return schema" >&2; exit 1; }
done

if (( template_mode )); then
  for link in \
    skills/take-a-step-back/SKILL.md \
    skills/take-a-step-back/agents/claude-profile.md \
    skills/take-a-step-back/agents/devin/AGENT.md \
    skills/take-a-step-back/agents/openai.yaml \
    skills/design-critique/SKILL.md \
    skills/design-critique/agents/claude-profile.md \
    skills/design-critique/agents/devin/AGENT.md \
    skills/design-critique/agents/openai.yaml \
    skills/code-critique/SKILL.md \
    skills/code-critique/agents/claude-profile.md \
    skills/code-critique/agents/devin/AGENT.md \
    skills/code-critique/agents/openai.yaml \
    skills/verify/SKILL.md \
    skills/verify/agents/claude-profile.md \
    skills/verify/agents/devin/AGENT.md \
    skills/verify/agents/openai.yaml \
    skills/refactor/SKILL.md \
    skills/refactor/agents/claude-profile.md \
    skills/refactor/agents/devin/AGENT.md \
    skills/refactor/agents/openai.yaml \
    skills/improve/SKILL.md \
    skills/improve/agents/claude-profile.md \
    skills/improve/agents/devin/AGENT.md \
    skills/improve/agents/openai.yaml \
    skills/retro/SKILL.md \
    skills/retro/agents/claude-profile.md \
    skills/retro/agents/devin/AGENT.md \
    skills/retro/agents/openai.yaml; do
    [[ -e "$link" ]] || { echo "missing template skill asset: $link" >&2; exit 1; }
  done
fi

# Registered skills must track their canonical source under skills/: copies
# must not drift, orphaned copies of a pruned skill must not linger, and a
# symlink to a pruned skill is dangling.
for regroot in .claude/skills .agents/skills .devin/skills; do
  [[ -d "$regroot" ]] || continue
  for reg in "$regroot"/*; do
    [[ -e "$reg" || -L "$reg" ]] || continue
    name=$(basename "$reg")
    if [[ -L "$reg" ]]; then
      [[ -e "$reg" ]] || { echo "registered skill link is dangling: $reg" >&2; exit 1; }
      continue
    fi
    [[ -d "$reg" ]] || continue
    [[ -d "skills/$name" ]] || { echo "orphaned registered skill copy: $reg has no skills/$name source" >&2; exit 1; }
    if ! diff -rq "$reg" "skills/$name" >/dev/null 2>&1; then
      echo "registered skill copy has drifted from its source: $reg vs skills/$name" >&2
      exit 1
    fi
  done
done

# In adopted mode metasystem.runtimes is live truth for registrations. Every
# remaining skill must be discoverable by each selected runtime, and copied
# launcher profiles must remain byte-identical to their canonical source.
if (( ! template_mode )); then
  # Through the config engine, honoring the same flag/env/local/conf
  # precedence the suite itself enforces (script-validate-8/D34).
  configured_runtimes=$(scripts/metasystem-config.sh get --key metasystem.runtimes)
  runtime_selected() { [[ ",$configured_runtimes," == *",$1,"* ]]; }
  for skill_dir in skills/*/; do
    [[ -d "$skill_dir" ]] || continue
    name=$(basename "$skill_dir")
    if runtime_selected claude; then
      [[ -e ".claude/skills/$name" ]] \
        || { echo "claude registration missing for skill: $name" >&2; exit 1; }
      if [[ -f "$skill_dir/agents/claude-profile.md" ]]; then
        profile=".claude/agents/$name.md"
        [[ -f "$profile" ]] || { echo "claude profile registration missing: $profile" >&2; exit 1; }
        cmp -s "$skill_dir/agents/claude-profile.md" "$profile" \
          || { echo "claude profile drifted from $skill_dir/agents/claude-profile.md: $profile" >&2; exit 1; }
      fi
    fi
    if runtime_selected codex || runtime_selected devin; then
      [[ -e ".agents/skills/$name" ]] \
        || { echo "shared .agents skill registration missing: $name" >&2; exit 1; }
    fi
    if runtime_selected devin; then
      [[ -e ".devin/skills/$name" ]] \
        || { echo "devin skill registration missing: $name" >&2; exit 1; }
      if [[ -f "$skill_dir/agents/devin/AGENT.md" ]]; then
        profile=".devin/agents/$name/AGENT.md"
        [[ -f "$profile" ]] || { echo "devin profile registration missing: $profile" >&2; exit 1; }
        cmp -s "$skill_dir/agents/devin/AGENT.md" "$profile" \
          || { echo "devin profile drifted from $skill_dir/agents/devin/AGENT.md: $profile" >&2; exit 1; }
      fi
    fi
  done
fi
fi

tmp=${METASYSTEM_SUITE_PROGRESS_TMP:-}
if [[ -z "$tmp" ]]; then
  tmp=$(mktemp -d)
else
  mkdir -p "$tmp"
fi
agent_supervision_repo=

if section_selected runtime-contract-audits; then
# The fake runtime is the only sandbox this suite owns. Its probe drives the
# denied write and network-call paths and reports the observed nonzero status;
# real adapters are inspected only for their declarations above.
fake_probe_root="$tmp/fake-envelope-probe"
mkdir -p "$fake_probe_root/scripts/agents/adapters"
cp scripts/agents/adapters/fake.sh "$fake_probe_root/scripts/agents/adapters/"
cp scripts/agents/fixture-budget.sh "$fake_probe_root/scripts/agents/"
fake_probe_result="$tmp/fake-envelope-probe-result.json"
# The bare probe root carries no engine; point the adapter at this checkout's.
fake_snapshot=$(METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT="$fake_probe_result" \
  METASYSTEM_BIN="$PWD/bin/metasystem" \
  "$fake_probe_root/scripts/agents/adapters/fake.sh" probe)
[[ "$("$engine" json get --file "$fake_snapshot" --field envelopeEnforcement)" == \
   "$(canonical_json '{"writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped"}')" ]] \
  || { echo "fake snapshot envelope enforcement drifted" >&2; cat "$fake_snapshot" >&2; exit 1; }
[[ "$(canonical_json "$(cat "$fake_probe_result")")" == \
   "$(canonical_json '{"writeRoots": {"observed": "denied", "exitStatus": 77}, "network": {"observed": "denied", "exitStatus": 77}}')" ]] \
  || { echo "fake envelope probe did not observe both denials with status 77" >&2; cat "$fake_probe_result" >&2; exit 1; }
fi

# Every fixture repository this suite arms, so cleanup can stop all of them.
# A single variable was tracked before, reassigned as the suite moved between
# repositories and emptied at the end, so the trap shut down nothing and each
# armed repository leaked its supervision owner. Two such owners were found
# alive after 25 hours, each scanning every process on the machine every five
# seconds. Killing components does not help: the owner restarts them by design,
# so only a shutdown of the owner ends it.
armed_supervision_repos=()
track_armed_supervision() { # repository
  local repo=$1 known
  [[ -n "$repo" ]] || return 0
  for known in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ "$known" == "$repo" ]] && return 0
  done
  armed_supervision_repos+=("$repo")
}
validation_cleanup_started=0
validation_cleanup() {
  (( validation_cleanup_started )) && return 0
  validation_cleanup_started=1
  checkout_execution_guard_release || true
  [[ -z "${gate_run_marker:-}" ]] || rm -f "$gate_run_marker"
  # The witness dies with the run (D33's lifecycle): the armed state
  # dir must never outlive the validation that produced it.
  [[ -z "${witness_state:-}" ]] || rm -rf "$witness_state"
  local repo
  for repo in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ -x "$repo/scripts/agents/arm-supervision.sh" ]] || continue
    if [[ "$repo" == "${runner_repo:-}" ]] && declare -p runner_process_env >/dev/null 2>&1; then
      "${runner_process_env[@]}" "$repo/scripts/agents/arm-supervision.sh" \
        --repo "$repo" --shutdown >/dev/null 2>&1 || true
    else
      "$repo/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown >/dev/null 2>&1 || true
    fi
  done
  # Shutdown returns before the owner and its children have fully exited, and
  # a straggler writing one last record while rm walks the tree turns a PASSED
  # validation into a red exit ("Directory not empty"). Wait for every process
  # still referencing the sandbox to finish, bounded, then delete; the leak
  # guard is this wait, not the rm exit code, so a residual wrinkle in the
  # teardown never overrules the verdict printed above it.
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    pgrep -f "$tmp" >/dev/null 2>&1 || break
    sleep 0.5
  done
  # PRESERVE FAILURE EVIDENCE (flight-recorder D-8, direct-fix bar): a
  # failing run's temp tree is the diagnosis, and deleting it forced every
  # investigation to re-run the suite with the trap stripped by hand. On a
  # nonzero exit the tree moves aside and its path is printed; only green
  # runs clean up. Evidence beats disk.
  if [[ "${validation_exit_status:-1}" != 0 && -d "$tmp" ]]; then
    keep="artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-$$"
    mkdir -p "$(dirname "$keep")"
    if mv "$tmp" "$keep" 2>/dev/null; then
      echo "suite failure evidence preserved: $keep" >&2
    fi
    return 0
  fi
  rm -rf "$tmp" 2>/dev/null || { sleep 1; rm -rf "$tmp" 2>/dev/null || true; }
}
on_signal() {
  local signal=$1
  (( validation_cleanup_started )) && return 0
  validation_exit_status=$((128 + signal))
  suite_progress_finish
  validation_cleanup
  trap - EXIT
  exit "$validation_exit_status"
}
trap 'validation_exit_status=$?; suite_progress_finish; validation_cleanup' EXIT
trap 'on_signal 2' INT
trap 'on_signal 15' TERM

if section_selected agent-protocol-fixtures; then
# IL-3: prove the audit's fallback with a PATH that contains its ordinary POSIX
# tools but deliberately contains no rg binary.
no_rg_bin="$tmp/no-rg-bin"
mkdir -p "$no_rg_bin"
for command_name in cat find grep sort tr wc; do
  cp "$(command -v "$command_name")" "$no_rg_bin/$command_name"
  chmod +x "$no_rg_bin/$command_name"
done
env PATH="$no_rg_bin" /bin/bash scripts/audit-metasystem.sh . >"$tmp/audit-no-rg.out"


# The brief contains only orchestrator-authored header fields. Dispatch owns
# job identity, role, runtime, model, and round, so none may appear as a brief
# header before those values exist.
brief=scripts/agents/templates/brief.md
for header in 'Working Mode:' 'Mission Stream:' 'Orchestrator Identity:' 'Date:'; do
  grep -q "^$header" "$brief" || { echo "brief template is missing authored header: $header" >&2; exit 1; }
done
if grep -Eq '^(Job-Id|Role|Runtime|Model|Round):' "$brief"; then
  echo "brief template contains a dispatch-assigned header" >&2
  exit 1
fi
grep -q '^Finding Id:' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template does not restate one finding" >&2; exit 1; }
grep -q '^Disposition:' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template does not restate the disposition" >&2; exit 1; }
grep -q '^# Unchanged Return Contract$' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template can lose the original return contract" >&2; exit 1; }
# Every <...> placeholder in the whole body, in order. The scan crosses
# line boundaries the way the placeholder grammar does; a parameter that
# somehow carried a newline is printed with a visible \n so it can never
# masquerade as two well-formed parameters.
host_turn_parameters=$(awk '
  { body = body $0 "\n" }
  END {
    n = length(body); start = 0
    for (i = 1; i <= n; i++) {
      ch = substr(body, i, 1)
      if (ch == "<") start = i
      else if (ch == ">" && start && i - start > 1) {
        parameter = substr(body, start + 1, i - start - 1)
        gsub(/\n/, "\\n", parameter)
        print parameter
        start = 0
      }
      else if (ch == ">") start = 0
    }
  }' scripts/agents/templates/host-turn-instruction.md)
[[ "$host_turn_parameters" == $'cycle-number\nfence-headroom\nyes | no' ]] \
  || { echo "host-turn instruction parameters drifted from cycle, fence headroom, reconciliation" >&2; exit 1; }
if grep -Fq 'Runtime:' scripts/agents/templates/host-turn-instruction.md; then
  echo "host-turn instruction is parameterized by runtime" >&2
  exit 1
fi

# Assert the machine-readable shapes item 2 owns. Later dispatcher fixtures
# exercise expansion and capability gating; here the shipped declarations
# must remain minimal and must not grow baseline guarantees into snapshots.
return_schema_roles='design-critic implementer code-critic verifier investigator behavior-judge'
return_schema_common_fields='jobId round runtime sessionId model evidence gaps mode'
return_schema_owned_fields() { # role: print the role-owned schema fields
  case $1 in
    design-critic) printf '%s\n' reviewedCommit findings verdictMaterialCount ;;
    implementer) printf '%s\n' riskiestPart diffBoundary whatWasDone ;;
    code-critic) printf '%s\n' reviewedTree findings verdictMaterialCount ;;
    verifier) printf '%s\n' riskiestPart whatWasDone ;;
    investigator) printf '%s\n' frozenFrame theories classifications stopLoss ;;
    behavior-judge) printf '%s\n' dimensions reliabilityWatch ;;
    *) echo "return_schema_owned_fields: unknown role $1" >&2; return 1 ;;
  esac
}

# A schema node member with its absent-member default. An explicit null is
# NOT folded into the default: a null properties or required list is a
# malformed schema and must refuse, not read as empty.
schema_node_member() { # node JSON, field, default when the field is absent
  local value
  if value=$("$engine" json get --value "$1" --field "$2" 2>/dev/null); then
    printf '%s\n' "$value"
  else
    printf '%s\n' "$3"
  fi
}

# One sorted-field-set check for a closed object schema: the properties,
# the required list, and the expected field roster must be the same set,
# and additionalProperties must be exactly false.
assert_schema_field_set() { # schema JSON, expected fields (newline-separated), failure message
  local schema=$1 expected=$2 message=$3 properties required_json actual required additional
  properties=$(schema_node_member "$schema" properties '{}')
  required_json=$(schema_node_member "$schema" required '[]')
  [[ "$properties" == \{* && "$required_json" == \[* ]] || { echo "$message" >&2; exit 1; }
  actual=$(json_object_keys "$properties" | LC_ALL=C sort -u)
  required=$(json_elements "$required_json" | sed 's/^"//; s/"$//' | LC_ALL=C sort -u)
  additional=$("$engine" json get --value "$schema" --field additionalProperties --default null)
  [[ "$actual" == "$expected" && "$required" == "$expected" && "$additional" == false ]] \
    || { echo "$message" >&2; exit 1; }
}

for role in $return_schema_roles; do
  role_schema=$(canonical_json "$(cat "scripts/agents/schemas/$role.schema.json")")
  role_expected_fields=$(printf '%s\n' $return_schema_common_fields; return_schema_owned_fields "$role")
  assert_schema_field_set "$role_schema" "$(printf '%s\n' "$role_expected_fields" | LC_ALL=C sort -u)" \
    "$role schema property set drifted from the protocol"
done

orchestrator_schema=$(canonical_json "$(cat scripts/agents/schemas/orchestrator.schema.json)")
orchestrator_expected_fields=$(printf '%s\n' turnId missionId cycle dispatched certified \
  streamUpdatesRequested askCandidates factsForLedger gaps identity | LC_ALL=C sort -u)
assert_schema_field_set "$orchestrator_schema" "$orchestrator_expected_fields" \
  "orchestrator schema property set drifted from the host-turn protocol"

# Every object node in the orchestrator schema must be fully enumerated:
# additionalProperties false and required naming every property, at every
# nesting level, through array item schemas too.
assert_closed_schema() { # canonical schema node, dotted path for the report
  local node=$1 path=$2 node_type properties required_json keys_sorted required_sorted member name child
  node_type=$("$engine" json get --value "$node" --field type --default '')
  if [[ "$node_type" == object ]]; then
    properties=$(schema_node_member "$node" properties '{}')
    required_json=$(schema_node_member "$node" required '[]')
    [[ "$properties" == \{* && "$required_json" == \[* ]] \
      || { echo "orchestrator schema object is not fully enumerated: $path" >&2; exit 1; }
    keys_sorted=$(json_object_keys "$properties" | LC_ALL=C sort -u)
    required_sorted=$(json_elements "$required_json" | sed 's/^"//; s/"$//' | LC_ALL=C sort -u)
    [[ "$("$engine" json get --value "$node" --field additionalProperties --default null)" == false \
       && "$required_sorted" == "$keys_sorted" ]] \
      || { echo "orchestrator schema object is not fully enumerated: $path" >&2; exit 1; }
    while IFS= read -r member; do
      [[ -n "$member" ]] || continue
      name=${member#\"}
      name=${name%%\":*}
      child=${member#\"$name\":}
      assert_closed_schema "$child" "$path.$name"
    done < <(json_elements "$properties")
  fi
  if [[ "$node_type" == array ]]; then
    child=$(schema_node_member "$node" items '{}')
    [[ "$child" == \{* ]] \
      || { echo "orchestrator schema array has no item schema: $path" >&2; exit 1; }
    assert_closed_schema "$child" "$path[]"
  fi
}
assert_closed_schema "$orchestrator_schema" '$'

# Network is granted by default: the container or VM is the isolation
# boundary, and a delegate that cannot resolve a dependency or read
# documentation cannot do real work. A repository narrows it for every role
# with dispatch.permissions.network=deny.
permission_preset_expected() { # preset name
  case $1 in
    none) printf '%s' '{"readRoots": ["."], "writeRoots": [], "network": "allow", "approvals": "deny", "tools": "read-only"}' ;;
    workspace) printf '%s' '{"readRoots": ["."], "writeRoots": ["<worktree>"], "network": "allow", "approvals": "deny", "tools": "runtime-default"}' ;;
    *) echo "permission_preset_expected: unknown preset $1" >&2; return 1 ;;
  esac
}
for preset in none workspace; do
  [[ "$(canonical_json "$(cat "scripts/agents/permissions/$preset.json")")" == \
     "$(canonical_json "$(permission_preset_expected "$preset")")" ]] \
    || { echo "$preset permission preset drifted from its envelope" >&2; exit 1; }
done

for role in $return_schema_roles; do
  requirement=$(canonical_json "$(cat "scripts/agents/roles/$role.requirements.json")")
  requirement_shape_ok=1
  "$engine" json get --value "$requirement" --field required >/dev/null 2>&1 || requirement_shape_ok=0
  "$engine" json get --value "$requirement" --field optional >/dev/null 2>&1 || requirement_shape_ok=0
  while IFS= read -r requirement_key; do
    [[ -n "$requirement_key" ]] || continue
    case $requirement_key in required|optional|waivers) ;; *) requirement_shape_ok=0 ;; esac
  done < <(json_object_keys "$requirement")
  (( requirement_shape_ok )) \
    || { echo "$role capability declaration has unknown top-level fields" >&2; exit 1; }
  # A declared waivers member must be an object of field -> runtime-name list.
  if "$engine" json get --value "$requirement" --field waivers >/dev/null 2>&1; then
    waivers=$("$engine" json get --value "$requirement" --field waivers)
  else
    waivers='{}'
  fi
  waivers_ok=1
  [[ "$waivers" == \{* ]] || waivers_ok=0
  if (( waivers_ok )); then
    while IFS= read -r waiver_member; do
      [[ -n "$waiver_member" ]] || continue
      waiver_field=${waiver_member#\"}
      waiver_field=${waiver_field%%\":*}
      waiver_runtimes=${waiver_member#\"$waiver_field\":}
      [[ "$waiver_runtimes" == \[* ]] || { waivers_ok=0; break; }
      while IFS= read -r waiver_runtime; do
        [[ -n "$waiver_runtime" ]] || continue
        [[ "$waiver_runtime" == \"* ]] || { waivers_ok=0; break; }
      done < <(json_elements "$waiver_runtimes")
      (( waivers_ok )) || break
    done < <(json_elements "$waivers")
  fi
  (( waivers_ok )) || { echo "$role capability waivers have an invalid shape" >&2; exit 1; }
  [[ "$("$engine" json get --value "$requirement" --field required)" == "[]" ]] \
    || { echo "$role incorrectly repeats adapter-guaranteed baseline capabilities" >&2; exit 1; }
  role_optional=$("$engine" json get --value "$requirement" --field optional)
  if [[ "$role" == implementer ]]; then
    implementer_optional_ok=1
    [[ "$role_optional" == \{* ]] || implementer_optional_ok=0
    [[ "$(json_object_keys "$role_optional" 2>/dev/null)" == resume ]] || implementer_optional_ok=0
    resume=$("$engine" json get --value "$role_optional" --field resume --default null 2>/dev/null) || resume=null
    [[ "$resume" == \{* ]] || implementer_optional_ok=0
    resume_fallback=$("$engine" json get --value "$resume" --field fallback --default '' 2>/dev/null) || resume_fallback=
    case $resume_fallback in ''|null|false|0|'[]'|'{}') implementer_optional_ok=0 ;; esac
    (( implementer_optional_ok )) \
      || { echo "implementer resume capability lacks its embed fallback" >&2; exit 1; }
  else
    [[ "$role_optional" == "{}" ]] \
      || { echo "$role declares a variable capability it does not need" >&2; exit 1; }
  fi
done

# Quote markers name their canonical source. The checker compares the content
# bytes rather than trusting a second prose copy of the binding criterion.
scripts/agents/check-preamble-quotes.sh
# Count the quote blocks from one source whose body carries one marker. A
# block runs from a `<!-- quote source="..." -->` line to the first
# `<!-- /quote -->` line; block bodies are whole lines, so a marker that
# names a full heading line matches as a line suffix and a bare phrase
# matches anywhere in a line.
count_quote_blocks() { # roles file, source, marker, 1 when the marker ends its line
  awk -v wanted_source="$2" -v marker="$3" -v marker_ends_line="$4" '
    BEGIN { count = 0; inblock = 0 }
    !inblock && /^<!-- quote source="[^"]+" -->$/ {
      block_source = $0
      sub(/^<!-- quote source="/, "", block_source)
      sub(/" -->$/, "", block_source)
      inblock = 1; hit = 0
      next
    }
    inblock && $0 == "<!-- /quote -->" {
      if (block_source == wanted_source && hit) count++
      inblock = 0
      next
    }
    inblock {
      if (marker_ends_line) {
        if (length($0) >= length(marker) && substr($0, length($0) - length(marker) + 1) == marker) hit = 1
      } else if (index($0, marker)) hit = 1
    }
    END { print count }
  ' "$1"
}
# Each mandated (source, marker) pair must appear in EXACTLY one block:
# zero means the preamble lost its mandated quote, and more than one means
# deleting the block would go undetected, so the requirement would not be
# protecting anything.
while IFS=$'\t' read -r quote_source quote_marker quote_marker_ends_line; do
  quote_block_count=$(count_quote_blocks scripts/agents/roles/orchestrator.md \
    "$quote_source" "$quote_marker" "$quote_marker_ends_line")
  [[ "$quote_block_count" -ge 1 ]] \
    || { echo "orchestrator preamble lacks mandated quote block: $quote_source $quote_marker" >&2; exit 1; }
  [[ "$quote_block_count" -eq 1 ]] \
    || { echo "orchestrator required-block deletion would go undetected: $quote_source $quote_marker appears in $quote_block_count blocks" >&2; exit 1; }
done <<'REQUIRED'
AGENTS.md	## Completion	1
docs/orchestration.md	## Delegation Contract	1
docs/orchestration.md	### Working without the human	1
docs/collaboration.md	## Review Guide in Reports	1
docs/collaboration.md	## Escalation Shape	1
docs/project-rules.md	These require explicit in-task approval	0
REQUIRED
cp -R scripts/agents/roles "$tmp/drifted-roles"
sed 's/build something DIFFERENT/build the same thing/' \
  "$tmp/drifted-roles/design-critic.md" >"$tmp/drifted-roles/design-critic.md.new"
mv "$tmp/drifted-roles/design-critic.md.new" "$tmp/drifted-roles/design-critic.md"
set +e
scripts/agents/check-preamble-quotes.sh --roles-dir "$tmp/drifted-roles" >"$tmp/quote-drift.out" 2>&1
quote_status=$?
set -e
if [[ $quote_status -eq 0 ]]; then
  echo "preamble quote checker accepted a drifted binding criterion" >&2
  exit 1
fi
[[ $quote_status -eq 1 ]] \
  || { echo "preamble quote checker used $quote_status instead of exit 1 for drift" >&2; exit 1; }
grep -q 'quote drifted from skills/design-critique/SKILL.md' "$tmp/quote-drift.out" \
  || { echo "preamble quote checker did not name the drifted source" >&2; exit 1; }
cp -R scripts/agents/roles "$tmp/drifted-code-roles"
sed 's/ship a defect/ship no defect/' \
  "$tmp/drifted-code-roles/code-critic.md" >"$tmp/drifted-code-roles/code-critic.md.new"
mv "$tmp/drifted-code-roles/code-critic.md.new" "$tmp/drifted-code-roles/code-critic.md"
set +e
scripts/agents/check-preamble-quotes.sh --roles-dir "$tmp/drifted-code-roles" >"$tmp/code-quote-drift.out" 2>&1
code_quote_status=$?
set -e
[[ $code_quote_status -eq 1 ]] \
  || { echo "preamble quote checker accepted a drifted code-critique criterion" >&2; exit 1; }
grep -q 'quote drifted from skills/code-critique/SKILL.md' "$tmp/code-quote-drift.out" \
  || { echo "preamble quote checker did not name the code-critique source" >&2; exit 1; }
cp -R scripts/agents/roles "$tmp/drifted-orchestrator-roles"
sed "s/The human's absence narrows/The human's presence narrows/" \
  "$tmp/drifted-orchestrator-roles/orchestrator.md" >"$tmp/drifted-orchestrator-roles/orchestrator.md.new"
mv "$tmp/drifted-orchestrator-roles/orchestrator.md.new" \
  "$tmp/drifted-orchestrator-roles/orchestrator.md"
set +e
scripts/agents/check-preamble-quotes.sh \
  --roles-dir "$tmp/drifted-orchestrator-roles" >"$tmp/orchestrator-quote-drift.out" 2>&1
orchestrator_quote_status=$?
set -e
[[ $orchestrator_quote_status -eq 1 ]] \
  || { echo "preamble quote checker accepted a drifted orchestrator quote" >&2; exit 1; }
grep -q 'quote drifted from docs/orchestration.md' "$tmp/orchestrator-quote-drift.out" \
  || { echo "preamble quote checker did not name the drifted orchestrator source" >&2; exit 1; }

# Build canonical positive returns and one role-specific negative per role.
# JSON remains canonical; these fixtures never rely on the Markdown view.
return_fixtures="$tmp/returns"
mkdir -p "$return_fixtures"
# The eight fields every role return shares, minus mode, which varies by
# role and is appended alongside the role-owned fields.
return_fixture_common='"jobId":"fixture-job","round":1,"runtime":"fake","sessionId":"session-1","model":{"requested":"fake-model","effective":"fake-model"},"evidence":[{"command":"scripts/validate-metasystem.sh","observed":"fixture output","level":"ran"}],"gaps":[]'
fixture_zero_sha=$(printf '0%.0s' {1..40})
fixture_ones_digest=$(printf '1%.0s' {1..64})

behavior_dimension() { # id, anchors JSON, findings JSON
  printf '{"id":"%s","score":4,"rationale":"fixture judgment","anchors":%s,"findings":%s}' "$1" "$2" "$3"
}
# The eight judged dimensions; only the first carries findings, and only
# its anchors ever vary, so the two are parameters.
behavior_dimensions_json() { # first dimension's anchors JSON, first dimension's findings JSON
  local out dimension_id
  out='['$(behavior_dimension brief-quality "$1" "$2")
  for dimension_id in adjudication-quality delegation-discipline gap-handling \
    spec-fidelity repeated-work proportionality evidence-honesty; do
    out+=,$(behavior_dimension "$dimension_id" "$behavior_ledger_anchor" '[]')
  done
  printf '%s]' "$out"
}
behavior_ledger_anchor='[{"file":"artifacts/agents/missions/fixture/ledger.md","line":1}]'
behavior_zero_line_anchor='[{"file":"artifacts/agents/missions/fixture/ledger.md","line":0}]'
behavior_first_findings='[{"id":"BQ-1","claim":"fixture finding","evidence":"fixture evidence","anchors":[{"file":"artifacts/agents/fixture/rounds/1/prompt.md","line":10}]}]'
behavior_first_findings_unanchored='[{"id":"BQ-1","claim":"fixture finding","evidence":"fixture evidence","anchors":[]}]'

printf '{"turnId":"turn-3","missionId":"fixture-mission","cycle":3,"dispatched":[{"jobId":"fixture-job","role":"implementer","stream":"stream-a"}],"certified":[{"jobId":"prior-job","verdict":"accepted","evidence":"focused checks passed","authorizationDigest":"%s"}],"streamUpdatesRequested":[{"streamId":"stream-a","requestedState":"active","reason":"work remains"}],"askCandidates":[{"streamId":"stream-b","reasonClass":"reserved-decision","question":"Approve the contract change?","supersedes":null}],"factsForLedger":["focused check exposed one new fact"],"gaps":[],"identity":{"runtime":"fake","model":"fake-model","sessionId":null}}\n' \
  "$fixture_ones_digest" >"$return_fixtures/orchestrator-positive.json"
printf '{%s,"mode":"design","reviewedCommit":"%s","findings":[{"id":"F-1","severity":"high","material":true,"claim":"contract gap","evidence":"read design"}],"verdictMaterialCount":1}\n' \
  "$return_fixture_common" "$fixture_zero_sha" >"$return_fixtures/design-critic-positive.json"
printf '{%s,"mode":"implement","reviewedTree":"%s","findings":[],"verdictMaterialCount":0}\n' \
  "$return_fixture_common" "$fixture_zero_sha" >"$return_fixtures/code-critic-positive.json"
printf '{%s,"mode":"implement","riskiestPart":"schema boundary","diffBoundary":["scripts/example.sh"],"whatWasDone":"implemented the brief"}\n' \
  "$return_fixture_common" >"$return_fixtures/implementer-positive.json"
printf '{%s,"mode":"verify","riskiestPart":"failure path","whatWasDone":"drove the runnable surface"}\n' \
  "$return_fixture_common" >"$return_fixtures/verifier-positive.json"
printf '{%s,"mode":"take-a-step-back","frozenFrame":"symptom and boundary frozen","theories":[{"statement":"owner lost state","evidenceFor":"trace","evidenceAgainst":"focused check"}],"classifications":["falsified-continue"],"stopLoss":{"triggered":false,"trigger":null}}\n' \
  "$return_fixture_common" >"$return_fixtures/investigator-positive.json"
printf '{%s,"mode":"verify","dimensions":%s,"reliabilityWatch":[{"dimension":"proportionality","mechanicalMetric":"fence-economy","agreement":"agrees","explanation":"fixture agreement","anchors":[{"file":"artifacts/agents/missions/fixture/state.json","line":5}]}]}\n' \
  "$return_fixture_common" \
  "$(behavior_dimensions_json "$behavior_ledger_anchor" "$behavior_first_findings")" \
  >"$return_fixtures/behavior-judge-positive.json"

# Every negative is the positive with ONE drift, derived rather than
# restated, so the checker below can only be failing for that drift.
for role in orchestrator design-critic implementer code-critic verifier investigator behavior-judge; do
  cp "$return_fixtures/$role-positive.json" "$return_fixtures/$role-negative.json"
done
json_remove_field "$return_fixtures/orchestrator-negative.json" factsForLedger
json_remove_field "$return_fixtures/design-critic-negative.json" findings
json_remove_field "$return_fixtures/design-critic-negative.json" verdictMaterialCount
"$engine" json set --file "$return_fixtures/code-critic-negative.json" \
  --field 'whatWasDone=critics do not own this section'
json_remove_field "$return_fixtures/implementer-negative.json" diffBoundary
json_insert_field "$return_fixtures/verifier-negative.json" diffBoundary '["not verifier-owned"]'
json_remove_field "$return_fixtures/investigator-negative.json" frozenFrame
json_remove_field "$return_fixtures/investigator-negative.json" theories
json_replace_field "$return_fixtures/behavior-judge-negative.json" dimensions \
  "$(behavior_dimensions_json "$behavior_ledger_anchor" "$behavior_first_findings_unanchored")"

cp "$return_fixtures/behavior-judge-positive.json" "$return_fixtures/behavior-judge-empty-dimensions.json"
json_replace_field "$return_fixtures/behavior-judge-empty-dimensions.json" dimensions '[]'
cp "$return_fixtures/behavior-judge-positive.json" "$return_fixtures/behavior-judge-invalid-anchor.json"
json_replace_field "$return_fixtures/behavior-judge-invalid-anchor.json" dimensions \
  "$(behavior_dimensions_json "$behavior_zero_line_anchor" "$behavior_first_findings")"

cp "$return_fixtures/design-critic-positive.json" "$return_fixtures/critic-miscount.json"
"$engine" json set --file "$return_fixtures/critic-miscount.json" --int verdictMaterialCount=0
cp "$return_fixtures/design-critic-positive.json" "$return_fixtures/critic-missing-verdict.json"
json_remove_field "$return_fixtures/critic-missing-verdict.json" verdictMaterialCount

for role in orchestrator design-critic implementer code-critic verifier investigator behavior-judge; do
  scripts/assert-return-complete.sh --role "$role" --file "$return_fixtures/$role-positive.json"
done

check_bad_return() { # role, file, required diagnostic text
  local role=$1 file=$2 expected=$3 output status
  output="$tmp/${role}-negative.out"
  set +e
  scripts/assert-return-complete.sh --role "$role" --file "$file" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "return checker accepted the negative $role fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "return checker used $status instead of exit 1 for $role validation" >&2; exit 1; }
  grep -Fq "$expected" "$output" \
    || { echo "return checker did not name the $role violation: $expected" >&2; exit 1; }
}

check_bad_return orchestrator "$return_fixtures/orchestrator-negative.json" '$.factsForLedger is required'
check_bad_return design-critic "$return_fixtures/design-critic-negative.json" '$.findings is required'
check_bad_return code-critic "$return_fixtures/code-critic-negative.json" '$.whatWasDone is not allowed'
check_bad_return implementer "$return_fixtures/implementer-negative.json" '$.diffBoundary is required'
check_bad_return verifier "$return_fixtures/verifier-negative.json" '$.diffBoundary is not allowed'
check_bad_return investigator "$return_fixtures/investigator-negative.json" '$.frozenFrame is required'
check_bad_return behavior-judge "$return_fixtures/behavior-judge-negative.json" '$.dimensions[0].findings[0].anchors must contain at least one file-and-line anchor'
check_bad_return behavior-judge "$return_fixtures/behavior-judge-empty-dimensions.json" '$.dimensions must contain at least one requested judged dimension with no duplicate ids'
check_bad_return behavior-judge "$return_fixtures/behavior-judge-invalid-anchor.json" '$.dimensions[0].anchors[0].line must be a positive one-based line number'
check_bad_return design-critic "$return_fixtures/critic-missing-verdict.json" '$.verdictMaterialCount is required'
check_bad_return design-critic "$return_fixtures/critic-miscount.json" '$.verdictMaterialCount must equal the count of findings with material=true'

if (( template_mode )); then
  set +e
  scripts/agents/dispatch.sh dispatch --role code-critic --brief scripts/agents/templates/brief.md \
    >"$tmp/code-critic-missing-reviews.out" 2>&1
  missing_reviews_status=$?
  scripts/agents/dispatch.sh dispatch --role implementer --brief scripts/agents/templates/brief.md --reviews fixture-job \
    >"$tmp/non-critic-reviews.out" 2>&1
  non_critic_reviews_status=$?
  set -e
  [[ $missing_reviews_status -eq 2 ]] \
    || { echo "code-critic dispatch without --reviews did not use exit 2" >&2; exit 1; }
  grep -Fq 'code-critic dispatch requires --reviews <implementer-job-id>' "$tmp/code-critic-missing-reviews.out" \
    || { echo "code-critic dispatch did not require its review relation" >&2; exit 1; }
  [[ $non_critic_reviews_status -eq 2 ]] \
    || { echo "non-critic --reviews dispatch did not use exit 2" >&2; exit 1; }
  grep -Fq -- '--reviews is only valid for the code-critic and warden roles' "$tmp/non-critic-reviews.out" \
    || { echo "dispatcher accepted --reviews for a non-critic role" >&2; exit 1; }
fi

set +e
scripts/assert-return-complete.sh >"$tmp/return-usage.out" 2>&1
return_usage_status=$?
set -e
[[ $return_usage_status -eq 2 ]] \
  || { echo "return checker used $return_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Host-turn prompts are checked against the canonical turn record and shipped
# preamble before a host process may start. The positive prompt is deliberately
# hand-authored so this fixture does not share assembly logic with the checker.
turn_fixture="$tmp/turn-prompt"
turn_dir="$turn_fixture/turn-3"
mkdir -p "$turn_dir"
cat >"$turn_dir/turn.json" <<'EOF'
{
  "missionId": "fixture-mission",
  "turnId": "turn-3",
  "cycle": 3,
  "runtime": "fake",
  "model": "fake-model",
  "hostSession": null,
  "reconciliation": false,
  "startedAt": "2026-08-04T12:00:00Z",
  "pid": 1234,
  "outcome": null
}
EOF
good_turn_prompt="$turn_fixture/good.md"
{
  printf '%s\n' \
    'Mission-Id: fixture-mission' \
    'Turn-Id: turn-3' \
    'Cycle: 3' \
    'Host-Session: none' \
    'Runtime: fake' \
    'Model: fake-model' \
    'Reconciliation: no'
  printf '\n'
  cat scripts/agents/roles/orchestrator.md
  printf '\n'
  cat <<'EOF'
## Mission Contract
Signed fixture mission contract.

## Ledger Tail
<<<DATA>>>
1	contract-improved	aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	metric=1
2	unresolved	bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb	metric=1
<<<END>>>

## Human Answers
<<<DATA>>>
ask-0	stream-a	2026-08-18T00:00:00Z	May the fixture proceed?	Yes, option A, this mission only.
<<<END>>>

## Open Asks
<<<DATA>>>
ask-1	stream-a	reserved-decision	Approve the named API contract?
<<<END>>>

## Streams
<<<DATA>>>
stream-a	active	Make the fixture gate pass	none	ask-0
stream-b	parked-reserved	Publish the fixture	Awaiting approval	none
<<<END>>>

## Reconciliation
<<<DATA>>>
(none)
<<<END>>>

## Landed Returns
<<<DATA>>>
chain-a	2	artifacts/agents/chain-a/rounds/2/return.json
chain-b	invalid	artifacts/agents/chain-b/rounds/1/return.json
chain-c	unreadable	none
<<<END>>>

## This Turn
Cycle: 3
Fence headroom: cycles=2,jobs=3
Reconciliation: no

Advance active streams by designing, dispatching, reviewing, and certifying. When Reconciliation is `yes`, reconcile the prior turn before starting new work. End this turn when work is dispatched and reviewed; never wait inside the turn.
EOF
} >"$good_turn_prompt"

scripts/assert-turn-prompt.sh --file "$good_turn_prompt" --turn "$turn_dir"

# Each mutation replaces the first occurrence of its needle; a mutation
# that changes nothing means the fixture and the needle drifted apart, so
# it refuses rather than testing a copy of the good prompt.
turn_prompt_source=$(cat "$good_turn_prompt")
write_turn_prompt_mutation() { # name, needle, replacement [, needle, replacement ...]
  local name=$1 mutated=$turn_prompt_source
  shift
  while (($#)); do
    mutated=${mutated/"$1"/"$2"}
    shift 2
  done
  [[ "$mutated" != "$turn_prompt_source" ]] \
    || { echo "turn-prompt mutation did not change fixture: $name" >&2; exit 1; }
  printf '%s\n' "$mutated" >"$turn_fixture/$name.md"
}
write_turn_prompt_mutation missing-header $'Model: fake-model\n' ''
write_turn_prompt_mutation turn-mismatch $'Turn-Id: turn-3\n' $'Turn-Id: turn-other\n'
write_turn_prompt_mutation mission-mismatch $'Mission-Id: fixture-mission\n' $'Mission-Id: other-mission\n'
write_turn_prompt_mutation altered-preamble \
  'You are the orchestrator for an unattended mission.' \
  'You are an orchestrator for an unattended mission.'
write_turn_prompt_mutation headings-out-of-order \
  '## Open Asks' '## TEMP' \
  '## Streams' '## Open Asks' \
  '## TEMP' '## Streams'
write_turn_prompt_mutation unfenced-data \
  $'## Open Asks\n<<<DATA>>>\nask-1\tstream-a\treserved-decision\tApprove the named API contract?\n<<<END>>>' \
  $'## Open Asks\nask-1\tstream-a\treserved-decision\tApprove the named API contract?'
write_turn_prompt_mutation malformed-record \
  $'ask-1\tstream-a\treserved-decision\tApprove the named API contract?' \
  $'ask-1\tstream-a\treserved-decision'

check_bad_turn_prompt() { # fixture name, failing check
  local name=$1 expected=$2 output status
  output="$turn_fixture/$name.out"
  set +e
  scripts/assert-turn-prompt.sh \
    --file "$turn_fixture/$name.md" --turn "$turn_dir" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "turn prompt checker accepted the negative $name fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "turn prompt checker used $status instead of exit 1 for $name" >&2; exit 1; }
  grep -Fq "[$expected]" "$output" \
    || { echo "turn prompt checker did not name the $expected check for $name" >&2; exit 1; }
}

check_bad_turn_prompt missing-header headers
check_bad_turn_prompt turn-mismatch identity
check_bad_turn_prompt mission-mismatch identity
check_bad_turn_prompt altered-preamble preamble
check_bad_turn_prompt headings-out-of-order headings
check_bad_turn_prompt unfenced-data fencing
check_bad_turn_prompt malformed-record records

set +e
scripts/assert-turn-prompt.sh >"$turn_fixture/usage.out" 2>&1
turn_prompt_usage_status=$?
set -e
[[ $turn_prompt_usage_status -eq 2 ]] \
  || { echo "turn prompt checker used $turn_prompt_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Critique closure joins the canonical return JSON against the one Markdown
# dispositions table. Reuse the item-2 return fixture shape and vary only the
# findings and table rows needed to exercise each join invariant.
critique_fixtures="$tmp/critiques"
mkdir -p "$critique_fixtures"
critique_material_finding='{"id":"F-1","severity":"high","material":true,"claim":"contract gap","evidence":"read design"}'
critique_nonmaterial_finding='{"id":"F-2","severity":"low","material":false,"claim":"wording issue","evidence":"read design"}'
critique_second_material_finding='{"id":"F-3","severity":"medium","material":true,"claim":"incorrect premise","evidence":"checked implementation"}'

# A critique return is the positive design-critic return with the given
# findings; the material count is derived from them, never restated.
write_critique_return() { # name, findings JSON array
  local name=$1 findings=$2 finding finding_material material_count=0
  while IFS= read -r finding; do
    [[ -n "$finding" ]] || continue
    finding_material=$("$engine" json get --value "$finding" --field material) \
      || { echo "critique fixture finding lacks a material flag: $finding" >&2; exit 1; }
    [[ "$finding_material" == true ]] || continue
    material_count=$((material_count + 1))
  done < <(json_elements "$findings")
  cp "$return_fixtures/design-critic-positive.json" "$critique_fixtures/$name.json"
  json_replace_field "$critique_fixtures/$name.json" findings "$findings"
  "$engine" json set --file "$critique_fixtures/$name.json" --int "verdictMaterialCount=$material_count"
}

write_critique_table() { # name, separator row, rows as "id<TAB>disposition<TAB>reasoning<TAB>amendment"
  local name=$1 separator=$2 row finding_id disposition reasoning amendment
  {
    echo '| Finding id | Disposition | Reasoning and evidence | Amendment |'
    echo "$separator"
    for row in "${@:3}"; do
      IFS=$'\t' read -r finding_id disposition reasoning amendment <<<"$row"
      printf '| %s | %s | %s | %s |\n' "$finding_id" "$disposition" "$reasoning" "$amendment"
    done
  } >"$critique_fixtures/$name.md"
}
critique_table_separator='| --- | --- | --- | --- |'

write_critique_return joinable \
  "[$critique_material_finding,$critique_nonmaterial_finding,$critique_second_material_finding]"
write_critique_table all-disposed "$critique_table_separator" \
  $'F-1\taccepted\tdesign amended\tsection 3' \
  $'F-2\tnoted\tdoes not change implementation\tnone' \
  $'F-3\trefuted\timplementation disproves the claim\tnone'
write_critique_table open-material "$critique_table_separator" \
  $'F-2\tnoted\tdoes not change implementation\tnone' \
  $'F-3\trefuted\timplementation disproves the claim\tnone'
write_critique_table noted-on-material "$critique_table_separator" \
  $'F-1\tnoted\tincorrect disposition\tnone' \
  $'F-2\tnoted\tdoes not change implementation\tnone' \
  $'F-3\trefuted\timplementation disproves the claim\tnone'
write_critique_table missing-nonmaterial-disposition "$critique_table_separator" \
  $'F-1\taccepted\tdesign amended\tsection 3' \
  $'F-3\trefuted\timplementation disproves the claim\tnone'
write_critique_table unknown-disposition "$critique_table_separator" \
  $'F-1\tdismissed\tnot a protocol value\tnone' \
  $'F-2\tnoted\tdoes not change implementation\tnone' \
  $'F-3\trefuted\timplementation disproves the claim\tnone'
write_critique_table unknown-finding-id "$critique_table_separator" \
  $'F-1\taccepted\tdesign amended\tsection 3' \
  $'F-2\tnoted\tdoes not change implementation\tnone' \
  $'F-3\trefuted\timplementation disproves the claim\tnone' \
  $'F-404\trefuted\tno matching finding\tnone'

write_critique_return duplicate-id \
  "[$critique_material_finding,$critique_material_finding,$critique_nonmaterial_finding,$critique_second_material_finding]"
write_critique_table duplicate-id "$critique_table_separator" \
  $'F-1\taccepted\tfirst row\tsection 3' \
  $'F-1\trefuted\tsecond row\tnone' \
  $'F-2\tnoted\tdoes not change implementation\tnone' \
  $'F-3\trefuted\timplementation disproves the claim\tnone'

cp "$return_fixtures/design-critic-positive.json" "$critique_fixtures/unjoinable-missing-findings.json"
json_remove_field "$critique_fixtures/unjoinable-missing-findings.json" findings
write_critique_table unjoinable-malformed-table '| --- | --- | --- |'

scripts/assert-critique-closed.sh \
  --findings "$critique_fixtures/joinable.json" \
  --dispositions "$critique_fixtures/all-disposed.md"

# Plan consistency: a rule stated in several places must not disagree with
# itself. Eight of nine rounds of one design critique found nothing else, and a
# paid round is the wrong instrument for drift a script finds instantly.
# This repository builds the metasystem and must run under it. Its own hooks were
# never installed: adopt.sh writes .claude/settings.json into adopted targets,
# and the template never adopts itself, so for the whole of development the
# session-start arming, the untracked-process report, the stale-supervisor
# warning and the open-work check were inert here. Everything was fixtured and
# nothing was live. This check is why that cannot recur silently.
# Template repository only. An adopted copy gets its hooks from adopt.sh at its
# own root, with a different layout; this is about the repository that builds the
# metasystem running under it.
# One derivation of template_mode (script-validate-12/D36): line ~272 owns it.
if (( template_mode )); then
  harness_own_settings=$(cd "$root/.." && pwd -P)/.claude/settings.json
  [[ -f "$harness_own_settings" ]] \
    || { echo "this repository has no .claude/settings.json: the metasystem is not running under itself" >&2; exit 1; }
  "$root/bin/metasystem" hooks check --runtime claude "$harness_own_settings" \
    "$root/scripts/enforcement/$("$root/bin/metasystem" runtime enforcement-config claude)"
  echo "metasystem runs under its own hooks"
fi

scripts/assert-plan-consistency.sh >"$tmp/plan-consistency.out"
grep -q 'retired term' "$tmp/plan-consistency.out" \
  || { echo "plan consistency check did not report its retired terms" >&2; exit 1; }

plan_fixture=$tmp/plan-consistency
mkdir -p "$plan_fixture"
cat >"$plan_fixture/owner.md" <<'FIXTURE'
RETIRED: widget check -- the gadget check
FIXTURE
cat >"$plan_fixture/clean.md" <<'FIXTURE'
The widget check was replaced by the gadget check.
FIXTURE
scripts/assert-plan-consistency.sh --plans-dir "$plan_fixture" >/dev/null \
  || { echo "plan consistency rejected a line that explains the change" >&2; exit 1; }
cat >"$plan_fixture/stale.md" <<'FIXTURE'
Test quality is measured by the widget check.
FIXTURE
set +e
scripts/assert-plan-consistency.sh --plans-dir "$plan_fixture" >"$tmp/plan-stale.out" 2>&1
plan_status=$?
set -e
(( plan_status == 1 )) \
  || { echo "plan consistency did not fail on a prescribed retired term" >&2; exit 1; }
grep -q "stale.md:1: prescribes 'widget check'" "$tmp/plan-stale.out" \
  || { echo "plan consistency did not name the file, line, and term" >&2; cat "$tmp/plan-stale.out" >&2; exit 1; }

check_open_critique() { # fixture name, findings file, dispositions file, diagnostics...
  local name=$1 findings_file=$2 dispositions_file=$3 output status expected
  shift 3
  output="$tmp/critique-$name.out"
  set +e
  scripts/assert-critique-closed.sh \
    --findings "$findings_file" \
    --dispositions "$dispositions_file" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "critique checker accepted the negative $name fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "critique checker used $status instead of exit 1 for $name" >&2; exit 1; }
  for expected in "$@"; do
    grep -Fq "$expected" "$output" || {
      echo "critique checker did not name the $name violation: $expected" >&2
      cat "$output" >&2
      exit 1
    }
  done
}

check_open_critique open-material \
  "$critique_fixtures/joinable.json" "$critique_fixtures/open-material.md" \
  "finding id 'F-1' has no disposition row"
check_open_critique noted-on-material \
  "$critique_fixtures/joinable.json" "$critique_fixtures/noted-on-material.md" \
  "material finding id 'F-1' cannot use disposition 'noted'"
check_open_critique missing-nonmaterial-disposition \
  "$critique_fixtures/joinable.json" "$critique_fixtures/missing-nonmaterial-disposition.md" \
  "finding id 'F-2' has no disposition row"
check_open_critique duplicate-id \
  "$critique_fixtures/duplicate-id.json" "$critique_fixtures/duplicate-id.md" \
  "duplicate finding id: 'F-1'" "duplicate disposition id: 'F-1'"
check_open_critique unknown-disposition \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unknown-disposition.md" \
  "disposition for finding id 'F-1' has unknown value 'dismissed'"
check_open_critique unknown-finding-id \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unknown-finding-id.md" \
  "disposition names unknown finding id: 'F-404'"
check_open_critique unjoinable-format-missing-findings \
  "$critique_fixtures/unjoinable-missing-findings.json" "$critique_fixtures/all-disposed.md" \
  '$.findings array is missing'
check_open_critique unjoinable-format-malformed-table \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unjoinable-malformed-table.md" \
  "malformed dispositions table: invalid separator row"

set +e
scripts/assert-critique-closed.sh >"$tmp/critique-usage.out" 2>&1
critique_usage_status=$?
set -e
[[ $critique_usage_status -eq 2 ]] \
  || { echo "critique checker used $critique_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Job mode derives the schema and return path from the record and then checks
# all four identity fields, one at a time, against a schema-valid return.
job_fixture="$tmp/job-metasystem"
mkdir -p "$job_fixture/scripts/agents" \
  "$job_fixture/artifacts/agents/jobs" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/1"
cp scripts/assert-return-complete.sh "$job_fixture/scripts/"
# The copied assert script resolves its engine as <fixture>/bin/metasystem;
# give the fixture checkout the real one (the python schema helper it used
# to copy is gone — the binary materializes schemas itself).
mkdir -p "$job_fixture/bin"
cp bin/metasystem "$job_fixture/bin/metasystem"
cp -R scripts/agents/schemas "$job_fixture/scripts/agents/"
cat >"$job_fixture/artifacts/agents/jobs/fixture-job.json" <<'EOF'
{
  "jobId": "fixture-job",
  "role": "implementer",
  "round": 1,
  "parentJob": null,
  "runtime": "fake",
  "sessionId": "session-1"
}
EOF
cp "$return_fixtures/implementer-positive.json" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/1/return.json"
(cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job)

mkdir -p "$job_fixture/artifacts/agents/fixture-job/rounds/2"
cat >"$job_fixture/artifacts/agents/jobs/fixture-job-r2.json" <<'EOF'
{
  "jobId": "fixture-job-r2",
  "role": "implementer",
  "round": 2,
  "parentJob": "fixture-job",
  "runtime": "fake",
  "sessionId": "session-2"
}
EOF
cp "$return_fixtures/implementer-positive.json" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/2/return.json"
"$engine" json set --file "$job_fixture/artifacts/agents/fixture-job/rounds/2/return.json" \
  --field jobId=fixture-job-r2 --int round=2 --field sessionId=session-2
(cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job-r2)

# One mismatched identity field per fixture, each derived from the same
# schema-valid positive return.
cp "$return_fixtures/implementer-positive.json" "$return_fixtures/identity-jobId.json"
"$engine" json set --file "$return_fixtures/identity-jobId.json" --field jobId=other-job
cp "$return_fixtures/implementer-positive.json" "$return_fixtures/identity-round.json"
"$engine" json set --file "$return_fixtures/identity-round.json" --int round=2
cp "$return_fixtures/implementer-positive.json" "$return_fixtures/identity-runtime.json"
"$engine" json set --file "$return_fixtures/identity-runtime.json" --field runtime=other-runtime
cp "$return_fixtures/implementer-positive.json" "$return_fixtures/identity-sessionId.json"
"$engine" json set --file "$return_fixtures/identity-sessionId.json" --field sessionId=other-session
for field in jobId round runtime sessionId; do
  cp "$return_fixtures/identity-$field.json" \
    "$job_fixture/artifacts/agents/fixture-job/rounds/1/return.json"
  set +e
  (cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job) >"$tmp/identity-$field.out" 2>&1
  identity_status=$?
  set -e
  if [[ $identity_status -eq 0 ]]; then
    echo "job-aware return checker accepted a mismatched $field" >&2
    exit 1
  fi
  [[ $identity_status -eq 1 ]] \
    || { echo "job-aware return checker used $identity_status instead of exit 1 for $field" >&2; exit 1; }
  grep -Fq "$.${field} identity mismatch" "$tmp/identity-$field.out" \
    || { echo "job-aware return checker did not name the $field mismatch" >&2; exit 1; }
done
fi

# Dispatcher and fake-adapter fixtures run in a minimal adopted-mode Git
# repository. Keeping their artifacts outside this checkout proves that the
# scripts derive every path from the repository they ship into. Nested
# adoption validations inherit the skip after this block, avoiding duplicate
# process-lifecycle runs while a direct adopted-repository validation still
# exercises the full contract.
if section_selected dispatcher-adapter-and-mission-runner-fixtures \
  && delegate_process_section "dispatcher, adapter selftest, and mission-runner process fixtures" \
  && ! delivery_contract_skip "dispatcher, adapter selftest, and mission-runner process fixtures"; then
  # Extracted to the sub-suite shape (script-validate-4/D35).
  bash scripts/agents/dispatch-fixtures.sh
fi

if section_selected workflow-tooling-fixtures; then
# The shipped Stop hook must stay rooted and surface via JSON output: hooks
# run in the session's cwd, receipt.sh resolves its ledger from there, and a
# non-blocking exit code shows only a first-line hook-error notice.
hooks_json=scripts/enforcement/claude-code-hooks.json
grep -Fq 'cd \"$CLAUDE_PROJECT_DIR\"' "$hooks_json" || { echo "stop hook is not rooted at CLAUDE_PROJECT_DIR" >&2; exit 1; }
grep -Fq 'systemMessage' "$hooks_json" || { echo "stop hook does not surface a systemMessage when a retro is due" >&2; exit 1; }
if grep -Fq '|| true' "$hooks_json"; then
  echo "stop hook masks the retro-due exit code with || true" >&2
  exit 1
fi
# The first Stop entry's first hook command, straight from the shipped
# hooks file; a missing level fails the extraction rather than testing
# an empty command.
stop_entries=$("$engine" json get --file "$hooks_json" --field hooks.Stop)
first_stop_entry=$(json_elements "$stop_entries" | head -n 1)
[[ -n "$first_stop_entry" ]] || { echo "the shipped hooks file has no Stop entry" >&2; exit 1; }
stop_hooks=$("$engine" json get --value "$first_stop_entry" --field hooks)
first_stop_hook=$(json_elements "$stop_hooks" | head -n 1)
[[ -n "$first_stop_hook" ]] || { echo "the shipped Stop entry carries no hooks" >&2; exit 1; }
hook_cmd=$("$engine" json get --value "$first_stop_hook" --field command)
[[ -n "$hook_cmd" ]] || { echo "the shipped Stop hook has no command" >&2; exit 1; }
hookrepo="$tmp/hookrepo"
mkdir -p "$hookrepo/scripts" "$hookrepo/plans" "$hookrepo/bin"
git -C "$hookrepo" init -q -b main
cp scripts/receipt.sh scripts/metasystem-config.sh "$hookrepo/scripts/"
cp bin/metasystem "$hookrepo/bin/metasystem"
cp metasystem.conf "$hookrepo/"
mkdir -p "$hookrepo/memory"
printf '1|1970-01-01T00:00:01Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|note=aged\n' >"$hookrepo/memory/receipts.log"
out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
grep -q systemMessage <<<"$out" || { echo "stop hook stayed silent on a due retro" >&2; exit 1; }
printf '%s|%s|RETRO|note=fixture\n' "$(date -u +%s)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$hookrepo/memory/receipts.log"
out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
[[ -z "$out" ]] || { echo "stop hook emitted output when no retro is due" >&2; exit 1; }
printf 'garbage\n' >"$hookrepo/memory/receipts.log"
out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
grep -q "errored" <<<"$out" || { echo "stop hook hid a failing receipt check" >&2; exit 1; }
if grep -q "retro due" <<<"$out"; then
  echo "stop hook misreported a check error as a due retro" >&2
  exit 1
fi
out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$tmp/definitely-missing" bash -c "$hook_cmd")
grep -q "project directory" <<<"$out" || { echo "stop hook stayed silent on an unresolvable project directory" >&2; exit 1; }

# The debug-java preflight is optional: absent in adopted repositories that
# excluded the skill, moved into skills/ in JVM repositories that enabled it.
for preflight in optional-skills/debug-java/scripts/preflight.sh skills/debug-java/scripts/preflight.sh; do
  if [[ -x "$preflight" ]]; then
    touch "$tmp/source" "$tmp/artifact"
    "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null
    touch -t 202001010000 "$tmp/artifact"
    if "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null 2>&1; then
      echo "debug preflight accepted a stale artifact" >&2
      exit 1
    fi
    break
  fi
done

cat >"$tmp/good.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | HIGH | Requirement | Behavior | `owner.py` | `owner.py` | `test_owner.py` | Not applicable: pure derivation | DONE | None |
EOF
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/good.md" >/dev/null

# Proof cells on critical/high rows must be concrete: a DONE row whose proof
# is vague prose must fail, or a declared status can outrun its evidence.
sed 's/| `test_owner.py` |/| covered somewhere |/' "$tmp/good.md" >"$tmp/vague.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/vague.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a DONE row with a vague proof cell" >&2
  exit 1
fi
# Keyword-carrying prose is still prose: promises of future proof, and owners
# without a code-shaped token, must fail.
cat >"$tmp/keyword.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | CRITICAL | Requirement | Behavior | someone will own this | we should test this later | needs testing | manual test pending | DONE | None |
EOF
if scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/keyword.md" >/dev/null 2>&1; then
  echo "obligation gate accepted keyword prose as proof and a prose owner" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable |/' "$tmp/good.md" >"$tmp/bare-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bare-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a bare Not applicable without a reason" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable: |/' "$tmp/good.md" >"$tmp/empty-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/empty-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted an empty-delimiter Not applicable" >&2
  exit 1
fi
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | pyproject.toml |/' "$tmp/good.md" >"$tmp/toml.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/toml.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked config-file proof path" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | module.mjs |/' "$tmp/good.md" >"$tmp/mjs.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/mjs.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked filename outside the old whitelist" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | compare e.g. the results |/' "$tmp/good.md" >"$tmp/eg.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/eg.md" >/dev/null 2>&1; then
  echo "obligation gate mistook abbreviation prose for a filename" >&2
  exit 1
fi
# Matrices shown inside fenced code blocks are documentation, not declarations.
{ printf '```markdown\n'; cat "$tmp/good.md"; printf '```\n'; } >"$tmp/fenced.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/fenced.md" >/dev/null 2>&1; then
  echo "obligation gate read a matrix out of a fenced code block" >&2
  exit 1
fi

sed 's/| DONE |/| MISSING |/' "$tmp/good.md" >"$tmp/bad.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bad.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a missing high obligation" >&2
  exit 1
fi

sed 's/| HIGH |/| MEDIUM |/; s/| DONE |/| PARTIAL |/' "$tmp/good.md" >"$tmp/medium.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/medium.md" >/dev/null || {
  echo "obligation gate rejected a valid medium-only matrix" >&2
  exit 1
}

scripts/assert-design-obligation-gate.sh --runtime-required --file docs/examples/design-obligation-matrix.md >/dev/null 2>&1 && {
  echo "example matrix with READY_FOR_RUNTIME passed --runtime-required; negative fixture broken" >&2
  exit 1
}
scripts/assert-design-obligation-gate.sh --file docs/examples/design-obligation-matrix.md >/dev/null

repo="$tmp/baseline-repo"
git init -q -b main "$repo"
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit --allow-empty -qm base
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null) || {
  echo "refactor baseline check blocked on the baseline file's own dirt right after record" >&2
  exit 1
}
git -C "$repo" add plans/refactor-baseline
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null)
echo dirty >"$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null 2>&1); then
  echo "refactor baseline check accepted a dirty worktree" >&2
  exit 1
fi
rm "$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check --max-commits 0 >/dev/null 2>&1); then
  echo "refactor baseline check ignored the commit-count backstop" >&2
  exit 1
fi
# Custom and absolute --file paths normalize to the repository root; paths
# outside the repository are rejected because git cannot see their dirt.
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file plans/custom-baseline >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file plans/custom-baseline >/dev/null) || {
  echo "refactor baseline check blocked a custom relative --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/custom-baseline
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm custom-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$repo/plans/abs-baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "$repo/plans/abs-baseline" >/dev/null) || {
  echo "refactor baseline check blocked an in-repository absolute --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/abs-baseline
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm abs-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/bäseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/bäseline" >/dev/null) || {
  echo "refactor baseline check blocked a non-ASCII --file right after record (quotePath)" >&2
  exit 1
}
git -C "$repo" add "plans/bäseline"
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm nonascii-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/my baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/my baseline" >/dev/null) || {
  echo "refactor baseline check blocked a space-containing --file right after record (C-quoting)" >&2
  exit 1
}
git -C "$repo" add "plans/my baseline"
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm space-baseline
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$tmp/outside-baseline" >/dev/null 2>&1); then
  echo "refactor baseline accepted a --file outside the repository" >&2
  exit 1
fi

(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --eval "declared eval" >/dev/null)
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79 >/dev/null 2>&1); then
  echo "frontier challenge accepted a score below the frontier" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 >/dev/null 2>&1); then
  echo "frontier challenge forgot the stored noise floor" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 --min-delta 0 >/dev/null)
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 82 >/dev/null)
git -C "$repo" add plans/frontier
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm frontier
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 75 --eval "declared eval" >/dev/null 2>&1); then
  echo "frontier record accepted a regression without --force" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=60\neval=declared\nartifact=\n' >"$tmp/frontier-old"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-old" >/dev/null 2>&1; then
  echo "frontier challenge compared against an expired frontier" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-nowindow"
scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-nowindow" >/dev/null
scripts/frontier.sh status --file "$tmp/frontier-nowindow" | grep -qx 'direction=max' || {
  echo "frontier status hid the effective direction of a legacy file" >&2
  exit 1
}
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=sideways\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-malformed"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-malformed" >/dev/null 2>&1; then
  echo "frontier challenge accepted a malformed persisted direction" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-emptydir"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-emptydir" >/dev/null 2>&1; then
  echo "frontier challenge accepted an empty persisted direction" >&2
  exit 1
fi

# Lower-is-better frontiers: persisted direction, force-gated changes, and a
# challenge that only ever uses the stored direction.
(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --direction min --eval "declared eval" --file plans/frontier-min >/dev/null)
git -C "$repo" add plans/frontier-min
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm frontier-min
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79.5 --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction challenge accepted a within-noise improvement" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null)
(cd "$repo" && METASYSTEM_FRONTIER_DIRECTION=max "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null) || {
  echo "challenge honored an environment direction instead of the persisted one" >&2
  exit 1
}
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 85 --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction record accepted a regression without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 99 --direction max --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier record accepted a direction change without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 1 --direction min --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier challenge accepted a direction flag" >&2
  exit 1
fi

scripts/assert-stop-loss.sh --file docs/examples/step-back-ledger.md >/dev/null
printf '### Cycle C1\n- Classification: no-progress\n### Cycle C2\n- Classification: no-progress\n' >"$tmp/stuck.md"
if scripts/assert-stop-loss.sh --file "$tmp/stuck.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed a third cycle after two no-progress results" >&2
  exit 1
fi
printf -- '- Cycle budget: 2\n### Cycle C1\n- Classification: contract-improved\n### Cycle C2\n- Classification: falsified-continue\n' >"$tmp/spent.md"
if scripts/assert-stop-loss.sh --file "$tmp/spent.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted cycle budget" >&2
  exit 1
fi
printf '### Cycle C1\n- Classification: falsified-dead-end\n' >"$tmp/deadend.md"
if scripts/assert-stop-loss.sh --file "$tmp/deadend.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed cycles after a dead end" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: falsified-continue\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted no-gain budget over a mixed trailing sequence" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: contract-improved\n### Cycle E3\n- Classification: unresolved\n### Cycle E4\n- Classification: falsified-continue\n' >"$tmp/nogain-reset.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-reset.md" >/dev/null || {
  echo "stop-loss check failed to reset the no-gain count on a contract-improved cycle" >&2
  exit 1
}
printf '### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: unresolved\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain-optout.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-optout.md" >/dev/null || {
  echo "stop-loss check blocked unresolved cycles without a declared no-gain budget" >&2
  exit 1
}
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-unclassified.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-unclassified.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count let an unclassified cycle vanish from the tail" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: not-contract-improved\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-fakegain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-fakegain.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count reset on a classification merely containing contract-improved" >&2
  exit 1
fi

knob_fixture="$tmp/conf-consuming-scripts"
mkdir -p "$knob_fixture/receipt/scripts" "$knob_fixture/watch/scripts" "$knob_fixture/watch/jobs" \
  "$knob_fixture/receipt/bin" "$knob_fixture/watch/bin"
cp scripts/receipt.sh scripts/metasystem-config.sh "$knob_fixture/receipt/scripts/"
cp bin/metasystem "$knob_fixture/receipt/bin/metasystem"
printf 'retro.max-receipts=0\nretro.max-age-days=30\n' >"$knob_fixture/receipt/metasystem.conf"
"$knob_fixture/receipt/scripts/receipt.sh" add --type implement --outcome shipped --file "$knob_fixture/receipt/receipts.log" >/dev/null
if "$knob_fixture/receipt/scripts/receipt.sh" check --file "$knob_fixture/receipt/receipts.log" >/dev/null 2>&1; then
  echo "receipt ignored the metasystem.conf receipt limit" >&2
  exit 1
fi
METASYSTEM_RETRO_MAX_RECEIPTS=2 "$knob_fixture/receipt/scripts/receipt.sh" check --file "$knob_fixture/receipt/receipts.log" >/dev/null \
  || { echo "receipt did not prefer the environment over metasystem.conf" >&2; exit 1; }
METASYSTEM_RETRO_MAX_RECEIPTS=0 "$knob_fixture/receipt/scripts/receipt.sh" check --max-receipts 2 --file "$knob_fixture/receipt/receipts.log" >/dev/null \
  || { echo "receipt did not prefer the flag over the environment" >&2; exit 1; }

cp scripts/watch-background-jobs.sh scripts/metasystem-config.sh "$knob_fixture/watch/scripts/"
cp bin/metasystem "$knob_fixture/watch/bin/metasystem"
printf 'watch.stale-min=7\nwatch.cap-min=%s\n' "$fixture_watcher_config_cap_min" >"$knob_fixture/watch/metasystem.conf"
touch "$knob_fixture/watch/state"
"$knob_fixture/watch/scripts/watch-background-jobs.sh" --dir "$knob_fixture/watch/jobs" --state "$knob_fixture/watch/state" --once >"$knob_fixture/watch.out"
grep -q "stale=7m cap=${fixture_watcher_config_cap_min}m" "$knob_fixture/watch.out" \
  || { echo "watcher ignored metasystem.conf ceilings" >&2; exit 1; }

refactor_knob="$knob_fixture/refactor"
mkdir -p "$refactor_knob/scripts" "$refactor_knob/bin"
cp scripts/refactor-baseline.sh scripts/metasystem-config.sh "$refactor_knob/scripts/"
cp bin/metasystem "$refactor_knob/bin/metasystem"
# The baseline recorder demands a clean worktree; the engine is a build
# artifact there exactly as in production.
printf 'bin/\n' >"$refactor_knob/.gitignore"
printf 'refactor.max-age-minutes=1440\nrefactor.max-commits=0\n' >"$refactor_knob/metasystem.conf"
git init -q -b main "$refactor_knob"
printf 'fixture\n' >"$refactor_knob/source.txt"
git -C "$refactor_knob" add source.txt metasystem.conf scripts .gitignore
git -C "$refactor_knob" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm initial
(cd "$refactor_knob" && scripts/refactor-baseline.sh record --gate fixture >/dev/null)
git -C "$refactor_knob" add plans/refactor-baseline
git -C "$refactor_knob" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm baseline
if (cd "$refactor_knob" && scripts/refactor-baseline.sh check >/dev/null 2>&1); then
  echo "refactor baseline ignored metasystem.conf commit cadence" >&2
  exit 1
fi
(cd "$refactor_knob" && scripts/refactor-baseline.sh check --max-commits 2 >/dev/null) \
  || { echo "refactor baseline did not prefer the cadence flag" >&2; exit 1; }

rfile="$tmp/receipts.log"
scripts/receipt.sh add --type implement --outcome shipped \
  --delegate codex:fixture-code:implementer-job \
  --delegate claude:fixture-review:code-critic-job --file "$rfile" >/dev/null
grep -q '|delegate=codex:fixture-code:implementer-job,claude:fixture-review:code-critic-job|' "$rfile" \
  || { echo "receipt did not join repeated delegate triples" >&2; exit 1; }
scripts/receipt.sh check --file "$rfile" >/dev/null
scripts/receipt.sh add --type review --outcome reworked --corrections 1 --file "$rfile" >/dev/null
if scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null 2>&1; then
  echo "receipt check ignored the receipt-count backstop" >&2
  exit 1
fi
scripts/receipt.sh retro "fixture retro" --file "$rfile" >/dev/null
scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null
if scripts/receipt.sh add --type bogus --outcome shipped --file "$rfile" >/dev/null 2>&1; then
  echo "receipt add accepted an invalid type" >&2
  exit 1
fi
printf '1|1970-01-01T00:00:01Z|RETRO|note=aged\n' >"$tmp/receipts-aged.log"
scripts/receipt.sh check --max-age-days 0 --file "$tmp/receipts-aged.log" >/dev/null || {
  echo "receipt check demanded a retro over an empty period" >&2
  exit 1
}
scripts/receipt.sh add --type improve --outcome shipped --verify caught --file "$rfile" >/dev/null
# The receipt-stats intermittent (records/misc/known-issue-receipt-stats-flake.md):
# the ledger is byte-perfect in every preserved failure yet a grep misses
# roughly every other Mac suite run. This probe captures the failing
# invocation itself — output, exit code, ledger bytes, environment — the
# evidence the dossier's leading suspicion (a transient shim/exec failure
# converted into a grep miss) needs. The if-guard keeps errexit from
# killing the run before the capture lands.
receipt_probe_dir="${TMPDIR:-/tmp}/receipt-evidence"; mkdir -p "$receipt_probe_dir"
receipt_stats_probe() { # label, expected pattern, ledger file, extra stats args...
  local label=$1 expected=$2 file=$3 out rc=0
  local stats_sh="${receipt_stats_sh:-scripts/receipt.sh}"
  shift 3
  if out=$("$stats_sh" stats "$@" --file "$file"); then rc=0; else rc=$?; fi
  if printf '%s\n' "$out" | grep -q "$expected"; then return 0; fi
  {
    echo "FAILURE $label rc=$rc at $(date -u +%Y%m%dT%H%M%SZ)"
    echo "--- stats output:"; printf '%s\n' "$out"
    echo "--- ledger bytes:"; cat "$file"
    echo "--- ledger stat:"; ls -la "$file"
    echo "--- env:"; env | grep -i METASYSTEM || true
    echo "--- binary:"; ls -la bin/metasystem 2>/dev/null || true; shasum bin/metasystem 2>/dev/null || true
  } >"$receipt_probe_dir/fail-$(date +%s)-$label.txt"
  echo "receipt stats probe captured $label into $receipt_probe_dir" >&2
  return 1
}
receipt_stats_probe receipts-1 '^receipts=1$' "$rfile" || { echo "receipt stats miscounted the post-retro period" >&2; exit 1; }
receipt_stats_probe type-improve '^type_improve=1$' "$rfile" || { echo "receipt stats missed the improve type" >&2; exit 1; }
receipt_stats_probe all-receipts-3 '^receipts=3$' "$rfile" --all || { echo "receipt stats --all miscounted" >&2; exit 1; }

receipt_relation="$tmp/receipt-relation"
mkdir -p "$receipt_relation/scripts" "$receipt_relation/artifacts/agents/jobs" "$receipt_relation/bin"
cp scripts/receipt.sh scripts/metasystem-config.sh "$receipt_relation/scripts/"
cp bin/metasystem "$receipt_relation/bin/metasystem"
printf 'retro.max-receipts=25\nretro.max-age-days=30\n' >"$receipt_relation/metasystem.conf"
printf '{"jobId":"fixture-implementer","role":"implementer","parentJob":null}\n' \
  >"$receipt_relation/artifacts/agents/jobs/fixture-implementer.json"
printf '{"jobId":"fixture-critic","role":"code-critic","parentJob":null,"reviews":"fixture-implementer"}\n' \
  >"$receipt_relation/artifacts/agents/jobs/fixture-critic.json"
printf '{"jobId":"unrelated-critic","role":"code-critic","parentJob":null,"reviews":"another-implementer"}\n' \
  >"$receipt_relation/artifacts/agents/jobs/unrelated-critic.json"
printf '{"jobId":"waived-implementer","role":"implementer","parentJob":null,"critiqueWaived":{"class":"prose-under-30"}}\n' \
  >"$receipt_relation/artifacts/agents/jobs/waived-implementer.json"
relation_log="$receipt_relation/receipts.log"
if "$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
    --skills code-critique --delegate fake:model:fixture-implementer \
    --file "$relation_log" >"$receipt_relation/missing-chain.out" 2>&1; then
  echo "receipt accepted code-critique without a related critic chain" >&2
  exit 1
fi
grep -Fq 'code-critic chain id and the implementer job id' "$receipt_relation/missing-chain.out" \
  || { echo "receipt refusal did not name the missing relation" >&2; exit 1; }
if "$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
    --skills code-critique --delegate fake:model:fixture-implementer \
    --delegate fake:model:unrelated-critic --file "$relation_log" >/dev/null 2>&1; then
  echo "receipt accepted an unrelated critic chain" >&2
  exit 1
fi
"$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
  --skills code-critique --delegate fake:model:fixture-implementer \
  --delegate fake:model:fixture-critic --file "$relation_log" >/dev/null
mkdir -p "$receipt_relation/artifacts/agents/waived-implementer"
printf 'Working Mode: implement\nMission Stream: waiver-stream\n' \
  >"$receipt_relation/artifacts/agents/waived-implementer/brief.md"
"$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
  --delegate fake:model:waived-implementer --file "$relation_log" >/dev/null
grep -Fq '|critique_waived=prose-under-30|waiver_stream=waiver-stream|' "$relation_log" \
  || { echo "receipt did not surface the accepted waiver and its stream" >&2; exit 1; }
# Probed like the three stats greps above: the 2026-08-14 flake landed on
# this previously unprobed grep inside a nested adopted-copy validation.
receipt_stats_sh="$receipt_relation/scripts/receipt.sh"
receipt_stats_probe critique-waivers '^critique_waivers=1$' "$relation_log" \
  || { echo "receipt stats did not count the stream waiver for retro" >&2; exit 1; }
receipt_stats_sh=""

correction_log="$tmp/receipt-correction.log"
scripts/receipt.sh add --type implement --outcome shipped --skills none \
  --file "$correction_log" >/dev/null
original_line=$(sed -n '1p' "$correction_log")
original_epoch=${original_line%%|*}
original_sha1=$(printf '%s' "$original_line" | shasum -a 1 | awk '{print $1}')
scripts/receipt.sh correct --ref-epoch "$original_epoch" --ref-sha1 "$original_sha1" \
  --field skills --was none --now review --reason 'fixture correction' \
  --file "$correction_log" >/dev/null
[[ "$(sed -n '1p' "$correction_log")" == "$original_line" ]] \
  || { echo "receipt correction edited the original line" >&2; exit 1; }
[[ $(wc -l <"$correction_log" | tr -d ' ') == 2 ]] \
  || { echo "receipt correction did not append exactly one line" >&2; exit 1; }
grep -Fq "|CORRECTION|ref_epoch=$original_epoch|ref_sha1=$original_sha1|field=skills|was=none|now=review|reason=fixture correction" "$correction_log" \
  || { echo "receipt correction line lost its unique reference or change fields" >&2; exit 1; }
correction_line=$(sed -n '2p' "$correction_log")
correction_epoch=${correction_line%%|*}
correction_sha1=$(printf '%s' "$correction_line" | shasum -a 1 | awk '{print $1}')
if scripts/receipt.sh correct --ref-epoch "$correction_epoch" --ref-sha1 "$correction_sha1" \
    --field reason --was 'fixture correction' --now invalid --reason 'must not correct a correction' \
    --file "$correction_log" >"$tmp/correct-correction.out" 2>&1; then
  echo "receipt correction accepted an earlier CORRECTION line as its original" >&2
  exit 1
fi
grep -Fq 'must identify an original RECEIPT line' "$tmp/correct-correction.out" \
  || { echo "receipt correction rejected a non-receipt without naming the contract" >&2; exit 1; }

# Every free-text field is sanitized by one shared path: CRLF through the
# note, the skills list, delegates, and the retro summary must each stay one log line.
crlf_fixture=$(printf 'a\r\nb')
rfile_crlf="$tmp/receipts-crlf.log"
scripts/receipt.sh add --type implement --outcome shipped --note "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 1 ]] || { echo "a CRLF note corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --skills "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 2 ]] || { echo "a CRLF skills list corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --delegate "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 3 ]] || { echo "a CRLF delegate corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh retro "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 4 ]] || { echo "a CRLF retro summary corrupted the receipt log" >&2; exit 1; }
if LC_ALL=C grep -q $'\r' "$rfile_crlf"; then
  echo "receipt sanitizer left a carriage return in the log" >&2
  exit 1
fi
fi

# adopt.sh self-test: extracted to its own sub-suite (script-validate-4/D35).
if section_selected adoption-fixtures && (( template_mode )); then
  bash scripts/adopt-fixtures.sh
fi
if section_selected gate-run-freeze-fixtures && (( template_mode )); then
  bash scripts/agents/gate-run-freeze-fixtures.sh
fi
if section_selected witness-gate-fixtures && (( template_mode )); then
  bash scripts/agents/witness-gate-fixtures.sh
fi
if section_selected suite-progress-fixtures && (( template_mode )); then
  bash scripts/agents/suite-progress-fixtures.sh
fi
if section_selected land-fixtures && (( template_mode )); then
  bash scripts/agents/land-fixtures.sh
fi

if section_selected watch-background-jobs-fixtures; then
# watch-background-jobs: all four reportable states plus baseline suppression.
# The state file is pre-created because a MISSING state file auto-baselines on
# first run (the 2026-08-03 hardening); an existing empty state means armed.
# Backdate a file's timestamps so stale and cap windows are separable;
# touch -t takes a local-time stamp, so the arithmetic stays in local time.
age_file() { # path, seconds into the past
  local stamp
  stamp=$(date -v -"$2"S +%Y%m%d%H%M.%S 2>/dev/null) \
    || stamp=$(date -d "-$2 seconds" +%Y%m%d%H%M.%S) \
    || { echo "age_file: this host's date cannot compute a past stamp" >&2; return 1; }
  touch -t "$stamp" "$1"
}
wbj="$tmp/wbj"; mkdir -p "$wbj/jobs"
printf '{"status":"completed"}' >"$wbj/jobs/done.json"
printf '{"status":"running"}'   >"$wbj/jobs/live.json"
touch "$wbj/s1" "$wbj/s2" "$wbj/s3" "$wbj/s3b" "$wbj/s3c" "$wbj/s6" "$wbj/s7" "$wbj/s8"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s1" --once >"$wbj/o1" 2>&1
grep -q "^DONE done status=completed" "$wbj/o1" || {
  echo "watch-background-jobs: terminal job not reported" >&2; exit 1; }
grep -q "live" "$wbj/o1" && {
  echo "watch-background-jobs: running job reported as terminal" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s1" --once >"$wbj/o2" 2>&1
grep -v "^ARMED " "$wbj/o2" | grep -q . && {
  echo "watch-background-jobs: re-reported an already-reported job" >&2; exit 1; }
# age the record by a controlled 10 minutes so stale and cap are separable
age_file "$wbj/jobs/live.json" 600
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s2" --stale-min 5 --cap-min "$fixture_watcher_nonfiring_cap_min" --once >"$wbj/o3" 2>&1
grep -q "^STALE live" "$wbj/o3" || {
  echo "watch-background-jobs: stale job not reported" >&2; exit 1; }
grep -q "^CAPPED live" "$wbj/o3" && {
  echo "watch-background-jobs: hard cap fired inside its own window" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s3" --stale-min 5 --cap-min "$fixture_watcher_firing_cap_min" --once >"$wbj/o4" 2>&1
grep -q "^CAPPED live" "$wbj/o4" || {
  echo "watch-background-jobs: hard cap not reported" >&2; exit 1; }
# A non-JSON record keeps the header's mtime-only contract (script-misc-4):
# never NEVER-STARTED from its empty status — a plain-text record aged past
# start-verify but inside the stale window reports NOTHING, and past the
# stale window it reports STALE, so the id is not marked seen prematurely.
mkdir -p "$wbj/plain/jobs"
printf 'plain text progress notes\n' >"$wbj/plain/jobs/notes.json"
age_file "$wbj/plain/jobs/notes.json" 360
touch "$wbj/s9" "$wbj/s10"
scripts/watch-background-jobs.sh --dir "$wbj/plain/jobs" --state "$wbj/s9" --start-verify-min 5 --stale-min 20 --once >"$wbj/o9" 2>&1
grep -q "NEVER-STARTED" "$wbj/o9" && {
  echo "watch-background-jobs: non-JSON record got a status-based NEVER-STARTED" >&2; exit 1; }
age_file "$wbj/plain/jobs/notes.json" 1500
scripts/watch-background-jobs.sh --dir "$wbj/plain/jobs" --state "$wbj/s10" --start-verify-min 5 --stale-min 20 --once >"$wbj/o10" 2>&1
grep -q "^STALE notes" "$wbj/o10" || {
  echo "watch-background-jobs: quiet non-JSON record did not report STALE" >&2; exit 1; }

# A job whose RECORD is old but whose sibling log is fresh is WORKING, not stale.
# Runners write the record once at dispatch and stream progress to the log, so a
# record-only liveness check cries wolf on every long phase.
mkdir -p "$wbj/live-log/jobs"
printf '{"status":"running"}' >"$wbj/live-log/jobs/busy.json"
printf 'building\n' >"$wbj/live-log/jobs/busy.log"
age_file "$wbj/live-log/jobs/busy.json" 3600
scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3b" --stale-min 5 --once >"$wbj/o4b" 2>&1
grep -q "^STALE busy" "$wbj/o4b" && {
  echo "watch-background-jobs: reported STALE for a job whose log is advancing" >&2; exit 1; }
# ...but when BOTH files go quiet it is genuinely stale and must still report.
age_file "$wbj/live-log/jobs/busy.log" 3600
scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3c" --stale-min 5 --once >"$wbj/o4c" 2>&1
grep -q "^STALE busy" "$wbj/o4c" || {
  echo "watch-background-jobs: missed a genuinely stale job (all files quiet)" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s4" --baseline >/dev/null 2>&1
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s4" --once >"$wbj/o5" 2>&1
grep -q "^DONE done" "$wbj/o5" && {
  echo "watch-background-jobs: baseline did not suppress pre-existing jobs" >&2; exit 1; }
if scripts/watch-background-jobs.sh --state "$wbj/s5" --once >/dev/null 2>&1; then
  echo "watch-background-jobs: accepted a call with no --dir" >&2; exit 1
fi
# sidecar records must not double-report or bypass scope
printf '{"status":"completed","workspaceRoot":"/r/other"}' >"$wbj/jobs/side.json"
printf 'log text, not json\n'                             >"$wbj/jobs/side.log"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s7" --once >"$wbj/o7" 2>&1
grep -q "side" "$wbj/o7" && {
  echo "watch-background-jobs: sidecar record bypassed the scope filter" >&2; exit 1; }
printf '{"status":"completed","workspaceRoot":"/r/mine"}' >"$wbj/jobs/dual.json"
printf 'log text, not json\n'                            >"$wbj/jobs/dual.log"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s8" --once >"$wbj/o8" 2>&1
[ "$(grep -c '^DONE dual' "$wbj/o8")" -eq 1 ] || {
  echo "watch-background-jobs: job with a sidecar did not report exactly once" >&2; exit 1; }
printf '{"jobId":"chain-r2","parentJob":"chain","round":2,"status":"completed","workspaceRoot":"/r/mine"}' >"$wbj/jobs/chain-r2.json"
touch "$wbj/s8b"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s8b" --once >"$wbj/o8b" 2>&1
grep -q '^DONE chain-r2 status=completed' "$wbj/o8b" || {
  echo "watch-background-jobs: follow-up child was not tracked under its own id" >&2; exit 1; }
# scope: own repo and its worktrees in, peer repo and prefix-collision out
printf '{"status":"completed","workspaceRoot":"/r/mine"}'                >"$wbj/jobs/sc-mine.json"
printf '{"status":"completed","workspaceRoot":"/r/mine/.worktrees/w"}'   >"$wbj/jobs/sc-wt.json"
printf '{"status":"completed","workspaceRoot":"/r/other"}'               >"$wbj/jobs/sc-peer.json"
printf '{"status":"completed","workspaceRoot":"/r/mine-other"}'          >"$wbj/jobs/sc-prefix.json"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s6" --once >"$wbj/o6" 2>&1
grep -q "^DONE sc-mine" "$wbj/o6" || {
  echo "watch-background-jobs: in-scope job not reported" >&2; exit 1; }
grep -q "^DONE sc-wt" "$wbj/o6" || {
  echo "watch-background-jobs: worktree job dropped by scope filter" >&2; exit 1; }
grep -q "sc-peer" "$wbj/o6" && {
  echo "watch-background-jobs: peer repository job reported" >&2; exit 1; }
grep -q "sc-prefix" "$wbj/o6" && {
  echo "watch-background-jobs: scope matched on a path prefix" >&2; exit 1; }
# distinct scopes must not share default state. Auto-baseline swallows the
# first pass per fresh default state, so warm each scope, then prove each
# reports a job that arrives after its own arming.
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine  --once >/dev/null 2>&1
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/other --once >/dev/null 2>&1
printf '{"status":"completed","workspaceRoot":"/r/mine"}'  >"$wbj/jobs/nu-mine.json"
printf '{"status":"completed","workspaceRoot":"/r/other"}' >"$wbj/jobs/nu-other.json"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine  --once 2>/dev/null | grep -q "^DONE nu-mine" || {
  echo "watch-background-jobs: post-arming job not reported under its scope's default state" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/other --once 2>/dev/null | grep -q "^DONE nu-other" || {
  echo "watch-background-jobs: distinct scopes shared a default state file" >&2; exit 1; }
fi

if [[ -n "$enumeration_section" ]]; then
  :
elif (( delivery_contract )); then
  echo "metasystem delivery contract validated"
  if (( delivery_reuse )); then
    echo "validation families skipped behind PAYLOAD and toolchain equality under the behavior-surface policy:"
    printf -- '- %s\n' ${delivery_skipped[@]+"${delivery_skipped[@]}"}
  else
    echo "no validation family was skipped because PAYLOAD and toolchain equality was not proven"
  fi
elif (( delegate_scope )); then
  [[ ${#delegate_skipped_sections[@]} -eq ${#delegate_owed_sections[@]} ]] \
    || { echo "delegate-scope skipped-section accounting drifted" >&2; exit 1; }
  for index in "${!delegate_owed_sections[@]}"; do
    [[ "${delegate_skipped_sections[$index]}" == "${delegate_owed_sections[$index]}" ]] \
      || { echo "delegate-scope skipped-section accounting drifted" >&2; exit 1; }
  done
  echo "delegate-scope validation passed"
  echo "orchestrator still owes these process-visibility sections:"
  printf -- '- %s\n' "${delegate_skipped_sections[@]}"
else
  # The isolated milestone controller owns checkpoint consumption against the
  # real checkout. A direct validation proves this tree but has no transaction
  # identity and therefore resets nothing.
  echo "metasystem validation passed"
fi
