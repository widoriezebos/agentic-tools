#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
token=$root/artifacts/agents/mains/worktree-commit-token.json

# Landing authority is fenced before lease re-entry so a declared brain gets
# the one required refusal regardless of its current checkout-lease posture.
landing_requested=0
for argument in "$@"; do
  [[ "$argument" != -- ]] || break
  case "$argument" in
    --chain|--direct-fix|--revert-of|--root-job|--test-receipt|--recertification|--carried) landing_requested=1 ;;
  esac
done
if (( landing_requested )); then
  set +e
  brain_fence=$("$ms" brain fence --root "$root" --act land)
  brain_fence_rc=$?
  set -e
  if [[ $brain_fence_rc -eq 2 ]]; then
    "$ms" json get --value "$brain_fence" --field detail >&2
    exit 2
  elif [[ $brain_fence_rc -ne 0 ]]; then
    echo "land refused: brain fence failed" >&2
    exit 1
  fi
fi

if [[ ${1:-} != __lease-held ]]; then
  result=$("$ms" lease require-holder --root "$root" --caller-pid "$$") || exit $?
  # --default "" collapses an absent or null claimEpoch to empty, so the
  # human-commit branch below is taken when there is no epoch.
  epoch=$("$ms" json get --value "$result" --field claimEpoch --default "")
  if [[ -n "$epoch" ]]; then
    exec "$ms" lease run-held --root "$root" --caller-pid "$$" \
      --expected-epoch "$epoch" -- "$0" __lease-held "$epoch" "$@"
  fi
  exec "$ms" lease run-held --root "$root" --caller-pid "$$" -- "$0" __lease-held human "$@"
fi
shift
expected_epoch=${1:-}
[[ -n "$expected_epoch" ]] || exit 2
shift
agent_commit=0
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
  agent_commit=1
  "$ms" lease require-holder --root "$root" --caller-pid "$$" \
    --expected-epoch "$expected_epoch" >/dev/null
else
  [[ "$expected_epoch" == human ]] || exit 2
  "$ms" lease require-holder --root "$root" --caller-pid "$$" >/dev/null
fi

push_after=0
if [[ ${1:-} == --push ]]; then
  push_after=1
  shift
fi

ratchet=
landing_chain=
landing_direct_fix=
landing_revert_of=
landing_goal=
landing_goal_set=0
landing_root_job=
landing_test_receipt=
landing_recertification=
landing_carried=
landing_ledger_tip=
landing_carried_by=
landing_carried_past=
commit_args=()
while (( $# )); do
  case "$1" in
    --ratchet)
      [[ $# -ge 2 && -z "$ratchet" ]] || {
        echo "commit refused: --ratchet requires one path" >&2
        exit 2
      }
      ratchet=$2
      shift 2
      ;;
    --chain)
      [[ $# -ge 2 && -z "$landing_chain" ]] || {
        echo "commit refused: --chain requires one chain root" >&2
        exit 2
      }
      landing_chain=$2
      shift 2
      ;;
    --direct-fix)
      [[ $# -ge 2 && -z "$landing_direct_fix" ]] || {
        echo "commit refused: --direct-fix requires one class" >&2
        exit 2
      }
      landing_direct_fix=$2
      shift 2
      ;;
    --revert-of)
      [[ $# -ge 2 && -z "$landing_revert_of" ]] || {
        echo "commit refused: --revert-of requires one commit" >&2
        exit 2
      }
      landing_revert_of=$2
      shift 2
      ;;
    --goal)
      [[ $# -ge 2 && $landing_goal_set -eq 0 ]] || {
        echo "commit refused: --goal requires one goal item" >&2
        exit 2
      }
      landing_goal=$2
      landing_goal_set=1
      shift 2
      ;;
    --root-job)
      [[ $# -ge 2 && -z "$landing_root_job" ]] || {
        echo "commit refused: --root-job requires one root implementer job" >&2
        exit 2
      }
      landing_root_job=$2
      shift 2
      ;;
    --test-receipt)
      [[ $# -ge 2 && -z "$landing_test_receipt" ]] || {
        echo "commit refused: --test-receipt requires one receipt path" >&2
        exit 2
      }
      landing_test_receipt=$2
      shift 2
      ;;
    --recertification)
      [[ $# -ge 2 && -z "$landing_recertification" ]] || {
        echo "commit refused: --recertification requires one canonical record path" >&2
        exit 2
      }
      landing_recertification=$2
      shift 2
      ;;
    --carried)
      [[ $# -ge 2 && -z "$landing_carried" ]] || { echo "commit refused: --carried requires one opid" >&2; exit 2; }
      landing_carried=$2
      shift 2
      ;;
    --ledger-tip)
      [[ $# -ge 2 && -z "$landing_ledger_tip" ]] || { echo "commit refused: --ledger-tip requires one commit" >&2; exit 2; }
      landing_ledger_tip=$2
      shift 2
      ;;
    --carried-by)
      [[ $# -ge 2 && -z "$landing_carried_by" ]] || { echo "commit refused: --carried-by requires one recorded actor" >&2; exit 2; }
      landing_carried_by=$2
      shift 2
      ;;
    --carried-past)
      [[ $# -ge 2 && -z "$landing_carried_past" ]] || { echo "commit refused: --carried-past requires one name" >&2; exit 2; }
      landing_carried_past=$2
      shift 2
      ;;
    --)
      commit_args+=("$1")
      shift
      while (( $# )); do
        commit_args+=("$1")
        shift
      done
      ;;
    *)
      commit_args+=("$1")
      shift
      ;;
  esac
done

if [[ -n "$landing_carried" && ( $landing_goal_set -eq 0 || -z "$landing_ledger_tip" || -z "$landing_carried_by" || -z "$landing_carried_past" ) ]]; then
  echo "commit refused: --carried requires --goal, --ledger-tip, --carried-by, and --carried-past" >&2
  exit 2
fi

if (( landing_goal_set )) && [[ -z "$landing_goal" || ${#landing_goal} -gt 100 || ! "$landing_goal" =~ ^[a-z0-9-]+$ ]]; then
  echo "commit refused: --goal must be a lowercase kebab identifier of at most 100 characters" >&2
  exit 2
fi

message_has_goal_item() { # message text
  LC_ALL=C grep -Eiq '^Goal-Item:' <<<"$1"
}

message_has_carried_item() { # message text
  LC_ALL=C grep -Eq '^(Carry|Carried-By|Carried-Tree|Carried-Past|Carried-Battery|Carried-Judge|Carried-Ledger):' <<<"$1"
}

scan_commit_message_inputs() {
  local index=0 arg value
  while (( index < ${#commit_args[@]} )); do
    arg=${commit_args[index]}
    case "$arg" in
      -m|--message|--trailer|-F|--file|-c|-C|--reuse-message|--reedit-message|-t|--template|--squash|--fixup)
        (( index + 1 < ${#commit_args[@]} )) || {
          echo "commit refused: $arg requires a value" >&2
          return 2
        }
        value=${commit_args[index+1]}
        case "$arg" in
          -m|--message|--trailer)
		    if message_has_carried_item "$value"; then
		      echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		      return 1
		    fi
		    if message_has_goal_item "$value"; then
		      echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		      return 2
		    fi
            ;;
          -F|--file)
            if [[ "$value" == - ]]; then
              echo "commit refused: $arg - is an unscannable commit message source" >&2
              return 2
            fi
            if [[ ! -f "$value" || ! -r "$value" ]]; then
              echo "commit refused: commit message file is not readable: $value" >&2
              return 2
            fi
		    if message_has_carried_item "$(<"$value")"; then
		      echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		      return 1
		    fi
		    if message_has_goal_item "$(<"$value")"; then
		      echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		      return 2
		    fi
            ;;
          *)
            echo "commit refused: $arg is an unscannable commit message source" >&2
            return 2
            ;;
        esac
        index=$((index + 2))
        ;;
      --message=*|--trailer=*)
        value=${arg#*=}
		if message_has_carried_item "$value"; then
		  echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		  return 1
		fi
		if message_has_goal_item "$value"; then
		  echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		  return 2
		fi
        index=$((index + 1))
        ;;
      --file=*)
        value=${arg#*=}
        if [[ "$value" == - ]]; then
          echo "commit refused: --file=- is an unscannable commit message source" >&2
          return 2
        fi
        if [[ ! -f "$value" || ! -r "$value" ]]; then
          echo "commit refused: commit message file is not readable: $value" >&2
          return 2
        fi
		if message_has_carried_item "$(<"$value")"; then
		  echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		  return 1
		fi
		if message_has_goal_item "$(<"$value")"; then
		  echo "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed" >&2
		  return 2
		fi
        index=$((index + 1))
        ;;
      --reuse-message=*|--reedit-message=*|--template=*|--squash=*|--fixup=*|--amend)
        echo "commit refused: $arg is an unscannable commit message source" >&2
        return 2
        ;;
      *)
        index=$((index + 1))
        ;;
    esac
  done
}

scan_commit_message_inputs

started=$("$ms" proc started-at --pid $$) || {
  echo "agent commit wrapper refused: wrapper process start time is unreadable" >&2
  exit 1
}
nonce=$("$ms" util token-hex --bytes 16)
"$ms" lease commit-token --path "$token" --pid "$$" --start "$started" --nonce "$nonce"
trap 'rm -f -- "$token"' EXIT
# A malformed session trailer has slipped through four times
# (claude.ac for claude.ai) and costs an amend plus a forced update
# on both remotes every time: refuse it at the door. The message
# arguments are scanned, not the repository — the wrapper stays a
# wrapper.
for arg in "${commit_args[@]}"; do
  if [[ "$arg" == *"claude.ac/"* ]]; then
    echo "commit refused: the session trailer says claude.ac — the domain is claude.ai" >&2
    exit 2
  fi
done

# Capture the one whole-project index endpoint before either legacy or shared
# proof consumption. A conflicted index cannot represent a candidate.
prefix=$(git -C "$root" rev-parse --show-prefix)
toplevel=$(git -C "$root" rev-parse --show-toplevel)
proved_tree=$(git -C "$toplevel" write-tree) || {
  echo "agent commit refused: the index cannot be proved as a tree (unmerged entries?)" >&2
  exit 1
}
testing_contract=$($ms config conf-value --file "$root/metasystem.conf" --key testing.contract 2>/dev/null || true)
static_reproof=1

# A carried landing asks the live installed engine first. A candidate may have
# made that engine unreadable; in that one case build the enrolled HEAD engine
# in a detached scratch worktree. The base engine is allowed to decide only
# after the Go observer proves the candidate changed none of its policy owners.
build_base_carry_judge() { # output path
  local output=$1 scratch worktree build_root build_rc remove_rc
  scratch=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-carry-judge.XXXXXX") || return $?
  worktree=$scratch/base
  git -C "$toplevel" worktree add --detach "$worktree" HEAD 1>&2 || {
    build_rc=$?
    rmdir "$scratch" 2>/dev/null || true
    return "$build_rc"
  }
  build_root=$worktree
  [[ -z "$prefix" ]] || build_root=$worktree/${prefix%/}
  (
    cd "$build_root" || exit $?
    go build -o "$output" ./cmd/metasystem
  ) 1>&2
  build_rc=$?
  git -C "$toplevel" worktree remove --force "$worktree" 1>&2
  remove_rc=$?
  rmdir "$scratch" 2>/dev/null || true
  (( build_rc == 0 )) || return "$build_rc"
  (( remove_rc == 0 )) || return "$remove_rc"
  chmod +x "$output"
}

sha256_file() { # path
  shasum -a 256 "$1" | awk '{print $1}'
}

json_csv() { # judge, json, delivery field
  local reader=$1 value=$2 field=$3 encoded
  encoded=$("$reader" json get --value "$value" --field "delivery.$field" 2>/dev/null) || return $?
  if [[ "$encoded" == '[]' ]]; then
    printf '%s\n' '-'
    return 0
  fi
  printf '%s\n' "$encoded" | sed -E 's/^\[//; s/\]$//; s/"//g; s/,[[:space:]]*/,/g'
}

# Direct legacy commits receive the staged-package coverage boundary. A
# migrated installation instead consumes the already-admitted shared result;
# verify is read-only and starts neither tests nor builds. Carried mode defers
# that judgment into its observer so a readable red battery can be carried.
if [[ -n "$testing_contract" ]]; then
  if [[ -z "$landing_carried" ]]; then
    testing_verify_args=(test verify --root "$root" --tree "$proved_tree" --mode auto --purpose delivery)
    [[ -z "$landing_goal" ]] || testing_verify_args+=(--goal "$landing_goal")
    "$ms" "${testing_verify_args[@]}" 1>&2 || {
      echo "agent commit refused: required shared testing proof is missing or insufficient" >&2
      exit 1
    }
  fi
  policy_engine=$ms
  # The retained proof already ran fast-static-build (gofmt, vet, staticcheck,
  # the refusal register and the build) on this exact tree; the boundary
  # consumes it and only builds its proof engine below.
  static_reproof=0
else
# The delta checker owns package discovery and reports every package below its
# floor before it refuses.
coverage_arguments=(--staged)
if [[ -n "$ratchet" ]]; then
  coverage_arguments+=(--ratchet "$ratchet")
fi
bash "$root/scripts/agents/coverage-delta.sh" "${coverage_arguments[@]}" || {
  echo "agent commit refused: staged Go package coverage check failed" >&2
  exit 1
}
# IL-28 static re-proof: no landing goes red on a static check. The
# boundary re-proves gofmt, vet, staticcheck, the refusal register, and the engine build via
# the fast gate — plus the always-loaded word audit — before any commit
# concludes. This is the last-line static re-proof; weight-triggered full
# validation is a separately governed direct run. No environment escape: the
# gate's own header explains why a switch that outlives its edit loop would
# silently weaken the boundary, and the same reasoning holds here. On a
# non-Go adopted checkout the fast gate skips itself; a damaged or
# unbuildable tree refuses the commit.
#
# The proofs read the WORKING TREE, so they bind the prospective commit only
# while the index and working tree agree on the LANDING projection. The
# proof-built engine owns that projection, including nested-prefix handling;
# a policy edit is therefore judged by the policy in the prospective bytes.
# Untracked and ignored projected inputs count because the tools can consume
# them while the commit omits them. A staged gitlink in the projection still
# refuses because it exposes a nested checkout the commit records only as an
# object id.
# Build the prospective policy owner without touching bin/metasystem. The same
# proof engine later runs the audit and weighs the landing, so a stale live
# binary cannot classify any prospective byte.
proof_engine=$(mktemp "${TMPDIR:-/tmp}/metasystem-proof-engine.XXXXXX")
trap 'rm -f -- "$proof_engine" "$token"' EXIT
if (( static_reproof )); then
  "$root/scripts/agents/go-gate.sh" --fast --proof-out "$proof_engine" 1>&2 || {
    echo "agent commit refused: the static re-proof failed (go-gate.sh --fast)" >&2
    exit 1
  }
else
  bash "$root/scripts/agents/go-build.sh" --out "$proof_engine" 1>&2 || {
    echo "agent commit refused: the proof engine could not be built (go-build.sh --out)" >&2
    exit 1
  }
fi
policy_engine=$proof_engine
if [[ -s "$proof_engine" ]]; then
  chmod +x "$proof_engine"
else
  # Adopted non-Go checkouts carry the committed engine binary but no source
  # from which the fast gate could build a proof artifact. They cannot carry a
  # prospective policy edit, so their own bundled engine remains the policy
  # owner; source checkouts always take the proof-built branch above.
  policy_engine=$root/bin/metasystem
  [[ -x "$policy_engine" ]] || {
    echo "agent commit refused: no behavior-surface policy engine is available" >&2
    exit 1
  }
fi
fi

enumerate_inputs_nul() {
  git -C "$toplevel" diff --no-renames --name-only -z --
  git -C "$toplevel" ls-files --others --exclude-standard --full-name -z
  git -C "$toplevel" ls-files --others -i --exclude-standard --full-name -z
}
select_landing_nul() {
  "$policy_engine" behavior-surface select --projection LANDING --prefix "$prefix" --nul
}
show_nul_paths() { # file
  while IFS= read -r -d '' selected; do printf '  %q\n' "$selected" >&2; done <"$1"
}

unbound_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-unbound.XXXXXX")
enumerate_inputs_nul | select_landing_nul >"$unbound_file"
if [[ -s "$unbound_file" ]]; then
  echo "agent commit refused: the LANDING comparison found projected working-tree bytes that are not what the commit would record at its index endpoint:" >&2
  show_nul_paths "$unbound_file"
  rm -f "$unbound_file"
  echo "stage, stash, or remove them so the proof binds the bytes the commit records" >&2
  exit 1
fi
rm -f "$unbound_file"

gitlinks_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-gitlinks.XXXXXX")
git -C "$toplevel" ls-files -s -z | {
  while IFS= read -r -d '' record; do
    metadata=${record%%$'\t'*}; path=${record#*$'\t'}
    [[ ${metadata%% *} == 160000 ]] && printf '%s\0' "$path"
  done
  # No matches is the lawful empty set, not a failed producer under pipefail.
  true
} | select_landing_nul >"$gitlinks_file"
if [[ -s "$gitlinks_file" ]]; then
  echo "agent commit refused: a staged gitlink inside the proof scope carries a nested checkout the committed tree does not record:" >&2
  show_nul_paths "$gitlinks_file"
  rm -f "$gitlinks_file"
  exit 1
fi
rm -f "$gitlinks_file"
# A SYMLINK at a proof-input path makes the proofs follow bytes the
# committed tree records only as a target string (IL28-R6-4): refused at
# the critical input names. Skill-registration symlinks stay lawful —
# following them is the audit's sanctioned mechanism.
symlinked_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-symlinks.XXXXXX")
git -C "$toplevel" ls-files -s -z | while IFS= read -r -d '' record; do
  metadata=${record%%$'\t'*}; path=${record#*$'\t'}
  [[ ${metadata%% *} == 120000 ]] || continue
  relative=$path
  [[ -z "$prefix" ]] || relative=${path#"$prefix"}
  case "$relative" in
    AGENTS.md|*/AGENTS.md|wow.md|*/wow.md|*.go|go.mod|go.sum|go.work|go.work.sum|docs|*/docs|docs/*|*/docs/*|docs/project-rules.md|*/docs/project-rules.md|metasystem.conf|*/metasystem.conf|cmd|*/cmd|cmd/*|*/cmd/*|internal|*/internal|internal/*|*/internal/*|scripts|*/scripts|scripts/*|*/scripts/*)
      printf '%s\0' "$path" ;;
  esac
done | select_landing_nul >"$symlinked_file"
if [[ -s "$symlinked_file" ]]; then
  echo "agent commit refused: a critical proof input is a symlink; the proofs would follow bytes the committed tree does not record:" >&2
  show_nul_paths "$symlinked_file"
  rm -f "$symlinked_file"
  exit 1
fi
rm -f "$symlinked_file"
# assume-unchanged and skip-worktree entries hide index/worktree
# divergence from every diff the closure runs (IL28-R6-2): in scope,
# they refuse — the proof cannot bind what git will not show it.
hidden_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-hidden.XXXXXX")
git -C "$toplevel" ls-files -v -z | {
  while IFS= read -r -d '' record; do
    marker=${record%% *}; path=${record#* }
    [[ "$marker" == S || "$marker" =~ ^[a-z]$ ]] && printf '%s\0' "$path"
  done
  # No matches is the lawful empty set, not a failed producer under pipefail.
  true
} | select_landing_nul >"$hidden_file"
if [[ -s "$hidden_file" ]]; then
  echo "agent commit refused: assume-unchanged or skip-worktree entries hide proof inputs from the divergence closure:" >&2
  show_nul_paths "$hidden_file"
  rm -f "$hidden_file"
  exit 1
fi
rm -f "$hidden_file"
# The proof's progress chatter is diagnostics, never landing output:
# callers own this wrapper's stdout (benchmark provisioning's
# three-human-steps contract reads it), so both proofs speak on stderr.
# And the proof is SIDE-EFFECT-FREE: the gate compiles to a scratch
# path and bin/metasystem stays byte-identical — a supervision-armed
# checkout fingerprints the live binary, and a commit-time swap under
# an armed watch broke the benchmark target's preflight the day this
# boundary first met one.
# The audit proof runs on the freshly PROOF-built engine with its
# override knobs cleared (IL28-R4-4): a stale exported cap or
# placeholder waiver is exactly the long-lived environment escape the
# boundary forbids. On a non-Go adopted checkout the fast gate skips
# without building, and the audit runs on the checkout's own engine.
if [[ -z "$testing_contract" ]]; then
  audit_engine=$policy_engine
  env -u METASYSTEM_MAX_ALWAYS_LOADED_WORDS -u METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS \
    METASYSTEM_BIN="$audit_engine" \
    "$root/scripts/audit-metasystem.sh" "$root" 1>&2 || {
    echo "agent commit refused: the static re-proof failed (audit-metasystem.sh)" >&2
    exit 1
  }
fi
settled_tree=$(git -C "$toplevel" write-tree) || {
  echo "agent commit refused: the index cannot be re-proved as a tree" >&2
  exit 1
}
settled_unbound=$(mktemp "${TMPDIR:-/tmp}/metasystem-settled-unbound.XXXXXX")
enumerate_inputs_nul | select_landing_nul >"$settled_unbound"
if [[ "$settled_tree" != "$proved_tree" ]] || [[ -s "$settled_unbound" ]]; then
  rm -f "$settled_unbound"
  echo "agent commit refused: the index or a gate input moved while the proof ran; re-stage and retry" >&2
  exit 1
fi
rm -f "$settled_unbound"

# Evaluate the exact project tree the commit is about to record. Every outcome
# remains a durable observation, while the base-tree promotion record may mark
# a named verdict as refusing for agent commits. Human commits never consume
# that refusal bit.
machine_nickname=$(git -C "$root" config --get metasystem.goal.machine || true)
[[ -n "$machine_nickname" ]] \
  || { echo "commit refused: no machine nickname is enrolled and hostnames are never published — run  git config metasystem.goal.machine <nickname>  once on this machine" >&2; exit 2; }
landing_actor="${machine_nickname}+${METASYSTEM_OWNER_LINEAGE:-human}"

landing_tree=$settled_tree
if [[ -n "$prefix" ]]; then
  resolved_landing_tree=$(git -C "$root" rev-parse "$settled_tree:${prefix%/}" 2>/dev/null || true)
  [[ -z "$resolved_landing_tree" ]] || landing_tree=$resolved_landing_tree
fi
landing_observe_args=(landing observe --root "$root" --tree "$landing_tree")
[[ -z "$landing_chain" ]] || landing_observe_args+=(--chain "$landing_chain")
[[ -z "$landing_direct_fix" ]] || landing_observe_args+=(--direct-fix "$landing_direct_fix")
[[ -z "$landing_revert_of" ]] || landing_observe_args+=(--revert-of "$landing_revert_of")
[[ -z "$landing_goal" ]] || landing_observe_args+=(--goal "$landing_goal")
[[ -z "$landing_root_job" ]] || landing_observe_args+=(--root-job "$landing_root_job")
[[ -z "$landing_test_receipt" ]] || landing_observe_args+=(--test-receipt "$landing_test_receipt")
[[ -z "$landing_recertification" ]] || landing_observe_args+=(--recertification "$landing_recertification")
if [[ -n "$landing_carried" ]]; then
  landing_observe_args+=(--carried "$landing_carried" --project-tree "$settled_tree" \
    --ledger-tip "$landing_ledger_tip" --carried-by "$landing_carried_by")
fi
landing_observe_args+=(--actor "$landing_actor")
landing_provenance="none change=unknown"
landing_verdict="would-refuse code=evaluator-unavailable"
landing_code="evaluator-unavailable"
landing_mode="refuse"
landing_observation=
landing_refusal=
judge=$policy_engine
judge_mode=
judge_tree=
judge_digest=
live_failure=
judge_trailer=
read_landing_observation() { # JSON reader
  local reader=$1
  observed_provenance=$("$reader" json get --value "$landing_observation" --field provenance 2>/dev/null || true)
  observed_verdict=$("$reader" json get --value "$landing_observation" --field verdictTrailer 2>/dev/null || true)
  observed_code=$("$reader" json get --value "$landing_observation" --field code 2>/dev/null || true)
  observed_mode=$("$reader" json get --value "$landing_observation" --field mode 2>/dev/null || true)
  observed_refusal=$("$reader" json get --value "$landing_observation" --field refusal --default "" 2>/dev/null || true)
  if [[ -n "$observed_provenance" && -n "$observed_verdict" && -n "$observed_code" \
    && ( "$observed_mode" == observe || "$observed_mode" == refuse ) ]]; then
    landing_provenance=$observed_provenance
    landing_verdict=$observed_verdict
    landing_code=$observed_code
    landing_mode=$observed_mode
    landing_refusal=$observed_refusal
    return 0
  fi
  return 1
}

if [[ -n "$landing_carried" ]]; then
  set +e
  landing_observation=$("$ms" "${landing_observe_args[@]}" --judge live 2>/dev/null)
  live_rc=$?
  set -e
  if (( live_rc == 0 )) && read_landing_observation "$ms"; then
    judge=$ms
    judge_mode=live
  else
    live_code=$("$ms" json get --value "$landing_observation" --field code 2>/dev/null || true)
    if [[ -n "$live_code" ]]; then
      live_failure=$live_code
    else
      live_failure=exit=$live_rc
    fi
    judge=$(mktemp "${TMPDIR:-/tmp}/metasystem-base-judge.XXXXXX") || exit $?
    trap 'rm -f -- "$judge" "$token"' EXIT
    if ! build_base_carry_judge "$judge"; then
      echo "no live or base judge decided; the base judge build failed; rebuild and arm an engine at a good commit with steward arm" >&2
      exit 3
    fi
    judge_tree=$(git -C "$root" rev-parse HEAD^{tree}) || exit $?
    set +e
    landing_observation=$("$judge" "${landing_observe_args[@]}" --judge base --live-failure "$live_failure" 2>/dev/null)
    base_rc=$?
    set -e
    if (( base_rc != 0 )) || ! read_landing_observation "$judge"; then
      echo "no live or base judge decided; base judge exit=$base_rc; rebuild and arm an engine at a good commit with steward arm" >&2
      exit 3
    fi
    judge_mode=base
  fi
  judge_digest=$(sha256_file "$judge") || exit $?
  judge_trailer="$judge_mode sha256=$judge_digest"
  if [[ "$judge_mode" == base ]]; then
    judge_trailer="base tree=$judge_tree sha256=$judge_digest live-failure=$live_failure"
  fi
elif landing_observation=$("$policy_engine" "${landing_observe_args[@]}" 2>/dev/null); then
  read_landing_observation "$ms" || true
fi

if [[ -n "$landing_carried" && "$landing_mode" == refuse ]]; then
  if [[ -n "$landing_refusal" ]]; then
    printf '%s: %s\n' "$landing_code" "$landing_refusal" >&2
  else
    printf 'carried landing asks: %s\n' "$landing_verdict" >&2
  fi
  exit 3
fi
if (( agent_commit )) && [[ "$landing_mode" == refuse ]]; then
  refusal_paths=$(mktemp "${TMPDIR:-/tmp}/metasystem-landing-refusal-paths.XXXXXX")
  git -C "$root" diff --cached --name-only -z -- >"$refusal_paths"
  case "$landing_code" in
    evaluator-unavailable)
      echo "agent commit refused: the landing evaluator failed or returned an incomplete decision ($landing_verdict)" >&2
      landing_repair="restore or rebuild the proof-built landing evaluator, then retry"
      ;;
    promotion-base-unreadable)
      echo "agent commit refused: the landing base tree is unreadable ($landing_verdict)" >&2
      landing_repair="restore a readable landing base tree at HEAD, then retry"
      ;;
    promotion-record-malformed)
      echo "agent commit refused: the landing promotion record is malformed ($landing_verdict)" >&2
      landing_repair="a human must repair the landing promotion record before an agent retries"
      ;;
    path-unclassified)
      echo "agent commit refused: the landing contains an unclassified path ($landing_verdict)" >&2
      [[ -z "$landing_refusal" ]] || printf '%s\n' "$landing_refusal" >&2
      landing_repair="classify every named path in scripts/agents/path-classes.txt, then retry"
      ;;
    ledger-path-not-goal-verb)
      echo "agent commit refused: ledger paths change only through goal verbs ($landing_verdict)" >&2
      landing_repair="use the owning goal verb instead of the commit wrapper"
      ;;
    runtime-path-refused)
      echo "agent commit refused: runtime paths cannot be landed ($landing_verdict)" >&2
      landing_repair="remove runtime output from the staged tree"
      ;;
    exact-revert-record-refused)
      echo "agent commit refused: exact revert cannot delete or truncate records ($landing_verdict)" >&2
      landing_repair="restore the record and carry a forward record instead"
      ;;
    goal-item-not-held)
      echo "agent commit refused: the Goal-Item is not held by this machine and lineage ($landing_verdict)" >&2
      landing_repair="use a goal claimed by this machine and lineage"
      ;;
    record-not-owned)
      echo "agent commit refused: the staged record is not owned by this landing ($landing_verdict)" >&2
      landing_repair="carry only new records or records owned by the held goal or actor"
      ;;
    register-carriage-policy-unreadable|direct-fix-policy-unreadable)
      echo "agent commit refused: the base path-class policy is unreadable ($landing_verdict)" >&2
      landing_repair="repair the path-class manifest through a reviewed implementation chain"
      ;;
    register-carriage-not-append-only)
      echo "agent commit refused: register carriage rewrote or deleted existing record bytes ($landing_verdict)" >&2
      landing_repair="restore existing bytes and append complete lines only"
      ;;
    *)
      echo "agent commit refused: promoted landing verdict $landing_verdict" >&2
      landing_repair=
      ;;
  esac
  echo "staged paths:" >&2
  show_nul_paths "$refusal_paths"
  rm -f "$refusal_paths"
  [[ -z "$landing_repair" ]] || echo "$landing_repair" >&2
  echo "lawful classification exits: declare the reviewed implementation chain with --chain <root-job-id>, or fix the Change-Class classification and retry" >&2
  exit 1
fi

carried_workspace=
carried_battery=
carried_missing=-
carried_failing=-
if [[ -n "$landing_carried" ]]; then
  if [[ "$landing_mode" != observe || "$landing_code" != human-carried \
    || "$landing_provenance" != *" opid=$landing_carried "* \
    || "$landing_provenance" != *" past=$landing_carried_past "* \
    || "$landing_provenance" != *" ledger=$landing_ledger_tip "* ]]; then
    echo "carried landing asks: the deciding observation does not bind the requested word, refusal, and ledger" >&2
    exit 3
  fi
  carried_workspace=$("$judge" landing workspace --root "$root" --tree "$settled_tree") || {
    echo "agent commit refused: the carried workspace projection is unreadable" >&2
    exit 1
  }
  testing_json=
  testing_args=(test verify --root "$root" --tree "$settled_tree" --mode auto --purpose delivery --carried --json)
  [[ -z "$landing_goal" ]] || testing_args+=(--goal "$landing_goal")
  set +e
  testing_json=$("$judge" "${testing_args[@]}" 2>/dev/null)
  testing_rc=$?
  set -e
  testing_sufficient=$("$judge" json get --value "$testing_json" --field delivery.sufficient 2>/dev/null || true)
  if [[ "$testing_sufficient" != true && "$testing_sufficient" != false ]]; then
    echo "test verify failed: no structured delivery result; no word carries an unverified battery; repair the testing tool or its evidence and rerun" >&2
    exit 3
  fi
  if [[ "$testing_sufficient" == true ]]; then
    (( testing_rc == 0 )) || {
      echo "agent commit refused: test verify returned success evidence with a failing process status" >&2
      exit 1
    }
    carried_battery=green
  else
    carried_battery=red
    carried_missing=$(json_csv "$judge" "$testing_json" missingGroups) || {
      echo "agent commit refused: test verify returned an unreadable missing-groups list" >&2
      exit 1
    }
    carried_failing=$(json_csv "$judge" "$testing_json" failingGroups) || {
      echo "agent commit refused: test verify returned an unreadable failing-groups list" >&2
      exit 1
    }
  fi
fi
carried_battery_trailer=$carried_battery
if [[ "$carried_battery" == red ]]; then
  carried_battery_trailer="red missing=$carried_missing failing=$carried_failing"
fi
# The proof binds THE INDEX; the postcondition proves the commit
# recorded exactly that tree. This replaces any argument grammar
# (IL28-R2-2, IL28-R3-2, IL28-R4-1, IL28-R4-5): whatever selected
# different content — a pathspec in any spelling, --only/--include/
# --all, an abbreviated option, or a hook staging bytes mid-commit —
# the landed tree differs from the proved tree, the commit is rolled
# back softly (index and worktree untouched), and the refusal names the
# principle.
proved_head=$(git -C "$root" rev-parse --verify --quiet HEAD || true)
# Every landing names the machine it came from — by its enrolled
# nickname, never its hostname: the trailer is pushed to shared
# remotes, and what a machine IS stays off them. The wrapper stamps
# it so it is uniform on every machine and never typed by an author.
commit_trailers=(
  --trailer "Machine: $landing_actor"
  --trailer "Landing-Provenance: $landing_provenance"
  --trailer "Landing-Provenance-Verdict: $landing_verdict"
)
[[ -z "$landing_goal" ]] || commit_trailers+=(--trailer "Goal-Item: $landing_goal")
if [[ -n "$landing_carried" ]]; then
  commit_trailers+=(
    --trailer "Carried-By: $landing_carried_by"
    --trailer "Carry: $landing_carried"
    --trailer "Carried-Tree: workspace=$carried_workspace project=$settled_tree"
    --trailer "Carried-Past: $landing_carried_past"
  )
  if [[ "$carried_battery" == green ]]; then
    commit_trailers+=(--trailer "Carried-Battery: green")
  else
    commit_trailers+=(--trailer "Carried-Battery: red missing=$carried_missing failing=$carried_failing")
  fi
  commit_trailers+=(
    --trailer "Carried-Judge: $judge_trailer"
    --trailer "Carried-Ledger: $landing_ledger_tip"
  )
fi
git -C "$root" commit "${commit_trailers[@]}" "${commit_args[@]}"
landed_tree=$(git -C "$root" rev-parse HEAD^{tree})
landed_message=$(git -C "$root" log -1 --format=%B)
goal_item_count=$(LC_ALL=C grep -Eic '^Goal-Item:' <<<"$landed_message" || true)
exact_goal_item_count=0
if [[ -n "$landing_goal" ]]; then
  exact_goal_item_count=$(grep -Fxc -- "Goal-Item: $landing_goal" <<<"$landed_message" || true)
fi
carried_postcondition=0
carried_postcondition_detail=
carried_keys=(Carry Carried-By Carried-Tree Carried-Past Carried-Battery Carried-Judge Carried-Ledger)
if [[ -n "$landing_carried" ]]; then
  for carried_key in "${carried_keys[@]}"; do
    carried_count=$(LC_ALL=C grep -Ec "^${carried_key}:" <<<"$landed_message" || true)
    if (( carried_count != 1 )); then
      carried_postcondition=1
      carried_postcondition_detail="$carried_key count=$carried_count"
      break
    fi
  done
  for expected_line in \
    "Carry: $landing_carried" \
    "Carried-By: $landing_carried_by" \
    "Carried-Tree: workspace=$carried_workspace project=$settled_tree" \
    "Carried-Past: $landing_carried_past" \
    "Carried-Battery: $carried_battery_trailer" \
    "Carried-Judge: $judge_trailer" \
    "Carried-Ledger: $landing_ledger_tip"; do
    if [[ $(grep -Fxc -- "$expected_line" <<<"$landed_message" || true) -ne 1 ]]; then
      carried_postcondition=1
      carried_postcondition_detail="wrong carried trailer: $expected_line"
      break
    fi
  done
else
  for carried_key in "${carried_keys[@]}"; do
    carried_count=$(LC_ALL=C grep -Ec "^${carried_key}:" <<<"$landed_message" || true)
    if (( carried_count != 0 )); then
      carried_postcondition=1
      carried_postcondition_detail="$carried_key must be absent"
      break
    fi
  done
fi
if [[ "$landed_tree" != "$proved_tree" || ( -n "$landing_goal" && ( $goal_item_count -ne 1 || $exact_goal_item_count -ne 1 ) ) || ( -z "$landing_goal" && $goal_item_count -ne 0 ) || $carried_postcondition -ne 0 ]]; then
  if [[ -n "$proved_head" ]]; then
    git -C "$root" reset --soft "$proved_head"
  else
    git -C "$root" update-ref -d HEAD
  fi
  if [[ "$landed_tree" != "$proved_tree" ]]; then
    echo "agent commit refused: the commit recorded a tree the static re-proof never judged (content selection beyond the index); the commit was rolled back — stage the exact bytes and commit them plainly" >&2
  elif (( carried_postcondition )); then
    echo "agent commit refused: the final commit message failed the carried-trailer postcondition ($carried_postcondition_detail); the commit was rolled back" >&2
  else
    echo "agent commit refused: the final commit message did not contain exactly one byte-exact Goal-Item stamped by --goal; the commit was rolled back" >&2
  fi
  exit 1
fi

# The landing is both remotes or it is not a landing (--push): agents
# remembered this rule around the tooling until one push was missed;
# now the wrapper owns it. Origin first; transport only if declared.
if (( push_after )); then
  branch=$(git -C "$root" symbolic-ref --short HEAD) || {
    echo "landing push refused: HEAD is not on a branch" >&2
    exit 1
  }
  git -C "$root" push origin "$branch" || {
    echo "landing push failed at origin; the commit stands locally — resolve and push both remotes" >&2
    exit 1
  }
  if git -C "$root" remote | grep -qx transport; then
    # Transport receives origin's ref, never the local branch: the
    # mirror sync cannot carry a commit origin has not accepted, so a
    # pre-review local chain cannot leak through this leg.
    bash "$root/scripts/agents/sync-transport.sh" "$branch" || {
      echo "landing push failed at transport with origin already pushed; resolve and rerun scripts/agents/sync-transport.sh" >&2
      exit 1
    }
  fi
fi

# The landing weighed LAST, after every remote the caller asked for
# has accepted it — a failed push exits above and adds nothing. The
# due line is a NUDGE toward the governed direct validator (findings fix
# forward), and weight bookkeeping never refuses a concluded landing.
git -C "$root" show --no-renames --numstat -z --format= HEAD 2>/dev/null \
  | "$policy_engine" gate weight-add --root "$root" --prefix "$prefix" \
      --commit "$(git -C "$root" rev-parse --short HEAD)" \
      ${landing_goal:+--goal "$landing_goal"} \
  || echo "validation-weight bookkeeping skipped (non-fatal)" >&2
