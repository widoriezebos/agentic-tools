#!/usr/bin/env bash
set -euo pipefail

root=${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)}
[[ -d "$root" ]] || { echo "skill inventory root is not a directory: $root" >&2; exit 2; }
cd "$root"

# Every present skill is valid. Empty skill directories are errors rather
# than silently disappearing from the SKILL.md walk.
for dir in skills optional-skills; do
  [[ -d "$dir" ]] || continue
  for skill_dir in "$dir"/*/; do
    [[ -d "$skill_dir" ]] || continue
    [[ -f "${skill_dir}SKILL.md" ]] \
      || { echo "skill directory without SKILL.md: ${skill_dir%/}" >&2; exit 1; }
  done
  while IFS= read -r skill_md; do
    scripts/validate-skill.sh "$(dirname "$skill_md")"
  done < <(find "$dir" -name SKILL.md | LC_ALL=C sort)
done
