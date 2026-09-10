Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round seven: the walk must survive the operating system's own processes

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round six. The fourth critic (hp-terminal-crit4-20260909)
executed the round-six walk on this macOS host and the seat gate ran the
command package; both fail the same way, for the same reason.

## HPT-13 (critical): the walk to the root cannot succeed on macOS

Every macOS ancestry ends at process 1, /sbin/launchd, owned by root,
and the kernel withholds its arguments from an ordinary user (the
critic's Go program printed pid=1 ppid=0 uid=0 argvReadable=false
exec="/sbin/launchd"). The stable read in
internal/humanauthority/authority.go refuses withheld arguments unless
the executable is exactly /usr/bin/login, so every walk to the root ends
ARGV_UNREADABLE: every real human is refused the terminal grade, and
since Enroll now runs the same walk, enrollment is broken on macOS. The
seat's agent-descended bed cannot see it (the walk meets the agent well
below the launcher) and the unit tests cannot either (the fake processes
are always readable and owned by uid 501, including the two process-1
rows round six added).

The rule, decided: anywhere in the walk, a process whose arguments are
withheld is admitted as a non-agent system node only when it is owned
by root AND its executable is a system-owned image the invoking user
cannot have written (the executable file owned by root and writable by
neither group nor others, read stably like the rest); it is recorded in
the proof with its arguments-withheld flag, and the walk continues past
it to process 1. A user-owned process with withheld arguments stays
ARGV_UNREADABLE. This subsumes the /usr/bin/login case. The walk does
NOT end at the first root-owned ancestor: a setuid login above an
agent's pseudo-terminal would hide the agent. Linux keeps the same rule;
its process 1 answers with readable arguments and passes as today.

## The command package: five tests fail ANCESTRY_CYCLE

On the seat, cmd/metasystem fails TestSyncReqLineage (both subtests),
TestStoppingRequestFallsBackToTerminalGrade,
TestStoppingRequestCarriesEnrolledGradeWithoutFallingBack,
TestGoalEnrollTerminalSucceedsOnEveryMachineAndFirstEndsRelay and
TestSessionStopAttendedHumanEndsQuietly, every one with
"ANCESTRY_CYCLE": their fake process tables end in a root whose parent
is itself (or otherwise loops), which the old walk never reached and the
new walk now visits. Make every fake table in the command package and in
internal/humanauthority model the real root: a process 1 with parent 0,
owner 0, arguments withheld, executable the platform launcher; and make
the walk's reading of such a root pass under the rule above. Run these
tests yourself; they need no process enumeration.

## HPT-14 (low)

If the shared readiness allowance expires between the keeper and holder
waits, the holder's start time is never recorded and cleanup skips the
holder, orphaning a 300-second sleep. Let cleanup terminate the holder
by its identity-file pid when the start time was never recorded, after
proving that pid's command is the fake agent.

## Mandate

1. The withheld-arguments rule above, in the stable read, with the node
   recorded (executable digest, arguments-withheld flag, owner known).
2. Unit tests in internal/humanauthority that model the real shape:
   process 1 root-owned with withheld arguments and a system executable
   passes the walk; a user-owned process with withheld arguments
   refuses ARGV_UNREADABLE; an agent below or above a root-owned system
   node still refuses AGENT_IN_AUTHORITY_CHAIN. Plus one live test that
   walks the test process's own ancestry with the real kernel reader
   and asserts the outcome is never ARGV_UNREADABLE and never
   ANCESTRY_CYCLE (whatever else it is on the machine at hand).
3. The five command-package tests pass with realistic fake roots.
4. HPT-14 as above.
5. Nothing else changes.

## Proof

Run internal/humanauthority and the five command-package tests named
above (`go test ./cmd/metasystem/ -run 'TestSyncReqLineage$|TestStoppingRequest|TestGoalEnrollTerminalSucceeds|TestSessionStopAttendedHumanEndsQuietly'`),
gofmt and vet; say what ran. The orchestrator runs the full packages,
the goal-cli bed and the hook suite on the seat. Report the round as
your own.

## Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
