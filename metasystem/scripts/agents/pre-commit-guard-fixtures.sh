#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-pre-commit-guard.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
fixture_root="$tmp/metasystem"
repository="$tmp/repository"
mkdir -p "$fixture_root/scripts/agents" "$repository/plans"
cp "$root/scripts/agents/pre-commit-guard.sh" "$fixture_root/scripts/agents/pre-commit-guard.sh"
cat >"$fixture_root/scripts/agents/worktree-lease.py" <<'SH'
#!/usr/bin/env bash
# A malformed or unreadable registry makes strict classification refuse.
exit 1
SH
chmod +x "$fixture_root/scripts/agents/worktree-lease.py"

git -C "$repository" init -q
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

echo "pre-commit guard fixtures: PASSED"
