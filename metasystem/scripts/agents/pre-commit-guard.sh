#!/usr/bin/env bash
# IL-14: the mechanical form of a rule that failed twice as prose.
#
# With a peer agent session holding work in the same tree, `git add` on a
# directory once committed a peer's plan file minutes after this session
# promised to leave it alone (0b9ca1b), and the explicit-paths rule written in
# response was then violated all period by its own author. A brand-new file
# appearing under plans/ in the staged set is exactly the shape of that
# accident, so it needs an explicit, per-commit acknowledgment:
#
#   METASYSTEM_ALLOW_NEW_PLAN=1 git commit ...
#
# Modifying a tracked plan stays free; only additions are challenged, because
# only additions can smuggle in a file the committer has never read.
set -euo pipefail

# Enrollment's execution probe: the goal CLI proves the fence RUNS —
# not merely that a hook mentions it — by invoking the hook chain
# with a nonce and requiring this acknowledgment. The distinct exit
# code stops the chain so no downstream hook does real work.
if [[ -n "${METASYSTEM_GUARD_PROBE:-}" ]]; then
  echo "guard-probe-ack ${METASYSTEM_GUARD_PROBE}"
  exit 42
fi

guard_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
git rev-parse --show-toplevel >/dev/null 2>&1 || exit 0
ms="${METASYSTEM_BIN:-$guard_root/bin/metasystem}"
if [[ -x "$ms" ]]; then
  classification=
  caller_class=
  if classification=$("$ms" lease classify --root "$guard_root" --caller-pid "$$" 2>/dev/null) \
    && caller_class=$("$ms" json get --value "$classification" --field class 2>/dev/null); then
    :
  fi
  # Human commits are sovereign. A broken classifier cannot positively prove
  # agent ancestry, so this hook leaves the commit untouched; valid agent
  # classifications still require the live wrapper token below.
  if [[ -n "$caller_class" && "$caller_class" != HUMAN ]]; then
    token=$guard_root/artifacts/agents/mains/worktree-commit-token.json
    # The wrapper-token proof: the token's fields must be valid, the wrapper
    # pid must appear in this process's ancestry, and the process at that pid
    # must have started at exactly the recorded second, so a recycled pid
    # never passes.
    if ! "$ms" validate wrapper-token --token "$token" --caller-pid "$$" 2>/dev/null; then
      echo "pre-commit guard: agent commit requires scripts/agents/commit.sh; the live wrapper ancestry token is missing" >&2
      exit 1
    fi
  fi
fi

# The goal ledger changes only through goal verbs (BGS-7): the verbs
# publish via plumbing that never runs this hook, so ANY staged
# change under plans/goals/ in an ordinary commit is a hand edit —
# and hand edits go through goal reconcile, which republishes them
# lawfully. This is the accidental-edit fence, not the authority
# boundary (the read-side validator is that). It runs BEFORE the
# new-plan acknowledgment and the unborn-HEAD exception: those two
# exits acknowledge NEW PLAN FILES and INITIAL PAYLOADS, and neither
# acknowledgment says anything about the ledger (review F15 — the
# fence was bypassable through both).
ledger=$(git diff --cached --name-only | grep -E '(^|/)plans/goals/' || true)
if [[ -n "$ledger" ]]; then
  echo "pre-commit guard: goal files change only through goal verbs; hand edits go through goal reconcile:" >&2
  printf '  %s
' $ledger >&2
  exit 1
fi

[[ "${METASYSTEM_ALLOW_NEW_PLAN:-}" == "1" ]] && exit 0

# An unborn branch has no peer work to capture: the initial commit stages the
# entire payload by design (adoption and provisioning both do), and refusing
# it broke provisioning the first time the two mechanisms met.
git rev-parse --verify HEAD >/dev/null 2>&1 || exit 0

added=$(git diff --cached --name-status --diff-filter=A | cut -f2- \
  | grep -E '(^|/)plans/[^/]+\.md$' || true)
[[ -z "$added" ]] && exit 0

echo "pre-commit guard: refusing to commit NEW plan file(s):" >&2
printf '  %s\n' $added >&2
echo "A new plan in the staged set is how a peer session's file gets committed" >&2
echo "by accident (0b9ca1b). If this addition is deliberate, acknowledge it:" >&2
echo "  METASYSTEM_ALLOW_NEW_PLAN=1 git commit ..." >&2
exit 1
