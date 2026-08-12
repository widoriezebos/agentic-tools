#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: scripts/validate-metasystem.sh [--delegate-scope]" >&2
}

delegate_scope=0
case ${1:-} in
  '') ;;
  --delegate-scope) [[ $# -eq 1 ]] || { usage; exit 2; }; delegate_scope=1 ;;
  -h|--help) usage; exit 0 ;;
  *) usage; exit 2 ;;
esac

delegate_owed_sections=(
  "supervision and census fixtures"
  "supervisor fingerprint heal harness"
  "dispatcher, adapter selftest, and mission-runner process fixtures"
)
delegate_skipped_sections=()
delegate_process_section() { # human-readable section name
  if (( delegate_scope )); then
    delegate_skipped_sections+=("$1")
    return 1
  fi
  return 0
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
# Captured AFTER the cd above: the sentinel must describe the suite's own
# root, never the caller's working directory — a nested adopted-copy run
# inherited the template's cwd and believed itself the template.
metasystem_here=$(pwd -P)

# Two concurrent suite runs trample each other's shared fixtures, so the
# suite refuses to start over a live gate run. The decision is the engine's
# (gate fence prunes dead markers and exempts this process's own chain);
# this is only the consult. On a first build there is no binary to ask.
# Only a REAL block (exit 1) refuses; a binary too old to know the verb
# (exit 2) must not brick the run that would rebuild it. The gate fence
# fixture below keeps that leniency honest: a usage bug fails there loudly.
if [[ "${METASYSTEM_ALLOW_CONCURRENT_GATE:-0}" != 1 && -x bin/metasystem ]]; then
  gate_fence_rc=0
  bin/metasystem gate fence --root "$root" --self-pid $$ || gate_fence_rc=$?
  if [[ "$gate_fence_rc" == 1 ]]; then
    echo "a gate run is already live in this checkout; refusing a concurrent suite (METASYSTEM_ALLOW_CONCURRENT_GATE=1 overrides)" >&2
    exit 1
  fi
fi

# A gate run is work in flight that no job record describes, so it says so
# itself rather than being guessed at from process command lines. The marker
# names this process by pid and start time; the turn-end report believes it
# only while that exact process is alive. On a first build the binary does
# not exist yet; the marker is best-effort and the register is skipped.
gate_run_marker=$(bin/metasystem gate register --root "$root" \
  --gate validate-metasystem.sh --pid $$ 2>/dev/null || true)

# The Go engine gate runs first (plans/go-migration.md): gofmt, vet, the
# race unit suite, and the build. A broken binary fails here, before any
# fixture tries to drive it. Skipped only in delegate scope (no toolchain
# guarantee in a delegate sandbox, and it needs no process visibility).
# The Go engine section runs only where the engine is present (a go.mod
# in this checkout). An adopted target that has not yet received the Go
# engine (a Phase 4 port) runs pure shell/python, so the whole section is
# a no-op there — the go gate, the seam tripwire, and the owner-alone
# fixtures alike. It also needs process visibility, so it is out of
# delegate scope.
metasystem_go_source=0
grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' go.mod && metasystem_go_source=1
if (( ! metasystem_go_source )) && [[ -f internal/missionrunner/stoploss.go || -f internal/mission/ledger.go ]]; then
  echo "metasystem Go source present but go.mod does not declare the metasystem module — damaged template" >&2
  exit 1
fi
if (( ! delegate_scope )) && (( metasystem_go_source )); then
  bash scripts/agents/go-gate.sh
  # The engine-seam tripwire and the Go-vs-python census conformance
  # harnesses (signature, fingerprint, run) retired with the migration:
  # the python reference no longer exists to diff against, and the Go
  # packages carry their own unit coverage under the go gate above.
  # Owner-alone Go supervision fixtures drive the running binary.
  bash scripts/agents/supervision-go-fixtures.sh

  # The gate fence, live: this suite's own marker never blocks (this shell
  # is the registered run's chain), a foreign live run blocks both the
  # fence and a standalone go-gate rebuild, and a dead run stops blocking.
  bin/metasystem gate fence --root "$root" --self-pid $$ \
    || { echo "the suite's own gate marker blocked its fence" >&2; exit 1; }
  sleep 60 & gate_fence_foreign=$!
  bin/metasystem gate register --root "$root" --gate fence-fixture --pid "$gate_fence_foreign" >/dev/null
  if bin/metasystem gate fence --root "$root" --self-pid $$ 2>/dev/null; then
    echo "a foreign live gate run did not block the fence" >&2; exit 1
  fi
  gate_fence_err=$(mktemp)
  if bash scripts/agents/go-gate.sh 2>"$gate_fence_err"; then
    echo "go-gate rebuilt over a foreign live gate run" >&2; exit 1
  fi
  grep -q "swap its binary mid-run" "$gate_fence_err" \
    || { echo "go-gate refusal did not come from the rebuild fence" >&2; exit 1; }
  rm -f "$gate_fence_err"
  kill "$gate_fence_foreign" 2>/dev/null; wait "$gate_fence_foreign" 2>/dev/null || true
  bin/metasystem gate fence --root "$root" --self-pid $$ \
    || { echo "a dead foreign gate run kept blocking the fence" >&2; exit 1; }
  echo "gate fence fixtures passed"
fi

source scripts/agents/fixture-budget.sh
if (( delegate_scope )); then
  # Load calibration is itself a real census. Delegate validation uses the
  # policy's minimum scale so no process enumeration occurs before the
  # process-sensitive sections are skipped.
  : "${METASYSTEM_FIXTURE_CAP_SCALE:=3}"
  export METASYSTEM_FIXTURE_CAP_SCALE
fi
harness_fixture_budget_init "$root"
fixture_minimum_cap_min=$(harness_fixture_semantic_cap minimum-minutes)
fixture_mission_job_cap_min=$(harness_fixture_semantic_cap mission-job-minutes)
fixture_dispatch_envelope_cap_min=$(harness_fixture_semantic_cap dispatch-envelope-minutes)
fixture_dispatch_over_envelope_cap_min=$(harness_fixture_semantic_cap dispatch-over-envelope-minutes)
fixture_watcher_config_cap_min=$(harness_fixture_semantic_cap watcher-config-minutes)
fixture_watcher_nonfiring_cap_min=$(harness_fixture_semantic_cap watcher-nonfiring-minutes)
fixture_watcher_firing_cap_min=$(harness_fixture_semantic_cap watcher-firing-minutes)

scripts/audit-metasystem.sh .

# The gate's own integrity (go-production-grade B8): a gofmt that cannot run
# must refuse the gate, not pass it silently. The shim exits before any
# expensive stage, so this replay is cheap; the nested gate needs the
# concurrency waiver because this suite's own run holds the fence.
if [[ -f go.mod ]] && command -v go >/dev/null 2>&1; then
  gofmt_shim_dir=$(mktemp -d)
  printf '#!/usr/bin/env bash\necho "shim: gofmt is broken" >&2\nexit 7\n' >"$gofmt_shim_dir/gofmt"
  chmod +x "$gofmt_shim_dir/gofmt"
  if METASYSTEM_ALLOW_CONCURRENT_GATE=1 PATH="$gofmt_shim_dir:$PATH" \
      bash scripts/agents/go-gate.sh >"$gofmt_shim_dir/out" 2>&1; then
    echo "go gate passed with a broken gofmt; the fail-open hole is back" >&2
    exit 1
  fi
  grep -q "gofmt itself failed" "$gofmt_shim_dir/out" \
    || { echo "go gate refused a broken gofmt without naming it" >&2; cat "$gofmt_shim_dir/out" >&2; exit 1; }
  rm -rf "$gofmt_shim_dir"
fi

# Validate every skill present, including project-added and moved optional
# skills, so this script holds in adopted repositories as well as the template.
# A skill directory without a SKILL.md is invisible to the find, so check for
# hollow directories explicitly first.
for dir in skills optional-skills; do
  [[ -d "$dir" ]] || continue
  for d in "$dir"/*/; do
    [[ -d "$d" ]] || continue
    [[ -f "${d}SKILL.md" ]] || { echo "skill directory without SKILL.md: ${d%/}" >&2; exit 1; }
  done
  while IFS= read -r skill_md; do
    scripts/validate-skill.sh "$(dirname "$skill_md")"
  done < <(find "$dir" -name SKILL.md | sort)
done

# Core assets are required everywhere. The full seven-skill set with every
# per-runtime profile is required only in the template repository, detected by
# its own directory name plus the development docs beside it: the metasystem
# never ships anything referencing outward, so the sibling test is guarded by
# the name test and adopted repositories never touch a parent path.
# Adopted repositories may prune unused skills, and
# each skill present is still validated by the loop above. In adopted mode,
# any profile a remaining skill does provide must be registered without drift;
# project-added skills are not required to invent profiles they never shipped.
template_mode=0
[[ "${metasystem_here##*/}" == metasystem && -f "${metasystem_here%/*}/development/metasystem-design.md" ]] && template_mode=1
if (( ! template_mode )); then
  scripts/metasystem-config.sh validate
fi

for link in \
  docs/project-rules.md \
  docs/orchestration.md \
  docs/collaboration.md \
  docs/design/design-principles.md \
  docs/design/design-obligation-gate.md \
  docs/examples/design-obligation-matrix.md \
  docs/examples/step-back-ledger.md \
  .gitattributes \
  plans/instruction-ledger.md \
  scripts/refactor-baseline.sh \
  scripts/frontier.sh \
  scripts/receipt.sh \
  scripts/adopt.sh \
  scripts/enforcement/github-actions-metasystem.yml \
  scripts/enforcement/claude-code-hooks.json \
  scripts/enforcement/codex-hooks.json \
  scripts/enforcement/devin-hooks.json \
  scripts/assert-stop-loss.sh \
  scripts/assert-mission.sh \
  docs/examples/mission-contract.md \
  docs/examples/mission-cron.example \
  docs/project-adaptation.md \
  docs/metasystem-reconciliation.md \
  docs/working-modes.md \
  docs/working-with-agents.md \
  plans/README.md; do
  [[ -e "$link" ]] || { echo "missing routed asset: $link" >&2; exit 1; }
done

# The agent protocol is runtime-neutral and ships in template and adopted
# repositories. Keep the six dispatchable roles in lockstep across their
# preamble, return schema, and capability declaration.
for link in \
  scripts/agents/templates/brief.md \
  scripts/agents/templates/follow-up.md \
  scripts/agents/templates/host-turn-instruction.md \
  scripts/agents/roles/orchestrator.md \
  scripts/agents/schemas/orchestrator.schema.json \
  scripts/agents/permissions/none.json \
  scripts/agents/permissions/workspace.json \
  metasystem.conf \
  scripts/metasystem-config.sh \
  scripts/agents/dispatch.sh \
  scripts/agents/commit.sh \
  scripts/agents/second-session.sh \
  scripts/agents/arm-supervision.sh \
  scripts/agents/fixture-budget.sh \
  scripts/agents/fingerprint-harness.sh \
  scripts/agents/supervision-hook.sh \
  scripts/agents/supervision-fixtures.sh \
  scripts/agents/telemetry-census-fixtures.sh \
  scripts/agents/return-schema-fixtures.sh \
  scripts/agents/config-identity-fixtures.sh \
  scripts/agents/authority-regression-fixtures.sh \
  scripts/agents/pre-commit-guard-fixtures.sh \
  scripts/agents/record-protocol-fixtures.sh \
  scripts/agents/evidence-segment-fixtures.sh \
  scripts/agents/second-session-fixtures.sh \
  scripts/agents/lease-succession-fixtures.sh \
  scripts/agents/flight-recorder-fixtures.sh \
  scripts/agents/mission-fixtures.sh \
  scripts/agents/delegate-caps-fixtures.sh \
  scripts/agents/mission-runner.sh \
  scripts/agents/hosts/claude.sh \
  scripts/agents/hosts/codex.sh \
  scripts/agents/hosts/devin.sh \
  scripts/agents/hosts/fake.sh \
  scripts/agents/schemas/mission-state.schema.json \
  scripts/agents/adapters/fake.sh \
  scripts/agents/adapters/runtime-common.sh \
  scripts/agents/adapters/codex-config-filter.v1.json \
  scripts/agents/adapters/claude-config-filter.v1.json \
  scripts/agents/adapters/devin-config-filter.v1.json \
  scripts/agents/assert-conformance.sh \
  scripts/agents/conformance-fixtures.sh \
  scripts/agents/instruction-bearing-paths.txt \
  scripts/assert-critique-closed.sh \
  scripts/assert-return-complete.sh \
  scripts/assert-turn-prompt.sh \
  scripts/agents/check-preamble-quotes.sh; do
  [[ -e "$link" ]] || { echo "missing agent protocol asset: $link" >&2; exit 1; }
done

# Section 3.11 and retained watch-list round S4 have one bounded fixture suite.
# Process-owning groups run serially and use separate temporary repositories,
# so their supervisors and dispatch jobs cannot share lifecycle state. They
# name S4-1 through S4-10 at their owning checks and contain no uncapped
# process wait (IL-1).
if [[ -z "${METASYSTEM_SKIP_AGENT_FIXTURES:-}" || $delegate_scope -eq 1 ]]; then
  if delegate_process_section "supervision and census fixtures"; then
    scripts/agents/supervision-fixtures.sh
  fi
  if delegate_process_section "supervisor fingerprint heal harness"; then
    scripts/agents/fingerprint-harness.sh --iterations 2
  fi
  scripts/agents/mission-fixtures.sh
fi

# Real runtime selftests spend model calls and remain manual acceptance steps.
# Validation covers only their static adapter contract.
bash -n scripts/agents/arm-supervision.sh
bash -n scripts/agents/fixture-budget.sh
bash -n scripts/agents/fingerprint-harness.sh
bash -n scripts/agents/supervision-hook.sh
bash -n scripts/agents/supervision-fixtures.sh
bash -n scripts/agents/telemetry-census-fixtures.sh
bash -n scripts/agents/return-schema-fixtures.sh
bash -n scripts/agents/config-identity-fixtures.sh
bash -n scripts/agents/record-protocol-fixtures.sh
bash -n scripts/agents/evidence-segment-fixtures.sh
bash -n scripts/agents/second-session-fixtures.sh
bash -n scripts/agents/lease-succession-fixtures.sh
bash -n scripts/agents/flight-recorder-fixtures.sh
bash -n scripts/agents/emit-event.sh
bash -n scripts/agents/pre-commit-guard-fixtures.sh
bash -n scripts/agents/mission-fixtures.sh
bash -n scripts/agents/delegate-caps-fixtures.sh
bash -n scripts/agents/mission-runner.sh
bash -n scripts/agents/conformance-fixtures.sh
bash -n scripts/agents/hosts/claude.sh
bash -n scripts/agents/hosts/codex.sh
bash -n scripts/agents/hosts/devin.sh
bash -n scripts/agents/hosts/fake.sh
bash -n scripts/assert-mission.sh
bash -n scripts/assert-return-complete.sh
bash -n scripts/assert-turn-prompt.sh
bash -n scripts/watch-background-jobs.sh
bash -n scripts/agents/dispatch.sh
bash -n scripts/agents/adapters/runtime-common.sh
bash scripts/agents/conformance-fixtures.sh
bash scripts/agents/telemetry-census-fixtures.sh
bash scripts/agents/return-schema-fixtures.sh
bash scripts/agents/config-identity-fixtures.sh
# worktree-lease-fixtures.py retired with the python lease helper: it
# monkeypatched that module's internals (started_at, live, classify, ...),
# which cannot be expressed against the Go engine and is owned by
# internal/lease's unit tests under the go gate. The cross-process
# behavioral coverage it also carried (succession, lock contention)
# lives in scripts/agents/lease-succession-fixtures.sh below.
bash scripts/agents/authority-regression-fixtures.sh
bash scripts/agents/pre-commit-guard-fixtures.sh
bash scripts/agents/record-protocol-fixtures.sh
bash scripts/agents/evidence-segment-fixtures.sh
bash scripts/agents/second-session-fixtures.sh
bash scripts/agents/lease-succession-fixtures.sh
bash scripts/agents/flight-recorder-fixtures.sh
bash scripts/agents/delegate-caps-fixtures.sh
[[ $(grep -Ec '^# Example model\.tier\.[123]=' metasystem.conf) -eq 3 ]] \
  || { echo "template demotion fixture: model tiers are not three commented examples" >&2; exit 1; }
[[ $(grep -Ec '^# Example mode\.[a-z0-9-]+\.role\.' metasystem.conf) -eq 3 ]] \
  || { echo "template demotion fixture: mode role overrides are not three commented examples" >&2; exit 1; }
if grep -Eq '^(model\.tier\.[1-9][0-9]*|mode\.[a-z0-9-]+\.role\.)' metasystem.conf; then
  echo "template demotion fixture: an optional tier or mode role key is still active" >&2
  exit 1
fi
python3 - scripts/enforcement/claude-code-hooks.json scripts/enforcement/codex-hooks.json scripts/enforcement/devin-hooks.json <<'PY'
import json, sys
for source in sys.argv[1:]:
    value=json.load(open(source))
    hooks=value["hooks"]
    assert set(hooks) >= {"SessionStart", "Stop", "SessionEnd"}
    flattened=json.dumps(hooks)
    assert "supervision-hook.sh" in flattened
PY
for runtime in claude codex devin; do
  adapter="scripts/agents/adapters/$runtime.sh"
  [[ -f "$adapter" ]] || { echo "missing $runtime runtime adapter: $adapter" >&2; exit 1; }
  [[ -x "$adapter" ]] || { echo "$runtime runtime adapter is not executable: $adapter" >&2; exit 1; }
  bash -n "$adapter"
  adapter_usage=$($adapter --help 2>&1)
  for verb in identity config-identity signature probe dispatch follow-up cancel selftest; do
    grep -Fq "adapters/$runtime.sh $verb" <<<"$adapter_usage" \
      || { echo "$runtime adapter usage does not advertise $verb" >&2; exit 1; }
  done
  grep -Fq "adapter_common_init $runtime" "$adapter" \
    || { echo "$runtime adapter does not bind its snapshot runtime identity" >&2; exit 1; }
  grep -Fq "write_capability_snapshot $runtime \"\$version\" \"\$hash\"" "$adapter" \
    || { echo "$runtime adapter does not write its named capability snapshot" >&2; exit 1; }
done
grep -Fq '{"writeRoots":"mapped","readRoots":"notEnforced","network":"mapped"}' \
  scripts/agents/adapters/codex.sh \
  || { echo "codex adapter envelope enforcement declaration drifted" >&2; exit 1; }
grep -Fq '{"writeRoots":"mapped","readRoots":"mapped","network":"mapped"}' \
  scripts/agents/adapters/claude.sh \
  || { echo "claude adapter envelope enforcement declaration drifted" >&2; exit 1; }
# Devin declares every member unenforced, and that is the measured truth rather
# than a weakening. --sandbox is the only flag that would enforce roots at the
# OS level and this organisation's policy refuses the mode it needs; with a
# shell granted, a Devin turn wrote outside its declared write root and read
# outside its declared read root. Both were demonstrated on 2026-08-08 and are
# recorded in plans/devin-support.md as O-9 and O-10. Changing this line back
# requires evidence that enforcement returned, which is exactly why the guard
# is here.
grep -Fq '{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}' \
  scripts/agents/adapters/devin.sh \
  || { echo "devin adapter envelope enforcement declaration drifted" >&2; exit 1; }
for runtime in claude codex fake; do
  host="scripts/agents/hosts/$runtime.sh"
  [[ -x "$host" ]] || { echo "$runtime host adapter is missing or not executable: $host" >&2; exit 1; }
  grep -Fq 'start-turn' <<<"$($host --help 2>&1)" \
    || { echo "$runtime host adapter does not advertise start-turn" >&2; exit 1; }
done
# The capability snapshot naming contract
# (<runtime>-<version>-<configHash>-<date>-<seq %03d>.json) moved into the
# engine with the python port; pin it at its Go source when that is present
# (template mode). Adopted repositories carry only the binary, whose
# selection fixtures above exercise the same names end to end.
if (( metasystem_go_source )); then
  grep -Fq 'prefix := fmt.Sprintf("%s-%s-%s-%s-", runtime, version, configHash, date)' \
    internal/adapter/snapshot.go \
    && grep -Fq 'name := fmt.Sprintf("%s%03d.json", prefix, sequence)' \
      internal/adapter/snapshot.go \
    || { echo "real adapter capability snapshot naming contract drifted" >&2; exit 1; }
fi
for role in design-critic implementer code-critic verifier investigator behavior-judge; do
  for suffix in md requirements.json; do
    [[ -f "scripts/agents/roles/$role.$suffix" ]] \
      || { echo "missing $role role asset: scripts/agents/roles/$role.$suffix" >&2; exit 1; }
  done
  [[ -f "scripts/agents/schemas/$role.schema.json" ]] \
    || { echo "missing $role return schema" >&2; exit 1; }
done

if (( template_mode )); then
  for link in \
    skills/take-a-step-back/SKILL.md \
    skills/take-a-step-back/agents/claude-profile.md \
    skills/take-a-step-back/agents/devin/AGENT.md \
    skills/take-a-step-back/agents/openai.yaml \
    skills/design-critique/SKILL.md \
    skills/design-critique/agents/claude-profile.md \
    skills/design-critique/agents/devin/AGENT.md \
    skills/design-critique/agents/openai.yaml \
    skills/code-critique/SKILL.md \
    skills/code-critique/agents/claude-profile.md \
    skills/code-critique/agents/devin/AGENT.md \
    skills/code-critique/agents/openai.yaml \
    skills/verify/SKILL.md \
    skills/verify/agents/claude-profile.md \
    skills/verify/agents/devin/AGENT.md \
    skills/verify/agents/openai.yaml \
    skills/refactor/SKILL.md \
    skills/refactor/agents/claude-profile.md \
    skills/refactor/agents/devin/AGENT.md \
    skills/refactor/agents/openai.yaml \
    skills/improve/SKILL.md \
    skills/improve/agents/claude-profile.md \
    skills/improve/agents/devin/AGENT.md \
    skills/improve/agents/openai.yaml \
    skills/retro/SKILL.md \
    skills/retro/agents/claude-profile.md \
    skills/retro/agents/devin/AGENT.md \
    skills/retro/agents/openai.yaml; do
    [[ -e "$link" ]] || { echo "missing template skill asset: $link" >&2; exit 1; }
  done
fi

# Registered skills must track their canonical source under skills/: copies
# must not drift, orphaned copies of a pruned skill must not linger, and a
# symlink to a pruned skill is dangling.
for regroot in .claude/skills .agents/skills .devin/skills; do
  [[ -d "$regroot" ]] || continue
  for reg in "$regroot"/*; do
    [[ -e "$reg" || -L "$reg" ]] || continue
    name=$(basename "$reg")
    if [[ -L "$reg" ]]; then
      [[ -e "$reg" ]] || { echo "registered skill link is dangling: $reg" >&2; exit 1; }
      continue
    fi
    [[ -d "$reg" ]] || continue
    [[ -d "skills/$name" ]] || { echo "orphaned registered skill copy: $reg has no skills/$name source" >&2; exit 1; }
    if ! diff -rq "$reg" "skills/$name" >/dev/null 2>&1; then
      echo "registered skill copy has drifted from its source: $reg vs skills/$name" >&2
      exit 1
    fi
  done
done

# In adopted mode metasystem.runtimes is live truth for registrations. Every
# remaining skill must be discoverable by each selected runtime, and copied
# launcher profiles must remain byte-identical to their canonical source.
if (( ! template_mode )); then
  configured_runtimes=$(sed -n 's/^metasystem\.runtimes=//p' metasystem.conf)
  runtime_selected() { [[ ",$configured_runtimes," == *",$1,"* ]]; }
  for skill_dir in skills/*/; do
    [[ -d "$skill_dir" ]] || continue
    name=$(basename "$skill_dir")
    if runtime_selected claude; then
      [[ -e ".claude/skills/$name" ]] \
        || { echo "claude registration missing for skill: $name" >&2; exit 1; }
      if [[ -f "$skill_dir/agents/claude-profile.md" ]]; then
        profile=".claude/agents/$name.md"
        [[ -f "$profile" ]] || { echo "claude profile registration missing: $profile" >&2; exit 1; }
        cmp -s "$skill_dir/agents/claude-profile.md" "$profile" \
          || { echo "claude profile drifted from $skill_dir/agents/claude-profile.md: $profile" >&2; exit 1; }
      fi
    fi
    if runtime_selected codex || runtime_selected devin; then
      [[ -e ".agents/skills/$name" ]] \
        || { echo "shared .agents skill registration missing: $name" >&2; exit 1; }
    fi
    if runtime_selected devin; then
      [[ -e ".devin/skills/$name" ]] \
        || { echo "devin skill registration missing: $name" >&2; exit 1; }
      if [[ -f "$skill_dir/agents/devin/AGENT.md" ]]; then
        profile=".devin/agents/$name/AGENT.md"
        [[ -f "$profile" ]] || { echo "devin profile registration missing: $profile" >&2; exit 1; }
        cmp -s "$skill_dir/agents/devin/AGENT.md" "$profile" \
          || { echo "devin profile drifted from $skill_dir/agents/devin/AGENT.md: $profile" >&2; exit 1; }
      fi
    fi
  done
fi

tmp=$(mktemp -d)
agent_supervision_repo=

# The fake runtime is the only sandbox this suite owns. Its probe drives the
# denied write and network-call paths and reports the observed nonzero status;
# real adapters are inspected only for their declarations above.
fake_probe_root="$tmp/fake-envelope-probe"
mkdir -p "$fake_probe_root/scripts/agents/adapters"
cp scripts/agents/adapters/fake.sh "$fake_probe_root/scripts/agents/adapters/"
cp scripts/agents/fixture-budget.sh "$fake_probe_root/scripts/agents/"
fake_probe_result="$tmp/fake-envelope-probe-result.json"
# The bare probe root carries no engine; point the adapter at this checkout's.
fake_snapshot=$(METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT="$fake_probe_result" \
  METASYSTEM_BIN="$PWD/bin/metasystem" \
  "$fake_probe_root/scripts/agents/adapters/fake.sh" probe)
python3 - "$fake_snapshot" "$fake_probe_result" <<'PY'
import json, sys
from pathlib import Path

snapshot = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
result = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
assert snapshot["envelopeEnforcement"] == {
    "writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped",
}
assert result == {
    "writeRoots": {"observed": "denied", "exitStatus": 77},
    "network": {"observed": "denied", "exitStatus": 77},
}
PY

# Every fixture repository this suite arms, so cleanup can stop all of them.
# A single variable was tracked before, reassigned as the suite moved between
# repositories and emptied at the end, so the trap shut down nothing and each
# armed repository leaked its supervision owner. Two such owners were found
# alive after 25 hours, each scanning every process on the machine every five
# seconds. Killing components does not help: the owner restarts them by design,
# so only a shutdown of the owner ends it.
armed_supervision_repos=()
track_armed_supervision() { # repository
  local repo=$1 known
  [[ -n "$repo" ]] || return 0
  for known in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ "$known" == "$repo" ]] && return 0
  done
  armed_supervision_repos+=("$repo")
}
validation_cleanup() {
  [[ -z "${gate_run_marker:-}" ]] || rm -f "$gate_run_marker"
  local repo
  for repo in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ -x "$repo/scripts/agents/arm-supervision.sh" ]] || continue
    if [[ "$repo" == "${runner_repo:-}" ]] && declare -p runner_process_env >/dev/null 2>&1; then
      "${runner_process_env[@]}" "$repo/scripts/agents/arm-supervision.sh" \
        --repo "$repo" --shutdown >/dev/null 2>&1 || true
    else
      "$repo/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown >/dev/null 2>&1 || true
    fi
  done
  # Shutdown returns before the owner and its children have fully exited, and
  # a straggler writing one last record while rm walks the tree turns a PASSED
  # validation into a red exit ("Directory not empty"). Wait for every process
  # still referencing the sandbox to finish, bounded, then delete; the leak
  # guard is this wait, not the rm exit code, so a residual wrinkle in the
  # teardown never overrules the verdict printed above it.
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    pgrep -f "$tmp" >/dev/null 2>&1 || break
    sleep 0.5
  done
  # PRESERVE FAILURE EVIDENCE (flight-recorder D-8, direct-fix bar): a
  # failing run's temp tree is the diagnosis, and deleting it forced every
  # investigation to re-run the suite with the trap stripped by hand. On a
  # nonzero exit the tree moves aside and its path is printed; only green
  # runs clean up. Evidence beats disk.
  if [[ "${validation_exit_status:-1}" != 0 && -d "$tmp" ]]; then
    keep="artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-$$"
    mkdir -p "$(dirname "$keep")"
    if mv "$tmp" "$keep" 2>/dev/null; then
      echo "suite failure evidence preserved: $keep" >&2
    fi
    return 0
  fi
  rm -rf "$tmp" 2>/dev/null || { sleep 1; rm -rf "$tmp" 2>/dev/null || true; }
}
trap 'validation_exit_status=$?; validation_cleanup' EXIT

# IL-3: prove the audit's fallback with a PATH that contains its ordinary POSIX
# tools but deliberately contains no rg binary.
no_rg_bin="$tmp/no-rg-bin"
mkdir -p "$no_rg_bin"
for command_name in cat find grep sort tr wc; do
  ln -s "$(command -v "$command_name")" "$no_rg_bin/$command_name"
done
env PATH="$no_rg_bin" /bin/bash scripts/audit-metasystem.sh . >"$tmp/audit-no-rg.out"

fill_harness_conf() { # config path, absolute evidence root
  python3 - "$1" "$2" <<'PY'
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
evidence = sys.argv[2]
model_key = re.compile(
    r"^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$"
)
lines = []
models = set()
for raw in path.read_text(encoding="utf-8").splitlines():
    if "=" not in raw:
        lines.append(raw)
        continue
    key, value = raw.split("=", 1)
    match = model_key.fullmatch(key)
    if key == "evidence.root":
        value = evidence
    elif match:
        runtime = match.group(1)
        value = f"fixture-{runtime}-model"
        models.add(f"{runtime}:{value}")
    elif key == "model.tier.1":
        value = "__MODELS__"
    elif key.startswith("model.tier."):
        value = ""
    lines.append(f"{key}={value}")
joined = "\n".join(lines).replace("model.tier.1=__MODELS__", f"model.tier.1={','.join(sorted(models))}")
path.write_text(joined + "\n", encoding="utf-8")
PY
}

# The brief contains only orchestrator-authored header fields. Dispatch owns
# job identity, role, runtime, model, and round, so none may appear as a brief
# header before those values exist.
brief=scripts/agents/templates/brief.md
for header in 'Working Mode:' 'Mission Stream:' 'Orchestrator Identity:' 'Date:'; do
  grep -q "^$header" "$brief" || { echo "brief template is missing authored header: $header" >&2; exit 1; }
done
if grep -Eq '^(Job-Id|Role|Runtime|Model|Round):' "$brief"; then
  echo "brief template contains a dispatch-assigned header" >&2
  exit 1
fi
grep -q '^Finding Id:' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template does not restate one finding" >&2; exit 1; }
grep -q '^Disposition:' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template does not restate the disposition" >&2; exit 1; }
grep -q '^# Unchanged Return Contract$' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template can lose the original return contract" >&2; exit 1; }
python3 - scripts/agents/templates/host-turn-instruction.md <<'PY'
import re
import sys
from pathlib import Path

body = Path(sys.argv[1]).read_text(encoding="utf-8")
parameters = re.findall(r"<([^<>]+)>", body)
if parameters != ["cycle-number", "fence-headroom", "yes | no"]:
    raise SystemExit("host-turn instruction parameters drifted from cycle, fence headroom, reconciliation")
if "Runtime:" in body:
    raise SystemExit("host-turn instruction is parameterized by runtime")
PY

# Assert the machine-readable shapes item 2 owns. Later dispatcher fixtures
# exercise expansion and capability gating; here the shipped declarations
# must remain minimal and must not grow baseline guarantees into snapshots.
python3 - "$root" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
common = {"jobId", "round", "runtime", "sessionId", "model", "evidence", "gaps", "mode"}
role_fields = {
    "design-critic": {"reviewedCommit", "findings", "verdictMaterialCount"},
    "implementer": {"riskiestPart", "diffBoundary", "whatWasDone"},
    "code-critic": {"reviewedTree", "findings", "verdictMaterialCount"},
    "verifier": {"riskiestPart", "whatWasDone"},
    "investigator": {"frozenFrame", "theories", "classifications", "stopLoss"},
    "behavior-judge": {"dimensions", "reliabilityWatch"},
}
for role, owned in role_fields.items():
    path = root / "scripts" / "agents" / "schemas" / f"{role}.schema.json"
    schema = json.loads(path.read_text())
    expected = common | owned
    actual = set(schema.get("properties", {}))
    required = set(schema.get("required", []))
    if actual != expected or required != expected or schema.get("additionalProperties") is not False:
        raise SystemExit(f"{role} schema property set drifted from the protocol")

orchestrator_path = root / "scripts" / "agents" / "schemas" / "orchestrator.schema.json"
orchestrator = json.loads(orchestrator_path.read_text())
orchestrator_fields = {
    "turnId", "missionId", "cycle", "dispatched", "certified",
    "streamUpdatesRequested", "askCandidates", "factsForLedger", "gaps", "identity",
}
if (
    set(orchestrator.get("properties", {})) != orchestrator_fields
    or set(orchestrator.get("required", [])) != orchestrator_fields
    or orchestrator.get("additionalProperties") is not False
):
    raise SystemExit("orchestrator schema property set drifted from the host-turn protocol")


def assert_closed_schema(node, path):
    if node.get("type") == "object":
        properties = node.get("properties", {})
        if node.get("additionalProperties") is not False or set(node.get("required", [])) != set(properties):
            raise SystemExit(f"orchestrator schema object is not fully enumerated: {path}")
        for name, child in properties.items():
            assert_closed_schema(child, f"{path}.{name}")
    if node.get("type") == "array":
        assert_closed_schema(node.get("items", {}), f"{path}[]")


assert_closed_schema(orchestrator, "$")

permission_expected = {
    # Network is granted by default: the container or VM is the isolation
    # boundary, and a delegate that cannot resolve a dependency or read
    # documentation cannot do real work. A repository narrows it for every role
    # with dispatch.permissions.network=deny.
    "none": {
        "readRoots": ["."], "writeRoots": [], "network": "allow",
        "approvals": "deny", "tools": "read-only",
    },
    "workspace": {
        "readRoots": ["."], "writeRoots": ["<worktree>"], "network": "allow",
        "approvals": "deny", "tools": "runtime-default",
    },
}
for name, expected in permission_expected.items():
    path = root / "scripts" / "agents" / "permissions" / f"{name}.json"
    if json.loads(path.read_text()) != expected:
        raise SystemExit(f"{name} permission preset drifted from its envelope")

for role in role_fields:
    path = root / "scripts" / "agents" / "roles" / f"{role}.requirements.json"
    requirement = json.loads(path.read_text())
    if not {"required", "optional"}.issubset(requirement) or not set(requirement).issubset({"required", "optional", "waivers"}):
        raise SystemExit(f"{role} capability declaration has unknown top-level fields")
    waivers = requirement.get("waivers", {})
    if not isinstance(waivers, dict) or any(
        not isinstance(field, str) or not isinstance(runtimes, list)
        or not all(isinstance(runtime, str) for runtime in runtimes)
        for field, runtimes in waivers.items()
    ):
        raise SystemExit(f"{role} capability waivers have an invalid shape")
    if requirement["required"] != []:
        raise SystemExit(f"{role} incorrectly repeats adapter-guaranteed baseline capabilities")
    if role == "implementer":
        resume = requirement["optional"].get("resume")
        if set(requirement["optional"]) != {"resume"} or not isinstance(resume, dict) or not resume.get("fallback"):
            raise SystemExit("implementer resume capability lacks its embed fallback")
    elif requirement["optional"] != {}:
        raise SystemExit(f"{role} declares a variable capability it does not need")
PY

# Quote markers name their canonical source. The checker compares the content
# bytes rather than trusting a second prose copy of the binding criterion.
scripts/agents/check-preamble-quotes.sh
python3 - scripts/agents/roles/orchestrator.md <<'PY'
import re
import sys
from pathlib import Path

body = Path(sys.argv[1]).read_bytes()
pattern = re.compile(
    br'^<!-- quote source="([^"\r\n]+)" -->\n(.*?)^<!-- /quote -->$',
    re.MULTILINE | re.DOTALL,
)
required = [
    (b"AGENTS.md", b"## Completion\n"),
    (b"docs/orchestration.md", b"## Delegation Contract\n"),
    (b"docs/orchestration.md", b"### Working without the human\n"),
    (b"docs/collaboration.md", b"## Review Guide in Reports\n"),
    (b"docs/collaboration.md", b"## Escalation Shape\n"),
    (b"docs/project-rules.md", b"These require explicit in-task approval"),
]


def matches(value):
    return list(pattern.finditer(value))


def missing(value):
    blocks = matches(value)
    return [
        (source, marker)
        for source, marker in required
        if not any(match.group(1) == source and marker in match.group(2) for match in blocks)
    ]


absent = missing(body)
if absent:
    raise SystemExit(f"orchestrator preamble lacks mandated quote blocks: {absent!r}")
for source, marker in required:
    match = next(
        item for item in matches(body)
        if item.group(1) == source and marker in item.group(2)
    )
    deleted = body[:match.start()] + body[match.end():]
    if (source, marker) not in missing(deleted):
        raise SystemExit(
            f"orchestrator required-block fixture did not detect deletion: {source!r} {marker!r}"
        )
PY
cp -R scripts/agents/roles "$tmp/drifted-roles"
sed 's/build something DIFFERENT/build the same thing/' \
  "$tmp/drifted-roles/design-critic.md" >"$tmp/drifted-roles/design-critic.md.new"
mv "$tmp/drifted-roles/design-critic.md.new" "$tmp/drifted-roles/design-critic.md"
set +e
scripts/agents/check-preamble-quotes.sh --roles-dir "$tmp/drifted-roles" >"$tmp/quote-drift.out" 2>&1
quote_status=$?
set -e
if [[ $quote_status -eq 0 ]]; then
  echo "preamble quote checker accepted a drifted binding criterion" >&2
  exit 1
fi
[[ $quote_status -eq 1 ]] \
  || { echo "preamble quote checker used $quote_status instead of exit 1 for drift" >&2; exit 1; }
grep -q 'quote drifted from skills/design-critique/SKILL.md' "$tmp/quote-drift.out" \
  || { echo "preamble quote checker did not name the drifted source" >&2; exit 1; }
cp -R scripts/agents/roles "$tmp/drifted-code-roles"
sed 's/ship a defect/ship no defect/' \
  "$tmp/drifted-code-roles/code-critic.md" >"$tmp/drifted-code-roles/code-critic.md.new"
mv "$tmp/drifted-code-roles/code-critic.md.new" "$tmp/drifted-code-roles/code-critic.md"
set +e
scripts/agents/check-preamble-quotes.sh --roles-dir "$tmp/drifted-code-roles" >"$tmp/code-quote-drift.out" 2>&1
code_quote_status=$?
set -e
[[ $code_quote_status -eq 1 ]] \
  || { echo "preamble quote checker accepted a drifted code-critique criterion" >&2; exit 1; }
grep -q 'quote drifted from skills/code-critique/SKILL.md' "$tmp/code-quote-drift.out" \
  || { echo "preamble quote checker did not name the code-critique source" >&2; exit 1; }
cp -R scripts/agents/roles "$tmp/drifted-orchestrator-roles"
sed "s/The human's absence narrows/The human's presence narrows/" \
  "$tmp/drifted-orchestrator-roles/orchestrator.md" >"$tmp/drifted-orchestrator-roles/orchestrator.md.new"
mv "$tmp/drifted-orchestrator-roles/orchestrator.md.new" \
  "$tmp/drifted-orchestrator-roles/orchestrator.md"
set +e
scripts/agents/check-preamble-quotes.sh \
  --roles-dir "$tmp/drifted-orchestrator-roles" >"$tmp/orchestrator-quote-drift.out" 2>&1
orchestrator_quote_status=$?
set -e
[[ $orchestrator_quote_status -eq 1 ]] \
  || { echo "preamble quote checker accepted a drifted orchestrator quote" >&2; exit 1; }
grep -q 'quote drifted from docs/orchestration.md' "$tmp/orchestrator-quote-drift.out" \
  || { echo "preamble quote checker did not name the drifted orchestrator source" >&2; exit 1; }

# Build canonical positive returns and one role-specific negative per role.
# JSON remains canonical; these fixtures never rely on the Markdown view.
return_fixtures="$tmp/returns"
mkdir -p "$return_fixtures"
python3 - "$return_fixtures" <<'PY'
import copy
import json
import sys
from pathlib import Path

out = Path(sys.argv[1])
common = {
    "jobId": "fixture-job",
    "round": 1,
    "runtime": "fake",
    "sessionId": "session-1",
    "model": {"requested": "fake-model", "effective": "fake-model"},
    "evidence": [{"command": "scripts/validate-metasystem.sh", "observed": "fixture output", "level": "ran"}],
    "gaps": [],
    "mode": "implement",
}
behavior_dimension_ids = [
    "brief-quality",
    "adjudication-quality",
    "delegation-discipline",
    "gap-handling",
    "spec-fidelity",
    "repeated-work",
    "proportionality",
    "evidence-honesty",
]
behavior_dimensions = [
    {
        "id": dimension_id,
        "score": 4,
        "rationale": "fixture judgment",
        "anchors": [{"file": "artifacts/agents/missions/fixture/ledger.md", "line": 1}],
        "findings": [],
    }
    for dimension_id in behavior_dimension_ids
]
behavior_dimensions[0]["findings"] = [{
    "id": "BQ-1",
    "claim": "fixture finding",
    "evidence": "fixture evidence",
    "anchors": [{"file": "artifacts/agents/fixture/rounds/1/prompt.md", "line": 10}],
}]
positive = {
    "orchestrator": {
        "turnId": "turn-3",
        "missionId": "fixture-mission",
        "cycle": 3,
        "dispatched": [{"jobId": "fixture-job", "role": "implementer", "stream": "stream-a"}],
        "certified": [{"jobId": "prior-job", "verdict": "accepted", "evidence": "focused checks passed"}],
        "streamUpdatesRequested": [{"streamId": "stream-a", "requestedState": "active", "reason": "work remains"}],
        "askCandidates": [{"streamId": "stream-b", "reasonClass": "reserved-decision", "question": "Approve the contract change?"}],
        "factsForLedger": ["focused check exposed one new fact"],
        "gaps": [],
        "identity": {"runtime": "fake", "model": "fake-model", "sessionId": None},
    },
    "design-critic": {
        **common,
        "mode": "design",
        "reviewedCommit": "0" * 40,
        "findings": [{"id": "F-1", "severity": "high", "material": True, "claim": "contract gap", "evidence": "read design"}],
        "verdictMaterialCount": 1,
    },
    "code-critic": {
        **common,
        "reviewedTree": "0" * 40,
        "findings": [],
        "verdictMaterialCount": 0,
    },
    "implementer": {
        **common,
        "riskiestPart": "schema boundary",
        "diffBoundary": ["scripts/example.sh"],
        "whatWasDone": "implemented the brief",
    },
    "verifier": {
        **common,
        "mode": "verify",
        "riskiestPart": "failure path",
        "whatWasDone": "drove the runnable surface",
    },
    "investigator": {
        **common,
        "mode": "take-a-step-back",
        "frozenFrame": "symptom and boundary frozen",
        "theories": [{"statement": "owner lost state", "evidenceFor": "trace", "evidenceAgainst": "focused check"}],
        "classifications": ["falsified-continue"],
        "stopLoss": {"triggered": False, "trigger": None},
    },
    "behavior-judge": {
        **common,
        "mode": "verify",
        "dimensions": behavior_dimensions,
        "reliabilityWatch": [{
            "dimension": "proportionality",
            "mechanicalMetric": "fence-economy",
            "agreement": "agrees",
            "explanation": "fixture agreement",
            "anchors": [{"file": "artifacts/agents/missions/fixture/state.json", "line": 5}],
        }],
    },
}
for role, value in positive.items():
    (out / f"{role}-positive.json").write_text(json.dumps(value, indent=2) + "\n")

negative = copy.deepcopy(positive)
negative["orchestrator"].pop("factsForLedger")
negative["design-critic"].pop("findings")
negative["design-critic"].pop("verdictMaterialCount")
negative["code-critic"]["whatWasDone"] = "critics do not own this section"
negative["implementer"].pop("diffBoundary")
negative["verifier"]["diffBoundary"] = ["not verifier-owned"]
negative["investigator"].pop("frozenFrame")
negative["investigator"].pop("theories")
negative["behavior-judge"]["dimensions"][0]["findings"][0]["anchors"] = []
for role, value in negative.items():
    (out / f"{role}-negative.json").write_text(json.dumps(value, indent=2) + "\n")

empty_dimensions = copy.deepcopy(positive["behavior-judge"])
empty_dimensions["dimensions"] = []
(out / "behavior-judge-empty-dimensions.json").write_text(json.dumps(empty_dimensions, indent=2) + "\n")

invalid_anchor = copy.deepcopy(positive["behavior-judge"])
invalid_anchor["dimensions"][0]["anchors"][0]["line"] = 0
(out / "behavior-judge-invalid-anchor.json").write_text(json.dumps(invalid_anchor, indent=2) + "\n")

miscount = copy.deepcopy(positive["design-critic"])
miscount["verdictMaterialCount"] = 0
(out / "critic-miscount.json").write_text(json.dumps(miscount, indent=2) + "\n")
missing_verdict = copy.deepcopy(positive["design-critic"])
missing_verdict.pop("verdictMaterialCount")
(out / "critic-missing-verdict.json").write_text(json.dumps(missing_verdict, indent=2) + "\n")
PY

for role in orchestrator design-critic implementer code-critic verifier investigator behavior-judge; do
  scripts/assert-return-complete.sh --role "$role" --file "$return_fixtures/$role-positive.json"
done

check_bad_return() { # role, file, required diagnostic text
  local role=$1 file=$2 expected=$3 output status
  output="$tmp/${role}-negative.out"
  set +e
  scripts/assert-return-complete.sh --role "$role" --file "$file" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "return checker accepted the negative $role fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "return checker used $status instead of exit 1 for $role validation" >&2; exit 1; }
  grep -Fq "$expected" "$output" \
    || { echo "return checker did not name the $role violation: $expected" >&2; exit 1; }
}

check_bad_return orchestrator "$return_fixtures/orchestrator-negative.json" '$.factsForLedger is required'
check_bad_return design-critic "$return_fixtures/design-critic-negative.json" '$.findings is required'
check_bad_return code-critic "$return_fixtures/code-critic-negative.json" '$.whatWasDone is not allowed'
check_bad_return implementer "$return_fixtures/implementer-negative.json" '$.diffBoundary is required'
check_bad_return verifier "$return_fixtures/verifier-negative.json" '$.diffBoundary is not allowed'
check_bad_return investigator "$return_fixtures/investigator-negative.json" '$.frozenFrame is required'
check_bad_return behavior-judge "$return_fixtures/behavior-judge-negative.json" '$.dimensions[0].findings[0].anchors must contain at least one file-and-line anchor'
check_bad_return behavior-judge "$return_fixtures/behavior-judge-empty-dimensions.json" '$.dimensions must contain at least one requested judged dimension with no duplicate ids'
check_bad_return behavior-judge "$return_fixtures/behavior-judge-invalid-anchor.json" '$.dimensions[0].anchors[0].line must be a positive one-based line number'
check_bad_return design-critic "$return_fixtures/critic-missing-verdict.json" '$.verdictMaterialCount is required'
check_bad_return design-critic "$return_fixtures/critic-miscount.json" '$.verdictMaterialCount must equal the count of findings with material=true'

if (( template_mode )); then
  set +e
  scripts/agents/dispatch.sh dispatch --role code-critic --brief scripts/agents/templates/brief.md \
    >"$tmp/code-critic-missing-reviews.out" 2>&1
  missing_reviews_status=$?
  scripts/agents/dispatch.sh dispatch --role implementer --brief scripts/agents/templates/brief.md --reviews fixture-job \
    >"$tmp/non-critic-reviews.out" 2>&1
  non_critic_reviews_status=$?
  set -e
  [[ $missing_reviews_status -eq 2 ]] \
    || { echo "code-critic dispatch without --reviews did not use exit 2" >&2; exit 1; }
  grep -Fq 'code-critic dispatch requires --reviews <implementer-job-id>' "$tmp/code-critic-missing-reviews.out" \
    || { echo "code-critic dispatch did not require its review relation" >&2; exit 1; }
  [[ $non_critic_reviews_status -eq 2 ]] \
    || { echo "non-critic --reviews dispatch did not use exit 2" >&2; exit 1; }
  grep -Fq -- '--reviews is only valid for the code-critic role' "$tmp/non-critic-reviews.out" \
    || { echo "dispatcher accepted --reviews for a non-critic role" >&2; exit 1; }
fi

set +e
scripts/assert-return-complete.sh >"$tmp/return-usage.out" 2>&1
return_usage_status=$?
set -e
[[ $return_usage_status -eq 2 ]] \
  || { echo "return checker used $return_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Host-turn prompts are checked against the canonical turn record and shipped
# preamble before a host process may start. The positive prompt is deliberately
# hand-authored so this fixture does not share assembly logic with the checker.
turn_fixture="$tmp/turn-prompt"
turn_dir="$turn_fixture/turn-3"
mkdir -p "$turn_dir"
cat >"$turn_dir/turn.json" <<'EOF'
{
  "missionId": "fixture-mission",
  "turnId": "turn-3",
  "cycle": 3,
  "runtime": "fake",
  "model": "fake-model",
  "hostSession": null,
  "reconciliation": false,
  "startedAt": "2026-08-04T12:00:00Z",
  "pid": 1234,
  "outcome": null
}
EOF
good_turn_prompt="$turn_fixture/good.md"
{
  printf '%s\n' \
    'Mission-Id: fixture-mission' \
    'Turn-Id: turn-3' \
    'Cycle: 3' \
    'Host-Session: none' \
    'Runtime: fake' \
    'Model: fake-model' \
    'Reconciliation: no'
  printf '\n'
  cat scripts/agents/roles/orchestrator.md
  printf '\n'
  cat <<'EOF'
## Mission Contract
Signed fixture mission contract.

## Ledger Tail
<<<DATA>>>
1	contract-improved	aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	metric=1
2	unresolved	bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb	metric=1
<<<END>>>

## Open Asks
<<<DATA>>>
ask-1	stream-a	reserved-decision	Approve the named API contract?
<<<END>>>

## Streams
<<<DATA>>>
stream-a	active	Make the fixture gate pass	none
stream-b	parked-reserved	Publish the fixture	Awaiting approval
<<<END>>>

## Reconciliation
<<<DATA>>>
(none)
<<<END>>>

## Landed Returns
<<<DATA>>>
chain-a	2	artifacts/agents/chain-a/rounds/2/return.json
chain-b	invalid	artifacts/agents/chain-b/rounds/1/return.json
chain-c	unreadable	none
<<<END>>>

## This Turn
Cycle: 3
Fence headroom: cycles=2,jobs=3
Reconciliation: no

Advance active streams by designing, dispatching, reviewing, and certifying. When Reconciliation is `yes`, reconcile the prior turn before starting new work. End this turn when work is dispatched and reviewed; never wait inside the turn.
EOF
} >"$good_turn_prompt"

scripts/assert-turn-prompt.sh --file "$good_turn_prompt" --turn "$turn_dir"

python3 - "$good_turn_prompt" "$turn_fixture" <<'PY'
import sys
from pathlib import Path

source = Path(sys.argv[1]).read_text(encoding="utf-8")
out = Path(sys.argv[2])
mutations = {
    "missing-header": source.replace("Model: fake-model\n", "", 1),
    "turn-mismatch": source.replace("Turn-Id: turn-3\n", "Turn-Id: turn-other\n", 1),
    "mission-mismatch": source.replace("Mission-Id: fixture-mission\n", "Mission-Id: other-mission\n", 1),
    "altered-preamble": source.replace(
        "You are the orchestrator for an unattended mission.",
        "You are an orchestrator for an unattended mission.",
        1,
    ),
    "headings-out-of-order": source.replace("## Open Asks", "## TEMP", 1)
        .replace("## Streams", "## Open Asks", 1)
        .replace("## TEMP", "## Streams", 1),
    "unfenced-data": source.replace(
        "## Open Asks\n<<<DATA>>>\nask-1\tstream-a\treserved-decision\tApprove the named API contract?\n<<<END>>>",
        "## Open Asks\nask-1\tstream-a\treserved-decision\tApprove the named API contract?",
        1,
    ),
    "malformed-record": source.replace(
        "ask-1\tstream-a\treserved-decision\tApprove the named API contract?",
        "ask-1\tstream-a\treserved-decision",
        1,
    ),
}
for name, value in mutations.items():
    if value == source:
        raise SystemExit(f"turn-prompt mutation did not change fixture: {name}")
    (out / f"{name}.md").write_text(value, encoding="utf-8")
PY

check_bad_turn_prompt() { # fixture name, failing check
  local name=$1 expected=$2 output status
  output="$turn_fixture/$name.out"
  set +e
  scripts/assert-turn-prompt.sh \
    --file "$turn_fixture/$name.md" --turn "$turn_dir" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "turn prompt checker accepted the negative $name fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "turn prompt checker used $status instead of exit 1 for $name" >&2; exit 1; }
  grep -Fq "[$expected]" "$output" \
    || { echo "turn prompt checker did not name the $expected check for $name" >&2; exit 1; }
}

check_bad_turn_prompt missing-header headers
check_bad_turn_prompt turn-mismatch identity
check_bad_turn_prompt mission-mismatch identity
check_bad_turn_prompt altered-preamble preamble
check_bad_turn_prompt headings-out-of-order headings
check_bad_turn_prompt unfenced-data fencing
check_bad_turn_prompt malformed-record records

set +e
scripts/assert-turn-prompt.sh >"$turn_fixture/usage.out" 2>&1
turn_prompt_usage_status=$?
set -e
[[ $turn_prompt_usage_status -eq 2 ]] \
  || { echo "turn prompt checker used $turn_prompt_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Critique closure joins the canonical return JSON against the one Markdown
# dispositions table. Reuse the item-2 return fixture shape and vary only the
# findings and table rows needed to exercise each join invariant.
critique_fixtures="$tmp/critiques"
mkdir -p "$critique_fixtures"
python3 - "$return_fixtures/design-critic-positive.json" "$critique_fixtures" <<'PY'
import copy
import json
import sys
from pathlib import Path

source = json.loads(Path(sys.argv[1]).read_text())
out = Path(sys.argv[2])
material = {
    "id": "F-1", "severity": "high", "material": True,
    "claim": "contract gap", "evidence": "read design",
}
nonmaterial = {
    "id": "F-2", "severity": "low", "material": False,
    "claim": "wording issue", "evidence": "read design",
}
second_material = {
    "id": "F-3", "severity": "medium", "material": True,
    "claim": "incorrect premise", "evidence": "checked implementation",
}


def write_return(name, findings):
    value = copy.deepcopy(source)
    value["findings"] = findings
    value["verdictMaterialCount"] = sum(1 for finding in findings if finding["material"])
    (out / f"{name}.json").write_text(json.dumps(value, indent=2) + "\n")


def write_table(name, rows, separator="| --- | --- | --- | --- |"):
    lines = [
        "| Finding id | Disposition | Reasoning and evidence | Amendment |",
        separator,
        *[f"| {finding_id} | {disposition} | {reasoning} | {amendment} |"
          for finding_id, disposition, reasoning, amendment in rows],
    ]
    (out / f"{name}.md").write_text("\n".join(lines) + "\n")


write_return("joinable", [material, nonmaterial, second_material])
write_table("all-disposed", [
    ("F-1", "accepted", "design amended", "section 3"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("open-material", [
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("noted-on-material", [
    ("F-1", "noted", "incorrect disposition", "none"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("missing-nonmaterial-disposition", [
    ("F-1", "accepted", "design amended", "section 3"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("unknown-disposition", [
    ("F-1", "dismissed", "not a protocol value", "none"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("unknown-finding-id", [
    ("F-1", "accepted", "design amended", "section 3"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
    ("F-404", "refuted", "no matching finding", "none"),
])

write_return("duplicate-id", [material, material, nonmaterial, second_material])
write_table("duplicate-id", [
    ("F-1", "accepted", "first row", "section 3"),
    ("F-1", "refuted", "second row", "none"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])

missing_findings = copy.deepcopy(source)
missing_findings.pop("findings")
(out / "unjoinable-missing-findings.json").write_text(
    json.dumps(missing_findings, indent=2) + "\n"
)
write_table("unjoinable-malformed-table", [], separator="| --- | --- | --- |")
PY

scripts/assert-critique-closed.sh \
  --findings "$critique_fixtures/joinable.json" \
  --dispositions "$critique_fixtures/all-disposed.md"

# Plan consistency: a rule stated in several places must not disagree with
# itself. Eight of nine rounds of one design critique found nothing else, and a
# paid round is the wrong instrument for drift a script finds instantly.
# This repository builds the metasystem and must run under it. Its own hooks were
# never installed: adopt.sh writes .claude/settings.json into adopted targets,
# and the template never adopts itself, so for the whole of development the
# session-start arming, the untracked-process report, the stale-supervisor
# warning and the open-work check were inert here. Everything was fixtured and
# nothing was live. This check is why that cannot recur silently.
# Template repository only. An adopted copy gets its hooks from adopt.sh at its
# own root, with a different layout; this is about the repository that builds the
# metasystem running under it.
if [[ "${metasystem_here##*/}" == metasystem && -f "${metasystem_here%/*}/development/metasystem-design.md" ]]; then
  harness_own_settings=$(cd "$root/.." && pwd -P)/.claude/settings.json
  [[ -f "$harness_own_settings" ]] \
    || { echo "this repository has no .claude/settings.json: the metasystem is not running under itself" >&2; exit 1; }
  "$root/bin/metasystem" hooks check "$harness_own_settings" \
    "$root/scripts/enforcement/claude-code-hooks.json"
  echo "metasystem runs under its own hooks"
fi

scripts/assert-plan-consistency.sh >"$tmp/plan-consistency.out"
grep -q 'retired term' "$tmp/plan-consistency.out" \
  || { echo "plan consistency check did not report its retired terms" >&2; exit 1; }

plan_fixture=$tmp/plan-consistency
mkdir -p "$plan_fixture"
cat >"$plan_fixture/owner.md" <<'FIXTURE'
RETIRED: widget check -- the gadget check
FIXTURE
cat >"$plan_fixture/clean.md" <<'FIXTURE'
The widget check was replaced by the gadget check.
FIXTURE
scripts/assert-plan-consistency.sh --plans-dir "$plan_fixture" >/dev/null \
  || { echo "plan consistency rejected a line that explains the change" >&2; exit 1; }
cat >"$plan_fixture/stale.md" <<'FIXTURE'
Test quality is measured by the widget check.
FIXTURE
set +e
scripts/assert-plan-consistency.sh --plans-dir "$plan_fixture" >"$tmp/plan-stale.out" 2>&1
plan_status=$?
set -e
(( plan_status == 1 )) \
  || { echo "plan consistency did not fail on a prescribed retired term" >&2; exit 1; }
grep -q "stale.md:1: prescribes 'widget check'" "$tmp/plan-stale.out" \
  || { echo "plan consistency did not name the file, line, and term" >&2; cat "$tmp/plan-stale.out" >&2; exit 1; }

check_open_critique() { # fixture name, findings file, dispositions file, diagnostics...
  local name=$1 findings_file=$2 dispositions_file=$3 output status expected
  shift 3
  output="$tmp/critique-$name.out"
  set +e
  scripts/assert-critique-closed.sh \
    --findings "$findings_file" \
    --dispositions "$dispositions_file" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "critique checker accepted the negative $name fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "critique checker used $status instead of exit 1 for $name" >&2; exit 1; }
  for expected in "$@"; do
    grep -Fq "$expected" "$output" || {
      echo "critique checker did not name the $name violation: $expected" >&2
      cat "$output" >&2
      exit 1
    }
  done
}

check_open_critique open-material \
  "$critique_fixtures/joinable.json" "$critique_fixtures/open-material.md" \
  "finding id 'F-1' has no disposition row"
check_open_critique noted-on-material \
  "$critique_fixtures/joinable.json" "$critique_fixtures/noted-on-material.md" \
  "material finding id 'F-1' cannot use disposition 'noted'"
check_open_critique missing-nonmaterial-disposition \
  "$critique_fixtures/joinable.json" "$critique_fixtures/missing-nonmaterial-disposition.md" \
  "finding id 'F-2' has no disposition row"
check_open_critique duplicate-id \
  "$critique_fixtures/duplicate-id.json" "$critique_fixtures/duplicate-id.md" \
  "duplicate finding id: 'F-1'" "duplicate disposition id: 'F-1'"
check_open_critique unknown-disposition \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unknown-disposition.md" \
  "disposition for finding id 'F-1' has unknown value 'dismissed'"
check_open_critique unknown-finding-id \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unknown-finding-id.md" \
  "disposition names unknown finding id: 'F-404'"
check_open_critique unjoinable-format-missing-findings \
  "$critique_fixtures/unjoinable-missing-findings.json" "$critique_fixtures/all-disposed.md" \
  '$.findings array is missing'
check_open_critique unjoinable-format-malformed-table \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unjoinable-malformed-table.md" \
  "malformed dispositions table: invalid separator row"

set +e
scripts/assert-critique-closed.sh >"$tmp/critique-usage.out" 2>&1
critique_usage_status=$?
set -e
[[ $critique_usage_status -eq 2 ]] \
  || { echo "critique checker used $critique_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Job mode derives the schema and return path from the record and then checks
# all four identity fields, one at a time, against a schema-valid return.
job_fixture="$tmp/job-metasystem"
mkdir -p "$job_fixture/scripts/agents" \
  "$job_fixture/artifacts/agents/jobs" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/1"
cp scripts/assert-return-complete.sh "$job_fixture/scripts/"
# The copied assert script resolves its engine as <fixture>/bin/metasystem;
# give the fixture checkout the real one (the python schema helper it used
# to copy is gone — the binary materializes schemas itself).
mkdir -p "$job_fixture/bin"
cp bin/metasystem "$job_fixture/bin/metasystem"
cp -R scripts/agents/schemas "$job_fixture/scripts/agents/"
cat >"$job_fixture/artifacts/agents/jobs/fixture-job.json" <<'EOF'
{
  "jobId": "fixture-job",
  "role": "implementer",
  "round": 1,
  "parentJob": null,
  "runtime": "fake",
  "sessionId": "session-1"
}
EOF
cp "$return_fixtures/implementer-positive.json" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/1/return.json"
(cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job)

mkdir -p "$job_fixture/artifacts/agents/fixture-job/rounds/2"
cat >"$job_fixture/artifacts/agents/jobs/fixture-job-r2.json" <<'EOF'
{
  "jobId": "fixture-job-r2",
  "role": "implementer",
  "round": 2,
  "parentJob": "fixture-job",
  "runtime": "fake",
  "sessionId": "session-2"
}
EOF
python3 - "$return_fixtures/implementer-positive.json" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/2/return.json" <<'PY'
import json
import sys
from pathlib import Path

value = json.loads(Path(sys.argv[1]).read_text())
value.update({"jobId": "fixture-job-r2", "round": 2, "sessionId": "session-2"})
Path(sys.argv[2]).write_text(json.dumps(value, indent=2) + "\n")
PY
(cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job-r2)

python3 - "$return_fixtures/implementer-positive.json" "$return_fixtures" <<'PY'
import copy
import json
import sys
from pathlib import Path

source = json.loads(Path(sys.argv[1]).read_text())
out = Path(sys.argv[2])
for field, value in {
    "jobId": "other-job",
    "round": 2,
    "runtime": "other-runtime",
    "sessionId": "other-session",
}.items():
    changed = copy.deepcopy(source)
    changed[field] = value
    (out / f"identity-{field}.json").write_text(json.dumps(changed, indent=2) + "\n")
PY
for field in jobId round runtime sessionId; do
  cp "$return_fixtures/identity-$field.json" \
    "$job_fixture/artifacts/agents/fixture-job/rounds/1/return.json"
  set +e
  (cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job) >"$tmp/identity-$field.out" 2>&1
  identity_status=$?
  set -e
  if [[ $identity_status -eq 0 ]]; then
    echo "job-aware return checker accepted a mismatched $field" >&2
    exit 1
  fi
  [[ $identity_status -eq 1 ]] \
    || { echo "job-aware return checker used $identity_status instead of exit 1 for $field" >&2; exit 1; }
  grep -Fq "$.${field} identity mismatch" "$tmp/identity-$field.out" \
    || { echo "job-aware return checker did not name the $field mismatch" >&2; exit 1; }
done

# Dispatcher and fake-adapter fixtures run in a minimal adopted-mode Git
# repository. Keeping their artifacts outside this checkout proves that the
# scripts derive every path from the repository they ship into. Nested
# adoption validations inherit the skip after this block, avoiding duplicate
# process-lifecycle runs while a direct adopted-repository validation still
# exercises the full contract.
if delegate_process_section "dispatcher, adapter selftest, and mission-runner process fixtures" \
  && [[ -z "${METASYSTEM_SKIP_AGENT_FIXTURES:-}" ]]; then
  agent_fixture="$tmp/agent-fixture"
  agent_repo="$agent_fixture/repo"
  agent_evidence="$agent_fixture/evidence"
  mkdir -p "$agent_repo/scripts" "$agent_repo/docs"
  agent_repo=$(cd "$agent_repo" && pwd -P)
  cp -R scripts/agents "$agent_repo/scripts/"
  cp scripts/metasystem-config.sh scripts/assert-mission.sh scripts/assert-stop-loss.sh \
    scripts/assert-return-complete.sh scripts/assert-turn-prompt.sh \
    scripts/watch-background-jobs.sh "$agent_repo/scripts/"
  cp docs/project-rules.md "$agent_repo/docs/"
  cp metasystem.conf "$agent_repo/"
  perl -0pi -e 's/^metasystem\.runtimes=.*$/metasystem.runtimes=fake/m; s|^evidence\.root=.*$|evidence.root='"$agent_evidence"'|m; s/^watch\.interval-sec=.*$/watch.interval-sec=5/m; s/^census\.log-max-bytes=.*$/census.log-max-bytes=4096/m; s/^role\.default\.runtime=.*$/role.default.runtime=fake/m; s/^role\.default\.model\.codex=.*$/role.default.model.fake=fake-model/m; s/^role\.default\.model\.(?:claude|devin)=.*\n//mg; s/^role\.code-critic\.runtime=.*$/role.code-critic.runtime=fake/m; s/^role\.code-critic\.model\.<runtime>=.*$/role.code-critic.model.fake=fake-model/m; s/^role\.investigator\.runtime=main$/role.investigator.runtime=fake/m; s/\.runtime=(?:codex|devin)$/\.runtime=fake/mg; s/\.model\.(?:codex|devin)=.*$/\.model.fake=fake-model/mg' "$agent_repo/metasystem.conf"
  printf '\nmodel.tier.1=fake:fake-model\nmodel.tier.2=fake:fake-premium\n' >>"$agent_repo/metasystem.conf"
  grep -q '^watch\.interval-sec=' "$agent_repo/metasystem.conf" || printf 'watch.interval-sec=5\n' >>"$agent_repo/metasystem.conf"
  grep -q '^census\.log-max-bytes=' "$agent_repo/metasystem.conf" || printf 'census.log-max-bytes=4096\n' >>"$agent_repo/metasystem.conf"
  git -C "$agent_repo" init -q -b main
  git -C "$agent_repo" add .
  git -C "$agent_repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm base
  # Production resolves its engine as <repo>/bin/metasystem — an untracked
  # build artifact that adoption ships. Stage the real engine the same way,
  # after the base commit so it stays untracked exactly like production.
  # The runner and selftest repositories below inherit it via cp -R.
  mkdir -p "$agent_repo/bin"
  cp bin/metasystem "$agent_repo/bin/metasystem"
  agent_dispatch="$agent_repo/scripts/agents/dispatch.sh"
  fake_adapter="$agent_repo/scripts/agents/adapters/fake.sh"
  agent_config="$agent_repo/scripts/metasystem-config.sh"
  good_agent_conf="$agent_fixture/good-metasystem.conf"
  cp "$agent_repo/metasystem.conf" "$good_agent_conf"

  # The mission runner and compound adapter selftest each get a pristine
  # repository and supervisor. They run only after the main dispatch fixture
  # set has shut down, so neither can queue behind its fixture state or reuse
  # its synthetic-process supervision set.
  runner_repo="$agent_fixture/runner-repo"
  runner_evidence="$agent_fixture/runner-evidence"
  cp -R "$agent_repo" "$runner_repo"
  runner_repo=$(cd "$runner_repo" && pwd -P)
  # Fixture git writes race the previous mission's trailing anchor: runners
  # are detached, so "the mission returned" does not mean "its last git op
  # finished". Wait for a live lock, remove a dead one's leavings (a killed
  # runner leaves index.lock forever), then run the git op.
  runner_git_cap_sec=$(harness_fixture_cap runner-git-lock)
  runner_git_stale_sec=$(( $(harness_fixture_base_cap runner-git-lock) / 2 ))
  runner_git() {
    local deadline=$((SECONDS + runner_git_cap_sec))
    while [[ -e "$runner_repo/.git/index.lock" ]] && (( SECONDS < deadline )); do
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    if [[ -e "$runner_repo/.git/index.lock" ]]; then
      python3 - "$runner_repo/.git/index.lock" "$runner_git_stale_sec" <<'LOCK'
import os, sys, time
lock = sys.argv[1]
try:
    if time.time() - os.stat(lock).st_mtime >= int(sys.argv[2]):
        os.unlink(lock)
except OSError:
    pass
LOCK
    fi
    git -C "$runner_repo" "$@"
  }

  perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$runner_evidence"'|m' \
    "$runner_repo/metasystem.conf"
  agent_selftest_repo="$agent_fixture/selftest-repo"
  agent_selftest_evidence="$agent_fixture/selftest-evidence"
  cp -R "$agent_repo" "$agent_selftest_repo"
  agent_selftest_repo=$(cd "$agent_selftest_repo" && pwd -P)
  perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$agent_selftest_evidence"'|m' \
    "$agent_selftest_repo/metasystem.conf"

  agent_fixture_cap_sec=$(harness_fixture_cap agent-command)
  agent_status_cap_sec=$(harness_fixture_cap agent-status)
  agent_cleanup_cap_sec=$(harness_fixture_cap agent-cleanup)
  agent_driver_stop_cap_sec=$(harness_fixture_cap agent-driver-stop)
  METASYSTEM_FIXTURE_AGENT_STATUS_CAP_SEC=$agent_status_cap_sec
  export METASYSTEM_FIXTURE_AGENT_STATUS_CAP_SEC

  wait_for_agent_census_fresh() { # fixture name
    local name=$1 started=$SECONDS deadline=$((SECONDS + agent_fixture_cap_sec)) expected elapsed
    [[ -n "${agent_supervision_repo:-}" ]] || return 0
    while (( SECONDS < deadline )); do
      expected=$("$agent_supervision_repo/scripts/agents/arm-supervision.sh" \
        fingerprint --repo "$agent_supervision_repo" 2>/dev/null || true)
      if [[ -n "$expected" ]] && python3 - \
          "$agent_supervision_repo/artifacts/agents/supervision/last-census.json" \
          "$agent_supervision_repo/artifacts/agents/supervision/state.json" \
          "$expected" <<'PY'
import json,sys,time
try: value=json.load(open(sys.argv[1])); state=json.load(open(sys.argv[2]))
except (OSError,ValueError): raise SystemExit(1)
age=int(time.time())-int(value.get("completedAtEpoch",0)); interval=int(value.get("intervalSec",0) or 0)
raise SystemExit(0 if value.get("verdict")=="SUCCESS" and value.get("fingerprint")==sys.argv[3]
                 and value.get("generation")==state.get("generation")
                 and 0 <= age <= max(1,interval//2) else 1)
PY
      then return 0; fi
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    elapsed=$((SECONDS - started))
    echo "agent fixture timed out waiting for a fresh census: $name (elapsed: ${elapsed}s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
    return 1
  }

  agent_fixture_job_from_args() {
    local previous= argument
    for argument in "$@"; do
      if [[ "$previous" == --job || "$previous" == --job-id ]]; then
        printf '%s\n' "$argument"
        return
      fi
      previous=$argument
    done
    printf '%s\n' -
  }

  agent_fixture_diagnostics() { # fixture name, job id or -, elapsed seconds
    local name=$1 job=$2 elapsed=$3 path
    echo "agent fixture timed out: $name (job: $job; elapsed: ${elapsed}s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
    [[ "$job" != - ]] || return
    for path in \
      "$agent_repo/artifacts/agents/jobs/$job.json" \
      "$agent_repo/artifacts/agents/jobs/$job.log" \
      "$agent_repo/artifacts/agents/hb/$job.start" \
      "$agent_repo/artifacts/agents/hb/$job" \
      "$agent_repo/artifacts/agents/hb/$job.waiting"; do
      if [[ -e "$path" ]]; then
        echo "--- $path" >&2
        sed -n '1,240p' "$path" >&2
      else
        echo "--- missing: $path" >&2
      fi
    done
  }

  stop_timed_out_agent_fixture() { # fixture name, job id or -, driver pid, wait start
    local name=$1 job=$2 driver_pid=$3 wait_started=$4 cleanup_pid cleanup_started cleanup_deadline driver_started driver_deadline elapsed status=
    elapsed=$((SECONDS - wait_started))
    agent_fixture_diagnostics "$name" "$job" "$elapsed"
    if [[ "$job" != - && -f "$agent_repo/artifacts/agents/jobs/$job.json" ]]; then
      status=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("status", "malformed"))' \
        "$agent_repo/artifacts/agents/jobs/$job.json" 2>/dev/null || true)
      if [[ "$status" == pending || "$status" == running ]]; then
        "$agent_dispatch" cancel --job "$job" >"$agent_fixture/$name-timeout-cancel.out" 2>&1 &
        cleanup_pid=$!
        cleanup_started=$SECONDS
        cleanup_deadline=$(( SECONDS + agent_cleanup_cap_sec ))
        while kill -0 "$cleanup_pid" 2>/dev/null && (( SECONDS < cleanup_deadline )); do sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"; done
        if kill -0 "$cleanup_pid" 2>/dev/null; then
          elapsed=$((SECONDS - cleanup_started))
          echo "agent fixture cleanup timed out: $name cancel pid $cleanup_pid (elapsed: ${elapsed}s; scaled cap: ${agent_cleanup_cap_sec}s)" >&2
          kill -TERM "$cleanup_pid" 2>/dev/null || true
          sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
          kill -KILL "$cleanup_pid" 2>/dev/null || true
        fi
      fi
    fi
    if kill -0 "$driver_pid" 2>/dev/null; then
      kill -TERM "$driver_pid" 2>/dev/null || true
      driver_started=$SECONDS
      driver_deadline=$(( SECONDS + agent_driver_stop_cap_sec ))
      while kill -0 "$driver_pid" 2>/dev/null && (( SECONDS < driver_deadline )); do sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"; done
      if kill -0 "$driver_pid" 2>/dev/null; then
        elapsed=$((SECONDS - driver_started))
        echo "agent fixture driver stop timed out: $name pid $driver_pid (elapsed: ${elapsed}s; scaled cap: ${agent_driver_stop_cap_sec}s); sending KILL" >&2
        kill -KILL "$driver_pid" 2>/dev/null || true
      fi
    fi
    exit 1
  }

  wait_for_agent_fixture_process() { # fixture name, job id or -, exact child pid
    local name=$1 job=$2 child_pid=$3 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec )) result
    while kill -0 "$child_pid" 2>/dev/null; do
      (( SECONDS < deadline )) || stop_timed_out_agent_fixture "$name" "$job" "$child_pid" "$started"
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    if wait "$child_pid"; then result=0; else result=$?; fi
    return "$result"
  }

  run_agent_fixture() { # fixture name, job id or -, command...
    local name=$1 job=$2 child_pid
    shift 2
    case ${2:-} in dispatch|follow-up) wait_for_agent_census_fresh "$name" ;; esac
    "$@" &
    child_pid=$!
    wait_for_agent_fixture_process "$name" "$job" "$child_pid"
  }

  run_agent_fixture_captured() { # fixture name, job id or -, output file, command...
    local name=$1 job=$2 output=$3 child_pid captured_result captured_attempt
    shift 3
    # The supervisor heals itself asynchronously; a dispatch landing between
    # an arming publication and its confirming census is refused with a
    # transient "retry in a moment". Every driver shares this runner, so the
    # retry lives here once — pass, fail, and TTY paths behave identically.
    for captured_attempt in 1 2 3; do
      case ${2:-} in dispatch|follow-up) wait_for_agent_census_fresh "$name" ;; esac
      "$@" >"$output" 2>&1 &
      child_pid=$!
      # No errexit toggling here: set -e is global, not function-scoped, and
      # re-enabling it before returning nonzero detonates in a caller that
      # disabled it around this very call. The if-form never trips errexit.
      if wait_for_agent_fixture_process "$name" "$job" "$child_pid"; then
        return 0
      else
        captured_result=$?
      fi
      grep -Fq 'censusGeneration=' "$output" 2>/dev/null || return "$captured_result"
      # Retry is only safe while the refusal preceded job creation. Once a
      # record exists the dispatch got past the census check, the failure is
      # the real answer, and a re-dispatch would overwrite the record it is
      # asserting about (one-shot fake markers made that visible).
      if [[ "$job" != - && -e "$agent_repo/artifacts/agents/jobs/$job.json" ]]; then
        return "$captured_result"
      fi
      sleep 1
    done
    return "$captured_result"
  }

  agent_fails() { # output name, expected text, command...
    local name=$1 expected=$2 result job
    shift 2
    job=$(agent_fixture_job_from_args "$@")
    set +e
    run_agent_fixture_captured "$name" "$job" "$agent_fixture/$name.out" "$@"
    result=$?
    set -e
    if [[ $result -eq 0 ]]; then
      echo "agent fixture unexpectedly passed: $name" >&2
      exit 1
    fi
    [[ -z "$expected" ]] || grep -Fq "$expected" "$agent_fixture/$name.out" || {
      echo "agent fixture $name did not report: $expected" >&2
      cat "$agent_fixture/$name.out" >&2
      # When the wrong refusal is the generation transient, the supervision
      # state explains WHY the retries could not outrun it; dump it so one
      # failing run carries its own diagnosis.
      if [[ -n "${agent_supervision_repo:-}" ]]; then
        echo "--- supervision state at failure:" >&2
        cat "$agent_supervision_repo/artifacts/agents/supervision/state.json" >&2 2>/dev/null || true
        echo "--- last census:" >&2
        cat "$agent_supervision_repo/artifacts/agents/supervision/last-census.json" >&2 2>/dev/null || true
        echo "--- supervisor log tail:" >&2
        tail -15 "$agent_supervision_repo/artifacts/agents/supervision/supervisor.log" >&2 2>/dev/null || true
      fi
      exit 1
    }
  }

  run_tty_agent_fixture() { # fixture name, typed line, expected exit, command...
    local name=$1 typed=$2 expected_exit=$3 attempt tty_result
    shift 3
    # The supervisor heals itself asynchronously, so a dispatch can land in
    # the moment between an arming publication and the census that confirms
    # it; that refusal is transient and says "retry in a moment", which is
    # what agent_fails already does and this driver must do too.
    for attempt in 1 2 3; do
      wait_for_agent_census_fresh "$name"
      set +e
      run_tty_agent_fixture_once "$name" "$typed" "$expected_exit" "$@"
      tty_result=$?
      set -e
      [[ $tty_result -eq 0 ]] && return 0
      grep -Fq 'censusGeneration=' "$agent_fixture/$name.out" 2>/dev/null || break
      sleep 1
    done
    return "$tty_result"
  }

  run_tty_agent_fixture_once() { # fixture name, typed line, expected exit, command...
    local name=$1 typed=$2 expected_exit=$3
    shift 3
    python3 - "$agent_fixture/$name.out" "$typed" "$expected_exit" "$agent_fixture_cap_sec" \
      "$agent_driver_stop_cap_sec" "$@" <<'PY'
import errno
import os
import pty
import select
import subprocess
import sys
import time
from pathlib import Path

output, typed, expected_exit, cap, stop_cap, *command = sys.argv[1:]
master, slave = pty.openpty()
process = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave, close_fds=True)
os.close(slave)
os.write(master, (typed + "\n").encode())
chunks = []
deadline = time.monotonic() + int(cap)
while process.poll() is None:
    if time.monotonic() >= deadline:
        process.terminate()
        try:
            process.wait(timeout=int(stop_cap))
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        raise SystemExit(f"TTY fixture timed out: {' '.join(command)}")
    ready, _, _ = select.select([master], [], [], 0.05)
    if ready:
        try:
            chunks.append(os.read(master, 65536))
        except OSError as error:
            if error.errno != errno.EIO:
                raise
while True:
    try:
        chunk = os.read(master, 65536)
    except OSError as error:
        if error.errno == errno.EIO:
            break
        raise
    if not chunk:
        break
    chunks.append(chunk)
os.close(master)
Path(output).write_bytes(b"".join(chunks))
if process.returncode != int(expected_exit):
    # A failure that hides its transcript cannot be diagnosed after teardown.
    sys.stderr.write(b"".join(chunks).decode(errors="replace")[-2000:] + "\n")
    raise SystemExit(f"TTY fixture exit {process.returncode}, expected {expected_exit}")
PY
  }

  make_agent_brief() { # output, mode, optional marker/value lines...
    local output=$1 mode=$2
    shift 2
    sed "s/^Working Mode:.*/Working Mode: $mode/" "$agent_repo/scripts/agents/templates/brief.md" >"$output"
    for line in "$@"; do printf '\n%s\n' "$line" >>"$output"; done
  }

  wait_for_agent_status() { # job, expected
    local job=$1 expected=$2 observed= started=$SECONDS deadline=$((SECONDS + agent_status_cap_sec)) elapsed
    while (( SECONDS < deadline )); do
      observed=$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job "$job" 2>/dev/null || true)
      [[ "$observed" == "$expected" ]] && return 0
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    elapsed=$((SECONDS - started))
    echo "agent fixture status timed out: $job -> $expected (last status: ${observed:-missing}; elapsed: ${elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2
    return 1
  }

  # Configuration resolution is flag, environment, mode, plain, default.
  config_order="$agent_fixture/config-order"
  mkdir -p "$config_order/scripts" "$config_order/bin"
  cp scripts/metasystem-config.sh "$config_order/scripts/"
  cp bin/metasystem "$config_order/bin/metasystem"
  cat >"$config_order/metasystem.conf" <<EOF
role.implementer.runtime=plain
mode.refactor.role.implementer.runtime=mode
plain.knob=plain-value
EOF
  [[ "$("$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor --flag flag)" == flag ]] \
    || { echo "metasystem config did not prefer the flag" >&2; exit 1; }
  [[ "$(METASYSTEM_ROLE_IMPLEMENTER_RUNTIME=environment "$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor)" == environment ]] \
    || { echo "metasystem config did not prefer the environment" >&2; exit 1; }
  [[ "$(env -u METASYSTEM_ROLE_IMPLEMENTER_RUNTIME "$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor)" == mode ]] \
    || { echo "metasystem config did not resolve the mode scope" >&2; exit 1; }
  [[ "$("$config_order/scripts/metasystem-config.sh" get --key plain.knob --mode refactor)" == plain-value ]] \
    || { echo "metasystem config did not resolve the plain key" >&2; exit 1; }
  [[ "$("$config_order/scripts/metasystem-config.sh" get --key absent.knob --default built-in)" == built-in ]] \
    || { echo "metasystem config did not resolve the built-in default" >&2; exit 1; }
  # An uncommitted local file carries values that must not ship to adopting
  # projects. It outranks the committed conf and yields to the environment.
  cat >"$config_order/metasystem.conf.local" <<'EOF'
plain.knob=local-value
EOF
  [[ "$(env -u METASYSTEM_PLAIN_KNOB "$config_order/scripts/metasystem-config.sh" get --key plain.knob)" == local-value ]] \
    || { echo "metasystem config did not prefer the local override" >&2; exit 1; }
  [[ "$(METASYSTEM_PLAIN_KNOB=environment "$config_order/scripts/metasystem-config.sh" get --key plain.knob)" == environment ]] \
    || { echo "local override outranked the environment" >&2; exit 1; }
  [[ "$(env -u METASYSTEM_ROLE_IMPLEMENTER_RUNTIME "$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor)" == mode ]] \
    || { echo "local override disturbed a key it does not carry" >&2; exit 1; }
  rm -f "$config_order/metasystem.conf.local"

  "$agent_config" validate
  no_tier_conf="$agent_fixture/no-tier-metasystem.conf"
  python3 - "$good_agent_conf" "$no_tier_conf" <<'PY'
import sys
from pathlib import Path
source, output = map(Path, sys.argv[1:])
output.write_text("\n".join(line for line in source.read_text().splitlines() if not line.startswith("model.tier.")) + "\n")
PY
  cp "$no_tier_conf" "$agent_repo/metasystem.conf"
  "$agent_config" validate >"$agent_fixture/no-tier-validate.out"
  [[ $(grep -Fc 'INFO: model tiers are absent; dispatch overrides therefore always escalate' "$agent_fixture/no-tier-validate.out") -eq 1 ]] \
    || { echo "tier-absence validation fixture did not emit its one informational line" >&2; exit 1; }
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^model\.tier\.1=.*$/model.tier.one=fake:fake-model/m' "$agent_repo/metasystem.conf"
  agent_fails malformed-tier-key 'not a supported model tier key' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^model\.tier\.1=.*$/model.tier.1=fake-model/m' "$agent_repo/metasystem.conf"
  agent_fails malformed-tier-member 'not runtime-qualified' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^role\.design-critic\.runtime=.*$/role.design-critic.runtime=ghost/m' "$agent_repo/metasystem.conf"
  agent_fails invalid-role-runtime 'outside metasystem.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  printf 'mode.refactor.role.implementer.runtime=ghost\n' >>"$agent_repo/metasystem.conf"
  agent_fails invalid-mode-runtime 'outside metasystem.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  printf 'role.default.model.ghost=ghost-model\n' >>"$agent_repo/metasystem.conf"
  agent_fails invalid-model-runtime 'outside metasystem.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^metasystem\.runtimes=.*$/metasystem.runtimes=ghost/m' "$agent_repo/metasystem.conf"
  agent_fails unsupported-runtime 'unsupported runtime' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^model\.tier\.1=.*$/model.tier.1=/m' "$agent_repo/metasystem.conf"
  agent_fails unmapped-model 'appears in 0 model tiers' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^model\.tier\.2=.*$/model.tier.2=fake:fake-model/m' "$agent_repo/metasystem.conf"
  agent_fails duplicate-model-tier 'appears in 2 model tiers' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^role\.design-critic\.model\.fake=.*\n//m; s/^role\.default\.model\.fake=.*\n//m' "$agent_repo/metasystem.conf"
  agent_fails missing-runtime-model 'has no model.fake value' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$agent_repo"'/evidence|m' "$agent_repo/metasystem.conf"
  agent_fails inside-evidence-root 'outside the repository' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"

  # All remaining dispatch fixtures run behind a real armed fake-runtime set.
  # The explicit synthetic process table is fixture-only and keeps this test
  # deterministic in restricted environments where ps enumeration is denied.
  agent_process_fixture="$agent_fixture/processes.json"
  agent_identity_fixture="$agent_fixture/process-identities.json"
  printf '[]\n' >"$agent_process_fixture"
  printf '{}\n' >"$agent_identity_fixture"
  export METASYSTEM_CENSUS_PROCESS_FILE="$agent_process_fixture"
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$agent_identity_fixture"
  agent_supervision_repo=$agent_repo
  track_armed_supervision "$agent_repo"
  agent_main_start=$("$agent_repo/bin/metasystem" proc started-at --pid "$$")
  METASYSTEM_AGENT_RUNTIME=fake "$agent_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_repo" --session validator --pid "$$" \
    --start-time "$agent_main_start" --tag metasystem-main-fake-validator \
    >"$agent_fixture/arming.out"

  first_snapshot=$($fake_adapter probe)
  second_snapshot=$($fake_adapter probe)
  [[ "$first_snapshot" == *-001.json && "$second_snapshot" == *-002.json && "$first_snapshot" != "$second_snapshot" ]] \
    || { echo "fake probe did not create immutable sequence-suffixed snapshots" >&2; exit 1; }

  happy_brief="$agent_fixture/happy.md"
  make_agent_brief "$happy_brief" design
  run_agent_fixture happy happy "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id happy --wait
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job happy)" == completed ]] \
    || { echo "valid fake dispatch did not complete" >&2; exit 1; }
  python3 - "$agent_repo" <<'PY'
import json, re, sys
from pathlib import Path
root = Path(sys.argv[1])
record = json.loads((root / "artifacts/agents/jobs/happy.json").read_text())
required = {
    "jobId", "role", "mission", "runtime", "round", "parentJob", "status", "phase", "error",
    "workspaceRoot", "baseSha", "branch", "permissions", "capMin", "pid", "pidStartedAt", "pgid", "instanceTag", "custodyProcesses",
    "sessionId", "turnId", "requestedModel", "effectiveModel", "overridden", "capabilitySnapshot",
    "sessionEstablishedTimeoutSec", "input", "startedAt", "endedAt", "usage", "mirror",
}
assert required.issubset(record) and record["status"] == "completed"
assert record["capabilitySnapshot"].endswith("-002.json")
snapshot = json.loads((root / record["capabilitySnapshot"]).read_text())
assert record["sessionEstablishedTimeoutSec"] == snapshot["capabilities"]["sessionEstablishedTimeoutSec"]
assert record["permissions"]["requested"]["preset"] == "none"
assert record["permissions"]["enforcementSnapshot"] == record["capabilitySnapshot"]
snapshot = json.loads((root / record["permissions"]["enforcementSnapshot"]).read_text())
assert snapshot["envelopeEnforcement"] == {
    "writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped",
}
assert record["input"]["delivery"] == "stdin" and record["input"]["bytes"] > 0
prompt = (root / "artifacts/agents/happy/rounds/1/prompt.md").read_text()
assert prompt.startswith("Job-Id: happy\nRole: design-critic\nRuntime: fake\nModel: fake-model\nRound: 1\n")
preamble = (root / "scripts/agents/roles/design-critic.md").read_text().rstrip("\n")
brief = (root / "artifacts/agents/happy/brief.md").read_text().rstrip("\n")
assert prompt.index(preamble) < prompt.index(brief)
PY
  mkdir -p "$agent_repo/artifacts/agents/locks/stale-lock.d"
  printf '{"pid":999999,"instanceTag":"dead-owner","acquiredAt":"2000-01-01T00:00:00Z"}\n' >"$agent_repo/artifacts/agents/locks/stale-lock.d/owner.json"
  run_agent_fixture stale-lock stale-lock "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id stale-lock --wait

  generated=$(run_agent_fixture generated-dispatch - "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief")
  [[ "$generated" =~ ^design-critic-[0-9]{8}t[0-9]{6}z-[a-f0-9]{4}$ ]] \
    || { echo "generated job id does not match the lowercase grammar: $generated" >&2; exit 1; }
  agent_fails collision 'job id collision' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id happy
  agent_fails malformed-job-id 'invalid job id' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id 'Bad_Id'
  agent_fails contradictory-mode "contradicts the brief's Working Mode" "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --mode verify
  agent_fails unregistered-override 'outside metasystem.runtimes' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --runtime ghost
  agent_fails main-override 'assigned to main' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --runtime main
  agent_fails costlier-unmapped 'cost direction is unranked' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model absent-from-tier
  agent_fails ranked-costlier 'higher (tier 1 -> tier 2)' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-premium

  # The recorded default is a real fallback, while its absence refuses.
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^role\.verifier\.runtime=.*\n//m' "$agent_repo/metasystem.conf"
  verifier_brief="$agent_fixture/verifier.md"
  make_agent_brief "$verifier_brief" verify
  run_agent_fixture default-role default-role "$agent_dispatch" dispatch --role verifier --brief "$verifier_brief" --permissions none --job-id default-role --wait
  cp "$agent_repo/metasystem.conf" "$agent_fixture/no-role.conf"
  perl -0pi -e 's/^role\.default\.runtime=.*\n//m' "$agent_repo/metasystem.conf"
  agent_fails no-role-default 'neither a runtime entry nor role.default.runtime' "$agent_dispatch" dispatch --role verifier --brief "$verifier_brief" --permissions none
  cp "$good_agent_conf" "$agent_repo/metasystem.conf"
  code_brief="$agent_fixture/code.md"
  make_agent_brief "$code_brief" implement
  review_target_brief="$agent_fixture/review-target.md"
  make_agent_brief "$review_target_brief" implement
  run_agent_fixture review-target review-target "$agent_dispatch" dispatch --role implementer --brief "$review_target_brief" --job-id review-target --worktree --wait
  run_agent_fixture flag-runtime flag-runtime "$agent_dispatch" dispatch --role code-critic --brief "$code_brief" --reviews review-target --runtime fake --permissions none --job-id flag-runtime --wait
  python3 - "$agent_repo/artifacts/agents/jobs/flag-runtime.json" <<'PY'
import json, sys
record = json.load(open(sys.argv[1])); assert record["runtime"] == "fake" and record["overridden"] is True
assert record["reviews"] == "review-target", record
PY
  investigator_brief="$agent_fixture/investigator.md"
  make_agent_brief "$investigator_brief" take-a-step-back
  run_agent_fixture investigator-role investigator-role "$agent_dispatch" dispatch --role investigator --brief "$investigator_brief" --runtime fake --permissions none --job-id investigator-role --wait

  no_signal="$agent_fixture/no-signal.md"
  make_agent_brief "$no_signal" design 'FAKE:no-session-signal'
  agent_fails no-session-signal '' "$agent_dispatch" dispatch --role design-critic --brief "$no_signal" --job-id no-signal --wait
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$agent_repo/artifacts/agents/jobs/no-signal.json")" == failed ]] \
    || { echo "non-signal handshake did not end failed" >&2; exit 1; }
  grep -Fq 'handshake_timeout' "$agent_repo/artifacts/agents/jobs/no-signal.json" \
    || { echo "non-signal handshake did not retain its error" >&2
         cat "$agent_repo/artifacts/agents/jobs/no-signal.json" >&2
         echo "--- dispatch said:" >&2
         cat "$agent_fixture/no-session-signal.out" >&2 2>/dev/null || true
         echo "--- job log:" >&2
         sed -n '1,60p' "$agent_repo/artifacts/agents/jobs/no-signal.log" >&2 2>/dev/null || true
         exit 1; }
  handshake_failure="$agent_fixture/handshake-failure.md"
  make_agent_brief "$handshake_failure" design 'FAKE:handshake-failure'
  agent_fails handshake-failure '' "$agent_dispatch" dispatch --role design-critic --brief "$handshake_failure" --job-id handshake-failure --wait
  grep -Fq 'authentication_failed' "$agent_repo/artifacts/agents/jobs/handshake-failure.json" \
    || { echo "fake handshake failure lost its named error" >&2; exit 1; }
  missing_session="$agent_fixture/missing-session.md"
  make_agent_brief "$missing_session" design 'FAKE:missing-session-id'
  agent_fails missing-session '' "$agent_dispatch" dispatch --role design-critic --brief "$missing_session" --job-id missing-session --wait
  grep -Fq 'handshake_missing_session_id' "$agent_repo/artifacts/agents/jobs/missing-session.json" \
    || { echo "missing session id did not fail the strong handshake" >&2; exit 1; }

  pending_brief="$agent_fixture/pending.md"
  make_agent_brief "$pending_brief" design 'FAKE:no-session-signal'
  wait_for_agent_census_fresh pending-chain
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_brief" --job-id pending-chain >/dev/null 2>&1) & pending_driver=$!
  wait_for_agent_status pending-chain pending
  pending_message="$agent_fixture/pending-follow.md"
  cp "$agent_repo/scripts/agents/templates/follow-up.md" "$pending_message"
  agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job pending-chain --message "$pending_message"
  wait_for_agent_fixture_process pending-chain-driver pending-chain "$pending_driver" || true

  # A just-created pending record with no launched supervisor is inside its
  # recorded handshake budget, so a sweep leaves it pending. Once the same
  # record is older than its budget, the existing process-loss classification
  # applies unchanged.
  launch_window_source="$agent_fixture/launch-window.json"
  launch_window_pending="$agent_fixture/launch-window-pending.json"
  # A record is created in pending-setup and only then completed into pending,
  # so the fixture takes both steps the dispatcher takes. Writing a pending
  # record straight into creation tested a transition the dispatcher no longer
  # performs.
  python3 - "$agent_repo/artifacts/agents/jobs/happy.json" "$launch_window_source" "$launch_window_pending" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text())
record.update({
    "jobId": "launch-window", "parentJob": None, "round": 1,
    "status": "pending-setup", "phase": "handshake", "error": None,
    "pid": None, "pidStartedAt": None, "pgid": None,
    "instanceTag": "metasystem-job-launch-window", "custodyProcesses": [],
    "sessionId": None, "endedAt": None, "usage": None, "mirror": None,
    "sessionEstablishedTimeoutSec": 60,
    "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
})
# This record has not launched, so it carries no launch-time stamps: the
# handshake deadline among them, which is stamped when a dispatcher starts
# waiting on an adapter it just started.
for key in ("ownershipProof", "chainUsage", "handshakeDeadline"):
    record.pop(key, None)
Path(sys.argv[2]).write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
Path(sys.argv[3]).write_text(json.dumps({**record, "status": "pending"}, indent=2, sort_keys=True) + "\n")
PY
  run_agent_fixture launch-window-create launch-window "$agent_dispatch" __record-create --job launch-window --source "$launch_window_source"
  run_agent_fixture launch-window-setup launch-window "$agent_dispatch" __record-setup --job launch-window --source "$launch_window_pending"
  run_agent_fixture launch-window-young-reap launch-window "$agent_dispatch" reap --job launch-window
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$agent_repo/artifacts/agents/jobs/launch-window.json")" == pending ]] \
    || { echo "pending record was reaped inside its handshake window" >&2; exit 1; }
  python3 - "$agent_repo/artifacts/agents/jobs/launch-window.json" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["startedAt"] = "2000-01-01T00:00:00Z"
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  run_agent_fixture launch-window-old-reap launch-window "$agent_dispatch" reap --job launch-window
  python3 - "$agent_repo/artifacts/agents/jobs/launch-window.json" <<'PY'
import json, sys
record = json.load(open(sys.argv[1]))
assert record["status"] == "failed" and record["error"] == "process-lost" and record["phase"] == "supervision", record
PY

  pending_loss_brief="$agent_fixture/pending-loss.md"
  make_agent_brief "$pending_loss_brief" design 'FAKE:pending-process-loss'
  wait_for_agent_census_fresh pending-loss
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_loss_brief" --job-id pending-loss >/dev/null 2>&1) & pending_loss_driver=$!
  wait_for_agent_status pending-loss pending
  # The supervisor is dying while this runs, so a single sweep can legitimately
  # land before the kill and observe a live match. The standing reaper sweeps on
  # an interval, so sweeping until the transition or a scaled ceiling is the
  # faithful check; the assertion itself is unchanged.
  pending_loss_started=$SECONDS
  pending_loss_deadline=$((SECONDS + agent_status_cap_sec))
  until grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/pending-loss.json"; do
    if (( SECONDS >= pending_loss_deadline )); then
      echo "dead pending supervisor did not transition through reap (elapsed: $((SECONDS - pending_loss_started))s; scaled cap: ${agent_status_cap_sec}s)" >&2
      exit 1
    fi
    run_agent_fixture pending-loss-reap pending-loss "$agent_dispatch" reap --job pending-loss
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  wait_for_agent_fixture_process pending-loss-driver pending-loss "$pending_loss_driver" || true

  malformed_brief="$agent_fixture/malformed-return.md"
  make_agent_brief "$malformed_brief" design 'FAKE:malformed-return'
  set +e
  run_agent_fixture malformed-return malformed-return "$agent_dispatch" dispatch --role design-critic --brief "$malformed_brief" --job-id malformed-return --wait
  malformed_status=$?
  set -e
  [[ $malformed_status -eq 3 ]] || { echo "malformed return mapped to $malformed_status instead of 3" >&2; exit 1; }
  grep -Fq 'protocol_error' "$agent_repo/artifacts/agents/jobs/malformed-return.json" \
    || { echo "malformed return did not record protocol_error" >&2; exit 1; }

  interrupted="$agent_fixture/interrupted.md"
  make_agent_brief "$interrupted" design 'FAKE:interrupted-atomic-write'
  run_agent_fixture interrupted interrupted "$agent_dispatch" dispatch --role design-critic --brief "$interrupted" --job-id interrupted --wait
  [[ -f "$agent_repo/artifacts/agents/record-locks/interrupted.interrupted" ]] \
    || { echo "interrupted atomic-write fixture did not leave its staged partial" >&2; exit 1; }
  python3 -m json.tool "$agent_repo/artifacts/agents/jobs/interrupted.json" >/dev/null
  terminal_patch="$agent_fixture/terminal-race.json"
  printf '{"error":"loser"}\n' >"$terminal_patch"
  set +e
  run_agent_fixture_captured terminal-race interrupted /dev/null "$agent_dispatch" __record-cas --job interrupted --expect running --status failed --patch "$terminal_patch"
  terminal_race_status=$?
  set -e
  [[ $terminal_race_status -eq 3 && "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job interrupted)" == completed ]] \
    || { echo "terminal compare-and-set did not preserve the first writer" >&2; exit 1; }
  agent_fails illegal-terminal-transition 'illegal job transition' "$agent_dispatch" __record-cas --job interrupted --expect completed --status failed --patch "$terminal_patch"

  # The fake reports network access it was not granted. Now that the presets
  # grant it by default, the request has to withhold it explicitly for the
  # report to be wider than the request at all.
  restrictive_permissions="$agent_fixture/restrictive-permissions.json"
  printf '{"readRoots":["."],"writeRoots":[],"network":"deny","approvals":"deny","tools":"read-only"}\n' >"$restrictive_permissions"
  effective_wider="$agent_fixture/effective-wider.md"
  make_agent_brief "$effective_wider" design 'FAKE:effective-wider'
  agent_fails effective-wider '' "$agent_dispatch" dispatch --role design-critic --brief "$effective_wider" --permissions "$restrictive_permissions" --job-id effective-wider --wait
  grep -Fq 'permissions_mismatch:network' "$agent_repo/artifacts/agents/jobs/effective-wider.json" \
    || { echo "wider effective envelope did not record the mismatch" >&2; exit 1; }
  permissive_permissions="$agent_fixture/permissive-permissions.json"
  printf '{"readRoots":["."],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}\n' >"$permissive_permissions"
  effective_narrower="$agent_fixture/effective-narrower.md"
  make_agent_brief "$effective_narrower" design 'FAKE:effective-narrower'
  run_agent_fixture effective-narrower effective-narrower "$agent_dispatch" dispatch --role design-critic --brief "$effective_narrower" --permissions "$permissive_permissions" --job-id effective-narrower --wait

  # The shipped presets grant network, and a repository may narrow it for every
  # role at once. Until 2026-08-05 the adapters hard-coded network off and never
  # read the field, so a job could be recorded as networked and still be cut off
  # (KI-12); these fixtures exist so that cannot recur silently.
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["network"])' scripts/agents/permissions/workspace.json)" == allow ]] \
    || { echo "the workspace preset no longer grants network" >&2; exit 1; }
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["network"])' scripts/agents/permissions/none.json)" == allow ]] \
    || { echo "the none preset no longer grants network" >&2; exit 1; }
  net_default="$agent_fixture/net-default.md"
  make_agent_brief "$net_default" design
  run_agent_fixture net-default net-default "$agent_dispatch" dispatch --role design-critic --brief "$net_default" --job-id net-default --wait
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["permissions"]["requested"]["network"])' "$agent_repo/artifacts/agents/jobs/net-default.json")" == allow ]] \
    || { echo "a delegate did not receive network by default" >&2; exit 1; }
  printf 'dispatch.permissions.network=deny\n' >>"$agent_repo/metasystem.conf"
  net_floor="$agent_fixture/net-floor.md"
  make_agent_brief "$net_floor" design
  run_agent_fixture net-floor net-floor "$agent_dispatch" dispatch --role design-critic --brief "$net_floor" --job-id net-floor --wait
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["permissions"]["requested"]["network"])' "$agent_repo/artifacts/agents/jobs/net-floor.json")" == deny ]] \
    || { echo "the repository network floor did not narrow the preset" >&2; exit 1; }
  perl -0pi -e 's/^dispatch\.permissions\.network=deny\n//m' "$agent_repo/metasystem.conf"
  agent_fails invalid-network-floor 'must be deny or allow' \
    env METASYSTEM_DISPATCH_PERMISSIONS_NETWORK=sometimes "$agent_dispatch" dispatch --role design-critic --brief "$net_default" --job-id bad-floor

  agent_fails writable-without-worktree 'writable permissions require --worktree' \
    "$agent_dispatch" dispatch --role implementer --brief "$code_brief" --job-id no-worktree
  custom_permissions="$agent_fixture/custom-permissions.json"
  cat >"$custom_permissions" <<EOF
{"readRoots":["."],"writeRoots":["$agent_fixture/outside"],"network":"deny","approvals":"deny","tools":"runtime-default"}
EOF
  agent_fails escaping-write-root 'escapes the job worktree' "$agent_dispatch" dispatch --role implementer --brief "$code_brief" --job-id escaping-root --worktree --permissions "$custom_permissions"

  oversized="$agent_fixture/oversized.md"
  make_agent_brief "$oversized" design 'FAKE:oversized-input'
  head -c 70000 /dev/zero | tr '\0' x >>"$oversized"
  agent_fails oversized-input 'pass a file reference' "$agent_dispatch" dispatch --role design-critic --brief "$oversized" --job-id oversized

  # Reap owns process loss, absolute caps, group death, and terminal mirroring.
  process_loss="$agent_fixture/process-loss.md"
  make_agent_brief "$process_loss" design 'FAKE:process-loss'
  set +e
  run_agent_fixture process-loss process-loss "$agent_dispatch" dispatch --role design-critic --brief "$process_loss" --job-id process-loss --wait
  process_loss_status=$?
  set -e
  [[ $process_loss_status -eq 3 ]] || { echo "process loss mapped to $process_loss_status instead of 3" >&2; exit 1; }
  grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/process-loss.json" \
    || { echo "reap did not name process-lost" >&2; exit 1; }
  [[ -f "$agent_repo/artifacts/agents/process-loss/rounds/1/child.stopped" ]] \
    || { echo "reap did not TERM the orphaned process-loss child" >&2; exit 1; }
  grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/process-loss.json" \
    || { echo "process-loss terminal record lacks group-death proof" >&2; exit 1; }

  timeout_brief="$agent_fixture/timeout.md"
  make_agent_brief "$timeout_brief" design 'FAKE:timeout'
  timeout_result="$agent_fixture/timeout.status"
  wait_for_agent_census_fresh timed
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id timed --cap-min "$fixture_minimum_cap_min" --wait
    printf '%s\n' "$?" >"$timeout_result"
  ) &
  timeout_driver=$!
  wait_for_agent_status timed running
  python3 - "$agent_repo/artifacts/agents/jobs/timed.json" <<'PY'
import json, sys
from pathlib import Path
# The engine judges the budget by capDeadline first (startedAt+capMin is
# only the fallback), so the fixture must backdate BOTH: backdating only
# startedAt left the real one-minute deadline live and the explicit reap
# inert, a coin-flip against the fixture's own wait ceiling.
path = Path(sys.argv[1]); value = json.loads(path.read_text())
value["startedAt"] = "2000-01-01T00:00:00Z"
value["capDeadline"] = "2000-01-01T00:01:00Z"
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  run_agent_fixture timed-reap timed "$agent_dispatch" reap --job timed
  wait_for_agent_fixture_process timed-driver timed "$timeout_driver"
  [[ "$(cat "$timeout_result")" == 4 ]] || {
    echo "timeout did not map to wait exit 4 (got $(cat "$timeout_result"))" >&2
    cat "$agent_repo/artifacts/agents/jobs/timed.json" >&2
    echo "--- reaper log:" >&2
    tail -20 "$agent_repo/artifacts/agents/supervision/reaper.log" >&2 2>/dev/null || true
    exit 1; }
  grep -Fq 'budget-cap' "$agent_repo/artifacts/agents/jobs/timed.json" \
    || { echo "absolute cap did not record budget-cap" >&2; exit 1; }
  [[ -f "$agent_repo/artifacts/agents/timed/rounds/1/child.stopped" ]] \
    || { echo "timeout did not TERM the whole owned group" >&2; exit 1; }
  grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/timed.json" \
    || { echo "timeout terminal record lacks group-death proof" >&2; exit 1; }

  cancel_result="$agent_fixture/cancel.status"
  wait_for_agent_census_fresh cancelled
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id cancelled --wait
    printf '%s\n' "$?" >"$cancel_result"
  ) &
  cancel_driver=$!
  wait_for_agent_status cancelled running
  run_agent_fixture cancelled-cancel cancelled "$agent_dispatch" cancel --job cancelled
  wait_for_agent_fixture_process cancelled-driver cancelled "$cancel_driver"
  [[ "$(cat "$cancel_result")" == 8 ]] || { echo "cancelled did not map to wait exit 8" >&2; exit 1; }
  [[ -f "$agent_repo/artifacts/agents/cancelled/rounds/1/child.stopped" ]] \
    || { echo "cancel did not TERM the whole owned group" >&2; exit 1; }
  grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/cancelled.json" \
    || { echo "cancelled terminal record lacks group-death proof" >&2; exit 1; }

  vanished_result="$agent_fixture/vanished.status"
  wait_for_agent_census_fresh vanished
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id vanished --wait
    printf '%s\n' "$?" >"$vanished_result"
  ) &
  vanished_driver=$!
  vanished_wait_started=$SECONDS
  vanished_wait_deadline=$((SECONDS + agent_status_cap_sec))
  while (( SECONDS < vanished_wait_deadline )); do
    [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] && break
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] \
    || { vanished_wait_elapsed=$((SECONDS - vanished_wait_started)); echo "agent fixture file wait timed out: vanished.waiting (job: vanished; elapsed: ${vanished_wait_elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2; exit 1; }
  mv "$agent_repo/artifacts/agents/jobs/vanished.json" "$agent_fixture/vanished.json"
  wait_for_agent_fixture_process vanished-driver vanished "$vanished_driver"
  mv "$agent_fixture/vanished.json" "$agent_repo/artifacts/agents/jobs/vanished.json"
  [[ "$(cat "$vanished_result")" == 5 ]] || { echo "vanished record did not map to wait exit 5 (got $(cat "$vanished_result"))" >&2; exit 1; }
  run_agent_fixture vanished-cancel vanished "$agent_dispatch" cancel --job vanished

  cancel_race="$agent_fixture/cancel-race.md"
  make_agent_brief "$cancel_race" design 'FAKE:cancel-race'
  run_agent_fixture_captured cancel-race-dispatch cancel-race /dev/null "$agent_dispatch" dispatch --role design-critic --brief "$cancel_race" --job-id cancel-race
  run_agent_fixture cancel-race-cancel cancel-race "$agent_dispatch" cancel --job cancel-race
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job cancel-race)" == completed ]] \
    || { echo "completion did not win the scripted cancellation race" >&2; exit 1; }

  mirror_failure="$agent_fixture/mirror-failure.md"
  make_agent_brief "$mirror_failure" design 'FAKE:mirror-failure'
  run_agent_fixture mirror-retry mirror-retry "$agent_dispatch" dispatch --role design-critic --brief "$mirror_failure" --job-id mirror-retry --wait
  [[ -f "$agent_repo/artifacts/agents/mirror-retry/.mirror-failed" ]] \
    || { echo "scripted first mirror failure did not occur" >&2; exit 1; }
  if [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")" == None ]]; then
    run_agent_fixture mirror-retry-first-reap mirror-retry "$agent_dispatch" reap --job mirror-retry
  fi
  mirror_hash_before=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"]["manifest"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")
  run_agent_fixture mirror-retry-second-reap mirror-retry "$agent_dispatch" reap --job mirror-retry
  mirror_hash_after=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"]["manifest"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")
  [[ "$mirror_hash_before" == "$mirror_hash_after" && "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job mirror-retry)" == completed ]] \
    || { echo "idempotent mirror retry changed terminal state or durable content" >&2; exit 1; }
  python3 - "$agent_repo/artifacts/agents/jobs/mirror-retry.json" <<'PY'
import hashlib, json, sys
from pathlib import Path
record = json.load(open(sys.argv[1])); mirror = record["mirror"]; assert mirror
manifest_path = Path(mirror["path"]) / "manifest.json"
assert hashlib.sha256(manifest_path.read_bytes()).hexdigest() == mirror["manifest"]
manifest = json.loads(manifest_path.read_text())
for relative, item in manifest["files"].items():
    assert hashlib.sha256((Path(mirror["path"]) / relative).read_bytes()).hexdigest() == item["sha256"]
PY

  # Follow-ups are child records under one serialized, explicitly closed chain.
  follow_message="$agent_fixture/follow.md"
  cp "$agent_repo/scripts/agents/templates/follow-up.md" "$follow_message"
  # A follow-up on a worktree chain whose trunk moved warns loudly: the
  # stale-worktree lesson was violated three times as prose before this line.
  stale_brief="$agent_fixture/stale-wt.md"
  make_agent_brief "$stale_brief" implement
  run_agent_fixture stale-wt stale-wt "$agent_dispatch" dispatch --role implementer --brief "$stale_brief" --job-id stale-wt --worktree --wait
  printf 'advance\n' >>"$agent_repo/trunk-advance.txt"
  git -C "$agent_repo" add trunk-advance.txt
  git -C "$agent_repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm trunk-advance
  "$agent_dispatch" follow-up --job stale-wt --message "$follow_message" --wait >"$tmp/stale-wt.out" 2>"$tmp/stale-wt.err" \
    || { echo "stale-worktree follow-up itself failed" >&2; cat "$tmp/stale-wt.err" >&2; exit 1; }
  grep -q 'WORKTREE-BEHIND' "$tmp/stale-wt.err" \
    || { echo "follow-up did not warn about a worktree behind its trunk" >&2; exit 1; }

  run_agent_fixture happy-follow-up happy-r2 "$agent_dispatch" follow-up --job happy --message "$follow_message" --wait
  [[ -d "$agent_repo/artifacts/agents/happy/rounds/1" && -d "$agent_repo/artifacts/agents/happy/rounds/2" ]] \
    || { echo "follow-up did not preserve round 1 and create round 2" >&2; exit 1; }
  python3 - "$agent_repo" <<'PY'
import json, sys
from pathlib import Path
root = Path(sys.argv[1]); parent = json.loads((root / "artifacts/agents/jobs/happy.json").read_text()); child = json.loads((root / "artifacts/agents/jobs/happy-r2.json").read_text())
assert child["parentJob"] == "happy" and child["round"] == 2 and child["sessionId"] == parent["sessionId"]
assert child["startedAt"] >= parent["startedAt"] and child["capMin"] == parent["capMin"]
snapshot = json.loads((root / child["capabilitySnapshot"]).read_text())
assert child["sessionEstablishedTimeoutSec"] == snapshot["capabilities"]["sessionEstablishedTimeoutSec"]
assert parent["chainUsage"]["providerUnits"]["fake"]["fake-unit"] == 2
assert parent["mirror"]["manifest"] == child["mirror"]["manifest"]
PY
  run_agent_fixture malformed-return-follow-up malformed-return-r2 "$agent_dispatch" follow-up --job malformed-return --message "$follow_message" --wait
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job malformed-return-r2)" == completed ]] \
    || { echo "protocol-error retry did not create a completed child" >&2; exit 1; }
  agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job cancelled --message "$follow_message"
  agent_fails timeout-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job timed --message "$follow_message"
  agent_fails process-loss-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job process-loss --message "$follow_message"

  python3 - "$agent_repo/artifacts/agents/jobs/default-role.json" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["sessionId"] = None; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  agent_fails null-session-follow-up 'fresh-context embed fallback' "$agent_dispatch" follow-up --job default-role --message "$follow_message"

  resume_root="$agent_fixture/resume-root.md"
  make_agent_brief "$resume_root" design
  run_agent_fixture resume-root resume-root "$agent_dispatch" dispatch --role design-critic --brief "$resume_root" --job-id resume-root --wait
  resume_collision="$agent_fixture/resume-collision.md"
  cp "$follow_message" "$resume_collision"
  printf '\nFAKE:resume-collision\n' >>"$resume_collision"
  set +e
  run_agent_fixture resume-collision resume-root-r2 "$agent_dispatch" follow-up --job resume-root --message "$resume_collision" --wait
  resume_status=$?
  set -e
  [[ $resume_status -eq 3 ]] || { echo "resume collision did not map to failed" >&2; exit 1; }
  grep -Fq 'resume_collision' "$agent_repo/artifacts/agents/jobs/resume-root-r2.json" \
    || { echo "resume collision did not retain its named error" >&2; exit 1; }

  active_brief="$agent_fixture/active.md"
  make_agent_brief "$active_brief" design 'FAKE:concurrent-turn'
  run_agent_fixture_captured active-turn-dispatch active-turn /dev/null "$agent_dispatch" dispatch --role design-critic --brief "$active_brief" --job-id active-turn
  agent_fails active-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job active-turn --message "$follow_message"
  run_agent_fixture active-turn-cancel active-turn "$agent_dispatch" cancel --job active-turn

  run_agent_fixture happy-close happy "$agent_dispatch" close --job happy
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["runnerClosed"])' "$agent_repo/artifacts/agents/jobs/happy.json")" == False ]] \
    || { echo "host-closed chain was stamped as runner-closed" >&2; exit 1; }
  agent_fails closed-follow-up 'job chain is closed' "$agent_dispatch" follow-up --job happy --message "$follow_message"

  # A close racing a follow-up cannot land between its open check and child creation.
  run_agent_fixture close-race close-race "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id close-race --wait
  close_rc="$agent_fixture/close-race.close"; follow_rc="$agent_fixture/close-race.follow"
  wait_for_agent_census_fresh close-race-follow
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh close --job close-race >/dev/null 2>&1; printf '%s\n' "$?" >"$close_rc") & close_pid=$!
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh follow-up --job close-race --message "$follow_message" >/dev/null 2>&1; printf '%s\n' "$?" >"$follow_rc") & follow_pid=$!
  wait_for_agent_fixture_process close-race-close close-race "$close_pid"
  wait_for_agent_fixture_process close-race-follow close-race-r2 "$follow_pid"
  close_won=$(cat "$close_rc"); follow_won=$(cat "$follow_rc")
  [[ ( "$close_won" == 0 && "$follow_won" != 0 ) || ( "$close_won" != 0 && "$follow_won" == 0 ) ]] \
    || { echo "close/follow-up race did not serialize to one winner" >&2; exit 1; }

  # Conformance uses the actual intent-to-add working-tree diff and protects
  # both plans/ and the ignored agent control plane.
  implement_brief="$agent_fixture/implement.md"
  make_agent_brief "$implement_brief" implement
  run_agent_fixture conformance conformance "$agent_dispatch" dispatch --role implementer --brief "$implement_brief" --job-id conformance --worktree --wait
  agent_fails close-before-diff 'diff.patch is not mirrored' "$agent_dispatch" close --job conformance
  (cd "$agent_repo" && scripts/agents/assert-conformance.sh --stage review --job conformance)
  [[ -f "$agent_repo/artifacts/agents/conformance/rounds/1/diff.patch" ]] \
    || { echo "conformance did not persist diff.patch" >&2; exit 1; }
  run_agent_fixture conformance-reap conformance "$agent_dispatch" reap --job conformance
  run_agent_fixture conformance-close conformance "$agent_dispatch" close --job conformance
  conformance_workspace=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["workspaceRoot"])' "$agent_repo/artifacts/agents/jobs/conformance.json")
  case "${conformance_workspace%/}/" in "${agent_repo%/}/"*) ;; *) echo "job worktree is outside the watcher scope" >&2; exit 1 ;; esac
  printf 'untracked change\n' >"$conformance_workspace/source.txt"
  agent_fails diff-boundary-mismatch 'changed paths fall outside the cumulative implementation boundary' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
  python3 - "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" source.txt <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["diffBoundary"] = sys.argv[2:]; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  (cd "$agent_repo" && scripts/agents/assert-conformance.sh --stage review --job conformance)
  mkdir -p "$conformance_workspace/plans"
  printf 'delegate plan\n' >"$conformance_workspace/plans/delegate.md"
  python3 - "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" source.txt plans/delegate.md <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["diffBoundary"] = sys.argv[2:]; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  agent_fails untracked-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
  git -C "$conformance_workspace" add source.txt plans/delegate.md
  git -C "$conformance_workspace" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm delegate-checkpoint
  agent_fails committed-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
  printf 'uncommitted change\n' >>"$conformance_workspace/plans/delegate.md"
  agent_fails uncommitted-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
  mkdir -p "$conformance_workspace/artifacts/agents"
  printf 'tamper\n' >"$conformance_workspace/artifacts/agents/tamper"
  agent_fails control-plane-change 'agent control plane contains delegate-created files' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance

  # Snapshot refusal, fallbacks, permission waivers, and raw/event degradations.
  snapshot_dir="$agent_repo/artifacts/agents/capabilities"
  snapshot_save="$agent_fixture/snapshots"
  mkdir -p "$snapshot_save"
  mv "$snapshot_dir"/*.json "$snapshot_save/"
  agent_fails no-snapshot 'no capability snapshot matches' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id no-snapshot
  "$fake_adapter" probe --age-days 31 >/dev/null
  agent_fails stale-snapshot 'capability snapshot is stale' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id stale-snapshot
  mv "$snapshot_dir"/*.json "$agent_fixture/"
  mv "$snapshot_save"/*.json "$snapshot_dir/"

  old_save="$agent_fixture/current-snapshots"
  mkdir -p "$old_save"
  mv "$snapshot_dir"/*.json "$old_save/"
  "$fake_adapter" probe --profile old >/dev/null
  old_brief="$agent_fixture/old-capabilities.md"
  make_agent_brief "$old_brief" implement 'FAKE:old-capability-set' 'FAKE:no-event-stream' 'FAKE:hook-unavailable'
  run_agent_fixture old-capabilities old-capabilities "$agent_dispatch" dispatch --role implementer --brief "$old_brief" --job-id old-capabilities --worktree --wait
  python3 - "$agent_repo/artifacts/agents/jobs/old-capabilities.json" <<'PY'
import json, sys
record = json.load(open(sys.argv[1])); assert record["sessionEstablishedSignal"] is False
assert any(item["capability"] == "resume" for item in record["capabilityFallbacks"])
PY
  [[ ! -e "$agent_repo/artifacts/agents/old-capabilities/rounds/1/events.jsonl" ]] \
    || { echo "no-event-stream fallback emitted a native event file" >&2; exit 1; }
  grep -Fq 'polling fallback used' "$agent_repo/artifacts/agents/jobs/old-capabilities.log" \
    || { echo "hook-unavailable fallback was not observable" >&2; exit 1; }
  run_agent_fixture old-capabilities-follow-up old-capabilities-r2 "$agent_dispatch" follow-up --job old-capabilities --message "$follow_message" --wait
  python3 - "$agent_repo" <<'PY'
import json, sys
from pathlib import Path
root = Path(sys.argv[1]); parent = json.loads((root / "artifacts/agents/jobs/old-capabilities.json").read_text()); child = json.loads((root / "artifacts/agents/jobs/old-capabilities-r2.json").read_text())
assert child["resumeMode"] == "fresh-context" and child["sessionId"] != parent["sessionId"]
prompt = (root / "artifacts/agents/old-capabilities/rounds/2/prompt.md").read_text()
assert "# Prior brief" in prompt and "# Prior return" in prompt and "# Correction" in prompt
PY
  mv "$snapshot_dir"/*.json "$agent_fixture/"
  mv "$old_save"/*.json "$snapshot_dir/"

  requirements="$agent_repo/scripts/agents/roles/design-critic.requirements.json"
  saved_requirements="$agent_fixture/design-critic.requirements.json"
  cp "$requirements" "$saved_requirements"
  "$fake_adapter" probe --profile unverified-network >/dev/null
  # This refusal is about a runtime that cannot verify a field the envelope
  # restricts, so the envelope has to restrict it. Since the presets now grant
  # network, the request is made restrictive explicitly rather than by default.
  agent_fails unverified-deny 'cannot enforce restrictive permission field network' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --permissions "$restrictive_permissions" --job-id unverified-deny
  # Merge the network waiver into whatever waivers the role already declares
  # (the shipped roles now waive readRoots/writeRoots for devin), rather than
  # string-injecting a second waivers key and producing invalid JSON.
  python3 - "$requirements" <<'PY'
import json, sys
from pathlib import Path
p = Path(sys.argv[1]); v = json.loads(p.read_text())
v.setdefault("waivers", {}).setdefault("network", []).append("fake")
p.write_text(json.dumps(v, indent=2) + "\n")
PY
  run_agent_fixture waived-deny waived-deny "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --permissions "$restrictive_permissions" --job-id waived-deny --wait
  cp "$saved_requirements" "$requirements"
  "$fake_adapter" probe >/dev/null

  # An EMPTY write scope is still restrictive on a runtime whose write boundary
  # is notEnforced (it can write through a shell), so such a role is refused
  # without a recorded writeRoots waiver and runs with one. Exercises the real
  # selector against a laid-down snapshot -- no live CLI, no dispatch.
  ew_caps="$agent_repo/artifacts/agents/capabilities"
  mkdir -p "$ew_caps"
  ew_snap="$ew_caps/ghostrt-1.0-20990101-001.json"
  printf '%s\n' '{"runtime":"ghostrt","cliVersion":"1.0","capturedAt":"2099-01-01T00:00:00Z","configHash":"cfg1","configKeyHashes":{},"sequence":1,"transports":[],"capabilities":{"sessionEstablishedTimeoutSec":10},"permissions":{"unverified":["readRoots","writeRoots","network"]},"envelopeEnforcement":{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}}' >"$ew_snap"
  ew_identity='{"runtime":"ghostrt","cliVersion":"1.0","configHash":"cfg1","configKeyHashes":{}}'
  ew_env="$agent_fixture/empty-write-envelope.json"
  printf '%s\n' '{"readRoots":[],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}' >"$ew_env"
  ew_role="$agent_repo/scripts/agents/roles/design-critic.requirements.json"
  ew_role_saved="$agent_fixture/ew-role-saved.json"
  cp "$ew_role" "$ew_role_saved"
  printf '%s\n' '{"required":[],"optional":{},"waivers":{}}' >"$ew_role"
  if bin/metasystem job snapshot-select --root "$agent_repo" --runtime ghostrt \
      --role design-critic --identity "$ew_identity" --max-age 40000 --envelope "$ew_env" \
      --output "$agent_fixture/ew-unwaived.out" 2>"$agent_fixture/ew-unwaived.err"; then
    cp "$ew_role_saved" "$ew_role"
    echo "empty writeRoots on a notEnforced runtime ran without a waiver (the bypass is open)" >&2; exit 1
  fi
  grep -Fq 'writeRoots' "$agent_fixture/ew-unwaived.err" \
    || { cp "$ew_role_saved" "$ew_role"; echo "empty-writeRoots refusal did not name the field" >&2; cat "$agent_fixture/ew-unwaived.err" >&2; exit 1; }
  printf '%s\n' '{"required":[],"optional":{},"waivers":{"writeRoots":["ghostrt"]}}' >"$ew_role"
  bin/metasystem job snapshot-select --root "$agent_repo" --runtime ghostrt \
    --role design-critic --identity "$ew_identity" --max-age 40000 --envelope "$ew_env" \
    --output "$agent_fixture/ew-waived.out" \
    || { cp "$ew_role_saved" "$ew_role"; echo "empty writeRoots was refused even with the writeRoots waiver on record" >&2; exit 1; }
  cp "$ew_role_saved" "$ew_role"
  rm -f "$ew_snap"

  nested_brief="$agent_fixture/nested.md"
  make_agent_brief "$nested_brief" design 'FAKE:nested-agent-events'
  run_agent_fixture nested-events nested-events "$agent_dispatch" dispatch --role design-critic --brief "$nested_brief" --job-id nested-events --wait
  grep -Fq '"topLevel":false' "$agent_repo/artifacts/agents/nested-events/rounds/1/events.jsonl" \
    || { echo "nested-agent event was not captured" >&2; exit 1; }
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job nested-events)" == completed ]] \
    || { echo "nested completion event ended the wrong lifecycle" >&2; exit 1; }
  malicious_brief="$agent_fixture/malicious.md"
  make_agent_brief "$malicious_brief" design 'Fake-Argument: $(touch should-not-exist)'
  run_agent_fixture malicious-argument malicious-argument "$agent_dispatch" dispatch --role design-critic --brief "$malicious_brief" --job-id malicious-argument --wait
  [[ ! -e "$agent_repo/should-not-exist" ]] || { echo "malicious provider argument was evaluated" >&2; exit 1; }
  grep -Fq '$(touch should-not-exist)' "$agent_repo/artifacts/agents/malicious-argument/rounds/1/raw.out" \
    || { echo "malicious provider argument was not transported verbatim as a value" >&2; exit 1; }

  # The no-tier guard fixtures use a fresh supervisor fingerprint after
  # changing the roster. The runtime-override roles return to their shipped
  # main assignment; fake remains the only registered fixture adapter.
  "$agent_repo/scripts/agents/arm-supervision.sh" --repo "$agent_repo" --shutdown >/dev/null
  cp "$no_tier_conf" "$agent_repo/metasystem.conf"
  perl -0pi -e 's/^role\.(code-critic|investigator)\.runtime=fake$/role.$1.runtime=main/mg' "$agent_repo/metasystem.conf"
  # The template now ships code-critic entries (C-2), so the conf snapshot
  # already carries role.code-critic.model.fake from the roster rewrite;
  # appending it again was a duplicate-key failure.
  cat >>"$agent_repo/metasystem.conf" <<'EOF'
role.investigator.model.fake=fake-implied-model
EOF
  METASYSTEM_AGENT_RUNTIME=fake "$agent_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_repo" --session validator-no-tiers --pid "$$" \
    --start-time "$agent_main_start" --tag metasystem-main-fake-validator \
    >"$agent_fixture/no-tier-arming.out"

  agent_fails no-tier-model-override 'Configure model.tier.* to rank both pairs' \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --job-id no-tier-model
  agent_fails escalation-non-tty 'requires an interactive TTY' \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --approve-escalation --job-id escalation-non-tty
  run_tty_agent_fixture escalation-declined NO 1 \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --approve-escalation --job-id escalation-declined
  grep -Fq 'escalation approval declined' "$agent_fixture/escalation-declined.out" \
    || { echo "declined escalation fixture did not name the corrective action" >&2; exit 1; }
  [[ ! -e "$agent_repo/artifacts/agents/jobs/escalation-declined.json" ]] \
    || { echo "declined escalation fixture created a job" >&2; exit 1; }
  run_tty_agent_fixture escalation-approved 'APPROVE Fixture Human' 0 \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --approve-escalation --job-id escalation-approved --wait
  python3 - "$agent_repo/artifacts/agents/jobs/escalation-approved.json" "$agent_fixture/escalation-approved.out" <<'PY'
import json
import re
import sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text())
display = Path(sys.argv[2]).read_text()
approval = record["escalationApproval"]
assert approval["name"] == "Fixture Human"
assert re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", approval["approvedAt"])
assert approval["rosterResolution"] == "fake:fake-model"
assert approval["requestedPair"] == "fake:fake-escalated"
assert approval["costDirection"] == "unranked (model tiers absent; overrides always escalate)"
for label, key in (("Roster resolution", "rosterResolution"), ("Requested pair", "requestedPair"), ("Cost direction", "costDirection")):
    assert f"{label}: {approval[key]}" in display
PY

  # Dispatch only reads mission leases. The future mission runner owns their
  # acquisition and renewal; this fixture fabricates the frozen shape around
  # a process whose command line carries the instance tag.
  mkdir -p "$agent_repo/plans" "$agent_repo/artifacts/agents/missions/mission-alpha"
  cat >"$agent_repo/plans/mission-mission-alpha.contract.md" <<'EOF'
```mission
fence.wall-clock-hours=2
fence.cycles=10
fence.jobs=20
fence.concurrency=2
fence.job-cap-min=FIXTURE_DISPATCH_CAP_MIN
envelope.dispatch-allow=fake:fake-escalated,fake:fake-model
```

```mission-seal
sealed.version=1
```
EOF
  perl -0pi -e 's/FIXTURE_DISPATCH_CAP_MIN/'"$fixture_dispatch_envelope_cap_min"'/g' \
    "$agent_repo/plans/mission-mission-alpha.contract.md"
  # The digest a human approval signs: Approval lines removed, every line
  # stripped of trailing spaces/tabs, trailing blank lines dropped. This is
  # contractCanonicalSignedBytes (internal/mission/contract.go), verified
  # byte-identical to the retired python contract_hash before its deletion.
  fixture_contract_hash() { # contract path
    awk '!/^Approval:/{sub(/[ \t]+$/,""); line[++n]=$0}
      END{while(n>0 && line[n]=="") n--; for(i=1;i<=n;i++) printf "%s%s", line[i], (i<n ? "\n" : "")}' \
      "$1" | shasum -a 256 | awk '{print $1}'
  }
  printf '\nApproval: name=Fixture-Human; date=2026-08-06; contract-sha256=%s\n' \
    "$(fixture_contract_hash "$agent_repo/plans/mission-mission-alpha.contract.md")" \
    >>"$agent_repo/plans/mission-mission-alpha.contract.md"
  stamp_fixture_contract() { # mission — seed the runner-owned contract pin
    # (Codex's caps ruling: fixtures below the runner lifecycle seed
    # approvedContractSha256 as the digest of the exact raw contract bytes.)
    python3 - "$agent_repo/plans/mission-$1.contract.md" "$agent_repo/artifacts/agents/missions/$1/fences.json" "$1" <<'PY_STAMP'
import hashlib, json, sys
from datetime import datetime, timezone
from pathlib import Path
contract, fences_path, mission = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]
if fences_path.exists():
    value = json.loads(fences_path.read_text())
else:
    value = {"schemaVersion": 1, "missionId": mission,
             "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
             "cycles": 0, "reservations": {}}
value["approvedContractSha256"] = hashlib.sha256(contract.read_bytes()).hexdigest()
fences_path.parent.mkdir(parents=True, exist_ok=True)
fences_path.write_text(json.dumps(value) + "\n")
PY_STAMP
  }
  stamp_fixture_contract mission-alpha
  dispatch_origin="$agent_fixture/dispatch-origin.git"
  git init -q -b main --bare "$dispatch_origin"
  git -C "$agent_repo" remote add origin "$dispatch_origin"
  git -C "$agent_repo" add metasystem.conf plans/mission-mission-alpha.contract.md
  git -C "$agent_repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm 'sign dispatch envelope fixture'
  git -C "$agent_repo" push -qu origin main
  git -C "$dispatch_origin" symbolic-ref HEAD refs/heads/main
  git -C "$agent_repo" remote set-head origin -a >/dev/null
  # The lease holder has no private lifetime: its bounded teardown below is the
  # only event that ends it, so section growth cannot turn its lifetime into a cap.
  "$agent_repo/bin/metasystem" util hold --tag mission-lease-tag & mission_pid=$!
  mission_pgid=$(ps -p "$mission_pid" -o pgid= | tr -d ' ')
  mission_identity="$agent_fixture/mission-process-identity.json"
  python3 - "$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" "$mission_identity" "$mission_pid" "$mission_pgid" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
pid, pgid = int(sys.argv[3]), int(sys.argv[4])
Path(sys.argv[1]).write_text(json.dumps({"missionId":"mission-alpha","pid":pid,"pgid":pgid,"instanceTag":"mission-lease-tag","startedAt":now,"renewedAt":now}) + "\n")
Path(sys.argv[2]).write_text(json.dumps({str(pid): {"pgid": pgid, "command": "metasystem util hold --tag mission-lease-tag"}}) + "\n")
PY
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$mission_identity"
  run_agent_fixture envelope-model-override envelope-model-override "$agent_dispatch" dispatch \
    --role design-critic --brief "$happy_brief" --model fake-escalated --job-id envelope-model-override --mission mission-alpha --wait
  run_agent_fixture envelope-runtime-override envelope-runtime-override "$agent_dispatch" dispatch \
    --role code-critic --brief "$code_brief" --reviews review-target --runtime fake --job-id envelope-runtime-override --mission mission-alpha --wait
  agent_fails envelope-runtime-implied-model 'add fake:fake-implied-model to a signed envelope.dispatch-allow' \
    "$agent_dispatch" dispatch --role investigator --brief "$investigator_brief" --runtime fake --job-id envelope-runtime-implied --mission mission-alpha
  run_agent_fixture mission-explicit mission-explicit "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-explicit --mission mission-alpha --wait
  METASYSTEM_MISSION_ID=mission-alpha METASYSTEM_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
    METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture \
    run_agent_fixture mission-inherited mission-inherited "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-inherited --wait
  # The caps implementation refuses over-cap mission dispatches with the
  # sharper pair-cap message (names both numbers) instead of the generic
  # lifecycle-fence wrapper; the expectation follows the sharper contract.
  agent_fails mission-cap 'above signed fence.job-cap-min' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-cap --mission mission-alpha --cap-min "$fixture_dispatch_over_envelope_cap_min"
  # Under the caps contract an over-cap request is an AUTHORIZATION
  # refusal — a synchronous host error, deliberately without a
  # fence-bound ask (Codex's delegate-caps fixtures assert exactly
  # this); asks remain the currency of genuine fence violations,
  # proven by the fence-* fixtures below and the timeout-ask above.
  [[ ! -f "$agent_repo/artifacts/agents/missions/mission-alpha/asks/fence-bound.json" ]] \
    || { echo "an authorization refusal wrote a fence-bound ask; that is the fence-violation channel" >&2; exit 1; }
  python3 - "$agent_repo" <<'PY'
import json, sys
from pathlib import Path
root=Path(sys.argv[1]); usage=json.loads((root/"artifacts/agents/missions/mission-alpha/usage.json").read_text())
units={(item["provider"],item["unit"]):item["value"] for item in usage["units"]}
assert units[("fake","provider.fake-unit")] == 4
for job in ("mission-explicit","mission-inherited"):
    prompt=(root/f"artifacts/agents/{job}/rounds/1/prompt.md").read_text()
    assert "\nMission: mission-alpha\n" in prompt
inherited=json.loads((root/"artifacts/agents/jobs/mission-inherited.json").read_text())
assert inherited["turnId"] == "mission-alpha-t1-fixture"
PY

  make_fence_mission() { # mission id, cycles, jobs, concurrency, wall hours
    local mission=$1 cycles=$2 jobs_limit=$3 concurrency=$4 wall=$5 mission_dir="$agent_repo/artifacts/agents/missions/$1"
    mkdir -p "$mission_dir" "$agent_repo/plans"
    cat >"$agent_repo/plans/mission-$mission.contract.md" <<EOF
\`\`\`mission
fence.wall-clock-hours=$wall
fence.cycles=$cycles
fence.jobs=$jobs_limit
fence.concurrency=$concurrency
fence.job-cap-min=$fixture_dispatch_envelope_cap_min
\`\`\`
EOF
    python3 - "$mission_dir/lease.json" "$mission" "$mission_pid" "$mission_pgid" <<'PY'
import json,sys
from datetime import datetime,timezone
from pathlib import Path
now=datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
Path(sys.argv[1]).write_text(json.dumps({"missionId":sys.argv[2],"pid":int(sys.argv[3]),"pgid":int(sys.argv[4]),"instanceTag":"mission-lease-tag","startedAt":now,"renewedAt":now})+"\n")
PY
  }

  assert_fence_ask() { # mission, expected reason
    local mission=$1 reason=$2 ask="$agent_repo/artifacts/agents/missions/$1/asks/fence-bound.json"
    [[ -f "$ask" ]] || { echo "mission fence $reason refusal wrote no batched ask" >&2; exit 1; }
    grep -Fq "\`$reason\`" "$ask" || { echo "mission fence ask omitted $reason" >&2; exit 1; }
  }

  make_fence_mission mission-wall 10 10 2 1
  printf '{"schemaVersion":1,"missionId":"mission-wall","startedAt":"2000-01-01T00:00:00Z","cycles":0,"reservations":{}}\n' \
    >"$agent_repo/artifacts/agents/missions/mission-wall/fences.json"
  stamp_fixture_contract mission-wall
  agent_fails fence-wall 'mission fence refused job (wall-clock-hours)' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-wall --mission mission-wall --wait
  assert_fence_ask mission-wall wall-clock-hours

  make_fence_mission mission-cycles 1 10 2 2
  printf '{"schemaVersion":1,"missionId":"mission-cycles","startedAt":"%s","cycles":1,"reservations":{}}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$agent_repo/artifacts/agents/missions/mission-cycles/fences.json"
  stamp_fixture_contract mission-cycles
  agent_fails fence-cycles 'mission fence refused job (cycles)' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-cycles --mission mission-cycles --wait
  assert_fence_ask mission-cycles cycles

  make_fence_mission mission-jobs 10 1 2 2
  printf '{"schemaVersion":1,"missionId":"mission-jobs","startedAt":"%s","cycles":0,"reservations":{"prior":{"reservedAt":"2000-01-01T00:00:00Z","capMin":%s}}}\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$fixture_minimum_cap_min" \
    >"$agent_repo/artifacts/agents/missions/mission-jobs/fences.json"
  stamp_fixture_contract mission-jobs
  agent_fails fence-jobs 'mission fence refused job (jobs)' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-jobs --mission mission-jobs --wait
  assert_fence_ask mission-jobs jobs

  make_fence_mission mission-concurrency 10 10 1 2
  printf '{"schemaVersion":1,"missionId":"mission-concurrency","startedAt":"%s","cycles":0,"reservations":{"active":{"reservedAt":"2000-01-01T00:00:00Z","capMin":%s}}}\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$fixture_minimum_cap_min" \
    >"$agent_repo/artifacts/agents/missions/mission-concurrency/fences.json"
  stamp_fixture_contract mission-concurrency
  agent_fails fence-concurrency 'mission fence refused job (concurrency)' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-concurrency --mission mission-concurrency --wait
  assert_fence_ask mission-concurrency concurrency

  make_fence_mission mission-timeout 10 10 2 2
  stamp_fixture_contract mission-timeout
  mission_timeout_result="$agent_fixture/mission-timeout.status"
  wait_for_agent_census_fresh mission-timeout-job
  (
    set +e
    cd "$agent_repo"
    # Same self-heal transient the shared runner retries; this dispatch runs
    # detached from that runner, so it carries the same bounded retry itself.
    for _ in 1 2 3; do
      output=$(scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id mission-timeout-job --mission mission-timeout --cap-min "$fixture_minimum_cap_min" --wait 2>&1)
      driver_status=$?
      printf '%s\n' "$output"
      [[ $driver_status -eq 0 ]] && break
      printf '%s' "$output" | grep -Fq 'censusGeneration=' || break
      # Same record boundary as the shared runner: once the job exists, the
      # nonzero status is the fixture's answer (here: the reaped job's exit 4).
      # Re-dispatching a reaped job id spawned a zombie adapter that raced
      # the suite's own git operations in this repository.
      [[ -e artifacts/agents/jobs/mission-timeout-job.json ]] && break
      sleep 1
    done
    printf '%s\n' "$driver_status" >"$mission_timeout_result"
  ) >"$agent_fixture/mission-timeout.out" 2>&1 &
  mission_timeout_driver=$!
  wait_for_agent_status mission-timeout-job running
  python3 - "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" <<'PY'
import json,sys
from pathlib import Path
# capDeadline first, startedAt only as fallback — backdate both (see the
# timed fixture above).
path=Path(sys.argv[1]); value=json.loads(path.read_text())
value["startedAt"]="2000-01-01T00:00:00Z"
value["capDeadline"]="2000-01-01T00:01:00Z"
path.write_text(json.dumps(value)+"\n")
PY
  run_agent_fixture mission-timeout-reap mission-timeout-job "$agent_dispatch" reap --job mission-timeout-job
  wait_for_agent_fixture_process mission-timeout-driver mission-timeout-job "$mission_timeout_driver"
  [[ "$(cat "$mission_timeout_result")" == 4 ]] || {
    echo "mission job timeout did not map to exit 4 (got $(cat "$mission_timeout_result"))" >&2
    python3 -c 'import json,sys;v=json.load(open(sys.argv[1]));print("status:",v.get("status"),"error:",v.get("error"),"phase:",v.get("phase"))' "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" >&2 2>/dev/null || true
    echo "--- driver output:" >&2; sed -n '1,40p' "$agent_fixture/mission-timeout.out" >&2 2>/dev/null || true
    exit 1; }
  assert_fence_ask mission-timeout job-cap-min

  # A provider-native unit with the same spelling from another provider stays
  # a separate typed tuple; no heterogeneous mission total exists.
  python3 - "$agent_repo/artifacts/agents/jobs/other-provider.json" <<'PY'
import json,sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({"jobId":"other-provider","mission":"mission-alpha","runtime":"other","status":"completed","usage":{"availability":"native","inputTokens":3,"cachedInputTokens":None,"outputTokens":None,"reasoningTokens":None,"cost":None,"providerUnits":{"name":"fake-unit","value":5}}})+"\n")
PY
  "$agent_repo/bin/metasystem" mission fence-aggregate-usage --repo "$agent_repo" --mission mission-alpha
  python3 - "$agent_repo/artifacts/agents/missions/mission-alpha/usage.json" <<'PY'
import json,sys
value=json.load(open(sys.argv[1])); units={(item["provider"],item["unit"]):item["value"] for item in value["units"]}
# 4 = explicit + inherited + the two envelope-allowlisted jobs F-4 added.
assert units[("fake","provider.fake-unit")] == 4
assert units[("other","provider.fake-unit")] == 5
assert not any(item["unit"] == "provider.total" for item in value["units"])
PY
  agent_fails missing-mission-lease 'does not have a live' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id missing-mission --mission missing
  agent_fails ambiguous-mission 'ambiguous mission context' env METASYSTEM_MISSION_ID=mission-alpha METASYSTEM_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-ambiguous --mission another
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mission"])' "$agent_repo/artifacts/agents/jobs/happy.json")" == None ]] \
    || { echo "unstamped interactive dispatch gained mission authority" >&2; exit 1; }
  kill "$mission_pid" 2>/dev/null || true
  wait_for_agent_fixture_process mission-lease-holder - "$mission_pid" 2>/dev/null || true
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$agent_identity_fixture"

  agent_fails unknown-status-job '' "$agent_dispatch" status --job absent
  set +e
  "$agent_dispatch" status --job absent >/dev/null 2>&1
  unknown_status=$?
  set -e
  [[ $unknown_status -eq 6 ]] || { echo "unknown status job mapped to $unknown_status instead of 6" >&2; exit 1; }
  printf '{malformed\n' >"$agent_repo/artifacts/agents/jobs/malformed-status.json"
  set +e
  "$agent_dispatch" status --job malformed-status >/dev/null 2>&1
  malformed_record_status=$?
  set -e
  [[ $malformed_record_status -eq 7 ]] || { echo "malformed status record mapped to $malformed_record_status instead of 7" >&2; exit 1; }

  "$agent_repo/scripts/agents/arm-supervision.sh" --repo "$agent_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=

  # The minimal mission runner is exercised only through its fake host. The
  # repository, origin, supervision set, signed contracts, frozen gate, turn
  # records, state, and ledger are all real; only the model call is simulated.
  # Unlike the dispatch fixtures above, it owns an isolated repository armed
  # against real process sources so the census can observe each freshly
  # launched runner. This is scoped like the nested ordinary-operator fixture
  # in supervision-fixtures.sh.
  runner_process_env=(env -u METASYSTEM_CENSUS_PROCESS_FILE -u METASYSTEM_FAKE_PROCESS_IDENTITY_FILE)
  runner="$runner_repo/scripts/agents/mission-runner.sh"
  runner_origin="$agent_fixture/runner-origin.git"
  runner_mission_identity_fixture="$agent_fixture/runner-mission-process-identities.json"
  mv "$runner_repo/scripts/agents/arm-supervision.sh" \
    "$runner_repo/scripts/agents/arm-supervision-real.sh"
  cat >"$runner_repo/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
fixture_root=$(git -C "$script_dir" rev-parse --show-toplevel)
called_at=$(date +%s)
"$script_dir/arm-supervision-real.sh" "$@"
[[ ${1:-} == fingerprint ]] && exit 0
for argument in "$@"; do [[ "$argument" == --shutdown ]] && exit 0; done
deadline=$((called_at + ${METASYSTEM_FIXTURE_AGENT_STATUS_CAP_SEC:?}))
while (( $(date +%s) <= deadline )); do
  completed=$(python3 - "$fixture_root/artifacts/agents/supervision/last-census.json" <<'PY' 2>/dev/null || true
import json,sys
try: print(json.load(open(sys.argv[1]))["completedAtEpoch"])
except (OSError,ValueError,KeyError,TypeError): pass
PY
  )
  [[ ${completed:-0} -ge $called_at ]] && break
  sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:?}"
done
if [[ -n "${METASYSTEM_MISSION_PROCESS_IDENTITY_FILE:-}" \
  && -f "$fixture_root/artifacts/agents/supervision/state.json" ]]; then
  python3 - "$fixture_root/artifacts/agents/supervision/state.json" \
    "$METASYSTEM_MISSION_PROCESS_IDENTITY_FILE" <<'PY'
import json,sys
from pathlib import Path
state=json.loads(Path(sys.argv[1]).read_text()); identities={}
for name in ("watcher","reaper"):
    value=state["components"][name]
    identities[str(value["pid"])]= {
        "pidStartedAt":value["pidStartedAt"],
        "command":f"fixture {value['instanceTag']}",
    }
Path(sys.argv[2]).write_text(json.dumps(identities)+"\n")
PY
fi
ARM
  chmod +x "$runner_repo/scripts/agents/arm-supervision.sh"
  runner_main_start=$("${runner_process_env[@]}" \
    "$runner_repo/bin/metasystem" proc started-at --pid "$$")
  agent_supervision_repo=$runner_repo
  track_armed_supervision "$runner_repo"
  "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake \
    "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --session runner-validator --pid "$$" \
    --start-time "$runner_main_start" --tag metasystem-main-fake-runner-validator \
    >"$agent_fixture/runner-arming.out" \
    || { echo "mission runner fixture could not arm real-source supervision" >&2; cat "$agent_fixture/runner-arming.out" >&2; exit 1; }
  mkdir -p "$runner_repo/scripts" "$runner_repo/truth"
  cat >"$runner_repo/scripts/gate.sh" <<'GATE'
#!/usr/bin/env bash
set -euo pipefail
score=$(cat candidate-score.txt)
printf 'metric=score=%s\n' "$score"
GATE
  chmod +x "$runner_repo/scripts/gate.sh"
  printf '0\n' >"$runner_repo/candidate-score.txt"
  printf 'runner truth\n' >"$runner_repo/truth/reference.txt"
  runner_git config user.name metasystem
  runner_git config user.email metasystem@example.invalid
  runner_git add scripts/gate.sh candidate-score.txt truth/reference.txt
  runner_git commit -qm 'add mission runner instruments'
  runner_git tag runner-instruments
  git init -q -b main --bare "$runner_origin"
  runner_branch=$(runner_git branch --show-current)
  runner_git remote add origin "$runner_origin"
  runner_git push -qu -u origin "$runner_branch"
  git -C "$runner_origin" symbolic-ref HEAD "refs/heads/$runner_branch"
  runner_git remote set-head origin -a >/dev/null

  make_runner_contract() { # mission, behavior, cycle fence, optional heading, runtime, model, extra mission keys
    local mission=$1 behavior=$2 cycles=$3 bad_heading=${4:-} runtime=${5:-fake} model=${6:-fake-model} extra_keys=${7:-}
    local contract="$runner_repo/plans/mission-$1.contract.md" contract_sha
    mkdir -p "$runner_repo/plans"
    cat >"$contract" <<EOF
# Intent

Advance the candidate through one unattended fake-host turn.
$bad_heading

# Non-goals

Do not publish or deploy.

# Initial streams

Keep the primary stream active until the frozen gate passes.

\`\`\`mission
gate.command=scripts/gate.sh
gate.ref=runner-instruments
gate.paths=scripts/gate.sh
truth.paths=truth/reference.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.score.command=scripts/gate.sh
guard.score.floor=1
guard.score.noise=0
guard.cadence=1
ledger.cycle-budget=5
ledger.no-gain-budget=3
fence.wall-clock-hours=2
fence.cycles=$cycles
fence.jobs=4
fence.concurrency=1
fence.job-cap-min=$fixture_mission_job_cap_min
host.runtime=$runtime
host.model=$model
host.turn-cap-min=$fixture_minimum_cap_min
stream.primary=FAKEHOST:$behavior advance the candidate.
envelope.dependencies=jq
exposure=EUR:1${extra_keys:+
$extra_keys}
\`\`\`
EOF
    contract_sha=$("$runner_repo/scripts/assert-mission.sh" --seal --file "$contract")
    printf '\nApproval: name=Fixture-Human; date=2026-08-04; contract-sha256=%s\n' "$contract_sha" >>"$contract"
    runner_git add "plans/mission-$mission.contract.md"
    runner_git commit -qm "sign mission $mission"
    runner_git push -qu origin "$runner_branch"
  }

  wait_lease_released() { # mission, description
    local mission=$1 what=$2 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
    # The runner writes the terminal mission status inside its cycle and
    # releases the lease as it exits, so release trails the status by design.
    while (( SECONDS < deadline )); do
      [[ -e "$runner_repo/artifacts/agents/missions/$mission/lease.d" ]] || return 0
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    echo "$what retained its runner lease (elapsed: $((SECONDS - started))s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
    exit 1
  }

  run_runner_expect() { # name, expected exit, command...
    local name=$1 expected=$2 result
    shift 2
    set +e
    run_agent_fixture_captured "$name" - "$agent_fixture/$name.out" "$@"
    result=$?
    set -e
    if [[ $result -ne $expected ]]; then
      echo "mission runner fixture $name exited $result instead of $expected" >&2
      cat "$agent_fixture/$name.out" >&2
      exit 1
    fi
  }

  wait_runner_status() { # mission, expected exit
    local mission=$1 expected=$2 result=7 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
    while (( SECONDS < deadline )); do
      set +e
      "$runner" status --mission "$mission" >"$agent_fixture/status-$mission.out" 2>&1
      result=$?
      set -e
      [[ $result -eq $expected ]] && return 0
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    echo "mission runner status timed out: $mission -> $expected (last exit: $result)" >&2
    cat "$agent_fixture/status-$mission.out" >&2
    return 1
  }

  wait_runner_file() { # path, description
    local path=$1 description=$2 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
    while (( SECONDS < deadline )); do
      [[ -e "$path" ]] && return 0
      sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    echo "mission runner file wait timed out: $description (elapsed: $((SECONDS - started))s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
    return 1
  }

  start_atomic_result_watcher() { # result path, fixture name
    local result_path=$1 name=$2
    python3 - "$result_path" "$agent_fixture_cap_sec" "$METASYSTEM_FIXTURE_POLL_INTERVAL_MS" \
      >"$agent_fixture/$name.out" 2>&1 <<'PY' &
import json, sys, time
from pathlib import Path
path, cap, poll_ms = Path(sys.argv[1]), int(sys.argv[2]), int(sys.argv[3])
deadline = time.monotonic() + cap
while time.monotonic() < deadline:
    if path.exists():
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            raise SystemExit(f"host result was observable in a partial state: {error}")
        assert set(value) == {"sessionId", "outcome", "usage", "rawPath", "returnPath"}, value
        assert value["outcome"] == "completed" and value["sessionId"] == "codex-fixture-session", value
        raise SystemExit(0)
    time.sleep(poll_ms / 1000)
raise SystemExit(f"host result did not appear within {cap}s: {path}")
PY
    atomic_result_watcher_pid=$!
  }

  printf '{}\n' >"$runner_mission_identity_fixture"
  export METASYSTEM_MISSION_PROCESS_IDENTITY_FILE="$runner_mission_identity_fixture"

  make_runner_contract runner-cycle return-ok 5
  # Both candidate-score commits are state-dependent without --allow-empty:
  # whether the byte they write already sits at HEAD depends on how earlier
  # missions' anchors interleaved, and each variant of this flake has now
  # red-gated an unrelated change. The classification needs the sha to
  # advance; the gate reads the file contents either way.
  printf '1\n' >"$runner_repo/candidate-score.txt"
  runner_git add candidate-score.txt
  runner_git commit --allow-empty -qm 'improve mission runner candidate'
  runner_git push -qu origin "$runner_branch"
  run_runner_expect runner-cycle-start 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-cycle
  wait_runner_status runner-cycle 10
  cycle_turn=$(find "$runner_repo/artifacts/agents/missions/runner-cycle/turns" -mindepth 1 -maxdepth 1 -type d | head -1)
  "$runner_repo/scripts/assert-turn-prompt.sh" --file "$cycle_turn/prompt.md" --turn "$cycle_turn"
  python3 - "$runner_repo" "$cycle_turn/prompt.md" <<'PY'
import sys
from pathlib import Path
root,prompt_path=Path(sys.argv[1]),Path(sys.argv[2])
prompt=prompt_path.read_bytes(); header_end=prompt.index(b"\n\n")
header=prompt[:header_end].decode().splitlines()
assert [line.split(": ",1)[0] for line in header] == [
    "Mission-Id","Turn-Id","Cycle","Host-Session","Runtime","Model","Reconciliation"
]
preamble=(root/"scripts/agents/roles/orchestrator.md").read_bytes()
assert prompt[header_end+2:header_end+2+len(preamble)] == preamble
text=prompt.decode(); headings=[
    "## Mission Contract","## Ledger Tail","## Open Asks",
    "## Streams","## Reconciliation","## Landed Returns","## This Turn",
]
positions=[text.index(heading) for heading in headings]
assert positions == sorted(positions)
PY
  grep -Fq -- '- Classification: contract-improved;' \
    "$runner_repo/artifacts/agents/missions/runner-cycle/ledger.md" \
    || { echo "full mission cycle did not record runner-measured contract improvement" >&2; exit 1; }
  # The degenerate case, live (plans/patience-satellite-4.md): a mission
  # whose contract carries no patience entries books no Patience line.
  if grep -q 'Patience' "$runner_repo/artifacts/agents/missions/runner-cycle/ledger.md"; then
    echo "an unconfigured mission booked a Patience line" >&2; exit 1
  fi

  # Patience floors, live (plans/patience-satellite-4.md): a sealed floor
  # plus a seeded orphan chain — one mission-stamped, terminal, started,
  # unwitnessed record whose parent walk breaks — books the floor-independent
  # orphan report in the same append as the cycle line, and the NEXT prompt's
  # This Turn carries the projected line. The orphan is deliberately not
  # closeable, so the runner's end-of-mission chain close never touches it.
  # The prior mission's runner anchors its exit AFTER its status flips;
  # committing while it still holds the git index races its lock.
  wait_lease_released runner-cycle 'patience fixture entry'
  # Reset the candidate below the gate threshold BEFORE sealing: the sealed
  # baseline must be failing, or the first measurement completes the mission
  # and no drought can ever accrue. Restored after the fixture.
  printf '0\n' >"$runner_repo/candidate-score.txt"
  runner_git add candidate-score.txt
  runner_git commit --allow-empty -qm 'reset candidate for the patience fixture'
  runner_git push -qu origin "$runner_branch"
  make_runner_contract runner-patience return-ok 6 '' fake fake-model \
    'patience.rounds.design-critic.fake.fake-model=1'
  mkdir -p "$runner_repo/artifacts/agents/jobs"
  cat >"$runner_repo/artifacts/agents/jobs/pat-lost.json" <<EOF
{"jobId": "pat-lost", "mission": "runner-patience", "parentJob": "pat-gone",
 "status": "completed", "role": "design-critic", "runtime": "fake",
 "effectiveModel": "fake-model", "requestedModel": "fake-model",
 "startedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)", "endedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"}
EOF
  run_runner_expect runner-patience-start 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-patience
  wait_runner_status runner-patience 11
  patience_ledger="$runner_repo/artifacts/agents/missions/runner-patience/ledger.md"
  grep -Fq -- '- Patience: orphan=pat-lost rounds=1' "$patience_ledger" \
    || { echo "patience orphan report missing from the ledger" >&2; cat "$patience_ledger" >&2; exit 1; }
  patience_turns=$(find "$runner_repo/artifacts/agents/missions/runner-patience/turns" \
    -mindepth 1 -maxdepth 1 -type d | sort)
  patience_turn_1=$(printf '%s\n' "$patience_turns" | sed -n 1p)
  patience_turn_2=$(printf '%s\n' "$patience_turns" | sed -n 2p)
  [[ -n "$patience_turn_2" ]] || { echo "the patience mission ran fewer than two turns" >&2; exit 1; }
  if grep -q 'Patience' "$patience_turn_1/prompt.md"; then
    echo "the first prompt carried a Patience line before any booking" >&2; exit 1
  fi
  grep -Fq 'Patience: orphan job pat-lost has unwitnessed spend' "$patience_turn_2/prompt.md" \
    || { echo "the second prompt did not project the patience line" >&2; exit 1; }
  "$runner_repo/scripts/assert-turn-prompt.sh" \
    --file "$patience_turn_2/prompt.md" --turn "$patience_turn_2"
  rm -f "$runner_repo/artifacts/agents/jobs/pat-lost.json"
  wait_lease_released runner-patience 'patience fixture exit'
  printf '1\n' >"$runner_repo/candidate-score.txt"
  runner_git add candidate-score.txt
  runner_git commit --allow-empty -qm 'restore candidate after the patience fixture'
  runner_git push -qu origin "$runner_branch"
  echo "patience floor fixtures passed"

  # Exercise the real mission runner through the Codex host with only the paid
  # model call replaced. Two turns prove first-turn and resumed identity,
  # workspace entry for `codex exec resume`, typed usage, atomic host results,
  # and the live instance tag that releases the runner's start gate.
  codex_host_fixture="$agent_fixture/codex-host"
  codex_host_bin="$codex_host_fixture/bin"
  mkdir -p "$codex_host_bin"
  cat >"$codex_host_bin/codex" <<'CODEX'
#!/usr/bin/env bash
set -euo pipefail
fixture=${METASYSTEM_CODEX_FIXTURE_DIR:?}
cap=${METASYSTEM_CODEX_FIXTURE_TIMEOUT_SEC:?}
mkdir -p "$fixture"
if [[ ! -e "$fixture/request-1.args" ]]; then
  sequence=1
elif [[ ! -e "$fixture/request-2.args" ]]; then
  sequence=2
else
  echo "codex host fixture received an unexpected third turn" >&2
  exit 9
fi
printf '%s\0' "$@" >"$fixture/request-$sequence.args"
printf '%s\n' "$PWD" >"$fixture/request-$sequence.cwd"
prompt="$fixture/request-$sequence.prompt"
cat >"$prompt"
printf 'ready\n' >"$fixture/ready-$sequence"
deadline=$((SECONDS + cap))
while [[ ! -e "$fixture/release-$sequence" ]]; do
  (( SECONDS < deadline )) || { echo "codex host fixture release $sequence timed out" >&2; exit 9; }
  sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:?}"
done
output=
arguments=("$@")
for ((index=0; index<${#arguments[@]}; index++)); do
  if [[ ${arguments[$index]} == -o && $((index + 1)) -lt ${#arguments[@]} ]]; then
    output=${arguments[$((index + 1))]}
  fi
done
[[ -n "$output" ]] || { echo "codex host fixture received no -o path" >&2; exit 9; }
if [[ $sequence -eq 2 ]]; then
  printf '1\n' >"$PWD/candidate-score.txt"
  git add candidate-score.txt
  # What the classification needs is the candidate SHA advancing with score 1,
  # not a content change: whether 1 is already committed here depends on how
  # the earlier missions interleaved, and requiring a diff made this a flake.
  git commit --allow-empty -qm 'improve candidate from codex host fixture'
fi
python3 - "$prompt" "$output" "$sequence" <<'PY'
import json, sys
from pathlib import Path
prompt, output, sequence = Path(sys.argv[1]), Path(sys.argv[2]), int(sys.argv[3])
headers = dict(
    line.split(": ", 1)
    for line in prompt.read_text(encoding="utf-8").split("\n\n", 1)[0].splitlines()
)
declared = None if headers["Host-Session"] == "none" else headers["Host-Session"]
if sequence == 1:
    assert declared is None
else:
    assert declared == "codex-fixture-session"
value = {
    "turnId": headers["Turn-Id"],
    "missionId": headers["Mission-Id"],
    "cycle": int(headers["Cycle"]),
    "dispatched": [],
    "certified": [],
    "streamUpdatesRequested": [],
    "askCandidates": [],
    "factsForLedger": [],
    "gaps": [],
    "identity": {
        "runtime": headers["Runtime"],
        "model": headers["Model"],
        "sessionId": declared,
    },
}
output.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
printf '%s\n' \
  '{"type":"thread.started","thread_id":"codex-fixture-session"}' \
  "{\"type\":\"turn.started\",\"turn_id\":\"codex-fixture-turn-$sequence\"}" \
  '{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":4,"reasoning_output_tokens":1}}'
CODEX
  chmod +x "$codex_host_bin/codex"

  printf '0\n' >"$runner_repo/candidate-score.txt"
  runner_git add candidate-score.txt
  runner_git commit --allow-empty -qm 'reset candidate for codex host mission'
  runner_git push -qu origin "$runner_branch"
  make_runner_contract runner-codex return-ok 5 '' codex gpt-5-fixture
  run_runner_expect runner-codex-start 0 "${runner_process_env[@]}" \
    PATH="$codex_host_bin:$PATH" METASYSTEM_AGENT_RUNTIME=fake \
    METASYSTEM_CODEX_FIXTURE_DIR="$codex_host_fixture" \
    METASYSTEM_CODEX_FIXTURE_TIMEOUT_SEC="$agent_fixture_cap_sec" \
    "$runner" start --mission runner-codex
  wait_runner_file "$codex_host_fixture/ready-1" "codex host first turn"
  codex_turn_one=$(find "$runner_repo/artifacts/agents/missions/runner-codex/turns" \
    -mindepth 1 -maxdepth 1 -type d | head -1)
  read -r codex_host_pid codex_host_tag < <(python3 - "$codex_turn_one/turn.json" <<'PY'
import json,sys
value=json.load(open(sys.argv[1])); assert value["status"]=="running"
assert value["pid"]==value["pgid"] and value["instanceTag"].startswith("metasystem-host-")
print(value["pid"], value["instanceTag"])
PY
  )
  codex_host_command=$(ps -ww -p "$codex_host_pid" -o command=)
  [[ "$codex_host_command" == *"$codex_host_tag"* ]] \
    || { echo "codex host process did not carry its recorded instance tag" >&2; exit 1; }
  start_atomic_result_watcher "$codex_turn_one/result.json" codex-result-one
  codex_result_one_watcher=$atomic_result_watcher_pid
  touch "$codex_host_fixture/release-1"
  wait_for_agent_fixture_process codex-result-one - "$codex_result_one_watcher" \
    || { cat "$agent_fixture/codex-result-one.out" >&2; exit 1; }
  wait_runner_file "$codex_host_fixture/ready-2" "codex host resumed turn"
  codex_turn_two=$(python3 - "$runner_repo/artifacts/agents/missions/runner-codex/turns" <<'PY'
import json,sys
from pathlib import Path
for path in Path(sys.argv[1]).glob("*/turn.json"):
    if json.loads(path.read_text())["cycle"] == 2:
        print(path.parent)
        raise SystemExit(0)
raise SystemExit("second codex host turn was not created")
PY
  )
  start_atomic_result_watcher "$codex_turn_two/result.json" codex-result-two
  codex_result_two_watcher=$atomic_result_watcher_pid
  touch "$codex_host_fixture/release-2"
  wait_for_agent_fixture_process codex-result-two - "$codex_result_two_watcher" \
    || { cat "$agent_fixture/codex-result-two.out" >&2; exit 1; }
  wait_runner_status runner-codex 10
  wait_lease_released runner-codex "completed codex-host mission"
  python3 - "$runner_repo" "$codex_host_fixture" "$codex_turn_one" "$codex_turn_two" <<'PY'
import json, sys
from pathlib import Path
root, fixture, first, second = map(Path, sys.argv[1:])

def arguments(sequence):
    return [part.decode() for part in (fixture / f"request-{sequence}.args").read_bytes().split(b"\0") if part]

first_args, second_args = arguments(1), arguments(2)
assert first_args[:2] == ["exec", "--json"]
assert first_args[first_args.index("-m") + 1] == "gpt-5-fixture"
assert first_args[first_args.index("--sandbox") + 1] == "workspace-write"
assert first_args[first_args.index("-C") + 1] == str(root)
assert 'approval_policy="never"' in first_args
assert "sandbox_workspace_write.network_access=true" in first_args
assert second_args[:3] == ["exec", "resume", "--json"]
assert "-C" not in second_args
assert 'model="gpt-5-fixture"' in second_args
assert 'sandbox_mode="workspace-write"' in second_args
assert 'approval_policy="never"' in second_args
assert "sandbox_workspace_write.network_access=true" in second_args
assert "codex-fixture-session" in second_args
assert (fixture / "request-1.cwd").read_text().strip() == str(root)
assert (fixture / "request-2.cwd").read_text().strip() == str(root)

expected_result_fields = {"sessionId", "outcome", "usage", "rawPath", "returnPath"}
expected_usage = {
    "availability": "native", "inputTokens": 10, "cachedInputTokens": 2,
    "outputTokens": 4, "reasoningTokens": 1, "cost": None, "providerUnits": None,
}
for turn in (first, second):
    result = json.loads((turn / "result.json").read_text())
    assert set(result) == expected_result_fields and result["outcome"] == "completed"
    assert result["sessionId"] == "codex-fixture-session" and result["usage"] == expected_usage
    assert Path(result["rawPath"]).resolve() == (turn / "raw.out").resolve()
    assert Path(result["returnPath"]).resolve() == (turn / "return.json").resolve()
    assert not list(turn.glob("result.json.*.tmp"))
first_return = json.loads((first / "return.json").read_text())
second_return = json.loads((second / "return.json").read_text())
second_turn = json.loads((second / "turn.json").read_text())
assert first_return["identity"]["sessionId"] is None
assert second_turn["hostSession"] == "codex-fixture-session"
assert second_return["identity"]["sessionId"] == second_turn["hostSession"]
PY
  runner_git push -qu origin "$runner_branch"

  run_runner_expect prompt-missing-turn 1 "$runner_repo/bin/metasystem" mission prompt-assemble \
    --repo "$runner_repo" \
    --mission runner-cycle --turn runner-cycle-t99-missing --output "$agent_fixture/missing-prompt.md"
  grep -Fq 'missing turn record' "$agent_fixture/prompt-missing-turn.out" \
    || { echo "prompt assembler did not name its missing turn record refusal" >&2; exit 1; }
  run_runner_expect prompt-oversized 1 env METASYSTEM_MISSION_MAX_PROMPT_KB=1 \
    "$runner_repo/bin/metasystem" mission prompt-assemble --repo "$runner_repo" --mission runner-cycle \
    --turn "$(basename "$cycle_turn")" --output "$agent_fixture/oversized-prompt.md"
  grep -Fq 'oversized block' "$agent_fixture/prompt-oversized.out" \
    || { echo "prompt assembler did not name the oversized block" >&2; exit 1; }

  make_runner_contract runner-bad-prompt return-ok 5 '## Streams'
  run_runner_expect runner-bad-prompt-start 3 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-bad-prompt
  wait_runner_status runner-bad-prompt 11
  bad_turn=$(find "$runner_repo/artifacts/agents/missions/runner-bad-prompt/turns" -mindepth 1 -maxdepth 1 -type d | head -1)
  [[ ! -e "$bad_turn/raw.out" ]] || { echo "prompt-checker refusal launched the fake host" >&2; exit 1; }
  grep -Fq 'prompt-refused' "$bad_turn/turn.json" \
    || { echo "prompt-checker refusal was not recorded on the turn" >&2; exit 1; }

  make_runner_contract runner-ghost dispatch-ghost 5
  run_runner_expect runner-ghost-start 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-ghost
  wait_runner_status runner-ghost 10
  python3 - "$runner_repo/artifacts/agents/missions/runner-ghost" <<'PY'
import json,sys
from pathlib import Path
mission=Path(sys.argv[1]); state=json.loads((mission/"state.json").read_text())
rejected=state["turnLog"][-1]["rejected"]
assert len(rejected)==1 and rejected[0]["kind"]=="dispatched"
assert "does not exist" in rejected[0]["reason"]
ask=json.loads((mission/"asks"/f"{rejected[0]['askId']}.json").read_text())
assert ask["reasonClass"]=="host-failure" and ask["answeredAt"] is None
PY

  make_runner_contract runner-fence return-ok 1
  mkdir -p "$runner_repo/artifacts/agents/missions/runner-fence"
  printf '{"schemaVersion":1,"missionId":"runner-fence","startedAt":"%s","cycles":1,"reservations":{}}\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$runner_repo/artifacts/agents/missions/runner-fence/fences.json"
  run_runner_expect runner-fence-start 3 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-fence
  wait_runner_status runner-fence 11
  python3 - "$runner_repo/artifacts/agents/missions/runner-fence" <<'PY'
import json,sys
from pathlib import Path
mission=Path(sys.argv[1]); state=json.loads((mission/"state.json").read_text())
assert state["status"]=="parked" and state["parkReason"]=="fence"
asks=[json.loads(path.read_text()) for path in (mission/"asks").glob("*.json")]
assert any(ask["reasonClass"]=="fence" and ask["answeredAt"] is None for ask in asks)
PY

  make_runner_contract runner-unverified return-ok 5
  run_runner_expect runner-unverified-start 3 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake \
    METASYSTEM_FAKE_HOST_START_UNVERIFIED=1 "$runner" start --mission runner-unverified
  wait_runner_status runner-unverified 11
  unverified_ask=$(python3 - "$runner_repo/artifacts/agents/missions/runner-unverified" <<'PY'
import json,sys
from pathlib import Path
mission=Path(sys.argv[1]); state=json.loads((mission/"state.json").read_text())
assert state["parkReason"]=="host-failure"
turns=list((mission/"turns").glob("*/turn.json")); assert len(turns)==1
turn=json.loads(turns[0].read_text()); assert turn["error"]=="start-unverified"
asks=[json.loads(path.read_text()) for path in (mission/"asks").glob("*.json")]
ask=next(value for value in asks if value["reasonClass"]=="host-failure" and value["answeredAt"] is None)
print(ask["askId"])
PY
  )
  run_runner_expect runner-unverified-answer 0 "$runner" answer --mission runner-unverified \
    --ask "$unverified_ask" --answer acknowledged
  wait_runner_status runner-unverified 0

  "${runner_process_env[@]}" "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=
  [[ ! -e "$runner_repo/artifacts/agents/missions/runner-unverified/lease.d" ]] \
    || { echo "parked mission retained its runner lease" >&2; exit 1; }
  agent_supervision_repo=$runner_repo
  track_armed_supervision "$runner_repo"
  run_runner_expect runner-unverified-resume 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" resume --mission runner-unverified
  wait_runner_status runner-unverified 10
  [[ -f "$runner_repo/artifacts/agents/supervision/state.json" ]] \
    || { echo "resume did not re-arm supervision" >&2; exit 1; }
  wait_lease_released runner-unverified "completed resumed mission"
  resumed_prompt=$(find "$runner_repo/artifacts/agents/missions/runner-unverified/turns" -name prompt.md | sort | tail -1)
  grep -Fq 'Reconciliation: yes' "$resumed_prompt" \
    || { echo "resumed turn did not carry reconciliation" >&2; exit 1; }
  grep -Fq $'\tfailed\tstart-unverified' "$resumed_prompt" \
    || { echo "resumed turn omitted the failed prior turn from reconciliation" >&2; exit 1; }

  "${runner_process_env[@]}" "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=
  agent_selftest_process_fixture="$agent_fixture/selftest-processes.json"
  agent_selftest_identity_fixture="$agent_fixture/selftest-process-identities.json"
  printf '[]\n' >"$agent_selftest_process_fixture"
  printf '{}\n' >"$agent_selftest_identity_fixture"
  export METASYSTEM_CENSUS_PROCESS_FILE="$agent_selftest_process_fixture"
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$agent_selftest_identity_fixture"
  agent_supervision_repo=$agent_selftest_repo
  track_armed_supervision "$agent_selftest_repo"
  agent_selftest_main_start=$("$agent_selftest_repo/bin/metasystem" proc started-at --pid "$$")
  METASYSTEM_AGENT_RUNTIME=fake "$agent_selftest_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_selftest_repo" --session selftest-validator --pid "$$" \
    --start-time "$agent_selftest_main_start" --tag metasystem-main-fake-selftest-validator \
    >"$agent_fixture/selftest-arming.out" \
    || { echo "adapter selftest fixture could not arm supervision" >&2; exit 1; }
  fake_selftest_adapter="$agent_selftest_repo/scripts/agents/adapters/fake.sh"
  run_agent_fixture_captured fake-selftest - "$agent_fixture/fake-selftest.out" "$fake_selftest_adapter" selftest
  grep -Fq 'full protocol sequence' "$agent_fixture/fake-selftest.out" \
    || { echo "fake adapter selftest did not run its full protocol sequence" >&2; exit 1; }
  python3 - "$agent_selftest_repo/artifacts/agents/selftests" <<'PY'
import json, sys
from pathlib import Path
paths = list(Path(sys.argv[1]).glob("fake-selftest-*.json")); assert paths
value = json.loads(max(paths, key=lambda path: path.stat().st_mtime).read_text())
assert "resume-identity" in value["provenBehaviorally"]
assert {"denied-write", "denied-network"}.issubset(value["provenBehaviorally"])
assert "network" not in value["constructedOnly"]
PY

  "$agent_selftest_repo/scripts/agents/arm-supervision.sh" --repo "$agent_selftest_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=
  unset METASYSTEM_CENSUS_PROCESS_FILE METASYSTEM_FAKE_PROCESS_IDENTITY_FILE \
    METASYSTEM_MISSION_PROCESS_IDENTITY_FILE
  export METASYSTEM_SKIP_AGENT_FIXTURES=1
fi
if (( delegate_scope )); then
  # Nested adopted-copy validations exercise the same static and grammar
  # checks, but must not re-enter process-owning fixtures after this outer run
  # has deliberately left them to the orchestrator.
  export METASYSTEM_SKIP_AGENT_FIXTURES=1
fi

# The shipped Stop hook must stay rooted and surface via JSON output: hooks
# run in the session's cwd, receipt.sh resolves its ledger from there, and a
# non-blocking exit code shows only a first-line hook-error notice.
hooks_json=scripts/enforcement/claude-code-hooks.json
grep -Fq 'cd \"$CLAUDE_PROJECT_DIR\"' "$hooks_json" || { echo "stop hook is not rooted at CLAUDE_PROJECT_DIR" >&2; exit 1; }
grep -Fq 'systemMessage' "$hooks_json" || { echo "stop hook does not surface a systemMessage when a retro is due" >&2; exit 1; }
if grep -Fq '|| true' "$hooks_json"; then
  echo "stop hook masks the retro-due exit code with || true" >&2
  exit 1
fi
if command -v python3 >/dev/null; then
  hook_cmd=$(python3 -c "import json; print(json.load(open('$hooks_json'))['hooks']['Stop'][0]['hooks'][0]['command'])")
  hookrepo="$tmp/hookrepo"
  mkdir -p "$hookrepo/scripts" "$hookrepo/plans" "$hookrepo/bin"
  cp scripts/receipt.sh scripts/metasystem-config.sh "$hookrepo/scripts/"
  cp bin/metasystem "$hookrepo/bin/metasystem"
  cp metasystem.conf "$hookrepo/"
  printf '1|1970-01-01T00:00:01Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|note=aged\n' >"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  grep -q systemMessage <<<"$out" || { echo "stop hook stayed silent on a due retro" >&2; exit 1; }
  printf '%s|%s|RETRO|note=fixture\n' "$(date -u +%s)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  [[ -z "$out" ]] || { echo "stop hook emitted output when no retro is due" >&2; exit 1; }
  printf 'garbage\n' >"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  grep -q "errored" <<<"$out" || { echo "stop hook hid a failing receipt check" >&2; exit 1; }
  if grep -q "retro due" <<<"$out"; then
    echo "stop hook misreported a check error as a due retro" >&2
    exit 1
  fi
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$tmp/definitely-missing" bash -c "$hook_cmd")
  grep -q "project directory" <<<"$out" || { echo "stop hook stayed silent on an unresolvable project directory" >&2; exit 1; }
fi

# The debug-java preflight is optional: absent in adopted repositories that
# excluded the skill, moved into skills/ in JVM repositories that enabled it.
for preflight in optional-skills/debug-java/scripts/preflight.sh skills/debug-java/scripts/preflight.sh; do
  if [[ -x "$preflight" ]]; then
    touch "$tmp/source" "$tmp/artifact"
    "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null
    touch -t 202001010000 "$tmp/artifact"
    if "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null 2>&1; then
      echo "debug preflight accepted a stale artifact" >&2
      exit 1
    fi
    break
  fi
done

cat >"$tmp/good.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | HIGH | Requirement | Behavior | `owner.py` | `owner.py` | `test_owner.py` | Not applicable: pure derivation | DONE | None |
EOF
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/good.md" >/dev/null

# Proof cells on critical/high rows must be concrete: a DONE row whose proof
# is vague prose must fail, or a declared status can outrun its evidence.
sed 's/| `test_owner.py` |/| covered somewhere |/' "$tmp/good.md" >"$tmp/vague.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/vague.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a DONE row with a vague proof cell" >&2
  exit 1
fi
# Keyword-carrying prose is still prose: promises of future proof, and owners
# without a code-shaped token, must fail.
cat >"$tmp/keyword.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | CRITICAL | Requirement | Behavior | someone will own this | we should test this later | needs testing | manual test pending | DONE | None |
EOF
if scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/keyword.md" >/dev/null 2>&1; then
  echo "obligation gate accepted keyword prose as proof and a prose owner" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable |/' "$tmp/good.md" >"$tmp/bare-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bare-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a bare Not applicable without a reason" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable: |/' "$tmp/good.md" >"$tmp/empty-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/empty-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted an empty-delimiter Not applicable" >&2
  exit 1
fi
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | pyproject.toml |/' "$tmp/good.md" >"$tmp/toml.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/toml.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked config-file proof path" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | module.mjs |/' "$tmp/good.md" >"$tmp/mjs.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/mjs.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked filename outside the old whitelist" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | compare e.g. the results |/' "$tmp/good.md" >"$tmp/eg.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/eg.md" >/dev/null 2>&1; then
  echo "obligation gate mistook abbreviation prose for a filename" >&2
  exit 1
fi
# Matrices shown inside fenced code blocks are documentation, not declarations.
{ printf '```markdown\n'; cat "$tmp/good.md"; printf '```\n'; } >"$tmp/fenced.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/fenced.md" >/dev/null 2>&1; then
  echo "obligation gate read a matrix out of a fenced code block" >&2
  exit 1
fi

sed 's/| DONE |/| MISSING |/' "$tmp/good.md" >"$tmp/bad.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bad.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a missing high obligation" >&2
  exit 1
fi

sed 's/| HIGH |/| MEDIUM |/; s/| DONE |/| PARTIAL |/' "$tmp/good.md" >"$tmp/medium.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/medium.md" >/dev/null || {
  echo "obligation gate rejected a valid medium-only matrix" >&2
  exit 1
}

scripts/assert-design-obligation-gate.sh --runtime-required --file docs/examples/design-obligation-matrix.md >/dev/null 2>&1 && {
  echo "example matrix with READY_FOR_RUNTIME passed --runtime-required; negative fixture broken" >&2
  exit 1
}
scripts/assert-design-obligation-gate.sh --file docs/examples/design-obligation-matrix.md >/dev/null

repo="$tmp/baseline-repo"
git init -q -b main "$repo"
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit --allow-empty -qm base
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null) || {
  echo "refactor baseline check blocked on the baseline file's own dirt right after record" >&2
  exit 1
}
git -C "$repo" add plans/refactor-baseline
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null)
echo dirty >"$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null 2>&1); then
  echo "refactor baseline check accepted a dirty worktree" >&2
  exit 1
fi
rm "$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check --max-commits 0 >/dev/null 2>&1); then
  echo "refactor baseline check ignored the commit-count backstop" >&2
  exit 1
fi
# Custom and absolute --file paths normalize to the repository root; paths
# outside the repository are rejected because git cannot see their dirt.
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file plans/custom-baseline >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file plans/custom-baseline >/dev/null) || {
  echo "refactor baseline check blocked a custom relative --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/custom-baseline
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm custom-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$repo/plans/abs-baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "$repo/plans/abs-baseline" >/dev/null) || {
  echo "refactor baseline check blocked an in-repository absolute --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/abs-baseline
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm abs-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/bäseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/bäseline" >/dev/null) || {
  echo "refactor baseline check blocked a non-ASCII --file right after record (quotePath)" >&2
  exit 1
}
git -C "$repo" add "plans/bäseline"
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm nonascii-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/my baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/my baseline" >/dev/null) || {
  echo "refactor baseline check blocked a space-containing --file right after record (C-quoting)" >&2
  exit 1
}
git -C "$repo" add "plans/my baseline"
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm space-baseline
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$tmp/outside-baseline" >/dev/null 2>&1); then
  echo "refactor baseline accepted a --file outside the repository" >&2
  exit 1
fi

(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --eval "declared eval" >/dev/null)
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79 >/dev/null 2>&1); then
  echo "frontier challenge accepted a score below the frontier" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 >/dev/null 2>&1); then
  echo "frontier challenge forgot the stored noise floor" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 --min-delta 0 >/dev/null)
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 82 >/dev/null)
git -C "$repo" add plans/frontier
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm frontier
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 75 --eval "declared eval" >/dev/null 2>&1); then
  echo "frontier record accepted a regression without --force" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=60\neval=declared\nartifact=\n' >"$tmp/frontier-old"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-old" >/dev/null 2>&1; then
  echo "frontier challenge compared against an expired frontier" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-nowindow"
scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-nowindow" >/dev/null
scripts/frontier.sh status --file "$tmp/frontier-nowindow" | grep -qx 'direction=max' || {
  echo "frontier status hid the effective direction of a legacy file" >&2
  exit 1
}
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=sideways\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-malformed"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-malformed" >/dev/null 2>&1; then
  echo "frontier challenge accepted a malformed persisted direction" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-emptydir"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-emptydir" >/dev/null 2>&1; then
  echo "frontier challenge accepted an empty persisted direction" >&2
  exit 1
fi

# Lower-is-better frontiers: persisted direction, force-gated changes, and a
# challenge that only ever uses the stored direction.
(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --direction min --eval "declared eval" --file plans/frontier-min >/dev/null)
git -C "$repo" add plans/frontier-min
git -C "$repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm frontier-min
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79.5 --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction challenge accepted a within-noise improvement" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null)
(cd "$repo" && METASYSTEM_FRONTIER_DIRECTION=max "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null) || {
  echo "challenge honored an environment direction instead of the persisted one" >&2
  exit 1
}
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 85 --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction record accepted a regression without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 99 --direction max --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier record accepted a direction change without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 1 --direction min --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier challenge accepted a direction flag" >&2
  exit 1
fi

scripts/assert-stop-loss.sh --file docs/examples/step-back-ledger.md >/dev/null
printf '### Cycle C1\n- Classification: no-progress\n### Cycle C2\n- Classification: no-progress\n' >"$tmp/stuck.md"
if scripts/assert-stop-loss.sh --file "$tmp/stuck.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed a third cycle after two no-progress results" >&2
  exit 1
fi
printf -- '- Cycle budget: 2\n### Cycle C1\n- Classification: contract-improved\n### Cycle C2\n- Classification: falsified-continue\n' >"$tmp/spent.md"
if scripts/assert-stop-loss.sh --file "$tmp/spent.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted cycle budget" >&2
  exit 1
fi
printf '### Cycle C1\n- Classification: falsified-dead-end\n' >"$tmp/deadend.md"
if scripts/assert-stop-loss.sh --file "$tmp/deadend.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed cycles after a dead end" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: falsified-continue\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted no-gain budget over a mixed trailing sequence" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: contract-improved\n### Cycle E3\n- Classification: unresolved\n### Cycle E4\n- Classification: falsified-continue\n' >"$tmp/nogain-reset.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-reset.md" >/dev/null || {
  echo "stop-loss check failed to reset the no-gain count on a contract-improved cycle" >&2
  exit 1
}
printf '### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: unresolved\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain-optout.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-optout.md" >/dev/null || {
  echo "stop-loss check blocked unresolved cycles without a declared no-gain budget" >&2
  exit 1
}
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-unclassified.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-unclassified.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count let an unclassified cycle vanish from the tail" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: not-contract-improved\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-fakegain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-fakegain.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count reset on a classification merely containing contract-improved" >&2
  exit 1
fi

knob_fixture="$tmp/conf-consuming-scripts"
mkdir -p "$knob_fixture/receipt/scripts" "$knob_fixture/watch/scripts" "$knob_fixture/watch/jobs" \
  "$knob_fixture/receipt/bin" "$knob_fixture/watch/bin"
cp scripts/receipt.sh scripts/metasystem-config.sh "$knob_fixture/receipt/scripts/"
cp bin/metasystem "$knob_fixture/receipt/bin/metasystem"
printf 'retro.max-receipts=0\nretro.max-age-days=30\n' >"$knob_fixture/receipt/metasystem.conf"
"$knob_fixture/receipt/scripts/receipt.sh" add --type implement --outcome shipped --file "$knob_fixture/receipt/receipts.log" >/dev/null
if "$knob_fixture/receipt/scripts/receipt.sh" check --file "$knob_fixture/receipt/receipts.log" >/dev/null 2>&1; then
  echo "receipt ignored the metasystem.conf receipt limit" >&2
  exit 1
fi
METASYSTEM_RETRO_MAX_RECEIPTS=2 "$knob_fixture/receipt/scripts/receipt.sh" check --file "$knob_fixture/receipt/receipts.log" >/dev/null \
  || { echo "receipt did not prefer the environment over metasystem.conf" >&2; exit 1; }
METASYSTEM_RETRO_MAX_RECEIPTS=0 "$knob_fixture/receipt/scripts/receipt.sh" check --max-receipts 2 --file "$knob_fixture/receipt/receipts.log" >/dev/null \
  || { echo "receipt did not prefer the flag over the environment" >&2; exit 1; }

cp scripts/watch-background-jobs.sh scripts/metasystem-config.sh "$knob_fixture/watch/scripts/"
cp bin/metasystem "$knob_fixture/watch/bin/metasystem"
printf 'watch.stale-min=7\nwatch.cap-min=%s\n' "$fixture_watcher_config_cap_min" >"$knob_fixture/watch/metasystem.conf"
touch "$knob_fixture/watch/state"
"$knob_fixture/watch/scripts/watch-background-jobs.sh" --dir "$knob_fixture/watch/jobs" --state "$knob_fixture/watch/state" --once >"$knob_fixture/watch.out"
grep -q "stale=7m cap=${fixture_watcher_config_cap_min}m" "$knob_fixture/watch.out" \
  || { echo "watcher ignored metasystem.conf ceilings" >&2; exit 1; }

refactor_knob="$knob_fixture/refactor"
mkdir -p "$refactor_knob/scripts" "$refactor_knob/bin"
cp scripts/refactor-baseline.sh scripts/metasystem-config.sh "$refactor_knob/scripts/"
cp bin/metasystem "$refactor_knob/bin/metasystem"
# The baseline recorder demands a clean worktree; the engine is a build
# artifact there exactly as in production.
printf 'bin/\n' >"$refactor_knob/.gitignore"
printf 'refactor.max-age-minutes=1440\nrefactor.max-commits=0\n' >"$refactor_knob/metasystem.conf"
git init -q -b main "$refactor_knob"
printf 'fixture\n' >"$refactor_knob/source.txt"
git -C "$refactor_knob" add source.txt metasystem.conf scripts .gitignore
git -C "$refactor_knob" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm initial
(cd "$refactor_knob" && scripts/refactor-baseline.sh record --gate fixture >/dev/null)
git -C "$refactor_knob" add plans/refactor-baseline
git -C "$refactor_knob" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm baseline
if (cd "$refactor_knob" && scripts/refactor-baseline.sh check >/dev/null 2>&1); then
  echo "refactor baseline ignored metasystem.conf commit cadence" >&2
  exit 1
fi
(cd "$refactor_knob" && scripts/refactor-baseline.sh check --max-commits 2 >/dev/null) \
  || { echo "refactor baseline did not prefer the cadence flag" >&2; exit 1; }

rfile="$tmp/receipts.log"
scripts/receipt.sh add --type implement --outcome shipped \
  --delegate codex:fixture-code:implementer-job \
  --delegate claude:fixture-review:code-critic-job --file "$rfile" >/dev/null
grep -q '|delegate=codex:fixture-code:implementer-job,claude:fixture-review:code-critic-job|' "$rfile" \
  || { echo "receipt did not join repeated delegate triples" >&2; exit 1; }
scripts/receipt.sh check --file "$rfile" >/dev/null
scripts/receipt.sh add --type review --outcome reworked --corrections 1 --file "$rfile" >/dev/null
if scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null 2>&1; then
  echo "receipt check ignored the receipt-count backstop" >&2
  exit 1
fi
scripts/receipt.sh retro "fixture retro" --file "$rfile" >/dev/null
scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null
if scripts/receipt.sh add --type bogus --outcome shipped --file "$rfile" >/dev/null 2>&1; then
  echo "receipt add accepted an invalid type" >&2
  exit 1
fi
printf '1|1970-01-01T00:00:01Z|RETRO|note=aged\n' >"$tmp/receipts-aged.log"
scripts/receipt.sh check --max-age-days 0 --file "$tmp/receipts-aged.log" >/dev/null || {
  echo "receipt check demanded a retro over an empty period" >&2
  exit 1
}
scripts/receipt.sh add --type improve --outcome shipped --verify caught --file "$rfile" >/dev/null
scripts/receipt.sh stats --file "$rfile" | grep -q '^receipts=1$' || { echo "receipt stats miscounted the post-retro period" >&2; exit 1; }
scripts/receipt.sh stats --file "$rfile" | grep -q '^type_improve=1$' || { echo "receipt stats missed the improve type" >&2; exit 1; }
scripts/receipt.sh stats --all --file "$rfile" | grep -q '^receipts=3$' || { echo "receipt stats --all miscounted" >&2; exit 1; }

receipt_relation="$tmp/receipt-relation"
mkdir -p "$receipt_relation/scripts" "$receipt_relation/artifacts/agents/jobs" "$receipt_relation/bin"
cp scripts/receipt.sh scripts/metasystem-config.sh "$receipt_relation/scripts/"
cp bin/metasystem "$receipt_relation/bin/metasystem"
printf 'retro.max-receipts=25\nretro.max-age-days=30\n' >"$receipt_relation/metasystem.conf"
python3 - "$receipt_relation/artifacts/agents/jobs" <<'PY'
import json, sys
from pathlib import Path

jobs = Path(sys.argv[1])
values = {
    "fixture-implementer": {
        "jobId": "fixture-implementer", "role": "implementer", "parentJob": None,
    },
    "fixture-critic": {
        "jobId": "fixture-critic", "role": "code-critic", "parentJob": None,
        "reviews": "fixture-implementer",
    },
    "unrelated-critic": {
        "jobId": "unrelated-critic", "role": "code-critic", "parentJob": None,
        "reviews": "another-implementer",
    },
    "waived-implementer": {
        "jobId": "waived-implementer", "role": "implementer", "parentJob": None,
        "critiqueWaived": {"class": "prose-under-30"},
    },
}
for name, value in values.items():
    (jobs / f"{name}.json").write_text(json.dumps(value) + "\n")
PY
relation_log="$receipt_relation/receipts.log"
if "$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
    --skills code-critique --delegate fake:model:fixture-implementer \
    --file "$relation_log" >"$receipt_relation/missing-chain.out" 2>&1; then
  echo "receipt accepted code-critique without a related critic chain" >&2
  exit 1
fi
grep -Fq 'code-critic chain id and the implementer job id' "$receipt_relation/missing-chain.out" \
  || { echo "receipt refusal did not name the missing relation" >&2; exit 1; }
if "$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
    --skills code-critique --delegate fake:model:fixture-implementer \
    --delegate fake:model:unrelated-critic --file "$relation_log" >/dev/null 2>&1; then
  echo "receipt accepted an unrelated critic chain" >&2
  exit 1
fi
"$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
  --skills code-critique --delegate fake:model:fixture-implementer \
  --delegate fake:model:fixture-critic --file "$relation_log" >/dev/null
mkdir -p "$receipt_relation/artifacts/agents/waived-implementer"
printf 'Working Mode: implement\nMission Stream: waiver-stream\n' \
  >"$receipt_relation/artifacts/agents/waived-implementer/brief.md"
"$receipt_relation/scripts/receipt.sh" add --type implement --outcome shipped \
  --delegate fake:model:waived-implementer --file "$relation_log" >/dev/null
grep -Fq '|critique_waived=prose-under-30|waiver_stream=waiver-stream|' "$relation_log" \
  || { echo "receipt did not surface the accepted waiver and its stream" >&2; exit 1; }
"$receipt_relation/scripts/receipt.sh" stats --file "$relation_log" | grep -q '^critique_waivers=1$' \
  || { echo "receipt stats did not count the stream waiver for retro" >&2; exit 1; }

correction_log="$tmp/receipt-correction.log"
scripts/receipt.sh add --type implement --outcome shipped --skills none \
  --file "$correction_log" >/dev/null
original_line=$(sed -n '1p' "$correction_log")
original_epoch=${original_line%%|*}
original_sha1=$(printf '%s' "$original_line" | shasum -a 1 | awk '{print $1}')
scripts/receipt.sh correct --ref-epoch "$original_epoch" --ref-sha1 "$original_sha1" \
  --field skills --was none --now review --reason 'fixture correction' \
  --file "$correction_log" >/dev/null
[[ "$(sed -n '1p' "$correction_log")" == "$original_line" ]] \
  || { echo "receipt correction edited the original line" >&2; exit 1; }
[[ $(wc -l <"$correction_log" | tr -d ' ') == 2 ]] \
  || { echo "receipt correction did not append exactly one line" >&2; exit 1; }
grep -Fq "|CORRECTION|ref_epoch=$original_epoch|ref_sha1=$original_sha1|field=skills|was=none|now=review|reason=fixture correction" "$correction_log" \
  || { echo "receipt correction line lost its unique reference or change fields" >&2; exit 1; }
correction_line=$(sed -n '2p' "$correction_log")
correction_epoch=${correction_line%%|*}
correction_sha1=$(printf '%s' "$correction_line" | shasum -a 1 | awk '{print $1}')
if scripts/receipt.sh correct --ref-epoch "$correction_epoch" --ref-sha1 "$correction_sha1" \
    --field reason --was 'fixture correction' --now invalid --reason 'must not correct a correction' \
    --file "$correction_log" >"$tmp/correct-correction.out" 2>&1; then
  echo "receipt correction accepted an earlier CORRECTION line as its original" >&2
  exit 1
fi
grep -Fq 'must identify an original RECEIPT line' "$tmp/correct-correction.out" \
  || { echo "receipt correction rejected a non-receipt without naming the contract" >&2; exit 1; }

# Every free-text field is sanitized by one shared path: CRLF through the
# note, the skills list, delegates, and the retro summary must each stay one log line.
crlf_fixture=$(printf 'a\r\nb')
rfile_crlf="$tmp/receipts-crlf.log"
scripts/receipt.sh add --type implement --outcome shipped --note "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 1 ]] || { echo "a CRLF note corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --skills "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 2 ]] || { echo "a CRLF skills list corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --delegate "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 3 ]] || { echo "a CRLF delegate corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh retro "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 4 ]] || { echo "a CRLF retro summary corrupted the receipt log" >&2; exit 1; }
if LC_ALL=C grep -q $'\r' "$rfile_crlf"; then
  echo "receipt sanitizer left a carriage return in the log" >&2
  exit 1
fi

# Adopted-mode contract: a copy without the template marker validates with a
# skill pruned, and a present-but-broken skill still fails. Template mode
# only, so the nested run (which lacks development/) cannot recurse.
if (( template_mode )); then
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
    [[ "$entry" == artifacts ]] && continue
    cp -R "$entry" "$to/"
  done)
}

  copy_tree_without_artifacts "$root" "$adopted"
  rm -rf "$adopted/development" "$adopted/skills/improve" "$adopted/plans/receipts.log" "$adopted/.claude"
  sed 's/<[^>]*>/filled/g' "$adopted/docs/project-rules.md" >"$adopted/docs/project-rules.md.new"
  mv "$adopted/docs/project-rules.md.new" "$adopted/docs/project-rules.md"
  perl -0pi -e 's/^metasystem\.runtimes=.*$/metasystem.runtimes=/m; s/^role\..*\n//mg; s/^mode\..*\.role\..*\n//mg' "$adopted/metasystem.conf"
  fill_harness_conf "$adopted/metasystem.conf" "$tmp/adopted-evidence"
  bash "$adopted/scripts/validate-metasystem.sh" >"$tmp/nested-pruned.log" 2>&1 || {
    echo "adopted-mode validation failed for a copy with one skill pruned" >&2
    tail -20 "$tmp/nested-pruned.log" >&2
    exit 1
  }
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
if (( template_mode )); then
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
  git -C "$nested_src" add .
  git -C "$nested_src" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm nested
  nested_tgt="$tmp/adopt-nested-target"
  mkdir -p "$nested_tgt"
  git -C "$nested_tgt" init -q -b main
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
  python3 - "$chain_root/artifacts/agents/jobs/implementer-20260101t000000z-cccc.json" <<'PYEOF'
import json, sys
path = sys.argv[1]
record = json.load(open(path))
record["chainClosed"] = True
json.dump(record, open(path, "w"))
PYEOF
  [[ -n "$("$root/bin/metasystem" report open-work --repo "$chain_root" | grep STALE-PLAN)" ]] \
    || { echo "a closed chain still suppressed the stale report" >&2; exit 1; }
  echo 'ignored-fixture.txt' >>"$srcrepo/.gitignore"
  echo junk >"$srcrepo/ignored-fixture.txt"
  git init -q -b main "$srcrepo"
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
  [[ "$(ls "$tgt/plans" | sort | tr '\n' ' ')" == "README.md instruction-ledger.md known-issues.md " ]] \
    || { echo "adopt: plans/ payload carries more than the standing ledgers" >&2; exit 1; }
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
  diff -r "$snap" "$tgt" >/dev/null || { echo "adopt: second run changed an adopted target" >&2; exit 1; }
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
  bash "$tgt/scripts/validate-metasystem.sh" >/dev/null 2>&1 || { echo "adopt: filled target failed validation" >&2; exit 1; }

  echo drift >>"$tgt/.claude/agents/verify.md"
  if bash "$tgt/scripts/validate-metasystem.sh" >"$tmp/profile-drift.out" 2>&1; then
    echo "adopt: validation missed a drifted claude profile" >&2
    exit 1
  fi
  grep -q 'profile drifted' "$tmp/profile-drift.out" \
    || { echo "adopt: profile-drift failure did not name the profile" >&2; exit 1; }
  cp "$tgt/skills/verify/agents/claude-profile.md" "$tgt/.claude/agents/verify.md"

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
  perl -0pi -e 's/^(.*\.model\.claude)=.*$/$1=claude-model/mg; s/^(.*\.model\.codex)=.*$/$1=codex-model/mg; s/^(.*\.model\.devin)=.*$/$1=devin-model/mg' "$tier_src/metasystem.conf"
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
  bash "$tmp/adopt-copy/scripts/validate-metasystem.sh" >"$tmp/nested-copied-skills.log" 2>&1 \
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

# watch-background-jobs: all four reportable states plus baseline suppression.
# The state file is pre-created because a MISSING state file auto-baselines on
# first run (the 2026-08-03 hardening); an existing empty state means armed.
wbj="$tmp/wbj"; mkdir -p "$wbj/jobs"
printf '{"status":"completed"}' >"$wbj/jobs/done.json"
printf '{"status":"running"}'   >"$wbj/jobs/live.json"
touch "$wbj/s1" "$wbj/s2" "$wbj/s3" "$wbj/s3b" "$wbj/s3c" "$wbj/s6" "$wbj/s7" "$wbj/s8"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s1" --once >"$wbj/o1" 2>&1
grep -q "^DONE done status=completed" "$wbj/o1" || {
  echo "watch-background-jobs: terminal job not reported" >&2; exit 1; }
grep -q "live" "$wbj/o1" && {
  echo "watch-background-jobs: running job reported as terminal" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s1" --once >"$wbj/o2" 2>&1
grep -v "^ARMED " "$wbj/o2" | grep -q . && {
  echo "watch-background-jobs: re-reported an already-reported job" >&2; exit 1; }
# age the record by a controlled 10 minutes so stale and cap are separable
python3 - "$wbj/jobs/live.json" <<'AGE'
import os, sys, time
t = time.time() - 600
os.utime(sys.argv[1], (t, t))
AGE
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s2" --stale-min 5 --cap-min "$fixture_watcher_nonfiring_cap_min" --once >"$wbj/o3" 2>&1
grep -q "^STALE live" "$wbj/o3" || {
  echo "watch-background-jobs: stale job not reported" >&2; exit 1; }
grep -q "^CAPPED live" "$wbj/o3" && {
  echo "watch-background-jobs: hard cap fired inside its own window" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s3" --stale-min 5 --cap-min "$fixture_watcher_firing_cap_min" --once >"$wbj/o4" 2>&1
grep -q "^CAPPED live" "$wbj/o4" || {
  echo "watch-background-jobs: hard cap not reported" >&2; exit 1; }
# A job whose RECORD is old but whose sibling log is fresh is WORKING, not stale.
# Runners write the record once at dispatch and stream progress to the log, so a
# record-only liveness check cries wolf on every long phase.
mkdir -p "$wbj/live-log/jobs"
printf '{"status":"running"}' >"$wbj/live-log/jobs/busy.json"
printf 'building\n' >"$wbj/live-log/jobs/busy.log"
python3 - "$wbj/live-log/jobs/busy.json" <<'AGE'
import os, sys, time
t = time.time() - 3600
os.utime(sys.argv[1], (t, t))
AGE
scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3b" --stale-min 5 --once >"$wbj/o4b" 2>&1
grep -q "^STALE busy" "$wbj/o4b" && {
  echo "watch-background-jobs: reported STALE for a job whose log is advancing" >&2; exit 1; }
# ...but when BOTH files go quiet it is genuinely stale and must still report.
python3 - "$wbj/live-log/jobs/busy.log" <<'AGE'
import os, sys, time
t = time.time() - 3600
os.utime(sys.argv[1], (t, t))
AGE
scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3c" --stale-min 5 --once >"$wbj/o4c" 2>&1
grep -q "^STALE busy" "$wbj/o4c" || {
  echo "watch-background-jobs: missed a genuinely stale job (all files quiet)" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s4" --baseline >/dev/null 2>&1
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s4" --once >"$wbj/o5" 2>&1
grep -q "^DONE done" "$wbj/o5" && {
  echo "watch-background-jobs: baseline did not suppress pre-existing jobs" >&2; exit 1; }
if scripts/watch-background-jobs.sh --state "$wbj/s5" --once >/dev/null 2>&1; then
  echo "watch-background-jobs: accepted a call with no --dir" >&2; exit 1
fi
# sidecar records must not double-report or bypass scope
printf '{"status":"completed","workspaceRoot":"/r/other"}' >"$wbj/jobs/side.json"
printf 'log text, not json\n'                             >"$wbj/jobs/side.log"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s7" --once >"$wbj/o7" 2>&1
grep -q "side" "$wbj/o7" && {
  echo "watch-background-jobs: sidecar record bypassed the scope filter" >&2; exit 1; }
printf '{"status":"completed","workspaceRoot":"/r/mine"}' >"$wbj/jobs/dual.json"
printf 'log text, not json\n'                            >"$wbj/jobs/dual.log"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s8" --once >"$wbj/o8" 2>&1
[ "$(grep -c '^DONE dual' "$wbj/o8")" -eq 1 ] || {
  echo "watch-background-jobs: job with a sidecar did not report exactly once" >&2; exit 1; }
printf '{"jobId":"chain-r2","parentJob":"chain","round":2,"status":"completed","workspaceRoot":"/r/mine"}' >"$wbj/jobs/chain-r2.json"
touch "$wbj/s8b"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s8b" --once >"$wbj/o8b" 2>&1
grep -q '^DONE chain-r2 status=completed' "$wbj/o8b" || {
  echo "watch-background-jobs: follow-up child was not tracked under its own id" >&2; exit 1; }
# scope: own repo and its worktrees in, peer repo and prefix-collision out
printf '{"status":"completed","workspaceRoot":"/r/mine"}'                >"$wbj/jobs/sc-mine.json"
printf '{"status":"completed","workspaceRoot":"/r/mine/.worktrees/w"}'   >"$wbj/jobs/sc-wt.json"
printf '{"status":"completed","workspaceRoot":"/r/other"}'               >"$wbj/jobs/sc-peer.json"
printf '{"status":"completed","workspaceRoot":"/r/mine-other"}'          >"$wbj/jobs/sc-prefix.json"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s6" --once >"$wbj/o6" 2>&1
grep -q "^DONE sc-mine" "$wbj/o6" || {
  echo "watch-background-jobs: in-scope job not reported" >&2; exit 1; }
grep -q "^DONE sc-wt" "$wbj/o6" || {
  echo "watch-background-jobs: worktree job dropped by scope filter" >&2; exit 1; }
grep -q "sc-peer" "$wbj/o6" && {
  echo "watch-background-jobs: peer repository job reported" >&2; exit 1; }
grep -q "sc-prefix" "$wbj/o6" && {
  echo "watch-background-jobs: scope matched on a path prefix" >&2; exit 1; }
# distinct scopes must not share default state. Auto-baseline swallows the
# first pass per fresh default state, so warm each scope, then prove each
# reports a job that arrives after its own arming.
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine  --once >/dev/null 2>&1
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/other --once >/dev/null 2>&1
printf '{"status":"completed","workspaceRoot":"/r/mine"}'  >"$wbj/jobs/nu-mine.json"
printf '{"status":"completed","workspaceRoot":"/r/other"}' >"$wbj/jobs/nu-other.json"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine  --once 2>/dev/null | grep -q "^DONE nu-mine" || {
  echo "watch-background-jobs: post-arming job not reported under its scope's default state" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/other --once 2>/dev/null | grep -q "^DONE nu-other" || {
  echo "watch-background-jobs: distinct scopes shared a default state file" >&2; exit 1; }

if (( delegate_scope )); then
  [[ ${#delegate_skipped_sections[@]} -eq ${#delegate_owed_sections[@]} ]] \
    || { echo "delegate-scope skipped-section accounting drifted" >&2; exit 1; }
  for index in "${!delegate_owed_sections[@]}"; do
    [[ "${delegate_skipped_sections[$index]}" == "${delegate_owed_sections[$index]}" ]] \
      || { echo "delegate-scope skipped-section accounting drifted" >&2; exit 1; }
  done
  echo "delegate-scope validation passed"
  echo "orchestrator still owes these process-visibility sections:"
  printf -- '- %s\n' "${delegate_skipped_sections[@]}"
else
  echo "metasystem validation passed"
fi
