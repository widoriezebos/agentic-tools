# A wait fails when its producer is gone, and a quiet unit is named

- Kind: design
- Id: 01M5A0PR00FVN1TEND50NT1ME0
- Status: accepted
- Goals: a-proof-unit-ends-on-time

The goal a-proof-unit-ends-on-time was opened by the steward in Wido's name on 2026-10-17 (lane test, finding 374); the Id above is permanent.

Anchors were read at origin/main 6e1b555cb in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-reads/metasystem. Proof records were read in the landing checkout, `artifacts/agents/landing/proofs/`. Rulings:

- R-35-m3 (`memory/rulings.md:62`): no load-slowness into failure;
- R-104 (`:163`): artificial clocks and injected observations;
- R-142 and R-143 (`:201-202`): a person decides.

The supervisor's policy belongs to plans/proof-groups-detect-hangs-by-progress-not-the-clock-design.md (`:18`, `:51`). This page changes none of it.

## Evidence

**Finding 374** (2026-10-17): the full proof of main 6e1b555cb hung for 74 minutes in metasystem/internal/deploy, in TestAnAdapterLeftByADeadRunHoldsTheDeployUntilPaused. The steward found the process and sent SIGQUIT. `landing status` said "proving main (236 of 237 units, 72 min)", and the only health row that fired was proof-drift. The repeat was green in 19 s.

**Finding 375:** at 05:51 `lsof +D` on the test's registry showed only the test binary. The adapter (the build script that should block on the bed's "stalled" FIFO) was not alive, and the fixture custodian's log ends at 04:40:39.

The dump (proof log `20261017T023627.110000000Z.log:94477`) shows the test in `testenv.Await` (`await.go:34`) under `(*bed).awaitHeld` (`internal/deploy/bed_test.go:303`), called from `runner_test.go:196`. **The wait was for a fact whose producer had already died.**

**Finding 240** (2026-10-12): the same package, 43 minutes, blocked in a channel receive (`runner_test.go:262`).

Over the last 84 proofs, the longest stretch without a finished test in a green run was 2098 s (one cmd/metasystem shard), 1314 s (adopt), and at most 27 s for deploy.

## Problem

**The wait has no failure condition but the deadline, which the lane removed.**

- `testenv.Await`, `AwaitOr` and `AwaitError` (`internal/testenv/await.go:32-49`) promise that a slower host makes the test slower, never red (`:24-25`).
- Their only end other than the fact is the test binary's deadline (`:26-31`, `deadlineRemaining` `:59-67`). The lane runs with `-timeout 0` (`proofrun/test_go.go:354`).
- So a wait whose producer is dead never ends. Nothing in the wait asks about the producer.
- The deploy bed knows its producers. `inFlight` waits for an adapter call that the bed's own child runner starts (`bed_test.go:125-135`, child from `:104`). `awaitHeld` waits for an adapter process group to hold a FIFO (`:301-311`). Both of its callers hold that group's pid: `building.PID` at `runner_test.go:193-196`, and `stalling.PID` at `ends_test.go:157-163`.
- testenv already probes process groups with `kill(target, 0)`, where ESRCH means gone (`internal/testenv/process_group.go:199-215`, `:227-232`).

**Nobody is told about a quiet unit.**

- The CPU supervisor (`proofrun/supervisor.go:200`, `:395-408`) saw progress for 74 minutes (hypothesis: Await's 10 ms poll).
- `goReadinessWriter` sees every test event live (`proofrun/test_build.go:1326-1356`), but nothing keeps when a test last finished.
- `landing status` shows counts only (`plain/status.go:610-636`), and proof-drift compares whole proofs (`health.go:397-399`, `:532`).
- The shard output reaches the proof log only after the native run (`test_build.go:1668`, `repoproof/full.go:235`). Live diagnostics therefore need their own protocol lines, keyed by unit *and* shard, because cmd/metasystem runs in many shards (`test_go.go:906-922`, `full.go:229`).

## Decisions

**D1. A wait may name its producer, and fails when that producer is observed gone, never because time passed.**

- testenv gains `AwaitFrom(t, what, observed, producer func() (alive bool, detail string), giveUp func() string)`.
- Each poll asks `observed` first. When it is false, it asks `producer`. When the producer is gone, `observed` is asked once more, because the fact may have landed just before the producer ended. If it is still false, the wait fails its own test with "<what>: its producer is gone (<detail>)" followed by `giveUp`'s output.
- An unknown or failed probe counts as alive, so doubt never fails a test. The binary's deadline works as today.
- `Await`, `AwaitOr` and `AwaitError` do not change.
- testenv also gains `ProcessGroupProducer(pgid)`: `kill(-pgid, 0)`, where only ESRCH is gone.
- **Adoption, Scrooge.** U1 adopts the contract in the deploy bed's waits that have a process producer *whose death the probe can see*. That is `awaitHeld`, which takes the adapter's group pid from both callers. Its `giveUp` returns each FIFO with its `.held` marker and the bed's adapter call record (`b.calls()`, `bed_test.go:225`). Tonight the wait would have failed as soon as the adapter's group was gone, naming the group and showing whether the build call ever ran. The incident evidence shows no open files and no custodian observation after 04:40:39 (finding 375). That is consistent with ESRCH but does not prove it: testenv itself tells closed files, exit and reaping apart (`process_group.go:218-225`).
- **Inventoried, not adopted:**
  - `inFlight`. Its producer is the bed's own child, which stays a zombie, alive to `kill(pid, 0)`, until the test reaps it. It needs a probe that does not reap; it follows when it bites.
  - The test-level waits on adapters: `runner_test.go:613` (the `rolling` group) and `ends_test.go:63` (which waits for death itself).
  - In-process producers: `runner_test.go:449` and `:499`.
  - The channel receives: `runner_test.go:262`, finding 240. These have no producer to ask, and D2 names them.
- **Why this keeps R-35-m3.** A wait fails only on an observed fact: the process that must produce the event no longer exists. A slow producer that is alive never fails, however long it takes, and a stopped one is alive. No elapsed time enters the decision.
- **Why this keeps R-104.** The contract is tested with injected producer observations and an injected pause. The group probe is tested with an injected `kill`. A real dead group is used once, with no timing in the assertion.
- **The failed test.** A failed wait follows the ordinary failure classification. The package ends with a FAIL naming the test, and the lane's existing path repeats it alone. A green repeat can confirm a flake (`health.go:423`, `:491`); a repeat that fails stays red.
- **Remedy.** The failure names the dead producer, the bed's call record and the FIFOs, so a person reads why the producer died.

**D2. A unit with no finished test for its bound is named, and nothing is killed.**

- **The bound.** `clamp(4 × L, 10 min, 60 min)`, rounded up to a whole minute. L is the unit's longest green gap between finished tests in the last 20 proofs; a unit without history gets 60 min. It is a pure function with named constants, used for this diagnostic only.
- **The gaps.** The marks are the package `start`, each test terminal and the package terminal. `proofOutput.testLine` (`plain/standing.go:351-368`) measures the gaps from event `Time` and stores them in the scope record as `gaps` (`scope.go:24-34`, `:340`). A record without `gaps` is filled once from its kept log by the same parser and written back.
- **The history's path.** The lane passes the history as `LANDING_UNIT_HISTORY` (`prove.go:1219-1221`). Each shard's bound is computed in `runGoPartitions` from its own environment. The VM path forwards it too (`full.go:413`). U2 reports native package runs and their batch-tag runs. Quiet reporting for declared groups (`RunNamedGroups`, `groups.go:89`) is deferred.
- **Observation per (unit, shard).**
  - `goReadinessWriter` keeps each shard's last finished test.
  - When a sample is neither stopped nor host-waiting (the existing classification, untouched) and no test has finished for the bound, the supervisor reports one observation. The reporter prints `landing quiet <unit> <shard> <last-finish-ms> <bound-ms> <test>`.
  - The first finished test after it emits `landing resumed <unit> <shard> <test>`. That line, or that shard's own `landing package <unit> <shard> …` line, clears the observation.
  - Another shard's completion clears nothing.
  - **One lock orders it all.** The finish record serializes three things under one shared lock: checking the threshold, marking and publishing quiet, and clearing and publishing resumed. The stream writer and the supervisor's sample loop run concurrently (`test_build.go:1333`, `supervisor.go:312`). Locking each method alone would let a test finish between the quiet line and its mark and leave a stale warning. A deterministic overlap fixture proves that a concurrent finish cannot leave a stale quiet observation. It uses an acknowledged pre-lock barrier to force `Finish` to reach the mutex boundary before quiet is published, then publishes quiet before admitting `Finish`, and requires the quiet and resumed lines in that order and a cleared status. Both ordering mutations fail unchanged assertions without relying on goroutine scheduling.
- **Surfaces.** The proving line gains "; metasystem/internal/deploy shard 91: no test finished for 31 min, last <test>", with minutes from the status clock. A health row `suspected-stall` names the same.
- **The supervisor kills nothing.** The lane says what it does, and a person ends a hang whose producer is alive.
- **Replay.** Replaying the 84 retained proofs (7785 green unit runs) against the history before each found no green gap reaching its rounded bound. The closest was adopt at 1,201.54 s against 1,620 s, about 74.2 %.
- **Remedy.** The row reads: "no test finished in <unit> shard <n> for <m> min (last <test>); if its producer is alive and it does not resume, end it by hand."

**D3. proof-drift and the CPU supervisor stay as they are.**

## Units

**U1 (D1).** The producer-based wait in testenv and the deploy bed's `awaitHeld` adoption. It works alone: a deploy test waiting for a hold from an adapter whose group is gone fails at once with the evidence, and is repeated. Its scope is the deploy bed's waits with a process producer; declared groups, other beds and the lane are outside U1. Inventory and bound in briefs/a-proof-unit-ends-on-time-U1-brief.md: 38–47 production lines, stop line 60.

**U2 (D2).** The quiet diagnostics:

- `gaps` in the scope record with the writeback bootstrap;
- the pure bound;
- the (unit, shard) observations with recovery;
- the status line and the health row.

It works alone: a quiet shard is named in status and health until it resumes or ends. It is ordered after the-lane-says-what-it-does' U1a by rebase, since both change `status.go:610-636` and `describe.go:73`. Inventory and bound in briefs/a-proof-unit-ends-on-time-U2-brief.md: 186–226 production lines, stop line 240.

## Tests

**U1:**

- the contract by table on an injected pause and an injected producer:
  - a producer that stays alive through hours of injected time, a simulated stop included, never fails;
  - a producer that is gone gives an immediate failure with the `giveUp` output;
  - a gone producer whose fact landed passes;
  - no producer, unchanged;
- the group probe with an injected `kill`;
- the bed's `awaitHeld` against a reaped real group.

Mutations: the producer is ignored; the wait fails on elapsed time; the `giveUp` output is dropped; the bed keeps `Await`; doubt counts as gone; the fact is not checked again after the producer is gone. The ignored-producer mutation uses a finite injected deadline sequence, and the bed recorder forwards the test's `Deadline` method, so both regressions end and fail the unchanged producer-gone assertion without a timeout panic. The elapsed-time mutant is rejected by the existing wall-time audit (`walltime_test.go:78`) on an isolated tree.

**U2:**

- the bound table, and the gaps with the writeback bootstrap;
- the observation with its exemptions kept;
- recovery while the package runs;
- shard identity;
- the status line and the health row from injected observations.

Mutations: floor or ceiling removed; recovery suppressed; observations keyed by unit alone; writeback skipped; status omits the shard; the observation raised while stopped.

## Critique record

- Round 1 (Astra, gpt-6-astra, 06:20, scratchpad/codex-crit-unitbound-1.log; 6 material, 1 minor): all folded; D1 redesigned as a progress bound within R-35-m3.
- Round 2 (Astra, 06:35, scratchpad/codex-crit-unitbound-2.log; 4 material, 2 minor): all folded; D1 moved to the test-side wait deadline, the supervisor observes only.
- Round 3 (Astra, 06:47, scratchpad/codex-crit-unitbound-3.log; 3 material, 2 minor): all folded; D1 is a producer-based wait contract, D2 diagnostics only, split U1/U2.
  - M1: no elapsed-time failure anywhere; the variable and the fallback deadline are dropped.
  - QUIET-RECOVERY: the `landing resumed` line.
  - QUIET-SHARD: (unit, shard) keys.
  - SIZE-01: the writeback is in U2's inventory.
  - FLAKE-01: the ordinary classification, stated in D1.
- Round 4 (Astra, 06:56, scratchpad/codex-crit-unitbound-4.log; 2 material, 2 minor): all folded.
  - QUIET-ORDER: one lock in D2, with the overlap fixture in the U2 brief.
  - MUTATION-END: the finite deadline sequence and the deadline-forwarding recorders.
  - INCIDENT-PROOF: D1's wording on finding 375.
  - GROUP-SCOPE: U1 covers the deploy bed's waits with a process producer; U2 covers native and batch-tag runs; declared groups are deferred.
- Round 5 (Astra, 07:07, scratchpad/codex-crit-unitbound-5.log; 1 material, 1 minor): all folded.
  - QUIET-MUTATION: the pre-lock barrier fixture in D2 and in the U2 brief.
  - REPLAY-ROUNDING: the replay sentence in D2.

## Not in scope

- Finding 375's cause, which is why the adapter died.
- Finding 379, a binary that dies.
- Finding 380, proof folder retention.
- Ending a hang whose producer is alive. A person does that; D2 names it.

## OPEN

- The goal and the design Id.
- A non-reaping probe for the bed's own child (`inFlight`). `identity`'s prober, which testenv already imports (`process_group.go:11`), may tell a zombie from a live process (hypothesis).
- The bootstrap's one-time read of up to 20 logs of about 50 MB each, then the writeback. Not measured.

## Steward decisions after round 4 (07:02, Wido's name)

9. U2's publish callbacks run under the finish record's lock and may only format and print or enqueue one line; no callback waits on I/O or another lock (the U2 brief states the constraint; its overlap fixture holds).
10. U1's mutations 2 and 8 run with `-timeout 2m` and take about 90 s of real time each; they run only in the builder's mutation runs, never in a proof.
11. Quiet reporting of declared groups (the scoped `go-batchtest` repeat included) is deferred beyond U2 and named in Not in scope.

## Steward decisions after round 5 (07:12, Wido's name)

12. U2's finish record carries a private test-only `beforeLock` hook (nil in production, an injection seam under R-104) for the overlap fixture's barrier; mutation 11 is caught by a `TryLock` check inside `publish`, the deterministic proof that quiet is published under the lock.

## Folded after round 7 (07:58, Wido's name)

- The build briefs this page names (U1, U2) live with the lane test's evidence at /Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/ (a-proof-unit-ends-on-time-U1-brief.md FINAL by the stop rule; a-proof-unit-ends-on-time-U2-brief.md DRAFT); their production-line inventories and numbered mutation fixtures are verified by each unit's build report and accountable read, not by this page.
- Incident certainty: the retained dump proves that `awaitHeld` remained blocked for 74 minutes; the other observations (no open files, no custodian observation after 04:40:39) suggest the producer was lost but do not prove that its process group had disappeared; D1's ESRCH condition is what the new wait tests.
- Round 7 (Astra, gpt-6-astra, 07:56, scratchpad/codex-crit-unitbound-7.log, against the registered page on main 0128fc34d; 0 material, 2 minor): both folded here. Lineage 6 → 4 → 3 → 2 → 1 → (admission) → 0: the critique stops; Status accepted.
