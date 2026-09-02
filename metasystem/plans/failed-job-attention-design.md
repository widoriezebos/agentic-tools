# Failed-job attention — design

Goal: failed-job-attention (plans/goals/failed-job-attention.md, revision 4).
Mode: design. Date: 2026-09-02. Appetite: 1.5h build; the SIMPLEST design
honoring the existing episode pattern wins.

The one-sentence design: the steward tick gains one bounded sweep that, for
every claimed goal, raises a durable escalation episode in the EXISTING alert
episode store when a delegate job under that goal sits in terminal failure
with its chain open, and when a breach-stop fence stands awaiting resume —
using the alert-channel design's already-landed episode identity for these two
classes so the future channel consumes these exact episodes instead of
fighting them — and surfaces each episode through what exists today: the
durable notification queue, the narrator digest, the narration line, and the
`watch` alerts view.

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
- **The job record vocabulary** (`internal/dispatch/record.go`): terminal
  statuses are `completed`, `failed`, `cancelled`, `timeout` (45–47).
  Records carry `goalId`, `goalRevision`, `role`, `round`, `parentJob`,
  `error`, `protocolError.violation`, `endedAt`, `createdAt`, `startedAt`,
  `reviews`; `chainClosed` is terminal metadata (92–95). There is no
  `failureReason` field — the input brief's word for it maps to `error`.
- **The stop fence**: a claimed goal file carries `StopFence` (`StopID` of the
  form `stop-<goal-id>-r<revision>-f<epoch>`, `Revision`, `ClosedAt`,
  `Reason` such as `ELAPSED_LIMIT`) after a breach-stop closes launches
  (`internal/goal/file.go` 96–105, `internal/dispatch/stop.go` 89–99). Only
  `goal resume --id <goal> --by <name>` plus the complete budget tuple
  (`--elapsed-limit`, `--attempt-limit`, `--reserved-job-minutes-limit`,
  `--active-job-limit`) removes it (`cmd/metasystem/goalsync_mutations.go`).
  Once the stop batch is COMPLETE, `FindBreachStops` skips the goal entirely
  (`stop.go` 294–296) — a standing completed stop is invisible to today's
  tick. That silence is exactly incident finding 2.
- **Enumerating claimed goals is a read the tick already pays**:
  `FindBreachStops` runs `goal.Project` over the live tree every tick
  (`stop.go` 270–288). The jobs directory holds on the order of 100 records
  today (`artifacts/agents/jobs`), each a small JSON file; evidence GC prunes
  mirrored terminal records after a 5,400-second default grace window.
- **The adjacent landed design** (`plans/alert-channel-design.md`, §11a.10):
  already specifies, for these SAME two conditions, the classes
  `delegate-job-failed` and `stop-awaiting-resume`; two additive
  `AlertEpisode` fields (`class`, `facts`, schema stays 1); the exact digest
  encoding (class literal, then the tuple elements, joined by single LF
  bytes, SHA-256, lowercase hex); episode ids of exactly
  `alert-<64-hex-digest>` (collision-free against health ids); write-once
  episodes deduplicated by one stat of the digest-named path
  (exists-by-digest); `delegate-job-failed` never auto-cleared with
  acknowledgment as the terminal human step; `stop-awaiting-resume` cleared
  only by a positive fence-gone proof through the reversible
  `alerts/stop-open/<goal-id>-r<revision>` marker (marker written durably
  BEFORE the episode; drained on fence-gone); and the restriction of
  `UpdateAlertEpisodes`' healthy-clear and resolve-others loops to health
  episodes by filename grammar. Its three pinned digest vectors recompute
  exactly (verified 2026-09-02 with sha256sum).

## 2. The governing seam decision

The task's seam instruction is: same episode store, this goal's episodes are
the simple immediate class, the channel later consumes them. Two identities
for the same failed job would mean double episodes and double alerts the day
the channel lands, and the channel's evidence-GC retention pin names only its
own digests. Therefore this design ADOPTS the channel's episode identity now
— class literals, digest encoding, `alert-<digest>` addressing, write-once
law, lifecycle, and the stop-open marker — while keeping delivery on today's
machinery (the durable notification queue, the digest, the narration). What
this goal does NOT take from the channel design: the transport sender
(`DeliverDueAlerts`), the escalation ladder, the answer-derivation table
(§11a.8's `answerAction`/`answerReason`), the retention pin (§11a.12), and
every external adapter. Those stay with goal alert-escalation-channel.

Deviations from the m3 input brief (input material, not a certified design),
each forced by a traced fact: episode ids are `alert-<64-hex>` rather than
`escalation-failed-job-<jobId>` (seam identity above; also an unbounded job
id could overflow the 96-character id validator); episodes are write-once
rather than "refreshed" (the channel's skip law — both classes' facts are
immutable at their source, so there is nothing to refresh; the NAG is
per-tick, the episode is not rewritten); the failure text comes from the
record's `error` field (no `failureReason` exists); `timeout` is included
beside `failed` (the channel's class definition covers both, and a worker the
reaper killed at its cap with nobody watching is this incident's exact
shape); the breach-stop clear is the marker-drain fence-gone proof rather
than a free-form "resumed goal" check (revision 10 of the channel design
proved the naive form regresses: the digest filename is one-way).

## 3. The two classes, exact

Both are `AlertEpisode` records with `Schema: 1`, `EpisodeID: "alert-" +
digest`, `Digest: <64-hex>`, `Message` set once at creation, `OpenedAt: now`,
`Attempts: []` (non-nil, empty), `TransportResult: PENDING` (truthful: the
episode-level transport is the channel's future job; today's delivery rides
the notification queue), plus the two additive fields `class` and `facts`
from channel §11a.10. `TransportResult` stays `PENDING` until that goal's
sender lands — that is the seam working, not dead state.

### 3a. delegate-job-failed

- **Raise predicate**, evaluated per tick over `artifacts/agents/jobs/*.json`:
  the record decodes; `status` is `failed` or `timeout`; `goalId` is nonempty
  and names a goal whose projected state is claimed (any machine — a local
  job record was dispatched here, and a dead delegate under someone's claim
  is attention-worthy here); `chainClosed` is absent or false; and
  exists-by-digest is false (no `alerts/alert-<digest>.json`). `cancelled`
  is excluded: cancellation is an operator's own act, already acknowledged by
  construction. `completed` is excluded trivially.
- **Digest**: SHA-256 lowercase hex of
  `delegate-job-failed` + LF + job id + LF + birth token, no trailing
  newline. The birth token is the record's minted birth generation when the
  depended-on contract (goal job-record-birth-token) lands; until then the
  fallback chain the channel design fixed: `createdAt`, then `startedAt`,
  else empty. Pinned vectors (recomputed, they match):
  `delegate-job-failed\nimplementer-c002e6035a243bdbc1400067\n2026-08-31T18:02:11Z`
  → `67d29d2adfffb3f29f5ce647444f7e24c0f75f5920da3d8aebb0a55b0253187f`;
  the empty-birth form → `1e329942b575f27aabf33a724c6fc7e0f5f24ceca58fb847c60e937c4d27f6a8`.
  Dedup per job (per incarnation of a reused job id) is this digest.
- **Facts** (`facts` map, exact keys from channel §11a.10 that this minimal
  scan can derive mechanically): `goalId`, `jobId`, `birth` (the token's
  exact bytes), `reason` (the record's `error` field verbatim, with
  `protocolError.violation` appended after `: ` when present, else `""`),
  `role`, `chainRoot` (the parentJob-walk result, `""` on any walk refusal),
  `reviews` (the record's `reviews` field verbatim, may be `""`).
  `answerAction` and `answerReason` are ABSENT: their derivation table is
  §11a.8's and belongs to the channel goal. Seam note for that goal, recorded
  here so the two never fight: its build either backfills exactly those two
  absent keys at its own journal time (a facts completion, not a re-mint), or
  its composer treats their absence as no-answer-derived; this design leaves
  that choice to the channel goal and its episodes valid either way.
- **Message**, plain words, set once (facts are terminal, so it never goes
  stale), shaped like:
  `Delegate job <jobId> (role <role>, round <round>) under goal <goalId>
  ended <status> at <endedAt>: <reason, or "no failure reason was recorded">.
  Nobody has closed its chain. The job record is
  artifacts/agents/jobs/<jobId>.json. Acknowledge with: metasystem health
  acknowledge-alert --episode alert-<digest> --repo <repository root>.`
- **Lifecycle**: never auto-cleared (the channel's law — never clearing is
  what keeps exists-by-digest dedup sound). `AcknowledgeAlert` is the
  terminal human step. The NAG (section 4) stops when any of: the episode is
  acknowledged; the record's `chainClosed` is true; the goal is no longer
  claimed; the record is gone. None of those clears the episode; the file
  remains evidence.

### 3b. stop-awaiting-resume

- **Raise predicate**, per tick over the same goal projection: a goal file
  whose state is claimed and whose `StopFence` is non-nil. The fence's
  existence IS "a breach-stop fired" — it stands from the moment the stop
  installs it, through batch completion, until resume. One rule, no
  batch-state branching.
- **Digest**: SHA-256 lowercase hex of `stop-awaiting-resume` + LF + goal id
  + LF + `StopFence.Revision` in base-10 with no leading zeros. Revisions
  never repeat within a goal (resume mints a fresh one), so this is the
  dedup per stop. Pinned vector (recomputed, it matches):
  `stop-awaiting-resume\nalert-escalation-channel\n8` →
  `8a6c1ffb2f72d5ae750e890e30c9a12a72bdceec6914029f73b3956e9f8e790d`.
- **The marker, before the episode** (channel §11a.9's ordering law, adopted
  verbatim): before writing the episode, the sweep durably writes
  `artifacts/agents/steward/alerts/stop-open/<goal-id>-r<revision>` whose
  entire content is the 64-hex digest, under the alert lock, using the
  store's atomic durable write. Marker first, then episode; re-deriving is
  idempotent (same name, same content). The marker filename is reversible:
  stripping the trailing `-r[0-9]+` yields the goal id and revision.
- **Facts**: `goalId`, `revision` (base-10), `stopId` — exactly the channel's
  keys, all in hand at raise time.
- **Message**, plain words, set once, shaped like:
  `Goal <goalId> revision <revision> was breach-stopped at <ClosedAt>
  (<Reason>, e.g. ELAPSED_LIMIT means the elapsed budget fence was hit). No
  new jobs can launch under it until a human resumes it. Resume from an
  agent-free terminal with: metasystem goal resume --id <goalId> --by <your
  name> --elapsed-limit <duration> --attempt-limit <count>
  --reserved-job-minutes-limit <minutes> --active-job-limit <count> — all
  four budget flags are required, as a fresh complete budget. Acknowledge
  with: metasystem health acknowledge-alert --episode alert-<digest> --repo
  <repository root>.`
- **Lifecycle**: cleared ONLY by the positive fence-gone proof. Each tick the
  sweep lists `alerts/stop-open/` (bounded by open stops, because draining
  removes markers) and evaluates, per marker, against the goal projection
  already in hand: fence still bound (the goal exists, is claimed, and
  carries a `StopFence` with that marker's revision) → the marker stays.
  Fence GONE → drain: read the marker's digest, open the episode, set exactly
  `Cleared`/`ClearedAt` if present and uncleared, then remove the marker; an
  absent or already-cleared episode still drains the marker. Anything
  unreadable HOLDS (a clear is never inferred from a failed read).
  Acknowledgment stops the nag but never clears.

## 4. Tick integration, read set, and today's delivery

**One new sweep**, `SweepWorkEscalations(repoRoot, now)`, in a new
`internal/steward/escalation.go`, called from `RunTick` immediately after
`ReapContinuations` (tick.go 170–173) — after `runBreachStopCustodian`, so a
fence installed this very tick escalates this very tick, and after the reap,
so records the reaper just finished are seen in their final state. On error
the tick takes the existing `degradedTick` path with a named reason
(`"work escalation sweep failed: …"`) — an unreadable store is never a silent
verdict. The sweep's writes (markers, episodes) run under the existing alert
flock; the arbitration lock already serializes ticks.

**Read set, bounded**: one `goal.Project` over the live tree (the same class
of read `FindBreachStops` performs every tick; the sweep does its own call
rather than refactoring the custodian — simplest, and the projection is the
bound); one `ReadDir` of `artifacts/agents/jobs` with one small-JSON decode
per record (~100 today, growth bounded by evidence GC); one `ReadDir` of
`alerts/stop-open/` (bounded by open stops); one stat per candidate episode
(exists-by-digest). No network, no processes, no record writes.

**What one sweep does**, in order: (1) enumerate claimed goals from the
projection; (2) stop-class pass — raise (marker, then episode) for every
standing fence without an episode, then the clear phase (drain fence-gone
markers); (3) failed-job pass — raise for every record matching 3a's
predicate; (4) the NAG — for every episode of either class whose condition
still stands (3a: record present, chain open, goal claimed; 3b: marker
present, fence bound) and which is not acknowledged, queue one durable
`PendingNotification{Nonce: episodeID, Message: episode.Message}` — exactly
the stalled-idle shape: the queue holds one pending message per standing
condition, delivery removes it, the next tick re-queues while it stands, so
at the default 600-second tick the nag is at most one delivery per condition
per 10 minutes and acknowledgment is the off switch; (5) return a report
(`[]WorkEscalationReport{Class, EpisodeID, GoalID, JobID, StopID, Raised,
Cleared}`) on `TickResult` for narration and digest.

**Digest and narration** (what exists today): `NarrateDigest` gains, from the
report, one lowlight entry per RAISED episode (`SourceType: "episode"`,
`SourceID: <episodeID>`, text naming the goal, the job or stop, and the
failure in plain words) and one highlight per CLEARED stop episode
(`SourceID: <episodeID>-cleared`, distinct so the digest's
exact-retry dedup cannot swallow it). Emission is on transition only — raise
and clear — never per standing tick, so the digest stays readable. The
best-effort narration line appends a note while anything stands:
`"<n> failed delegate job(s) and <m> breach-stop(s) are waiting for
attention"`.

## 5. The one edit to shared code: scoping the health join

`UpdateAlertEpisodes`' healthy-clear loop (alert_episode.go 246–268) and its
resolve-others loop (270–279) are restricted to health episodes by the
filename grammar the channel design fixed: the health path loads only
directory entries NOT matching `alert-<64 lowercase hex>.json`. By
construction (section 1, naming trace) that set is exactly the health
episodes — a producer-named file is never even opened by the health path.
`AlertEpisodes()` (the full reader), `AcknowledgeAlert`, and
`migrateHeldHealthNotifications` are unchanged (legacy migration nonces are
health digests and produce health-named ids). This is the channel design's
§11a.10 restriction implemented early — cooperation, not divergence — and it
is REQUIRED now: without it, the first healthy tick wipes both new classes.
The stalled-idle lifecycle is untouched: its verdict still rides the notify
queue, and the health-joined episodes behave byte-identically (the fixture in
section 6 proves it).

## 6. Fixtures (all deterministic: temp repos, injected clock, no sleeps, the
steward package's existing test idiom)

1. `TestFailedJobUnderClaimedGoalRaisesEscalation` — claimed goal file plus a
   `failed` job record naming it; one sweep raises
   `alerts/alert-<digest>.json` with the right class, facts, and message; the
   digest matches the pinned vector recomputed in-test from the tuple bytes;
   one notification with nonce `alert-<digest>` is pending; the raise appears
   in the digest entries.
2. `TestFailedJobEscalationIsWriteOnceAndNagsPerTick` — a second sweep leaves
   the episode file byte-identical, creates no second episode, and re-queues
   the notification after a simulated delivery (`MarkDelivered`).
3. `TestClosedChainReleasedGoalOrMissingRecordStopsTheNag` — flipping
   `chainClosed`, releasing the goal, or removing the record each stop the
   re-queue; the episode remains uncleared in all three.
4. `TestTimeoutRaisesAndCancelledCompletedUnclaimedDoNot` — the predicate's
   four exclusions, one sweep each.
5. `TestBreachStopWritesMarkerThenEpisodeWithResumeCommand` — claimed goal
   with a `StopFence`; after one sweep the marker exists with the digest as
   its entire content, the episode exists, and the message contains
   `goal resume --id` and all four budget flag names; digest matches the
   pinned stop vector shape recomputed in-test.
6. `TestResumeDrainsMarkerAndClearsStopEpisode` — the fence removed (as
   resume does); the next sweep sets exactly `Cleared`/`ClearedAt`, removes
   the marker, and a further sweep raises nothing; an absent-episode marker
   also drains.
7. `TestAcknowledgedEscalationGoesQuietButStands` — after
   `AcknowledgeAlert`, no further notification is queued while the condition
   stands; the episode file remains, uncleared.
8. `TestHealthyVerdictLeavesEscalationEpisodesUntouched` — the scoping proof:
   a healthy `UpdateAlertEpisodes` pass clears health episodes and leaves
   both producer-class files byte-identical; an unhealthy pass resolves
   neither of them.
9. `TestEscalationSweepFailureDegradesTheTick` — an unreadable jobs
   directory yields the degraded verdict with the named reason, via `RunTick`.
10. `TestTickRaisesFailedJobEscalationEndToEnd` — one full `RunTick` in a
    temp repo proving the wiring: episode on disk, notification pending,
    digest entry present.

## 7. Blast radius — every consumer of episode listings, checked

- **`watch` alerts view**: raw-reads every regular `alerts/*.json`
  (records/watch-verb/watch-verb-design.md, table row "alerts"); the new
  episodes appear automatically; `stop-open/` is a subdirectory and is
  skipped by both the watch glob and `loadAlertEpisodesUnlocked`'s
  `IsDir` check.
- **`AlertEpisodes()` and the loader invariants**: new episodes satisfy every
  loader requirement (section 3 preamble), so the full reader keeps working.
- **`metasystem health`** (`runStewardHealth`): calls the now-scoped
  `UpdateAlertEpisodes`; output bytes unchanged; escalation episodes are
  invisible to it by the filename grammar.
- **`health acknowledge-alert`**: unchanged and class-agnostic; the 70-char
  ids pass `validEpisodeID`.
- **The narrator digest and its consumers** (`steward digest-pending`, the
  seat): additive entries only, on transitions only.
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
- **The alert-channel goal**: finds these episodes already existing by
  digest, skips re-minting (its own skip law), takes over transport
  (`TransportResult` finally advances past PENDING), and inherits the one
  recorded seam note from 3a (`answerAction`/`answerReason` absent on
  episodes minted here). Its stop-open marker mechanism is already in place.

## 8. Scope fence

No new delivery machinery; no external sends; no transport changes; no reaper
or breach-stop behavior changes (the sweep only reads their records); no GC
changes; no counselor changes; no changes to stalled-idle verdicts, health
verdicts, or the acknowledge verb. Implementation touches: new
`internal/steward/escalation.go` (+tests), two additive `AlertEpisode`
fields, the filename-scoped health-join loops in `alert_episode.go`, the one
sweep call plus report field in `tick.go`, and the transition entries in
`narrate.go`.

## 9. Self-grade

**Grade: B+.** Strengths: every mechanism is an existing, traced pattern
(episode store, notify queue nonce dedup, digest entries, degraded-tick
honesty); the seam with the landed channel design is identity-level, so the
two goals converge on the same files instead of colliding; every predicate,
digest byte, and lifecycle rule is mechanical, and the three digest vectors
were recomputed rather than trusted. Weaknesses, honestly: (1) adopting the
channel's identity imports the stop-open marker and the birth-token fallback
— real but bounded weight against a 1.5h appetite; the alternative (private
`escalation-` ids) was rejected because it guarantees double episodes at
channel arrival; (2) the `answerAction`/`answerReason` absence is a recorded
seam note, not a closed contract — the channel goal must disposition it; (3)
the nag rides a notification channel that is unconfigured on Linux hosts
today, so until the channel goal lands, visibility there is the durable
pending queue, `steward status`, the digest, and `watch` — stated, not
hidden, and identical to stalled-idle's current reality.

**Reject this design if any of these fail**: a recomputation of the three
pinned digest vectors disagrees; the `UpdateAlertEpisodes` scoping change
alters any byte of an existing health-episode fixture's expected output; the
channel design's §11a.10/§11a.9 text is shown to specify a different digest
tuple, id form, or marker rule than quoted here; or the orchestrator rules
identity adoption out of the 1.5h appetite — in which case the fallback is
the private-namespace variant (ids `escalation-failed-job-<digest16>-<n>` /
`escalation-breach-stop-<digest16>-<n>`, same predicates, same lifecycle,
same scoping edit keyed on the `escalation-` prefix), with the double-episode
cost at channel arrival explicitly accepted by the goal owner.
