#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/adopt.sh <target-dir> [--runtimes claude,devin,codex|none] \
      [--enable <optional-skill>] [--copy-skills]

First installation of the harness into a fresh repository. Performs the
mechanical adaptation steps: exports the payload from the template's tracked
HEAD, registers skills and profiles for the selected runtimes, installs the
shipped enforcement, creates the gitignored artifacts/ directory, and records
the template SHA in docs/project-rules.md. What remains manual afterwards:
fill docs/project-rules.md with verified project facts, then run
scripts/validate-harness.sh in the target; it must pass with zero
placeholders.

--runtimes defaults to claude. claude registers skills under .claude/skills
(symlinks unless --copy-skills), profiles under .claude/agents, and writes
.claude/settings.json with the shipped Stop hook. devin copies AGENT.md
profiles under .devin/agents. codex symlinks skills under .agents/skills,
where OpenAI runtimes read each skill's agents/openai.yaml in place. none
skips every runtime registration. The CI workflow installs regardless; it is
runtime-neutral.

--enable moves the named optional skill (for example debug-java) into
skills/; unselected optional skills are not copied at all.

Refusals: a dirty template worktree (the recorded SHA must identify the
payload exactly); a target carrying any detectable foreign instruction asset
(follow docs/harness-reconciliation.md instead); a target with an older
installation of this harness (follow the upgrade path there); and payload
paths that already exist in the target with different content. A target that
is already this template's own installation at the same SHA is a no-op.

A script can detect only file-shaped instruction assets. Agent-directed
prose in READMEs, prompt directories under other names, and hooks or CI
encoding agent rules must be checked by a human before calling a repository
fresh.

Exit codes: 0 adopted or already adopted; 1 refused; 2 usage or environment error.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
target=
runtimes=claude
copy_skills=0
enable_skills=()

while (($#)); do
  case "$1" in
    --runtimes) runtimes=${2:-}; shift 2 ;;
    --enable) enable_skills+=("${2:-}"); shift 2 ;;
    --copy-skills) copy_skills=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -*) usage; exit 2 ;;
    *)
      [[ -z "$target" ]] || { usage; exit 2; }
      target=$1; shift ;;
  esac
done
[[ -n "$target" ]] || { usage; exit 2; }
[[ -n "$runtimes" ]] || { usage; exit 2; }

mkdir -p "$target"
target=$(cd "$target" && pwd -P)

# Source provenance: the payload is exported from the tracked HEAD, never the
# filesystem, so ignored or untracked content cannot ride along; a dirty
# worktree is refused so the recorded SHA identifies the payload exactly.
git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1 \
  || die 2 "template source is not a git checkout: $root"
[[ -z "$(git -C "$root" status --porcelain -- .)" ]] \
  || die 1 "template worktree is dirty; commit first so the recorded SHA identifies the copied payload"
sha=$(git -C "$root" rev-parse HEAD)
prefix=$(git -C "$root" rev-parse --show-prefix)

# Target recognition: our own installation is a no-op at the same SHA (or the
# unreplaced placeholder right after a first run); an older installation goes
# to the upgrade path; any other detectable instruction asset goes to
# reconciliation.
rules="$target/docs/project-rules.md"
if [[ -f "$target/wow.md" && -f "$rules" ]] && grep -q '^- Adopted from template SHA:' "$rules"; then
  recorded_line=$(grep '^- Adopted from template SHA:' "$rules" | head -1)
  if [[ "$recorded_line" == *"<template sha>"* || "$recorded_line" == *"$sha"* ]]; then
    echo "target is already this template's installation; nothing to do"
    exit 0
  fi
  die 1 "target carries an installation at another SHA; follow the upgrade path in docs/harness-reconciliation.md"
fi
for asset in AGENTS.md CLAUDE.md GEMINI.md wow.md .cursorrules .cursor/rules \
  .github/copilot-instructions.md .windsurfrules .claude .devin .agents skills; do
  [[ -e "$target/$asset" ]] \
    && die 1 "target contains an existing instruction asset ($asset); follow docs/harness-reconciliation.md instead"
done
[[ -e "$target/.github/workflows/harness.yml" ]] \
  && die 1 "target already has .github/workflows/harness.yml; follow docs/harness-reconciliation.md instead"

echo "note: only file-shaped instruction assets are detectable. Confirm by hand that no agent-directed prose, prompt directories, or agent-encoding hooks/CI exist before treating this repository as fresh."

# Stage the payload from the tracked HEAD.
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
if [[ -n "$prefix" ]]; then treeish="HEAD:${prefix%/}"; else treeish=HEAD; fi
git -C "$root" archive "$treeish" >"$stage/payload.tar"
tar -xf "$stage/payload.tar" -C "$stage"
rm -f "$stage/payload.tar"
rm -rf "$stage/meta" "$stage/README.md" "$stage/LICENSE"
# plans/ ships only its standing ledgers: task-local plans and handoff notes
# are template-repository state, and receipts.log is its receipts history.
for p in "$stage"/plans/*; do
  case "$(basename "$p")" in
    README.md|instruction-ledger.md|known-issues.md) ;;
    *) rm -rf "$p" ;;
  esac
done
for s in ${enable_skills[@]+"${enable_skills[@]}"}; do
  [[ -d "$stage/optional-skills/$s" ]] || die 2 "unknown optional skill: $s"
  mv "$stage/optional-skills/$s" "$stage/skills/$s"
done
rm -rf "$stage/optional-skills"

# Collision policy: .gitattributes and .gitignore merge by line-append; every
# other payload path that exists in the target with different content refuses,
# never overwriting and never skipping silently.
collisions=0
while IFS= read -r p; do
  rel=${p#"$stage"/}
  case "$rel" in .gitattributes|.gitignore) continue ;; esac
  if [[ -e "$target/$rel" ]] && ! cmp -s "$p" "$target/$rel"; then
    echo "collision: $rel" >&2
    collisions=$((collisions + 1))
  fi
done < <(find "$stage" -type f)
(( collisions == 0 )) \
  || die 1 "target already contains $collisions differing payload path(s); resolve them or follow docs/harness-reconciliation.md"

# Copy the payload, then merge the line-append files.
(cd "$stage" && find . -type f ! -name .gitattributes ! -name .gitignore -print0) \
  | while IFS= read -r -d '' p; do
      rel=${p#./}
      mkdir -p "$target/$(dirname "$rel")"
      cp "$stage/$rel" "$target/$rel"
    done
for mf in .gitattributes .gitignore; do
  [[ -f "$stage/$mf" ]] || continue
  touch "$target/$mf"
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    grep -qxF "$line" "$target/$mf" || echo "$line" >>"$target/$mf"
  done <"$stage/$mf"
done

mkdir -p "$target/artifacts"
touch "$target/.gitignore"
grep -qxF 'artifacts/' "$target/.gitignore" || echo 'artifacts/' >>"$target/.gitignore"

sed "s|<template sha>|$sha|" "$rules" >"$rules.new" && mv "$rules.new" "$rules"

# Runtime registrations.
register_skill_dir() { # $1 = runtime dir for skill links, $2 = skill name
  if (( copy_skills )); then
    cp -R "$target/skills/$2" "$1/$2"
  else
    ln -s "../../skills/$2" "$1/$2"
  fi
}

skill_names=()
for d in "$target"/skills/*/; do
  [[ -d "$d" ]] || continue
  skill_names+=("$(basename "$d")")
done

IFS=, read -ra selected <<<"$runtimes"
for rt in "${selected[@]}"; do
  case "$rt" in
    claude)
      mkdir -p "$target/.claude/skills" "$target/.claude/agents"
      for n in ${skill_names[@]+"${skill_names[@]}"}; do
        [[ -e "$target/.claude/skills/$n" ]] || register_skill_dir "$target/.claude/skills" "$n"
        [[ -f "$target/skills/$n/agents/claude-profile.md" ]] \
          && cp "$target/skills/$n/agents/claude-profile.md" "$target/.claude/agents/$n.md"
      done
      sed '/"_comment"/d' "$target/scripts/enforcement/claude-code-hooks.json" >"$target/.claude/settings.json"
      ;;
    devin)
      for n in ${skill_names[@]+"${skill_names[@]}"}; do
        if [[ -f "$target/skills/$n/agents/devin/AGENT.md" ]]; then
          mkdir -p "$target/.devin/agents/$n"
          cp "$target/skills/$n/agents/devin/AGENT.md" "$target/.devin/agents/$n/AGENT.md"
        fi
      done
      ;;
    codex)
      mkdir -p "$target/.agents/skills"
      for n in ${skill_names[@]+"${skill_names[@]}"}; do
        [[ -e "$target/.agents/skills/$n" ]] || register_skill_dir "$target/.agents/skills" "$n"
      done
      ;;
    none)
      ;;
    *)
      die 2 "unknown runtime: $rt (claude, devin, codex, or none)"
      ;;
  esac
done

# Runtime-neutral enforcement.
mkdir -p "$target/.github/workflows"
cp "$target/scripts/enforcement/github-actions-harness.yml" "$target/.github/workflows/harness.yml"

# Structural check now; the placeholder check waits for the facts.
(cd "$target" && HARNESS_AUDIT_ALLOW_PLACEHOLDERS=1 bash scripts/audit-harness.sh . >/dev/null) \
  || die 1 "structural audit failed in the adopted target"

echo "adopted at template SHA $sha"
echo "finish the adoption:"
echo "  1. Fill docs/project-rules.md with verified project facts (commands, invariants, budgets, reserved decisions)."
echo "  2. Run scripts/validate-harness.sh in the target; it must pass with zero placeholders."
