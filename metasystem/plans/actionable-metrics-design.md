# actionable-metrics — design record (2026-08-27)

Working Mode: design

Owner: m1 coordinator. Goal `actionable-metrics` (6h across three
slices; this design opens slice one, 2h + the ruled provenance
delta). Ratified brief: the metric family from Wido's 2026-08-25
rulings on the `continuous-self-improvement` goal (tips
5959cc8/bf957fb) — plain human names, and NO METRIC SHIPS WITHOUT
ITS PAIRED ACTION AND OWNER. Metrics are machinery that is watched
and acted on, never dashboard decoration.

Round budget: 3 focused rounds on one chain (`actionable-metrics`,
critique-round.sh, gpt-5.6-sol at xhigh). Failsafe round 3 was
declared at loop start. ROUND 1: 16 material + 1 not-material; 15
accepted and folded, R1-13 part-refuted on the ratified brief's own
words; R1-01 escalated and RULED (forward-only provenance). ROUND 2:
15 material + 1 out-of-scope observation; the round-1 join passed;
ALL 15 are mechanical-grain and fixture-expressible, so THE DECLARED
EARLY-CLOSE FIRED AT ROUND 2: every finding is folded one-to-one
below and into the obligation rows, round 3 goes unused, and
code-critique of the implementation is MANDATORY — the fixtures and
the code critic together are the arbiter prose review stopped being.

Threat model: accidents and drift, not adversaries — metrics that
silently compute wrong numbers, read rotted inputs, fire on noise, or
ship without a consumer. Two trusted operators; hostile input is out
of scope.

## The attribution rulings (R1-01 + R2-04 — Wido, 2026-08-27)

The ratified sentence "all inputs exist as records today (jobs/,
landings, goal history)" was half-true: the RECORDS exist, the
ATTRIBUTION does not. No job record carries a goal id (all 241
current records have empty goal/mission/stream; the serving-goal
projection is brief-context only, internal/dispatch/servinggoal.go:9),
receipts carry no work-item, builder, or commit id, critique chains
are named freely, and landings only name their lander.

WIDO RULED: FORWARD-ONLY PROVENANCE. Three small additive contract
changes land inside slice one:

- Job records gain an optional `goalId`, declared by an explicit
  `--goal` argument at reservation. IT SURVIVES THE LIFECYCLE
  (R2-02): record-setup's full-record constructor carries it across
  the pending-setup replacement (internal/dispatch/record.go:200,
  build.go:86) and follow-up records inherit it from their root
  (build.go:320); the fixture proves it reaches terminal and
  follow-up records.
- RECEIPT rows gain an optional `goal=` key (additive to the F11
  grammar; absent on old rows).
- RECEIPT rows gain an optional `built_by=` key, values
  `coordinator | delegate | mixed` (RULED on R2-04): the builder
  fact, distinct from `delegate=` which lists every delegate
  involved, critics included. Old rows print "builder unrecorded".

The script-side chain-attribution writer was STRIPPED at the second
critique exhaustion under Wido's 2026-08-27 ruling, after five
consecutive rounds of material findings in that region. Chain
attribution returns when critique rounds route through the job
machinery; the severity-tiered-rigor design already prescribes
retiring the script driver.

Historical records stay unattributed; per-goal process metrics
compute over exactly-matching attributed records and print
attribution coverage (attributed / total in window) beside every
per-goal value; unattributed records land in the explicit
UNATTRIBUTED coverage bucket, never inferred.

LANDING ATTRIBUTION (R2-03): a landing commit is attributed to goal
G iff its diff ADDS a receipt row carrying goal=G — the same-commit
receipt invariant (docs/project-rules.md) is the join. A landing
adding no goal-tagged receipt is UNATTRIBUTED; a commit adding rows
for several goals counts its payload for EACH named goal and the
report labels it "shared commit".

## Fact sheet (verified 2026-08-27, anchors current at tip ab59e2f)

Facts the computations stand on. A mechanism claim in this design
that lacks an anchor here is a defect in review. Live counts are
as-of reads, not invariants (R1-17).

- F1 Job records: `artifacts/agents/jobs/<job-id>.json`, created by
  the Go engine (`metasystem job record-create`,
  cmd/metasystem/dispatch_verbs.go:38) on behalf of
  scripts/agents/dispatch.sh; later fields via `adapter result-patch`
  / `job chain-usage`; record-setup REPLACES the pending-setup husk
  with the full record (internal/dispatch/record.go:200). Reliable
  across all 241 current records: `role`, `status`, `endedAt`,
  `jobId`; near-universal (233/241): `round`, `startedAt`, `usage`,
  `effectiveModel`, `runtime`; `critiqueExhaustions` later-added
  (181/241). `usage` is null on some records; `usage.cost` non-null
  on 9/241, `providerUnits` on 48/241; the usage owner groups token
  dimensions by runtime, costs by currency, provider units by
  runtime/name (internal/dispatch/usage.go:8,19). NO record carries
  a goal id (R1-01). The local dir is LOSSY: terminal records whose
  mirror is past the grace window are pruned
  (internal/evidence/gc.go:82); the durable copy is the evidence-root
  chain mirror `<evidence.root>/agents/<segment>/<chain>/`
  (gc.go:232-267).
- F2 THE JOBS GAP: the newest job record is 2026-08-12. All codex
  work since ran outside dispatch.sh. Process metrics computed from
  job records alone read ZERO for the last two weeks of real work.
- F3 Critique chains: `artifacts/agents/critiques/<chain>/` holds
  `r<N>-input.md`, `r<N>-output.md`, `r<N>-stderr.log` per round
  (critique-round.sh:36-41). File mtimes are the only timestamps;
  chain names are free-form.
- F4 Goal ledger: canonical reads come from the accepted ref
  `refs/metasystem/goals/accepted`, tree prefix `plans/goals/`
  (internal/goal/validate.go:28, project.go:32). Live goals at
  `plans/goals/<id>.md`; CONCLUDED goals at
  `plans/goals/done/<id>.md` (validate.go:53; 71 files). History
  rows `- <iso8601> <opid> <verb> actor=<...> [targets=]
  [displaced=<machine>+<lineage>@<at>] [reason=...]`
  (internal/goal/file.go:426-486). No ConcludedAt field — the
  conclusion timestamp is the `done` History row's At. LIFECYCLES
  ARE IRREGULAR (R1-03): queued→done is lawful (verbs.go:419),
  goals re-claim across epochs, migrated done goals may lack a done
  row. Goals also conclude WITHOUT the CLI verb (R2-07): the
  supported edit-then-`goal reconcile` path
  (internal/goal/reconcilepub.go:289) and journal recovery
  (recover.go:155). APPETITE IS MECHANICALLY PARSEABLE:
  `ParseAppetite` (project.go:104-137), literal `Appetite:
  <int><m|h|d>` prefix (d = 8 working hours). The accepted ref is
  clone-local and can lag (project.go:29,92) — R1-12.
- F4b Goal-verb transaction journal:
  `artifacts/agents/goal-transactions/<opid>.json`
  (internal/goal/journal.go:104-111), outcome enum confirmed|
  confirmed-late|lost|abandoned|rejected|expired (journal.go:38-45),
  `attempts`, `intent.verb`, timestamps. SEMANTICS (R1-09, R2-06):
  `rejected` records refusals (txn.go:613); `lost` records a
  competing writer winning (txn.go:699) but names NO counterpart —
  same-machine and cross-machine losses are indistinguishable;
  `confirmed-late` corrects a pessimistic belief when history proves
  the operation actually landed (journal.go:337) — it is eventual
  SUCCESS, not a loss. Machine-local.
- F5 Proof-run records: (a) local battery verdicts
  `artifacts/agents/battery/<UTC>.{log,codes}` — 3 runs, all
  2026-08-22, effectively abandoned; (b) milestone-battery envelopes
  at `<evidence.root>/suite-failures/<runId>/` with `outcome.json`
  (verdict has VARIANTS beyond green — R1-11), `timings.json`
  (endedAt = completion; the runId stamp is invocation time),
  `validation.log` — envelopes can be torn/incomplete
  (milestone-battery.sh:983,995). (c) per-named-suite pass/fail
  exists ONLY in `artifacts/agents/enumeration-report.txt`
  (39 sections, OVERWRITTEN each run, opt-in). A per-leg run-history
  time series DOES NOT EXIST. (d) battery-weight.json is a rolling
  accumulator, not a history. (e) plain validate-metasystem.sh
  writes no timestamped run record.
- F6 `.gitignore` is exactly `artifacts/`, `bin/`, `/metasystem`:
  everything under artifacts/ and the evidence root is
  MACHINE-LOCAL; everything under plans/ is tracked and
  fleet-visible.
- F7 Draft queue: `plans/goals-drafts/` exists, tracked, free-prose,
  NO machinery. Slice three defines the first machine format.
- F8 Landings vs goal transactions: both interleave on main. The
  mechanical landing predicate is author email ≠
  `goals@metasystem.invalid`. The only tooling-guaranteed trailer is
  `Machine: <nickname>+<lineage>` (commit.sh:236) — the LANDER,
  never who authored the bytes.
- F9 Slice-two consumer seams: (a) steward incidents are durable
  queue entries `artifacts/agents/steward/pending/<nonce>.json`
  {nonce, message} (internal/steward/intervene.go:285-296); nonce
  families `verdict-*`, `appetite-<goalId>`; precedent block
  internal/steward/tick.go:129-138. (b) The stop message is
  assembled by internal/goal/turnverdict.go `decide` (:324-430);
  its input `report.Scan` (internal/report/scan.go:25) already
  reads job records. (c) The counselor DOES NOT EXIST. Related
  queued goals: suite-outcomes-as-steward-incidents,
  incident-proposal-drafting, stop-message-truth.
- F10 Report-generator precedent: `metasystem receipt stats`
  (internal/receipt/receipt.go:272-346) emits `key=value` rows;
  `receipt check` (:351) is due-detection; frontier writes
  atomically via tmp+rename (internal/report/frontier.go:251).
- F11 Receipts: `plans/receipts.log` (TRACKED). RECEIPT rows carry
  `type=`, `outcome=`, `corrections=`, `delegate=` (a comma list
  `runtime:model:<free-form label>` naming EVERY delegate involved,
  critics included — receipt.go:158; labels are not role-typed),
  `critique_waived=`, `verify=`, `note=`; no work-item id (R1-01).
  CORRECTION rows APPEND a field replacement referencing the
  original value, leaving the original row unchanged; multiple
  corrections may reference the same field (receipt.go:202,223).
  RETRO rows mark retro boundaries.

## The shape in one paragraph

One new engine family, `metasystem metrics`, computes the whole
family from records already on disk and writes plain-English report
documents: a compact tracked period report per machine, a detailed
machine-local report per period and per concluded goal. Every metric
row carries four things or the metric does not ship: its plain name,
its value WITH the coverage of the records it actually read, its
threshold (or "context-only" where the ratified brief says so), and
its paired action with the owning role. Computation never blocks or
refuses any other machinery — a crossed threshold produces a report
line (slice one), an incident / stop-message line to its owner
(slice two), and a draft-queue entry carrying evidence and the
paired action (slice three). Thresholds trigger conversations,
never gates. A metric whose input records have rotted, thinned, or
never existed SAYS so in its coverage line rather than printing a
confident zero.

## Design decisions

- D-A The engine computes, plumbing stays thin: `internal/metrics`
  owns every computation and threshold judgment; `metasystem metrics
  report` is the only entry; scripts only invoke.
- D-B Two report tiers (R1-16/Q4), with a CLOSED file interface
  (R2-15): the COMPACT PERIOD REPORT for a calendar week is TRACKED
  at `plans/metrics/<machine>/<ISO-week>.md` (e.g. 2026-W35) —
  machine id, exact window, source-ref identities, every metric row
  with value+coverage, context-only rows included. ONLY
  calendar-week runs write tracked reports. Custom windows
  (--since/--period-end backfills) write machine-local
  `artifacts/agents/metrics/period-<startZ>-<endZ>.md`; per-goal
  reports write `artifacts/agents/metrics/goal-<id>.md`. Detailed
  per-goal and per-source evidence stays machine-local.
- D-C Coverage is a first-class output with fixed vocabulary
  (R1-06): per source, four counts — FOUND (enumerated), USABLE
  (parsed and in-scope), REJECTED (malformed, each named in a
  detail line), MISSING (a wanted source with no records). Metric
  values compute over USABLE only; zero usable inputs prints
  "unavailable" plus coverage, never zero. Per-record failures
  never abort the report; only an unwritable report path fails the
  run.
- D-D Thresholds are TYPED in code with shipped defaults,
  overridable via metasystem.conf keys (R1-05). VALIDITY CONTRACT
  (R2-12): an override that fails its type or range (negative age,
  share outside [0,1], min>max, NaN, unparseable) disables that
  threshold — the affected row prints "threshold invalid:
  <key>=<value>; threshold disabled" and never fires; the run
  continues. Crossing a valid threshold NEVER refuses work.
- D-E Scope labels are honest (R1-12): FLEET-SYNCED metrics read
  only fleet-synced inputs (F4, F11, F8) and pin the exact input
  identity (accepted ref tip, main tip, receipts blob sha) as
  "fleet-synced-as-of <tips>"; two machines at the same tips
  compute identical values (O7). THIS-MACHINE metrics say so. No
  metric mixes scopes in one value; clearly-labelled this-machine
  DETAIL lines never feed a fleet value.
- D-F Determinism: `metrics report` over the same records with the
  same injected period is byte-identical; "now" is never sampled
  mid-computation.
- D-G The family is extensible by record, not by code fork.
- D-H Job-record discovery law (R1-07): the record set is the UNION
  of local `artifacts/agents/jobs/*.json` and every evidence-root
  chain mirror `jobs/` dir (current hashed-segment and legacy
  per-chain layouts), deduplicated by jobId; on conflict the LOCAL
  record wins; duplicates disagreeing on terminal status count as
  REJECTED and are named.
- D-I Period and scoping law (R1-14/Q2, R2-01): the period is a
  half-open UTC calendar week `[Monday 00:00, next Monday 00:00)`;
  the automatic weekly report covers the LAST COMPLETED week;
  `--since`/`--period-end` are explicit overrides. Every source
  assigns events by COMPLETION time: jobs endedAt, journal
  terminalAt, goal rows the History At, landings the committer
  timestamp, receipts the row timestamp, envelopes
  timings.endedAt. RETRO rows are events, never boundaries. THE
  WINDOW BINDS ONLY EVENT METRICS (rates, counts, shares). AGE
  METRICS (Stale checks, Debt age) evaluate at the REPORT INSTANT —
  the period end for period reports — over ALL records regardless
  of window. PER-GOAL reports read the goal's WHOLE lifecycle,
  never a week slice.
- D-J Per-goal reports have a GUARANTEED production invocation
  (R1-15/Q3, R2-07, R2-08): fast path — after a confirmed `goal
  done` publish, the CLI triggers `metrics report --goal <id>`
  post-transaction, best-effort, warning-only. Guarantee — every
  period report ALSO generates any missing per-goal report for
  goals concluded in-window (this sweep covers reconcile-done and
  recovery-done conclusions that never ran the CLI verb). ALL
  report writes are ATOMIC: tmp + rename in the target directory
  (F10 frontier precedent); a failed write leaves any prior report
  intact and prints the exact target path.
- D-K Self-exclusion (R2-05): commits whose diff touches only
  `plans/metrics/` — and the receipt rows those commits add (type
  `metrics-report`) — are excluded from EVERY metric input. The
  report's own landing never feeds the metrics it reports.

## Threshold keys (D-D; shipped defaults, all overridable)

- metrics.overhead.spend-min / spend-max — floats, agent wall-hours
  / appetite hours; defaults 0.25 / 3.0.
- metrics.overhead.density-min / density-max — floats, critique+
  correction rounds per 100 landed changed lines; defaults 0.5 / 10.
- metrics.stale-checks.max-days — integer days since last green
  proof evidence per surface; default 7.
- metrics.rework.max-per-item — integer corrections on one work
  item; default 3.
- metrics.rework.max-share — float, corrected items / receipted
  items in period; default 0.5.
- metrics.waiting.max-share — float, proving / total claim-to-done
  time per goal; default 0.5.
- metrics.debt-age.max-days — integer days; default 30.
- metrics.delegates.min-share — float, built_by=delegate items /
  items carrying built_by= in period; default 0.5.
- metrics.collisions.max-per-period — integer TRUE cross-machine
  events (displaced= rows + steal verbs); default 3.
- (Friction and Cost per result ship without thresholds — context
  rows by ratified ruling and Q1.)

## The metric family

Each row: plain name — computation with its equation (record
sources) — threshold — paired action — owner — scope label (D-E).

1. **Overhead ratio** (process-to-payload) [this-machine; per-goal
   values over attributed records per the attribution rulings] —
   per concluded goal: wall-hours = sum of endedAt−startedAt over
   ALL attributed job records (D-H), any role; critique rounds =
   sum over attributed critic chains (job records with critic roles
   grouped by root, plus F3 chains carrying a readable exact-goal
   decision) of
   max(0, rounds−1) (R2-10); corrections from metric 3's law;
   payload = insertions+deletions summed over the goal's attributed
   landing commits (the R2-03 join). SPEND = wall-hours /
   appetite-hours (F4 ParseAppetite; "no parseable appetite" line
   when absent). DENSITY = (critique rounds + corrections) per 100
   landed changed lines; zero payload → DENSITY prints
   "unavailable (no landed lines)" and never fires (R2-10).
   Threshold: either ratio outside its band. Action: draft naming
   the goal and direction. Owner: coordinator.
2. **Stale checks** [this-machine] — per proof surface, days since
   the newest GREEN evidence at the report instant (D-I): envelopes
   whose outcome.json verdict is exactly `green` (every other
   variant listed as its own labelled row, never counted green —
   R1-11), timestamped by timings.json endedAt; an envelope WITH
   outcome.json but without timings.json is USABLE with the
   envelope mtime as labelled fallback; an envelope WITHOUT
   outcome.json is REJECTED torn (R2-14). Local battery codes rows
   `=0`; the enumeration report's sections at the file's own single
   timestamp. The value is evidence age alone. Threshold:
   metrics.stale-checks.max-days. Action: run it or drop it from
   green's meaning (draft names the surface). Owner: steward.
   Named residue: no per-leg run-history record exists (F5).
3. **Rework rate** [fleet-synced] — corrections per work item from
   EFFECTIVE receipt rows. PROJECTION LAW (R2-13): an effective
   receipt is the original row with corrections applied per field
   in file order (the last correction to a field wins); effective
   values belong to the ORIGINAL row's period; CORRECTION rows are
   never work items themselves (R1-08). Value = `corrections=` on
   effective RECEIPT rows; period share = items with corrections>0
   / items receipted in period. Critique rounds print as a
   this-machine DETAIL line, never summed in. Thresholds:
   metrics.rework.max-per-item, .max-share. Action: fix the brief
   or the diagnosis. Owner: coordinator.
4. **Friction rate** [this-machine, context-only until
   classification exists] — per verb: rejected terminal operations
   / all terminal operations from the journal (F4b), numerator and
   denominator printed (Q1). Rows carry verb + evidence excerpts,
   labelled "classification is a human read"; the counselor
   inherits classification when it exists. No threshold. Action:
   classify at retro; build the missing surface when a class
   repeats. Owner: coordinator today, counselor when built.
5. **Time waiting on checks** [fleet-synced] — per concluded goal
   with a regular lifecycle: the CONCLUDING EPOCH is the last claim
   row before the done row (R1-03); building = claim→last
   attributed landing, proving = last landing→done (R2-03 join,
   whole lifecycle per D-I). Goals with no claim row, no done row,
   or no attributable landing land under "lifecycle incomplete"
   coverage, excluded from aggregates; earlier epochs print as an
   epochs count. Battery wall time from envelope timings
   (this-machine detail). Threshold: metrics.waiting.max-share.
   Action: draft naming the slowest recorded proof surface
   (whole-battery granularity today — F5). Owner: coordinator.
6. **Debt age** [fleet-synced] — ages at the report instant (D-I),
   listed by name: parked goals from Parked at=; queued goals with
   no parseable appetite (unsized debt) from OpenedAt — the chosen
   anchor, stated: always present; re-queue transitions are not
   re-aged. No residue register exists (`residue-demands-a-token`
   queued); the report names that gap. Threshold:
   metrics.debt-age.max-days. Action: own it or close it (draft per
   aging item). Owner: coordinator.
7. **Built by delegates** [fleet-synced] — share of landed work
   items BUILT by delegates: items with `built_by=delegate` /
   items carrying `built_by=` in period (the R2-04 ruling); mixed
   listed separately; rows without the key print "builder
   unrecorded" and stay out of the share — `delegate=` alone
   cannot classify (F11: it lists critics too). Landed BYTES stay
   deferred until post-ruling provenance accumulates; the report
   says so. Threshold: metrics.delegates.min-share. Action: draft
   asking why the coordinator built instead of delegating (or why
   the builder went unrecorded). Owner: coordinator.
8. **Cross-machine collisions** [mixed scopes, separated] — the
   THRESHOLD binds on TRUE cross-machine evidence only (R2-06):
   fleet-synced displaced= History rows and steal verbs (F4; both
   zero today — the rows exist for when they fire). Journal `lost`
   outcomes print as "contested transactions (counterpart
   unidentifiable — same-machine and cross-machine look alike)"
   this-machine context; `confirmed-late` is excluded entirely
   (eventual success, F4b); attempts>1 prints as contention
   context. Transport push failures have no record — MISSING
   coverage. Threshold: metrics.collisions.max-per-period. Action:
   draft naming the goal, verb, and class. Owner: coordinator.
9. **Cost per result** [this-machine; context-only BY RATIFIED
   RULING — "context/trend only, not a trigger" (Wido 2026-08-25,
   bf957fb)] — per concluded goal and per period, DIMENSIONED
   (R2-11): token sums grouped by (runtime, dimension), costs by
   currency, provider units by (runtime, unit name) — mirroring
   internal/dispatch/usage.go; wall-hours beside them; each group
   with its own coverage count (cost present on 9/241 today). The
   period's result count = goals concluded in-window; per-result
   division printed only where both sides are defined, never a
   collapsed cross-dimension total. No threshold BY RULING.
   Standing action: the coordinator reads the trend at retro.
   Owner: coordinator (R1-13).

## Slice plan

- Slice one (2h + the ruled provenance delta): `internal/metrics` +
  `metasystem metrics report [--period-end <iso8601>] [--since
  <iso8601>] [--goal <id>]` (D-I defaults); the nine computations
  with D-C coverage; D-B file interface; D-D typed thresholds with
  the validity contract; D-J invocation fast path + sweep + atomic
  writes; D-K self-exclusion; the three ruled provenance additions
  (job goalId through the full lifecycle, receipt goal= and
  built_by=); fixtures per
  obligations.
- Slice two (2h): watch wiring — the steward tick computes the
  period report and queues crossings as incidents in a new
  `metric-<name>` nonce family (F9a precedent); the stop message
  gains a current-outliers line at the F9b plug point; counselor
  consumes friction rows when it exists. Coordinates with queued
  goals suite-outcomes-as-steward-incidents and stop-message-truth.
- Slice three (2h): act hook — a crossed threshold emits a draft
  entry into plans/goals-drafts/ naming metric, evidence excerpt,
  and paired action, deduplicated by (metric, subject);
  incident-proposal-drafting's queue design consumes the format.

## Obligations (slice one; fixture-expressible)

- O1 Each metric computes from a canned record tree fixture and the
  report asserts value + coverage verbatim.
- O2 The jobs-gap case: a period fixture with landings and no job
  records produces the loud coverage line, not zeros (F2).
- O3 Goal-transaction commits never count as landings; the predicate
  is author email (F8).
- O4 Threshold crossing changes only report content — no exit code,
  lock, or gate changes.
- O5 The named-gap lines appear when their inputs are absent: no
  parseable appetite (1), no per-leg history (2), no classification
  (4), no residue register (6), builder unrecorded (7), partial
  cost coverage (9), no transport-failure record (8).
- O6 Byte-identical re-run over the same records and injected
  period (D-F).
- O7 A second fixture machine at the same fleet input identity
  computes identical fleet-synced values; a lagging machine prints
  a different as-of identity, never a silently different value.
- O8 Malformed inputs: truncated job JSON, invalid timestamp, and
  an envelope WITHOUT outcome.json show as REJECTED by name, the
  metric computing over the remainder; an envelope WITH
  outcome.json but no timings.json is USABLE on labelled mtime
  fallback (R2-14).
- O9 Dedup: the same jobId in local jobs/ and two mirror layouts
  counts once, local winning; disagreeing terminal statuses land in
  REJECTED (D-H).
- O10 Lifecycle edges: queued→done, multi-epoch, and no-done-row
  goal fixtures land in "lifecycle incomplete" / epochs-count
  lines, excluded from aggregates (R1-03).
- O11 Collision semantics: displaced=/steal rows fire the
  threshold; `lost` prints as contested context and never fires it;
  `confirmed-late` and `rejected` are excluded; attempts>1 with a
  confirmed outcome prints as contention context only (R2-06).
- O12 goal done with an unwritable metrics path still concludes the
  goal, printing the warning with the exact target path; a
  simulated failed write leaves the prior report bytes intact
  (atomic tmp+rename, R2-08).
- O13 Provenance lifecycle: a job reserved with --goal carries
  goalId on the terminal record AND on a follow-up child's record
  (R2-02); a receipt row with goal= and built_by= parses while old
  rows without them still parse; the critique-chain reader accepts
  exact `goal <id>` and explicit `unattributed` decisions, treats an
  absent decision as historical-unattributed, and sends unreadable
  or malformed decisions to REJECTED by name; metrics attribute on
  exact goal-id match only.
- O14 Self-exclusion: a fixture commit touching only plans/metrics/
  with its metrics-report receipt row changes NO metric value
  (D-K, R2-05).
- O15 Landing join: a fixture landing whose diff adds a goal=G
  receipt row counts for G; one adding none is unattributed; one
  adding rows for two goals counts for both and is labelled shared
  (R2-03).
- O16 Window scoping: an eight-day-old green envelope and a
  month-old parked goal CROSS their age thresholds in a weekly
  report (age metrics ignore the window); the same records never
  appear in event-metric counts outside their week; a per-goal
  report on a three-week goal counts all three weeks (R2-01, D-I).
- O17 Threshold validity: fixtures for negative age, share > 1,
  min>max, and NaN each print "threshold invalid" and never fire;
  the run completes (R2-12).
- O18 Correction projection: a corrected corrections= value uses
  the latest correction, attributed to the original row's period;
  two corrections to one field resolve last-wins deterministically
  (R2-13).
- O19 Cost dimensions: mixed-runtime, mixed-currency fixtures
  produce grouped sums only — no collapsed total exists in the
  report (R2-11).
- O20 The period-report sweep generates a missing per-goal report
  for a goal concluded in-window via a non-CLI path (R2-07).

## Round 1 dispositions (r1-output.md, 2026-08-27)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| R1-01 | accept, escalated → RULED | Verified: no goal id on any job record; servinggoal.go:9 is brief-context only; receipts carry no item id. Wido ruled forward-only provenance (in-session). | "The attribution rulings"; provenance additions in slice one; O13 |
| R1-02 | accept | Re-checked live: cost non-null 9/241, providerUnits 48/241 — my "null everywhere" was false. | F1 corrected; metric 9 sums where present with coverage |
| R1-03 | accept | verbs.go:419 queued→done; multi-epoch and migrated no-done-row goals exist in done/. | Concluding-epoch law in metric 5; O10 |
| R1-04 | accept | "Against" was not an equation. | SPEND/DENSITY equations, rework share, debt OpenedAt anchor; threshold key table |
| R1-05 | accept | Config resolves untyped strings (resolve.go:42). | Typed-in-code defaults + full key table (D-D) |
| R1-06 | accept | No rot semantics was the design's own threat, unhandled. | D-C four-count coverage vocabulary; O8 |
| R1-07 | accept | gc.go:80,243 — grace-window duplication and two mirror layouts are real. | D-H union+dedup law; O9 |
| R1-08 | accept | receipt.go:202 — CORRECTION amends fields; summing three signals double-counts. | Metric 3 on corrections= only; rounds demoted to detail |
| R1-09 | accept | txn.go:613 vs :699 — rejected=refusal, lost=CAS loss; verified in source. | Metric 8 recomputed; rejected moved to friction; O11 |
| R1-10 | accept | Conformance binds patches to trees, not landings; chains hold Markdown. | Metric 7 recomputed on receipts; bytes deferred |
| R1-11 | accept | Verdict variants + invocation-vs-completion timestamps verified. | Metric 2: green-only accept set, endedAt, age-alone semantics |
| R1-12 | accept | Accepted ref is clone-local; local main lagged 15 commits during review. | D-E as-of input identity; metric 3 de-mixed; O7 |
| R1-13 | part-refute | Context-only is itself Wido's ratified word (bf957fb). Accepted half: the row still needs owner+action. | Metric 9 owner (coordinator) + standing action (read trend at retro) |
| R1-14 | accept | Undecided boundary changes report identity. | D-I period law |
| R1-15 | accept | A habit is not a consumer. | D-J invocation; O12 |
| R1-16 | accept | Crossings-only drafts hide context rows from the fleet. | D-B two-tier reports |
| R1-17 | not-material, recorded | Count drift; enumeration happens at report time. | Fact sheet marked as-of |

## Round 2 dispositions (r2-output.md, 2026-08-27; early close)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| R2-01 | accept | The period law over-bound age/lifecycle metrics — my round-1 fold's own edge. | D-I scoping law (event vs age vs lifecycle); O16 |
| R2-02 | accept | record.go:200 replaces the husk; build.go:320 copies a closed field list — a record-create-only goalId dies. | Ruling text: goalId through setup + follow-up inheritance; O13 |
| R2-03 | accept | goal= names a goal, not a commit; the same-commit receipt invariant is the join. | LANDING ATTRIBUTION law; O15 |
| R2-04 | accept, escalated → RULED | delegate= lists critics (receipt.go:158; critic-only rows live at receipts.log:188,199); labels are free-form. Wido ruled built_by= (in-session). | Ruling text; metric 7 recomputed; O13/O5 |
| R2-05 | accept | The tracked report's own landing would feed the metrics — the declared fire-on-noise threat. | D-K self-exclusion; O14 |
| R2-06 | accept | lost names no counterpart; confirmed-late is eventual success (journal.go:337). | Metric 8: threshold on displaced=/steal only; O11 rewritten |
| R2-07 | accept | reconcilepub.go:289 and recover.go:155 conclude goals without the CLI verb. | D-J sweep guarantee; O20 |
| R2-08 | accept | Round-1 Q3 adoption said atomic; the fold dropped it. | D-J atomic tmp+rename (frontier.go:251 precedent); O12 |
| R2-09 | accept; later superseded by the strip | Late attribution binding rewrites history retroactively. | Writer stripped under Wido's 2026-08-27 second-exhaustion ruling; reader behavior remains in O13 |
| R2-10 | accept | Rounds/roles/zero-denominator each admitted multiple readings. | Metric 1 equations pinned |
| R2-11 | accept | usage.go:8,19 groups dimensions deliberately; collapsing them is meaningless. | Metric 9 dimensioned; O19 |
| R2-12 | accept | Typed defaults without a bad-override contract change firing behavior. | D-D validity contract; O17 |
| R2-13 | accept | receipt.go:223 appends replacements; projection was undefined. | Metric 3 projection law; O18 |
| R2-14 | accept | Torn vs fallback used one word for two shapes. | outcome.json presence is the discriminator; O8 |
| R2-15 | accept | Backfill runs had no file identity. | D-B closed file interface |
| R2-16 | out-of-scope, ACTED ON | The goal-next warning-contamination defect REPRODUCED with mechanism (txn.go:68 CombinedOutput → project.go:38 trims as tip). Not a metrics finding. | Backlogged as its own goal |

Critic's round-1 Q-recommendations Q1-Q4: adopted (metric 4, D-I,
D-J, D-B).
