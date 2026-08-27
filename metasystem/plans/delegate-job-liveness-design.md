# delegate-job-liveness — design record (2026-08-27; THE PARENT MAP after the round-4 split)

Working Mode: design

## THE SPLIT (RULING 6 — Wido, 2026-08-27 evening)

Round 4 rose to 12 material findings (9 shape-level) after
13→10→7 — the no-progress signal firing on one design carrying
three contract surfaces. Wido ruled the SATELLITE SPLIT, and
amended RULING 3: seam observation records live in a SIBLING
registry (canonical consumers untouched; the sweep reads both;
metrics reads seam records for coverage only). This document is now
the PARENT MAP: the rulings, the shared converged ground, and the
finding routing. Each satellite inherits without re-litigating,
carries its routed findings, and converges alone with its own
critique loop:

- SATELLITE 1 — the custodial launch machine (Lane A):
  plans/custody-launch-machine-design.md. Routed: R4-01
  (heartbeat-vs-progress), R4-02 (fingerprint/totality/atomic busy
  check), R4-07 (product-signal law for proven records in shared
  checkouts), R4-08 (group-generation identity), R4-12 (identity
  sandwich + exact tag matching). INHERITANCE REFINEMENT (S1R2-05):
  this map's D-B phrase "reservation, caps, and launch run under
  the cap-authority lock" is imprecise — the shipped implementation
  releases the lock before launch (dispatch.sh:1187); the binding
  reading is "single cap AUTHORITY, bounded lock hold", as the
  satellite states.
- SATELLITE 2 — the liveness sweep and stall episodes over proven
  records: design note to follow S1. Routed: R4-05 (episode
  reducer/CAS/terminal interplay), R4-09 (tick-failure
  independence), R4-10 (concurrent drainers), R4-11 (table
  completions incl. episode transitions; the D-F core table is
  inherited ground).
- SATELLITE 3 — the seam/observation domain on the SIBLING
  registry: design note follows S2. Routed: R4-03 (import and
  attribution transaction), R4-04 (resolved by the registry
  amendment — sibling store, no canonical-consumer migration),
  R4-06 (seam occupancy release proof).

Ordering is by dependency on truth: S1 makes the signal honest, S2
consumes it, S3 extends it to what we only observe. Slices and
appetites per satellite, each returning to Wido at its design close
(RULING 4 unchanged).

Owner: m1 coordinator. The custody arc as Wido re-scoped and
approved 2026-08-27 (high priority, ordered ahead of
actionable-metrics slices two/three). Absorbs
plans/goals-drafts/agent-liveness-contract.md on its intended
trigger (F59). Facts: plans/delegate-job-liveness-facts.md (F1..F83
at tip 3f57b04, round-1 and round-2 corrections folded — note the
F30 darwin-identity correction and the F38/F39 cap-lock amendment).

Round budget: 3 focused rounds on chain `delegate-job-liveness`
(critique-round.sh, sol/xhigh); the first budget SPENT at round 3
with findings open — EXHAUSTION #1 recorded below, successor budget
rounds 4-6 on the same chain. ROUND 1: 13 material + 2 not — five
findings decision-shaped, ruled by Wido in four rulings (Ruling 1
covers R1-01 and R1-07). ROUND 2: 10 material + 1 not — execution
gaps, all folded. ROUND 3 (failsafe): 7 material — trajectory
13→10→7; one finding forked on a semantic Wido ruled in-session
(RULING 5, at-least-once delivery), the rest are folded into this
v4. Dispositions for all rounds at the bottom.

## The four rulings (Wido, 2026-08-27 evening, in-session)

- RULING 1 (R1-01, R1-07): LANE A IS THE ONLY SANCTIONED LAUNCH
  LANE. The custodial channel (codex exec and future CLI runtimes
  under full process-proof custody) is how the metasystem launches
  delegates. The companion is DEMOTED TO OBSERVATION — never
  presented as custody, never a sanctioned dispatch path.
- RULING 2 (R1-05): SESSION-WIDE REFUSAL — a launch against a
  session with live work refuses loudly.
- RULING 3 (R1-11): THIS ARC OWNS THE REGISTRY — one canonical
  `artifacts/agents/jobs/` registry; session-coexistence re-points
  to consume it.
- RULING 4 (R1-13): APPETITE PAUSED — re-appetite at convergence;
  the critique-driver cutover is built once with the severity task.
- RULING 5 (R3-01): STALL-INCIDENT DELIVERY IS AT-LEAST-ONCE. A
  crash between delivery and the delivered-mark may duplicate ONE
  notification per episode; the episode machine bounds duplicates
  to one per crash; no notifier contract change. The design says
  this plainly wherever episodes are described.

## Threat model

Accidents, crashes, and drift — silent process death, lying status
records, double-fired launches, recycled pids, stranded records.
Two trusted operators; hostile processes and operator sabotage out
of scope.

## The shape in one paragraph

One launch channel, one registry, one sweep. Every delegate the
metasystem runs is launched through the existing dispatch/adapter
transaction extended with a custodial non-mission mode — under the
same cap-authority lock, caps, and kill ownership (never a second
launcher). Launches are idempotent by a TOTAL state machine with a
replay outcome for terminal history and reconciliation that adopts
a provable survivor. Process identity is exact per platform
(microseconds on darwin, ticks+bootID on linux) and propagates
everywhere identity is compared. What the metasystem did not launch
it OBSERVES in records whose status vocabulary NEVER intersects the
custodial terminal set — a runtime's self-report is data, not an
outcome. The steward's resident sweep applies a fully-written
liveness decision table to both record classes, tracks stall
EPISODES durably, and names every incident: job, goal, lane, stale
legs. Detection SLA: the first tick after the ceiling.

## Design decisions

- D-A Proof levels are the first law: every NEW record carries an
  immutable `proofLevel` — `proven` (custodial launches) or `seam`
  (observation records). A record WITHOUT the field is
  LEGACY-UNPROVEN (R2-10): an explicit read state, never rewritten,
  never inferred proven; consumers treat it as historical evidence
  of unknown proof. The census never classifies a process CUSTODY
  without an exact identity join; seam attribution is job-level
  reporting, never process inventory.
- D-B One launcher: the custodial path is a non-mission mode of the
  dispatch/adapter transaction — reservation, caps, and launch
  UNDER THE EXISTING CAP-AUTHORITY LOCK (R2-09; F38/F39 as
  amended), the adapter's custodial exec shape (F33-F37), the
  execution guard as today (F60). Droppable: mission provenance,
  worktrees, escalation. Never droppable: caps, the cap-authority
  lock, kill ownership. Decisions in Go; composition in shell.
- D-C Exact process identity, one representation per platform
  (R2-02): darwin uses the kernel's microsecond start
  (startedAtUnixMicro, identity_darwin.go:48); linux uses
  (ticks, bootID). The chosen representation is persisted in the
  PRIMARY ownership fields, EVERY custody entry, and compared by
  `proc alive` and the census join — the seconds fallback survives
  only for legacy records and is labeled as such. A recycled-pid
  fixture exists per representation.
- D-D The launch state machine is TOTAL (R2-05). `job claim-launch
  --opid --session <namespaced-key>` resolves in this order:
  1. SAME-OPID RESOLUTION FIRST: a standing record for the opid is
     examined before any busy gate. Live+verified (three-way: pid +
     exact start identity + instance tag) → BOUND. Terminal →
     REPLAYED-<status> (the recorded outcome returns; nothing
     launches). Reservation without identity → IN-PROGRESS: wait
     bounded (handshake deadline + grace, F17), re-read, then bind
     or hand to reconciliation. Unprovable → REFUSED-UNPROVABLE
     with the evidence.
  2. THE BUSY-SESSION GATE applies only to operations that would
     CREATE a process (RULING 2 scope). WHAT OCCUPIES A SESSION
     (R3-02, closed): a session key is BUSY iff any of its records
     is (a) a reservation — pending-setup or pending, including a
     first launch's PID-less husk; (b) live-verified custodial
     work; (c) an INDETERMINATE custodial record — indeterminacy
     never yields WON, it refuses with the indeterminacy named; or
     (d) a seam observation in `observing` or `seam-stalled` whose
     evidence is not proven-ended — an observed companion task
     occupies its session exactly like the double-fire proved.
     FREE: terminal records, reconciled-proven-absent, and
     `seam-archived`/`seam-opaque` (opaque cannot prove occupancy;
     the refusal-vs-proceed there is stated: proceed, with the
     opacity named in the launch record). Distinct-opid launch
     against a busy key → REFUSED-SESSION-BUSY.
  3. WON: reservation proceeds under the standing two-phase
     invariant (F4), persisting opid AND the namespaced session key
     atomically in the reservation.
  Reconciliation for the reservation-to-identity crash window
  (R2-04): a standing sweep scans the DETERMINISTIC instance tag
  under a complete census; exactly one matching survivor → adopt
  and stamp it; proven absence (complete census, no match) → fail
  the reservation; multiple or unknown matches → DEFER with the
  deferral event. Record-only aging alone never terminal-stamps a
  reservation.
- D-E Group-death proof matches EVERY live member (R2-03): the
  survivor gate joins TaggedSurvivors with recorded custody
  identities AND treats any live member of the recorded process
  group that matches NEITHER a tag NOR a recorded custody identity
  as indeterminacy — defeat or defer, never certain death. The
  pre-registration crash window (supervisor dies between CLI fork
  and custody-add) is a named fixture: the untagged, unregistered
  CLI child must defeat terminal-stamping.
- D-F The liveness decision table IS the design (R2-06). Inputs per
  record: RECORD CLASS (proven | seam | legacy-unproven), RECORD
  STATE (terminal/cancelling | live), PROCESS (alive-proven |
  dead-proven | indeterminate | none-recorded), PRODUCTS over
  declared roots (fresh | stale | missing-root | unreadable-root),
  SEAM HANDLES (intact | rotted; seam only), EXTERNAL EVIDENCE AGE
  (lastExternalEvidenceAt vs the ceiling). The ceiling:
  `dispatch.stall-ceiling-min`, default = watch.stale-min (20).
  Precedence, evaluated top-down, first match wins:
  1. Record terminal or cancelling → NOT SWEPT (the reaper's
     domain).
  2. proven + dead-proven → DEAD-STALE (death outranks freshness).
  3. Any declared root missing or unreadable → EVIDENCE-DEGRADED
     (a fresh sibling root never hides a broken one; the process
     state rides in the incident detail).
  4. seam + handles rotted → SEAM-OPAQUE.
  5. Any signal fresh within the ceiling (products fresh, OR
     process alive-proven with lastExternalEvidenceAt within
     ceiling, OR external evidence fresh) → HEALTHY.
  6. Everything stale past the ceiling (process alive, indeterminate,
     none-recorded, or seam) → STALLED-SUSPECTED.
  FRESHNESS NEVER READS THE RECORD FILE'S MTIME (R2-07): the sweep
  reads `lastExternalEvidenceAt`, a field only NON-SWEEP writers
  update (adapter heartbeats, custody events, product scans);
  sweep-owned patches (verdicts, episodes) never refresh it.
  TIMESTAMP SEMANTICS ARE TOTAL (R3-05): writers record EVENT time
  (a product's own mtime, a custody event's time) — never
  observation time — and updates are monotonic-max, so repeated
  scans of an unchanged product never advance freshness. On a
  NEW-format record the field is required from creation; absent or
  malformed there → EVIDENCE-DEGRADED. Legacy records (no field)
  evaluate with a read-side fallback labeled as such:
  max(product mtimes, startedAt).
  SEAM HANDLES HAVE A REDUCER (R3-07): the handles are enumerated
  per record (log file, thread/turn queryability, companion state
  readability); INTACT iff ANY evidence-bearing handle can answer
  freshness (a readable log OR a readable state record); ROTTED
  only when none can. Pinned rows: deleted log + readable state →
  intact; fresh log + lost thread → intact; unreadable state + no
  log → rotted.
  DECLARED PRODUCT ROOTS HAVE ONE OWNER (R3-03): the CALLER
  declares them at claim-launch (`--product-root`, repeatable),
  captured immutably into the record at setup (`productRoots`);
  adapters may APPEND derived roots at custody registration
  (append-only, provenance recorded, closed once running); absent
  declaration → the record's workspaceRoot is the sole default
  root.
- D-G Observation records (RULING 1) live in the canonical registry
  with `proofLevel: seam` and a status vocabulary DISJOINT from the
  custodial terminal set (R2-01): `observing | seam-stalled |
  seam-opaque | seam-archived`. THE SEAM STATE MACHINE (R3-04):
  each sweep maps its D-F verdict onto status — HEALTHY→observing;
  STALLED-SUSPECTED→seam-stalled; EVIDENCE-DEGRADED→seam-stalled
  with the degraded detail named; SEAM-OPAQUE→seam-opaque.
  Recovery is memoryless: fresher evidence returns any live seam
  state to observing on the next sweep. `seam-archived` is entered
  ONLY when selfReport is terminal AND (handles are rotted OR age
  since lastExternalEvidenceAt exceeds `dispatch.seam-archive-min`,
  default 1440); it is irreversible and counts as terminal for
  D-F row 1 (not swept). The runtime's self-report is stored under
  `selfReport` verbatim and NEVER moves `status`; no consumer may
  read seam states as completions — metrics reads seam records for
  coverage lines only until it learns the domain (a named consumer
  task in the slices). Status/result probes are read-only; no
  auto-resume; runtime cancel documented UNSAFE-ON-RECYCLED-PID and
  never automated.
- D-H Stall EPISODES are a durable state machine (R2-08): the
  record carries `stallEpisode {nonce, openedAt, state:
  recorded|delivered|cleared}`. Order: the episode is RECORDED on
  the record first, then queued with its nonce; delivery marks
  DELIVERED; a healthy verdict CLEARS the episode; a continuing
  stall never RE-QUEUES within one episode; a NEW episode (after
  clear) may. DELIVERY IS AT-LEAST-ONCE (RULING 5): the notifier's
  side effect precedes the durable mark by existing deliberate
  design, so a crash in that gap duplicates at most one
  notification per episode — recovery re-queues any
  recorded-undelivered episode, accepting the possible duplicate
  and never losing the incident. One durable episode per stall;
  at-least-once delivery per episode.
- D-I No new kill authority: probe-first intervention, then the
  timed verdict; kills stay where they live (F15); escalation
  remains steward-owned-execution's residue (F62).
- D-J Records revive the pipeline: custodial launches write real
  job records; critique rounds route through the custodial channel
  built ONCE with the severity design's driver-cutover task.
- D-K One deferral vocabulary at our seams; unification of the
  three existing group-death ladders (F26) recorded as residue.
- D-L The sweep is steward-owned and resident: rides the tick; SLA
  = first tick after the ceiling; the record scan runs goal-free
  (stranded records visible without owned work); no launch-session
  watcher is institutionalized.

## Slices (shape only — appetites return to Wido at design close)

- Slice one: custodial non-mission mode + claim-launch total state
  machine (D-D) + platform-exact identity propagation (D-C) +
  survivors completeness (D-E) + legacy proofLevel read state
  (D-A) + session-wide refusal + guard membership.
- Slice two: the steward sweep — the D-F table verbatim, seam
  observation records with the disjoint vocabulary (D-G), stall
  episodes (D-H), attribution incidents + narrator noticing +
  goal-free scan (D-L); the metrics seam-domain consumer task.
- Slice three: doctrine (probe-first, incremental-landing rule into
  docs/orchestration.md), ownershipProof Go-built patch (F10),
  driver routing with the severity cutover (D-J),
  deferral-vocabulary residue.

## Obligations (fixture-expressible)

- O1 A custodial launch writes a proofLevel-proven record with
  platform-exact identity; census CUSTODY. Killing the process has
  TWO lawful outcomes, both fixtured (R3-06): the reaper reaps
  first → the record is terminal and NOT SWEPT; the record is still
  non-terminal at sweep time → DEAD-STALE at the first tick past
  the ceiling. Neither order loses the death.
- O2 claim-launch totality: WON | IN-PROGRESS(bounded→bind/
  reconcile) | BOUND(three-way verified) | REPLAYED-<status> |
  REFUSED-SESSION-BUSY | REFUSED-UNPROVABLE — a fixture per
  outcome, including terminal-history replay and same-second pid
  reuse per platform representation.
- O3 Double-fire: one process, one record; loser binds only
  post-verification; crash windows reconcile by tag-adoption
  (exactly-one), fail on proven absence, defer on multiple —
  fixtures for all three.
- O4 A seam record reports job-level with the disjoint vocabulary;
  census inventory never CUSTODY for it; the incident names job,
  goal, lane, stale legs; metrics reads it only as coverage.
- O5 The lying-status zombie → STALLED-SUSPECTED/DEAD-STALE at the
  first tick past the ceiling; any fresh signal → HEALTHY.
- O6 Every D-F table row has a fixture, including missing root,
  unreadable root, fresh-log/stale-products, dead+fresh (DEAD-STALE
  wins), the unrelated-live-codex case, and the handle-reducer rows
  (deleted log + readable state → intact; fresh log + lost thread →
  intact; unreadable state + no log → rotted); plus timestamp
  totality (absent/malformed lastExternalEvidenceAt on a new record
  → EVIDENCE-DEGRADED; unchanged product rescanned → freshness
  unmoved; legacy fallback labeled).
- O7 Supervisor-only death and pre-registration crash both defeat
  terminal-stamping (D-E); cap expiry reaps with the existing
  verdict order; deferral events fire at our seams.
- O8 The sweep contains no kill path.
- O9 Episodes: recorded→queued→delivered→cleared; both crash orders
  reconcile; one incident per episode; a new episode after clear
  re-notifies.
- O10 Terminal honesty: custodial terminals CAS/reaper-stamped
  only; seam records never enter the custodial terminal set;
  selfReport never moves status.
- O11 Sweep-owned patches never refresh freshness
  (lastExternalEvidenceAt untouched by the sweep — fixture proves a
  stall patch does not flip the next tick to HEALTHY).
- O12 A legacy record (no proofLevel) reads LEGACY-UNPROVEN,
  is never rewritten, and never classifies CUSTODY by the seconds
  join alone.
- O13 Busy-session totality (R3-02): pending reservation → busy; a
  live-verified record → busy; indeterminate → busy (never WON);
  live seam observation → busy; terminal / proven-absent /
  seam-archived / seam-opaque → free (opacity named in the launch
  record) — a fixture per arm.
- O14 Product roots (R3-03): caller-declared roots capture
  immutably at setup; adapter additions append-only until running;
  no declaration → workspaceRoot is the root; fixtures prove
  attribution follows the declared set.
- O15 Seam state machine (R3-04): verdict→status mapping fixtures,
  memoryless recovery to observing, seam-archived only under its
  entry rule and irreversible.

## Round 1 dispositions (r1-output.md)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| R1-01 | accepted, RULED | Companion cannot carry process proof. | RULING 1; D-G |
| R1-02 | accepted | Untagged CLI + tag-only survivors can lie. | D-E; O7 |
| R1-03 | accepted | supplementWorkers cannot attribute. | D-A/D-G; O4 |
| R1-04 | accepted | No state machine, no reconciliation. | D-D; O2/O3 |
| R1-05 | accepted, RULED | Ruling text: busy sessions refuse. | RULING 2; D-D step 2 |
| R1-06 | accepted | No decision table; companion logs deleted. | D-F table in the design; O6 |
| R1-07 | accepted, RULED | Lane B cannot prove terminals — as observation it never claims them. | RULING 1; D-G; O10 |
| R1-08 | accepted | Probes read-only; cancel unsafe. | D-G; D-I |
| R1-09 | accepted | Second launcher forks cap/kill authority. | D-B; F38/F39 amended (R2-09) |
| R1-10 | accepted | pid+seconds not recycle-proof. | D-C (superseded by R2-02's platform law) |
| R1-11 | accepted, RULED | One canonical registry. | RULING 3; D-A/D-G |
| R1-12 | accepted | Tick SLA, goal-free scan, persistence. | D-L; D-H |
| R1-13 | accepted, RULED | Appetite unsupported. | RULING 4; slices shape-only |
| R1-14 | not-material, corrected | Hash 99ab15a → 782d7bc. | Facts |
| R1-15 | not-material, corrected | RecordProtocolError also moves status. | F6 |

## Round 2 dispositions (r2-output.md)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| R2-01 | accepted | Seam agree-then-terminal would corrupt metrics (data.go:24, compute.go:81 read any terminal). | D-G disjoint vocabulary; metrics consumer task named; O4/O10 |
| R2-02 | accepted | Darwin emits ticks 0/bootID '-' → seconds join; custody stores seconds; census discards the pair. Kernel has microseconds. | D-C platform-exact identity, propagated; F30 corrected; O2 per-platform fixtures |
| R2-03 | accepted | Fork-to-registration window: readable untagged group member + certain absence = lie. | D-E unmatched-member indeterminacy; O7 |
| R2-04 | accepted | reap-facts is record-only; aging alone lies, refusing forever strands. | D-D reconciliation: tag-adoption / proven-absence / defer |
| R2-05 | accepted | BOUND vs session-refusal contradiction; no terminal outcome; no session key in the interface. | D-D total ordering: same-opid first, REPLAYED-<status>, busy gate only for process-creating ops, session key persisted |
| R2-06 | accepted | The table was delegated to the implementer. | D-F precedence table written in the design; ceiling key named; handle axis added; O6 |
| R2-07 | accepted | Record mtime + sweep patches = freshness loop (record.go:341). | lastExternalEvidenceAt; O11 |
| R2-08 | accepted | Nonce dedup dies with delivery; no atomic join. | D-H episode state machine; O9 |
| R2-09 | accepted | F38/F39 contradicted the accepted cap-lock fold. | Facts amended: cap-authority lock never droppable |
| R2-10 | accepted | 241 legacy records lack proofLevel; terminal allowlist excludes it. | D-A legacy-unproven read state; O12; migration policy = none (read-side only) |
| R2-11 | not-material, corrected | Ruling count wording. | Header: five findings, four rulings |

Round-2 side note: the critic hit the goal-next stderr-contamination
defect again — the third sighting (and a fourth in round 3); the
backlogged goal-git-stderr-pollution goal carries the evidence.

## Round 3 dispositions (r3-output.md) — and CRITIQUE EXHAUSTION #1

The first three-round budget closed at the declared failsafe with
seven material findings open; all seven are dispositioned below
(one ruled, six folded), and the successor budget (rounds 4-6)
opens on the same chain per the round-budget law. A second
exhaustion stops the design on the human.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| R3-01 | accepted, RULED | notify.go:70 deliberately re-delivers on crash; the fork is semantic. Wido: at-least-once. | RULING 5; D-H |
| R3-02 | accepted | "Live recorded work" was not total; a pending husk or a lying record broke both readings. | D-D busy-session totality; O13 |
| R3-03 | accepted | No owner produced the declared roots. | Caller declares at claim-launch; capture at setup; adapter append-only; workspaceRoot default (D-F); O14 |
| R3-04 | accepted | Verdicts and statuses were two vocabularies with no mapping or archival rule. | D-G seam state machine + archive entry rule + row-1 terminality; O15 |
| R3-05 | accepted | Absent/malformed timestamps satisfied neither side of the ceiling; scan-time writes would freshen unchanged products. | D-F timestamp totality (event time, monotonic-max, legacy fallback labeled); O6 |
| R3-06 | accepted | The reaper legitimately terminals a dead running record before any sweep. | O1 two lawful outcomes, both fixtured |
| R3-07 | accepted | One binary hid a second outcome table. | D-F handle reducer with pinned rows; O6 |
