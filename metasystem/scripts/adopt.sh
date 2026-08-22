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

# Every probe of the TARGET's repository runs with git's steering env
# scrubbed: an inherited GIT_DIR or config override must not make the
# probe answer for some other repository.
git_target() {
  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR -u GIT_INDEX_FILE \
    -u GIT_CEILING_DIRECTORIES -u GIT_OBJECT_DIRECTORY \
    -u GIT_ALTERNATE_OBJECT_DIRECTORIES -u GIT_CONFIG \
    -u GIT_CONFIG_PARAMETERS -u GIT_CONFIG_COUNT -u GIT_CONFIG_GLOBAL \
    -u GIT_CONFIG_SYSTEM -u GIT_CONFIG_NOSYSTEM -u GIT_GRAFT_FILE \
    -u GIT_SHALLOW_FILE -u GIT_REPLACE_REF_BASE git "$@"
}

# A hook enrolls the guard only if running it PROVES the guard runs:
# the hook chain is executed with a probe nonce and must return the
# guard's acknowledgment. Static reading of shell is an arms race the
# fence cannot afford to lose.
enrolls_guard() {
  local hook="$1" nonce out
  [[ -x "$hook" ]] || return 1
  nonce="probe-$$-$RANDOM$RANDOM"
  out=$(cd "$target" && METASYSTEM_GUARD_PROBE="$nonce" "$hook" 2>/dev/null) || true
  [[ "$out" == *"guard-probe-ack $nonce"* ]]
}

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

# The Go toolchain is an adoption prerequisite (go-production-grade Phase
# 0a): the engine is always rebuilt from source, never copied on trust, so
# a Go-less machine must refuse HERE — before any target mutation — not
# fail halfway through with the payload already installed.
command -v go >/dev/null 2>&1 \
  || die 1 "adoption requires the Go toolchain: the engine is always rebuilt from the template source; install Go and re-run"
bash "$root/scripts/agents/preflight-commands.sh" \
  || die 1 "adoption refused: install the named commands first"

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

# Enrollment feasibility is proven BEFORE any target mutation:
# adoption itself performs goal mutations (genesis), whose enrollment
# refuses a target carrying both pre-commit and pre-commit.local
# without the guard — discovering that AFTER the payload landed would
# leave a half-adopted repository behind the same refusal. Same
# message, zero writes.
if probe_out=$(git_target -C "$target" rev-parse --is-inside-work-tree 2>&1); then
  pre_hook_dir=$(git_target -C "$target" rev-parse --path-format=absolute --git-path hooks)
  if [[ -e "$pre_hook_dir/pre-commit" && -e "$pre_hook_dir/pre-commit.local" ]] \
    && ! enrolls_guard "$pre_hook_dir/pre-commit"; then
    die 1 "target carries both pre-commit and pre-commit.local and neither enrolls the guard; compose them by hand, then re-run adoption"
  fi
elif [[ "$probe_out" != *"not a git repository"* ]]; then
  # Exit 128 alone is not proof of a missing repository: a malformed
  # configuration in a VALID repository fails the same way, and
  # adopting through that would strand a half-adoption behind the
  # goal CLI's own refusal later.
  die 1 "target's repository shape cannot be proven: $probe_out"
fi

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
# cmd/, internal/, go.mod, go.sum are the engine source (D17): the payload
# ships source and CI rebuilds, so the pair of scripts and engine can prove
# its coherence by building. The adoption-time binary copy into gitignored
# bin/ remains a host convenience, never the delivery.
payload_allow=".gitattributes .gitignore AGENTS.md CLAUDE.md cmd docs go.mod go.sum internal metasystem.conf optional-skills plans scripts skills wow.md"
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
# The goal ledger ships as a PAIR with its accepted baseline (goal-system
# GOAL-14: initialization is reconcile-only, and adoption is the one other
# legal genesis — both files together, never a ledger without its
# baseline). The declaration's scan digest reproduces goal.ScanDigest's
# exact rule — sorted plans/*.md basenames minus goals.md, newline-joined,
# no trailing newline — over the seeded set. A live example goal would
# parse as real work, so the skeleton declares goal-free instead.
goal_scan_digest=$(printf '%s' "README.md
instruction-ledger.md
known-issues.md" | shasum -a 256 | cut -d' ' -f1)
cat >"$stage/plans/goals.md" <<GOALSKELETON
# Goals

## Goal-free: declared $(date -u +%Y-%m-%dT%H:%M:%SZ) by human over $goal_scan_digest
GOALSKELETON
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
  # The goal pair is SEED-ONCE (goal-system GOAL-14): after adoption it is
  # the project's own state (the declaration carries adoption's timestamp
  # and digest), so a re-adoption neither collides on it nor overwrites it.
  case "$rel" in plans/goals.md|plans/goals-accepted.json)
    [[ -e "$target/plans/goals.md" ]] && continue ;; esac
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
      case "$rel" in plans/goals.md|plans/goals-accepted.json)
        [[ -e "$target/plans/goals.md" ]] && continue ;; esac
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

# Adoption ships the engine: the harness scripts decide nothing without the
# metasystem binary, so a target without it would be armed but inert. The
# engine is ALWAYS rebuilt from the template source through the shared
# fenced build (go-production-grade Phase 0a) — a pre-existing bin/ may be
# stale, foreign, or unstamped, and no verb can prove freshness against a
# dirty tree, so nothing that already exists is ever copied on trust.
bash "$root/scripts/agents/go-build.sh" \
  || die 1 "could not build the metasystem engine for adoption"
mkdir -p "$target/bin"
cp "$root/bin/metasystem" "$target/bin/metasystem"
chmod +x "$target/bin/metasystem"

# The goal baseline is written by the engine's ONE genesis authority path
# (goal-system GOAL-14: initialization is reconcile-only): the seeded
# ledger is adopted as the first accepted state, leaving the
# goals.md + goals-accepted.json pair standing together. The caller is
# classified against the TARGET — the root being written, like every
# goal verb — and genesis admits any non-holder for exactly this shape:
# a goal-free ledger on a checkout whose history carries none. A target
# that already holds a healthy pair (its ledger matches its accepted
# baseline) needs no write at all, and reconciling it would run
# holder-only against a target that cannot know this caller — a re-run
# must not turn a no-op into a refusal.
pair_state=$("$target/bin/metasystem" goal list --root "$target" 2>/dev/null || true)
if [[ -n "$pair_state" ]] \
  && [[ $("$ms" json get --value "$pair_state" --field baselinePresent --default false) == true ]] \
  && [[ $("$ms" json get --value "$pair_state" --field baselineMatches --default false) == true ]]; then
  : # the pair is already the accepted state
else
  "$target/bin/metasystem" goal reconcile --root "$target" >/dev/null \
    || die 1 "goal baseline genesis failed in the target"
fi

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
      # Structurally, not by line-deleting JSON: the annotated enforcement
      # asset keeps its comment for humans, the runtime config never sees it.
      "$ms" json strip --file "$target/scripts/enforcement/claude-code-hooks.json" \
        --key _comment >"$target/.claude/settings.json"
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
if git_target -C "$target" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  # Same scrubbed, hooksPath-honoring resolution as the early gate:
  # raw git here could be steered into ANOTHER repository's hooks,
  # and .git/hooks is wrong wherever core.hooksPath points elsewhere.
  hook_dir=$(git_target -C "$target" rev-parse --path-format=absolute --git-path hooks)
  mkdir -p "$hook_dir"
  # The composer runs the guard FIRST, then hands off to whatever hook
  # the project already had (preserved as pre-commit.local). Declining
  # to enroll because a hook existed left the ledger fence unenforced
  # in exactly the repositories that care about their hooks (F15).
  # The guard path is prefix-aware (a target nested below its git
  # toplevel resolves under its own prefix) and the relative part
  # rides SINGLE-QUOTED so lawful path bytes never become syntax:
  # the template carries a placeholder, substituted once below.
  target_prefix=$(git_target -C "$target" rev-parse --show-prefix)
  guard_rel="${target_prefix}scripts/agents/pre-commit-guard.sh"
  guard_rel_quoted="'${guard_rel//\'/\'\\\'\'}'"
  composer='#!/usr/bin/env bash
guard="$(git rev-parse --show-toplevel)/"__GUARD_REL__
if [[ ! -x "$guard" ]]; then
  echo "pre-commit: the metasystem ledger guard is missing at $guard; refusing to commit without the fence" >&2
  exit 1
fi
"$guard" || exit $?
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ -x "$here/pre-commit.local" ]]; then
  exec "$here/pre-commit.local" "$@"
fi
exit 0
'
  composer=${composer/__GUARD_REL__/$guard_rel_quoted}
  if [[ -e "$hook_dir/pre-commit" ]] && enrolls_guard "$hook_dir/pre-commit"; then
    echo "pre-commit guard already enrolled in the target's hook; left as is"
  else
    if [[ -e "$hook_dir/pre-commit" && -e "$hook_dir/pre-commit.local" ]] \
      && ! enrolls_guard "$hook_dir/pre-commit"; then
      # Never clobber (R2-15) — and never limp on either: adoption
      # itself performs goal mutations (genesis), and the CLI's
      # enrollment refuses this exact shape, so a warn-and-continue
      # would fail later with a worse name. Refuse up front; both
      # files stay untouched.
      die 1 "target carries both pre-commit and pre-commit.local and neither enrolls the guard; compose them by hand, then re-run adoption"
    else
      if [[ -e "$hook_dir/pre-commit" ]]; then
        mv "$hook_dir/pre-commit" "$hook_dir/pre-commit.local"
        echo "existing pre-commit hook preserved as pre-commit.local; the guard now runs first"
      fi
      printf '%s' "$composer" >"$hook_dir/pre-commit"
      chmod +x "$hook_dir/pre-commit"
    fi
  fi
else
  echo "target is not a git repository; the new-plan guard hook was not installed (install it when git init happens)" >&2
fi
echo "  3. Run scripts/validate-metasystem.sh in the target; it must pass with zero placeholders."
