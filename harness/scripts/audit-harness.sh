#!/usr/bin/env bash
set -euo pipefail

root=${1:-.}
cd "$root"

command -v rg >/dev/null || { echo "ripgrep (rg) is required; without it the reference and placeholder checks cannot run" >&2; exit 2; }

required=(AGENTS.md wow.md docs/project-rules.md docs/orchestration.md docs/collaboration.md docs/design/design-principles.md docs/design/design-obligation-gate.md)
for file in "${required[@]}"; do
  [[ -f "$file" ]] || { echo "missing required file: $file" >&2; exit 1; }
done

# Scope the outside-reference check to harness-owned files ONLY, by explicit
# list rather than by directory: in adopted repositories docs/ and scripts/
# also hold project-owned files that legitimately contain dot-dot path
# segments (script root resolution) or absolute paths (project registers,
# frozen histories). docs/project-rules.md is project-owned and deliberately
# excluded, and so is this script itself: explicit file arguments bypass rg
# glob filters, so it must not appear in the scan list at all.
outside_pattern='/'"Users/"'|\.\.'"/"
scan=()
for p in AGENTS.md CLAUDE.md wow.md \
  docs/orchestration.md docs/collaboration.md docs/working-modes.md \
  docs/working-with-agents.md docs/project-adaptation.md docs/harness-reconciliation.md \
  docs/design/design-principles.md docs/design/design-obligation-gate.md docs/examples \
  skills optional-skills meta \
  scripts/validate-harness.sh scripts/validate-skill.sh \
  scripts/assert-design-obligation-gate.sh scripts/refactor-baseline.sh scripts/frontier.sh \
  scripts/receipt.sh scripts/assert-stop-loss.sh scripts/enforcement \
  plans/README.md plans/instruction-ledger.md; do
  [[ -e "$p" ]] && scan+=("$p")
done
if rg -n "$outside_pattern" "${scan[@]}"; then
  echo "references outside the harness are forbidden in harness-owned files" >&2
  exit 1
fi

echo "Always-loaded words"
wc -w AGENTS.md wow.md
echo "Skill inventory"
skill_dirs=(skills)
[[ -d optional-skills ]] && skill_dirs+=(optional-skills)
find "${skill_dirs[@]}" -name SKILL.md -print | sort
echo "Instruction inventory"
find . -type f \( -name 'AGENTS.md' -o -name 'CLAUDE.md' -o -name 'wow.md' -o -name 'SKILL.md' -o -name 'AGENT.md' \) -print | sort

if rg -n 'TODO|TBD|<one paragraph>|<command>|<paths' AGENTS.md wow.md "${skill_dirs[@]}"; then
  echo "unresolved placeholders in active instructions" >&2
  exit 1
fi

# Template detection uses a positive marker, not the mere absence of a meta/
# directory (any project may have one of those): only the template repository
# carries meta/harness-design.md. Everywhere else, project-rules.md must be
# filled in.
if [[ ! -f meta/harness-design.md ]]; then
  if rg -n '<[a-z][a-z -]*>' docs/project-rules.md; then
    echo "adopted repository has unreplaced placeholders in docs/project-rules.md" >&2
    exit 1
  fi
fi

max_words=${HARNESS_MAX_ALWAYS_LOADED_WORDS:-1400}
words=$(cat AGENTS.md wow.md | wc -w | tr -d ' ')
(( words <= max_words )) || { echo "always-loaded instructions exceed $max_words words" >&2; exit 1; }

echo "harness audit passed"
