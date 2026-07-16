#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

scripts/audit-harness.sh .

# Validate every skill present, including project-added and moved optional
# skills, so this script holds in adopted repositories as well as the template.
for dir in skills optional-skills; do
  [[ -d "$dir" ]] || continue
  while IFS= read -r skill_md; do
    scripts/validate-skill.sh "$(dirname "$skill_md")"
  done < <(find "$dir" -name SKILL.md | sort)
done

for link in \
  docs/project-rules.md \
  docs/orchestration.md \
  docs/collaboration.md \
  docs/design/design-principles.md \
  docs/design/design-obligation-gate.md \
  docs/examples/design-obligation-matrix.md \
  docs/examples/step-back-ledger.md \
  .gitattributes \
  plans/instruction-ledger.md \
  skills/take-a-step-back/SKILL.md \
  skills/take-a-step-back/agents/claude-profile.md \
  skills/take-a-step-back/agents/devin/AGENT.md \
  skills/take-a-step-back/agents/openai.yaml \
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
  skills/retro/agents/openai.yaml \
  scripts/refactor-baseline.sh \
  scripts/frontier.sh \
  scripts/receipt.sh \
  scripts/enforcement/github-actions-harness.yml \
  scripts/enforcement/claude-code-hooks.json \
  scripts/assert-stop-loss.sh \
  docs/project-adaptation.md \
  docs/harness-reconciliation.md \
  docs/working-modes.md \
  docs/working-with-agents.md \
  plans/README.md; do
  [[ -e "$link" ]] || { echo "missing routed asset: $link" >&2; exit 1; }
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

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
| OBL-1 | HIGH | Requirement | Behavior | Owner | Code | Test | Not applicable | DONE | None |
EOF
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/good.md" >/dev/null

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
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm frontier
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

scripts/assert-stop-loss.sh --file docs/examples/step-back-ledger.md >/dev/null
printf '### Cycle C1\n- Classification: no-progress\n### Cycle C2\n- Classification: no-progress\n' >"$tmp/stuck.md"
if scripts/assert-stop-loss.sh --file "$tmp/stuck.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed a third cycle after two no-progress results" >&2
  exit 1
fi
printf '- Cycle budget: 2\n### Cycle C1\n- Classification: contract-improved\n### Cycle C2\n- Classification: falsified-continue\n' >"$tmp/spent.md"
if scripts/assert-stop-loss.sh --file "$tmp/spent.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted cycle budget" >&2
  exit 1
fi
printf '### Cycle C1\n- Classification: falsified-dead-end\n' >"$tmp/deadend.md"
if scripts/assert-stop-loss.sh --file "$tmp/deadend.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed cycles after a dead end" >&2
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
printf '1|1970-01-01T00:00:01Z|RETRO|note=aged\n' >"$tmp/receipts-aged.log"
scripts/receipt.sh check --max-age-days 0 --file "$tmp/receipts-aged.log" >/dev/null || {
  echo "receipt check demanded a retro over an empty period" >&2
  exit 1
}
scripts/receipt.sh add --type improve --outcome shipped --verify caught --file "$rfile" >/dev/null
scripts/receipt.sh stats --file "$rfile" | grep -q '^receipts=1$' || { echo "receipt stats miscounted the post-retro period" >&2; exit 1; }
scripts/receipt.sh stats --file "$rfile" | grep -q '^type_improve=1$' || { echo "receipt stats missed the improve type" >&2; exit 1; }
scripts/receipt.sh stats --all --file "$rfile" | grep -q '^receipts=3$' || { echo "receipt stats --all miscounted" >&2; exit 1; }

echo "harness validation passed"
