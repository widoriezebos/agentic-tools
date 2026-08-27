#!/usr/bin/env bash
set -euo pipefail

# adopt.sh self-test (script-validate-4/D35): extracted verbatim from
# validate-metasystem.sh's inline template-mode blocks into the sub-suite
# shape the file already used everywhere else. Template mode only — the
# orchestrator gates the invocation; the nested targets (which lack
# development/) cannot recurse. Failure evidence is preserved exactly like
# the orchestrator preserves its own.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
cd "$root"
source scripts/agents/fixture-budget.sh
# Every adoption in this harness runs from a STERILE SNAPSHOT source
# under whatever ancestry invoked the suite — agent or terminal. Genesis
# no longer cares which: a non-holder is admitted for exactly the
# adoption shape (a goal-free ledger on a checkout whose history carries
# none), judged against the target itself, so these fixtures carry no
# invocation-shape dependence and no authority-root env hook.
tmp=$(mktemp -d)
witness_state=
cleanup() {
  status=$?
  [[ -n "$witness_state" ]] && rm -rf "$witness_state" 2>/dev/null
  if [[ $status != 0 && -d "$tmp" ]]; then
    keep="artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-adopt-$$"
    mkdir -p "$(dirname "$keep")"
    mv "$tmp" "$keep" 2>/dev/null && echo "adopt fixture evidence preserved: $keep" >&2
    return 0
  fi
  rm -rf "$tmp" 2>/dev/null || true
}
trap cleanup EXIT

# A standalone run arms the D33 witness once (battery-wall-clock lever
# 1): every nested validate this harness spawns then proves the
# identical bytes by digest check instead of re-running the race suite.
# An outer validate's witness is inherited untouched; a dirty tree arms
# nothing and the nested runs pay their own gates exactly as before
# (fallback "none": this harness needs no worktree gate of its own).
if [[ -z "${METASYSTEM_GATE_WITNESS:-}" ]] \
  && grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' go.mod; then
  delivery_contract=0
  WITNESS_GATE_FALLBACK=none source scripts/agents/witness-gate.sh
fi

fill_harness_conf() { # config path, absolute evidence root
  # Point evidence at the harness sandbox and give every rostered
  # role a fixture model, so a nested validation never resolves a
  # real model or writes evidence outside the fixture tree. Tier 1
  # then names exactly the fixture models (sorted, deduplicated) and
  # every deeper tier empties.
  local path=$1 evidence=$2 line key value runtime joined out=""
  local model_key='^(role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$'
  local models=()
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" != *=* ]]; then out+="$line"$'\n'; continue; fi
    key=${line%%=*}
    value=${line#*=}
    if [[ "$key" == evidence.root ]]; then
      value=$evidence
    elif [[ "$key" =~ $model_key ]]; then
      runtime=${BASH_REMATCH[2]}
      value="fixture-$runtime-model"
      models+=("$runtime:$value")
    elif [[ "$key" == model.tier.1 ]]; then
      value=__MODELS__
    elif [[ "$key" == model.tier.* ]]; then
      value=""
    fi
    out+="$key=$value"$'\n'
  done <"$path"
  joined=""
  if [[ ${#models[@]} -gt 0 ]]; then
    joined=$(printf '%s\n' "${models[@]}" | LC_ALL=C sort -u | paste -sd, -)
  fi
  printf '%s' "${out//model.tier.1=__MODELS__/model.tier.1=$joined}" >"$path"
}

# Adopted-mode contract: a copy without the template marker validates with a
# skill pruned, and a present-but-broken skill still fails. Template mode
# only, so the nested run (which lacks development/) cannot recurse.
if true; then  # template-gated by the orchestrator
  adopted="$tmp/adopted"
  mkdir -p "$adopted"
copy_tree_without_artifacts() { # source root, destination
  # Only for copies whose source is the live metasystem root: artifacts/ is
  # runtime state, not shipped content, and copying it races
  # with any job writing lock directories, and an adoption fixture has no use
  # for it. Excluding it makes the suite safe to run while work is in flight.
  local from=$1 to=$2 entry
  mkdir -p "$to"
  (cd "$from" && for entry in * .[!.]*; do
    [[ -e "$entry" ]] || continue
    [[ "$entry" == artifacts || "$entry" == .git || "$entry" == metasystem.conf.local ]] && continue
    cp -R "$entry" "$to/"
  done)
}

  copy_tree_without_artifacts "$root" "$adopted"
  rm -rf "$adopted/development" "$adopted/skills/improve" "$adopted/plans/receipts.log" "$adopted/.claude"
  # A real adopted installation sits inside the app's git repository; the
  # copy must too, or entrypoints that verify repo scope refuse to start.
  git -C "$adopted" init -q -b main
  sed 's/<[^>]*>/filled/g' "$adopted/docs/project-rules.md" >"$adopted/docs/project-rules.md.new"
  mv "$adopted/docs/project-rules.md.new" "$adopted/docs/project-rules.md"
  conf_edit "$adopted/metasystem.conf" replace-line-first '^metasystem[.]runtimes=.*$' 'metasystem.runtimes='
  conf_edit "$adopted/metasystem.conf" delete-lines '^role[.].*$'
  conf_edit "$adopted/metasystem.conf" delete-lines '^mode[.].*[.]role[.].*$'
  conf_edit "$adopted/metasystem.conf" delete-lines '^validate[.]extra-suites=.*$'
  fill_harness_conf "$adopted/metasystem.conf" "$tmp/adopted-evidence"
  bash "$adopted/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/nested-pruned.log" 2>&1 || {
    echo "adopted-mode validation failed for a copy with one skill pruned" >&2
    tail -20 "$tmp/nested-pruned.log" >&2
    exit 1
  }
  grep -Fq 'metasystem delivery contract validated' "$tmp/nested-pruned.log" \
    || { echo "nested-pruned run did not end on the contract verdict" >&2; exit 1; }
  mkdir "$adopted/skills/hollow"
  if bash "$adopted/scripts/validate-metasystem.sh" >/dev/null 2>&1; then
    echo "adopted-mode validation accepted a skill directory without SKILL.md" >&2
    exit 1
  fi
  rmdir "$adopted/skills/hollow"
  grep -v '^name:' "$adopted/skills/verify/SKILL.md" >"$adopted/skills/verify/SKILL.md.new"
  mv "$adopted/skills/verify/SKILL.md.new" "$adopted/skills/verify/SKILL.md"
  if bash "$adopted/scripts/validate-metasystem.sh" >/dev/null 2>&1; then
    echo "adopted-mode validation accepted a present skill with broken frontmatter" >&2
    exit 1
  fi
fi

# adopt.sh self-test, template mode only. The source is a committed snapshot
# of the current working tree: a git clone would exercise committed HEAD, not
# the implementation under review.
if true; then  # template-gated by the orchestrator
  srcrepo="$tmp/adopt-src"
  mkdir -p "$srcrepo"
  copy_tree_without_artifacts "$root" "$srcrepo"

  # The same adoption must work when the template is vendored one level below
  # the git toplevel, which is the real repository's own layout. A tree path
  # after the colon in git archive resolves relative to the cwd, so archiving
  # HEAD:<prefix> from inside the prefix yields an empty archive with exit 0;
  # every fixture stages at a root, which is why the first real adoption from
  # the vendored layout found it and no fixture did.
  nested_src="$tmp/adopt-nested"
  mkdir -p "$nested_src/vendored"
  copy_tree_without_artifacts "$root" "$nested_src/vendored"
  git -C "$nested_src" init -q -b main
  git -C "$nested_src" config metasystem.goal.machine fixture-machine
  git -C "$nested_src" add .
  git -C "$nested_src" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm nested
  nested_tgt="$tmp/adopt-nested-target"
  mkdir -p "$nested_tgt"
  git -C "$nested_tgt" init -q -b main
  git -C "$nested_tgt" config metasystem.goal.machine fixture-machine
  "$nested_src/vendored/scripts/adopt.sh" "$nested_tgt" --runtimes claude >"$tmp/adopt-nested.out" 2>&1 \
    || { echo "nested-prefix adoption failed" >&2; cat "$tmp/adopt-nested.out" >&2; exit 1; }
  [[ -f "$nested_tgt/metasystem.conf" && -d "$nested_tgt/scripts/agents" ]] \
    || { echo "nested-prefix adoption staged an empty payload" >&2; exit 1; }

  # The measuring kit must never reach an adopted repository: it carries every
  # spec's held-out grader, so a benchmark target receiving it would ship the
  # builders their own answer key. Assert on the payload, not on the source.
  [[ ! -e "$nested_tgt/benchmark" ]] \
    || { echo "adoption leaked the benchmark kit into the target" >&2; exit 1; }
  [[ ! -e "$nested_tgt/development" ]] \
    || { echo "adoption leaked development/ into the target" >&2; exit 1; }

  # The payload ships FRESH ledgers, never this repository's. The real
  # instruction ledger reached a live benchmark run before this assertion
  # existed, handing the builders the developers' own lessons.
  if grep -qE '^\| (IL|KI)-[0-9]' "$nested_tgt/plans/instruction-ledger.md" "$nested_tgt/plans/known-issues.md" 2>/dev/null; then
    echo "adoption shipped the template repository's own ledger rows" >&2; exit 1
  fi
  [[ ! -e "$nested_tgt/plans/receipts.log" && ! -e "$nested_tgt/README.md" ]] \
    || { echo "adoption shipped template-repository state" >&2; exit 1; }

  # IL-14: a brand-new plan file in the staged set needs explicit
  # acknowledgment; modifying a tracked plan stays free. The prose form of this
  # rule was violated by its own author, so it is a mechanism now.
  guard_repo="$tmp/guard-repo"
  # This fixture proves the NEW-PLAN rule, not the agent-wrapper rule,
  # so it must be invocation-shape independent (KI-31: the real
  # classifier answers differently under a live agent ancestor than
  # detached). Same pattern as pre-commit-guard-fixtures.sh: a copied
  # guard beside a refusing engine stub takes the fail-open HUMAN
  # path; the wrapper-token rule keeps its own coverage.
  guard_stub_root="$tmp/guard-stub-metasystem"
  mkdir -p "$guard_stub_root/scripts/agents" "$guard_stub_root/bin"
  cp "$root/scripts/agents/pre-commit-guard.sh" "$guard_stub_root/scripts/agents/pre-commit-guard.sh"
  printf '#!/usr/bin/env bash\nexit 1\n' >"$guard_stub_root/bin/metasystem"
  chmod +x "$guard_stub_root/bin/metasystem"
  guard_under_test="$guard_stub_root/scripts/agents/pre-commit-guard.sh"
  mkdir -p "$guard_repo/plans"
  git -C "$guard_repo" init -q -b main
  git -C "$guard_repo" config metasystem.goal.machine fixture-machine
  echo old >"$guard_repo/plans/existing.md"
  git -C "$guard_repo" add plans/existing.md
  git -C "$guard_repo" -c user.name=m -c user.email=m@example.invalid commit -qm seed
  echo new >"$guard_repo/plans/surprise.md"
  git -C "$guard_repo" add plans/surprise.md
  if (cd "$guard_repo" && "$guard_under_test" >/dev/null 2>&1); then
    echo "guard allowed a new plan file without acknowledgment" >&2; exit 1
  fi
  (cd "$guard_repo" && METASYSTEM_ALLOW_NEW_PLAN=1 "$guard_under_test") \
    || { echo "guard refused an acknowledged new plan" >&2; exit 1; }
  unborn_repo="$tmp/guard-unborn"
  mkdir -p "$unborn_repo/plans"
  git -C "$unborn_repo" init -q -b main
  git -C "$unborn_repo" config metasystem.goal.machine fixture-machine
  echo first >"$unborn_repo/plans/new.md"
  git -C "$unborn_repo" add plans/new.md
  (cd "$unborn_repo" && "$guard_under_test") \
    || { echo "guard refused the initial commit of an unborn branch" >&2; exit 1; }
  git -C "$guard_repo" reset -q
  echo changed >"$guard_repo/plans/existing.md"
  git -C "$guard_repo" add plans/existing.md
  (cd "$guard_repo" && "$guard_under_test") \
    || { echo "guard refused a modification to a tracked plan" >&2; exit 1; }
  [[ -x "$nested_tgt/.git/hooks/pre-commit" ]] \
    || { echo "adoption did not install the pre-commit guard hook" >&2; exit 1; }

  # F15: a target with its OWN pre-commit hook gets the guard COMPOSED
  # in front of it, not declined — the old hook is preserved as
  # pre-commit.local and still runs; re-adoption never stacks.
  compose_tgt="$tmp/compose-target"
  mkdir -p "$compose_tgt"
  git -C "$compose_tgt" init -q -b main
  git -C "$compose_tgt" config metasystem.goal.machine fixture-machine
  mkdir -p "$compose_tgt/.git/hooks"
  printf '#!/usr/bin/env bash\ntouch "$(git rev-parse --show-toplevel)/.project-hook-ran"\nexit 0\n' \
    >"$compose_tgt/.git/hooks/pre-commit"
  chmod +x "$compose_tgt/.git/hooks/pre-commit"
  "$nested_src/vendored/scripts/adopt.sh" "$compose_tgt" --runtimes claude >"$tmp/adopt-compose.out" 2>&1 \
    || { echo "adoption over a hooked target failed" >&2; cat "$tmp/adopt-compose.out" >&2; exit 1; }
  grep -q "pre-commit-guard.sh" "$compose_tgt/.git/hooks/pre-commit" \
    || { echo "adoption did not enroll the guard over an existing hook" >&2; exit 1; }
  [[ -x "$compose_tgt/.git/hooks/pre-commit.local" ]] \
    || { echo "adoption did not preserve the existing hook as pre-commit.local" >&2; exit 1; }
  # KI-31 again: the composed hook must prove COMPOSITION, not the
  # wrapper-token rule — the refusing engine stub takes the guard
  # down its fail-open HUMAN path regardless of this suite's own
  # ancestry.
  (cd "$compose_tgt" && METASYSTEM_BIN="$guard_stub_root/bin/metasystem" .git/hooks/pre-commit) \
    || { echo "the composed hook refused a clean tree" >&2; exit 1; }
  [[ -f "$compose_tgt/.project-hook-ran" ]] \
    || { echo "the preserved project hook no longer runs" >&2; exit 1; }
  "$nested_src/vendored/scripts/adopt.sh" "$compose_tgt" --runtimes claude >"$tmp/adopt-recompose.out" 2>&1 \
    || { echo "re-adoption over the composed hook failed" >&2; cat "$tmp/adopt-recompose.out" >&2; exit 1; }
  grep -q "pre-commit-guard.sh" "$compose_tgt/.git/hooks/pre-commit.local" \
    && { echo "re-adoption stacked the composer into pre-commit.local" >&2; exit 1; }
  grep -q "touch" "$compose_tgt/.git/hooks/pre-commit.local" \
    || { echo "re-adoption clobbered the preserved project hook" >&2; exit 1; }

  # R2-15: a target ALREADY carrying both files gets neither
  # clobbered — adoption warns and leaves both byte-identical.
  both_tgt="$tmp/both-hooks-target"
  mkdir -p "$both_tgt"
  git -C "$both_tgt" init -q -b main
  git -C "$both_tgt" config metasystem.goal.machine fixture-machine
  mkdir -p "$both_tgt/.git/hooks"
  printf '#!/bin/sh\necho main-hook\n' >"$both_tgt/.git/hooks/pre-commit"
  printf '#!/bin/sh\necho local-hook\n' >"$both_tgt/.git/hooks/pre-commit.local"
  chmod +x "$both_tgt/.git/hooks/pre-commit" "$both_tgt/.git/hooks/pre-commit.local"
  cp "$both_tgt/.git/hooks/pre-commit" "$tmp/both-pre-commit.snap"
  cp "$both_tgt/.git/hooks/pre-commit.local" "$tmp/both-pre-commit-local.snap"
  if "$nested_src/vendored/scripts/adopt.sh" "$both_tgt" --runtimes claude >"$tmp/adopt-both.out" 2>&1; then
    echo "adoption accepted a both-hooks target it cannot enroll (R2-15/R2-11)" >&2
    cat "$tmp/adopt-both.out" >&2
    exit 1
  fi
  grep -q "compose them by hand" "$tmp/adopt-both.out" \
    || { echo "the both-hooks refusal did not name the fix" >&2; cat "$tmp/adopt-both.out" >&2; exit 1; }
  cmp -s "$tmp/both-pre-commit.snap" "$both_tgt/.git/hooks/pre-commit" \
    || { echo "the existing pre-commit is not byte-identical after the refusal (R2-15)" >&2; exit 1; }
  cmp -s "$tmp/both-pre-commit-local.snap" "$both_tgt/.git/hooks/pre-commit.local" \
    || { echo "the existing pre-commit.local is not byte-identical after the refusal (R2-15)" >&2; exit 1; }
  # The refusal must land BEFORE any target mutation: no payload, no
  # binary, no genesis commit — an empty worktree and an unborn HEAD.
  stray=$(find "$both_tgt" -mindepth 1 -not -path "$both_tgt/.git" -not -path "$both_tgt/.git/*" | head -5)
  [[ -z "$stray" ]] \
    || { echo "the refused adoption mutated the target worktree:" >&2; echo "$stray" >&2; exit 1; }
  git -C "$both_tgt" rev-parse --verify HEAD >/dev/null 2>&1 \
    && { echo "the refused adoption created a commit in the target" >&2; exit 1; }

  # IL-16: an open chain counts as current for a plan's in-flight claim, within
  # a bounded window; a closed or aged chain does not. jobs_in_flight stays
  # strict on purpose, so the stop hook still refuses abandoning an open chain.
  chain_root="$tmp/chain-repo"
  mkdir -p "$chain_root/plans" "$chain_root/artifacts/agents/jobs"
  cat >"$chain_root/plans/stream.md" <<'PLAN'
- In flight right now: chain implementer-20260101t000000z-cccc (round 2 adjudicating)
- Waiting on the human: nothing blocking
- Next step: none
PLAN
  printf '{"jobId":"implementer-20260101t000000z-cccc","status":"completed"}\n' \
    >"$chain_root/artifacts/agents/jobs/implementer-20260101t000000z-cccc.json"
  [[ -z "$("$root/bin/metasystem" report open-work --repo "$chain_root" | grep STALE-PLAN)" ]] \
    || { echo "a plan naming an open chain between rounds was called stale" >&2; exit 1; }
  [[ -n "$(METASYSTEM_CHAIN_GRACE_SECONDS=0 "$root/bin/metasystem" report open-work --repo "$chain_root" | grep STALE-PLAN)" ]] \
    || { echo "an aged-out chain still suppressed the stale report" >&2; exit 1; }
  # The same two-field fixture record, now with its chain closed —
  # chainClosed must be a JSON boolean for the report to honor it.
  printf '{"jobId":"implementer-20260101t000000z-cccc","status":"completed","chainClosed":true}\n' \
    >"$chain_root/artifacts/agents/jobs/implementer-20260101t000000z-cccc.json"
  [[ -n "$("$root/bin/metasystem" report open-work --repo "$chain_root" | grep STALE-PLAN)" ]] \
    || { echo "a closed chain still suppressed the stale report" >&2; exit 1; }
  echo 'ignored-fixture.txt' >>"$srcrepo/.gitignore"
  echo junk >"$srcrepo/ignored-fixture.txt"
  git init -q -b main "$srcrepo"
  git -C "$srcrepo" config metasystem.goal.machine fixture-machine
  git -C "$srcrepo" add -A
  git -C "$srcrepo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm snapshot
  adopt="$srcrepo/scripts/adopt.sh"
  src_sha=$(git -C "$srcrepo" rev-parse HEAD)


  tgt="$tmp/adopt-default"
  mkdir -p "$tgt"
  printf 'project readme\n' >"$tgt/README.md"
  bash "$adopt" "$tgt" >/dev/null
  [[ -f "$tgt/.github/workflows/metasystem.yml" ]] || { echo "adopt: CI workflow not installed" >&2; exit 1; }
  [[ -L "$tgt/.claude/skills/verify" ]] || { echo "adopt: claude skill symlink missing" >&2; exit 1; }
  [[ -f "$tgt/.claude/agents/verify.md" ]] || { echo "adopt: claude agent profile missing" >&2; exit 1; }
  [[ -L "$tgt/.claude/skills/code-critique" && -f "$tgt/.claude/agents/code-critique.md" ]] \
    || { echo "adopt: code-critique was not registered for claude" >&2; exit 1; }
  [[ -f "$tgt/scripts/agents/dispatch.sh" && -f "$tgt/metasystem.conf" ]] \
    || { echo "adopt: orchestration payload missing" >&2; exit 1; }
  [[ -f "$tgt/go.mod" && -d "$tgt/internal" && -d "$tgt/cmd" ]] \
    || { echo "adopt: engine source did not ship (D17)" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=claude' "$tgt/metasystem.conf" \
    || { echo "adopt: default runtime selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=claude' "$tgt/metasystem.conf" \
    || { echo "adopt: selected claude was not made the roster default" >&2; exit 1; }
  # Active keys only: F-4 demoted optional families to commented examples,
  # and a comment is documentation, not a roster entry.
  if grep -Ev '^[[:space:]]*#' "$tgt/metasystem.conf" | grep -Eq '(^|\.)model\.(codex|devin)=|\.runtime=(codex|devin)$'; then
    echo "adopt: unselected runtime-valued keys survived the default selection" >&2
    exit 1
  fi
  grep -q systemMessage "$tgt/.claude/settings.json" || { echo "adopt: settings.json lacks the shipped hook" >&2; exit 1; }
  grep -q 'SessionStart' "$tgt/.claude/settings.json" \
    && grep -q 'supervision-hook.sh.*claude start' "$tgt/.claude/settings.json" \
    || { echo "adopt: Claude session-start supervision hook missing" >&2; exit 1; }
  [[ ! -e "$tgt/optional-skills" ]] || { echo "adopt: unselected optional skills were copied" >&2; exit 1; }
  [[ "$(cat "$tgt/README.md")" == "project readme" ]] || { echo "adopt: the project's own README was touched" >&2; exit 1; }
  [[ ! -e "$tgt/ignored-fixture.txt" ]] || { echo "adopt: ignored source content entered the payload" >&2; exit 1; }
  [[ "$(ls "$tgt/plans" | sort | tr '\n' ' ')" == "README.md goals-accepted.json goals.md instruction-ledger.md known-issues.md " ]] \
    || { echo "adopt: plans/ payload must carry the standing ledgers INCLUDING the goal pair (goal-system GOAL-14)" >&2; exit 1; }
  grep -q '^## Goal-free: declared .* over ' "$tgt/plans/goals.md" \
    || { echo "adopt: the seeded goal ledger lacks its digest-pinned Goal-free declaration" >&2; exit 1; }
  # Read-only pair check via the ENGINE's own fact (goal list's
  # baselineMatches: bytes and digest equal the accepted baseline) —
  # the same fact a second reconcile's "already reconciled" proved.
  # The old probe was a MUTATING verb, so it classified the fixture
  # caller for holder-only authority and passed or refused depending
  # on who ran the harness (the KI-31 invocation-shape class); a
  # consistency assertion needs no write authority — and no python
  # (the kill-python doctrine: metasystem-coupled decisions live in
  # the Go engine).
  "$tgt/bin/metasystem" goal list --root "$tgt" | grep -q '"baselineMatches":true' \
    || { echo "adopt: the seeded goal pair is not reconciled (baseline out of step)" >&2; exit 1; }
  # Genesis authorizes seeding, nothing more: once the pair stands, an
  # intent-bearing write is holder-only against the TARGET. Probe a COPY
  # of the adopted target so the probe's own write (or refusal) leaves
  # the real target byte-identical for the re-adoption walk below. When
  # this harness runs under agent ancestry the copy classifies us as a
  # delegate and `goal open` must refuse; under a terminal ancestry we
  # are its human and it must succeed — either way the write authority
  # is the target's own judgment, never something adoption granted.
  probe_tgt="$tmp/adopt-authority-probe"
  rm -rf "$probe_tgt"
  mkdir -p "$probe_tgt"
  cp -R "$tgt/." "$probe_tgt"
  fixture_view=$("$tgt/bin/metasystem" lease classify --root "$probe_tgt" --caller-pid $$ 2>/dev/null || true)
  fixture_class=$("$tgt/bin/metasystem" json get --value "$fixture_view" --field class 2>/dev/null || true)
  if open_out=$("$tgt/bin/metasystem" goal open --root "$probe_tgt" --id post-adopt-probe \
      --intent "authority probe" --next "none" 2>&1); then
    [[ "$fixture_class" == HUMAN ]] \
      || { echo "adopt: a $fixture_class caller opened a goal in the adopted target; genesis must not confer write authority" >&2; exit 1; }
  else
    [[ "$fixture_class" != HUMAN ]] \
      || { echo "adopt: the target's own human was refused goal open: $open_out" >&2; exit 1; }
    echo "$open_out" | grep -q 'lease holder' \
      || { echo "adopt: post-adoption open refused for the wrong reason: $open_out" >&2; exit 1; }
  fi
  rm -rf "$probe_tgt"
  [[ -d "$tgt/artifacts" ]] || { echo "adopt: artifacts directory missing" >&2; exit 1; }
  grep -qxF 'artifacts/' "$tgt/.gitignore" || { echo "adopt: artifacts/ not gitignored" >&2; exit 1; }
  grep -q "$src_sha" "$tgt/docs/project-rules.md" || { echo "adopt: template SHA not recorded" >&2; exit 1; }
  if grep -q '<template sha>' "$tgt/docs/project-rules.md"; then
    echo "adopt: template SHA placeholder left unreplaced" >&2
    exit 1
  fi
  snap="$tmp/adopt-snap"
  mkdir -p "$snap"
  cp -R "$tgt/." "$snap"
  bash "$adopt" "$tgt" >/dev/null
  # Explicit walk, not diff -r (KI-3, recurred here 2026-08-14): the adopted
  # tree carries .claude/skills symlinks, and BSD diff both warns "Directory
  # loop detected" and silently SKIPS those directories — a weakened check
  # behind noisy output. Same entry lists, same symlink targets, same bytes.
  (cd "$snap" && find . -mindepth 1 | LC_ALL=C sort) >"$tmp/adopt-idem-snap"
  (cd "$tgt" && find . -mindepth 1 | LC_ALL=C sort) >"$tmp/adopt-idem-tgt"
  cmp -s "$tmp/adopt-idem-snap" "$tmp/adopt-idem-tgt" \
    || { echo "adopt: second run changed an adopted target (entry lists differ)" >&2; exit 1; }
  while IFS= read -r rel; do
    a="$snap/$rel"; b="$tgt/$rel"
    if [[ -L "$a" || -L "$b" ]]; then
      [[ -L "$a" && -L "$b" && "$(readlink "$a")" == "$(readlink "$b")" ]] \
        || { echo "adopt: second run changed symlink $rel" >&2; exit 1; }
    elif [[ -f "$a" ]]; then
      cmp -s "$a" "$b" || { echo "adopt: second run changed $rel" >&2; exit 1; }
    fi
  done <"$tmp/adopt-idem-snap"
  if bash "$tgt/scripts/validate-metasystem.sh" >/dev/null 2>&1; then
    echo "adopt: target validated with unreplaced placeholders" >&2
    exit 1
  fi
  sed 's/<[^>]*>/filled/g' "$tgt/docs/project-rules.md" >"$tgt/docs/project-rules.md.new"
  mv "$tgt/docs/project-rules.md.new" "$tgt/docs/project-rules.md"
  if bash "$tgt/scripts/validate-metasystem.sh" >"$tmp/conf-placeholder.out" 2>&1; then
    echo "adopt: target validated while metasystem.conf placeholders remained" >&2
    exit 1
  fi
  grep -q 'metasystem.conf' "$tmp/conf-placeholder.out" \
    || { echo "adopt: placeholder failure did not name metasystem.conf" >&2; exit 1; }
  fill_harness_conf "$tgt/metasystem.conf" "$tmp/adopt-default-evidence"
  # The covenant evidence gate rides the same green validate (counselor
  # slice one): a fully valid covenant + canonical table + present deps
  # must pass, and the run must PROVE the gate fired, not skipped.
  printf '#!/usr/bin/env bash\nprintf "metric=greets=0\\n"\n' >"$tgt/gate.sh"
  mkdir -p "$tgt/src"
  printf 'print("hello")\n' >"$tgt/src/app.py"
  cat >"$tgt/covenant.json" <<'COVENANT'
{
  "schemaVersion": 1,
  "identity": {"name": "adopt-bed", "entryPoint": "bash gate.sh", "sourcePaths": ["src/"]},
  "requirements": [
    {"id": "1", "ref": "criterion 1: the app greets by name", "proof": "greets"}
  ],
  "battery": {"command": "bash gate.sh", "metric": "greets", "direction": "max", "threshold": ">=1"},
  "budgets": [],
  "guards": [],
  "guardrails": ["gate.sh", "docs/covenant-evidence.md"]
}
COVENANT
  cat >"$tgt/docs/covenant-evidence.md" <<'EVIDENCE'
# Covenant evidence — adopt-bed

| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | The app greets by name | greets | repo | bash gate.sh | gate.sh,src/app.py | gate.sh runs the entrypoint | observed |

Wired: 1. Floating: 0.
EVIDENCE
  # Capture, never discard: the receipt-stats flake's nested firings kept
  # dying invisibly behind this redirect (2026-08-14, evidence 51987/94210).
  # Digest equality before the nested run: the staged payload must be the
  # exact content the outer witness gate proved (D33) — when a witness is
  # armed, the check-only probe IS that comparison.
  if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]]; then
    ( cd "$tgt" && METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=DELIVERY \
        bash scripts/agents/go-gate.sh --witness-check-only >/dev/null ) \
      || { echo "adopt: staged payload digest does not match the witness-gated tree" >&2; exit 1; }
  fi
  bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/adopt-filled.out" 2>&1 || {
    echo "adopt: filled target failed validation" >&2
    tail -20 "$tmp/adopt-filled.out" >&2
    exit 1
  }
  # D17's whole point, asserted: with the source shipped, the adopted
  # target's own validation rebuilds and gates the engine.
  grep -Fq 'metasystem delivery contract validated' "$tmp/adopt-filled.out" \
    || { echo "adopt: filled target did not end on the contract verdict" >&2; exit 1; }
  if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]]; then
    grep -Fq 'outer witness' "$tmp/adopt-filled.out" \
      || { echo "adopt: filled target did not accept the outer witness" >&2; exit 1; }
  fi
  grep -Fq 'go gate: PASSED' "$tmp/adopt-filled.out" \
    || { echo "adopt: filled target did not run the go gate" >&2; exit 1; }
  grep -Fq 'covenant evidence gate passed' "$tmp/adopt-filled.out" \
    || { echo "adopt: the covenant evidence gate did not fire in the green run" >&2; exit 1; }
  # The red half: remove exactly the one criterion/proof pair the
  # covenant cites and the same validation must refuse, NAMING the
  # missing pair — then the table restores byte-for-byte so the later
  # fixtures keep judging the green bed.
  cp "$tgt/docs/covenant-evidence.md" "$tmp/evidence-green-reference.md"
  sed 's/| greets |/| salutes |/' "$tgt/docs/covenant-evidence.md" >"$tgt/docs/covenant-evidence.md.new"
  mv "$tgt/docs/covenant-evidence.md.new" "$tgt/docs/covenant-evidence.md"
  if bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/evidence-red.out" 2>&1; then
    echo "adopt: validation passed while the covenant cited a proof the table does not record" >&2
    exit 1
  fi
  grep -Fq 'bound to proof greets in the covenant but records proof salutes' "$tmp/evidence-red.out" \
    || { echo "adopt: the evidence refusal did not name the missing pair" >&2; tail -5 "$tmp/evidence-red.out" >&2; exit 1; }
  cp "$tmp/evidence-green-reference.md" "$tgt/docs/covenant-evidence.md"
  # A malformed entry at the covenant's home must REFUSE, never read
  # as absent: a dangling symlink is the shape only no-follow presence
  # detection sees (directory and FIFO refusals are the engine's,
  # pinned by its unit tests on the same branch).
  mv "$tgt/covenant.json" "$tmp/covenant-green-reference.json"
  ln -s covenant-that-does-not-exist.json "$tgt/covenant.json"
  if bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/evidence-symlink.out" 2>&1; then
    echo "adopt: validation passed while the covenant home held a dangling symlink" >&2
    exit 1
  fi
  grep -q "symlink" "$tmp/evidence-symlink.out" \
    || { echo "adopt: the symlinked covenant refusal did not name the symlink" >&2; tail -5 "$tmp/evidence-symlink.out" >&2; exit 1; }
  rm "$tgt/covenant.json"
  mv "$tmp/covenant-green-reference.json" "$tgt/covenant.json"

  echo drift >>"$tgt/.claude/agents/verify.md"
  if bash "$tgt/scripts/validate-metasystem.sh" >"$tmp/profile-drift.out" 2>&1; then
    echo "adopt: validation missed a drifted claude profile" >&2
    exit 1
  fi
  grep -q 'profile drifted' "$tmp/profile-drift.out" \
    || { echo "adopt: profile-drift failure did not name the profile" >&2; exit 1; }
  cp "$tgt/skills/verify/agents/claude-profile.md" "$tgt/.claude/agents/verify.md"

  # The D17 fail-open, closed and asserted (D33): a source-delivery target
  # whose go.mod vanished must FAIL — never read as "no engine expected".
  mv "$tgt/go.mod" "$tgt/go.mod.hidden"
  if bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/gomod-gone.out" 2>&1; then
    echo "adopt: a source-delivery target without go.mod validated green" >&2
    exit 1
  fi
  grep -Fq 'engine source did not ship' "$tmp/gomod-gone.out" \
    || { echo "adopt: the missing-go.mod refusal did not name the delivery" >&2; tail -5 "$tmp/gomod-gone.out" >&2; exit 1; }
  mv "$tgt/go.mod.hidden" "$tgt/go.mod"

  mv "$tgt/.claude/skills" "$tgt/.claude/skills.missing"
  if "$tgt/scripts/metasystem-config.sh" validate >"$tmp/missing-registration.out" 2>&1; then
    echo "adopt: configuration validation missed a selected runtime registration" >&2
    exit 1
  fi
  grep -q 'registration directory .claude/skills is missing' "$tmp/missing-registration.out" \
    || { echo "adopt: missing-registration failure did not name the directory" >&2; exit 1; }
  mv "$tgt/.claude/skills.missing" "$tgt/.claude/skills"

  bash "$adopt" "$tmp/adopt-devin" --runtimes devin >/dev/null
  [[ -f "$tmp/adopt-devin/.devin/agents/verify/AGENT.md" ]] || { echo "adopt: devin profile missing" >&2; exit 1; }
  [[ -L "$tmp/adopt-devin/.agents/skills/verify" && -L "$tmp/adopt-devin/.devin/skills/verify" ]] \
    || { echo "adopt: devin skill registrations missing" >&2; exit 1; }
  [[ -L "$tmp/adopt-devin/.devin/skills/code-critique" && -f "$tmp/adopt-devin/.devin/agents/code-critique/AGENT.md" ]] \
    || { echo "adopt: code-critique was not registered for devin" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=devin' "$tmp/adopt-devin/metasystem.conf" \
    || { echo "adopt: devin selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=devin' "$tmp/adopt-devin/metasystem.conf" \
    || { echo "adopt: devin was not selected as the roster default" >&2; exit 1; }
  [[ -f "$tmp/adopt-devin/.devin/config.json" ]] \
    && grep -q 'supervision-hook.sh.*devin start' "$tmp/adopt-devin/.devin/config.json" \
    || { echo "adopt: Devin-compatible session-start supervision hook missing" >&2; exit 1; }
  [[ ! -e "$tmp/adopt-devin/.claude" ]] || { echo "adopt: devin-only target got .claude state" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-codex" --runtimes codex >/dev/null
  [[ -L "$tmp/adopt-codex/.agents/skills/verify" ]] || { echo "adopt: codex skill registration missing" >&2; exit 1; }
  [[ -L "$tmp/adopt-codex/.agents/skills/code-critique" ]] || { echo "adopt: code-critique was not registered for codex" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=codex' "$tmp/adopt-codex/metasystem.conf" \
    || { echo "adopt: codex selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=codex' "$tmp/adopt-codex/metasystem.conf" \
    || { echo "adopt: codex was not selected as the roster default" >&2; exit 1; }
  [[ -f "$tmp/adopt-codex/.codex/hooks.json" ]] \
    && grep -q 'supervision-hook.sh.*codex start' "$tmp/adopt-codex/.codex/hooks.json" \
    || { echo "adopt: Codex session-start supervision hook missing" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-none" --runtimes none >/dev/null
  [[ ! -e "$tmp/adopt-none/.claude" && ! -e "$tmp/adopt-none/.devin" && ! -e "$tmp/adopt-none/.agents" ]] \
    || { echo "adopt: --runtimes none still registered a runtime" >&2; exit 1; }
  [[ -f "$tmp/adopt-none/.github/workflows/metasystem.yml" ]] || { echo "adopt: CI workflow skipped for --runtimes none" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=' "$tmp/adopt-none/metasystem.conf" \
    || { echo "adopt: --runtimes none did not record an empty runtime selection" >&2; exit 1; }
  if grep -Eq '^(role\.|mode\..*\.role\.)' "$tmp/adopt-none/metasystem.conf"; then
    echo "adopt: --runtimes none retained roster lines" >&2
    exit 1
  fi

  tier_src="$tmp/adopt-tier-src"
  cp -R "$srcrepo/." "$tier_src"
  conf_edit "$tier_src/metasystem.conf" awk '
    function replace_last(text, token, value,    rest, offset, position, found) {
      rest = text
      while ((position = index(rest, token)) != 0) {
        found = offset + position
        offset += position
        rest = substr(rest, position + 1)
      }
      if (found) return substr(text, 1, found - 1) value
      return text
    }
    {
      $0 = replace_last($0, ".model.claude=", ".model.claude=claude-model")
      $0 = replace_last($0, ".model.codex=", ".model.codex=codex-model")
      $0 = replace_last($0, ".model.devin=", ".model.devin=devin-model")
      printf "%s%s", $0, (FNR < conf_edit_line_count || conf_edit_final_terminated ? ORS : "")
    }
  '
  printf '\nmodel.tier.1=claude:claude-model,codex:codex-model,devin:devin-model\n' >>"$tier_src/metasystem.conf"
  git -C "$tier_src" add metasystem.conf
  git -C "$tier_src" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm tier-fixture
  bash "$tier_src/scripts/adopt.sh" "$tmp/adopt-tier-claude" --runtimes claude >/dev/null
  grep -qxF 'model.tier.1=claude:claude-model' "$tmp/adopt-tier-claude/metasystem.conf" \
    || { echo "adopt: model tier retained an unselected runtime" >&2; exit 1; }
  if grep -Eq '^(role\.|mode\..*\.role\.).*(\.model\.(codex|devin)=|\.runtime=(codex|devin)$)' "$tmp/adopt-tier-claude/metasystem.conf"; then
    echo "adopt: concrete unselected model or runtime keys survived pruning" >&2
    exit 1
  fi
  bash "$adopt" "$tmp/adopt-java" --enable debug-java >/dev/null
  [[ -f "$tmp/adopt-java/skills/debug-java/SKILL.md" ]] || { echo "adopt: --enable did not move the optional skill" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-copy" --runtimes claude,codex --copy-skills >/dev/null
  [[ -d "$tmp/adopt-copy/.claude/skills/verify" && ! -L "$tmp/adopt-copy/.claude/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy" >&2; exit 1; }
  [[ -d "$tmp/adopt-copy/.agents/skills/verify" && ! -L "$tmp/adopt-copy/.agents/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy the codex registration" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=claude,codex' "$tmp/adopt-copy/metasystem.conf" \
    || { echo "adopt: multi-runtime selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=codex' "$tmp/adopt-copy/metasystem.conf" \
    || { echo "adopt: runtime default did not follow codex, devin, claude precedence" >&2; exit 1; }
  sed 's/<[^>]*>/filled/g' "$tmp/adopt-copy/docs/project-rules.md" >"$tmp/adopt-copy/docs/project-rules.md.new"
  mv "$tmp/adopt-copy/docs/project-rules.md.new" "$tmp/adopt-copy/docs/project-rules.md"
  fill_harness_conf "$tmp/adopt-copy/metasystem.conf" "$tmp/adopt-copy-evidence"
  bash "$tmp/adopt-copy/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/nested-copied-skills.log" 2>&1 \
    || { echo "adopt: copied-skills target failed validation" >&2; tail -20 "$tmp/nested-copied-skills.log" >&2; exit 1; }
  echo drift >>"$tmp/adopt-copy/.claude/skills/verify/SKILL.md"
  if bash "$tmp/adopt-copy/scripts/validate-metasystem.sh" >/dev/null 2>&1; then
    echo "adopt: validation missed a drifted claude skill copy" >&2
    exit 1
  fi
  cp "$tmp/adopt-copy/skills/verify/SKILL.md" "$tmp/adopt-copy/.claude/skills/verify/SKILL.md"
  echo drift >>"$tmp/adopt-copy/.agents/skills/verify/SKILL.md"
  if bash "$tmp/adopt-copy/scripts/validate-metasystem.sh" >/dev/null 2>&1; then
    echo "adopt: validation missed a drifted codex skill copy" >&2
    exit 1
  fi
  cp "$tmp/adopt-copy/skills/verify/SKILL.md" "$tmp/adopt-copy/.agents/skills/verify/SKILL.md"
  rm -rf "$tmp/adopt-copy/skills/verify"
  if bash "$tmp/adopt-copy/scripts/validate-metasystem.sh" >"$tmp/orphan.out" 2>&1; then
    echo "adopt: validation missed an orphaned copy of a pruned skill" >&2
    exit 1
  fi
  grep -q "orphaned" "$tmp/orphan.out" || {
    echo "adopt: pruned-skill failure did not name the orphaned copy" >&2
    exit 1
  }

  mkdir -p "$tmp/adopt-foreign"
  touch "$tmp/adopt-foreign/.cursorrules"
  if bash "$adopt" "$tmp/adopt-foreign" >/dev/null 2>&1; then
    echo "adopt: accepted a target with a foreign instruction asset" >&2
    exit 1
  fi
  mkdir -p "$tmp/adopt-collide/docs"
  echo different >"$tmp/adopt-collide/docs/collaboration.md"
  if bash "$adopt" "$tmp/adopt-collide" >/dev/null 2>"$tmp/collide.err"; then
    echo "adopt: overwrote or skipped a colliding payload path" >&2
    exit 1
  fi
  grep -q 'docs/collaboration.md' "$tmp/collide.err" || {
    echo "adopt: collision refusal did not name the colliding path" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-bogus" --runtimes codez >/dev/null 2>&1; then
    echo "adopt: accepted an unknown runtime name" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-bogus/wow.md" ]] || {
    echo "adopt: a rejected runtime name still mutated the target" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-nonemix" --runtimes none,claude >/dev/null 2>&1; then
    echo "adopt: accepted the contradictory none-plus-runtime form" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-nonemix/wow.md" ]] || {
    echo "adopt: a rejected runtime combination still mutated the target" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-duplicate" --runtimes codex,codex >/dev/null 2>&1; then
    echo "adopt: accepted a duplicate runtime selection" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-duplicate/wow.md" ]] || {
    echo "adopt: a rejected duplicate runtime still mutated the target" >&2
    exit 1
  }
  bash "$adopt" "$tmp/adopt-partial" >/dev/null
  rm "$tmp/adopt-partial/.github/workflows/metasystem.yml"
  if bash "$adopt" "$tmp/adopt-partial" >/dev/null 2>&1; then
    echo "adopt: rerun over an incomplete installation reported success" >&2
    exit 1
  fi
  bash "$adopt" "$tmp/adopt-partial2" >/dev/null
  rm "$tmp/adopt-partial2/AGENTS.md"
  if bash "$adopt" "$tmp/adopt-partial2" >/dev/null 2>&1; then
    echo "adopt: rerun over a structurally broken installation reported success" >&2
    exit 1
  fi
  echo dirty >>"$srcrepo/wow.md"
  if bash "$adopt" "$tmp/adopt-dirty" >/dev/null 2>&1; then
    echo "adopt: ran from a dirty template worktree" >&2
    exit 1
  fi
  git -C "$srcrepo" checkout -q -- wow.md
  rm -rf "$tgt/skills/take-a-step-back"
  if bash "$tgt/scripts/validate-metasystem.sh" >"$tmp/dangling.out" 2>&1; then
    echo "adopt: validation missed a dangling registered skill link" >&2
    exit 1
  fi
  grep -q "dangling" "$tmp/dangling.out" || {
    echo "adopt: pruned-skill failure did not name the dangling link" >&2
    exit 1
  }
fi

# The two-part law, the half adoption can prove today: the app-owned
# covenant survives INITIAL adoption and a same-version re-run byte for
# byte (adopt.sh refuses cross-version reruns outright and routes them
# to the documented upgrade path — upgrade-time survival is that
# machinery's obligation, proven when it exists).
# The inception products ride the same law: the doctrine and a
# covenant-referenced net file are app-owned exactly like the covenant.
mkdir -p "$tmp/adopt-covenant/docs"
printf '{"identity": {"name": "the-app"}, "guardrails": ["goldens/", "gate.sh"]}\n' >"$tmp/adopt-covenant/covenant.json"
printf '# the-app doctrine\npatterns the delegates honor\n' >"$tmp/adopt-covenant/docs/app-doctrine.md"
cat >"$tmp/adopt-covenant/docs/covenant-evidence.md" <<'EVIDENCE'
# Covenant evidence — the-app

| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | The score holds | score | repo | bash gate.sh | gate.sh | gate.sh emits the metric | observed |

Wired: 1. Floating: 0.
EVIDENCE
printf '#!/usr/bin/env bash\nprintf "metric=score=1\\n"\n' >"$tmp/adopt-covenant/gate.sh"
for f in covenant.json docs/app-doctrine.md docs/covenant-evidence.md gate.sh; do
  mkdir -p "$tmp/adopt-covenant-reference/$(dirname "$f")"
  cp "$tmp/adopt-covenant/$f" "$tmp/adopt-covenant-reference/$f"
done
bash "$adopt" "$tmp/adopt-covenant" >/dev/null
for f in covenant.json docs/app-doctrine.md docs/covenant-evidence.md gate.sh; do
  cmp -s "$tmp/adopt-covenant/$f" "$tmp/adopt-covenant-reference/$f" || {
    echo "adopt: adoption altered the app-owned $f" >&2
    exit 1
  }
done
bash "$adopt" "$tmp/adopt-covenant" >/dev/null
for f in covenant.json docs/app-doctrine.md docs/covenant-evidence.md gate.sh; do
  cmp -s "$tmp/adopt-covenant/$f" "$tmp/adopt-covenant-reference/$f" || {
    echo "adopt: a same-version re-run altered the app-owned $f" >&2
    exit 1
  }
done

echo "adopt fixtures passed"
