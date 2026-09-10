# Design: a proof group is hung when it makes no progress, never because a clock ran out

Goal: proof-groups-detect-hangs-by-progress-not-the-clock (tier 3,
DESIGN-BEARING). **Revision 1, 2026-09-10**, written on the m1 seat
(lineage main-1788940932-18533-7fa6c2) against main at 99f2a9633.

Wido, 2026-09-10 13:45Z: "we have a test that is load dependent. That's
not a test at all. Remove the entire timeout. We need to replace it with
something else, something that is not load dependent."

## The defect, against the code

Four wall-clock bounds decide proof outcomes today, and every one of them
turns host load into a refusal:

1. `internal/proofrun/test_go.go` builds `go test -json -count=1 ...`
   with no `-timeout`, so Go's own ten-minute default kills the test
   binary. On 2026-09-10 `goal-full-coverage` (internal/goal) ended with
   `panic: test timed out after 10m0s` at 600.466 s and again at 600.258 s
   with every test that ran passing; the same package took 596 s in a
   receipt that passed and 523 s in a quiet seat gate. The m1 host was
   shared by four seats at load 6 to 12.
2. `internal/proofrun/test_go.go` wraps native test discovery in a
   thirty-minute `context.WithTimeout`.
3. `internal/proofrun/watchdog.go` ends the whole attempt when the
   absolute proof deadline expires ("absolute proof deadline expired",
   then the CONT, TERM, KILL ladder in `SignalSuiteGroup`); the deadline
   is `startedAt + 2h` on every record this seat has.
4. `scripts/agents/fixture-budget.sh` owns the harness caps of the shell
   sections ("every harness cap is at least 10x the expected duration it
   guards"), which are seconds of wall-clock time.

A fifth bound, the census freshness window of dispatch admission
(`internal/dispatch/attest.go`, "census verdict is stale (age=2s
window=2s)"), refused the supervision fixture's stop-everything scenario
under the same load; it is a freshness rule rather than a test bound and
is goal supervision-fixture-census-window-fails-under-load.

Each bound measures the host, not the candidate. A candidate that is
correct fails when the host is busy and passes when it is quiet, which is
the definition of not a test.

## Decision 1: what "hung" means

**A group is hung when, over one long window, its child process group
produced no progress event and consumed no CPU time.** Both must hold.

- A progress event is a byte of output on the adapter's stream: for a go
  group a `-json` event line (`run`, `pass`, `fail`, `skip`, `output`,
  `bench`); for a section adapter a line appended to its stage results or
  its log (the suite progress log the sections already write under
  `METASYSTEM_SUITE_PROGRESS_LOG`).
- CPU time is the sum of user and system time of every live process in
  the child's process group, read with `ps -o cputime= -g <pgid>` on both
  Darwin and Linux (or the equivalent `/proc` read on Linux), sampled every
  30 seconds. A child that is starved by load still accrues CPU time,
  slowly; a child that is deadlocked, blocked on a lock it will never get,
  or asleep forever accrues none.
- The window is one constant, `noProgressWindow = 15 * time.Minute`, in
  `internal/proofrun`. It is not a configuration key and it is not derived
  from `targetMs`: a key or a derivation would make the bound tunable back
  into a clock (the round-two finding CLN-R2-TIMEOUT-CAP on another design
  showed what a tunable bound does). Fifteen minutes is far above the
  longest legitimate gap between two events observed on this fleet (the
  goal package's slowest single test takes about 40 s) and far below the
  two-hour attempt deadline it replaces as the hang rule.
- CPU progress counts only when it exceeds one second over the window, so
  a process that merely wakes to poll a timer does not count as working.

Under this rule a slow group under load is never hung: it emits events
and burns CPU. A truly hung group is caught within one window whatever
the load, because absence of CPU time is not a property of load.

## Decision 2: what the adapter does about it

**`-timeout 0` on every go group, a streaming supervisor for every group,
and one kill ladder with a stated reason.**

In `internal/proofrun/test_build.go` the group child is run today with
`command.Run()` and a buffered `bytes.Buffer` for stdout and stderr. The
supervisor replaces that:

1. Argument change (`test_go.go` `goArgumentsCached`): `-timeout 0`
   follows `-count=1`. Go then never kills the binary. The parked chain
   gotimeout-build1-20260910 put `-timeout <targetMs>` there; this
   design puts zero.
2. The child starts with `Setpgid` (its own process group, as the suite
   launcher already does) and its combined output goes through a tee: the
   bytes reach the same log file and buffer as today, and every write
   moves `lastProgress` to now.
3. A sampler goroutine, every 30 seconds, reads the process group's CPU
   time; when it grew by more than one second since the last sample,
   `lastProgress` moves to now.
4. When `now - lastProgress > noProgressWindow`, the group is hung:
   `SIGQUIT` to the process group (a Go test binary dumps every goroutine
   to stderr, which the tee captures into the log), ten seconds, then
   `SIGKILL` to the group, then `Wait`. The group's status is `hung` (a
   new value beside `passed`, `failed`, `invalid`, `unavailable`), its
   `notRunReason` is `no progress for 15m0s: last event <age> ago, cpu
   +<seconds>s in the window`, and its record carries `progressRule:
   "events-or-cpu/15m"` so a reader knows which rule judged it.
5. The discovery step keeps a context, but its bound becomes the same
   progress rule applied to `go list`/`go test -list` output, not thirty
   minutes of wall clock.
6. `targetMs` keeps its meaning as a declared target for reporting
   (`declaredTargetMs` in the cost record); nothing reads it as a bound.

The section adapter gets the same supervisor unchanged: its progress
events are the lines the section writes to its log and stage results.
The harness caps inside `fixture-budget.sh` are slice 3 (below) and stay
as they are in slice 1, so a section that a cap kills still fails as
today; the supervisor only adds the hang verdict for a section that stops
writing and stops computing.

## Decision 3: the attempt deadline

**The attempt's absolute deadline stops being a kill rule and stays a
reservation horizon.** Today `watchdog.go` ends the suite when
`options.Deadline` passes. After slice 2 the watchdog ends the suite when
the suite's current group is hung by Decision 1 (the supervisor reports
it through the progress file the watchdog already reads), and the
deadline only bounds what the goal's budget reserves (the proof
reservation minutes of dispatch admission). A receipt that runs longer
than its reservation is not killed; it is accounted at the minutes it
ran when it ends, which is what a633b7a08 already does for jobs.

## Decision 4: proof

| # | Obligation | Fixture | Home |
| --- | --- | --- | --- |
| 1 | A go group under a starved host passes: a fixture package whose one test sleeps on a timer but does CPU work in a loop for 25 s while the supervisor's window is shortened to 10 s by an unexported test hook passes with `progressRule` on its record | Go test in internal/proofrun with the window hook | internal/proofrun |
| 2 | A deadlocked go test (two goroutines waiting on each other, no output) is failed as `hung` within one window, with the goroutine dump in the log and the reason naming the window and the cpu delta | Go test with the window hook at 5 s | internal/proofrun |
| 3 | A go test that emits no output but burns CPU for longer than the window is not hung | Go test with the window hook | internal/proofrun |
| 4 | A go test that emits an event every few seconds but burns no CPU (a sleep loop that prints) is not hung | Go test with the window hook | internal/proofrun |
| 5 | The argv carries `-timeout 0` and no other timeout for every go group; a grep guard fails on any `-timeout` literal other than `0` in internal/proofrun | Go test | internal/proofrun |
| 6 | A section whose script stops writing and stops computing is `hung`; one that writes slowly is not | Section fixture leg with a fake section script | internal/proofrun |
| 7 | The record's `status` of `hung` counts as not passed for delivery and names its reason in the TEST-GROUP line | Go test on the delivery verdict | internal/proofrun |
| 8 | The watchdog no longer ends a suite for the deadline alone (slice 2) | Go test on the watchdog with a past deadline and a progressing suite | internal/proofrun |
| 9 | `goal-full-coverage` passes in a receipt on a host at load above 6 | Observed on m1 at the first landing after slice 1, recorded in the landing message | m1 seat |

Residual risks, named: (a) a test in a genuine infinite busy loop with no
output is never hung under this rule and runs until a human cancels the
attempt; that is the price of removing the clock, and the `run cancel`
and `landing` verbs already end an attempt by hand; (b) CPU accounting of
a process group misses work done by processes that leave the group
(setsid children), which the suite launcher forbids and the census
already refuses; (c) the fifteen-minute window is a judgement, stated
once as a constant; a fleet with a single test slower than that would
raise it in code, with a critic, not in a config key.

## Slices

1. Go adapter and section supervisor (Decisions 1 and 2), rows 1 to 7
   and 9. This is what unblocks every deep landing on m1.
2. The watchdog and the attempt deadline (Decision 3), row 8.
3. `fixture-budget.sh` harness caps become progress bounds through the
   same rule, so a section is never killed by seconds.

## Self-grade

Grounding: `test_build.go` (the group run, lines around the native launch),
`test_go.go` (argv, discovery context), `watchdog.go` (deadline ladder),
`attempt.go` (deadline record), `fixture-budget.sh` header, `attest.go`
(census window), and the two failed receipts of 2026-09-10 read in full.
The CPU-time sampling of a process group by `ps -o cputime= -g` was
checked by hand on this Mac. Nothing was executed against a live
receipt. Grade: pass against everything read.

**Reject condition. Reject this design if any of the following is
shown:** any group or attempt still ended by wall-clock time alone after
slice 2; a slow group with events or CPU judged hung; a deadlocked group
not judged hung within one window; the window readable from
configuration or derived from `targetMs`; a `hung` group counted as
passed or as anything but a failed delivery; the goroutine dump absent
from the log of a hung go group; the record silent about the rule that
judged the group.
