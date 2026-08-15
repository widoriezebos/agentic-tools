# The monitor facility: tracked long-running work with terminal-state watching

Working Mode: design

Owner: main session (delegate), goal monitor-facility (detail notes
at plans/backlog-notes.md item 15). Status: IMPLEMENTED (2026-08-15;
converged via the FIXTURES-AS-ARBITER EXIT (critiques at
plans/monitor-facility-critique-r{1..6}.md; trajectory
12/10/8/6/6/4; the r6 critic noted the second three-round critique
budget exhausted). The exit's ratified conditions all hold: the
trajectory is falling past its budget; every r6 finding is
mechanical-grain (an incarnation discriminator, a waiter target
field, a canonical human key, a scan-consistency rule + one honest
promise weakening) with no invariant-grade finding; each is folded
1:1 below AND as a NAMED fixture obligation (FIX-R6-01..04 in the
matrix); code-critique of the implementation is MANDATORY, not a
judgement call; and this header records the switch, as the
flight-recorder instance did. Recorded as D72 for the human's
review.
r6 pins: the kinship predicate in pgid terms only with
adopted-unverified never signaled; provisionalVerdict frozen at
draining entry with endedAt as the wind-down clock; owner-keyed
waiter records so a foreign waiter neither satisfies nor blocks
anyone, with disjoint operational exit codes; the tagged cross-kind
digest encoding; MON-07 gated on the corrected attestation
contract with freshness from the armed state's loaded intervalSec;
and green exactly-once on a lock-assigned terminal sequence. r5 splits Watched
into Supervised and WaiterLive, adds typed JobFacts and pins
unwatched-before-Busy, keys the attestation on full lifecycle
triples with a named freshness bound, totalizes the draining
transition across modes with a provisional verdict and defines the
adopted-verified kinship predicate, makes waiter registration
exclusive-by-liveness with pinned exit codes, routes the event rows
through the real string-payload emitter with full-row conformance
and a CAS-refusal event, and replaces the green FIFO with a
monotonic cursor. Human rulings fixed as input: the
exchangeability doctrine, the verbatim monitor-pattern intent, and
backlog item 1's literal waiter contract.

## The problem, in the human's words and two incidents

A detached suite run is invisible to the metasystem: the census
flags it UNTRACKED, no record says what it is or what should happen
when it ends, and only one runtime's harness knows to care. The
harness's 10-minute cap forced detached launches; a monitor caught
a 205-second failure a fallback timer would have slept 15 minutes
on. The pattern works — it must become metasystem behavior.

## One mechanism, two record kinds, ONE waiter shape (r3 finding 1)

Backlog item 1 folds in as written, and runs get the SAME waiter:

- `dispatch.sh dispatch` without `--wait` prints the exact waiter
  command: `scripts/agents/dispatch.sh watch --job <id>`.
- `run launch` prints ITS exact waiter command the same way:
  `bin/metasystem run watch --id <run-id>`.
- Both watch verbs BLOCK until their record is terminal and exit
  with the terminal status — the shape every runtime's background
  facility turns into a wake-up. The wake is the waiter deciding
  from the record and kernel state; flight-recorder events narrate
  and are never the wake authority (their contract permits lost
  finals).
- Every waiter REGISTERS ITSELF, keyed by OWNER (r5 finding 3 — a
  foreign waiter must neither satisfy your unwatched rule nor block
  your own watching):
  `artifacts/agents/waiters/<kind>-<id>-<owner-digest>.json`
  {kind: "job"|"run", pid, pidStartedAt, session, mainId,
  target: {startedAt} for jobs | {generation, launchNonce} for
  runs} where owner-digest is sha256(mainId, or "human:"+uid for
  HUMAN callers — the OS user id is the canonical stable human key,
  r6 finding 3: classification gives humans no session and the
  printed command carries none; one waiter per human user per work,
  documented) truncated 12 hex. WaiterLive requires the TARGET to
  match the work's current lifecycle (r6 finding 2: a live waiter
  from a dead lifecycle must neither satisfy WaiterLive nor hold
  the slot — a mismatched-target live waiter is replaceable by the
  same identity-checked compare-and-delete, since its own watch is
  about to exit on the terminal record it is actually reading). Identity-verified at write, removed on exit, provably dead
  otherwise. Exclusive PER KEY by liveness: registration refuses
  while a live record holds the key; a dead owner is replaced by
  identity-checked compare-and-delete under a bounded flock.
  WaiterLive is therefore an OWNER-CORRELATED fact: the unwatched
  rule counts only a live waiter whose mainId equals the work
  owner's. Multiple owners may watch the same work; nobody's waiter
  suppresses anybody else's block.
- `run watch` exit mappings, pinned like the job waiter's:
  outcomes green=0, red=1, ended-unknown=2, launch-failed=3, record
  missing or malformed=4; OPERATIONAL failures are disjoint from
  outcomes — live same-owner waiter already registered=64, waiter
  lock timeout or write failure=65, waiter record with unknown
  identity=66. Non-waiting FOLLOW-UPS print their waiter command
  exactly as the initial dispatch does.
- The turn-end rule spans both kinds: an in-flight job whose record
  mainId equals the CALLER's mainId, or a launching/running/
  draining run this mainId registered, with no LIVE waiter record,
  blocks the turn end once. `report turn-verdict` gains `--main-id`
  (the hook already computes main_id); TurnVerdict joins waiters
  internally.

Run records exist only for NON-JOB work; jobs keep their one
lifecycle. The goal system carries WHAT next; this carries WHEN;
the verdict is where both speak.

## The run record: artifacts/agents/runs/<run-id>.json

Engine-written. ALL mutations — launch, register, adopt, ack,
conclude, prune — hold the runs-directory lock
(`artifacts/agents/runs/.lock`, bounded flock) for the WHOLE
operation: this is the operation-spanning fence, run-owned because
`lease run-held` deliberately does not fence HUMAN callers (r3
finding 3 — the lease contract is not changed; the bypass is why
this lock exists). Lease authority stays the point-in-time check
plus an epoch recheck INSIDE the lock. Per-record CAS is over
(status, generation). Schema (v1):

    {"schemaVersion": 1,
     "runId": "<kebab, unique among live records, ≤64>",
     "kind": "suite" | "cohort" | "custom",
     "display": "<from --display only, ≤200>",
     "custody": "wrapped" | "adopted-verified" | "adopted-unverified",
     "generation": <int, starts 1>,
     "pid": <int|null>, "pidStartedAt": <int|null>, "pgid": <int|null>,
     "launchNonce": "<32 hex>",
     "log": "<path ≤512; absolute or repo-relative, resolved at
             bind; contained to the repo or /tmp; symlinks resolved
             at bind and re-resolved per read, mismatch surfaces>",
     "startedAt": "<ISO>",
     "mainId": "<str|null>", "ownerLineage": "<str|null>",
     "claimEpoch": <int|null>,       // null for HUMAN callers: human
                                      // runs have no epoch, swept only
                                      // by identity-death
     "sessionId": "<normalized>", "goalId": "<informational>",
     "staleAfterMin": <int 1..1440>, "hungSince": "<ISO|null>",
     "windDownMin": <int 1..120, default 10>,
     "evidence": {"mode": "exit-sidecar" | "pattern" | "none",
                  "verdictPattern": "<RE2 ≤256, adopted only>"},
     "expect": {"green": "≤240", "red": "≤240",
                "hung": "≤240", "unknown": "≤240"},
     "status": "launching" | "running" | "draining"
              | "green" | "red" | "ended-unknown" | "launch-failed",
     "provisionalVerdict": "green"|"red"|"ended-unknown"|null,
                                      // frozen at draining ENTRY:
                                      // adopted-pattern evidence is
                                      // evaluated once there, so
                                      // descendants writing the log
                                      // later cannot change it
     "terminalSeq": <int|null>,       // assigned under the runs lock
                                      // at terminalization: the total
                                      // order the green cursor rides
     "acked": false, "error": "<str|null>",
     "exitCode": <int|null>,
     "endedAt": "<ISO|null>"}          // stamped at draining entry (the
                                       // leader's death IS the run's
                                       // end; the wind-down clock runs
                                       // from it; terminalization only
                                       // finalizes status)

Legal transitions (CAS refusals loud):
launching → running (wrapper binds via nonce);
launching → launch-failed (launcher error path or the 2-minute
fence — an error note, never deletion; prune ages failures);
running → draining in EVERY mode whenever the leader is dead and
the recorded group is non-empty (r4 finding 3: adopted runs have no
sidecar, and a dead evidence-less leader with survivors must not
shed them) — the draining record carries its PROVISIONAL verdict
(from evidence if present, else ended-unknown);
running → green | red | ended-unknown DIRECTLY only when the group
is provably empty at leader death (evidence decides which; no
evidence means ended-unknown — the one dead-no-evidence verdict);
draining → green | red | ended-unknown (the provisional verdict
finalizes when the group provably empties or windDownMin expires;
survivors past the wind-down surface as UNTRACKED — honest);
adopt: RUNNING record only, and ONLY when the old generation's
leader is provably dead AND its recorded group provably empty
(refuse otherwise); identity replaced, generation++, hungSince
cleared. ADOPTED-VERIFIED'S PREDICATE, exactly and in pgid terms only (r5
finding 1 — "session leader" was ambiguous and no session-id
primitive exists): there exists a live process P with P.pgid equal
to the target's pgid such that P is an ancestor of the caller or
the caller is an ancestor of P (the ParentPid walk both ways).
Provable process-group kinship at registration time; anything less
is adopted-unverified, honestly labeled — and adopted-unverified is
never signaled by anyone.

## Custody, stated honestly PER MODE (r3 finding 2)

The universal three-factor rule was false; each mode carries the
strongest proof it can actually have:

- WRAPPED, launching/running: pgid match + leader pid/start + the
  launch nonce visible in the leader's argv (`run wrap --nonce`).
  Three factors, leader alive. ArgvKnown=false surfaces, never
  proves or condemns.
- DRAINING (any mode): the leader is dead by definition — draining
  IS the dead-leader-with-survivors state (for wrapped runs its
  last act was the sidecar; adopted and evidence-less runs enter
  with a provisional ended-unknown). Custody = pgid + the record's claim,
  bounded by windDownMin. The pgid-reuse window during wind-down is
  a NAMED ACCEPTED RESIDUAL (≤120 minutes by bound, 10 by default);
  census labels these "RUN <id> (draining)".
- ADOPTED: no wrapper exists, so no argv nonce can — two factors
  (pgid + leader pid/start), plus the ancestry/group-membership
  proof at registration for adopted-verified; adopted-unverified is
  the honest label when even that is absent. Evidence mode for
  adopted records is pattern|none only.

Census owns launching, running, and draining records; terminal
records own nothing.

## Exit evidence

The wrapper's last act: `runs/<run-id>.g<generation>.exit.json`
atomic, {"runId","generation","nonce","exitCode","endedAt"} —
believed only when nonce AND generation match. exit 0 → green;
nonzero (incl. 128+n) → red. The `.g<n>.exit.json` suffix is
excluded from the record glob by filename grammar. verdictPattern
(adopted only): RE2 ≤256 over a 64 KiB tail; match → green;
no-match → ended-unknown, never a red guess; unreadable/missing log
at conclusion → ended-unknown + the path surfaces. Unknown identity
concludes nothing. Prune removes a record's sidecars with it; id
reuse after prune is legal — every once-only key below uses the
LIFECYCLE identity (id, generation, launchNonce), never the public
id alone (r3 finding 6).

## The watcher and the attestation that cannot be faked (r3 finding 4)

The run pass lives in the Go WatcherPass armed supervision
executes. Each pass: three-way identity per live record; the launch
fence; evidence on death; the drain check; hungSince by log mtime;
one event per transition. After each SUCCESSFUL pass the watcher
atomically writes `artifacts/agents/supervision/runs-pass.json`:
{completedAt, watcherPid, watcherStart, scannedRuns: [{id,
generation, launchNonce}]} — the FULL lifecycle triple (r4 finding
2: id reuse is legal and generations restart, so (id,generation)
alone can bless a lifecycle never scanned; the dispatch attestation
precedent binds to exact identity for the same reason). The
verdict's Supervised fact requires ALL of: the run's complete
triple in scannedRuns; completedAt within the NAMED bound (twice
the watcher interval, from the same config the component reads);
AND the attesting watcher identity equal to the currently armed
watcher component's recorded identity AND that process alive — a
one-shot `supervise watcher-pass` can never fake a standing
watcher.

## Verbs and authority

- launch / register / adopt / ack / prune — holder-only (HUMAN
  passes with nullable coordinates); all under the runs lock.
- conclude — record-writer (the watcher; holders may); evidence
  rules only; under the runs lock.
- watch — open; blocks, registers its waiter record, exits with the
  terminal status.
- status / list — open reads; {"schemaVersion":1,"runs":[...]}
  sorted by startedAt.
- prune — acked terminal >14 days, drops reported.

Takeover (stale claimEpoch, running/draining): with the per-mode
proof above — SIGTERM the group, bounded drain, conclude
ended-unknown unacked. Without proof (including every
adopted-unverified record): refuse loudly, surface, never signal.

## Scanner and verdict (r3 finding 5; r4 finding 1 splits Watched)

WATCHED WAS TWO FACTS WEARING ONE NAME. They separate:
- `Supervised`: the standing watcher's attestation covers this
  lifecycle (it cannot wake anyone; it proves scanning).
- `WaiterLive`: a live identity-verified waiter record exists (it
  wakes a session; it proves nothing about scanning).
The UNWATCHED block requires WaiterLive for the caller's work; the
warning "supervision is not scanning your runs" keys on Supervised
separately. ScanResult gains `Jobs []JobFact{Id, MainId, Status,
WaiterLive}` (typed — the Busy reduction discards mainId, so job
facts get their own field) and `Runs []RunFact{Id, Generation,
Nonce, Status, ProbeState, OwnedByCaller, Supervised, WaiterLive,
Acked, HungSince, Continuation ≤240}`. PRECEDENCE PINNED: the
unwatched-own-work block is evaluated BEFORE the Busy suppression
(a watched active run counts as Busy; an unwatched one blocks
despite being busy — that is the point). ScanResult also gains
`RunUnreadable []string` — run-reader failures ride
their own channel OUTSIDE the shipped ladder (the ladder's
Unreadable keeps its exact shipped semantics, including Busy
suppression; run failures must never be hidden by Busy, so they do
not share its slot).

- Run WARNINGS, always prepended to Display: terminal-unacked in
  {red, ended-unknown, launch-failed}, currently hung,
  unknown-identity, and every RunUnreadable entry — continuation
  verbatim (launch-failed and ended-unknown speak expect.unknown).
- GREEN terminal surfaces once per session via the MONOTONIC
  TERMINAL SEQUENCE with PREFIX-CONSISTENT observation (r6 finding
  4): terminalization assigns terminalSeq from a counter file under
  the runs lock; the VERDICT re-reads terminal run records inside
  its own state flock (runs are few; the re-read is cheap), so no
  scan can see seq 11 without having seen seq 10; an UNREADABLE run
  record freezes cursor advancement entirely for that turn. The
  promise, stated honestly: exactly-once per RETAINED session state
  — state eviction (30 days / 128 sessions) or a malformed-state
  reset may replay greens, the same documented replay class as
  every block-once slot in the item-14 state file.
- The UNWATCHED block covers launching, running, AND draining work
  owned by the caller (jobs via mainId + runs), keyed on the sha256
  of the sorted TAGGED lifecycle strings — "job:<jobId>@<the job
  record's own startedAt>" and "run:<id>.g<generation>.<nonce>"
  (r5 finding 4 pinned the tags; r6 finding 1: job ids are reusable
  after evidence GC, so the record's immutable startedAt is the
  incarnation discriminator) —
  blockedUnwatchedDigests, ≤16 FIFO, additive to the item-14 state
  schema.
- Required mixed-state display tests: Busy+RunUnreadable,
  Open+RunRed, Busy+RunHung — both parts visible in each.

## Events (r3 finding 7 — the rows in full, per the registry grammar)

Component `run`; authorized emitters: the run verbs and the watcher
(both emit as component "run"); runId joins the canonical
identifier set. The real emitter carries map[string]string and
validates only name+emitter (r4 finding 5), so the rows declare
STRING payloads and the conformance obligation validates FULL rows
against the registry grammar, not mere emission. Rows:
- run-launched — required: runId (identifier), kind (string, enum
  suite|cohort|custom), custody (string, enum wrapped|
  adopted-verified|adopted-unverified). Emitter: run verbs.
- run-transition — required: runId (identifier), from (string,
  status enum), to (string, status enum), generation (string,
  decimal int). Emitters: run verbs, watcher.
- run-swept — required: runId (identifier), reason (string).
  Emitter: component run — the run package emits it when the lease
  sweep invokes it (ONE emitter; the sweep itself keeps emitting
  its own lease events as component lease, unchanged).
- run-cas-refused — required: runId (identifier), expected
  (string), found (string). Emitters: run verbs, watcher — the
  serialization refusals are evidence, not silence.

## Exchangeability

Files and engine verbs only: any runtime's session prints and runs
the same waiter commands, arms the same supervision, reads
`run status` by instruction, and receives the same verdict through
the same hook contract.

## Blast radius

internal/run (NEW), internal/dispatch (job watch decision core +
waiter records), scripts/agents/dispatch.sh (watch plumbing + the
printed line), cmd/metasystem supervise_component.go (run pass +
attestation), internal/census (per-mode RUN custody),
internal/goal (RunFacts, RunUnreadable, --main-id, two state
arrays), internal/report scan.go, internal/lease (run sweep),
internal/events + scripts/agents/event-registry.json,
cmd/metasystem (run family + watch verbs), fixtures, docs.

## Design-obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| MON-01 | CRITICAL | Launch topology | Pending-before-process with nullable identity; nonce CAS bind; fence and error path conclude launch-failed with a note, never deletion | internal/run | internal/run/run.go | run_test.go TestLaunchReservationAndFence | supervision-fixtures.sh S4-16 launches a real wrapped run; live smoke 2026-08-15 launch-wrap-watch round-trip | DONE | — |
| MON-02 | CRITICAL | Evidence | Generation-scoped nonce-checked sidecars; pattern only on adopted, evaluated ONCE at draining entry into provisionalVerdict; dead+no-evidence=ended-unknown; endedAt stamps at draining entry; Unknown concludes nothing | internal/run | run.go Conclude | run_test.go TestConcludeEvidenceTable + TestDrainingFreezesVerdict | run_test.go drives sidecar green/red, forged and stale-generation sidecars, unknown against real files | DONE | — |
| MON-03 | CRITICAL | Custody per mode | Wrapped three-factor; draining pgid+claim bounded (named residual); adopted two-factor with registration proof; census labels draining; survivors surface after wind-down | internal/census + internal/run | internal/census/run.go loadRunOwners | run_owner_test.go TestRunGroupCustody | run_owner_test.go TestRunGroupCustody with fixture processes; supervision-fixtures.sh S4-16 sandbox census | DONE | — |
| MON-04 | CRITICAL | The waiter contract | dispatch AND run launch print their exact watch commands; watch verbs block to terminal with the pinned outcome and operational exit codes; waiter records are owner-keyed, exclusive per key by liveness, replaced only by identity-checked compare-and-delete; WaiterLive is owner-correlated | internal/dispatch + internal/run + dispatch.sh | internal/dispatch/watch.go + internal/run/waiter.go | watch_test.go TestJobWatchRoundTrip + run_test.go TestWaiterContract | dispatch-fixtures.sh watch leg (printed line + completed-job exit 0) + supervision-fixtures.sh S4-16 run watch | DONE | — |
| MON-05 | CRITICAL | Turn verdict | Unwatched (launching/running/draining, jobs+runs, by mainId) blocks once on lifecycle-triple digests; warnings incl. launch-failed and RunUnreadable always prepend; green once per session | internal/goal + internal/report | turnverdict.go + scan.go | turnverdict_test.go TestUnwatchedAndWarnings + TestHumanOwnsHumanRuns + scan_test.go TestMonitorFacts | supervision-fixtures.sh S4-16: unwatched block once, STILL WORKING with the run, green surfaced exactly once through the real hook | DONE | — |
| MON-06 | CRITICAL | Serialization + lease | The runs lock spans every mutation (the HUMAN run-held bypass is why); CAS over (status,generation); adopt requires the old generation provably dead; sweep signals only with per-mode proof | internal/run + internal/lease | run.go + sweep.go | run_test.go generation fencing + sweep_run_test.go TestRunSweepProofOrRefuse | sweep_run_test.go TestRunSweepProofOrRefuse: unproven stale run refuses loudly, record untouched | DONE | — |
| MON-07 | HIGH | Attestation | runs-pass.json carries watcher identity + full scanned lifecycle triples (id, generation, launchNonce); Supervised requires triple membership, freshness within twice the armed state's loaded intervalSec, armed-component identity match, AND that watcher process alive | cmd/metasystem + internal/run + internal/goal | supervise_component.go + turnverdict.go | scan_test.go TestMonitorFacts attestation rules | run_conformance_test.go proves the events; scan_test.go TestMonitorFacts proves a dead watcher's attestation is believed by no one | DONE | — |
| MON-08 | HIGH | Authority | Holder-only mutations with nullable HUMAN coordinates; conclude record-writer; custody labels honest and evented | internal/run + cmd/metasystem | run.go + cmd wiring | run_test.go TestHumanCoordinatesNullable + TestVerbRefusalsAndHelpers | run_test.go TestHumanCoordinatesNullable + the CLI authority path exercised by S4-16's human-classed fixture shell | DONE | — |
| MON-09 | HIGH | Events | The three rows as specified (fields, requiredness, emitters); runId canonical; conformance test proves acceptance | internal/events + scripts/agents/event-registry.json | emit.go + registry | run_conformance_test.go TestRunEventConformance | run_conformance_test.go emits all four rows into the closed catalogue and reads them back | DONE | — |
| MON-10 | MEDIUM | Ledger honesty | prune acked-terminal >14d with sidecars, drops reported | internal/run | run.go Prune | run_test.go TestPruneReportsDrops | run_test.go TestPruneReportsDrops removes sidecars with records against real files | DONE | — |
| FIX-R6-01 | HIGH | Arbiter exit r6-1 | The unwatched block re-arms for a reused job id: the digest keys on jobId@startedAt | internal/goal | turnverdict.go | turnverdict_test.go TestUnwatchedAndWarnings | turnverdict_test.go TestUnwatchedAndWarnings: same id different startedAt re-arms | DONE | — |
| FIX-R6-02 | HIGH | Arbiter exit r6-2 | A live waiter whose target mismatches the current lifecycle neither satisfies WaiterLive nor blocks re-registration | internal/run + internal/dispatch | internal/run/waiter.go RegisterWaiter | run_test.go TestWaiterContract | run_test.go TestWaiterContract: stale-lifecycle live waiter replaced, same-lifecycle refuses busy | DONE | — |
| FIX-R6-03 | HIGH | Arbiter exit r6-3 | HUMAN waiters key on human:<uid>; the printed command works with no session plumbing | internal/run + internal/dispatch | internal/run/run.go OwnerDigest | run_test.go TestWaiterContract | run_test.go TestWaiterContract: human waiter keyed on the OS user id, no session plumbing | DONE | — |
| FIX-R6-04 | HIGH | Arbiter exit r6-4 | Greens surface in terminalSeq order under the verdict's flock; an unreadable run record freezes the cursor; replay-on-state-loss is the documented class | internal/goal + internal/run | turnverdict.go re-read | turnverdict_test.go TestGreenPrefixConsistency | turnverdict_test.go TestGreenPrefixConsistency: freeze under unreadable, ordered surfacing, no repeats | DONE | — |
| MON-11 | MEDIUM | Grammar | Every bound at the source incl. windDownMin 1..120 and the log containment/symlink rule; sidecar suffix excluded from the record glob; status shape pinned | internal/run | run.go validation | run_test.go TestBounds | run_test.go TestBounds refuses every bound at the source | DONE | — |

## Code-critique fold (mandatory review, 2026-08-15)

The arbiter exit made a code critique of the implementation MANDATORY.
It ran against the gated tree (Mac 04a1bbe, VM d50909a) and returned
REVISE with 11 material findings — 4 critical — every one real, every
one folded with a test. The critique is preserved verbatim at
plans/monitor-facility-code-critique.md.

| Finding | Fix | Proof |
| --- | --- | --- |
| 1 CRITICAL sweep unserialized, one proof rule, no forced conclusion | Store.SweepStale under the runs lock: wrapped-only signaling, loud refusal on other custody, 5s bounded drain, forced ended-unknown; lease side thinned to seams | sweep_run_test.go TestRunSweepProofOrRefuse |
| 2 CRITICAL no in-lock epoch recheck | Store.CurrentEpoch seam checked inside every mutation's lock; CLI always wires it via lease | run_test.go TestEpochSeamRefusesStale |
| 3 CRITICAL green cursor can skip a green; counter resets when malformed | decideGreens re-reads records from disk inside the verdict flock; unreadable freezes; nextTerminalSeq refuses a malformed counter | turnverdict_test.go TestGreenCursorRereadsDisk |
| 4 CRITICAL waiters miss timeout/cancelled; no lifecycle compare; remove by pid only | JobWatch full terminal map 0/1/2/3, unknown=4; run Watch compares generation+nonce each poll; RemoveWaiter compares pid AND start | watch_test.go + run_test.go TestWaiterContract |
| 5 HIGH Supervised not tied to the armed watcher | readRunsPass loads state.json: armed identity match, 2x loaded intervalSec, future stamps rejected; verdict warns "supervision is not scanning run" | scan_test.go TestMonitorFacts armed-state legs |
| 6 HIGH draining custody unbounded | census skips draining owners past endedAt+windDownMin | run_owner_test.go expired-drain leg |
| 7 HIGH symlinked log redirects pattern evidence | resolveLog resolves the FILE; separator-safe containment; conclusion re-resolves and requires equality | run_test.go TestPatternEvidenceSymlinkSwap |
| 8 HIGH bind/adopt/emptiness proofs not preserved | Bind verifies kernel pgid; Adopt recomputes custody with downgrade; groupEmpty treats non-ESRCH errors as unprovable | run_test.go kinship legs |
| 9 HIGH invalid records treated as valid | Read runs Validate; List surfaces failures and sorts by startedAt; assessRunning nil-identity guard | run_test.go + census/report fixture legs |
| 10 HIGH launch failures never become launch-failed; fenced records unprunable | FailLaunch on every post-reserve error path; terminalize stamps endedAt | run_test.go TestLaunchReservationAndFence |
| 11 MEDIUM registry grammar wrong; conformance test name-only; cmd emitter identityless | flat runId ids + enum payload types; payload-field assertions; cmd emitter self-probes pidStartedAt; sweep emits as run | run_conformance_test.go |
