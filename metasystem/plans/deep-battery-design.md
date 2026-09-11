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
is half the cores, at most six, at least one: six on the 18-core Mac, two on
the 4-vCPU VM. Section groups spawn process trees, so the cap is about
processes, not cores. Wido decides the VM's committed value if two is wrong.

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

`goal-full-coverage` and `missionrunner-full-coverage` declare four shards.
Measured before: 538 s and 444 s alone, 919 s and 711 s under the pool.

Found on the way: the coverage parser's JSON loop retried a decode error
forever; it now ends at the first error and keeps what it read.

## Slices 3b and 5: the big sections and the race gate

Not in this landing. Splitting the dispatcher, adoption and supervision
sections into per-scenario groups is the next cut; the race gate follows
the shards.

## Proof

- `TestStageRunsIndependentGroupsSideBySide`: three two-second groups under a
  cap of three finish in under five seconds, keep plan order, count their
  launches and child time, and record one start and one end each.
- `TestStageWithCapOfOneRunsSerially`: the old behaviour under a cap of one.
- Cadence runs on the Mac after landing: the attempt ids and wall times are
  in the goal's conclusion.
