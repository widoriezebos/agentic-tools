#!/usr/bin/env bash
set -euo pipefail

skill=${1:-}
[[ -n "$skill" && -d "$skill" ]] || { echo "usage: $0 <skill-directory>" >&2; exit 2; }
file=$skill/SKILL.md
[[ -f "$file" ]] || { echo "missing $file" >&2; exit 1; }

first=$(sed -n '1p' "$file")
[[ "$first" == '---' ]] || { echo "$file: frontmatter must start on line 1" >&2; exit 1; }
closing=$(awk 'NR > 1 && $0 == "---" { print NR; exit }' "$file")
[[ -n "$closing" ]] || { echo "$file: frontmatter is not closed" >&2; exit 1; }

frontmatter=$(sed -n "2,$((closing - 1))p" "$file")
name=$(awk -F': *' '$1 == "name" {sub(/^name: */, ""); print; exit}' <<<"$frontmatter")
description=$(awk -F': *' '$1 == "description" {sub(/^description: */, ""); print; exit}' <<<"$frontmatter")

[[ -n "$name" ]] || { echo "$file: missing name" >&2; exit 1; }
[[ -n "$description" ]] || { echo "$file: missing description" >&2; exit 1; }
[[ "$name" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]] || { echo "$file: invalid skill name" >&2; exit 1; }
[[ ${#name} -le 64 ]] || { echo "$file: skill name exceeds 64 characters" >&2; exit 1; }
[[ "$(basename "$skill")" == "$name" ]] || { echo "$file: folder and skill name differ" >&2; exit 1; }
# Extra frontmatter keys are allowed: runtimes add their own (allowed-tools,
# model, ...) and project skills must not fail validation for using them.
[[ -s "$file" ]] || { echo "$file: empty skill" >&2; exit 1; }
echo "$name is valid"
