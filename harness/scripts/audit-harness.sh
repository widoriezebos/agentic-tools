#!/usr/bin/env bash
set -euo pipefail

root=${1:-.}
cd "$root"

required=(AGENTS.md wow.md docs/project-rules.md docs/orchestration.md docs/design/design-principles.md docs/design/design-obligation-gate.md)
for file in "${required[@]}"; do
  [[ -f "$file" ]] || { echo "missing required file: $file" >&2; exit 1; }
done

outside_pattern='/'"Users/"'|\.\.'"/"
if rg -n "$outside_pattern" . --glob '!scripts/audit-harness.sh'; then
  echo "references outside the harness are forbidden" >&2
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

max_words=${HARNESS_MAX_ALWAYS_LOADED_WORDS:-1400}
words=$(cat AGENTS.md wow.md | wc -w | tr -d ' ')
(( words <= max_words )) || { echo "always-loaded instructions exceed $max_words words" >&2; exit 1; }

echo "harness audit passed"
