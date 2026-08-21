#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-pre-commit-guard.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
fixture_root="$tmp/metasystem"
repository="$tmp/repository"
mkdir -p "$fixture_root/scripts/agents" "$repository/plans"
cp "$root/scripts/agents/pre-commit-guard.sh" "$fixture_root/scripts/agents/pre-commit-guard.sh"
# The copied guard resolves its engine as $fixture_root/bin/metasystem. A
# refusing stub there reproduces the old refusing worktree-lease.py leg:
# classification fails strictly, and the guard must fail open to HUMAN.
mkdir -p "$fixture_root/bin"
cat >"$fixture_root/bin/metasystem" <<'SH'
#!/usr/bin/env bash
# A malformed or unreadable registry makes strict classification refuse.
exit 1
SH
chmod +x "$fixture_root/bin/metasystem"

git -C "$repository" init -q -b main
printf 'tracked\n' >"$repository/tracked.txt"
git -C "$repository" add tracked.txt
git -C "$repository" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm seed
printf 'human change\n' >"$repository/tracked.txt"
git -C "$repository" add tracked.txt
(cd "$repository" && "$fixture_root/scripts/agents/pre-commit-guard.sh") || {
  echo "pre-commit guard fixture: classifier failure gated a human tracked-file commit" >&2
  exit 1
}

# The unrelated existing new-plan safeguard remains active after the human
# authority path fails open.
printf 'new plan\n' >"$repository/plans/new.md"
git -C "$repository" add plans/new.md
if (cd "$repository" && "$fixture_root/scripts/agents/pre-commit-guard.sh" \
  >"$tmp/new-plan.out" 2>"$tmp/new-plan.err"); then
  echo "pre-commit guard fixture: classifier fallback disabled the new-plan safeguard" >&2
  exit 1
fi
grep -Fq 'refusing to commit NEW plan' "$tmp/new-plan.err"

# The ledger fence outranks the new-plan acknowledgment (F15): the
# ALLOW_NEW_PLAN escape acknowledges new plan files, and says nothing
# about the goal ledger — a staged plans/goals/ change refuses even
# under the acknowledgment.
git -C "$repository" reset -q
mkdir -p "$repository/plans/goals"
printf '# smuggled\n' >"$repository/plans/goals/smuggled.md"
git -C "$repository" add plans/goals/smuggled.md
if (cd "$repository" && METASYSTEM_ALLOW_NEW_PLAN=1 "$fixture_root/scripts/agents/pre-commit-guard.sh" \
  >"$tmp/ledger-ack.out" 2>"$tmp/ledger-ack.err"); then
  echo "pre-commit guard fixture: ALLOW_NEW_PLAN bypassed the goal-ledger fence" >&2
  exit 1
fi
grep -Fq 'goal files change only through goal verbs' "$tmp/ledger-ack.err"
# The acknowledgment keeps its own meaning: a NEW plan outside the
# ledger still passes under it.
git -C "$repository" reset -q
git -C "$repository" add plans/new.md
(cd "$repository" && METASYSTEM_ALLOW_NEW_PLAN=1 "$fixture_root/scripts/agents/pre-commit-guard.sh") || {
  echo "pre-commit guard fixture: the acknowledgment no longer admits an acknowledged new plan" >&2
  exit 1
}

# The ledger fence outranks the unborn-HEAD exception too: an initial
# payload never lawfully hand-writes the ledger.
unborn="$tmp/unborn"
mkdir -p "$unborn/plans/goals"
git -C "$unborn" init -q -b main
printf '# smuggled\n' >"$unborn/plans/goals/smuggled.md"
git -C "$unborn" add plans/goals/smuggled.md
if (cd "$unborn" && "$fixture_root/scripts/agents/pre-commit-guard.sh" \
  >"$tmp/ledger-unborn.out" 2>"$tmp/ledger-unborn.err"); then
  echo "pre-commit guard fixture: the unborn-HEAD exception bypassed the goal-ledger fence" >&2
  exit 1
fi
grep -Fq 'goal files change only through goal verbs' "$tmp/ledger-unborn.err"
# The exception keeps its own meaning: an unborn initial payload
# without ledger files still passes.
git -C "$unborn" reset -q
printf 'payload\n' >"$unborn/payload.txt"
printf 'plan\n' >"$unborn/plans/initial.md"
mkdir -p "$unborn/plans"
git -C "$unborn" add payload.txt plans/initial.md
(cd "$unborn" && "$fixture_root/scripts/agents/pre-commit-guard.sh") || {
  echo "pre-commit guard fixture: the unborn-HEAD exception no longer admits an initial payload" >&2
  exit 1
}

echo "pre-commit guard fixtures: PASSED"
