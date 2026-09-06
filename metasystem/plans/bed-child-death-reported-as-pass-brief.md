Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal bed-child-death-reported-as-pass, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

On this host's only bash (3.2.57, the stock macOS bash) the supervision
bed in metasystem/scripts/agents/supervision-fixtures.sh reports a
scenario as passed after its child process died. Reproduced on main at
c1525b90 on 2026-09-06 12:25Z by running a scenario child directly the
way the bed does:

    bash scripts/agents/supervision-fixtures.sh --fixture-bed-child rearm-rebuild <capability file>
    scripts/agents/supervision-fixtures.sh: line 662: enrolled: unbound variable
    child rc=0

Two defects, both to fix:

1. install_rearm_engine at line 661 declares
   `local built=$1 enrolled=$2 staged=$enrolled.replacement` in one
   statement; bash 3.2 expands `$enrolled` before that statement assigns
   it, so under set -u the child dies there, and the engine at the
   enrolled path is never replaced. Every re-arm scenario (rearm-rebuild,
   rearm-launch-fails, rearm-provenance, at lines 978, 1027, 1094, 1104
   and 1115) therefore never exercised a replaced engine. Assign
   `staged` on its own line after `enrolled`. Audit the same file and
   the other fixture scripts under metasystem/scripts/agents whose names
   end in -fixtures.sh for the same shape (a `local` that reads a name
   declared earlier in the same statement); the orchestrator's grep
   found no other instance, but read rather than trust it.

2. The child's EXIT trap, `cleanup` (defined around line 552; it starts
   with `local status=$? keep`, runs the shutdown and stray-process
   sweeps, then keeps or removes the temp directory), lets the script
   exit 0 after an expansion death: the death's status is captured in
   `status` but the function ends without exiting with it, and on bash
   3.2 the shell's exit status is then the trap's last command status.
   Make `cleanup` end with `exit "$status"` on every path that reaches
   its end (the early `return 0` for a second entry stays), and check
   `on_signal`, which calls cleanup and then exits with its own signal
   status, still behaves. Apply the same discipline to
   `fixture_bed_parent_cleanup` at line 18 if its end can swallow a
   status the same way; say what you found.

3. A bed self-test proves the trap discipline: add one scenario to the
   bed's list, named `bed-death-self-test`, whose child body deliberately
   reads an unset variable under set -u inside the same trap regime
   (nothing else; no repository, no engine). The parent loop in
   `run_fixture_bed_scenarios` treats that one scenario as expected to
   die: it passes only when the child's status is nonzero and its log
   carries the "unbound variable" line, and a zero status fails the
   suite with the message "the bed reported a dead child as passed". Put
   it first in the list so the discipline is proven before any scenario
   that relies on it. Ordinary scenarios keep their pass rule.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/scripts/agents/supervision-fixtures.sh
May touch: any other file under metasystem/scripts/agents whose name
ends in -fixtures.sh, only where the audit in point 1 finds the same
shape; name each.
Must not touch: anything else; nothing under plans; no Go code; no
engine behavior.

# Inputs

- metasystem/scripts/agents/supervision-fixtures.sh: the parent loop
  (lines 30 to 75), `fixture_bed_parent_cleanup` (line 18), `cleanup`
  and `on_signal` (around lines 552 to 590), install_rearm_engine (line
  661), the re-arm scenarios (lines 970 to 1130).
- metasystem/scripts/agents/fixture-budget.sh: the private child
  invocation and capability minting (harness_fixture_bed_child_scenario,
  harness_fixture_bed_mint_capability).
- The goal record metasystem/plans/goals/bed-child-death-reported-as-pass.md
  carries the m1 seat's observation that deaths by exit 1 or a failed
  command still propagate; only the expansion-death shape is hidden.

# Constraints

- Bash 3.2 clean throughout: no BASHPID, no associative arrays, no
  `local -n`, no `${var,,}`.
- Never weaken a scenario; the only inverted pass rule is the named
  self-test.
- Do not run the whole bed in your sandbox (it arms beds and reads live
  processes); run the commands below. The orchestrator runs the bed
  seat-side under the stock bash and reads the three re-arm scenarios'
  logs for the receipt.
- Hazard class DESIGN-BEARING: a proof discipline every bed pass rests
  on; an independent critique follows. At most 90 minutes of wall
  clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n 'staged=' ./scripts/agents/supervision-fixtures.sh` (expected: the assignment on its own line)
- `grep -n 'exit "\$status"' ./scripts/agents/supervision-fixtures.sh` (expected: at the end of cleanup, and of the parent cleanup if changed)
- `grep -n 'bed-death-self-test' ./scripts/agents/supervision-fixtures.sh` (expected: the list entry, the child body, the parent's expected-to-die rule)
- A direct child probe you can run without arming anything: `cap=$(mktemp) && printf 'bed-death-self-test\n' > "$cap" && chmod 600 "$cap" && METASYSTEM_SUPERVISION_FIXTURE_SUITE_PID=$$ METASYSTEM_SUPERVISION_FIXTURE_SEAT_REGISTRY_HOME=$HOME bash ./scripts/agents/supervision-fixtures.sh --fixture-bed-child bed-death-self-test "$cap"; echo "rc=$?"` (expected: the unbound-variable line and a nonzero rc)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. The direct child probe above exits nonzero; before this change the
   same shape exited 0.
2. `install_rearm_engine` assigns staged after enrolled on its own
   line; no other same-shape declaration remains in the fixture scripts.
3. The bed's scenario list starts with bed-death-self-test and the
   parent passes it only on a nonzero child status.
4. No scenario's assertions changed.

# Gap Rule

stop and report a gap; never fill it silently.
