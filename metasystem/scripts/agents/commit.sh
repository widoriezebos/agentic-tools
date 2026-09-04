#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
token=$root/artifacts/agents/mains/worktree-commit-token.json

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

if (( landing_goal_set )) && [[ -z "$landing_goal" || ${#landing_goal} -gt 100 || ! "$landing_goal" =~ ^[a-z0-9-]+$ ]]; then
  echo "commit refused: --goal must be a lowercase kebab identifier of at most 100 characters" >&2
  exit 2
fi

message_has_goal_item() { # message text
  LC_ALL=C grep -Eiq '^Goal-Item:' <<<"$1"
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
            if message_has_goal_item "$value"; then
              echo "commit refused: Goal-Item is stamped by --goal, never typed" >&2
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
            if message_has_goal_item "$(<"$value")"; then
              echo "commit refused: Goal-Item is stamped by --goal, never typed" >&2
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
        if message_has_goal_item "$value"; then
          echo "commit refused: Goal-Item is stamped by --goal, never typed" >&2
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
        if message_has_goal_item "$(<"$value")"; then
          echo "commit refused: Goal-Item is stamped by --goal, never typed" >&2
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

# Direct commits receive the same staged-package coverage boundary as landings.
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
# boundary re-proves gofmt, vet, staticcheck, and the engine build via
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
prefix=$(git -C "$root" rev-parse --show-prefix)
toplevel=$(git -C "$root" rev-parse --show-toplevel)

# The INDEX TREE is captured before either proof and checked again afterward.
# A conflicted index cannot be represented as the one tree being judged.
proved_tree=$(git -C "$root" write-tree) || {
  echo "agent commit refused: the index cannot be proved as a tree (unmerged entries?)" >&2
  exit 1
}

# Build the prospective policy owner without touching bin/metasystem. The same
# proof engine later runs the audit and weighs the landing, so a stale live
# binary cannot classify any prospective byte.
proof_engine=$(mktemp "${TMPDIR:-/tmp}/metasystem-proof-engine.XXXXXX")
trap 'rm -f -- "$proof_engine" "$token"' EXIT
"$root/scripts/agents/go-gate.sh" --fast --proof-out "$proof_engine" 1>&2 || {
  echo "agent commit refused: the static re-proof failed (go-gate.sh --fast)" >&2
  exit 1
}
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
audit_engine=$policy_engine
env -u METASYSTEM_MAX_ALWAYS_LOADED_WORDS -u METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS \
  METASYSTEM_BIN="$audit_engine" \
  "$root/scripts/audit-metasystem.sh" "$root" 1>&2 || {
  echo "agent commit refused: the static re-proof failed (audit-metasystem.sh)" >&2
  exit 1
}
settled_tree=$(git -C "$root" write-tree) || {
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
landing_observe_args+=(--actor "$landing_actor")
landing_provenance="none change=unknown"
landing_verdict="would-refuse code=evaluator-unavailable"
landing_code="evaluator-unavailable"
landing_mode="refuse"
landing_observation=
landing_refusal=
if landing_observation=$("$policy_engine" "${landing_observe_args[@]}" 2>/dev/null); then
  observed_provenance=$("$ms" json get --value "$landing_observation" --field provenance 2>/dev/null || true)
  observed_verdict=$("$ms" json get --value "$landing_observation" --field verdictTrailer 2>/dev/null || true)
  observed_code=$("$ms" json get --value "$landing_observation" --field code 2>/dev/null || true)
  observed_mode=$("$ms" json get --value "$landing_observation" --field mode 2>/dev/null || true)
  observed_refusal=$("$ms" json get --value "$landing_observation" --field refusal --default "" 2>/dev/null || true)
  if [[ -n "$observed_provenance" && -n "$observed_verdict" && -n "$observed_code" \
    && ( "$observed_mode" == observe || "$observed_mode" == refuse ) ]]; then
    landing_provenance=$observed_provenance
    landing_verdict=$observed_verdict
    landing_code=$observed_code
    landing_mode=$observed_mode
    landing_refusal=$observed_refusal
  fi
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
git -C "$root" commit "${commit_trailers[@]}" "${commit_args[@]}"
landed_tree=$(git -C "$root" rev-parse HEAD^{tree})
landed_message=$(git -C "$root" log -1 --format=%B)
goal_item_count=$(LC_ALL=C grep -Eic '^Goal-Item:' <<<"$landed_message" || true)
exact_goal_item_count=0
if [[ -n "$landing_goal" ]]; then
  exact_goal_item_count=$(grep -Fxc -- "Goal-Item: $landing_goal" <<<"$landed_message" || true)
fi
if [[ "$landed_tree" != "$proved_tree" || ( -n "$landing_goal" && ( $goal_item_count -ne 1 || $exact_goal_item_count -ne 1 ) ) || ( -z "$landing_goal" && $goal_item_count -ne 0 ) ]]; then
  if [[ -n "$proved_head" ]]; then
    git -C "$root" reset --soft "$proved_head"
  else
    git -C "$root" update-ref -d HEAD
  fi
  if [[ "$landed_tree" != "$proved_tree" ]]; then
    echo "agent commit refused: the commit recorded a tree the static re-proof never judged (content selection beyond the index); the commit was rolled back — stage the exact bytes and commit them plainly" >&2
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
  || echo "validation-weight bookkeeping skipped (non-fatal)" >&2
