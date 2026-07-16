#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

scripts/audit-harness.sh .

scripts/validate-skill.sh skills/take-a-step-back
scripts/validate-skill.sh skills/verify
scripts/validate-skill.sh skills/refactor
scripts/validate-skill.sh optional-skills/debug-java

for link in \
  docs/project-rules.md \
  docs/orchestration.md \
  docs/collaboration.md \
  docs/design/design-principles.md \
  docs/design/design-obligation-gate.md \
  docs/examples/design-obligation-matrix.md \
  docs/examples/step-back-ledger.md \
  skills/take-a-step-back/SKILL.md \
  skills/take-a-step-back/agents/claude.md \
  skills/take-a-step-back/agents/devin/AGENT.md \
  skills/verify/SKILL.md \
  skills/verify/agents/claude.md \
  skills/verify/agents/devin/AGENT.md \
  skills/refactor/SKILL.md \
  skills/refactor/agents/claude.md \
  skills/refactor/agents/devin/AGENT.md \
  scripts/refactor-baseline.sh \
  optional-skills/debug-java/SKILL.md \
  docs/project-adaptation.md \
  plans/README.md; do
  [[ -e "$link" ]] || { echo "missing routed asset: $link" >&2; exit 1; }
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
touch "$tmp/source" "$tmp/artifact"
optional-skills/debug-java/scripts/preflight.sh --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null

cat >"$tmp/good.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | HIGH | Requirement | Behavior | Owner | Code | Test | Not applicable | DONE | None |
EOF
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/good.md" >/dev/null

sed 's/| DONE |/| MISSING |/' "$tmp/good.md" >"$tmp/bad.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bad.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a missing high obligation" >&2
  exit 1
fi

scripts/assert-design-obligation-gate.sh --runtime-required --file docs/examples/design-obligation-matrix.md >/dev/null 2>&1 && {
  echo "example matrix with READY_FOR_RUNTIME passed --runtime-required; negative fixture broken" >&2
  exit 1
}
scripts/assert-design-obligation-gate.sh --file docs/examples/design-obligation-matrix.md >/dev/null

repo="$tmp/baseline-repo"
git init -q "$repo"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit --allow-empty -qm base
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" >/dev/null)
git -C "$repo" add plans/refactor-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm baseline
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

echo "harness validation passed"
