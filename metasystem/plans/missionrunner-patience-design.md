# Design: patience for the missionrunner terminate-group test family

- Goal: `plans/goals/missionrunner-terminate-flake.md` (claimed m0b,
  lineage main-1788250419-3170380-8a1fb3)
- Revision 2, 2026-09-02, by the design delegate of job
  implementer-8873cb1aebfbeab677c2e9d8. Revision 1 was authored by
  implementer-45c39b11f8b164818040e7d6 from the brief
  `plans/missionrunner-patience-design-brief.md`; the round-1 critique
  (`records/misc/missionrunner-patience-critique-r1.md`, nine material
  findings DC-PAT-001 through DC-PAT-009) was answered by an executable
  spike whose verdicts and implied rules are the evidence base of this
  revision: `records/misc/missionrunner-spike-verdicts.md`
  (implementer-9159613b56eed6b68e44b78f). Each fold below names its
  finding; the spike's five declared gaps are recorded at the fold they
  bound, not resolved by assertion.
- Spike coverage boundary (spike gap 1, recorded verbatim in substance):
  the spike brief named `internal/proc`, a package that does not exist;
  the spike actually ran against `internal/missionrunner/proc.go` plus
  `internal/identity` and `internal/census`. Those are exactly the
  surfaces this design cites, so the spike's coverage matches this
  design's scope — but any reading of "internal/proc" as a different
  package is uncovered.
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
- `host.go:120-131` (spike-confirmed, DC-PAT-005 anchor) — SIGTERM is
  sent at `:120` BEFORE the `ScaledWaitAtLeast` and `Interval` calls at
  `:125-131`, either of which can return a CONFIGURATION error. A
  non-nil return therefore does NOT prove SIGKILL was sent: the spike
  drove the real `terminateGroup` with an invalid heartbeat-interval
  environment variable and got a configuration error in 476
  microseconds with no SIGKILL ever sent, while the TERM-responsive
  variant then died of the already-sent SIGTERM. Only the exact error
  text at `:175`, `survived the kill-through window`, proves ownership
  was held and SIGKILL was sent.
- `proc.go:85-101` — `groupHasSubstantiveMember`: a member is
  substantive iff Alive with readable argv; zombie-only groups are
  finished work (the winddown-zombie-ownership-linux rule). Note the
  product's own asymmetry: an `AllPids` failure returns true ("unknown:
  stay conservative", `:88`) — uncertainty is never death.
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
- Census row semantics: `census/tagged.go:165-229`. A tag-verified
  process gets a Tagged row with PGID (`:216-218`); an
  identity-indeterminate process gets an Indeterminate row with PID and
  Reason but NO PGID and NO start identity (`:219-223`, `:56-61` —
  spike-confirmed, DC-PAT-006 anchor); and a process whose argv is
  readable but does not match the tag (`VerificationNotOurs`,
  `:224-226`) contributes **no row at all**. The untagged `sleep` child
  of every fixture group is that last case: substantive to the kernel,
  invisible to the census by design. Any custody rule quantified over
  ALL group members is therefore unsatisfiable on this tree; see the
  flagged fold in §3.4.
- Only the bash leader carries the tag (`winddown_test.go:25`: argv
  `[bash -c <script> metasystem util hold --tag <tag>]`); the `sleep`
  child does not. Census scans by tag see only the leader; group-member
  observation (Getpgid walk) sees both.
- Measured kill latency (spike, DC-PAT-003): across 30 loaded rounds at
  load average 14.35 on this 4-CPU guest, kill-to-down latency was 58
  microseconds to 5.8 milliseconds, and every healthy kill produced
  exactly 2 distinct group fingerprints — alive, then gone. There is no
  intermediate progress on the healthy path.
- Quiet-run behavior at HEAD: both flapping tests pass 3/3 in the
  revision-1 delegate's sandbox (`go test -run 'TestTerminateGroupKillsThroughATermImmuneOwnedGroup|TestTerminateGroupLeaksNoGroupsUnderCompression' -count=3`
  → PASS, 0.78s). The family reds only under load, consistent with the
  goal record's evidence.

## 2. Failure anatomy (design question 1)

Per test, the wall-clock assumptions, the teardown races, and the
signal each wait should key on:

| Test | Wall-clock assumption | Race | Decisive signal |
| --- | --- | --- | --- |
| KillsThroughATermImmuneOwnedGroup | `waitGroupDead` 3s; instant-fatal on ANY `terminateGroup` error, including the product's 1s kill floor erring | transient ownership refusal (skip) is indistinguishable from a slow kill; zombie leader awaiting the reap goroutine counts as alive; a configuration error can be laundered as died-late (DC-PAT-005) | a nil return is adjudicated IMMEDIATELY by one complete group observation (refusal versus death, §3.3); only the proven-kill error enters a patient death wait whose 30s stall is a hang failsafe over a millisecond-scale kernel event (§3.2) |
| LeaksNoGroupsUnderCompression | `waitGroupDead` 3s; 3 unpaced census scans | a group dying AFTER the 3s window but BEFORE/DURING the scans classifies `leaked`; identity blur makes custody unprovable within 3 instant scans; whole-census churn fakes progress (DC-PAT-004); natural fixture expiry launders refusal (DC-PAT-002) | paced scans joined kernel-first to a complete group observation (§3.4), a TARGET-RESTRICTED fingerprint resetting the stall, and a fixture hold that outlives the observation ceiling |
| NeverSignalsAForeignGroup | none (must-STAY-alive is correctly an instant check) | none observed | unchanged |
| KillThroughRefusalExhaustionStaysRed (new, §3.8) | none | none — deterministic by construction (spike DC-PAT-009: 3/3 refusals in ~30ms) | the refusal-exhaustion outcome of the real actor loop against a wrong tag |
| ClassifyLiveWindDown… (table test) | none (pure) | none | rewritten to drive the new classifier (§3.6) |
| GroupOwnershipFixtureFallbackStaysExact | none | none | unchanged |
| Family-external: cleanup `SIGKILL` fire-and-forget (`:35-37`) | — | leaked groups/goroutines outlive the test into siblings — the goal's cross-package interference lead (internal/supervise flap); leader reap alone does not prove group exit (DC-PAT-007, spike-proven with a `sleep 600 & exit` leader) | reap-channel close AND group absence, red on failsafe expiry (§3.7) |

Taxonomy fit (doctrine `records/patience/patience-attempts.md`,
"Taxonomy of waits"), amended per DC-PAT-003: the critic is right that
the group-membership fingerprint carries no intermediate progress on
the healthy kill path — the spike measured exactly 2 distinct
fingerprints per healthy kill, so the stall clock there is honestly a
TIMEOUT, not a progress tracker. The design says so plainly: the 30s
stall in §3.2 is a hang failsafe over a kernel event measured at 58
microseconds to 5.8 milliseconds under load 14.35 — four orders of
magnitude of margin — and it adjudicates only post-kill death, never a
nil return (those are adjudicated immediately, §3.3, per DC-PAT-002).
The one true attempt-shaped actor is `terminateGroup` itself, which the
kill-through test invokes in a bounded attempt loop (§3.3).

## 3. The port (design question 2) — mechanical rules

All edits are in `internal/missionrunner/winddown_test.go`. Constants,
signatures, and control flow are specified so the implementer decides
nothing.

### 3.0 Constants and fixture hold

```go
const (
    killThroughAttempts = 3                      // mirrors the retired censusHandoffProofAttempts rationale
    observationPoll     = 20 * time.Millisecond  // the existing waitGroupDead poll, kept
    observationStall    = 30 * time.Second       // hang failsafe: fires only after ZERO observable change
    observationCeiling  = 10 * observationStall  // 300s; hang detector for oscillation; motion is not progress
    fixtureHoldSeconds  = 600                    // fixture hold; MUST exceed observationCeiling (DC-PAT-002)
    killThroughErrText  = "survived the kill-through window" // host.go:175, the only error that proves SIGKILL (DC-PAT-005)
)
```

`observationStall` deliberately does NOT use `ScaledWait*`: the
compression scale compresses PRODUCT waits to provoke leaks; the test's
own observation patience must not compress with it. 30s matches the
steward-precedent failsafe, and per DC-PAT-003 it is named for what it
is: a hang failsafe over a millisecond-scale kernel event, with the
measured margin recorded in §1. Delete `waitGroupDead` and
`censusHandoffProofAttempts` entirely (no other callers exist).

**Fixture hold (folds DC-PAT-002 rule a).** `spawnTaggedGroup`'s script
becomes `fmt.Sprintf("sleep %d", fixtureHoldSeconds)`, and the
TERM-immune variant becomes
`fmt.Sprintf("trap \"\" TERM; sleep %d", fixtureHoldSeconds)` — the
same two script shapes as today with the literal 30 replaced by the
constant. The spike proved the laundering: with the
shipped `sleep 30` fixture, a refused wind-down passed when the fixture
expired naturally inside the wait. A hold of 600s is 2× the 300s
observation ceiling, so natural expiry cannot race any wait in this
design. *Spike gap 5, recorded here:* the laundering was demonstrated
with a 3-second fixture against the 30-second stall; the exact
30-versus-30 race was never run because its winner is nondeterministic
by construction — that nondeterminism is itself the finding, and the
2× separation is the remedy, not a measurement.

### 3.1 Group observation and fingerprint (folds DC-PAT-001)

```go
// groupObservation is one kernel-fact snapshot of the group under wind-down.
type groupObservation struct {
    alive           bool    // groupAlive(pgid)
    walkComplete    bool    // false on pid-enumeration error or any in-group probe error
    substantivePids []int64 // sorted pids in the group that are Alive with readable argv
}

func observeGroup(pgid int) groupObservation
```

`observeGroup` sets `alive` from `groupAlive(pgid)`, then walks
`identity.AllPids()`. An `AllPids` error yields `walkComplete = false`
and nil `substantivePids`. Otherwise `walkComplete` starts true; for
each pid, `unix.Getpgid` selects group members (a `Getpgid` error skips
the pid — the process is gone); each member is probed with
`identity.KernelProber{}`: a probe ERROR sets `walkComplete = false`
(the member's state is unknown, not dead); a successful probe appends
the pid when Alive with `ArgvKnown`.

`down()` — the spike-amended rule that replaces revision 1's defective
predicate: **`!alive || (walkComplete && len(substantivePids) == 0)`.**
The spike proved revision 1's version wrong executably: an injected
enumeration failure on a LIVE group produced alive=true, zero
substantive pids, down=true under the old rule; the amended rule
answered correctly on the same observation. Uncertainty is
distinguishable from death: down requires either group absence or a
COMPLETE walk with zero substantive members (zombie-only is dead work —
the recorded harness rule at `winddown_test.go:30-33`). *Spike gap 2,
recorded here:* only the enumeration-failure variant was forced
executably; the per-process probe-error variant (a live member whose
argv read fails transiently) is covered by code reading of this walk
alone, so the `walkComplete=false` probe-error branch rests on read
evidence, plus table row 6 in §3.6 which pins its adjudication
behavior with a scripted observation.

The group fingerprint is the deterministic string rendering of `alive`,
`walkComplete`, and the sorted pids.

### 3.2 The post-kill death wait (folds DC-PAT-003)

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
%v — motion is not progress"); sleep `observationPoll`.

Scope, per DC-PAT-003's rule: this wait is entered ONLY from the
proven-kill error branch of §3.3 — its subject is post-kill death, a
kernel event the spike measured at 58 microseconds to 5.8 milliseconds
under load 14.35 on 4 CPUs. The 30-second stall is therefore a hang
failsafe with four orders of magnitude of margin, not a speed
assertion, and — via DC-PAT-002's rule — it never adjudicates a nil
return: those never reach this wait.

### 3.3 The kill-through actor loop (folds DC-PAT-002 rule b, DC-PAT-005, DC-PAT-009)

Extract the actor loop into a helper shared with §3.8's deterministic
stays-red test:

```go
type killThroughOutcome int
const (
    killThroughGroupDown killThroughOutcome = iota
    killThroughRefusalExhausted
)

type nilReturnVerdict int
const (
    nilReturnGroupDown nilReturnVerdict = iota
    nilReturnRefusal
)

func adjudicateNilReturn(t *testing.T, pgid int) nilReturnVerdict
func runKillThrough(t *testing.T, engine *Engine, pgid int, tag string) killThroughOutcome
```

`adjudicateNilReturn`, exactly: loop from `start := time.Now()`:
`g := observeGroup(pgid)`; if `g` is down → `nilReturnGroupDown`; if
`g.walkComplete` (the group is alive with at least one substantive
member on a COMPLETE walk — by the `host.go` nil-return invariant, a
skip happened) → `nilReturnRefusal`; if elapsed ≥ `observationCeiling`
→ `t.Fatalf` ("group %d observation walk stayed incomplete for %v —
cannot adjudicate the nil return"); sleep `observationPoll`. No stall
clock: per DC-PAT-002, a refusal is detected immediately — nil return
plus ONE complete observation showing a substantive member counts one
refusal, with no 30-second wait for the fixture to launder it.

`runKillThrough`, exactly:

```go
for attempt := 1; attempt <= killThroughAttempts; attempt++ {
    err := engine.terminateGroup(pgid, tag, false)
    if err != nil {
        if !strings.Contains(err.Error(), killThroughErrText) {
            // DC-PAT-005: SIGTERM precedes the configuration-error
            // returns (host.go:120-131); any error other than the
            // kill-through text does not prove SIGKILL and must not
            // enter the patient branch, or a configuration failure
            // launders into died-late.
            t.Fatalf("wind-down failed before the kill-through window: %v", err)
        }
        // Ownership was proven and SIGKILL was sent; the product's 1s
        // kill floor expired. Under load that is slowness, not a leak,
        // unless the group then fails to die with zero progress.
        if waitGroupDown(t, pgid) == groupWentDown {
            t.Logf("kill-through reported %v but the group died late — load slowness, not a leak", err)
            return killThroughGroupDown
        }
        t.Fatalf("kill-through wind-down failed and the group made no progress for %v: %v", observationStall, err)
    }
    switch adjudicateNilReturn(t, pgid) {
    case nilReturnGroupDown:
        return killThroughGroupDown
    case nilReturnRefusal:
        // One refusal counted; retry the actor immediately.
    }
}
return killThroughRefusalExhausted
```

`TestTerminateGroupKillsThroughATermImmuneOwnedGroup` becomes: spawn
the TERM-immune tagged group as today, then

```go
if runKillThrough(t, engine, pgid, tag) == killThroughRefusalExhausted {
    t.Fatalf("group %d still holds substantive members after %d wind-down attempts whose ownership proof was refused every time — the identity-proof-refusal shape (vm-epoch lead); this red is genuine and must not be silenced",
        pgid, killThroughAttempts)
}
```

The attempt loop is the doctrine's Tier-1 shape applied to the one real
actor: K invocations of `terminateGroup` that all fail to produce a
signalled group is a genuine defect on ANY machine, regardless of how
slow each invocation is. Retrying is safe: `terminateGroup` is
idempotent against a dead or dying group (`host.go:95`), and the only
misread direction of the discriminator (a dying member transiently
reading as substantive) costs one harmless extra invocation.

### 3.4 classifyLiveWindDown rewritten as a patient observer (folds DC-PAT-004, DC-PAT-006, DC-PAT-008)

New contract (same file):

```go
const (
    liveWindDownLeaked            liveWindDownClassification = "leaked"
    liveWindDownLeakedOscillating liveWindDownClassification = "leaked-oscillating"
    liveWindDownAbandonedToCensus liveWindDownClassification = "abandoned-to-census"
    liveWindDownDiedLate          liveWindDownClassification = "died-late"
)

type windDownObservation struct {
    scan    func() census.TaggedProcessCensus
    group   func() groupObservation
    now     func() time.Time // time.Now in the real harness
    pause   func()           // time.Sleep(observationPoll) in the real harness
    stall   time.Duration    // observationStall in the real harness
    ceiling time.Duration    // observationCeiling in the real harness
}

func classifyLiveWindDown(pgid int, windDownErr error, obs windDownObservation) (liveWindDownClassification, census.TaggedProcessCensus)
```

Precondition, caller-enforced (§3.5): `windDownErr` is nil or contains
`killThroughErrText`. Any other error never reaches the classifier —
the caller fatals first (DC-PAT-005).

**The ceiling classification (folds DC-PAT-008).** Revision 1's
contract demanded oscillation wording the fixed return type could not
carry. The spike's rule: a DISTINCT classification value,
`liveWindDownLeakedOscillating`, that the caller counts as leaked and
reports with the oscillation wording. No signature violation, no lost
diagnostic. *Spike gap 4, recorded here:* DC-PAT-008 is a pure
specification contradiction; its verdict rests on reading the design
text, not on a test run — which is sufficient, since the defect was in
the specification, and this fold removes it from the specification.

**Census custody, joined kernel-first (folds DC-PAT-006 — with one
flagged deviation).** `censusHoldsGroup` is replaced:

```go
func censusHoldsGroup(observed census.TaggedProcessCensus, g groupObservation) bool
```

True iff `g.walkComplete` and `len(g.substantivePids) > 0` and at least
one pid in `g.substantivePids` appears among the PIDs of `observed`'s
signalable-universe rows (Tagged and Indeterminate together). Row PGID
values grant nothing on their own: a row whose PID is not a CURRENT
member is stale evidence and is exactly the identity-unsafe join the
spike condemned. The join is kernel-first and identity-safe by
ordering: within each classifier iteration the scan completes BEFORE
the group observation, so a member alive at observation time was alive
continuously through the scan, and a row bearing its pid named that
same process — there is no exit-and-reuse window. Revision 1's
zero-PGID `Getpgid` resolver is deleted; the `resolvePGID` field is
gone from the observation struct.

*Flagged deviation from the spike's literal rule — the loop must
adjudicate this.* The spike's rule says custody requires EVERY current
substantive member pid to appear among the scan's signalable row pids.
That quantifier is unsatisfiable on this tree: `census/tagged.go:224-226`
gives a readable-but-non-matching process (`VerificationNotOurs`) no
row at all, and every fixture group's untagged `sleep` child is such a
process — substantive to the kernel, invisible to the census by design.
Under "every", no lawful abandonment could ever classify
`abandoned-to-census` and the compression test would red on every skip.
This revision therefore folds the rule with "at least one" in place of
"every": the kernel-first direction, the completeness requirement, and
the staleness rejection — the three properties the spike's verdict
actually established structurally — are all kept; only the quantifier
is adapted to the census's own row semantics. If the loop rejects this
adaptation, the design returns to the spike for an executable custody
rule, because the literal rule is unimplementable against the current
census. *Spike gap 3, recorded here:* DC-PAT-006 was adjudicated
structurally with a scripted resolver; a physical pid exit-and-reuse
race between scan and resolve was never produced, as pid reuse cannot
be forced deterministically on that guest. The ordering argument above
is therefore a code-level proof, not a reproduced race.

**Target-restricted fingerprint (folds DC-PAT-004).** The spike proved
the whole-census fingerprint is false progress: 78 scans over 8 seconds
under churn changed it 11 times while the wedged target group's
restricted fingerprint changed 0 times. The census part of the
fingerprint is therefore the deterministic rendering of
`EnumerationError` plus only the target-relevant rows: every Tagged or
signalable-Indeterminate row whose `PGID == pgid` OR whose `PID`
appears in the same iteration's `g.substantivePids`. ("Whose pid
resolves to the target" is implemented through the iteration's own
kernel walk rather than a separate `Getpgid` call, keeping one
resolution mechanism and DC-PAT-006's identity discipline.) The
iteration fingerprint is the group fingerprint (§3.1) plus this
restricted census fingerprint; on the error path the census part is
empty.

Loop, exactly — `start := obs.now()`, `lastChange := start`, previous
fingerprint unset, `observed` zero-valued:

1. On the nil path only: `observed = obs.scan()`. The error path never
   scans and never grants census custody: an errored kill-through with
   a still-live group is only excused by actual death (preserves the
   old "kill-through error is a leak despite census visibility" rule,
   now with death-patience added).
2. `g := obs.group()`; if `g` is down (§3.1 rule) →
   `liveWindDownDiedLate`, returning `observed`.
3. On the nil path only: if `censusHoldsGroup(observed, g)` →
   `liveWindDownAbandonedToCensus`, returning `observed`.
4. Compute the iteration fingerprint; on the first iteration record it;
   on a change, `lastChange = obs.now()`.
5. If `obs.now().Sub(lastChange) >= obs.stall` → `liveWindDownLeaked`,
   returning the last `observed` (zero-value on the error path).
6. If `obs.now().Sub(start) >= obs.ceiling` →
   `liveWindDownLeakedOscillating`, returning the last `observed`.
7. `obs.pause()`; repeat.

### 3.5 TestTerminateGroupLeaksNoGroupsUnderCompression

Per cycle, replace the wait-then-classify block with:

```go
windDownErr := engine.terminateGroup(pgid, tag, false)
if windDownErr != nil && !strings.Contains(windDownErr.Error(), killThroughErrText) {
    t.Fatalf("wind-down failed before the kill-through window: %v", windDownErr) // DC-PAT-005
}
classification, observed := classifyLiveWindDown(pgid, windDownErr, realWindDownObservation(tag, pgid))
switch classification {
case liveWindDownDiedLate:
    continue
case liveWindDownAbandonedToCensus:
    abandonedToCensus++
    continue
case liveWindDownLeakedOscillating:
    leaked++
    t.Logf("group %d observation kept changing without converging for %v — motion is not progress; wind-down error %v; census enumeration error %q, tagged %d, unknown within the signalable universe %d",
        pgid, observationCeiling, windDownErr, observed.EnumerationError, len(observed.Tagged), observed.UnknownWithinUniverse())
default:
    leaked++
    t.Logf(... existing leak message, unchanged fields ...)
}
```

`realWindDownObservation(tag, pgid)` wires `scanTaggedGroup(tag)`,
`observeGroup(pgid)`, `time.Now`, `time.Sleep(observationPoll)`,
`observationStall`, `observationCeiling`. The `t.Setenv` compression
scale, the 4-cycle loop, the alternating TERM-immunity, and the final
leak assertion are unchanged.

### 3.6 The table test rewritten

`TestClassifyLiveWindDownDistinguishesLeakFromLawfulCensusHandoff`
drives the new contract with a scripted fake: `group` and `scan`
consume per-row sequences (the last element repeats when exhausted),
`now` reads a fake clock, `pause` advances it by 1 fake second, `stall`
= 5 fake seconds, `ceiling` = 50 fake seconds. Fixture pids: `pgid` =
700, leader pid 701, child pid 702; `members` denotes the group
observation `{alive: true, walkComplete: true, substantivePids: [701, 702]}`.
Scan counts below are literals derived from the §3.4 loop with this
pacing (revision 1 was docked for leaving them as derivations). Rows,
each asserting classification and consumed scan count:

1. *transient census failures clear into lawful custody* — group:
   `members` throughout; scans: enumeration error, then a census whose
   only row is a tagged `{PID: 999, PGID: 800}` (no member pid), then
   `{PID: 701, PGID: 700}` tagged signalable → abandoned-to-census, 3
   scans.
2. *a blurred leader row joined kernel-first holds custody* — group:
   `members`; one scan whose only row is Indeterminate
   `{PID: 701, PGID: 0, Universe: signalable}` — the very no-PGID row
   shape DC-PAT-006 was about, now granted through the live member pid,
   not a resolver → abandoned-to-census, 1 scan.
3. *stale row evidence grants nothing* — group:
   `{alive, complete, [702]}` frozen (the leader is gone; only the
   census-invisible child remains); scans forever: tagged
   `{PID: 701, PGID: 700}` — a row whose pid is no longer a member →
   leaked at the stall, 6 scans (iterations at fake seconds 0 through
   5).
4. *global churn is not target progress* — group: `members` frozen;
   scans alternate two censuses that differ only in irrelevant rows
   (Indeterminate `{PID: 5000}` versus `{PID: 6000}`, neither a member,
   neither `PGID` 700) → the restricted fingerprint never changes →
   leaked at the stall, 6 scans. (Under revision 1's whole-census
   fingerprint this row would have run to the ceiling — this is
   DC-PAT-004's regression pin.)
5. *oscillating target evidence hits the ceiling* — group: `members`
   frozen; scans alternate `{EnumerationError: "blur a"}` and
   `{EnumerationError: "blur b"}` (restricted fingerprint flips every
   iteration, custody never provable) → `liveWindDownLeakedOscillating`
   at the ceiling, 51 scans (iterations at fake seconds 0 through 50).
6. *an incomplete walk is uncertainty, not death* — group observations:
   `{alive: true, walkComplete: false, substantivePids: nil}`, then
   `{alive, complete, [701]}`, then `{alive: false}`; scans: empty
   censuses → died-late, 3 scans — proving the incomplete first
   observation adjudicated nothing (DC-PAT-001's regression pin).
7. *group death mid-observation is not a leak* — group: `members`, then
   `{alive: false}` → died-late, 2 scans.
8. *kill-through error is a leak only while the group lives* —
   `windDownErr` containing `killThroughErrText`, group: `members`
   frozen → leaked, 0 scans.
9. *kill-through error excused by actual death* — `windDownErr` as in
   row 8, group: `members`, then `{alive: false}` → died-late, 0 scans.

### 3.7 Cleanup handshake on every exit path (folds DC-PAT-007)

The spike disproved revision 1's handshake executably: a leader running
`sleep 600 & exit` was reaped while its group still held a live
substantive member, so the reap channel provably does not mean the
group left the table. Per the spike's rule, cleanup waits on BOTH
signals and expiry is red. `spawnTaggedGroup` changes:

```go
reaped := make(chan struct{})
go func() { _ = cmd.Wait(); close(reaped) }()
t.Cleanup(func() {
    _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
    deadline := time.Now().Add(observationStall)
    reapDone := false
    for {
        if !reapDone {
            select {
            case <-reaped:
                reapDone = true
            default:
            }
        }
        if reapDone && !groupAlive(cmd.Process.Pid) {
            return
        }
        if time.Now().After(deadline) {
            t.Errorf("group %d cleanup handshake failed after %v: reaped=%v groupAlive=%v — the group or its reaper goroutine may outlive this test into siblings",
                cmd.Process.Pid, observationStall, reapDone, groupAlive(cmd.Process.Pid))
            return
        }
        time.Sleep(observationPoll)
    }
})
```

`t.Errorf`, not `t.Logf`: an expired handshake is a red, because a
surviving group or goroutine is exactly the cross-package interference
path (the internal/supervise flap) this cleanup exists to close.
Cleanup LIFO ordering already places this before the TempDir removal
registered by the earlier `t.TempDir()` call, matching the steward
landing's ordering argument.
`TestTerminateGroupNeverSignalsAForeignGroup` keeps its existing
synchronous inline reap; it is not flapping and its must-stay-alive
assertion is correctly instantaneous.

### 3.8 The deterministic stays-red test (folds DC-PAT-009)

The spike proved the central stays-red invariant is provable with zero
fakes: the real `terminateGroup` against a tag that never matches
refuses ownership deterministically (nil return, group untouched), and
with immediate refusal detection the loop exhausted 3 of 3 attempts in
about 30 milliseconds with no laundering window. Revision 2 requires
that test:

```go
func TestTerminateGroupKillThroughRefusalExhaustionStaysRed(t *testing.T) {
    engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown-staysred"}
    tag := fmt.Sprintf("metasystem-job-staysred-%d", os.Getpid())
    cmd := spawnTaggedGroup(t, tag, false)
    pgid := cmd.Process.Pid
    if got := runKillThrough(t, engine, pgid, tag+"-not-ours"); got != killThroughRefusalExhausted {
        t.Fatalf("a group whose tag never matches must exhaust all %d wind-down attempts as ownership refusals, got outcome %d", killThroughAttempts, got)
    }
    if !groupAlive(pgid) {
        t.Fatal("a refused wind-down must not have signaled the group")
    }
}
```

This asserts, deterministically and on every run, that persistent
refusal reaches `killThroughRefusalExhausted` — the exact branch whose
laundering DC-PAT-009 showed no other specified verification could
force — and that refusal sent no signal. The §3.3 kill-through test
remains the red side of the same coin: with the REAL tag, exhaustion is
`t.Fatalf` with the refusal message.

## 4. Identity-proof refusal: patience versus masking (design question 3)

The flap is BOTH things, and the design separates them:

- **The patience defect (fixed here):** fixed 3-second windows, unpaced
  proof scans, insta-fatal on the product's 1-second kill floor, a leak
  verdict that races late death, uncertainty read as death, fixture
  expiry laundering refusals, and false census-churn progress. All are
  load-to-failure (or load-to-false-green) conversions in TEST code; §3
  removes them.
- **The identity class (kept red, sharpened):** from the code text, the
  in-verification clock-step case is already immune (paired
  StartTicks+BootID compare, `identity/identity.go:122`,
  `identity_test.go:230-252`), so the expected refusal source under load
  is transient /proc blur — which clears within the K=3 actor attempts,
  each now adjudicated immediately rather than after a 30-second
  laundering window. A refusal that does NOT clear is the real
  vm-epoch-class suspect, and the port keeps it red on purpose: the
  §3.3 exhaustion message names "ownership proof refused on every
  attempt", turning today's mute wall-clock red into a diagnosable
  specimen — and §3.8 proves deterministically, on every run, that this
  red actually fires when refusal persists. Patience here can only mask
  a bug that heals within three fresh wind-down invocations while the
  group stays otherwise healthy; such a transient is indistinguishable,
  by the product's own fail-closed design, from the /proc blur the
  product deliberately tolerates by skipping — so the residual masking
  window is the product's own contract, not a test choice.
- **Configuration failures (new, kept red):** per DC-PAT-005, a
  `terminateGroup` error other than the exact kill-through text is an
  immediate fatal in both live tests — never patient, never excusable
  by late death, because SIGTERM precedes the configuration-error
  returns and the error proves nothing about SIGKILL.
- **A separated product defect, named, not fixed here:** the census
  records identity-indeterminate processes with no PGID and no start
  identity (`census/tagged.go:219-223`, `:56-61`), so custody of a
  blurred member is unprovable by the census's own evidence. The §3.4
  kernel-first join compensates on the test side with a live-member
  fact. The product-side gap (stamp the group and start identity on
  indeterminate rows, or record why not) should be appended to the goal
  record as a follow-up lead; it is outside this design's boundary.

What stays red after the port: a kill-through-text error whose group
then makes zero progress; any other `terminateGroup` error, instantly;
K exhausted ownership refusals (proven fireable by §3.8); a live
substantive group outside kernel-first census custody with zero
target-relevant change for `observationStall`; any wait that oscillates
past `observationCeiling` (now its own classification); an expired
cleanup handshake. What is absorbed: slow death, slow zombie reap, late
death after the product's floors, transient identity blur, transient
walk incompleteness, and custody proven on any later paced scan.

## 5. Proof profile (design question 4)

Matching the steward-tick precedent (65c36111) on the 4-CPU guest:

1. Delegate-side: `go vet ./internal/missionrunner/` clean; focused run
   `go test -race -count=20 -run 'TestTerminateGroup|TestClassifyLiveWindDown' ./internal/missionrunner/`
   20/20 green — the filter now also matches §3.8's stays-red test,
   whose PASS is the deterministic refusal-exhaustion proof (DC-PAT-009).
2. Orchestrator-side (outside the sandbox, per KI-15): compile once with
   `go test -race -c ./internal/missionrunner/`, then run 8 concurrent
   instances of the binary, 20 repetitions each, filtered to the family
   (`-test.run 'TestTerminateGroup|TestClassifyLiveWindDown' -test.count=20`),
   expecting 160/160 green — 8 process-spawning instances on 4 CPUs IS
   the contention profile that exposed the original defect (the spike
   measured its verdicts at load average 14.35 built the same way). The
   spawned fixture groups of concurrent instances double as load.
3. One full-package run under that same concurrent load
   (`go test -race ./internal/missionrunner/` while the 8 instances
   run) to confirm no ballooning: patient waits return on their
   condition — the spike's measured kill-to-down latency of 58
   microseconds to 5.8 milliseconds at load 14.35 says the family's
   wall time must stay within the same order as the quiet run, not
   stretch toward the failsafes.
4. Acceptance: a red in (2)/(3) from the §3.3 refusal-exhaustion
   message under the REAL tag is a TRUE finding for the vm-epoch lead —
   record it on the goal, do not loosen the test. The §3.8 test's
   refusal exhaustion is its pass condition, not a finding. Any other
   red fails the design.

## 6. Explicitly unchanged

`terminateGroup` and every product file (including
`census/tagged.go`, whose missing-PGID defect is a named follow-up
lead, §4); `host_process_test.go` (its terminateGroup verdicts are
instant-fact checks on dead/foreign groups);
`TestTerminateGroupNeverSignalsAForeignGroup` and
`TestGroupOwnershipFixtureFallbackStaysExact`; the compression scale
env; `scanTaggedGroup`; the janitor, identity, and census packages.

## 7. Self-grade

**Grade: A-.** All nine round-1 findings are folded from executable or
structural spike verdicts, each with its implied rule written as
mechanical control flow, and all five spike gaps are recorded at their
fold sites as boundaries rather than argued away. Uncertainty can no
longer read as death (walk completeness), refusals are adjudicated
immediately against a fixture that outlives every wait, the one
remaining stall is named as the hang failsafe it is with its measured
four-orders-of-magnitude margin, progress is target-restricted, custody
is joined kernel-first, the ceiling has its own classification, cleanup
is red on expiry, and the stays-red invariant has a deterministic
zero-fake test. Docked from A for one honest reason: the DC-PAT-006
fold deviates from the spike's literal quantifier ("every" member →
"at least one" member) because the literal rule is unsatisfiable
against `census/tagged.go:224-226`; the deviation is flagged in §3.4
with its evidence and must be adjudicated by the loop, and until it is,
the custody rule rests on a code-level ordering proof rather than a
spike-executed one.

**Reject condition.** Reject this design if any of the following holds:
(a) the implementer needs any decision not mechanically fixed by §3
(that is a design gap — stop and report, never fill); (b) the
`host.go:94-182` nil-return invariant of §1, or the `host.go:120-131`
SIGTERM-before-configuration-error ordering, is contradicted by the
tree at implementation time (the refusal and error discriminators would
then be unsound); (c) the proof profile's step 3 shows the family's
loaded wall time reaching the `observationStall` failsafe on green runs
(the failsafe would then be load-sensitive, recreating the defect
class); (d) implementing it requires touching any file other than
`internal/missionrunner/winddown_test.go`; (e) the loop rejects the
flagged DC-PAT-006 quantifier adaptation in §3.4 — in that case the
design returns to the spike for an executable custody rule rather than
shipping either an unimplementable one or an unadjudicated one.
