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
