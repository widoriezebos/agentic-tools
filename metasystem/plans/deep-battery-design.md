# The deep battery under ten minutes

Owner: the proof runner (`internal/proofrun`) and its launcher knobs in
`cmd/metasystem/proof_run.go`. Goal: deep-battery-under-ten-minutes, slices
2, 3 and 5 of `plans/suite-speed-plan.md`. Ruling R-16 stands: runs continue
and collect; nothing stops early.

## Slice 2: groups run side by side inside one attempt

Every group already runs in its own detached worktree of the candidate tree
with its own logs, so the groups of one stage are independent. `RunTestPlan`
now runs each stage's groups under a bounded pool and appends their results
in plan order; the stage order stays canary, standard, deep. The progress
file and the `TEST-GROUP` lines on stdout are written through one mutex, so
the launcher's silence watch and the structure check see the same events as
before, interleaved. A group that fails still preserves its evidence while
the others run. A progress-record failure stops new launches; the groups
already running finish and report, and the failure is returned after the
stage drains. The pool lives inside the single worker process: no scheduler,
no daemon, no second proof store.

Launch order inside a stage is longest first: each group's last measured
duration in the retained attempts, with the contract's `targetMs` as the
floor when nothing was measured. The first run with the pool showed why:
every section declares 120 s, so the 574 s dispatcher section started in
the middle of the queue and the stage's wall time was its start plus its
length. With the measured order the long groups start first and the short
ones fill the remaining slots.

The cap is `testing.concurrency` in `metasystem.conf`, 1 to 64. Absent, it
is a third of the cores, at most six, at least one: six on the 18-core Mac,
one on the 4-vCPU VM. The first pooled cadence runs (cap six) inflated
every heavy group 1.5x to 1.7x because two serial giants and the whole race
gate ran as single processes, and a quarter (four) was the afternoon's
measured answer; once the giants ran in shards and every bed's scenarios
side by side, cadence run 8 (969 s) had its four slots full with 8.9 of 18
cores busy on average and the groups summing to 3796 s, so the slot count
was the wall; run 9 then measured six slots at 896 s with every heavy group
30 to 60 percent slower (the walls summed to 5110 s) and one more fixed
fixture cap tripped, so a quarter stays the default until the duplicated
coverage work at cadence is gone. Wido decides the VM's committed value.

Expected on the Mac: the cadence battery drops from about 56 minutes to the
length of its longest group, the dispatcher section at about 9.5 minutes.

## Slice 3a: the long Go groups run in shards inside their group

A whole-package `go` group may declare `shards` (1 to 64, `tests` must be
`all`). The runner partitions the discovered test names round-robin and
runs that many `go test -json` launches at once inside the one group, each
supervised like a single launch, each with its own log beside the group's;
the group log is their concatenation in shard order, which is what the
group's parsers read. The outcome is the merged supervision: any verdict,
the first wait error, the summed CPU, the longest silences, the first
nonzero exit. With `coverage` on, every shard writes coverage data under
the group's log directory (`-test.gocoverdir`), and `go tool covdata
percent` merges them; the merged per-package percentages join the group's
JSON stream as output events in go test's own summary shape, so the
coverage floors judge the whole package, never a shard. The group's
identity, evidence, progress events and reuse are unchanged: one group.

`goal-full-coverage` and `missionrunner-full-coverage` declare four shards
(six for one evening on 2026-09-11: six shards each plus the gate's launches
oversubscribed the 18 cores; the 78.3 percent that the internal/goal merge
read against its 80 percent floor in cadence run 8 was first blamed on the
six shards, but run 9 read the identical 78.3 with four: the shortfall is
ce59b313's untested additions to the package, another seat's landing, and
the shard count was never its cause).
Measured before: 538 s and 444 s alone, 919 s and 711 s under the pool.

On 2026-09-12 both groups left the cadence list (R-99-m1e, Wido's word):
the race gate's sharded coverage run measures the same packages against
the same floors in scripts/agents/coverage-ratchet.json, so cadence paid
about 2000 CPU seconds twice for one proof. The groups stay defined for
diagnostic runs (`--groups goal-full-coverage`), and the contract tests
now pin their absence from every selection list. Run 10 (proof-mtxie5w1)
was the last with them: 579 s and 489 s beside a 680 s gate, wall 1039 s.
The two floors that moved to their loaded reading the same morning
(R-100-m1e: internal/supervise 89.0, internal/validate 63.6) landed as
their own human commit: the delivery machinery refuses a candidate whose
ratchet floors sit below the policy base's (TEST_POLICY_COVERAGE_FLOOR_LOWERED),
by design, so a floor moves only on a recorded human word outside a receipt.

Found on the way: the coverage parser's JSON loop retried a decode error
forever; it now ends at the first error and keeps what it read.

## Correction of 2026-09-12: the night's numbers were measured on eight cores

Every cadence run and diagnostic from run 5 (2026-09-11 22:xx) to run 11
(2026-09-12 08:21) ran beside orphaned `yes` processes at a full core each,
one per execution of TestProcessTreeCPUSecondsCountsDescendants (landed
89711b7d): the test's Process.Kill ended the shell before the shell's own
`kill $!`, and each `yes` was reparented to launchd. Ten of them were found
at 08:40 by `ps` sorted on CPU and killed; the test owns a process group
since e773669f and asserts the group is empty afterwards. The walls 1177,
1189, 797, 957, 887, 1039 and 946 s, the six-slot comparison (run 9) and
the shard-count note above were all measured with up to ten of the
eighteen cores taken. They stand as recorded, on that box; run 12 onward
measures the real one.

## Slices 3b and 5: the big sections and the race gate

Landed 2026-09-11 evening. Slice 5 (b13771d7): the race gate runs its two
giants as METASYSTEM_GATE_SHARDS race+cover launches beside one run over
the rest, coverage merged by covdata; since the follow-up landing the cmd
package's tests run beside the unit shards instead of after them, and the
default is six shards. Slice 3b (bb1e2dfd): every bed runs its scenarios
through the shared runner in fixture-bed-scenarios.sh (the dispatch,
supervision, goal-cli and health beds had private serial loops; the runner
gained prepare, capability-mint, verdict and cleanup hooks for their
private needs), and the adoption bed is eight scenarios, each snapshotting
the source itself. Measured (attempt proof-mtxc3kvu): adoption 929 to
448 s, dispatcher 713 to 585 s, supervision 422 to 389 s, goal-cli 126 to
87 s.

Correction (2026-09-12, from the first critique round, F-3): bb1e2dfd's
commit message claims the whole command package green under the gate's
environment; that run was started before the bed edits the landing carried
and verified an earlier tree, and the dispatcher-bed literal test it missed
was red on main for about fifty minutes until 9a10d20e. A verification is
of the tree that lands. Dispositions of the round: dispositions/deep-battery-code-critique-r1.md.

What remains of the walls is the dispatcher bed's `dispatch` scenario: one
bed, 81 legs, 43 dispatched jobs, 574 s under the pooled battery. A phase
trace (attempt proof-mtxcowtr, not landed) put a fake dispatch at 3.0 to
3.8 s alone and 5 to 6 s under contention, spread evenly over its phases
(preparation, snapshot, record build and setup, launch, fence, the job's
run, reap): the cost is the dispatcher's own shape, every sub-step a fresh
run of dispatch.sh under the lease with dozens of engine calls, not a wait.
The scenario cannot be cut at any leg: six jobs are referenced across most
of it (`happy` from line 1517 to 3468, `review-target` 2008 to 3304,
`default-role` 1986 to 2890, `malformed-return` 2236 to 2878,
`process-loss` 2328 to 2887, `flag-runtime` 2027 to 2499). The next cut is
therefore clusters that each build their own bed and re-dispatch the shared
jobs they reference (a few five-second dispatches per cluster against a
574-second scenario), with the fixture's own assertions on those jobs kept
where they are; or a cheaper dispatch.sh, which is a goal of its own.

The leg map (2026-09-11, lines of dispatch-fixtures.sh) gives five
clusters with few crossings: A, lines 1517 to 2060 (happy through
investigator-role), self-contained; B, 2081 to 2450 (pending-chain through
mirror-retry), where only mirror-retry's second reap reads flag-runtime and
review-target from A; C, 2454 to 2590 (the cap chains and repeat-follow),
which need flag-runtime and review-target from A; D, 2589 to 2830 (the
worktree chains), self-contained; E, 2826 to 3496 (follow-ups through the
mission legs), which needs happy, review-target and default-role from A
and malformed-return and process-loss from B. A cluster child runs the bed
setup, then the shared legs it needs as functions (each leg keeps its own
assertions), then its own legs. Hidden state the map does not show:
metasystem.conf edits between legs (the good, no-tier and tier-renamed
configurations), which a cluster must restore before its legs.

## Proof

- `TestStageRunsIndependentGroupsSideBySide`: three two-second groups under a
  cap of three finish in under five seconds, keep plan order, count their
  launches and child time, and record one start and one end each.
- `TestStageWithCapOfOneRunsSerially`: the old behaviour under a cap of one.
- Cadence runs on the Mac after landing: the attempt ids and wall times are
  in the goal's conclusion.
