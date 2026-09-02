# Failed-job attention — design

Goal: failed-job-attention (plans/goals/failed-job-attention.md, revision 11).
Mode: design. Date: 2026-09-02. Appetite: 1.5h build; the SIMPLEST design
honoring the existing episode pattern wins. **Design revision 3**: folds the
four material findings of records/misc/failed-job-attention-critique-r2.md
(fold record in section 11). Two of the four were regressions revision 2's
folds introduced, and this revision REMOVES those folds rather than patching
them: the write-ahead transition journal is gone (every narrated transition
is now derived from committed state on the next tick), and the birth-token
fallback chain is gone (the design blocks on goal job-record-birth-token
instead of hashing bytes the tree cannot keep stable). Revision 2's fold
record stands in section 10.

The one-sentence design: the steward tick gains one bounded sweep that, for
every claimed goal, raises a durable escalation episode in the EXISTING alert
episode store when a delegate job under that goal sits in terminal failure
with its chain open, and when a breach-stop fence stands awaiting resume —
under this design's OWN episode namespace (`escalation-<digest>`), disjoint
from the alert-channel design's future `alert-<digest>` namespace, whose
landing retires this sweep (section 2) — and surfaces each episode through
what exists today: the durable notification queue, the narrator digest, the
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
  without clearing, is class-agnostic, and takes ONLY the alert lock — never
  the arbitration lock.
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
- **The digest already narrates standing conditions by re-emission, not by
  a journal**: `NarrateDigest` (`narrate.go` 47–95) is called on every
  successful tick with the tick's in-memory result and emits the SAME entry
  for a standing notify verdict on every tick (82–87, source id built from
  the verdict and head commit); `narratordigest.Append` (`digest.go`
  109–149) composes each entry's signature from kind, flattened text, and the
  source marker and skips any entry whose exact signature already appears in
  the digest body. Re-emitting an identical entry on a later tick is
  therefore idempotent, and a digest that grows only on genuine change is
  the existing pattern — one this design now reuses instead of journaling.
- **The pending queue's exact primitives** (`internal/steward/intervene.go`
  280–349): one JSON file per pending message at
  `artifacts/agents/steward/pending/<nonce>.json`; `QueueNotification`
  (299–314) is an atomic durable write keyed by nonce, `PendingNotifications`
  (317–344) lists and decodes the whole directory, and `MarkDelivered`
  (347–349) is `os.Remove` of the nonce-named file — removal by nonce is the
  queue's only retirement primitive.
- **Delivery runs OUTSIDE the tick's lock, on a snapshot**: `RunTick` takes
  the arbitration lock (`tick.go` 110–114, `AcquireArbitration` is a blocking
  exclusive flock, `arbitration.go` 28–43) and releases it when it returns;
  every `DeliverPending` caller — the resident runner (`runner.go` 131) and
  the external tick command's error and success branches
  (`steward_verbs.go` 236, 270) — calls it AFTER `RunTick` has returned, so
  no caller holds the arbitration lock during delivery. `DeliverPending`
  decodes the whole queue once (`notify.go` 65) and then sends each decoded
  message without re-checking that its file still exists (83–96). Two
  overlapping passes (the resident runner plus a `metasystem steward tick`
  invocation) can therefore each hold the same message in memory while a
  sweep withdraws its file. Precedent for holding a lock across an external
  send already exists in the tree: the tick's deferred health completion
  (`tick.go` 131–136, registered after the lock's deferred release at 114
  and therefore run BEFORE it) calls `UpdateAlertEpisodes`, which holds the
  exclusive alert lock across `Deliver` (`alert_episode.go` 341), so a health
  alert is sent today with the arbitration lock held.
- **The tick's fallible stretch and its degraded exit**: between the sweep's
  insertion point (immediately after `ReapContinuations`, `tick.go` 170–173)
  and `NarrateDigest` (230) sit ledger attention, evidence loading, marks,
  and the decision — several of which degrade or fail the tick (192, 196).
  `degradedTick` (320–334) queues one degraded-verdict notification and
  returns WITHOUT reaching `NarrateDigest`, so a tick that degrades after
  the sweep narrates nothing from that sweep; an in-memory-only transition
  report does not survive that stretch, which is why every narrated
  transition must be re-derivable from durable state (section 4).
- **The job record vocabulary** (`internal/dispatch/record.go`): terminal
  statuses are `completed`, `failed`, `cancelled`, `timeout` (45–47).
  Records carry `goalId`, `goalRevision`, `role`, `round`, `parentJob`,
  `error`, `protocolError.violation`, `endedAt`, `createdAt`, `startedAt`,
  `reviews`; `chainClosed` is terminal metadata (92–95). There is no
  `failureReason` field — the input brief's word for it maps to `error`.
  `RecordCreate` (222–273) refuses only while the record file exists
  (246–247), so a job id is lawfully reused once evidence GC has pruned the
  old record.
- **No shipped record field identifies an incarnation** (the executable
  spike, records/misc/alert-channel-spike-verdicts.md, verdict 1, run
  against the real writers): `createdAt` is absent from `immutableFields`
  (60–75), optional at create, and `RecordCAS` rewrote it; `startedAt` and
  `claimEpoch` are immutable (63) but optional and caller-supplied;
  `operationId` is the job id by default (`build.go` 150, 367, 478–479) or
  a caller-supplied value (598), so it repeats on every reuse or is under
  the caller's control; inode and file birth change on every atomic
  rewrite. The spike's implied rule is
  goal job-record-birth-token (plans/goals/job-record-birth-token.md, state
  queued, 4h box): every create path mints a birth generation — timestamp
  plus nonce — under the record lock, ignores any caller value, and the
  field joins `immutableFields`. The alert-channel design's slice 1 already
  lands only after that goal (alert-channel-design.md §11a.12, "Slice
  placement").
- **Blocking is a first-class goal edge**: a goal's `BlockedBy` list refuses
  `claim` while any blocker is not done (`internal/goal/verbs.go` 458–462),
  and a CLAIMED goal can never be given a new blocker — the editor refuses
  with "park or release first" (1099–1107). Adding this design's blocker
  to goal failed-job-attention, which is claimed today, is therefore a
  release-or-park, edit, and later re-claim by the coordinator, not a bare
  edit.
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
  (`internal/evidence/gc.go` 375–449) prunes mirrored terminal records after
  a 5,400-second default grace window measured from the mirror time (444).
- **The adjacent landed design** (`plans/alert-channel-design.md`, §11a):
  specifies, for these same two real-world conditions, its OWN classes
  (`delegate-job-failed`, `stop-awaiting-resume`), episode ids of exactly
  `alert-<64-hex-digest>`, a facts contract that REQUIRES
  `answerAction`/`answerReason` derived at journal time, a skip law that
  treats any existing episode at its digest as already-minted, and a stop
  alerting condition of successful `VerifyStopBatchComplete` — a COMPLETE
  batch, not a bare fence. Those are that goal's contracts; this design
  deliberately does not write into that namespace (section 2). That design
  contains no mention of this design's `escalation-` grammar today
  (exact search, 2026-09-02); the migration sentence it must add is stated
  in section 2 and in the goal-facing residual list (section 9).

## 2. The governing seam decision (revised; folds FJA-R1-CHANNEL-PARTIAL-FACTS, FJA-R1-STOP-PREDICATE, FJA-R2-CHANNEL-MIGRATION-UNOWNED)

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

Therefore this design takes ITS OWN identifier namespace and schema, and
never writes a partial record under the channel's final identifiers:

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

**The migration window has one owner and one end (folds
FJA-R2-CHANNEL-MIGRATION-UNOWNED).** Revision 2 left the duplicate-alert
window at channel arrival to "the channel's own migration decision" with no
owner and no bound; because this design's failed-job episodes are never
auto-cleared, two designs each nagging the same standing record would be two
CONTINUING nags, not a temporary duplicate. The rule now:

- **Owner: goal alert-escalation-channel, in its slice-1 landing commit.**
  This design's sweep is a stand-in for that goal's two slice-1 producers
  over the same two conditions; the goal that replaces it retires it. The
  channel's landing MUST include, in the same commit that wires its
  producers into the tick, the following retirement (the sentence its
  design must add): "Slice 1 retires the failed-job-attention sweep: it
  removes `SweepWorkEscalations` and `DrainNarratedStopMarkers` from
  `RunTick`, and its first tick withdraws every pending notification whose
  nonce matches `escalation-[0-9a-f]{64}`, marks every
  `alerts/escalation-<64hex>.json` episode `Cleared`/`ClearedAt` (never
  `Acknowledged`, which is a human word), and removes every
  `alerts/escalation-stop-open/` marker; the files stay as history under the
  health-join exclusion, which is kept."
- **End condition**: the first completed tick after the channel's landing.
  From that tick, only the channel raises for these conditions; every
  private episode is cleared history.
- **The bound on the duplicate**: ZERO private nag deliveries after that
  first post-landing tick, because the sweep that re-queues private nags no
  longer runs and the pending files are withdrawn before that tick's
  delivery pass (the runner delivers only after `RunTick` returns,
  `runner.go` 131). Before that tick, the private nag queued by the LAST
  pre-landing sweep can be delivered at most once by the pre-landing
  delivery pass — an ordinary delivery of a then-valid condition, not a
  duplicate. In the `watch` alerts view the human sees, for a condition
  still standing at the landing, one live channel episode and one cleared
  private episode — visible, bounded, and terminal.
- **Why the channel and not this design**: this design could self-retire
  only by testing for the channel's episodes, which means computing the
  channel's digest tuple — that design's revising contract (revision 13
  today). A hidden dependency on another design's literal bytes would break
  silently on that design's next revision; a retirement written into the
  landing that introduces the successor cannot drift.

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
  newline. The birth element is OPAQUE BYTES to this encoding: the vectors
  pin the joining, hashing, and casing, not the token's internal shape,
  which goal job-record-birth-token owns. Pinned vectors (recomputed
  2026-09-02 with `shasum -a 256`):
  `escalation-failed-job\nimplementer-c002e6035a243bdbc1400067\n2026-08-31T18:02:11Z`
  (an arbitrary token byte string) →
  `cfce3b7f36d66fdd6fd777ba613bbe753cf6f2dbc92def0154a936b60587c3d9`;
  the empty-birth form (`escalation-failed-job\n<jobId>\n`, pre-contract
  records only, below) →
  `65e76dc4bfba829b3cf2939899ba71f1a63c5e386c8b322c259ce4c25b9b973a`.
  Dedup per job incarnation is this digest.
- **The birth token, blocked rather than faked (folds FJA-R1-BIRTH-ABA and
  FJA-R2-BIRTH-ABA-REMAINS)**: the tuple's third element is the record's
  MINTED birth generation, verbatim — the field goal job-record-birth-token
  lands — and NOTHING else. Revision 2's fallback chain (`createdAt`, else
  `startedAt`, else empty) is removed: the spike traced in section 1 proved
  through the real writers that every shipped candidate is optional,
  caller-supplied, or rewritable, so a lawful identifier reuse CAN repeat
  the fallback bytes and the old episode would then permanently suppress
  the new incarnation's alert. That is not a bounded loss; it is the
  incident's silence reproduced by design, and no field the tree holds
  today prevents it (section 1's spike trace: `createdAt`, `startedAt`,
  `claimEpoch`, `operationId`, inode — each refuted). **Decision: this goal
  declares `BlockedBy: job-record-birth-token`**, so the build cannot start
  before the mint exists, and the sweep reads the minted field by the name
  that goal lands (the implementation brief names it; it is the one open
  literal in this design and it is owned upstream). Why this choice over
  inventing a per-incarnation key here: the only sound key is a
  record-lock mint, which is a change to the dispatch record contract that
  goal already owns and the alert-channel design already waits for; a
  second mint in this design would be the same 4-hour item done twice. The
  schedule consequence is stated plainly in section 9's residual list.
  **The pre-contract rule (an absent token never creates a second episode
  for an old record)**: a record without the minted field is a
  pre-contract record; its birth element is EMPTY, forever. No fallback
  bytes are ever hashed, so no record's digest ever changes: a record
  either carries the mint from its creation (the field is immutable and
  minted only at create) or never carries it. Because this design ships
  only after the mint exists, no episode was ever minted under a fallback
  digest, so there is no earlier digest to migrate from — the upgrade
  revision 2 promised no longer exists as a step. ABA among pre-contract
  records is impossible by construction: at land time at most one
  pre-contract incarnation per job id exists on disk (one file per id), any
  earlier incarnation was pruned before this sweep existed and never had
  an episode, and every incarnation created after landing carries a mint
  and therefore a different digest from the pre-contract one. Fixture 4b
  proves both halves.
- **Facts** (`facts` map, this design's own keys, all mechanical at raise
  time): `goalId`, `jobId`, `birth` (the minted token's exact bytes; empty
  for a pre-contract record), `reason` (the record's `error` field verbatim,
  with `protocolError.violation` appended after `: ` when present, else
  `""`), `role`, `chainRoot` (the parentJob-walk result, `""` on any walk
  refusal), `reviews` (the record's `reviews` field verbatim, may be `""`).
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
  is the dedup per stop. Pinned vector (recomputed 2026-09-02 with
  `shasum -a 256`): `escalation-breach-stop\nalert-escalation-channel\n8` →
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
- **Lifecycle (revised: the marker outlives the clear until the clear is
  narrated)**: cleared ONLY by the positive fence-gone proof. Each tick the
  sweep lists `alerts/escalation-stop-open/` (bounded by open stops, because
  draining removes markers) and evaluates, per marker, against the goal
  projection already in hand: fence still bound (the goal exists, is
  claimed, and carries a `StopFence` with that marker's revision) → the
  marker stays. Fence GONE → the clear: read the marker's digest, open the
  episode, set exactly `Cleared`/`ClearedAt` if present and uncleared, and
  report the episode as cleared for this tick's narration (section 4). The
  marker is NOT removed by the sweep: it is removed by `RunTick` only after
  this tick's `NarrateDigest` has returned success (section 4, "the
  post-narration drain"), so the durable pair "marker present, episode
  cleared" is exactly the state "clear committed, narration not yet
  proven", and the next tick re-derives the cleared narration from it. A
  fence-gone marker whose episode is ABSENT has nothing to narrate and is
  removed by the sweep at once. Anything unreadable HOLDS (a clear is never
  inferred from a failed read). Acknowledgment stops the nag but never
  clears.

## 4. Tick integration, read set, and today's delivery

**One new sweep**, `SweepWorkEscalations(repoRoot, now)`, in a new
`internal/steward/escalation.go`, called from `RunTick` immediately after
`ReapContinuations` (tick.go 170–173) — after `runBreachStopCustodian`, so a
fence installed this very tick escalates this very tick, and after the reap,
so records the reaper just finished are seen in their final state. On error
the tick takes the existing `degradedTick` path with a named reason
(`"work escalation sweep failed: …"`) — an unreadable store is never a silent
verdict. The sweep's writes (markers, episodes, queue entries, withdrawals)
run under the existing alert flock; the arbitration lock the tick holds
serializes sweeps against each other and against delivery (below).

**Read set, bounded (folds FJA-R1-READ-BOUND; candidacy widened by
FJA-R2-TRANSITION-PHANTOM's fold)**: one `goal.Project` over the live tree
(the same class of read `FindBreachStops` performs every tick; the sweep
does its own call rather than refactoring the custodian — simplest, and the
projection is the bound); one `ReadDir` of `artifacts/agents/jobs` with one
small-JSON decode per record (~100 today, growth bounded by evidence GC);
one `ReadDir` of `alerts/escalation-stop-open/` (bounded by open stops);
**one open-and-decode of the digest-named episode file per candidate** — a
candidate being (3a) every decodable record whose `status` is `failed` or
`timeout` with a nonempty `goalId`, REGARDLESS of its chain state or its
goal's claim state, or (3b) a standing fence, or a listed marker — because
a stat proves only existence while the NAG needs each candidate episode's
acknowledgment state and write-once message, and because the narration
(below) must re-derive a raise from the episode itself even after its nag
off-switch has flipped. The open serves the raise dedup (absent file →
raise, subject to 3a's full predicate), the NAG (present file → read
`AcknowledgedAt` and `Message`), and the narration (present file → the
raised entry). The widening costs one open attempt per closed-chain or
unclaimed-goal terminal record per tick — an `ENOENT` when no episode was
ever raised for it — and is bounded by the jobs listing, never by history.
One `ReadDir`-and-decode of the pending queue (`PendingNotifications`,
bounded by queued messages) serves the withdraw phase below. The sweep
NEVER calls the full reader `AlertEpisodes()`: retained episodes with no
standing record, fence, or marker — the accumulating history — are never
opened. An unreadable CANDIDATE episode fails the sweep into the degraded
tick (honest, like every other torn read), while non-candidate files are
not read at all (fixture 13). No network, no processes, no record writes.

**What one sweep does**, in order: (1) enumerate claimed goals from the
projection; (2) stop-class pass — for every standing fence, open the
candidate episode; absent → raise (marker, then episode); then the clear
phase — for every fence-gone marker, set `Cleared`/`ClearedAt` on its
episode if present and uncleared, report it cleared, and leave the marker
(3b); (3) failed-job pass — for every candidate record, open the candidate
episode; absent and 3a's full predicate holds → raise (episode); present →
report it standing; (4) the NAG-or-withdraw phase (folds
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
matches no other producer's nonce (`verdict-*`, `ledger-attention-*`,
`revive-failure`, and intent nonces never fit it). No-orphan proof: every
producer-class pending file is examined every sweep and either re-queued
(condition stands, unacknowledged) or removed (anything else), so a queued
notification whose off-switch flipped never outlives the next sweep.
Withdrawal never depends on delivery succeeding, so a down notification
channel cannot strand a withdrawn condition either. (5) return a report
(`[]WorkEscalationReport{Class, EpisodeID, GoalID, JobID, StopID, Message,
Standing, Cleared}`) on `TickResult` for the narration and the
post-narration drain.

**Delivery under the tick's own lock (folds FJA-R2-PENDING-SNAPSHOT-RACE)**.
Revision 2 claimed "at most one stale delivery" from the runner's
sequential tick-then-deliver order alone; section 1's trace shows that
order holds within ONE process while a second pass (the external tick
command, or a second runner instance during a hand-over) can hold the same
decoded queue snapshot and send after a sweep withdrew the file. The one
lock rule: `DeliverPending` keeps its whole-queue snapshot but performs
each item's delivery as one critical section under the arbitration lock —
the same `AcquireArbitration` the tick holds — in this order: acquire;
re-stat `pending/<nonce>.json` and skip the item if it is gone; for a nonce
matching `escalation-[0-9a-f]{64}`, load `alerts/<nonce>.json` and, if the
episode is absent or `Acknowledged`, remove the pending file and skip
(delivery itself enforces the acknowledgment off-switch without waiting for
a sweep); otherwise send; on success mark the intent notified and remove
the pending file; release. The lock is held across the send (bounded by the
existing 15-second `notifyTimeout`), which the tree already does for the
health alert (section 1's defer-order trace). Lock discipline: no caller
may hold the arbitration lock when calling `DeliverPending`; all three
shipped call sites call it after `RunTick` has returned, and the fixture
for this section pins that. **The bound that actually holds, stated**: a
message withdrawn by a sweep is delivered ZERO times after the withdrawal,
by any number of overlapping passes, because withdrawal happens under the
arbitration lock and every send re-checks existence under that same lock;
one queued file is delivered at most ONCE per queueing, because the send
and the removal are one critical section; after an acknowledgment, ZERO
deliveries occur unless the acknowledgment lands inside a single item's
critical section between the episode re-check and the send (acknowledgment
takes only the alert lock, section 1), in which case exactly ONE delivery
follows it and the next sweep queues nothing. For the off-switches delivery
does not re-check — chain closed, goal released, record gone — the bound
is at most one delivery after the flip: the file queued by the last sweep
before the flip, which no later sweep re-queues. Fixture 3b proves the two
overlapping-pass cases.

**Transitions reach the digest by derivation from committed state (folds
FJA-R1-DIGEST-TRANSITION-LOSS; re-folded by FJA-R2-TRANSITION-PHANTOM,
which REMOVES revision 2's write-ahead journal)**. Revision 2 journaled
each transition durably BEFORE the state change and had `NarrateDigest`
emit every journal entry; the critique proved that a failed episode write
after the journal append leaves a durable entry narrating a raise that
never existed. There is no journal now. Every narrated transition is
derived, on every tick, from state that is already durable when it is
read, using the tree's own re-emission-plus-dedup pattern (section 1's
digest trace):

- **Raised**: for every escalation episode the sweep opened this tick and
  found PRESENT on disk (both classes; a raise performed this tick counts
  only after its durable write returned success), `NarrateDigest` emits one
  lowlight `{SourceType: "episode", SourceID: <episodeID>, Text:
  <episode.Message>}`. The text is the write-once `Message` verbatim (the
  digest flattens whitespace), so the entry's signature is byte-identical
  on every tick and `narratordigest.Append`'s exact-retry dedup makes the
  re-emission a no-op after the first success. No phantom is possible: an
  entry exists only for an episode file that exists.
- **Cleared** (stop class only; failed-job episodes never clear): for every
  fence-gone marker whose episode is `Cleared` on disk, `NarrateDigest`
  emits one highlight `{SourceType: "episode", SourceID:
  <episodeID>-cleared, Text: "Goal <goalId> revision <revision> was resumed;
  its breach-stop escalation is cleared."}`, derived from the episode's
  facts. The marker's survival past the clear (3b) is what makes this
  derivable next tick.
- **The post-narration drain**: immediately after `NarrateDigest` returns
  success (tick.go 230–232), `RunTick` calls
  `DrainNarratedStopMarkers(repoRoot, result.WorkEscalations)`, which
  removes, under the alert lock, exactly the markers of this tick's cleared
  entries. A failure there degrades the tick by name; the markers stay and
  the next tick re-derives and retries.
- **Loss proof, every crash point**: a crash after the episode or `Cleared`
  write and before `NarrateDigest` (the fallible stretch, or a degraded
  exit) leaves the episode present (or cleared with its marker present);
  the next tick's sweep opens it again — the widened candidacy keeps a
  failed-job episode a candidate even if its chain closed or its goal was
  released in between — and re-derives the same entry, which the digest
  appends once. A crash between `NarrateDigest` and the drain leaves the
  marker; the next tick re-derives the cleared entry (dedup absorbs it) and
  drains. A failed episode write fails the sweep, degrades the tick, and
  narrates nothing — there is no state to derive from. The one residual:
  a raise whose tick crashed before narration AND whose record is removed
  by hand before the next tick (evidence GC cannot do it: its 5,400-second
  grace window is at least eight default ticks) leaves an episode and a
  queued nag but no digest line; disclosed, and no second durable record
  is spent on it.
- Emission is a per-tick re-derivation whose dedup makes the digest grow
  only on genuine change, so the digest stays readable; this is the same
  shape the standing notify verdict already uses (narrate.go 82–87).

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
extend the same exclusion to its own `alert-<64hex>` grammar when it lands
and KEEPS this one (section 2's retirement leaves the private files as
history that the health path must still never open). The edit is REQUIRED
now: without it, the first healthy tick wipes both new classes. The
stalled-idle lifecycle is untouched: its verdict still rides the notify
queue, and the health-joined episodes behave byte-identically (fixture 8
proves it).

## 6. Fixtures (all deterministic: temp repos, injected clock, no sleeps, the
steward package's existing test idiom — `notify_test.go`'s file-appending
notify command, `arbitration_test.go`'s one-shot `beforeArbitrationWait`
hook)

1. `TestFailedJobUnderClaimedGoalRaisesEscalation` — claimed goal file plus a
   `failed` job record naming it and carrying a minted birth token; one
   sweep raises `alerts/escalation-<digest>.json` with the right class,
   facts (`birth` equals the token bytes), and message; the digest matches
   the pinned vector recomputed in-test from the tuple bytes; one
   notification with nonce `escalation-<digest>` is pending; the report
   lists the episode as standing.
2. `TestFailedJobEscalationIsWriteOnceAndNagsPerTick` — a second sweep leaves
   the episode file byte-identical, creates no second episode, and re-queues
   the notification after a simulated delivery (`MarkDelivered`).
3. `TestOffSwitchStopsTheNagAndWithdrawsTheQueuedMessage` — with a
   notification already durably queued, flipping `chainClosed`, releasing
   the goal, or removing the record each make the next sweep REMOVE the
   pending file (the withdraw rule) and queue nothing new; the episode
   remains uncleared in all three; a pending `verdict-*` nonce in the same
   queue is untouched.
   3b. `TestOverlappingDeliveryPassesNeverDeliverAWithdrawnEscalation`
   (folds FJA-R2-PENDING-SNAPSHOT-RACE) — two passes over one queue. Case
   A: pass one decodes the queue holding `escalation-<digest>`; the
   one-shot `beforeArbitrationWait` hook then, before pass one can take the
   lock, acknowledges the episode and runs a full sweep (which withdraws
   the file under the lock); pass one proceeds and the file-appending
   notify command records ZERO deliveries. Case B: the notify command
   itself acknowledges the episode as its side effect (the acknowledgment
   inside the critical section); exactly ONE delivery is recorded, the
   pending file is gone, and the next sweep queues nothing. Case C: two
   passes decode the same queue and run in sequence with no intervening
   sweep; exactly ONE delivery is recorded — the second pass's re-stat
   finds the file gone. A fourth assertion pins the lock discipline: the
   three shipped `DeliverPending` call sites run after `RunTick` returns
   (a static check over the tick command and runner sources, so a future
   caller holding the lock fails the test rather than deadlocking).
4. `TestTimeoutRaisesAndCancelledCompletedUnclaimedDoNot` — the predicate's
   four exclusions, one sweep each.
   4b. `TestBirthTokenSeparatesIncarnationsAndPreContractRecordsStayStable`
   (folds FJA-R2-BIRTH-ABA-REMAINS) — a failed record with minted token T1
   raises episode D1; the record file is removed as GC would; a new record
   with the same job id and minted token T2 fails; one sweep raises a
   second episode D2 ≠ D1 and nags it even though D1 is acknowledged. Then
   the pre-contract half: a record with NO minted field (and with
   `createdAt` and `startedAt` present, to prove they are ignored) raises
   the empty-birth digest; rewriting those two fields and sweeping again
   changes nothing — same episode, no second file.
5. `TestBreachStopWritesMarkerThenEpisodeWithResumeCommand` — claimed goal
   with a `StopFence`; after one sweep the marker exists under
   `escalation-stop-open/` with the digest as its entire content, the
   episode exists, and the message contains `goal resume --id`, all four
   budget flag names, and the retry-after-a-tick sentence; digest matches
   the pinned stop vector recomputed in-test.
6. `TestResumeClearsStopEpisodeAndTheMarkerDrainsAfterNarration` — the fence
   removed (as resume does); the next sweep alone sets exactly
   `Cleared`/`ClearedAt`, reports the episode cleared, and LEAVES the
   marker; `NarrateDigest` then `DrainNarratedStopMarkers` remove it; a
   further sweep raises nothing and reports nothing; a fence-gone marker
   with an absent episode drains in the sweep itself.
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
   `RunTick`; so does an unreadable CANDIDATE episode file; in both cases
   the digest gains no entry.
10. `TestTickRaisesFailedJobEscalationEndToEnd` — one full `RunTick` in a
    temp repo proving the wiring: episode on disk, notification pending,
    digest entry present exactly once; a second `RunTick` adds no digest
    line.
11. `TestProtocolErrorRecordRaisesWithNestedViolation` (folds
    FJA-R1-PROTOCOL-WRITER-PROOF) — the fixture drives the REAL writer: it
    creates a running-status record and invokes
    `dispatch.RecordProtocolError` to stamp it, never hand-writing the
    terminal shape; one sweep raises the episode, and the fixture asserts
    the facts `reason` and the message both render
    `protocol_error: <violation>` — the `error` field with the nested
    `protocolError.violation` appended per 3a.
12. `TestTransitionsAreDerivedNotJournaled` (folds
    FJA-R1-DIGEST-TRANSITION-LOSS and FJA-R2-TRANSITION-PHANTOM) — raise
    half: run the sweep alone (episode written, digest untouched),
    simulating a tick that died before `NarrateDigest`; flip `chainClosed`
    true (the off-switch); a full `RunTick` still emits the raised entry
    exactly once (widened candidacy); a further tick adds nothing. Clear
    half: fence removed, sweep alone (episode cleared, marker present),
    then a full tick: the cleared highlight appears once and the marker is
    gone; another tick adds nothing. Phantom half: make the alerts
    directory unwritable so the episode write fails; `RunTick` degrades by
    name and the digest gains NO entry; remove the record; a further healthy
    tick narrates nothing for it — no durable state ever said it was raised.
13. `TestSweepNeverOpensNonCandidateEpisodes` (folds FJA-R1-READ-BOUND) — a
    syntactically corrupt `escalation-<hex>.json` for which no record,
    fence, or marker stands does not perturb the sweep (proving history is
    never opened), while the same corrupt bytes at a CANDIDATE's digest
    path — including a closed-chain terminal record's path, under the
    widened candidacy — degrade the tick.

## 7. Blast radius — every consumer of episode listings, checked

- **`watch` alerts view**: raw-reads every regular `alerts/*.json`
  (records/watch-verb/watch-verb-design.md, table row "alerts"); the new
  episodes appear automatically; `escalation-stop-open/` is a subdirectory
  and is skipped by both the watch glob and `loadAlertEpisodesUnlocked`'s
  `IsDir` check. No other file is added to the alerts directory (revision
  2's `escalation-transitions.json` no longer exists).
- **`AlertEpisodes()` and the loader invariants**: new episodes satisfy every
  loader requirement (section 3 preamble); the loader needs no exclusion
  because nothing but episodes lives at `alerts/*.json`.
- **`metasystem health`** (`runStewardHealth`): calls the now-scoped
  `UpdateAlertEpisodes`; output bytes unchanged; escalation episodes are
  invisible to it by the filename grammar.
- **`health acknowledge-alert`**: unchanged and class-agnostic; the
  75-character ids pass `validEpisodeID`. Its alert-lock-only discipline is
  what bounds the post-acknowledgment delivery to one (section 4).
- **The narrator digest and its consumers** (`steward digest-pending`, the
  seat): additive entries only, re-derived per tick and absorbed by the
  digest's exact-retry dedup after the first success, so consumers see each
  transition once.
- **`DeliverPending` and every notification producer**: the per-item
  critical section changes no message bytes and no producer; every other
  nonce class is delivered exactly as today plus one existence re-stat
  under the lock. The health alert's own send path (`UpdateAlertEpisodes`)
  is untouched.
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
  skip law), keys its own episodes on the SAME minted birth token this
  design now waits for, retires this sweep in its slice-1 landing under
  section 2's rule, takes over transport under its own ids, and extends
  the health-join exclusion to its own grammar while keeping this one.
- **Goal job-record-birth-token**: gains nothing from this design except a
  second consumer of its minted field (the first is the channel); Ruling R
  on that goal — "whoever builds this runs every reader of the record
  identity" — will find this sweep as one such reader once it exists.

## 8. Scope fence

No new delivery machinery; no external sends; no transport changes; no reaper
or breach-stop behavior changes (the sweep only reads their records); no GC
changes; no counselor changes; no changes to stalled-idle verdicts, health
verdicts, or the acknowledge verb; no writes into the channel's
`alert-<digest>` namespace; no journal file; no change to the dispatch
record contract (the birth token is the sibling goal's). Implementation
touches: new `internal/steward/escalation.go` (+tests) including the sweep,
the withdraw phase, and `DrainNarratedStopMarkers`; two additive
`AlertEpisode` fields; the filename-scoped health-join loops in
`alert_episode.go`; the sweep call, the report field, and the
post-narration drain call in `tick.go`; the derived transition entries in
`narrate.go`; and the per-item critical section in `notify.go`'s
`DeliverPending`.

## 9. Self-grade

**Grade: A−.** Strengths: every mechanism is an existing, traced pattern
(episode store, notify queue nonce dedup and removal, the digest's
re-emission-plus-dedup narration of standing conditions, the arbitration
lock the tick already holds and already holds across a send, degraded-tick
honesty), and every predicate, digest byte, lifecycle rule, read, lock
order, and crash boundary is mechanical — the eleven findings across two
critique rounds each closed with a rule an implementer applies without
deciding anything; this revision is SMALLER than revision 2 (no journal, no
fallback chain), which is what the design's own rule asks for; the three
pinned vectors were recomputed, not trusted. Weaknesses, honestly: (1) the
build is blocked behind goal job-record-birth-token, a queued 4-hour item —
the honest price of not shipping a refuted identity, and a schedule call
the coordinator must make against Wido's "before you do anything else"
(residual list below); (2) delivery now holds the arbitration lock for up
to 15 seconds per message, which delays a concurrent tick or worker
enrollment by that much — the same exposure the health alert's send already
carries, now on a second path; (3) one narration residual remains (a
crashed raise whose record is hand-removed within one tick loses its
digest line, never its episode or nag); (4) the nag rides a notification
channel that is unconfigured on Linux hosts today, so until the channel
goal lands, visibility there is the durable pending queue, `steward
status`, the digest, and `watch` — stated, not hidden, and identical to
stalled-idle's current reality.

**Goal-facing residuals (for the goal record's Next step; each is a
sentence the coordinator carries, not a promise this design makes)**:

- Goal failed-job-attention takes `BlockedBy: job-record-birth-token`. The
  goal is claimed, and the editor refuses a blocker on a claimed goal
  (section 1), so this is: release or park failed-job-attention, add the
  edge, and re-claim once job-record-birth-token is done; if Wido's
  priority order stands, job-record-birth-token is the next item claimed,
  not this one.
- The implementation brief names the minted birth field by the name
  job-record-birth-token lands, and adds one fixture vector using a real
  minted value once that shape exists.
- Goal alert-escalation-channel's design must add section 2's retirement
  sentence to its slice-1 landing: remove `SweepWorkEscalations` and
  `DrainNarratedStopMarkers` from `RunTick`, withdraw every pending nonce
  matching `escalation-[0-9a-f]{64}`, mark every
  `alerts/escalation-<64hex>.json` episode `Cleared`, remove every
  `escalation-stop-open/` marker, and keep the health-join exclusion for
  the private grammar.

**Reject this design if any of these fail**: a recomputation of the three
pinned digest vectors in section 3 disagrees; the `UpdateAlertEpisodes`
scoping change alters any byte of an existing health-episode fixture's
expected output; the `escalation-<64hex>` grammar is shown to collide with
any existing or channel-design filename, nonce, or id grammar; the
no-orphan proof in section 4 is shown to leave any pending-queue file
unexamined by the sweep; a `DeliverPending` caller is found that holds the
arbitration lock (a deadlock, not a delay); any narrated entry is shown to
be derivable from state that is not durable at the time it is read; a
shipped record field is shown to identify an incarnation soundly through
the real writers, which would make the block unnecessary (the spike says
none does); or the orchestrator rules the withdraw phase or the per-item
delivery lock out of the 1.5h appetite — in which case the fallback is NOT
revision 1's channel-identity variant nor revision 2's journal (both doors
are closed by critique) but an explicit scope cut recorded on the goal:
ship raise/nag/acknowledge without the per-item delivery lock (the
overlapping-pass double delivery reopens, bounded by the number of
concurrent passes) — a cut only the goal owner may accept, in writing.

## 10. Fold record — critique round 1 (records/misc/failed-job-attention-critique-r1.md)

- **FJA-R1-BIRTH-ABA** → section 3a "The birth token": revision 2 declared
  a dependency with a fallback chain; revision 3 replaces the fallback with
  a block (see FJA-R2-BIRTH-ABA-REMAINS in section 11).
- **FJA-R1-STOP-PREDICATE** → sections 2 and 3b: this design's own fence
  predicate specified on facts the tick holds; the difference from the
  channel's batch-complete predicate stated; the refuse-then-retry
  consequence carried in the message; no write-once collision remains
  because the namespaces are disjoint.
- **FJA-R1-CHANNEL-PARTIAL-FACTS** → section 2: own identifier namespace
  (`escalation-<64hex>`), own class literals, own facts schema without the
  channel's required keys; new pinned vectors computed for the new tuples.
- **FJA-R1-PENDING-LIFECYCLE** → section 4 step 4: the one withdraw rule
  for queued notifications at off-switch flip, with the no-orphan proof;
  fixtures 3 and 7. The one-stale-delivery bound revision 2 attached here
  was unsound and is re-stated under FJA-R2-PENDING-SNAPSHOT-RACE.
- **FJA-R1-DIGEST-TRANSITION-LOSS** → section 4 "Transitions reach the
  digest by derivation": revision 2's write-ahead journal was a phantom
  source and is removed; the loss proof now rests on re-derivation from
  committed state plus the digest's traced exact-retry dedup; fixture 12.
- **FJA-R1-READ-BOUND** → section 4 read set: the stat replaced by one
  open-and-decode per candidate serving raise dedup, NAG, and narration;
  the full reader banned from the sweep; boundedness argued on candidates
  (now every terminal failed/timeout goal-bearing record), not history;
  fixture 13.
- **FJA-R1-PROTOCOL-WRITER-PROOF** → fixture 11: the real
  `dispatch.RecordProtocolError` path drives the fixture; the nested
  violation rendering is asserted in facts and message.

## 11. Fold record — critique round 2 (records/misc/failed-job-attention-critique-r2.md)

- **FJA-R2-BIRTH-ABA-REMAINS** → section 3a "The birth token, blocked rather
  than faked" and section 1's spike trace: the fallback chain is REMOVED;
  the tuple's third element is the minted birth generation or, for a
  pre-contract record, empty forever; the goal declares
  `BlockedBy: job-record-birth-token` (the choice and its reason stated;
  the claimed-goal editing mechanics traced); the pre-contract rule proves
  no record's digest ever changes and no second episode can arise for an
  old record; fixture 4b. Goal-facing residual recorded in section 9.
- **FJA-R2-TRANSITION-PHANTOM** → section 4 "Transitions reach the digest
  by derivation": the write-ahead journal is REMOVED everywhere (sections
  4, 6, 7, 8); raised entries derive from episodes present on disk, cleared
  entries from cleared episodes whose marker still stands; the marker now
  outlives the clear until `RunTick`'s post-narration drain (section 3b);
  the crash between state write and digest append is covered by
  re-derivation, with candidacy widened so an off-switch flip cannot hide
  a raise from the next tick; fixture 12 rewritten with a phantom half.
- **FJA-R2-PENDING-SNAPSHOT-RACE** → section 4 "Delivery under the tick's
  own lock": one lock rule — each `DeliverPending` item is a critical
  section under the arbitration lock with existence and episode-state
  re-checks before the send — the bound restated as it actually holds
  (zero after withdrawal, at most one per queueing, zero after
  acknowledgment except inside one critical section, at most one after
  the non-re-checked off-switches); lock discipline stated; fixture 3b with
  two overlapping passes.
- **FJA-R2-CHANNEL-MIGRATION-UNOWNED** → section 2 "The migration window
  has one owner and one end": owner is goal alert-escalation-channel's
  slice-1 landing commit, which retires the sweep, withdraws private nags,
  clears private episodes, and drains markers; end condition is the first
  completed post-landing tick; the duplicate bound is zero private
  deliveries after it and one cleared private file beside one live channel
  episode in `watch`; the sentence the channel design must add is in
  section 2 and in section 9's goal-facing residual list; the reason the
  owner is not this design (a hidden dependency on the channel's revising
  digest bytes) is stated.
