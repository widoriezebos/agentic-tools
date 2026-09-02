# Failed-job attention — design

Goal: failed-job-attention (plans/goals/failed-job-attention.md, revision 4).
Mode: design. Date: 2026-09-02. Appetite: 1.5h build; the SIMPLEST design
honoring the existing episode pattern wins. **Design revision 2**: folds all
seven findings of records/misc/failed-job-attention-critique-r1.md; the fold
record in section 10 maps each finding id to its fold.

The one-sentence design: the steward tick gains one bounded sweep that, for
every claimed goal, raises a durable escalation episode in the EXISTING alert
episode store when a delegate job under that goal sits in terminal failure
with its chain open, and when a breach-stop fence stands awaiting resume —
under this design's OWN episode namespace (`escalation-<digest>`), disjoint
from the alert-channel design's future `alert-<digest>` namespace, which that
goal migrates on its own terms — and surfaces each episode through what
exists today: the durable notification queue, the narrator digest, the
narration line, and the `watch` alerts view.

The incident this fixes (records/misc/idle-loss-2026-09-01.md): a worker died
82 seconds after launch, the job record said `failed` within seconds, and no
mechanism carried that fact anywhere. Six hours were lost. The breach-stop
that finally fired also told nobody. Both events now become episodes the
machinery nags about until a human acknowledges or answers them.

## 1. Traced facts this design stands on

- **The episode store** (`internal/steward/alert_episode.go`): one JSON file
  per episode under `artifacts/agents/steward/alerts/`, flock-serialized
  (`lockAlerts`, lines 74–92), atomic durable writes (`saveAlertEpisode`,
  152–165). The loader (110–124) requires `Schema == 1`, a valid lowercase
  `[a-z0-9-]` id of at most 96 characters, a 64-lowercase-hex digest
  (`validEvidenceDigest`, `component_evidence.go` 441–444), a nonempty
  `Message`, a nonzero `OpenedAt`, non-nil `Attempts`, and a nonempty
  `TransportResult`. `AcknowledgeAlert` (366–392) records acknowledgment
  without clearing and is class-agnostic.
- **The health join clears everything**: `UpdateAlertEpisodes` (231–362)
  resolves-and-clears EVERY episode on a healthy aggregate (246–268) and
  resolves every episode whose digest differs from the current finding
  (270–279). Any new episode class written into this store is wiped by the
  next healthy tick unless the health join is scoped. This is the one edit to
  shared code this design requires (section 5).
- **The stalled-idle escalation pattern** (`internal/steward/tick.go`
  215–226): a notify verdict queues one durable `PendingNotification` keyed by
  a stable nonce; the queue holds one pending message per standing condition,
  `DeliverPending` (`notify.go` 64–99) delivers and removes it, and the next
  tick re-queues while the condition stands. The digest gets a lowlight entry
  with `SourceType: "episode"` (`narrate.go` 82–87). This raise/nag shape is
  what the two new classes follow.
- **The pending queue's exact primitives** (`internal/steward/intervene.go`
  280–349): one JSON file per pending message at
  `artifacts/agents/steward/pending/<nonce>.json`; `QueueNotification`
  (299–314) is an atomic durable write keyed by nonce, `PendingNotifications`
  (317–344) lists and decodes the whole directory, and `MarkDelivered`
  (347–349) is `os.Remove` of the nonce-named file — removal by nonce is the
  queue's only retirement primitive, and the runner (`runner.go` 131) calls
  `DeliverPending` after `RunTick` in the same process, sequentially.
- **The digest deduplicates exact retries**
  (`internal/narratordigest/digest.go` 109–135): `Append` composes each
  entry's signature from kind, flattened text, and the source marker, and
  skips any entry whose exact signature already appears in the digest body.
  Re-emitting an identical entry on a later tick is therefore idempotent.
  `NarrateDigest` runs at `tick.go` 230 and a failure fails the tick.
- **The tick's fallible stretch**: between the sweep's insertion point
  (immediately after `ReapContinuations`, `tick.go` 170–173) and
  `NarrateDigest` (230) sit ledger attention, evidence loading, marks, and
  the decision — several of which degrade or fail the tick (192, 196). An
  in-memory-only transition report does not survive that stretch.
- **The job record vocabulary** (`internal/dispatch/record.go`): terminal
  statuses are `completed`, `failed`, `cancelled`, `timeout` (45–47).
  Records carry `goalId`, `goalRevision`, `role`, `round`, `parentJob`,
  `error`, `protocolError.violation`, `endedAt`, `createdAt`, `startedAt`,
  `reviews`; `chainClosed` is terminal metadata (92–95). There is no
  `failureReason` field — the input brief's word for it maps to `error`.
  `RecordCreate` lawfully reuses a job id once the old record file is gone,
  and nothing protects `createdAt` as immutable.
- **The direct protocol-error writer** (`record.go` 417–465):
  `RecordProtocolError` bypasses `RecordCAS`, stamps `status: "failed"`,
  `error: "protocol_error"`, and a nested
  `protocolError: {key, violation, detectedAt}`, and sets `endedAt` when
  absent. It is a distinct terminal writer the sweep must provably cover.
- **The stop fence**: a claimed goal file carries `StopFence` (`StopID` of the
  form `stop-<goal-id>-r<revision>-f<epoch>`, `Revision`, `ClosedAt`,
  `Reason` such as `ELAPSED_LIMIT`) after a breach-stop closes launches
  (`internal/goal/file.go` 96–105, `internal/dispatch/stop.go` 89–99). The
  fence is durable BEFORE the local cancellation batch exists or completes;
  `goal resume` refuses until `VerifyStopBatchComplete` proves the complete
  batch (`internal/goal/stop.go` 248–253, called from resume's mutation at
  390). Only `goal resume --id <goal> --by <name>` plus the complete budget
  tuple (`--elapsed-limit`, `--attempt-limit`,
  `--reserved-job-minutes-limit`, `--active-job-limit`) removes the fence
  (`cmd/metasystem/goalsync_mutations.go`). Once the stop batch is COMPLETE,
  `FindBreachStops` skips the goal entirely (`stop.go` 294–296) — a standing
  completed stop is invisible to today's tick. That silence is exactly
  incident finding 2.
- **Enumerating claimed goals is a read the tick already pays**:
  `FindBreachStops` runs `goal.Project` over the live tree every tick
  (`stop.go` 270–288). The jobs directory holds on the order of 100 records
  today (`artifacts/agents/jobs`), each a small JSON file; evidence GC
  (`internal/evidence/gc.go`) prunes mirrored terminal records after a
  5,400-second default grace window.
- **The adjacent landed design** (`plans/alert-channel-design.md`, §11a):
  specifies, for these same two real-world conditions, its OWN classes
  (`delegate-job-failed`, `stop-awaiting-resume`), episode ids of exactly
  `alert-<64-hex-digest>`, a facts contract that REQUIRES
  `answerAction`/`answerReason` derived at journal time, a skip law that
  treats any existing episode at its digest as already-minted, and a stop
  alerting condition of successful `VerifyStopBatchComplete` — a COMPLETE
  batch, not a bare fence. Those are that goal's contracts; revision 2 of
  this design deliberately does not write into that namespace (section 2).

## 2. The governing seam decision (revised; folds FJA-R1-CHANNEL-PARTIAL-FACTS and FJA-R1-STOP-PREDICATE)

Revision 1 adopted the channel's episode identity — its class literals,
digest tuples, and `alert-<digest>` ids — to avoid double episodes at channel
arrival. The critique proved that adoption unsound twice over: this design
cannot honestly mint the channel's episodes (it omits the REQUIRED
`answerAction`/`answerReason` facts, whose journal-time derivation may be
impossible to backfill after evidence GC — FJA-R1-CHANNEL-PARTIAL-FACTS),
and it does not share the channel's stop predicate (this design alerts on a
standing fence; the channel alerts only on a proven-complete batch — a
write-once early episode under the channel's digest would permanently block
the channel's own correct episode — FJA-R1-STOP-PREDICATE).

Therefore revision 2 takes THIS design's own identifier namespace and
schema, and never writes a partial record under the channel's final
identifiers:

- **Ids**: `escalation-<64-hex-digest>`, filename
  `alerts/escalation-<digest>.json` (75 characters, passes the 96-character
  id validator; collision-free against health ids and against the channel's
  future `alert-<digest>` grammar).
- **Class literals**: `escalation-failed-job` and `escalation-breach-stop` —
  this design's own, entering this design's own digest tuples (section 3).
- **Facts**: this design's own keys, all mechanically derivable at raise
  time from the record or fence in hand. No `answerAction`, no
  `answerReason`, no promise of them — those keys belong to the channel's
  contract, which this design no longer claims to satisfy.
- **Marker directory**: `alerts/escalation-stop-open/` — the same
  marker-before-episode mechanism (section 3b) in this design's own
  directory, so the channel's future `stop-open/` markers never collide.
- **The channel migrates by its own design**: when goal
  alert-escalation-channel lands, it finds a disjoint, honestly-scoped
  namespace. Whether it consumes these episodes, supersedes them with its
  own, or leaves them as history is that goal's migration decision, made
  with its own answer-derivation and retention machinery in hand. The only
  obligation this design leaves it is written here: episodes matching
  `escalation-<64hex>.json` are this goal's, complete under this section's
  schema, and safe to treat as such. The transient cost accepted in
  exchange: if the channel chooses parallel episodes for a condition still
  standing at its arrival, the human may see the same condition under two
  ids for one migration window — a visible, bounded cost, against revision
  1's invisible one (a permanently wrong channel episode).

What this goal still does NOT build: the transport sender
(`DeliverDueAlerts`), the escalation ladder, the answer-derivation table,
the retention pin (§11a.12), and every external adapter. Those stay with
goal alert-escalation-channel.

Deviations from the m3 input brief (input material, not a certified design),
each forced by a traced fact: episode ids are `escalation-<64-hex>` rather
than `escalation-failed-job-<jobId>` (an unbounded job id could overflow the
96-character id validator; a digest id keeps exists-by-digest dedup);
episodes are write-once rather than "refreshed" (both classes' facts are
immutable at their source, so there is nothing to refresh; the NAG is
per-tick, the episode is not rewritten); the failure text comes from the
record's `error` field (no `failureReason` exists); `timeout` is included
beside `failed` (a worker the reaper killed at its cap with nobody watching
is this incident's exact shape); the breach-stop clear is the marker-drain
fence-gone proof rather than a free-form "resumed goal" check (the digest
filename is one-way; the marker restores the reverse mapping).

## 3. The two classes, exact

Both are `AlertEpisode` records with `Schema: 1`, `EpisodeID: "escalation-"
+ digest`, `Digest: <64-hex>`, `Message` set once at creation, `OpenedAt:
now`, `Attempts: []` (non-nil, empty), `TransportResult: PENDING` (truthful:
no episode-level transport exists yet; today's delivery rides the
notification queue), plus two additive fields `class` and `facts`. The
fields are additive to the shared struct and schema stays 1; health episodes
never set them.

### 3a. escalation-failed-job

- **Raise predicate**, evaluated per tick over `artifacts/agents/jobs/*.json`:
  the record decodes; `status` is `failed` or `timeout`; `goalId` is nonempty
  and names a goal whose projected state is claimed (any machine — a local
  job record was dispatched here, and a dead delegate under someone's claim
  is attention-worthy here); `chainClosed` is absent or false; and the
  candidate's episode file does not exist (the candidate open in section 4 —
  dedup is by digest-named path). `cancelled` is excluded: cancellation is
  an operator's own act, already acknowledged by construction. `completed`
  is excluded trivially. This predicate covers BOTH terminal writers: the
  ordinary CAS path and `RecordProtocolError`'s direct stamp (traced in
  section 1) produce records this predicate matches; fixture 11 proves the
  direct writer specifically.
- **Digest**: SHA-256 lowercase hex of
  `escalation-failed-job` + LF + job id + LF + birth token, no trailing
  newline. Pinned vectors (computed 2026-09-02 with sha256sum):
  `escalation-failed-job\nimplementer-c002e6035a243bdbc1400067\n2026-08-31T18:02:11Z`
  → `cfce3b7f36d66fdd6fd777ba613bbe753cf6f2dbc92def0154a936b60587c3d9`;
  the empty-birth form (`escalation-failed-job\n<jobId>\n`) →
  `65e76dc4bfba829b3cf2939899ba71f1a63c5e386c8b322c259ce4c25b9b973a`.
  Dedup per job incarnation is this digest.
- **The birth token, honestly (folds FJA-R1-BIRTH-ABA)**: the digest's third
  element is DECLARED dependent on goal job-record-birth-token. When that
  goal lands its minted birth generation on the record, the upgrade here is
  one line: the tuple's third element becomes the minted token verbatim.
  Until then the fallback chain is `createdAt`, else `startedAt`, else
  empty — and its reuse exposure is declared, not hidden: `RecordCreate`
  lawfully reuses a job id after evidence GC removes the old record file,
  `createdAt` is optional and unprotected, and a reused id whose fallback
  bytes are byte-identical to a prior incarnation's collapses to the old
  digest — the old episode (possibly acknowledged) then suppresses the new
  failure. The bounded-lifetime argument for accepting this: (1) the
  exposure exists only in the window before the birth-token goal lands, and
  the upgrade is the one-line tuple change above; (2) inside that window,
  when `createdAt` is present, the two incarnations' creation stamps are
  separated by at least the GC grace window (5,400 seconds) plus the first
  incarnation's lifetime, so byte-identical `createdAt` values require a
  hand-edited record, not the mechanism; (3) the genuinely exposed set is
  records carrying neither `createdAt` nor `startedAt`, whose incarnations
  all collapse to the one empty-birth digest per job id — accepted as a
  declared bounded loss for the window, on the argument that a system
  minting timestamp-free job records has a defect upstream of attention.
- **Facts** (`facts` map, this design's own keys, all mechanical at raise
  time): `goalId`, `jobId`, `birth` (the token's exact bytes, possibly
  empty), `reason` (the record's `error` field verbatim, with
  `protocolError.violation` appended after `: ` when present, else `""`),
  `role`, `chainRoot` (the parentJob-walk result, `""` on any walk refusal),
  `reviews` (the record's `reviews` field verbatim, may be `""`).
- **Message**, plain words, set once (facts are terminal, so it never goes
  stale), shaped like:
  `Delegate job <jobId> (role <role>, round <round>) under goal <goalId>
  ended <status> at <endedAt>: <reason, or "no failure reason was recorded">.
  Nobody has closed its chain. The job record is
  artifacts/agents/jobs/<jobId>.json. Acknowledge with: metasystem health
  acknowledge-alert --episode escalation-<digest> --repo <repository root>.`
- **Lifecycle**: never auto-cleared (never clearing is what keeps
  exists-by-digest dedup sound). `AcknowledgeAlert` is the terminal human
  step. The NAG (section 4) stops when any of: the episode is acknowledged;
  the record's `chainClosed` is true; the goal is no longer claimed; the
  record is gone. None of those clears the episode; the file remains
  evidence.

### 3b. escalation-breach-stop

- **Raise predicate, this design's own (folds FJA-R1-STOP-PREDICATE)**,
  per tick over the same goal projection: a goal file whose state is claimed
  and whose `StopFence` is non-nil. The fence's existence IS "a breach-stop
  fired" — it stands from the moment the stop installs it, through batch
  completion, until resume. One rule, no batch-state branching, on facts
  the projection already holds. **Stated difference from the channel's
  predicate**: the channel design alerts only on successful
  `VerifyStopBatchComplete` — a proven-COMPLETE cancellation batch — because
  its externally delivered alert prescribes a resume that must succeed on
  first try. This design deliberately alerts EARLIER, at the fence, because
  its job is attention (the incident's six silent hours began at the fence,
  not at batch completion), and because its episodes live in this design's
  own namespace the channel's later, stricter episode is never blocked by
  this one (section 2). The honest consequence is carried in the message: a
  resume attempted between fence close and batch completion refuses
  (`VerifyStopBatchComplete` is resume's precondition), the custodian
  (`runBreachStopCustodian`) advances the batch every tick, so the message
  says a refused resume should be retried after the next steward tick.
- **Digest**: SHA-256 lowercase hex of `escalation-breach-stop` + LF + goal
  id + LF + `StopFence.Revision` in base-10 with no leading zeros.
  Revisions never repeat within a goal (resume mints a fresh one), so this
  is the dedup per stop. Pinned vector (computed 2026-09-02 with sha256sum):
  `escalation-breach-stop\nalert-escalation-channel\n8` →
  `d03a223ce097e9993dec72feee3b26227589fa9dae588b14cfbbe90071ba0155`.
- **The marker, before the episode**: before writing the episode, the sweep
  durably writes `artifacts/agents/steward/alerts/escalation-stop-open/
  <goal-id>-r<revision>` whose entire content is the 64-hex digest, under
  the alert lock, using the store's atomic durable write. Marker first, then
  episode; re-deriving is idempotent (same name, same content). The marker
  filename is reversible: stripping the trailing `-r[0-9]+` yields the goal
  id and revision.
- **Facts**: `goalId`, `revision` (base-10), `stopId` — all in hand at raise
  time.
- **Message**, plain words, set once, shaped like:
  `Goal <goalId> revision <revision> was breach-stopped at <ClosedAt>
  (<Reason>, e.g. ELAPSED_LIMIT means the elapsed budget fence was hit). No
  new jobs can launch under it until a human resumes it. Resume from an
  agent-free terminal with: metasystem goal resume --id <goalId> --by <your
  name> --elapsed-limit <duration> --attempt-limit <count>
  --reserved-job-minutes-limit <minutes> --active-job-limit <count> — all
  four budget flags are required, as a fresh complete budget. If resume
  refuses because the stop's cancellation batch is still completing, wait
  one steward tick and retry. Acknowledge with: metasystem health
  acknowledge-alert --episode escalation-<digest> --repo <repository
  root>.`
- **Lifecycle**: cleared ONLY by the positive fence-gone proof. Each tick
  the sweep lists `alerts/escalation-stop-open/` (bounded by open stops,
  because draining removes markers) and evaluates, per marker, against the
  goal projection already in hand: fence still bound (the goal exists, is
  claimed, and carries a `StopFence` with that marker's revision) → the
  marker stays. Fence GONE → drain: read the marker's digest, open the
  episode, set exactly `Cleared`/`ClearedAt` if present and uncleared, then
  remove the marker; an absent or already-cleared episode still drains the
  marker. Anything unreadable HOLDS (a clear is never inferred from a
  failed read). Acknowledgment stops the nag but never clears.

## 4. Tick integration, read set, and today's delivery

**One new sweep**, `SweepWorkEscalations(repoRoot, now)`, in a new
`internal/steward/escalation.go`, called from `RunTick` immediately after
`ReapContinuations` (tick.go 170–173) — after `runBreachStopCustodian`, so a
fence installed this very tick escalates this very tick, and after the reap,
so records the reaper just finished are seen in their final state. On error
the tick takes the existing `degradedTick` path with a named reason
(`"work escalation sweep failed: …"`) — an unreadable store is never a silent
verdict. The sweep's writes (markers, episodes, the transition journal) run
under the existing alert flock; the arbitration lock already serializes
ticks.

**Read set, bounded (folds FJA-R1-READ-BOUND)**: one `goal.Project` over the
live tree (the same class of read `FindBreachStops` performs every tick; the
sweep does its own call rather than refactoring the custodian — simplest,
and the projection is the bound); one `ReadDir` of `artifacts/agents/jobs`
with one small-JSON decode per record (~100 today, growth bounded by
evidence GC); one `ReadDir` of `alerts/escalation-stop-open/` (bounded by
open stops); **one open-and-decode of the digest-named episode file per
candidate** — a candidate being a record matching 3a's status/goal/chain
tests or a standing fence or a listed marker — because a stat proves only
existence while the NAG needs each candidate episode's acknowledgment state
and write-once message; the open serves both the raise dedup (absent file →
raise) and the NAG (present file → read `AcknowledgedAt` and `Message`);
one `ReadDir`-and-decode of the pending queue (`PendingNotifications`,
bounded by queued messages) for the withdraw phase below. The sweep NEVER
calls the full reader `AlertEpisodes()`: retained episodes with no standing
condition — the accumulating history — are never opened. The opens are
proportional to live candidates, not to history; an unreadable CANDIDATE
episode fails the sweep into the degraded tick (honest, like every other
torn read), while non-candidate files are not read at all (fixture 13). No
network, no processes, no record writes.

**What one sweep does**, in order: (1) enumerate claimed goals from the
projection; (2) stop-class pass — for every standing fence, open the
candidate episode; absent → raise (journal entry, then marker, then
episode); then the clear phase — drain fence-gone markers (journal entry,
then `Cleared`, then marker removal); (3) failed-job pass — for every record
matching 3a's predicate, open the candidate episode; absent → raise
(journal entry, then episode); (4) the NAG-or-withdraw phase (folds
FJA-R1-PENDING-LIFECYCLE) — compute the nag set: every candidate episode
open in this tick whose condition still stands (3a: record present, chain
open, goal claimed; 3b: marker present, fence bound) and whose
`AcknowledgedAt` is unset. For each episode in the nag set, queue one
durable `PendingNotification{Nonce: episodeID, Message: episode.Message}` —
exactly the stalled-idle shape, so at the default 600-second tick the nag is
at most one delivery per condition per 10 minutes. **The withdraw rule, one
rule**: every pending notification whose nonce matches
`escalation-[0-9a-f]{64}` exactly and is NOT in this tick's nag set is
removed from the queue by nonce (the same `os.Remove` primitive
`MarkDelivered` uses; an already-absent file is ignored). The grammar
matches no other producer's nonce (`verdict-*`, `ledger-attention-*`, and
intent nonces never fit it). No-orphan proof: every producer-class pending
file is examined every sweep and either re-queued (condition stands,
unacknowledged) or removed (anything else) — a queued notification whose
off-switch flipped never outlives the next sweep, and because the runner
delivers only after `RunTick` returns (runner.go 131, same process,
sequential), the worst case is exactly one stale delivery for a flip that
happens after the sweep and before that tick's delivery pass, after which
the next sweep withdraws. Withdrawal never depends on delivery succeeding,
so a down notification channel cannot strand a withdrawn condition either.
(5) return a report (`[]WorkEscalationReport{Class, EpisodeID, GoalID,
JobID, StopID, Raised, Cleared}`) on `TickResult` for the narration line.

**Transitions survive the tick (folds FJA-R1-DIGEST-TRANSITION-LOSS) —
write-ahead, chosen over derive-on-next-tick** (deriving would need
per-episode "narrated" state mutations on write-once files; the journal
keeps episodes clean). The durable transition journal is
`artifacts/agents/steward/alerts/escalation-transitions.json` — a JSON list
of `{kind: "raised"|"cleared", episodeID, class, goalId, jobId, stopId}` —
written atomically under the alert flock. Ordering law: the journal entry is
appended durably BEFORE the state change that would make its transition
undetectable (before the episode write for a raise; before the marker
removal for a clear), deduplicated within the journal by `episodeID`+`kind`
so a crash-and-retry never doubles an entry. `NarrateDigest` (tick.go 230)
reads the journal, emits one digest entry per journal entry — lowlight for
`raised` (`SourceType: "episode"`, `SourceID: <episodeID>`, text naming the
goal, the job or stop, and the failure in plain words), highlight for
`cleared` (`SourceID: <episodeID>-cleared`) — and only after
`narratordigest.Append` returns success rewrites the journal without the
emitted entries. Loss proof: a crash between the episode/marker write and
the digest append leaves the journal entry in place; the next tick's
`NarrateDigest` re-emits it, and the digest's exact-retry dedup
(digest.go 109–135, traced in section 1) makes re-emission idempotent — so
every transition reaches the digest at least once and appears exactly once.
Emission is on transition only — never per standing tick — so the digest
stays readable.

**Narration** (what exists today): the best-effort narration line appends a
note while anything stands: `"<n> failed delegate job(s) and <m>
breach-stop(s) are waiting for attention"`, from the sweep report.

## 5. The one edit to shared code: scoping the health join

`UpdateAlertEpisodes`' healthy-clear loop (alert_episode.go 246–268) and its
resolve-others loop (270–279) are restricted to health episodes by filename
grammar: the health path loads only directory entries NOT matching
`escalation-<64 lowercase hex>.json`. By construction (section 1, naming
trace) the excluded set is exactly this design's episodes — a
producer-named file is never even opened by the health path.
`AlertEpisodes()` (the full reader), `AcknowledgeAlert`, and
`migrateHeldHealthNotifications` are unchanged (legacy migration nonces are
health digests and produce health-named ids). The alert-channel goal will
extend the same exclusion to its own `alert-<64hex>` grammar when it lands;
this design's edit covers only its own grammar — minimal now. The edit is
REQUIRED now: without it, the first healthy tick wipes both new classes.
The stalled-idle lifecycle is untouched: its verdict still rides the notify
queue, and the health-joined episodes behave byte-identically (fixture 8
proves it).

## 6. Fixtures (all deterministic: temp repos, injected clock, no sleeps, the
steward package's existing test idiom)

1. `TestFailedJobUnderClaimedGoalRaisesEscalation` — claimed goal file plus a
   `failed` job record naming it; one sweep raises
   `alerts/escalation-<digest>.json` with the right class, facts, and
   message; the digest matches the pinned vector recomputed in-test from the
   tuple bytes; one notification with nonce `escalation-<digest>` is
   pending; the journal holds the raised entry.
2. `TestFailedJobEscalationIsWriteOnceAndNagsPerTick` — a second sweep leaves
   the episode file byte-identical, creates no second episode, adds no
   second journal entry, and re-queues the notification after a simulated
   delivery (`MarkDelivered`).
3. `TestOffSwitchStopsTheNagAndWithdrawsTheQueuedMessage` — with a
   notification already durably queued, flipping `chainClosed`, releasing
   the goal, or removing the record each make the next sweep REMOVE the
   pending file (the withdraw rule) and queue nothing new; the episode
   remains uncleared in all three; a pending `verdict-*` nonce in the same
   queue is untouched.
4. `TestTimeoutRaisesAndCancelledCompletedUnclaimedDoNot` — the predicate's
   four exclusions, one sweep each.
5. `TestBreachStopWritesMarkerThenEpisodeWithResumeCommand` — claimed goal
   with a `StopFence`; after one sweep the marker exists under
   `escalation-stop-open/` with the digest as its entire content, the
   episode exists, and the message contains `goal resume --id`, all four
   budget flag names, and the retry-after-a-tick sentence; digest matches
   the pinned stop vector recomputed in-test.
6. `TestResumeDrainsMarkerAndClearsStopEpisode` — the fence removed (as
   resume does); the next sweep sets exactly `Cleared`/`ClearedAt`, removes
   the marker, journals the cleared transition, and a further sweep raises
   nothing; an absent-episode marker also drains.
7. `TestAcknowledgedEscalationGoesQuietWithdrawsAndStands` — after
   `AcknowledgeAlert`, the next sweep withdraws any queued notification and
   queues no further one while the condition stands; the episode file
   remains, uncleared.
8. `TestHealthyVerdictLeavesEscalationEpisodesUntouched` — the scoping
   proof: a healthy `UpdateAlertEpisodes` pass clears health episodes and
   leaves both producer-class files byte-identical; an unhealthy pass
   resolves neither of them.
9. `TestEscalationSweepFailureDegradesTheTick` — an unreadable jobs
   directory yields the degraded verdict with the named reason, via
   `RunTick`; so does an unreadable CANDIDATE episode file.
10. `TestTickRaisesFailedJobEscalationEndToEnd` — one full `RunTick` in a
    temp repo proving the wiring: episode on disk, notification pending,
    digest entry present, journal drained.
11. `TestProtocolErrorRecordRaisesWithNestedViolation` (folds
    FJA-R1-PROTOCOL-WRITER-PROOF) — the fixture drives the REAL writer: it
    creates a running-status record and invokes
    `dispatch.RecordProtocolError` to stamp it, never hand-writing the
    terminal shape; one sweep raises the episode, and the fixture asserts
    the facts `reason` and the message both render
    `protocol_error: <violation>` — the `error` field with the nested
    `protocolError.violation` appended per 3a.
12. `TestTransitionSurvivesTickFailureBeforeDigest` (folds
    FJA-R1-DIGEST-TRANSITION-LOSS) — run the sweep alone (episode written,
    journal entry present), simulating a tick that died before
    `NarrateDigest`; then call `NarrateDigest`: the raised entry appears in
    the digest and the journal drains; a second `NarrateDigest` adds
    nothing (exact-retry dedup); the same shape for a cleared stop
    transition with the marker already drained.
13. `TestSweepNeverOpensNonCandidateEpisodes` (folds FJA-R1-READ-BOUND) — a
    syntactically corrupt `escalation-<hex>.json` for which no record,
    fence, or marker stands does not perturb the sweep (proving history is
    never opened), while the same corrupt bytes at a CANDIDATE's digest
    path degrade the tick.

## 7. Blast radius — every consumer of episode listings, checked

- **`watch` alerts view**: raw-reads every regular `alerts/*.json`
  (records/watch-verb/watch-verb-design.md, table row "alerts"); the new
  episodes appear automatically; `escalation-stop-open/` is a subdirectory
  and is skipped by both the watch glob and `loadAlertEpisodesUnlocked`'s
  `IsDir` check; `escalation-transitions.json` has no `Schema` field and is
  a watch-visible raw file only — acceptable, it is small and drains.
- **`AlertEpisodes()` and the loader invariants**: new episodes satisfy every
  loader requirement (section 3 preamble) — except that
  `escalation-transitions.json` must not be parsed as an episode; the
  loader's per-file decode tolerates-and-skips a non-episode JSON shape, and
  the implementation must keep it skipped, not fatal (the journal name is
  fixed, so a one-name exclusion in the loader is the mechanical rule).
- **`metasystem health`** (`runStewardHealth`): calls the now-scoped
  `UpdateAlertEpisodes`; output bytes unchanged; escalation episodes are
  invisible to it by the filename grammar.
- **`health acknowledge-alert`**: unchanged and class-agnostic; the
  75-character ids pass `validEpisodeID`.
- **The narrator digest and its consumers** (`steward digest-pending`, the
  seat): additive entries only, on transitions only, journal-backed.
- **The counselor**: `counselor_carriage.go` reads no episodes; no impact.
- **`scripts/agents/health-fixtures.sh`**: its beds contain no claimed goals
  or failed jobs, so no escalation episodes arise there; its `alerts/*.json`
  loop tolerates unrelated files regardless.
- **Evidence GC**: prunes mirrored terminal records after the 5,400-second
  grace window. Disclosed residual: a record pruned before any sweep ran
  would escape escalation; at the 600-second default tick at least eight
  sweeps see every record inside the grace window, and the channel goal's
  §11a.12 retention pin closes the hole for good. This goal does not touch
  GC.
- **The alert-channel goal**: finds a DISJOINT namespace
  (`escalation-<64hex>` never matches its `alert-<64hex>` grammar or its
  skip law), migrates these episodes by its own design (section 2), takes
  over transport under its own ids, and extends the health-join exclusion
  to its own grammar at its own land time.

## 8. Scope fence

No new delivery machinery; no external sends; no transport changes; no reaper
or breach-stop behavior changes (the sweep only reads their records); no GC
changes; no counselor changes; no changes to stalled-idle verdicts, health
verdicts, or the acknowledge verb; no writes into the channel's
`alert-<digest>` namespace. Implementation touches: new
`internal/steward/escalation.go` (+tests) including the transition journal
and withdraw phase, two additive `AlertEpisode` fields, the filename-scoped
health-join loops plus the one-name journal exclusion in
`alert_episode.go`, the one sweep call plus report field in `tick.go`, and
the journal-driven transition entries in `narrate.go`.

## 9. Self-grade

**Grade: A−.** Strengths: every mechanism is an existing, traced pattern
(episode store, notify queue nonce dedup and removal, digest exact-retry
dedup, degraded-tick honesty), and every predicate, digest byte, lifecycle
rule, read, and crash boundary is now mechanical — the seven critique
findings each closed with a rule an implementer applies without deciding
anything; the namespace split makes this goal honest on its own terms
instead of impersonating a stricter future contract; the three pinned
vectors were computed, not trusted. Weaknesses, honestly: (1) the namespace
split accepts a possible bounded double-visibility window at channel
arrival (section 2) — a deliberate trade, but a real one the channel goal
must disposition in its migration; (2) the birth-token fallback keeps a
declared ABA residue for timestamp-free or hand-edited records until the
sibling goal lands (3a) — declared, bounded, not eliminated; (3) the
transition journal and withdraw phase add two small mechanisms revision 1
did not carry, spending some of the 1.5h appetite on crash-correctness the
critique proved necessary; (4) the nag rides a notification channel that is
unconfigured on Linux hosts today, so until the channel goal lands,
visibility there is the durable pending queue, `steward status`, the
digest, and `watch` — stated, not hidden, and identical to stalled-idle's
current reality.

**Reject this design if any of these fail**: a recomputation of the three
pinned digest vectors in section 3 disagrees; the `UpdateAlertEpisodes`
scoping change alters any byte of an existing health-episode fixture's
expected output; the `escalation-<64hex>` grammar is shown to collide with
any existing or channel-design filename, nonce, or id grammar; the
no-orphan proof in section 4 is shown to leave any pending-queue file
unexamined by the sweep; the write-ahead ordering in section 4 is shown to
admit a transition that can never reach the digest; or the orchestrator
rules the transition journal or withdraw phase out of the 1.5h appetite —
in which case the fallback is NOT revision 1's channel-identity variant
(the critique closed that door) but an explicit scope cut recorded on the
goal: ship raise/nag/acknowledge without the journal (digest entries become
best-effort, the loss window reopens) — a cut only the goal owner may
accept, in writing.

## 10. Fold record — critique round 1 (records/misc/failed-job-attention-critique-r1.md)

- **FJA-R1-BIRTH-ABA** → section 3a "The birth token, honestly": declared
  dependency on goal job-record-birth-token, one-line upgrade named,
  fallback reuse exposure declared with its bounded-lifetime argument.
- **FJA-R1-STOP-PREDICATE** → sections 2 and 3b: this design's own fence
  predicate specified on facts the tick holds; the difference from the
  channel's batch-complete predicate stated; the refuse-then-retry
  consequence carried in the message; no write-once collision remains
  because the namespaces are disjoint.
- **FJA-R1-CHANNEL-PARTIAL-FACTS** → section 2: own identifier namespace
  (`escalation-<64hex>`), own class literals, own facts schema without the
  channel's required keys; the channel migrates by its own design; new
  pinned vectors computed for the new tuples.
- **FJA-R1-PENDING-LIFECYCLE** → section 4 step 4: the one withdraw rule
  for queued notifications at off-switch flip, with the no-orphan proof and
  the one-stale-delivery bound; fixtures 3 and 7.
- **FJA-R1-DIGEST-TRANSITION-LOSS** → section 4 "Transitions survive the
  tick": write-ahead journal chosen and specified, ordering law, loss
  proof on the digest's traced exact-retry dedup; fixture 12.
- **FJA-R1-READ-BOUND** → section 4 read set: the stat replaced by one
  open-and-decode per candidate serving raise dedup and NAG; the full
  reader banned from the sweep; boundedness argued on candidates, not
  history; fixture 13.
- **FJA-R1-PROTOCOL-WRITER-PROOF** → fixture 11: the real
  `dispatch.RecordProtocolError` path drives the fixture; the nested
  violation rendering is asserted in facts and message.
