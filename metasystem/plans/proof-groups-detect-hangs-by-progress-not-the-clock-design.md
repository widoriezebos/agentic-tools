# Design: a proof group is bounded by what it consumes, never by the host's clock

Goal: proof-groups-detect-hangs-by-progress-not-the-clock (tier 3,
DESIGN-BEARING). **Revision 3, 2026-09-10 22:20Z**, written on the m1 seat
(lineage main-1788940932-18533-7fa6c2). Revision 1 (landed 19775023a as
plans/proof-groups-progress-hang-design.md, which this file supersedes:
records carried under a goal are named after it) was critiqued by phd-design-crit1b-20260910 (codex gpt-5.6-sol); its
nine material findings are folded below by id. The critic could not
write its record (read-only sandbox); its findings live in the job's
return record and are carried into
records/misc/proof-groups-progress-hang-critique-r1-dispositions.md.

Wido, 2026-09-10 13:45Z: "we have a test that is load dependent. That's
not a test at all. Remove the entire timeout. We need to replace it with
something else, something that is not load dependent."


## Revision 3: the second critique folded as decisions, then the build

Revision 2 was critiqued by phd-design-crit2c-20260910 (nine material
findings, carried in records/misc/proof-groups-detect-hangs-by-progress-not-the-clock-critique-r2-dispositions.md).
Under the implementation-first ruling (two prose rounds spent), this
revision records the decisions and the build proceeds behind the
fixtures; what the fixtures cannot settle is named as residual.

| Finding | Decision |
| --- | --- |
| PHD-ZERO-SCHEDULER-STATE | The dead rule reads task state as well as consumption: a tree is dead only when, over the window, consumption is zero, output is absent, AND no member is stopped (Darwin state `T`, Linux `T`/`t`) or in uninterruptible wait (`U`/`D`). A stopped tree is reported `stopped` and left alone; a tree in uninterruptible wait is `waiting on the host` and left alone; each is a named verdict on the record, never a kill. Quota and container freezing do not exist on this fleet's hosts and are residual (d). |
| PHD-CPU-BUDGET-NONINVARIANT | The budget is a ceiling against runaway, not a measurement of the test: it is set at eight times the group's measured consumption, and the design assumes contention inflates consumption by less than that factor (residual (e), with the measured figures on every record so the assumption is checked by the fleet's own history). Proof row 4 asserts the same order of magnitude across quiet and loaded runs, not equality. |
| PHD-COUNTER-UNIT-RACE | Units: Linux ticks divided by `sysconf(_SC_CLK_TCK)`; Darwin `ps -o cputime=` parsed as [[dd-]hh:]mm:ss, one-second precision. "Consumption" over the window means the high-water counter rose by at least one second at some sample. A member that vanishes between the tree scan and its read is a partial sample: the counter keeps its high-water value and the sample counts as read. Only a reader that fails outright three samples in a row ends the group `invalid`; that rule counts failures, not time. |
| PHD-SEED-MULTICORE | No seed from wall time. `cpuBudgetSeconds` is optional; a group without it runs under the dead rule only and its record carries the measured consumption; a later contract change sets the budget from the fleet's measurements (eight times the largest observed). The seed question dissolves. |
| PHD-DUMP-NONTERMINATING | The supervisor drains the child's output continuously into the log (the tee), so a dumping process never blocks on a pipe. After SIGQUIT the supervisor waits for the process to exit; if the tree keeps consuming for one more sample after the signal, it has ignored the dump request and is killed at once with `dump: not produced, the process kept computing`; if it stops consuming, the zero-consumption rule applies as the only fallback. |
| PHD-FIXTURE-WAIT-SEMANTICS | Slice 3 leaves this goal: fixture waits are owned-producer contracts, one per wait, and are goal fixture-waits-name-their-producer. This goal's DONE sentence is narrowed to the receipt's groups and attempt (slices 1 and 2). |
| PHD-CLOCK-INVENTORY-GAPS | Inventory rows added, all slice 2: the two-minute trusted-policy-engine context, the section-cap contexts around candidate-engine construction, metadata preparation and retained-result verification, the sixty-second detached-evidence preservation bound (which can turn a group `invalid`), and the one-second attempt mutation-lock ceiling. Each becomes a progress or retry rule with no clock, or is shown to decide no proof outcome. |
| PHD-BUDGET-PROTECTION | `cpuBudgetSeconds` is part of the protected contract: for a group present in both base and candidate the effective value is the base's (a candidate may lower it in the same change only through the protected-policy path that already guards the contract); a new group takes the candidate's value. `testpolicy.ProtectedContract` is where this lives, with a test that a raised value in the candidate is ignored. |
| PHD-FOUR-LOOPS-NOT-STARVATION | Rows 1 and 4 measure their precondition: the fixture starts two busy loops per online processor, samples the test's own scheduling share (consumed CPU over wall time) and asserts it below one half before asserting the outcome; on a host where the share cannot be pushed down the row is skipped by name, not passed. |

## Build decisions after slice 1 round 1 (2026-09-11)

The implementer stopped at three gaps in revision 3; the designer decided
them in the round-2 follow-up brief and they bind from here:

| Gap | Decision |
| --- | --- |
| Linux clock ticks without cgo | AT_CLKTCK from /proc/self/auxv, read once per process; if unreadable, 100 (USER_HZ) and `ticks/assumed-100` on the progress rule. |
| Lifecycle of a stopped or host-waiting tree | Not terminal: a current reading on the record and the progress line; the supervisor waits without bound until the tree exits or the attempt's existing cancellation takes custody; the zero-consumption window restarts when the tree leaves that state. Only `dead`, `runaway` and `invalid` end a group. |
| Domain of `cpuBudgetSeconds` | Nullable integer, strictly positive when present, refused otherwise; absent means the dead rule only; zero is never a budget. |
| Darwin cputime shape (found by the seat gate of round 2) | Revision 3 said [[dd-]hh:]mm:ss at one-second precision; BSD ps prints `minutes:seconds.hundredths` with unbounded minutes and no hours field (`0:00.00`, `137:14.32`). The Darwin reader parses `[dd-][hh:]mm:ss[.ff]`; precision is hundredths; the "rose by at least one second" rule is unchanged. |
| The one-second rise at the test window (found by the seat gate of round 3) | "Rose by at least one second" is unreachable in a 400 ms test window and, under contention, would need a window of one second over the child's share, which is load dependent. The rise that counts is derived from the window: max(10 ms, window / 1800), exactly one second at the production window; the dump ladder's "still rises" uses the same threshold; the hook stays the pair (budget, window). |
| The contention precondition measured too early (found by the seat gate of round 4) | Twenty re-executed helpers were still starting when the row measured its share (0.77 on the seat), so the row skipped on every Mac and, as an expected test, would fail its group in the receipt. The competitors now hand-shake: a `busy-competitor` helper prints `spinning` the instant it enters its loop, the fixture reads that line from every competitor, then measures its own share over 500 ms; skip by name when the share is still at or above one half. |
| Two spinners per processor leave the share at one half on the fleet's Macs (found by the seat gate of round 5: 0.52; the seat measured 0.45 to 0.52 at two per processor, 0.24 at four, 0.10 at eight) | The contention rows start two competitors per online processor, measure, and double up to eight per processor until the share is below one half; skip by name only after eight per processor, naming the last share and the count. |
| Code critique round 1 (Opus, on round 6): seven fixtures raced a helper against a shortened wall window (PHD-01) | D-R7-1: every helper announces `ready` after its setup; the scripted reader reports rising CPU until the handshake is read; fixtures wait on reader calls or the process, never on a sleep. |
| The counter was the high-water of a sum, and Darwin `ps -S` does not fold reaped children (PHD-02; the seat verified it: a shell whose reaped child burned seconds shows 0:00.00 with and without -S) | D-R7-2: per-member high-water, summed over every member ever observed and retained after it vanishes; Linux keeps cutime/cstime. Residual (f): on Darwin a child shorter than one sample interval is invisible to the counter; the parent's own consumption and any output keep the group alive. No libproc: the engine is cgo-free. |
| Rows 3, 5, 6 and 14 never touched the real reader (PHD-03) | D-R7-3: rows 5, 6 and 14 use the real reader with hand-shaken Setsid helpers; row 3 asserts the goroutine dump in the log. |
| Row 12's test passed on the old code (PHD-04) | D-R7-4: tests that ValidateTestResult accepts runaway and dead and refuses unknown, and that the TEST-GROUP line carries the reason. |
| Competitor helpers could leak and burn every processor (PHD-05) | D-R7-5: every helper exits when its stdin reaches end of file; the fixture holds the write end. |
| A windowless group waits forever after SIGQUIT; a Linux ESRCH read counts as an outright failure (PHD-07) | D-R7-6: the dump wait of a windowless group ends at the first sample with no rise; ENOENT and ESRCH are partial samples. |
| The seat gate on round 7 (plain and under -race) ended three real-reader rows `dead`: a real sample's latency under load is unbounded, so a shortened wall window in a real-reader fixture is load dependent by construction | D-R8-1: real-reader rows run with no window and no budget unless the row proves the budget; the zero-consumption rule, the stopped and host-waiting readings, the dump ladder and reader failures are proven with the scripted reader; every real-reader row ends by the process's own exit or by the budget. |
| The section row counted `sample.Members`, which the Darwin reader stopped filling when round 7 keyed members by pid and start time | D-R8-2: every sample carries `Members` on both platforms, derived from the scan that fills `MemberCPU`. |
| The section row read its last gated sample, taken while the helper was exiting, instead of its first (round 8 gate; the round-9 brief blamed the nested handshake, which the implementer showed already composes, and the seat reproduced the real cause in a scratch copy) | D-R9-1: a fixture callback that runs on every gated sample records the largest member count it has seen, never the last; readiness composes as built. |
| Code critique round 2 (Opus, on round 10): four wall margins a starved scheduler exposes (PHD-08 dump duration, PHD-09 pre-readiness synthetic CPU in the budget, PHD-10 SIGCONT before the self-stop, PHD-11 a nested child exiting before a real sample) | D-R11-1 (restated in round 12 after the implementer showed the fixture cannot see the verdict): `supervisorOptions` carries an unexported nil-by-default `OnVerdict` callback invoked once when a terminal verdict is recorded, before SIGQUIT; a fixture input on the options struct like Reader, not a second package-level hook; the dead-dump row uses it to mark output until the helper exits. D-R11-2: pre-readiness keep-alive marks output, never CPU. D-R11-3: the stopped row reads the real task state and resumes only a stopped helper, repeating on every sample. D-R11-4: nested children hold their work until the row releases them by closing their stdin. |
| Linux counted a reaped child twice under D-R7-2 read literally (PHD-13) | D-R11-5: retention of vanished members is a Darwin rule only; Linux counts live members plus the root's cutime and cstime. |
| A real-reader row hangs when the reader has the defect it guards against, and slice 2 removes the attempt deadline (PHD-14) | D-R11-6: every real-reader row carries a CPU budget of ten times its expected consumption, so a reader defect ends the row `runaway` by name; consumption, never a clock. |
| Code critique round 3 (Opus, on round 12): the host-wait row decided by a sample count (PHD-15); the Linux counter of D-R11-5 decreases when a non-root member reaps its own child (PHD-16); Darwin retention untested (PHD-17); a backwards output mark (PHD-18) | D-R13-1: the fixture seam gains `OnReading`, and rows wait for the reading, never for a count. D-R13-2 (replaces D-R11-5): every live Linux member counts its own utime, stime, cutime and cstime, no retention; Darwin keeps retention. D-R13-3: a scripted row fails without Darwin retention. D-R13-4: stage-result growth keeps the later output time. |
| The landing receipt refused slice 1 twice on the same schema skew: first the enrolled engine's protected probe read the candidate worker's record strictly (fixed as a mechanical step, 673f62f51: the probe reads a newer candidate's result); then the land fixture's receipt-cutover scenario, whose pinned pre-cutover engine (6bc19ba1c) built the NEW engine as its own candidate through a path baked into the seed's go-build stub | D-R15-1 (restated in round 16 after the implementer showed the stub runs in a materialized worktree that is removed before its wrapper runs): in the land fixtures a checkout's candidate engine is the engine installed in that checkout; delivered through PATH, the one channel both engines keep (round 16 showed the control-root variable reaches only the proof-suite child): every engine invocation the fixture makes for a checkout that mints, lands or re-verifies a receipt runs with that checkout's bin directory first on PATH (round 17 showed a producer-only PATH changes the proof's environment identity), and the seed's go-build stub copies `$(command -v metasystem)` into the requested output, failing loudly when none is on PATH, so the old engine's receipt is an exact old-engine receipt and the pin stays. Rule for the fleet: a candidate that adds a record field lands only after the tolerant reader is enrolled everywhere. |

## What changed from revision 1

| Finding | Fold |
| --- | --- |
| PHD-PROGRESS-NOT-HANG | The "no output and no CPU" rule is withdrawn. Two rules replace it, each with a stated ground: a CPU budget catches runaway compute; only exactly-zero consumption over a long window counts as a deadlock, and a legitimate wait shorter than that window is never touched. Decision 1. |
| PHD-DEADLINE-REMAINS | The attempt deadline is enforced in five places, all named; slice 2 removes every one of them as a kill or refusal and keeps the deadline as a reservation figure. Decision 3. |
| PHD-REMAINING-CLOCKS | The inventory now lists every wall-clock bound found by search (two shell `go test -timeout` calls, the thirty-minute discovery context, twenty-one scripts with `SECONDS` deadlines, the harness caps, the census window) and assigns each to a slice. No slice leaves a clock "for later" without saying which clock and which slice. Inventory. |
| PHD-PROCESS-GROUP-SELECTOR, PHD-CPU-SNAPSHOT | The counter is defined per platform and monotonic: the process tree by parent links, self plus reaped-children CPU, from `/proc/<pid>/stat` on Linux and `ps -S -o cputime=` on Darwin; short-lived children are folded into their parent when reaped; counter errors are mapped to outcomes. Decision 2. |
| PHD-SECTION-ESCAPE | The supervised set is the descendant tree, not the process group, so `Setsid` children stay observed; section progress is the section's own stage log, not the worker journal. Decision 2. |
| PHD-FIFTEEN-MINUTE-BASIS | No window governs the runaway rule (a budget does). The deadlock window is grounded in what it must not misjudge (a legitimate wait) rather than in a fleet measurement it cannot have; the supervisor records the longest silent and zero-consumption intervals of every group so the fleet gains that measurement from the first landing on. Decision 1. |
| PHD-DUMP-GRACE | The dump step waits for the dumped process to exit, observed, with the same zero-consumption rule as its only fallback; no fixed grace. Decision 2. |
| PHD-PROOF-MATRIX | The proof table is rewritten around the decisive paths the critic named: legitimate I/O wait, a runaway loop, a starved host, exited and between-sample children, Linux and Darwin selectors, sampler failure, dump completion, every deadline site, and a test-only override with a named interface and an assertion on the production default. Decision 4. |

## Inventory: every wall-clock bound that decides a proof outcome

| # | Bound | Where | Slice |
| --- | --- | --- | --- |
| 1 | Go's default ten-minute `-timeout` | every go group; `internal/proofrun/test_go.go` builds no `-timeout` | 1 |
| 2 | Thirty-minute discovery context | `internal/proofrun/test_go.go`, `context.WithTimeout(..., 30*time.Minute)` around `goArgumentsCached` | 1 |
| 3 | The worker context of the attempt | `cmd/metasystem/test.go`, `context.WithDeadline(..., attempt deadline)` passed to every `exec.CommandContext` | 2 |
| 4 | The attempt's child-admission, authentication and finalization checks | `internal/proofrun/attempt.go`: child launch refused at or after the deadline, "governed reservation owner has no live deadline", "proof attempt deadline expired before success", "proof parent context has reached its absolute deadline" | 2 |
| 5 | The watchdog's "absolute proof deadline expired" ladder | `internal/proofrun/watchdog.go` | 2 |
| 6 | `go test -race -cover -timeout 60m` | `scripts/agents/go-gate.sh` (the cadence gate) | 3 |
| 7 | `go test -cover -timeout 30m` | `scripts/agents/coverage-delta.sh` | 3 |
| 8 | Harness caps in seconds | `scripts/agents/fixture-budget.sh` and the `SECONDS` deadlines in twenty-one scripts under `scripts/agents` (dispatch-fixtures.sh alone has forty-six) | 3 |
| 9 | Two-second census freshness window | `internal/dispatch/attest.go`; a freshness rule of admission, not a test bound | goal supervision-fixture-census-window-fails-under-load |

Slices 1 and 2 make every receipt free of the clock. Slice 3 makes the
cadence gate and the fixture beds free of it through the same supervisor,
exposed as one engine verb the scripts call.

## Decision 1: two rules, each with its ground

**Rule A, runaway: a group may consume at most its CPU budget.** The
budget is `cpuBudgetSeconds` on the group in `metasystem/testing.json`,
a required field for every group, set from the group's measured CPU
consumption on the fleet times four (the contract change of slice 1
seeds it from the retained receipts on this Mac, which record wall time
today and CPU time from the first landing on; until a group has a
measurement its seed is its `targetMs` in seconds times four, and the
supervisor writes the measured figure onto the record so the next
contract change can replace the seed). A group that reaches its budget
is runaway: it is stopped with a dump (Decision 2) and fails with the
reason `cpu budget exhausted: <consumed>s of <budget>s`. Ground: CPU
seconds are consumed by the workload, not by the host; the same work
consumes the same CPU on a quiet or a loaded host, so the rule cannot
be reached by load. This is the replacement for the timeout Wido
removed: it bounds the same failure (a test that never finishes because
it keeps computing) by the test's own consumption.

**Rule B, deadlock: a group whose whole tree consumed exactly zero CPU
and wrote nothing for `zeroConsumptionWindow` is dead.** The window is
thirty minutes, one constant in `internal/proofrun`. Ground: a
runnable process on a loaded host is slowed, not stopped; over thirty
minutes it is scheduled and consumes some CPU, so exactly zero over the
window means every process in the tree is blocked forever or asleep.
The rule misjudges only a legitimate wait longer than thirty minutes
with no activity at all in the tree; no test on this fleet waits like
that, and one that did would be reporting nothing for half an hour,
which is itself a defect to fix in the test. The critic's cases are
answered directly: a test blocked on a socket or a disk for seconds or
minutes consumes nothing and is left alone, because the window is
thirty minutes, not fifteen and not one sample; a test doing useful work
in another process that is still in its descendant tree is counted
(Decision 2); a runaway loop is caught by rule A, not by rule B.

Neither rule reads `targetMs`. `targetMs` stays a declared target for
the cost record and for choosing plans; it never bounds anything.

## Decision 2: the supervisor

One supervisor in `internal/proofrun` runs every group child, replacing
`command.Run()` with the buffered output in `test_build.go`:

1. **Launch** with `Setpgid` as today. The supervised set is the child's
   descendant tree: every live process whose parent chain reaches the
   child (`/proc/<pid>/stat` field 4 on Linux; `ps -axo pid=,ppid=` on
   Darwin), rebuilt at every sample. A `Setsid` grandchild is still a
   descendant, so the section launchers' sessions are covered. A process
   reparented away by its parent's exit is lost to the set; that loss is
   residual (b) and is bounded by the census, which refuses to leave
   unowned processes behind at the end of a run.
2. **Counter.** CPU of the set is the sum over its live members of
   self CPU plus reaped-children CPU: on Linux fields 14 to 17 of
   `/proc/<pid>/stat` (utime, stime, cutime, cstime), on Darwin `ps -S
   -o cputime= -p <pids>` (the `-S` flag folds reaped children in). A
   child that exits between samples is folded into its parent's
   children figure when reaped, so the counter does not lose it; a
   counter that decreases (a member vanished before reaping) keeps its
   last high-water value; a sample that fails to read the tree does not
   move the counter and, after three consecutive failures, ends the
   group as `invalid` with the reader's error, never as hung. Samples
   every ten seconds.
3. **Output.** The child's stdout and stderr flow through a tee to the
   same buffer and log the parsers read today, byte for byte, and every
   write marks activity. For a section the stage results file it writes
   is watched the same way (its size, sampled), because in worker mode
   the section's output goes to its own stage log and the shared
   progress journal receives only section boundaries.
4. **Verdict per sample:** consumed CPU at or above the budget: runaway
   (rule A). Otherwise, if the counter and the output are both unchanged
   for the window: dead (rule B). Otherwise keep waiting, without bound.
5. **Stop ladder, observed.** `SIGQUIT` to the tree (a Go test binary
   dumps every goroutine to stderr and exits with status 2); the
   supervisor then waits for the process to exit, which it observes;
   the only fallback is rule B applied to the dumping process: if it
   consumes exactly nothing for the window after the signal, `SIGKILL`.
   No fixed grace, so the dump under load is waited for, not raced. The
   record says whether the dump completed (`dump: complete` or `dump:
   killed while writing`).
6. **Record.** Every group record carries `progressRule:
   "cpu-budget/<n>s+zero-window/30m"`, `cpuSeconds` consumed,
   `longestSilentSeconds` (no output) and `longestZeroCpuSeconds`, so the
   fleet learns its real intervals from the first landing on.
7. **Discovery** (`go list`, `go test -list`) runs under the same
   supervisor with a budget of sixty CPU seconds and no window
   (discovery does not wait on anything).

The supervisor is also exposed as `metasystem proc supervise --cpu-budget
<seconds> -- <command>`, printing the group record's fields on exit, so
slice 3 can give the shell gate and the fixture beds the same two rules
in place of `-timeout` and `SECONDS`.

## Decision 3: the attempt deadline is a reservation figure

Today five sites end or refuse an attempt by the clock (inventory rows 3
to 5). After slice 2: the worker context carries no deadline (a
`context.WithCancel` that the cancel verb and the supervisor's verdicts
cancel); child admission, authentication and finalization no longer
compare `now` with the deadline; the watchdog ends a suite only when the
supervisor has judged its current group runaway or dead, or when a
human's cancellation intent is recorded. The `deadline` field stays on
the attempt record and keeps its one remaining meaning: the proof
reservation minutes dispatch admission charges while the attempt is
live, which a633b7a08 already settles to observed minutes when the
attempt ends. An attempt that outlives its reservation is not killed; it
is charged what it used.

## Decision 4: proof

Every row names the path it exercises. The window and the budget are
overridden in tests only through one unexported hook,
`supervisorLimitsForTest`, whose zero value is the production pair; a
test asserts that pair (thirty minutes, and the budget from the group).

| # | Obligation | Fixture | Home |
| --- | --- | --- | --- |
| 1 | A test that computes steadily on a host starved by four CPU-bound competitors passes under a CPU budget it does not reach, however long it takes | Go test: the fixture launches four busy loops, then the group; assert `passed` and `cpuSeconds` below budget | internal/proofrun |
| 2 | A test that waits on a listening socket for 90 s with zero consumption and no output passes under a window of 120 s set by the hook: a legitimate wait shorter than the window is never touched, and the window is not one sample | Go test | internal/proofrun |
| 3 | A deadlock (two goroutines waiting on each other, no output) with window 20 s is `dead` within one window, the goroutine dump is in the log and `dump: complete` is recorded | Go test | internal/proofrun |
| 4 | A busy loop that never finishes is `runaway` when it reaches a budget of 5 CPU seconds, on a quiet host and on a starved host alike, and the wall time differs while the CPU figure does not | Go test with the four competitors of row 1 | internal/proofrun |
| 5 | A child that forks a grandchild with `Setsid` which does the work: the counter grows while the direct child is idle; the group is not dead | Go test with a helper binary | internal/proofrun |
| 6 | A child that spawns short-lived children between samples: their CPU is present in the parent's reaped figure and the counter never decreases | Go test | internal/proofrun |
| 7 | Linux and Darwin tree and counter readers agree on a synthetic tree; the Linux reader runs on the VM seat's gate | Go test with build tags; the Linux half is a cadence row on a Linux seat | internal/proofrun |
| 8 | Three consecutive sampler failures end the group `invalid` with the reader's error, never `dead` | Go test with an injected reader | internal/proofrun |
| 9 | The argv carries `-timeout 0` for every go group; no `-timeout` literal other than `0` in internal/proofrun; `go-gate.sh` and `coverage-delta.sh` carry none after slice 3 | Go test and a grep guard over `scripts/agents` | internal/proofrun |
| 10 | Every attempt-deadline site is gone after slice 2: a fixture attempt with a deadline in the past launches a child, authenticates, and finalizes successfully | Go test on attempt.go and test.go paths | internal/proofrun |
| 11 | The watchdog ends a suite for a `dead` or `runaway` verdict and for a cancellation intent, and for nothing else | Go test | internal/proofrun |
| 12 | A `dead` or `runaway` group is a failed delivery with its reason on the TEST-GROUP line | Go test on the delivery verdict | internal/proofrun |
| 13 | `cpuBudgetSeconds` is required on every group and the contract validator refuses a group without it | Go test in internal/testpolicy | internal/testpolicy |
| 14 | The section adapter counts the section's stage-results growth as output and its descendant tree as consumption | Section fixture with a fake section that only writes stage results from a `Setsid` child | internal/proofrun |
| 15 | `proc supervise` gives a shell command the same two rules and the same record | Shell fixture leg | scripts/agents |

Residual risks, named: (a) a legitimate wait with no activity in the
whole tree for more than thirty minutes is judged dead; the record
names the interval so the constant can be raised in code with a critic
if a real case appears; (b) a descendant reparented away by its parent's
exit leaves the observed set, so its work is invisible to both rules and
it can outlive the group until the census sweeps it; (c) CPU budgets
seeded from `targetMs` are guesses until the first measured landing
replaces them; a seed too small fails a correct group once, with the
consumed figure on the record, and the fix is one contract line.

## Slices

1. The supervisor for go and section groups, `-timeout 0`, the
   `cpuBudgetSeconds` field and its seeds, the discovery budget, rows
   1 to 9 and 12 to 14. Unblocks every receipt on this Mac.
2. The attempt deadline as a reservation figure only (Decision 3), rows
   10 and 11.
3. `proc supervise` for the shell gate, the coverage run and the
   fixture beds, row 15; `SECONDS` deadlines and harness caps become
   calls to it.

## Self-grade

Grounding: `test_build.go` (group run), `test_go.go` (argv, discovery
context), `test.go` (worker context), `attempt.go` (four deadline
sites), `watchdog.go`, `go-gate.sh` line 630, `coverage-delta.sh` line
245, the `SECONDS` search over `scripts/agents`, `validate-metasystem.sh`
worker mode, the Darwin `ps` manual (`-S`, `-g`), the Linux `/proc/<pid>/stat`
fields, and the critic's nine findings. Nothing was executed against a
live receipt. Grade: pass against everything read, with the seeds of
the CPU budgets named as the one guess the design makes on purpose.

**Reject condition. Reject this design if any of the following is
shown:** a group or attempt ended or refused by wall-clock time after
slice 2; a group with any consumption or any output over the window
judged dead; a runaway loop under its budget not caught when it reaches
it; a budget or window readable from configuration other than the
group's `cpuBudgetSeconds`, or derived from `targetMs` anywhere but the
seed; a counter that can decrease and cause a verdict; a `dead` or
`runaway` group counted as passed; a dump raced against a fixed grace;
the record silent about the rule, the consumption or the intervals.
