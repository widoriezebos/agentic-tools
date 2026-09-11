Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 7: the code critique's five material findings, decided

Chain phd-build1-20260910. The code critic (phd-build1-crit2-20260910,
reviewing round 6, tree 0df8a6d06c51259b0a84d2bb51792df9fe5f85c0) found
five material defects and two notes. The designer decided each below;
every decision binds. Report `round` as 7. Do not fetch or rebase the
worktree. Nothing written is deleted; tests are rewritten in place.

## Decisions

D-R7-1 (PHD-01, critical: seven fixtures race a helper against a
shortened wall window). No fixture of this slice decides an outcome by
a wall margin. Every helper mode announces readiness on its stdout with
one line `ready` after its setup is complete (after signal.Ignore in the
SIGQUIT-ignoring mode; just before it stops itself in the stop mode;
after its grandchild is spinning in the setsid mode; after its first
stage-results write in the section mode); the fixture reads that line
before the supervisor's judgement can start. The scripted reader gates
on readiness: until the handshake is read it reports rising CPU (so no
window can elapse), and only then does it serve its script. Where a
test must wait for the helper's next step (SIGCONT after stop, the exit
of a finite helper) it waits on the reader's calls or on the process,
never on time.After or a fixed sleep. The stop row's five-second wait
goes. The shortened window remains only as the bound on how long an
idle child is watched after readiness.

D-R7-2 (PHD-02, high: the counter is the high-water of a sum and
Darwin cannot see reaped children). The seat verified the critic's
inference: on this fleet's Macs `ps -S` adds nothing for a finished
child (a shell whose reaped child burned seconds shows 0:00.00 with and
without -S). The counter therefore accumulates per member: for each
process id (with its start time where the platform gives one) the
supervisor keeps that member's own high-water CPU and the group counter
is the sum over every member ever observed, retained after the member
vanishes; it can never drop when a member exits. Linux keeps the
parent's cutime/cstime fields, which do fold reaped children in.
Darwin's residual is recorded on the design (residual f): children that
live shorter than one sample interval are invisible to the Darwin
counter; the parent's own consumption and any output still keep the
group alive, and the thirty-minute window makes a false `dead` need a
parent that consumes nothing, writes nothing and only ever runs
sub-ten-second children for half an hour. No libproc call: the engine
is cgo-free and Go on Darwin has no supported raw syscall path.

D-R7-3 (PHD-03, medium: rows 3, 5, 6, 14 never touch the real reader).
Rows 5, 6 and 14 run with the real platform reader and hand-shaken
helpers: row 5's helper starts a Setsid grandchild that spins and
announces `ready` once the grandchild has consumed at least a quarter
second (the helper reads its grandchild's ps or /proc figure itself),
and the fixture asserts the real reader's counter at or above a quarter
second while the direct child stays idle; row 6 on Linux is the
build-tagged half (cutime after short children are reaped) and on
Darwin asserts the per-member accumulation of D-R7-2 with a child that
lives longer than the shortened sample interval; row 14's section
helper writes its stage results from a Setsid child and the fixture
asserts the growth was seen as output by the real tree. Row 3 asserts
the goroutine dump text (`goroutine ` and `SIGQUIT`) in the captured
log after a dead verdict on a Go helper.

D-R7-4 (PHD-04, low: row 12 proves nothing new). Add tests that
ValidateTestResult accepts a `runaway` and a `dead` group and refuses an
unknown status, and that the TEST-GROUP line for each carries its
reason.

D-R7-5 (PHD-05, medium: competitors can leak). Every helper mode reads
its stdin in a goroutine and exits the moment stdin reaches end of file;
the fixture holds the write end of each helper's stdin, so a test binary
that dies for any reason closes the pipes and every helper exits on its
own. The contention row's deferred cleanup stays as the ordinary path.
A test proves it: start a competitor, close its stdin, and observe it
exit without a signal.

D-R7-6 (PHD-07, note, folded): a group without a window ends its dump
wait at the first sample after SIGQUIT whose counter did not rise, then
kills; on Linux a /proc read that fails because the process is gone
(ENOENT or ESRCH) is a partial sample, never an outright failure.

PHD-06 (note): accepted as a note; the ratio bound stands and the
comment above the comparison already names the precondition.

## Mandate for this round

1. Implement D-R7-1 to D-R7-6 above. The production supervisor changes
   only for D-R7-2 and D-R7-6; every other change is in the fixtures and
   the two small tests of D-R7-4.
2. Keep every test of proof-standard in testing.json by its current
   name where it survives; if a name changes, update the group's list.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green (the race run is the cadence gate's form). The seat gate then runs
the same on the host with the real reader. Report the round as your own.

## Constraints

Wall-clock budget: 40 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
