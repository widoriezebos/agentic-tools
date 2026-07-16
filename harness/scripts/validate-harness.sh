#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

scripts/audit-harness.sh .

scripts/validate-skill.sh skills/take-a-step-back
scripts/validate-skill.sh skills/verify
scripts/validate-skill.sh skills/refactor
scripts/validate-skill.sh skills/improve
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
  skills/improve/SKILL.md \
  skills/improve/agents/claude.md \
  skills/improve/agents/devin/AGENT.md \
  scripts/refactor-baseline.sh \
  scripts/frontier.sh \
  scripts/receipt.sh \
  optional-skills/debug-java/SKILL.md \
  docs/project-adaptation.md \
  docs/harness-reconciliation.md \
  docs/working-modes.md \
  docs/working-with-agents.md \
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

(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --eval "declared eval" >/dev/null)
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79 >/dev/null 2>&1); then
  echo "frontier challenge accepted a score below the frontier" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 --min-delta 1 >/dev/null 2>&1); then
  echo "frontier challenge ignored the noise floor" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 82 --min-delta 1 >/dev/null)
git -C "$repo" add plans/frontier
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm frontier
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 75 --eval "declared eval" >/dev/null 2>&1); then
  echo "frontier record accepted a regression without --force" >&2
  exit 1
fi

rfile="$tmp/receipts.log"
scripts/receipt.sh add --type implement --outcome shipped --file "$rfile" >/dev/null
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

echo "harness validation passed"
