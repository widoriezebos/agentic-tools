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
# excluded, and so are this script and scripts/adopt.sh: explicit file
# arguments bypass rg glob filters, and both legitimately contain dot-dot
# segments (root resolution, relative symlink targets), so neither may
# appear in the scan list at all.
# The path pattern anchors on a non-word, non-slash character before the
# leading slash so prose like "rule/home/owner" does not false-positive.
outside_pattern='(^|[^[:alnum:]/])/(Users|home|root|tmp|var|opt|etc|private|workspace)/|\.\.'"/"
scan=()
for p in AGENTS.md CLAUDE.md wow.md \
  docs/orchestration.md docs/collaboration.md docs/working-modes.md \
  docs/working-with-agents.md docs/project-adaptation.md docs/harness-reconciliation.md \
  docs/design/design-principles.md docs/design/design-obligation-gate.md docs/examples \
  skills optional-skills meta \
  scripts/validate-harness.sh scripts/validate-skill.sh \
  scripts/assert-design-obligation-gate.sh scripts/refactor-baseline.sh scripts/frontier.sh \
  scripts/receipt.sh scripts/assert-stop-loss.sh scripts/enforcement \
  plans/README.md plans/instruction-ledger.md plans/known-issues.md; do
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
  # Look for the template's own literal placeholders, exactly as the check on
  # AGENTS.md and the skills does above. A generic any-angle-bracket pattern
  # false-positives on legitimately parameterized commands in a filled file
  # (<port>, <pid>, <prompt> and the like), which a real project-rules is full of.
  # HARNESS_AUDIT_ALLOW_PLACEHOLDERS tolerates them for the structural check
  # scripts/adopt.sh runs right after copying, before the facts are filled in;
  # the closing validate-harness run enforces them again.
  if [[ -z "${HARNESS_AUDIT_ALLOW_PLACEHOLDERS:-}" ]]; then
    if rg -n '<one paragraph>|<command>|<paths|<policy>|<list them here>|<sources and handling>|<forbidden list>|<location>|<path outside the repository>|<amount and period>|<warning threshold>|<who approves>|<usage source>|<template sha>' docs/project-rules.md; then
      echo "adopted repository has unreplaced placeholders in docs/project-rules.md" >&2
      exit 1
    fi
  fi
fi

max_words=${HARNESS_MAX_ALWAYS_LOADED_WORDS:-1400}
words=$(cat AGENTS.md wow.md | wc -w | tr -d ' ')
(( words <= max_words )) || { echo "always-loaded instructions exceed $max_words words" >&2; exit 1; }

# Report only, uncapped: the effective per-task footprint. Project rules,
# collaboration, and the completion gate load on nearly every repo-changing
# task, and design principles on most; stating the real number keeps the
# capped always-loaded metric honest.
bundle=(AGENTS.md wow.md docs/project-rules.md docs/collaboration.md docs/design/design-obligation-gate.md docs/design/design-principles.md)
present=()
for f in "${bundle[@]}"; do
  [[ -f "$f" ]] && present+=("$f")
done
bundle_words=$(cat "${present[@]}" | wc -w | tr -d ' ')
echo "Effective common-path bundle: $bundle_words words (report only)"

echo "harness audit passed"
