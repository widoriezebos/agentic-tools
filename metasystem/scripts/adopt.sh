#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/adopt.sh <target-dir> [--runtimes claude,devin,codex|none] \
      [--enable <optional-skill>] [--copy-skills]

First installation of the metasystem into a fresh repository. Performs the
mechanical adaptation steps: exports the payload from the template's tracked
HEAD, writes metasystem.conf for the selected runtimes, registers skills and
profiles, installs the shipped enforcement, creates the gitignored artifacts/
directory, and records the template SHA in docs/project-rules.md. What remains
manual afterwards: fill docs/project-rules.md and metasystem.conf with verified
project facts, then run
scripts/validate-metasystem.sh in the target; it must pass with zero
placeholders.

--runtimes defaults to claude. claude registers skills under .claude/skills
(symlinks unless --copy-skills), profiles under .claude/agents, and writes
.claude/settings.json with the shipped lifecycle hooks. devin registers skills under
both .agents/skills and .devin/skills and copies AGENT.md profiles under
.devin/agents, then installs its Claude-compatible hook config. codex symlinks skills under .agents/skills and installs .codex/hooks.json,
where OpenAI runtimes read each skill's agents/openai.yaml in place. none
skips every runtime registration. The CI workflow installs regardless; it is
runtime-neutral.

--enable moves the named optional skill (for example debug-java) into
skills/; unselected optional skills are not copied at all.

Refusals: a dirty template worktree (the recorded SHA must identify the
payload exactly); a target carrying any detectable foreign instruction asset
(follow docs/metasystem-reconciliation.md instead); a target with an older
installation of this metasystem (follow the upgrade path there); and payload
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
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
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

# Validate runtime names before touching anything: a typo must not leave a
# partially adopted target behind.
IFS=, read -ra selected_runtimes <<<"$runtimes"
for rt in "${selected_runtimes[@]}"; do
  case "$rt" in
    claude|devin|codex|none) ;;
    *) die 2 "unknown runtime: $rt (claude, devin, codex, or none)" ;;
  esac
done
if [[ "$runtimes" == *none* && "$runtimes" != none ]]; then
  die 2 "--runtimes none cannot be combined with other runtimes"
fi
if [[ "$runtimes" != none ]]; then
  unique_runtimes=$(printf '%s\n' "${selected_runtimes[@]}" | sort -u | wc -l | tr -d ' ')
  [[ "$unique_runtimes" -eq "${#selected_runtimes[@]}" ]] \
    || die 2 "--runtimes contains a duplicate runtime"
fi

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
    if [[ -f "$target/.github/workflows/metasystem.yml" ]] \
      && (cd "$target" && METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS=1 bash scripts/audit-metasystem.sh . >/dev/null 2>&1); then
      echo "target is already this template's installation; nothing to do"
      exit 0
    fi
    die 1 "target carries this template's marker but is not a complete healthy installation (missing workflow or failing structural audit); finish it manually per docs/project-adaptation.md or start from a clean target"
  fi
  die 1 "target carries an installation at another SHA; follow the upgrade path in docs/metasystem-reconciliation.md"
fi
for asset in AGENTS.md CLAUDE.md GEMINI.md wow.md .cursorrules .cursor/rules \
  .github/copilot-instructions.md .windsurfrules .claude .devin .agents skills; do
  [[ -e "$target/$asset" ]] \
    && die 1 "target contains an existing instruction asset ($asset); follow docs/metasystem-reconciliation.md instead"
done
[[ -e "$target/.github/workflows/metasystem.yml" ]] \
  && die 1 "target already has .github/workflows/metasystem.yml; follow docs/metasystem-reconciliation.md instead"

echo "note: only file-shaped instruction assets are detectable. Confirm by hand that no agent-directed prose, prompt directories, or agent-encoding hooks/CI exist before treating this repository as fresh."

# Stage the payload from the tracked HEAD.
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
# A tree path after the colon resolves relative to the current directory, so
# archiving HEAD:<prefix> from inside the prefix silently yields an empty
# archive with exit 0. Every fixture stages the template at a repository root
# (empty prefix), which is why this survived until the first real adoption from
# the vendored layout. Archive from the toplevel, where the prefix means what
# it says.
toplevel=$(git -C "$root" rev-parse --show-toplevel)
if [[ -n "$prefix" ]]; then
  git -C "$toplevel" archive "HEAD:${prefix%/}" >"$stage/payload.tar"
else
  git -C "$root" archive HEAD >"$stage/payload.tar"
fi
tar -xf "$stage/payload.tar" -C "$stage"
rm -f "$stage/payload.tar"
# benchmark/ is the measuring kit: it measures the metasystem rather than
# serving adopted projects, and it carries every spec's HELD-OUT grader. A
# The payload is an allowlist: what is not named here does not ship. Three
# leaks in one day came from ship-by-default — the held-out grader, the
# development ledgers, and the roster — and an allowlist inverts the failure
# mode: a forgotten new file stays home instead of going out.
payload_allow=".gitattributes .gitignore AGENTS.md CLAUDE.md docs metasystem.conf optional-skills plans scripts skills wow.md"
for entry in "$stage"/* "$stage"/.[!.]*; do
  [[ -e "$entry" || -L "$entry" ]] || continue
  keep=0
  for allowed in $payload_allow; do
    [[ "$(basename "$entry")" == "$allowed" ]] && { keep=1; break; }
  done
  (( keep )) || rm -rf "$entry"
done
# plans/ ships its README and FRESH ledgers, never this repository's. Shipping
# the real instruction ledger and known-issues register was an early deliberate
# choice ("standing ledgers") and it was wrong: an adopted project starts with
# its own history, and a benchmark run must not read its builders our lessons.
for p in "$stage"/plans/*; do
  case "$(basename "$p")" in README.md) ;; *) rm -rf "$p" ;; esac
done
cat >"$stage/plans/instruction-ledger.md" <<'SKELETON'
# Instruction Ledger

Standing ledger of instruction changes adopted by retros (`skills/retro/SKILL.md`). Rows enter as `ADOPTED` with `Review by` naming the next retro; that retro replaces the status with a verdict: `KEPT`, `KEPT-UNPROVEN`, `AMENDED`, or `REVERTED`. Two consecutive `KEPT-UNPROVEN` verdicts revert by default.

| Id | Retro | Change | Owner doc | Evidence pattern | Expected effect | Review by | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
SKELETON
cat >"$stage/plans/known-issues.md" <<'SKELETON'
# Known Issues

Accepted defects and risks, each with its consequence and the condition that reopens it. A row here is a decision, not a backlog item: it says this project knows and accepts the issue until the stated condition changes.

| Id | Date | Issue | Consequence | Reopen when | Status |
| --- | --- | --- | --- | --- | --- |
SKELETON
for s in ${enable_skills[@]+"${enable_skills[@]}"}; do
  [[ -d "$stage/optional-skills/$s" ]] || die 2 "unknown optional skill: $s"
  mv "$stage/optional-skills/$s" "$stage/skills/$s"
done
rm -rf "$stage/optional-skills"

# Tailor the committed configuration template before collision checks and
# copying. The selected-runtime list is durable state, and no unselected
# runtime's model placeholder or mode override may leak into the adopted
# repository.
conf="$stage/metasystem.conf"
[[ -f "$conf" ]] || die 1 "payload is missing metasystem.conf"
"$ms" config tailor --conf "$conf" --runtimes "$runtimes"

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
  || die 1 "target already contains $collisions differing payload path(s); resolve them or follow docs/metasystem-reconciliation.md"

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

[[ -f "$target/metasystem.conf" && -f "$target/scripts/agents/dispatch.sh" ]] \
  || die 1 "adopted payload is missing metasystem.conf or scripts/agents/"

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

for rt in "${selected_runtimes[@]}"; do
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
      mkdir -p "$target/.agents/skills" "$target/.devin/skills"
      for n in ${skill_names[@]+"${skill_names[@]}"}; do
        [[ -e "$target/.agents/skills/$n" ]] || register_skill_dir "$target/.agents/skills" "$n"
        [[ -e "$target/.devin/skills/$n" ]] || register_skill_dir "$target/.devin/skills" "$n"
        if [[ -f "$target/skills/$n/agents/devin/AGENT.md" ]]; then
          mkdir -p "$target/.devin/agents/$n"
          cp "$target/skills/$n/agents/devin/AGENT.md" "$target/.devin/agents/$n/AGENT.md"
        fi
      done
      cp "$target/scripts/enforcement/devin-hooks.json" "$target/.devin/config.json"
      ;;
    codex)
      mkdir -p "$target/.agents/skills" "$target/.codex"
      for n in ${skill_names[@]+"${skill_names[@]}"}; do
        [[ -e "$target/.agents/skills/$n" ]] || register_skill_dir "$target/.agents/skills" "$n"
      done
      cp "$target/scripts/enforcement/codex-hooks.json" "$target/.codex/hooks.json"
      ;;
    none)
      ;;
  esac
done

# Runtime-neutral enforcement.
mkdir -p "$target/.github/workflows"
cp "$target/scripts/enforcement/github-actions-metasystem.yml" "$target/.github/workflows/metasystem.yml"

# Structural check now; the placeholder check waits for the facts.
(cd "$target" && METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS=1 bash scripts/audit-metasystem.sh . >/dev/null) \
  || die 1 "structural audit failed in the adopted target"

echo "adopted at template SHA $sha"
echo "finish the adoption:"
echo "  1. Fill docs/project-rules.md with verified project facts (commands, invariants, budgets, reserved decisions)."
echo "  2. Fill metasystem.conf with verified models, tiers, and the durable evidence root."
# --git-common-dir answers relative to the target, so resolve it there or the
# hook lands wherever this script happens to be standing (it once created a
# stray .git inside the template itself).
# A target need not be a git repository yet; the guard only makes sense once
# it is, so a non-repository target skips the install with a note instead of
# killing the adoption.
if hook_common=$(cd "$target" && git rev-parse --path-format=absolute --git-common-dir 2>/dev/null); then
  hook_dir="$hook_common/hooks"
  if [[ ! -e "$hook_dir/pre-commit" ]]; then
    mkdir -p "$hook_dir"
    printf '#!/usr/bin/env bash\nguard="$(git rev-parse --show-toplevel)/scripts/agents/pre-commit-guard.sh"\n[[ -x "$guard" ]] && exec "$guard"\nexit 0\n' >"$hook_dir/pre-commit"
    chmod +x "$hook_dir/pre-commit"
  else
    echo "pre-commit hook already present in the target; the new-plan guard was NOT installed over it" >&2
  fi
else
  echo "target is not a git repository; the new-plan guard hook was not installed (install it when git init happens)" >&2
fi
echo "  3. Run scripts/validate-metasystem.sh in the target; it must pass with zero placeholders."
