# Design: patience for the missionrunner terminate-group test family

- Goal: `plans/goals/missionrunner-terminate-flake.md` (claimed m0b,
  lineage main-1788250419-3170380-8a1fb3)
- Authored: 2026-09-02 by the design delegate of job
  implementer-45c39b11f8b164818040e7d6, from the brief
  `plans/missionrunner-patience-design-brief.md`
- Law: R-35-m3 (`memory/rulings.md:62`) — anything that converts
  load-slowness into failure is a defect to fix with progress-based
  patience. Doctrine: `records/patience/patience-attempts.md`. Precedent:
  the steward tick landing 65c36111 (close-the-channel, cleanup handshake
  on every exit path, progress-based deadline).
- Scope: exactly one product-test file changes,
  `internal/missionrunner/winddown_test.go`. No product code changes. No
  other test file changes.

## 1. Facts traced (file:line, current tree)

- `winddown_test.go:41-50` — `waitGroupDead(pgid, patience)`: a FIXED
  3-second wall-clock window polling `groupAlive` (kill(-pgid, 0),
  `proc.go:67-77`), used by both flapping tests (`:108`, `:214`).
  `groupAlive` counts a zombie leader as alive; the harness reaps the
  leader on a separate goroutine (`:34`), so the window must also cover
  goroutine scheduling under load.
- `winddown_test.go:77-89` — `classifyLiveWindDown` runs exactly 3
  census scans BACK-TO-BACK with no pacing and never re-checks whether
  the group is still alive. A group that dies between `waitGroupDead`
  giving up and the scans is classified `leaked` (red) though nothing
  leaked; three scans inside the same overloaded instant are not
  independent observations.
- `host.go:94-182` — `terminateGroup`'s nil-return invariant: it returns
  nil only when (a) the group is dead (`:95`, `:178`), (b) the group is
  zombie-only, no substantive member (`:160-168`), or (c) it SKIPPED
  signalling — ownership not provable (`:109-116`) or provably recycled
  (`:143-150`). Therefore: **nil return + the group still holding a
  live-with-readable-argv member ⇒ a skip happened.** This kernel-fact
  discriminator is what lets the tests tell "proof refused" from "kill
  sent, group slow to die" without parsing the event stream.
- `host.go:151-175` — after SIGKILL the product waits only
  `ScaledWaitAtLeast(1, time.Second)` (= 1s at any compression) before
  returning the error "survived the kill-through window". Under load a
  KILLed `sleep` whose exit processing lags 1s makes `terminateGroup`
  err while the group is about to die. The old kill-through test
  insta-fatals on that err (`winddown_test.go:105-106`) — a pure
  load-to-failure conversion. The product cap itself is by design (the
  census is the product's safety net for leftovers, `host.go:81-93`) and
  is NOT changed here; the tests stop treating its expiry as instant
  proof of a leak.
- `proc.go:85-101` — `groupHasSubstantiveMember`: a member is
  substantive iff Alive with readable argv; zombie-only groups are
  finished work (the winddown-zombie-ownership-linux rule).
- Ownership proof: `janitor/killproof.go:146-199` +
  `identity/verification.go:33-64`. Under load, `VerifyProcess` returns
  Indeterminate on any transient /proc read failure, and one
  indeterminate member makes the group `GroupIndeterminate`
  (`killproof.go:181-188`) → `terminateGroup` skips. The double
  start-time read inside one verification compares in PAIRED mode
  (StartTicks + BootID), which is clock-step immune
  (`identity/identity.go:122`, proven by `TestAliveRefClockStepImmunity`,
  `identity/identity_test.go:230-252`) — so a btime step alone does not
  explain a PERSISTENT refusal; transient /proc blur under load does
  explain a TRANSIENT one.
- Census custody proof: `census/tagged.go:219-223` — a
  `VerificationIndeterminate` process is recorded with PID and Reason
  but **no PGID**. `censusHoldsGroup` (`winddown_test.go:52-65`) matches
  indeterminate rows by PGID, so an identity-blurred member of the very
  group under wind-down can NEVER prove custody. Under load this biases
  the compression test toward `leaked`. See §4 for the separation.
- Only the bash leader carries the tag (`winddown_test.go:25`: argv
  `[bash -c <script> metasystem util hold --tag <tag>]`); the `sleep`
  child does not. Census scans by tag see only the leader; group-member
  observation (Getpgid walk) sees both. Progress tracking must therefore
  fingerprint BOTH the census picture and the group membership.
- Quiet-run behavior at HEAD: both flapping tests pass 3/3 in this
  delegate's sandbox (`go test -run 'TestTerminateGroupKillsThroughATermImmuneOwnedGroup|TestTerminateGroupLeaksNoGroupsUnderCompression' -count=3`
  → PASS, 0.78s). The family reds only under load, consistent with the
  goal record's evidence.

## 2. Failure anatomy (design question 1)

Per test, the wall-clock assumptions, the teardown races, and the
progress signal each wait should key on:

| Test | Wall-clock assumption | Race | Progress signal |
| --- | --- | --- | --- |
| KillsThroughATermImmuneOwnedGroup | `waitGroupDead` 3s; instant-fatal on the product's 1s kill floor erring | transient ownership refusal (skip) is indistinguishable from a slow kill; zombie leader awaiting the reap goroutine counts as alive | group membership: the set of substantive member pids of the pgid shrinking, or the group leaving the process table |
| LeaksNoGroupsUnderCompression | `waitGroupDead` 3s; 3 unpaced census scans | a group dying AFTER the 3s window but BEFORE/DURING the scans classifies `leaked`; identity blur makes custody unprovable within 3 instant scans | same group-membership fingerprint, PLUS the census observation fingerprint (tagged rows, indeterminate rows, enumeration error) changing |
| NeverSignalsAForeignGroup | none (must-STAY-alive is correctly an instant check) | none observed | unchanged |
| ClassifyLiveWindDown… (table test) | none (pure) | none | rewritten to drive the new classifier (§3.4) |
| GroupOwnershipFixtureFallbackStaysExact | none | none | unchanged |
| Family-external: cleanup `SIGKILL` fire-and-forget (`:35-37`) | — | leaked groups/goroutines outlive the test into siblings — the goal's cross-package interference lead (internal/supervise flap) | the reap goroutine completing; the group leaving the table |

Taxonomy fit (doctrine `records/patience/patience-attempts.md`,
"Taxonomy of waits"): every wait here is on an **external/OS event**
(process death, /proc settling) — there is no monotonic actor attempt
counter like the census `scanSeq` to count. The doctrine's honest tool
for this row is a condition-based wait with a generous,
progress-resetting failsafe, not attempt counting; the one true
attempt-shaped actor is `terminateGroup` itself, which the kill-through
test invokes in a bounded attempt loop (§3.3).

## 3. The port (design question 2) — mechanical rules

All edits are in `internal/missionrunner/winddown_test.go`. Constants,
signatures, and control flow are specified so the implementer decides
nothing.

### 3.0 Constants

```go
const (
    killThroughAttempts = 3                      // mirrors the retired censusHandoffProofAttempts rationale
    observationPoll     = 20 * time.Millisecond  // the existing waitGroupDead poll, kept
    observationStall    = 30 * time.Second       // failsafe: fires only after ZERO observable change
    observationCeiling  = 10 * observationStall  // hang detector for oscillation; motion is not progress
)
```

`observationStall` deliberately does NOT use `ScaledWait*`: the
compression scale compresses PRODUCT waits to provoke leaks; the test's
own observation patience must not compress with it. 30s matches the
steward-precedent failsafe. Delete `waitGroupDead` and
`censusHandoffProofAttempts` entirely (no other callers exist).

### 3.1 Group observation and fingerprint

```go
// groupObservation is one kernel-fact snapshot of the group under wind-down.
type groupObservation struct {
    alive           bool    // groupAlive(pgid)
    substantivePids []int64 // sorted pids in the group that are Alive with readable argv
}

func observeGroup(pgid int) groupObservation
```

`observeGroup` walks `identity.AllPids()`, keeps pids whose
`unix.Getpgid` equals pgid, probes each with `identity.KernelProber{}`,
and records those Alive with `ArgvKnown` (the same walk as
`groupHasSubstantiveMember`, `proc.go:85-101`, but returning the pids).
An `AllPids` error yields `substantivePids = nil` with `alive` from
`groupAlive` — conservative, and the fingerprint change on the NEXT
successful walk still counts as progress. `down()` on the observation
means `!alive || len(substantivePids) == 0` (zombie-only is dead work —
the recorded harness rule at `winddown_test.go:30-33`).

The fingerprint of an observation is its deterministic string rendering
(alive flag + sorted pids). The census fingerprint is the deterministic
rendering of `EnumerationError`, sorted Tagged `(PID,PGID)` pairs, and
sorted Indeterminate `(PID,PGID,Reason)` triples.

### 3.2 The condition-based wait with progress-resetting failsafe

```go
type groupWaitOutcome int
const (
    groupWentDown groupWaitOutcome = iota
    groupStalled                    // zero observable change for observationStall
)

func waitGroupDown(t *testing.T, pgid int) groupWaitOutcome
```

Loop: observe; if `down()` return `groupWentDown`; if the fingerprint
differs from the previous iteration's, reset the stall clock; if the
stall clock exceeds `observationStall`, return `groupStalled`; if total
elapsed exceeds `observationCeiling`, `t.Fatalf` naming the ceiling and
the last observation ("observation kept changing without converging for
%v — motion is not progress"); sleep `observationPoll`. The ceiling is a
hang detector, not a speed assertion: a converging wait returns on its
condition regardless of load.

### 3.3 TestTerminateGroupKillsThroughATermImmuneOwnedGroup

Replace the body after `pgid := cmd.Process.Pid` with:

```go
for attempt := 1; attempt <= killThroughAttempts; attempt++ {
    err := engine.terminateGroup(pgid, tag, false)
    if err != nil {
        // Ownership was proven and the kill happened; the product's 1s
        // kill floor expired. Under load that is slowness, not a leak,
        // unless the group then fails to die with zero progress.
        if waitGroupDown(t, pgid) == groupWentDown {
            t.Logf("kill-through reported %v but the group died late — load slowness, not a leak", err)
            return
        }
        t.Fatalf("kill-through wind-down failed and the group made no progress for %v: %v", observationStall, err)
    }
    if waitGroupDown(t, pgid) == groupWentDown {
        return
    }
    // nil return + the group still substantive after a stalled patient
    // wait ⇒ terminateGroup took a skip path (host.go nil-return
    // invariant): the ownership proof was refused. Retry the actor.
}
t.Fatalf("group %d still holds substantive members after %d wind-down attempts whose ownership proof was refused every time — the identity-proof-refusal shape (vm-epoch lead); this red is genuine and must not be silenced",
    pgid, killThroughAttempts)
```

The attempt loop is the doctrine's Tier-1 shape applied to the one real
actor: K invocations of `terminateGroup` that all fail to produce a
signalled group is a genuine defect on ANY machine, regardless of how
slow each invocation is. Retrying is safe: `terminateGroup` is
idempotent against a dead or dying group (`host.go:95`), and the only
misread direction of the discriminator (a dying member transiently
reading as substantive) costs one harmless extra invocation.

### 3.4 classifyLiveWindDown rewritten as a patient observer

New contract (same file):

```go
const (
    liveWindDownLeaked            liveWindDownClassification = "leaked"
    liveWindDownAbandonedToCensus liveWindDownClassification = "abandoned-to-census"
    liveWindDownDiedLate          liveWindDownClassification = "died-late"
)

type windDownObservation struct {
    scan        func() census.TaggedProcessCensus
    group       func() groupObservation
    resolvePGID func(pid int64) (int64, error) // unix.Getpgid in the real harness
    now         func() time.Time               // time.Now in the real harness
    pause       func()                         // time.Sleep(observationPoll) in the real harness
    stall       time.Duration                  // observationStall in the real harness
    ceiling     time.Duration                  // observationCeiling in the real harness
}

func classifyLiveWindDown(pgid int, windDownErr error, obs windDownObservation) (liveWindDownClassification, census.TaggedProcessCensus)
```

Loop, exactly:

1. `g := obs.group()`; if `g` is down → `liveWindDownDiedLate` (this
   first check subsumes and replaces the old `waitGroupDead` call —
   the fast path for a promptly dead group is one group observation).
2. If `windDownErr == nil`: `observed = obs.scan()`; if
   `censusHoldsGroup(pgid, observed, obs.resolvePGID)` →
   `liveWindDownAbandonedToCensus`. When `windDownErr != nil`, never
   scan and never grant census custody: an errored kill-through with a
   still-live group is only excused by actual death (preserves the old
   "kill-through error is a leak despite census visibility" rule,
   now with death-patience added).
3. Fingerprint = group fingerprint + census fingerprint (empty census
   part on the err path). Changed since last iteration → reset the
   stall clock.
4. Stall clock ≥ `obs.stall` with no change → `liveWindDownLeaked`,
   returning the last scanned census (zero-value census on the err
   path).
5. Total elapsed ≥ `obs.ceiling` → `liveWindDownLeaked` with the
   oscillation wording from §3.2 in the caller's log.
6. `obs.pause()`; repeat.

`censusHoldsGroup` gains the resolver parameter:

```go
func censusHoldsGroup(pgid int, observed census.TaggedProcessCensus, resolvePGID func(int64) (int64, error)) bool
```

Rule: unchanged matching for Tagged and Indeterminate rows in the
signalable universe with `PGID == pgid`; ADDITIONALLY, an Indeterminate
signalable row with `PGID == 0` and `PID != 0` holds the group when
`resolvePGID(PID)` returns pgid with nil error. This is proof
completion with a kernel fact, not proof weakening: it establishes both
that the process is in our group (kernel) and that the census has it in
view (the row) — exactly the abandoned-to-census definition. It
compensates for the census's missing PGID on identity-indeterminate
rows (§4, separated defect) without touching product code.

### 3.5 TestTerminateGroupLeaksNoGroupsUnderCompression

Per cycle, replace the wait-then-classify block with:

```go
windDownErr := engine.terminateGroup(pgid, tag, false)
classification, observed := classifyLiveWindDown(pgid, windDownErr, realWindDownObservation(tag, pgid))
switch classification {
case liveWindDownDiedLate:
    continue
case liveWindDownAbandonedToCensus:
    abandonedToCensus++
    continue
default:
    leaked++
    t.Logf(... existing message, unchanged fields ...)
}
```

`realWindDownObservation(tag, pgid)` wires `scanTaggedGroup(tag)`,
`observeGroup(pgid)`, `unix.Getpgid`, `time.Now`,
`time.Sleep(observationPoll)`, `observationStall`, `observationCeiling`.
The `t.Setenv` compression scale, the 4-cycle loop, the alternating
TERM-immunity, and the final leak assertion are unchanged.

### 3.6 The table test rewritten

`TestClassifyLiveWindDownDistinguishesLeakFromLawfulCensusHandoff`
drives the new contract with a scripted fake: `group` and `scan` consume
per-row sequences (the last element repeats when exhausted), `now` reads
a fake clock, `pause` advances it by 1 fake second, `stall` = 5 fake
seconds, `ceiling` = 50 fake seconds, `resolvePGID` is a scripted map.
Rows, each asserting classification and consumed scan count:

1. *transient census failures clear into lawful custody* — group up
   throughout; scans: enumeration error, indeterminate row not matching,
   tagged row with `PGID: pgid` → abandoned-to-census, 3 scans.
2. *exact indeterminate group remains in census custody* — one scan
   with the indeterminate `PGID: pgid` signalable row → abandoned, 1
   scan.
3. *zero-PGID indeterminate resolved by the kernel holds custody* —
   one scan with `{PID: p, PGID: 0, Universe: signalable}` and
   `resolvePGID(p) = pgid` → abandoned, 1 scan (the §3.4 completion
   rule's positive case).
4. *frozen live group outside custody is a leak* — group up and
   unchanging; scans forever return a tagged row for `pgid+1`; the fake
   clock crosses stall → leaked, scans = the deterministic count the
   fake pacing yields (the row states the number; the implementer
   computes it from stall=5, pause=1: 6 scans).
5. *oscillating observations that never converge hit the ceiling* —
   scans alternate two distinct non-matching censuses; → leaked at the
   ceiling, deterministic scan count from ceiling=50, pause=1.
6. *group death mid-observation is not a leak* — group up for 2
   observations then down → died-late, 2 scans.
7. *kill-through error is a leak only while the group lives* —
   `windDownErr` set, group up and frozen → leaked, 0 scans.
8. *kill-through error excused by actual death* — `windDownErr` set,
   group down on the second observation → died-late, 0 scans.

### 3.7 Cleanup handshake on every exit path (steward precedent)

`spawnTaggedGroup` changes:

```go
reaped := make(chan struct{})
go func() { _ = cmd.Wait(); close(reaped) }()
t.Cleanup(func() {
    _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
    select {
    case <-reaped:
    case <-time.After(observationStall):
        t.Logf("group %d: reap handshake failsafe expired after %v", cmd.Process.Pid, observationStall)
    }
})
```

Cleanup LIFO ordering already places this before the TempDir removal
registered by the earlier `t.TempDir()` call, matching the steward
landing's ordering argument. The handshake bounds how long a killed
group and its reaper goroutine can outlive the test — the goal's
cross-package interference lead (the internal/supervise flap) is
exactly the exposure this closes. `TestTerminateGroupNeverSignalsAForeignGroup`
keeps its existing synchronous inline reap; it is not flapping and its
must-stay-alive assertion is correctly instantaneous.

## 4. Identity-proof refusal: patience versus masking (design question 3)

The flap is BOTH things, and the design separates them:

- **The patience defect (fixed here):** fixed 3-second windows, unpaced
  proof scans, insta-fatal on the product's 1-second kill floor, and a
  leak verdict that races late death. All are load-to-failure
  conversions in TEST code; §3 removes them.
- **The identity class (kept red, sharpened):** from the code text, the
  in-verification clock-step case is already immune (paired
  StartTicks+BootID compare, `identity/identity.go:122`,
  `identity_test.go:230-252`), so the expected refusal source under load
  is transient /proc blur — which clears within the K=3 actor attempts.
  A refusal that does NOT clear is the real vm-epoch-class suspect, and
  the port keeps it red on purpose: the §3.3 exhaustion message names
  "ownership proof refused on every attempt", turning today's mute
  wall-clock red into a diagnosable specimen. Patience here can only
  mask a bug that heals within three fresh wind-down invocations while
  the group stays otherwise healthy; such a transient is
  indistinguishable, by the product's own fail-closed design, from the
  /proc blur the product deliberately tolerates by skipping — so the
  residual masking window is the product's own contract, not a test
  choice.
- **A separated product defect, named, not fixed here:** the census
  records identity-indeterminate processes with no PGID
  (`census/tagged.go:219-223`), so custody of a blurred member is
  unprovable by the census's own evidence. The §3.4 resolver completion
  compensates on the test side with a kernel fact. The product-side gap
  (stamp the group on indeterminate rows, or record why not) should be
  appended to the goal record as a follow-up lead; it is outside this
  design's boundary.

What stays red after the port: a `terminateGroup` leak error whose
group then makes zero progress; K exhausted ownership refusals; a live
substantive group outside census custody with zero observable change
for `observationStall`; any wait that oscillates past
`observationCeiling`. What is absorbed: slow death, slow zombie reap,
late death after the product's floors, transient identity blur, and
custody proven on any later paced scan.

## 5. Proof profile (design question 4)

Matching the steward-tick precedent (65c36111) on the 4-CPU guest:

1. Delegate-side: `go vet ./internal/missionrunner/` clean; focused run
   `go test -race -count=20 -run 'TestTerminateGroup|TestClassifyLiveWindDown' ./internal/missionrunner/`
   20/20 green.
2. Orchestrator-side (outside the sandbox, per KI-15): compile once with
   `go test -race -c ./internal/missionrunner/`, then run 8 concurrent
   instances of the binary, 20 repetitions each, filtered to the family
   (`-test.run 'TestTerminateGroup|TestClassifyLiveWindDown' -test.count=20`),
   expecting 160/160 green — 8 process-spawning instances on 4 CPUs IS
   the contention profile that exposed the original defect. The
   spawned `sleep 30` groups of concurrent instances double as load.
3. One full-package run under that same concurrent load
   (`go test -race ./internal/missionrunner/` while the 8 instances
   run) to confirm no ballooning: patient waits return on their
   condition, so the family's wall time must stay within the same order
   as the quiet run, not stretch toward the failsafes.
4. Acceptance: any red in (2)/(3) that prints the §3.3 refusal-exhaustion
   message is a TRUE finding for the vm-epoch lead — record it on the
   goal, do not loosen the test. Any other red fails the design.

## 6. Explicitly unchanged

`terminateGroup` and every product file; `host_process_test.go` (its
terminateGroup verdicts are instant-fact checks on dead/foreign groups);
`TestTerminateGroupNeverSignalsAForeignGroup` and
`TestGroupOwnershipFixtureFallbackStaysExact`; the compression scale
env; `scanTaggedGroup`; the janitor, identity, and census packages.

## 7. Self-grade

**Grade: A-.** Every wait in the family is now keyed to a named
progress signal with a progress-resetting failsafe and a hang-detecting
ceiling; the refusal shape is separated from slowness by a kernel-fact
invariant quoted from `host.go`, not by guesswork; the masking question
is answered from code text with the residual window stated honestly;
all control flow, constants, signatures, and table rows are specified.
Docked from A: the deterministic scan counts in table rows 4 and 5 are
stated as derivations rather than literals, and the `observeGroup`
error-path fingerprint behavior, while specified, has no table row of
its own.

**Reject condition.** Reject this design if any of the following holds:
(a) the implementer needs any decision not mechanically fixed by §3
(that is a design gap — stop and report, never fill); (b) the
`host.go:94-182` nil-return invariant of §1 is contradicted by the tree
at implementation time (the refusal discriminator would then be
unsound); (c) the proof profile's step 3 shows the family's loaded wall
time reaching the `observationStall` failsafe on green runs (the
failsafe would then be load-sensitive, recreating the defect class);
(d) implementing it requires touching any file other than
`internal/missionrunner/winddown_test.go`.
