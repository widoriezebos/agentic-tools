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
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
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
for arg in "$@"; do
  if [[ "$arg" == *"claude.ac/"* ]]; then
    echo "commit refused: the session trailer says claude.ac — the domain is claude.ai" >&2
    exit 2
  fi
done
# IL-28 static re-proof: no landing goes red on a static check. The
# boundary re-proves gofmt, vet, staticcheck, and the engine build via
# the fast gate — plus the always-loaded word audit — before any commit
# concludes. This is the last-line static re-proof, never the landing
# gate: the full battery (go-gate, mission-fixtures, dispatch-fixtures)
# remains the landing requirement. No environment escape: the gate's
# own header explains why a switch that outlives its edit loop would
# silently weaken the boundary, and the same reasoning holds here. On a
# non-Go adopted checkout the fast gate skips itself; a damaged or
# unbuildable tree refuses the commit.
#
# The proofs read the WORKING TREE, so they bind the prospective commit
# only while the index and the worktree agree on every input they
# consume. The closure is INVERTED (IL28-R5-4/5): everything under the
# module prefix is a proof input — the gate and audit read the tree —
# except the paths no proof consumes (plans/, bin/, artifacts/) and
# machine-local overrides that are never committed content
# (metasystem.conf.local); a committed toplevel go.work would steer the
# build and is in scope unprefixed. Untracked and IGNORED inputs count
# too — the toolchain consumes them while the commit omits them — and
# every enumeration uses --full-name so nested-prefix layouts share one
# path space (IL28-R5-3). A staged gitlink inside the scope would make
# the tools read a nested checkout the committed tree does not carry
# (IL28-R5-6) and refuses.
prefix=$(git -C "$root" rev-parse --show-prefix)
# The prefix is interpolated into extended regexes, so its metacharacters
# are escaped literally (IL28-R6-3) — a lawful directory name must never
# silently disable the closure.
prefix_re=$(printf '%s' "$prefix" | sed 's/[][\.|$(){}?+*^]/\\&/g')
# At the repository toplevel (empty prefix) everything is in scope; the
# alternation form would leave an empty branch some greps refuse.
if [[ -n "$prefix_re" ]]; then
  gate_scope_re="^(${prefix_re}|go\.work$|go\.work\.sum$)"
else
  gate_scope_re="^"
fi
# plans/ is exempt EXCEPT the three plan files the audit itself reads
# (IL28-R6-1); bin/, artifacts/, and the never-committed local override
# stay out of scope entirely.
gate_exempt_re="^${prefix_re}((plans|bin|artifacts)/|metasystem\.conf\.local$)"
audit_plans_re="^${prefix_re}plans/(README|instruction-ledger|known-issues)\.md$"
enumerate_inputs() {
  git -C "$root" diff --name-only --
  git -C "$root" ls-files --others --exclude-standard --full-name
  git -C "$root" ls-files --others -i --exclude-standard --full-name
}
unbound_gate_inputs() {
  enumerate_inputs | grep -E "$gate_scope_re" | grep -Ev "$gate_exempt_re" || true
  enumerate_inputs | grep -E "$audit_plans_re" || true
}
unbound=$(unbound_gate_inputs)
if [[ -n "$unbound" ]]; then
  echo "agent commit refused: the static re-proof reads the working tree, and these gate inputs are not what the commit would record:" >&2
  printf '  %s\n' $unbound >&2
  echo "stage, stash, or remove them so the proof binds the bytes the commit records" >&2
  exit 1
fi
gitlinks=$(git -C "$root" ls-files -s | awk '$1 == "160000"' | cut -f2 | grep -E "$gate_scope_re" | grep -Ev "$gate_exempt_re" || true)
if [[ -n "$gitlinks" ]]; then
  echo "agent commit refused: a staged gitlink inside the proof scope carries a nested checkout the committed tree does not record:" >&2
  printf '  %s\n' $gitlinks >&2
  exit 1
fi
# A SYMLINK at a proof-input path makes the proofs follow bytes the
# committed tree records only as a target string (IL28-R6-4): refused at
# the critical input names. Skill-registration symlinks stay lawful —
# following them is the audit's sanctioned mechanism.
symlinked=$(git -C "$root" ls-files -s | awk '$1 == "120000"' | cut -f2 \
  | grep -E "$gate_scope_re" | grep -Ev "$gate_exempt_re" \
  | grep -E '(^|/)((AGENTS|wow)\.md$|.*\.go$|go\.(mod|sum|work)$|docs/project-rules\.md$|metasystem\.conf$|scripts/)' || true)
if [[ -n "$symlinked" ]]; then
  echo "agent commit refused: a critical proof input is a symlink; the proofs would follow bytes the committed tree does not record:" >&2
  printf '  %s\n' $symlinked >&2
  exit 1
fi
# assume-unchanged and skip-worktree entries hide index/worktree
# divergence from every diff the closure runs (IL28-R6-2): in scope,
# they refuse — the proof cannot bind what git will not show it.
hidden=$(git -C "$root" ls-files -v | awk '$1 ~ /^[a-z]$|^S$/' | cut -d' ' -f2- \
  | grep -E "$gate_scope_re" | grep -Ev "$gate_exempt_re" || true)
if [[ -n "$hidden" ]]; then
  echo "agent commit refused: assume-unchanged or skip-worktree entries hide proof inputs from the divergence closure:" >&2
  printf '  %s\n' $hidden >&2
  exit 1
fi
# The INDEX TREE is captured before the proofs run and re-checked after
# (IL28-R5-2): bytes staged mid-proof were never judged and refuse. A
# conflicted index cannot write a tree and refuses first.
proved_tree=$(git -C "$root" write-tree) || {
  echo "agent commit refused: the index cannot be proved as a tree (unmerged entries?)" >&2
  exit 1
}
# The proof's progress chatter is diagnostics, never landing output:
# callers own this wrapper's stdout (benchmark provisioning's
# three-human-steps contract reads it), so both proofs speak on stderr.
# And the proof is SIDE-EFFECT-FREE: the gate compiles to a scratch
# path and bin/metasystem stays byte-identical — a supervision-armed
# checkout fingerprints the live binary, and a commit-time swap under
# an armed watch broke the benchmark target's preflight the day this
# boundary first met one.
proof_engine=$(mktemp "${TMPDIR:-/tmp}/metasystem-proof-engine.XXXXXX")
trap 'rm -f -- "$proof_engine"' EXIT
"$root/scripts/agents/go-gate.sh" --fast --proof-out "$proof_engine" 1>&2 || {
  echo "agent commit refused: the static re-proof failed (go-gate.sh --fast)" >&2
  exit 1
}
# The audit proof runs on the freshly PROOF-built engine with its
# override knobs cleared (IL28-R4-4): a stale exported cap or
# placeholder waiver is exactly the long-lived environment escape the
# boundary forbids. On a non-Go adopted checkout the fast gate skips
# without building, and the audit runs on the checkout's own engine.
audit_engine="$proof_engine"
if [[ -s "$audit_engine" ]]; then
  chmod +x "$audit_engine"
else
  audit_engine="$root/bin/metasystem"
fi
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
if [[ "$settled_tree" != "$proved_tree" ]] || [[ -n "$(unbound_gate_inputs)" ]]; then
  echo "agent commit refused: the index or a gate input moved while the proof ran; re-stage and retry" >&2
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
machine_nickname=$(git -C "$root" config --get metasystem.goal.machine || true)
[[ -n "$machine_nickname" ]] \
  || { echo "commit refused: no machine nickname is enrolled and hostnames are never published — run  git config metasystem.goal.machine <nickname>  once on this machine" >&2; exit 2; }
git -C "$root" commit --trailer "Machine: ${machine_nickname}+${METASYSTEM_OWNER_LINEAGE:-human}" "$@"
landed_tree=$(git -C "$root" rev-parse HEAD^{tree})
if [[ "$landed_tree" != "$proved_tree" ]]; then
  if [[ -n "$proved_head" ]]; then
    git -C "$root" reset --soft "$proved_head"
  else
    git -C "$root" update-ref -d HEAD
  fi
  echo "agent commit refused: the commit recorded a tree the static re-proof never judged (content selection beyond the index); the commit was rolled back — stage the exact bytes and commit them plainly" >&2
  exit 1
fi

# The landing is both remotes or it is not a landing (--push): agents
# remembered this rule around the tooling until one push was missed;
# now the wrapper owns it. Origin first; transport only if declared.
# Every landing folds its measured weight into the battery
# accumulator; the due line is a NUDGE toward the milestone battery
# (findings fix forward), and weight bookkeeping never refuses a
# landing that already concluded.
git -C "$root" show --numstat --format= HEAD 2>/dev/null \
  | "$ms" gate weight-add --root "$root" --commit "$(git -C "$root" rev-parse --short HEAD)" \
  || echo "battery-weight bookkeeping skipped (non-fatal)" >&2

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
