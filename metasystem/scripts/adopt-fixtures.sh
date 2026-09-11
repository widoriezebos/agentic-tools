#!/usr/bin/env bash
set -euo pipefail

adopt_fixture_selection=all
adopt_fixture_forward=()
if [[ $# -eq 1 && $1 == --comparison ]]; then
  adopt_fixture_selection=comparison
  adopt_fixture_forward=(--comparison)
  shift
fi
# The bed's legs are scenarios (fixture-bed-scenarios.sh): the parent runs
# them side by side as children of this same script, each with its own
# temp root and its own snapshot of the source; a child gates its legs on
# $fixture_scenario below.
fixture_bed_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$fixture_bed_root/scripts/agents/fixture-budget.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario adopt "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
if (( ! fixture_bed_child )) && [[ $# -ne 0 ]]; then
  echo "adopt fixtures: accepts only --comparison" >&2
  exit 64
fi
adopt_fixture_scenarios=(default runtimes nested refusals landing-refs covenant tracer skills)
if (( fixture_bed_child )); then
  case "$fixture_scenario" in
    default | runtimes | nested | refusals | landing-refs | covenant | tracer | skills) ;;
    *) echo "adopt fixtures: unknown scenario: $fixture_scenario" >&2; exit 64 ;;
  esac
fi
adopt_leg() { # scenario name
  [[ "$fixture_scenario" == "$1" ]]
}

# adopt.sh self-test (script-validate-4/D35): extracted verbatim from
# validate-metasystem.sh's inline template-mode blocks into the sub-suite
# shape the file already used everywhere else. Template mode only — the
# orchestrator gates the invocation; the nested targets (which lack
# development/) cannot recurse. Failure evidence is preserved exactly like
# the orchestrator preserves its own.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
cd "$root"
source scripts/agents/fixture-budget.sh

adopt_progress_path="$root/artifacts/agents/supervision/suite-progress.jsonl"
adopt_progress_parent=0
if (( fixture_bed_child )); then
  adopt_progress_parent=1
elif [[ "${METASYSTEM_SUITE_PROGRESS_ACTIVE:-0}" == 1 \
  && "${METASYSTEM_SUITE_PROGRESS_ROOT:-}" == "$root" ]]; then
  adopt_progress_auth_bin=${METASYSTEM_PROOF_AUTH_BIN:-$root/bin/metasystem}
  if [[ -x "$adopt_progress_auth_bin" ]] \
    && "$adopt_progress_auth_bin" proof-run worker-authorized --root "$root" >/dev/null 2>&1; then
    adopt_progress_parent=1
  fi
fi
if (( ! adopt_progress_parent )); then
  adopt_progress_run="$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM"
  adopt_progress_tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-adopt.XXXXXX")
  adopt_progress_log="$root/artifacts/agents/supervision/suite-logs/adopt-$adopt_progress_run.log"
  adopt_progress_engine="$adopt_progress_tmp/metasystem"
  go build -o "$adopt_progress_engine" ./cmd/metasystem
  adopt_banner=$("$adopt_progress_engine" proof-run banner \
    --suite adopt-fixtures --root "$root" \
    --progress "$adopt_progress_path" --log "$adopt_progress_log")
  adopt_depth=$(( ${METASYSTEM_SUITE_PROGRESS_DEPTH:--1} + 1 ))
  exec "$adopt_progress_engine" proof-run launch \
    --suite adopt-fixtures --root "$root" --conf "$root/metasystem.conf" \
    --progress "$adopt_progress_path" --log "$adopt_progress_log" \
    --tmp "$adopt_progress_tmp" --banner "$adopt_banner" -- \
    env METASYSTEM_SUITE_PROGRESS_ACTIVE=1 \
      METASYSTEM_SUITE_PROGRESS_SUITE=adopt-fixtures \
      METASYSTEM_SUITE_PROGRESS_ROOT="$root" \
      METASYSTEM_SUITE_PROGRESS_DEPTH="$adopt_depth" \
      METASYSTEM_SUITE_PROGRESS_TMP="$adopt_progress_tmp" \
      METASYSTEM_SUITE_PROGRESS_LOG="$adopt_progress_log" \
      bash "$root/scripts/adopt-fixtures.sh" ${adopt_fixture_forward[@]+"${adopt_fixture_forward[@]}"}
elif (( ! fixture_bed_child )) && [[ "${METASYSTEM_SUITE_PROGRESS_SUITE:-}" != adopt-fixtures ]]; then
  "$root/bin/metasystem" proof-run banner --suite adopt-fixtures --root "$root" \
    --progress "$adopt_progress_path" --log "$METASYSTEM_SUITE_PROGRESS_LOG"
fi
# The progress worker still checks the fixture budget against the engine it will exercise.
harness_fixture_warn_if_engine_stale "$root"
# Every adoption in this harness runs from a STERILE SNAPSHOT source
# under whatever ancestry invoked the suite — agent or terminal. Genesis
# no longer cares which: a non-holder is admitted for exactly the
# adoption shape (a goal-free ledger on a checkout whose history carries
# none), judged against the target itself, so these fixtures carry no
# invocation-shape dependence and no authority-root env hook.
if (( fixture_bed_child )); then
  mkdir -p "${METASYSTEM_SUITE_PROGRESS_TMP:-${TMPDIR:-/tmp}}"
  tmp=$(mktemp -d "${METASYSTEM_SUITE_PROGRESS_TMP:-${TMPDIR:-/tmp}}/adopt-$fixture_scenario.XXXXXX")
elif [[ "${METASYSTEM_SUITE_PROGRESS_SUITE:-}" == adopt-fixtures \
  && -n "${METASYSTEM_SUITE_PROGRESS_TMP:-}" ]]; then
  tmp=$METASYSTEM_SUITE_PROGRESS_TMP
elif [[ -n "${METASYSTEM_SUITE_PROGRESS_TMP:-}" ]]; then
  mkdir -p "$METASYSTEM_SUITE_PROGRESS_TMP"
  tmp=$(mktemp -d "$METASYSTEM_SUITE_PROGRESS_TMP/adopt.XXXXXX")
else
  tmp=$(mktemp -d)
fi
witness_state=
cleanup() {
  status=$?
  [[ -n "$witness_state" ]] && rm -rf "$witness_state" 2>/dev/null
  if [[ $status != 0 && -d "$tmp" ]]; then
    keep="artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-adopt${fixture_scenario:+-$fixture_scenario}-$$"
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
if (( ! fixture_bed_child )) && [[ "$adopt_fixture_selection" == all && -z "${METASYSTEM_GATE_WITNESS:-}" \
  && -z "${METASYSTEM_PROOF_ATTEMPT:-}" ]] \
  && grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' go.mod; then
  delivery_contract=0
  WITNESS_GATE_FALLBACK=none source scripts/agents/witness-gate.sh
fi

source "$root/scripts/adopt-fixture-helpers.sh"

run_adoption_comparison() {
  # The actual fixture owns source-specific prerequisites and real main
  # custody in Go. The public shell remains the serial process parent.
  METASYSTEM_ADOPTION_COMPARISON=1 go test -json -count=1 -timeout=30m -run '^TestAdoptionComparisonSelectedScenarios$' ./cmd/metasystem
}

if [[ "$adopt_fixture_selection" == comparison ]]; then
  run_adoption_comparison
  exit
fi

# Each scenario child snapshots the source itself: the committed snapshot
# of the working tree (never a clone of HEAD) that adopt.sh runs from, and
# the vendored layout the nested legs adopt from.
adopt_prepare_srcrepo() {
  srcrepo="$tmp/adopt-src"
  mkdir -p "$srcrepo"
  copy_tree_without_artifacts "$root" "$srcrepo"
  echo 'ignored-fixture.txt' >>"$srcrepo/.gitignore"
  echo junk >"$srcrepo/ignored-fixture.txt"
  git init -q -b main "$srcrepo"
  git -C "$srcrepo" config metasystem.goal.machine fixture-machine
  git -C "$srcrepo" add -A
  git -C "$srcrepo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm snapshot
  adopt="$srcrepo/scripts/adopt.sh"
  src_sha=$(git -C "$srcrepo" rev-parse HEAD)
}
adopt_prepare_nested_src() {
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
}

if (( ! fixture_bed_child )); then
  source "$root/scripts/agents/fixture-bed-scenarios.sh"
  adopt_parent_cleanup() {
    [[ -z "$witness_state" ]] || rm -rf "$witness_state" 2>/dev/null || true
    rm -rf "$tmp" 2>/dev/null || true
  }
  fixture_bed_parent_extra_cleanup=adopt_parent_cleanup
  adopt_fixture_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios adopt "adopt fixtures passed" "$adopt_fixture_script" "${adopt_fixture_scenarios[@]}"
fi

# Adopted-mode contract: a copy without the template marker validates with a
# skill pruned, and a present-but-broken skill still fails. These legs call
# the validator's skill-inventory owner directly so an unrelated engine or
# process fixture cannot replace the result being asserted.
if adopt_leg skills; then
  adopted="$tmp/adopted"
  mkdir -p "$adopted"

  copy_tree_without_artifacts "$root" "$adopted"
  rm -rf "$adopted/development" "$adopted/skills/improve" "$adopted/memory/receipts.log" "$adopted/.claude"
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
  echo "adopt fixture leg started: a canonical skill pruned before runtime registration must validate" >&2
  bash "$adopted/scripts/agents/validate-skill-inventory.sh" "$adopted" >"$tmp/nested-pruned.log" 2>&1 || {
    nested_pruned_rc=$?
    echo "adopt fixture leg failed: pruned canonical skill; expected adopted inventory validation rc=0, observed rc=$nested_pruned_rc" >&2
    tail -80 "$tmp/nested-pruned.log" >&2
    exit 1
  }
  grep -Fq 'verify is valid' "$tmp/nested-pruned.log" \
    || { echo "adopt fixture leg failed: pruned canonical skill passed without validating the remaining inventory" >&2; exit 1; }
  echo "adopt fixture leg passed: pruned canonical skill remained valid" >&2

  echo "adopt fixture leg started: an empty canonical skill directory must be rejected" >&2
  mkdir "$adopted/skills/hollow"
  hollow_rc=0
  bash "$adopted/scripts/agents/validate-skill-inventory.sh" "$adopted" >"$tmp/nested-hollow.log" 2>&1 \
    || hollow_rc=$?
  if [[ $hollow_rc -eq 0 ]]; then
    echo "adopt fixture leg failed: empty canonical skill directory; expected rejection, observed rc=0" >&2
    exit 1
  fi
  grep -Fq 'skill directory without SKILL.md: skills/hollow' "$tmp/nested-hollow.log" \
    || { echo "adopt fixture leg failed: empty canonical skill directory was rejected for an unrelated reason" >&2; tail -80 "$tmp/nested-hollow.log" >&2; exit 1; }
  echo "adopt fixture leg passed: empty canonical skill directory was rejected with its reason" >&2
  rmdir "$adopted/skills/hollow"

  echo "adopt fixture leg started: broken canonical skill frontmatter must be rejected" >&2
  grep -v '^name:' "$adopted/skills/verify/SKILL.md" >"$adopted/skills/verify/SKILL.md.new"
  mv "$adopted/skills/verify/SKILL.md.new" "$adopted/skills/verify/SKILL.md"
  broken_skill_rc=0
  bash "$adopted/scripts/agents/validate-skill-inventory.sh" "$adopted" >"$tmp/nested-broken-skill.log" 2>&1 \
    || broken_skill_rc=$?
  if [[ $broken_skill_rc -eq 0 ]]; then
    echo "adopt fixture leg failed: broken canonical skill frontmatter; expected rejection, observed rc=0" >&2
    exit 1
  fi
  grep -Fq 'skills/verify/SKILL.md: missing name' "$tmp/nested-broken-skill.log" \
    || { echo "adopt fixture leg failed: broken canonical skill frontmatter was rejected for an unrelated reason" >&2; tail -80 "$tmp/nested-broken-skill.log" >&2; exit 1; }
  echo "adopt fixture leg passed: broken canonical skill frontmatter was rejected with its reason" >&2
fi

# adopt.sh self-test, template mode only. The source is a committed snapshot
# of the current working tree: a git clone would exercise committed HEAD, not
# the implementation under review.
if adopt_leg nested; then
  adopt_prepare_nested_src

  # HOST-C3-001: adoption has always accepted an explicit installation
  # beneath an application's Git root. Runtime registration must stay in that
  # installation and must not strand a fully copied payload when it discovers
  # the containing repository.
  nested_app="$tmp/adopt-existing-application"
  mkdir -p "$nested_app"
  git -C "$nested_app" init -q -b main
  git -C "$nested_app" config metasystem.goal.machine fixture-machine
  printf 'application readme\n' >"$nested_app/README.md"
  git -C "$nested_app" add README.md
  git -C "$nested_app" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm application-base
  nested_install="$nested_app/vendor/metasystem runtime"
  "$nested_src/vendored/scripts/adopt.sh" "$nested_install" --runtimes claude >"$tmp/adopt-inside-application.out" 2>&1 \
    || { echo "adoption beneath an existing application failed" >&2; cat "$tmp/adopt-inside-application.out" >&2; exit 1; }
  [[ -f "$nested_install/metasystem.conf" && -f "$nested_install/.claude/settings.json" && -L "$nested_install/.claude/skills/verify" ]] \
    || { echo "nested application adoption omitted its local Claude registration" >&2; exit 1; }
  [[ ! -e "$nested_app/.claude" && "$(cat "$nested_app/README.md")" == "application readme" ]] \
    || { echo "nested application adoption modified its parent application's files" >&2; exit 1; }
  "$nested_install/bin/metasystem" runtime setup \
    --repo "$nested_install/skills/verify" --runtimes claude --check >/dev/null \
    || { echo "nested application adoption failed shared runtime setup check" >&2; exit 1; }

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
  if grep -qE '^\| (IL|KI)-[0-9]' "$nested_tgt/memory/instruction-ledger.md" "$nested_tgt/memory/known-issues.md" 2>/dev/null; then
    echo "adoption shipped the template repository's own ledger rows" >&2; exit 1
  fi
  [[ ! -e "$nested_tgt/memory/receipts.log" && ! -e "$nested_tgt/README.md" ]] \
    || { echo "adoption shipped template-repository state" >&2; exit 1; }

  # Exercise the writers through an installation genuinely vendored beneath
  # the application repository. The prefix intentionally contains only the
  # entrypoints and installation configuration: application state must not be
  # recreated beside them.
  vendored_prefix="$nested_tgt/metasystem"
  mkdir -p "$vendored_prefix/bin" "$vendored_prefix/scripts"
  cp "$nested_tgt/bin/metasystem" "$vendored_prefix/bin/metasystem"
  cp "$nested_tgt/scripts/receipt.sh" "$vendored_prefix/scripts/receipt.sh"
  cp "$nested_tgt/metasystem.conf" "$vendored_prefix/metasystem.conf"
  [[ ! -e "$vendored_prefix/memory" ]] \
    || { echo "vendored fixture began with application memory" >&2; exit 1; }

  # A commitless repository ticks DEGRADED by design and a degraded tick
  # never persists evidence; real adopted repositories have commits, so
  # the fixture gives its target one before asserting healthy-tick writes.
  git -C "$nested_tgt" add -A
  git -C "$nested_tgt" -c core.hooksPath=/dev/null -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm adopted-base
  METASYSTEM_BIN="$vendored_prefix/bin/metasystem" \
    "$vendored_prefix/scripts/receipt.sh" add --type implement --outcome shipped \
    --skills none --verify clean --corrections 0 --stop-loss no --note "adopted state-root fixture" >/dev/null
  tick_out=$("$vendored_prefix/bin/metasystem" steward tick --repo "$nested_tgt") \
    || { echo "adopted steward tick failed" >&2; exit 1; }
  grep -vq '"verdict": "degraded"' <<<"$tick_out" \
    || { echo "adopted steward tick unexpectedly degraded: $tick_out" >&2; exit 1; }
  [[ -f "$nested_tgt/memory/receipts.log" && -f "$nested_tgt/artifacts/agents/steward/highwater.json" ]] \
    || { echo "adopted writers did not use the application state trees" >&2; exit 1; }
  [[ ! -e "$vendored_prefix/memory" ]] \
    || { echo "adopted writers recreated metasystem/memory/" >&2; exit 1; }
  for register in known-issues.md flake-registry.md rulings.md instruction-ledger.md receipts.log backlog-notes.md proposal-drafts.md llm-wiki-pattern.md; do
    [[ ! -e "$vendored_prefix/$register" && ! -e "$vendored_prefix/plans/$register" && ! -e "$vendored_prefix/memory/$register" ]] \
      || { echo "adopted writer placed $register under the vendored tree" >&2; exit 1; }
  done

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
  if (cd "$guard_repo" && METASYSTEM_BIN="$guard_stub_root/bin/metasystem" "$guard_under_test" >/dev/null 2>&1); then
    echo "guard allowed a new plan file without acknowledgment" >&2; exit 1
  fi
  (cd "$guard_repo" && METASYSTEM_BIN="$guard_stub_root/bin/metasystem" METASYSTEM_ALLOW_NEW_PLAN=1 "$guard_under_test") \
    || { echo "guard refused an acknowledged new plan" >&2; exit 1; }
  unborn_repo="$tmp/guard-unborn"
  mkdir -p "$unborn_repo/plans"
  git -C "$unborn_repo" init -q -b main
  git -C "$unborn_repo" config metasystem.goal.machine fixture-machine
  echo first >"$unborn_repo/plans/new.md"
  git -C "$unborn_repo" add plans/new.md
  (cd "$unborn_repo" && METASYSTEM_BIN="$guard_stub_root/bin/metasystem" "$guard_under_test") \
    || { echo "guard refused the initial commit of an unborn branch" >&2; exit 1; }
  git -C "$guard_repo" reset -q
  echo changed >"$guard_repo/plans/existing.md"
  git -C "$guard_repo" add plans/existing.md
  (cd "$guard_repo" && METASYSTEM_BIN="$guard_stub_root/bin/metasystem" "$guard_under_test") \
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
fi

if adopt_leg landing-refs; then
  adopt_prepare_srcrepo


  echo "adopt fixture leg started: a detached target must finish adoption without seeding a landing ref" >&2
  detached_tgt="$tmp/adopt-detached"
  git init -q -b trunk "$detached_tgt"
  printf 'detached target\n' >"$detached_tgt/README.md"
  git -C "$detached_tgt" add README.md
  git -C "$detached_tgt" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm base
  git -C "$detached_tgt" checkout -q --detach
  if ! bash "$adopt" "$detached_tgt" --runtimes none >"$tmp/adopt-detached.out" 2>&1; then
    echo "adopt fixture leg failed: detached target did not finish adoption" >&2
    tail -80 "$tmp/adopt-detached.out" >&2
    exit 1
  fi
  grep -Fq 'landing ref was not seeded: the target checkout is detached; automatic machine re-arm remains disabled until the key is configured' "$tmp/adopt-detached.out" \
    || { echo "adopt fixture leg failed: detached target omitted the landing-ref note" >&2; tail -80 "$tmp/adopt-detached.out" >&2; exit 1; }
  if git -C "$detached_tgt" config --local --no-includes --get metasystem.steward.landing-ref >/dev/null 2>&1; then
    echo "adopt fixture leg failed: detached target received a landing ref" >&2
    exit 1
  fi
  echo "adopt fixture leg passed: detached target finished with automatic machine re-arm disabled" >&2

  echo "adopt fixture leg started: a preset landing ref must be kept" >&2
  preset_tgt="$tmp/adopt-preset-landing-ref"
  git init -q -b trunk "$preset_tgt"
  printf 'preset landing ref target\n' >"$preset_tgt/README.md"
  git -C "$preset_tgt" add README.md
  git -C "$preset_tgt" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm base
  git -C "$preset_tgt" remote add origin "$preset_tgt"
  git -C "$preset_tgt" fetch -q origin
  git -C "$preset_tgt" branch --set-upstream-to=origin/trunk trunk >/dev/null
  git -C "$preset_tgt" config --local metasystem.steward.landing-ref refs/remotes/kept/stable
  if ! bash "$adopt" "$preset_tgt" --runtimes none >"$tmp/adopt-preset-landing-ref.out" 2>&1; then
    echo "adopt fixture leg failed: target with a preset landing ref did not finish adoption" >&2
    tail -80 "$tmp/adopt-preset-landing-ref.out" >&2
    exit 1
  fi
  grep -Fq 'landing ref was kept: metasystem.steward.landing-ref=refs/remotes/kept/stable' "$tmp/adopt-preset-landing-ref.out" \
    || { echo "adopt fixture leg failed: preset landing ref was kept without a note" >&2; tail -80 "$tmp/adopt-preset-landing-ref.out" >&2; exit 1; }
  [[ "$(git -C "$preset_tgt" config --local --no-includes --get metasystem.steward.landing-ref)" == refs/remotes/kept/stable ]] \
    || { echo "adopt fixture leg failed: preset landing ref was overwritten" >&2; exit 1; }
  echo "adopt fixture leg passed: preset landing ref was kept" >&2

  echo "adopt fixture leg started: a detached target with a preset landing ref must keep it" >&2
  detached_preset_tgt="$tmp/adopt-detached-preset-landing-ref"
  git init -q -b trunk "$detached_preset_tgt"
  printf 'detached preset landing ref target\n' >"$detached_preset_tgt/README.md"
  git -C "$detached_preset_tgt" add README.md
  git -C "$detached_preset_tgt" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm base
  git -C "$detached_preset_tgt" config --local metasystem.steward.landing-ref refs/remotes/kept/detached
  git -C "$detached_preset_tgt" checkout -q --detach
  if ! bash "$adopt" "$detached_preset_tgt" --runtimes none >"$tmp/adopt-detached-preset-landing-ref.out" 2>&1; then
    echo "adopt fixture leg failed: detached target with a preset landing ref did not finish adoption" >&2
    tail -80 "$tmp/adopt-detached-preset-landing-ref.out" >&2
    exit 1
  fi
  grep -Fq 'landing ref was kept: metasystem.steward.landing-ref=refs/remotes/kept/detached' "$tmp/adopt-detached-preset-landing-ref.out" \
    || { echo "adopt fixture leg failed: detached target omitted the kept landing-ref note" >&2; tail -80 "$tmp/adopt-detached-preset-landing-ref.out" >&2; exit 1; }
  if grep -Fq 'landing ref was not seeded' "$tmp/adopt-detached-preset-landing-ref.out"; then
    echo "adopt fixture leg failed: detached target with a preset landing ref printed a not-seeded note" >&2
    tail -80 "$tmp/adopt-detached-preset-landing-ref.out" >&2
    exit 1
  fi
  [[ "$(git -C "$detached_preset_tgt" config --local --no-includes --get metasystem.steward.landing-ref)" == refs/remotes/kept/detached ]] \
    || { echo "adopt fixture leg failed: detached target's preset landing ref was overwritten" >&2; exit 1; }
  echo "adopt fixture leg passed: detached target kept its preset landing ref" >&2
fi

if adopt_leg default; then
  adopt_prepare_srcrepo
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
  for suite_setting in \
      'suite.progress-silence-min=30' \
      'suite.section-cap-min=45' \
      'suite.evidence-copy-timeout-sec=60' \
      'suite.evidence-copy-max-mb=512'; do
    grep -qxF "$suite_setting" "$tgt/metasystem.conf" \
      || { echo "adopt: tailored configuration dropped suite progress setting $suite_setting" >&2; exit 1; }
  done
  # Active keys only: F-4 demoted optional families to commented examples,
  # and a comment is documentation, not a roster entry.
  if grep -Ev '^[[:space:]]*#' "$tgt/metasystem.conf" | grep -Eq '(^|\.)model\.(codex|devin)=|\.runtime=(codex|devin)$'; then
    echo "adopt: unselected runtime-valued keys survived the default selection" >&2
    exit 1
  fi
  grep -q 'SessionStart' "$tgt/.claude/settings.json" \
    && grep -q 'supervision-hook.sh.*claude start' "$tgt/.claude/settings.json" \
    || { echo "adopt: Claude session-start supervision hook missing" >&2; exit 1; }
  "$tgt/bin/metasystem" runtime setup --repo "$tgt" --runtimes claude --check >/dev/null \
    || { echo "adopt: default Claude registration failed shared setup check" >&2; exit 1; }
  [[ ! -e "$tgt/optional-skills" ]] || { echo "adopt: unselected optional skills were copied" >&2; exit 1; }
  [[ "$(cat "$tgt/README.md")" == "project readme" ]] || { echo "adopt: the project's own README was touched" >&2; exit 1; }
  [[ ! -e "$tgt/ignored-fixture.txt" ]] || { echo "adopt: ignored source content entered the payload" >&2; exit 1; }
  cmp -s "$srcrepo/records/misc/goals-migration-manifest.md" \
    "$tgt/records/misc/goals-migration-manifest.md" \
    || { echo "adopt: the engine's goals migration manifest did not ship byte-for-byte" >&2; exit 1; }
  [[ "$(ls "$tgt/plans" | sort | tr '\n' ' ')" == "README.md goals-accepted.json goals.md " ]] \
	|| { echo "adopt: plans/ payload must carry only live intent, including the goal pair" >&2; exit 1; }
  [[ "$(ls "$tgt/memory" | sort | tr '\n' ' ')" == "README.md instruction-ledger.md known-issues.md rulings.md " ]] \
	|| { echo "adopt: memory/ payload must carry only fresh living registers" >&2; exit 1; }
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
  # The direct audit proves the rule itself; validation proves its cheap refusal precedes the engine gate.
  if (cd "$tgt" && METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS= \
      bash scripts/audit-metasystem.sh .) >"$tmp/project-rules-placeholder-audit.out" 2>&1; then
    echo "adopt: direct audit accepted unreplaced project-rules placeholders" >&2
    exit 1
  fi
  placeholder_refusal_message='adopted repository has unreplaced placeholders in docs/project-rules.md or metasystem.conf'
  placeholder_refusal_started=$SECONDS
  if METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS= \
      bash "$tgt/scripts/validate-metasystem.sh" >"$tmp/project-rules-placeholder.out" 2>&1; then
    echo "adopt: target validated with unreplaced placeholders" >&2
    exit 1
  fi
  grep -Fqx "$placeholder_refusal_message" "$tmp/project-rules-placeholder.out" \
    || { echo "adopt: project-rules placeholder failed without the static placeholder-scan refusal" >&2; exit 1; }
  placeholder_refusal_elapsed=$((SECONDS - placeholder_refusal_started))
  (( placeholder_refusal_elapsed < 60 )) \
    || { echo "adopt: project-rules placeholder refusal took ${placeholder_refusal_elapsed}s; expected the pre-gate scan to fail within seconds" >&2; exit 1; }
  sed 's/<[^>]*>/filled/g' "$tgt/docs/project-rules.md" >"$tgt/docs/project-rules.md.new"
  mv "$tgt/docs/project-rules.md.new" "$tgt/docs/project-rules.md"
  # The configuration leg applies the same direct-rule and cheap-entrypoint proofs.
  if (cd "$tgt" && METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS= \
      bash scripts/audit-metasystem.sh .) >"$tmp/conf-placeholder-audit.out" 2>&1; then
    echo "adopt: direct audit accepted unreplaced configuration placeholders" >&2
    exit 1
  fi
  placeholder_refusal_started=$SECONDS
  if METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS= \
      bash "$tgt/scripts/validate-metasystem.sh" >"$tmp/conf-placeholder.out" 2>&1; then
    echo "adopt: target validated while metasystem.conf placeholders remained" >&2
    exit 1
  fi
  grep -Fqx "$placeholder_refusal_message" "$tmp/conf-placeholder.out" \
    || { echo "adopt: configuration placeholder failed without the static placeholder-scan refusal" >&2; exit 1; }
  placeholder_refusal_elapsed=$((SECONDS - placeholder_refusal_started))
  (( placeholder_refusal_elapsed < 60 )) \
    || { echo "adopt: configuration placeholder refusal took ${placeholder_refusal_elapsed}s; expected the pre-gate scan to fail within seconds" >&2; exit 1; }
  fill_harness_conf "$tgt/metasystem.conf" "$tmp/adopt-default-evidence"
  fill_harness_testing_contract "$srcrepo/testing.json" "$tgt/testing.json"
  prepare_filled_target_covenant "$tgt"
  METASYSTEM_ADOPTION_TARGET="$tgt" METASYSTEM_ADOPTION_KIND=filled \
    METASYSTEM_ADOPTION_FIXTURE_ROOT="$tmp" run_adoption_comparison


  mv "$tgt/.claude/skills" "$tgt/.claude/skills.missing"
  if "$tgt/scripts/metasystem-config.sh" validate >"$tmp/missing-registration.out" 2>&1; then
    echo "adopt: configuration validation missed a selected runtime registration" >&2
    exit 1
  fi
  grep -q 'registration directory .claude/skills is missing' "$tmp/missing-registration.out" \
    || { echo "adopt: missing-registration failure did not name the directory" >&2; exit 1; }
  mv "$tgt/.claude/skills.missing" "$tgt/.claude/skills"
fi

if adopt_leg runtimes; then
  adopt_prepare_srcrepo
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
  "$tmp/adopt-devin/bin/metasystem" runtime setup --repo "$tmp/adopt-devin" --runtimes devin --check >/dev/null \
    || { echo "adopt: Devin registration failed shared setup check" >&2; exit 1; }
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
  "$tmp/adopt-codex/bin/metasystem" runtime setup --repo "$tmp/adopt-codex" --runtimes codex --check >/dev/null \
    || { echo "adopt: Codex registration failed shared setup check" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-none" --runtimes none >/dev/null
  [[ ! -e "$tmp/adopt-none/.claude" && ! -e "$tmp/adopt-none/.devin" && ! -e "$tmp/adopt-none/.agents" ]] \
    || { echo "adopt: --runtimes none still registered a runtime" >&2; exit 1; }
  [[ -f "$tmp/adopt-none/.github/workflows/metasystem.yml" ]] || { echo "adopt: CI workflow skipped for --runtimes none" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=' "$tmp/adopt-none/metasystem.conf" \
    || { echo "adopt: --runtimes none did not record an empty runtime selection" >&2; exit 1; }
  "$tmp/adopt-none/bin/metasystem" runtime setup --repo "$tmp/adopt-none" --runtimes none --check >/dev/null \
    || { echo "adopt: none registration failed shared setup check" >&2; exit 1; }
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
  "$tmp/adopt-copy/bin/metasystem" runtime setup --repo "$tmp/adopt-copy" \
    --runtimes claude,codex --copy-skills --check >/dev/null \
    || { echo "adopt: copied registrations failed shared setup check" >&2; exit 1; }
  grep -qxF 'metasystem.runtimes=claude,codex' "$tmp/adopt-copy/metasystem.conf" \
    || { echo "adopt: multi-runtime selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=codex' "$tmp/adopt-copy/metasystem.conf" \
    || { echo "adopt: runtime default did not follow codex, devin, claude precedence" >&2; exit 1; }
  sed 's/<[^>]*>/filled/g' "$tmp/adopt-copy/docs/project-rules.md" >"$tmp/adopt-copy/docs/project-rules.md.new"
  mv "$tmp/adopt-copy/docs/project-rules.md.new" "$tmp/adopt-copy/docs/project-rules.md"
  fill_harness_conf "$tmp/adopt-copy/metasystem.conf" "$tmp/adopt-copy-evidence"
  fill_harness_testing_contract "$srcrepo/testing.json" "$tmp/adopt-copy/testing.json"
  METASYSTEM_ADOPTION_TARGET="$tmp/adopt-copy" METASYSTEM_ADOPTION_KIND=copied \
    METASYSTEM_ADOPTION_FIXTURE_ROOT="$tmp" run_adoption_comparison
fi

if adopt_leg refusals; then
  adopt_prepare_srcrepo
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
fi

if adopt_leg default; then
  rm -rf "$tgt/skills/take-a-step-back"
  if METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready \
      bash "$tgt/scripts/agents/validate-section-selector.sh" run runtime-contract-audits \
      >"$tmp/dangling.out" 2>&1; then
    echo "adopt: validation missed a dangling registered skill link" >&2
    exit 1
  fi
  grep -Fq 'registered skill link is dangling: .claude/skills/take-a-step-back' "$tmp/dangling.out" || {
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
if adopt_leg covenant; then
adopt_prepare_srcrepo
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
fi


# The declared-touch-list tracer (memory-architecture slice 3, R-10's
# provable half): adoption's writes are exactly the expectation
# computed from the SOURCE tree plus the enumerated seeds — one
# inventory, no wildcards [MAC-S3-003]; pre-existing app bytes are
# digest-compared so modification and deletion are caught, not just
# creation [MAC-S3-002]; two runtimes exercise the branchy writers
# [MAC-S3-005].
if adopt_leg tracer; then
  adopt_prepare_nested_src
  for tracer_runtime in claude codex; do
  tracer_tgt="$tmp/tracer-target-$tracer_runtime"
  mkdir -p "$tracer_tgt/docs" "$tracer_tgt/src"
  git -C "$tracer_tgt" init -q -b main
  git -C "$tracer_tgt" config metasystem.goal.machine fixture-machine
  printf 'app content\n' >"$tracer_tgt/src/app.txt"
  printf 'app doc\n' >"$tracer_tgt/docs/app.md"
  (cd "$tracer_tgt" && find . -type f -not -path './.git/*' -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) >"$tmp/tracer-before-digest"
  "$nested_src/vendored/scripts/adopt.sh" "$tracer_tgt" --runtimes "$tracer_runtime" >"$tmp/adopt-tracer.out" 2>&1 \
    || { echo "tracer adoption failed ($tracer_runtime)" >&2; cat "$tmp/adopt-tracer.out" >&2; exit 1; }
  # Pre-existing app bytes: digest-identical after adoption, except the
  # enumerated append-seeds.
  while IFS= read -r line; do
    digest=${line%% *}; f=${line#*  }
    case "${f#./}" in
      .gitignore|.gitattributes) continue ;;
    esac
    [[ "$(shasum -a 256 "$tracer_tgt/${f#./}" 2>/dev/null | cut -d' ' -f1)" == "$digest" ]] \
      || { echo "adoption altered or deleted app-owned bytes ($tracer_runtime): $f" >&2; exit 1; }
  done <"$tmp/tracer-before-digest"
  # New writes: exactly the source's shippable inventory + enumerated seeds.
  (cd "$nested_src/vendored" && find . -type f \
      -not -path './.git/*' -not -path './artifacts/*' -not -path './bin/*' \
      -not -path './development/*' -not -path './memory/*' \
      -not -path './plans/*' -not -path './records/*' \
      | sed 's|^\./||' | LC_ALL=C sort) >"$tmp/tracer-expected-src"
  (cd "$tracer_tgt" && find . -type f -not -path './.git/*' -not -path './artifacts/*' | sed 's|^\./||' | LC_ALL=C sort) >"$tmp/tracer-after"
  (cd "$tracer_tgt" && find .git/hooks -type f 2>/dev/null | LC_ALL=C sort) >"$tmp/tracer-hooks"
  while IFS= read -r written; do
    grep -qxF "$written" "$tmp/tracer-expected-src" && continue
    case "$written" in
      src/app.txt|docs/app.md|.gitignore|.gitattributes) continue ;;
      metasystem.conf|plans/goals-accepted.json|bin/metasystem) continue ;;
      .github/workflows/metasystem.yml) continue ;;
      memory/known-issues.md|memory/instruction-ledger.md|memory/rulings.md) continue ;;
      plans/goals.md|plans/goals/*|plans/README.md|memory/README.md|records/README.md|records/goals/.gitkeep|records/misc/goals-migration-manifest.md|records/misc/fleet-coordinator-brain-role-packet.md) continue ;;
      .claude/*|.agents/*|.devin/*|.codex/hooks.json) continue ;;
      *) echo "adoption wrote outside the computed inventory ($tracer_runtime): $written" >&2; exit 1 ;;
    esac
  done <"$tmp/tracer-after"
  [[ -s "$tmp/tracer-hooks" ]] || { echo "adoption installed no hook ($tracer_runtime)" >&2; exit 1; }
  [[ ! -e "$tracer_tgt/metasystem/memory" && -f "$tracer_tgt/memory/rulings.md" ]] \
    || { echo "adoption did not place the tailored landing rulings at the application root ($tracer_runtime)" >&2; exit 1; }
  done
fi

echo "adopt fixture scenario complete: $fixture_scenario"
