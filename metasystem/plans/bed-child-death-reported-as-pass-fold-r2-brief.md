Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round two of chain bcd-build1 under goal bed-child-death-reported-as-pass, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Round one stopped on a real gap, correctly: on this host's bash 3.2 an
expansion death enters the EXIT trap with `$?` already zero, so
capturing the status and exiting with it cannot tell a dead child from
a finished one. The orchestrator verified that on the stock bash and
verified the design below on it too (a probe with this exact trap
shape: expansion death exits 70, an `exit 1` stays 1, a normal end
exits 0, a TERM exits 143). Build exactly this.

# The design

1. A completion sentinel. Near the traps at lines 578 to 580 of
   metasystem/scripts/agents/supervision-fixtures.sh declare
   `fixture_child_completed=0` and `fixture_signal_status=` before the
   traps are set. The very last statement of the script, after the
   final isolation assertions at its tail (after the
   assert_seat_main_is_rejected block), is `fixture_child_completed=1`.
   Nothing may follow it.

2. cleanup (line 523) keeps `local status=$?` as its first line and its
   early `return 0` for a second entry. Immediately after the
   `cleanup_started=1` line add: if `fixture_signal_status` is
   non-empty, `status=$fixture_signal_status`; otherwise, if
   `fixture_child_completed` is not 1 and `status` is 0, print
   "supervision fixture scenario $fixture_scenario: the scenario child
   died before completing" to stderr and set `status=70`. The body then
   runs unchanged (the evidence-keeping branch already tests `$status
   -ne 0`, so a death now keeps its evidence). The function's last
   statement becomes `exit "$status"`.

3. on_signal (line 571) no longer exits itself: it sets
   `fixture_signal_status=$((128 + signal))`, calls cleanup, and ends;
   cleanup exits with that status. Remove its `trap - EXIT` and `exit`
   lines. `fixture_bed_parent_cleanup` (line 18) already returns the
   incoming status and stays as it is.

4. The two declarations that read a name declared in the same
   statement, which bash 3.2 expands before assigning: install_rearm_engine
   at line 662 (`staged=$enrolled.replacement`) and make_repo at line
   613 (`evidence=$tmp/evidence-$(basename "$repo")`). In both, declare
   the derived name on its own `local` line after the line that assigns
   what it reads. Nothing else in those functions changes.

5. The self-test scenario `bed-death-self-test`, first in the bed's
   scenario list at line 88 (and in the case at line 94): its child
   body deliberately expands an unset variable under set -u inside the
   normal trap regime and does nothing else (no repository, no engine,
   no arming), placed as its own `if [[ "$fixture_scenario" ==
   bed-death-self-test ]]` block before the first real scenario. In
   `run_fixture_bed_scenarios` (lines 30 to 75) that one scenario name
   is expected to die: it counts as passed only when the child's status
   is nonzero and its log contains "unbound variable"; a zero status
   fails the suite with "the bed reported a dead child as passed" and
   is listed among the failed scenarios. Every other scenario keeps the
   existing rule (zero passes).

# Workspace

Your existing worktree for chain bcd-build1 (nothing was changed in
round one).
May touch: metasystem/scripts/agents/supervision-fixtures.sh
Must not touch: anything else.

# Constraints

- Bash 3.2 clean: no BASHPID, no associative arrays, no `local -n`.
- No scenario assertion changes beyond the new self-test.
- Do not run the whole bed; run the commands below. At most 60 minutes
  of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `tail -3 ./scripts/agents/supervision-fixtures.sh` (expected: the completion line last)
- `grep -n 'fixture_child_completed\|fixture_signal_status\|died before completing' ./scripts/agents/supervision-fixtures.sh` (expected: the declarations, the cleanup rule, the final line)
- `grep -n 'bed-death-self-test' ./scripts/agents/supervision-fixtures.sh` (expected: the list entry, the case entry, the child block, the parent rule)
- The direct child probe: `cap=$(mktemp) && printf 'bed-death-self-test\n' > "$cap" && chmod 600 "$cap" && METASYSTEM_SUPERVISION_FIXTURE_SUITE_PID=$$ METASYSTEM_SUPERVISION_FIXTURE_SEAT_REGISTRY_HOME=$HOME bash ./scripts/agents/supervision-fixtures.sh --fixture-bed-child bed-death-self-test "$cap"; echo "rc=$?"` (expected: the unbound-variable line, the "died before completing" line, rc=70)
- `git diff --stat` (expected: the one file)

# Acceptance Criteria

1. The direct child probe exits 70 with both lines on stderr.
2. A scenario ending normally still exits 0 (reason from reading: the
   completion line is last and nothing after it can fail).
3. on_signal's status reaches the exit through cleanup.
4. Both same-statement declarations are split; the self-test is first
   in the list and expected to die.

# Gap Rule

stop and report a gap; never fill it silently.
