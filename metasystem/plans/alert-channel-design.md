# Alert Channel Design — alert-escalation-channel (revision 9)

Status: revision 9 folds all six material findings of the Sol
round-2 critique of revision 8
(`records/misc/alert-channel-critique-r8.md`), none refuted — every
one checked true against the shipped code. The critical fold:
revision 8's crash-window proof leaned on a FALSE traced fact
("terminal records are never deleted") — evidence GC prunes mirrored
terminal job records after a grace window (default 5,400 seconds),
so a runner outage longer than retention silently re-opened the
pre-journal loss window the scan exists to close. Revision 9 closes
it with a RETENTION HANDSHAKE (new 11a.12): GC may not collect a
terminal failed/timeout record carrying a goal until the alert
episode its digest names exists durably; the converse collection
rule bounds the episode store
(AC8-JOB-SOURCE-RETENTION-001). The other folds: the stop scan's
alerting condition is now resume's own precondition
`VerifyStopBatchComplete`, full binding comparison included
(AC8-STOP-BATCH-BINDING-001, 11a.9); the stop episode's lifecycle is
one positive predicate each way — create on verify-pass, clear only
on proof the fence is gone, HOLD on anything unreadable — which also
answers the COMPLETE-turned-unreadable contradiction
(AC8-STOP-INDETERMINATE-LIFECYCLE-001, 11a.9/11a.10); a pre-send
source recheck built on that same predicate states the resume-race
ordering rule with its disclosed residual window
(AC8-STOP-RESUME-RACE-001, 11a.9/§5); both scans get enumerated,
GC-bounded read sets — digest-from-filename skip, write-once
episodes, the episode store itself as the durable checkpoint, no
history walk under tick locks (AC8-SCAN-BOUNDEDNESS-001,
11a.8/11a.9/11a.12); and both producers' Answer lines are fixed
byte-exactly under §6's new placeholder law, resume's real flag set
included (AC8-ANSWER-BYTES-AND-ACTION-001, §6/11a.8/11a.9).
SELF-CONSISTENCY PASS: performed this revision over every changed
rule and its touched sections — the pairs read together and made to
agree are 11a.12↔11a.8 (one job-id-and-digest derivation, one pin
proof); 11a.12↔11a.10 (collection versus never-auto-cleared, dedup
proof rewritten over record absence); 11a.12↔§1 (both cite the
corrected GC trace); 11a.8↔§1 (the record-deletion fact corrected
in both places); 11a.9↔§1 (resume's flag set and goal-revision lock
traced where used); 11a.9↔§5 (the pre-send recheck slotted into the
transport phase); 11a.9↔11a.10 (one clear predicate, stated once
and referenced); 11a.9↔11a.5 (a suppressed episode is cleared, so
it leaves due and undelivered together); 11a.8↔11a.10 (write-once
episodes replace refresh-on-every-pass in both); §6↔11a.8/11a.9
(the placeholder law and both Answer byte layouts); and §11↔11a.12
(the handshake lands in slice 1, ordered before the producers).

Design for the promoted goal `plans/goals/alert-escalation-channel.md`:
escalations and blocked-on-human states reach Wido IMMEDIATELY over an
external channel instead of terminating in a git-landed log he must
poll. Driving specimen: `records/misc/idle-loss-2026-08-31.md`.

Wido's design requirement, verbatim, binding:

> "it needs to have an abstraction/adapter. I want to be able to have
> email, slack, telegram, whatsapp etc underneath by simple
> configuration."

And his additions of 2026-08-31 (verbatim): "We can use the same
mechanism for the session bridge too, so there is a bit of reuse
there" and "Another one would be slack, which has threaded messages.
that also needs to fit the design of the alert channel and session
bridge." Telegram is confirmed as the first adapter.

Design only. No code ships with this document.

## 1. What exists today (traced facts)

- **The alert episode store** (`internal/steward/alert_episode.go`):
  one JSON file per episode under `artifacts/agents/steward/alerts/`,
  flock-serialized, attempts journal
  (PENDING/TRANSPORT_SUBMITTED/TRANSPORT_FAILED), digest-keyed dedup,
  crash-safe pending reuse, `AcknowledgeAlert`. The transport send
  currently runs INSIDE the alert lock (lines 324–356). Episode
  files are written atomically with durability verified
  (`saveAlertEpisode`, lines 152–165), and NO shipped path deletes a
  file under `alerts/` — deletion enters this design only through
  11a.12's collection rule.
- **The outer lock is wider than revision 2 admitted**: `RunTick`
  (`internal/steward/tick.go` 102–112) takes the repository
  ARBITRATION lock for the whole tick and still holds it through
  `UpdateAlertEpisodes` (lines 237–265); lease announcement
  (`internal/lease/verbs.go` 106–110) and revival
  (`internal/steward/revive.go` 32–36, 68–74) contend on that same
  lock. Releasing only the alert lock therefore frees nothing that
  matters, and releasing both invites concurrent senders.
- **The transport seam** (`internal/steward/notify.go`): synchronous
  `Deliver`, git-config command or macOS `osascript`, 15-second
  timeout, raw output embedded in errors.
- **The pending-notification queue** (`internal/steward/intervene.go`,
  fed from `runner.go`, `reap.go`, tick, noticings): durable second
  delivery state, delete-on-success, no receipts. Retired by §7.
- **The launch gate**: `EnsureRunner` and `arm` refuse when
  `NotifyCommand` reports no channel — availability check, no send.
- **The narrator digest register** (`internal/narratordigest/`): byte
  offset plus prefix-hash cursor; the Stop hook
  (`scripts/agents/supervision-hook.sh`) is its existing consumer.
- **The health verdict line is already plumbed to both floors**: the
  tick narrates `health.Line()` durably and the Stop hook composes
  health plus digest — so a fact added to the health verdict line
  reaches the terminal verb AND the Stop-hook payload with no new
  plumbing (used by §10's floor and §11's slice 1).
- **Real human answering verbs** (verified in
  `cmd/metasystem/main.go`): `metasystem goal resume` and `metasystem
  mission-runner answer`. No approve/reject verb exists. `goal
  resume`'s exact interface
  (`cmd/metasystem/goalsync_mutations.go` 347–417): `--id` and
  `--by` are required, and the COMPLETE budget tuple is mandatory —
  `--elapsed-limit`, `--attempt-limit`,
  `--reserved-job-minutes-limit`, `--active-job-limit`
  (`budgetTuple(true)`); `--root` defaults to `.`. Resume serializes
  on the GOAL-REVISION lock (`goalrevision.Acquire`, lines 404–408),
  NOT the steward arbitration lock the tick holds (`tick.go`
  107–114) — the two paths share no lock, which is 11a.9's race
  fact.
- **Job-failure and breach-stop facts, traced for the 2026-09-01
  producers**: a delegate job record
  (`artifacts/agents/jobs/<job-id>.json`) carries a `goalId` field
  (read back by dispatch, `scripts/agents/dispatch.sh` 1759) and
  reaches terminal failure through TWO writers, corrected from
  revision 7's only-through-the-CAS over-claim: the record CAS's
  transition table — `failed` or `timeout`, distinct from deliberate
  `cancelled` (`internal/dispatch/record.go` 39–46, `RecordCAS` 475)
  — AND `RecordProtocolError` (`record.go` 417–464), which directly
  stamps a pending or running record `failed` with `error`
  `protocol_error` without calling `RecordCAS`; both respect the same
  edge set, and the optional `error` field carries the reason. The
  reaper performs the process-loss and timeout transitions within
  seconds of death (`internal/supervise/reaper.go`, through the same
  CAS). The reaper mirrors terminal records to the durable evidence
  root and never removes them itself — but revision 8's "no shipped
  path deletes them" was FALSE: evidence GC
  (`internal/evidence/gc.go`, `pruneMirroredRecords` 375–449)
  removes a terminal record once its mirror manifest carries a
  semantic hash equal to the record's current one and the mirror is
  older than the grace window (`scripts/agents/evidence-gc.sh` line
  48: default 5,400 seconds; that script's header comment "Job
  records (jobs/*.json) always stay" is stale against the Go code it
  execs). `keepsSpendingFact` (gc.go 477–512) retains only records
  still contributing spend to the CURRENT claimed revision — a
  record bound to a superseded revision, or to a goal no longer
  claimed, is prunable. A terminal record is therefore durable,
  rescannable source state ONLY under 11a.12's retention handshake,
  which pins failed/timeout records carrying a goal until their
  episode exists. The
  breach-stop custodian runs INSIDE `RunTick`
  (`internal/steward/tick.go` 153, 69–90) and returns one
  `BreachStopReport{GoalID, Revision, StopID, State}` per stopped
  revision; `FindBreachStops` OMITS a fence whose stop batch reads
  COMPLETE (`internal/dispatch/stop.go` 295–296), so a completed
  stop's report appears once and never again — but the fence itself
  persists in the goal file until `metasystem goal resume`, which
  "verifies the exact stop batch during the transaction, installs
  the complete fresh tuple, creates a new revision, and clears the
  old fence" (`internal/goal/stop.go` 353–355). A claimed goal
  carrying a `StopFence` whose batch is COMPLETE is therefore the
  exact durable awaiting-resume state, present every tick until the
  human acts.
- **Both tick drivers already end with a delivery step, on success
  AND failure** (traced): the resident runner loop calls `RunTick`,
  prints a failed tick to stderr WITHOUT returning, and reaches its
  `DeliverPending` call on every iteration
  (`internal/steward/runner.go` 99–101, 131); the external
  `metasystem steward tick` command has TWO branches — a failed
  `RunTick` calls `DeliverPending` and returns 1
  (`cmd/metasystem/steward_verbs.go` 234–240), a successful one
  calls it before printing the report (line 270) — each branch a
  natural additive slot for §5's transport phase (law in 11a.11).
- **Provider facts** (from the round-2 critique's cited official
  documentation, adopted as design constraints): Slack incoming
  webhooks return no message timestamp — threading needs the Web API
  (`chat.postMessage` returns `ts`); Telegram replies need
  `reply_to_message_id` and its polling owns an update offset,
  mutually exclusive with webhooks; email replies need
  Message-ID/In-Reply-To/References (RFC 5322); WhatsApp replies need
  `context.message_id` and inbound delivery is an HTTPS callback.
- **The configuration idiom** (`internal/config/resolve.go`): flag >
  derived environment variable > uncommitted `metasystem.conf.local` >
  committed `metasystem.conf` > default.

## 2. The channel contract — outbound, shared by three callers

(Folds AC2-CONTRACT-001 and AC2-RECEIPT-001; rescopes revision 2's
over-claim.)

**Definition, to remove the round-2 ambiguity:** the CHANNEL LAYER is
one concrete engine package, `internal/channel`, the sole
implementation of the caller-facing contract below. It sits between
callers (the episode store's sender, the digest batcher, the bridge's
outbound path) and adapters, and it owns destination resolution,
secret scrubbing, chunking, and the conversation reference store
(§3). Adapters implement a NARROWER internal interface (§2a) and stay
stateless.

```
type DestinationName string   // named, configuration-resolved place

type Message struct {
    Class          MessageClass // "alert" | "digest" | "bridge"
    Sender         string       // asserted origin label (see §2b: NOT authenticated identity)
    ConversationID string       // caller's stable correlation key
    Deadline       time.Time    // zero, or when an unanswered ask escalates
    Happened, Asked, Answer string // human-alert content; empty for bridge/digest
    Body           string       // pre-composed text (digests, bridge payloads)
    Spans          []ContentSpan // digest only: ordered source-entry spans
                                 // (register byte ranges) composing Body
    EpisodeID      string
}

type MessageRef struct {
    ID       string // provider message handle (Telegram message_id, Slack ts,
                    // email Message-ID, WhatsApp message id)
    ThreadID string // provider thread identity when the adapter threads
}

// One transport submission's outcome. A Message may take several
// submissions (§9); every one is visible to the caller.
type ChunkOutcome struct {
    Ref   MessageRef
    Span  ContentSpan // the slice of Body/Spans this submission carried
    Err   error       // nil, or typed+sanitized (ErrUnconfigured | ErrSendFailed)
}

type SendResult struct { Chunks []ChunkOutcome } // ordered

type Channel interface {
    Send(dest DestinationName, msg Message) (SendResult, error)
    Ready(dest DestinationName) (bool, string)        // no side effects; §6 gate
    Capabilities(dest DestinationName) AdapterCapabilities // fields defined in 11a.2
}
```

`Send`'s top-level error covers only CALLER MISUSE — exactly 11a.3's
list: unknown destination name, invalid class, empty required compose
field, over-limit message. An UNCONFIGURED destination is NOT a
top-level error: `Send` returns a nil error and exactly one
`ChunkOutcome` whose `Err` is `ErrUnconfigured` (one contract with
11a.3, which owns the journaling side). Once submissions begin, every
outcome — success or failure, per submission — is a `ChunkOutcome`. This is the
per-chunk receipt AC2-RECEIPT-001 found unrepresentable: the episode
owner journals one attempt per outcome (§7), and the digest owner
advances its cursor from the outcomes' spans (§9).

### 2a. The adapter interface

```
AdapterSend(ctx context.Context, resolved DestinationConfig,
            text string, conv ConversationState) (MessageRef, error)
```

One submission, one reference, one typed sanitized error. The `ctx`
is created per submission by the channel layer and carries the
15-second transport bound (ownership and duration in 11a.7); an
adapter sets no timeout of its own and returns promptly with a typed
sanitized error when the context cancels. No
chunking, no retries, no state: `ConversationState` (the
conversation's first and latest known `MessageRef`, read from §3's
store by the channel layer) is passed IN, which is how Telegram sets
`reply_to_message_id`, email sets In-Reply-To/References, WhatsApp
sets `context.message_id`, and Slack sets `thread_ts` — without any
adapter owning storage. The two-value store SUFFICES only because of
the thread-state sufficiency invariant of §3a: whatever an adapter
needs beyond the latest provider message id to continue its thread,
it must carry inside the `ThreadID` it returns. Registry: `email`, `slack`, `telegram`,
`whatsapp`, `command`, `desktop`, `none`. The Slack adapter uses the
Web API with a bot token (`chat.postMessage`), NOT an incoming
webhook, because the webhook returns no `ts` and therefore cannot
thread — a provider fact, folded (§1): its settings are
`slack.channel-id` plus secret `slack.bot-token`.

### 2b. The reuse boundary with the session bridge — stated honestly

Revision 2 claimed this design fixes "the shared contract those loops
ride" while the contract had no receive half; the disposition table
then claimed the fold complete. Both statements were wrong. The
honest scope, this revision's law:

**This design's contract is the OUTBOUND half only.** Three callers
share it: alerts, digests, and the bridge's outbound sends. The
INBOUND half — receiving a seat's or a human's reply over a provider
transport — is NOT in this contract and is owned by the
seat-mutual-awareness design. It is real contract work, not a
boolean: the enumerated obligations reserved for that design are
(1) per-provider ingress transport — Telegram `getUpdates` with an
owned update offset, mutually exclusive with webhooks; Slack Events;
WhatsApp HTTPS callbacks — which requires a listener or poller this
design's stateless adapters deliberately cannot be; (2) a durable
inbound handoff store with checkpoint/acknowledgment tokens;
(3) ordering and duplicate semantics; (4) typed receive errors; and
(5) AUTHENTICATED sender provenance — `Message.Sender` here is an
asserted label sufficient for a human reading an alert, and this
design explicitly does NOT claim it satisfies seat-mutual-awareness's
assertable-seat-identity requirement.

What the bridge REUSES from here, matching Wido's "a bit of reuse":
the destination registry and configuration idiom, the credential and
redaction laws, the adapters' outbound sends, the conversation
vocabulary (`ConversationID`, `MessageRef`) and the §3 reference
store, so a bridge exchange threads on threaded transports. The
acknowledgment path for ALERTS needs no ingress at all: it is the
existing `health acknowledge-alert` CLI verb, which is why the alert
channel ships without the receive half.

## 3. Threading and the conversation reference store

(Folds AC2-THREAD-001.)

`ConversationID` remains all a CALLER sets. But revision 2 hid the
provider-reference problem inside one Slack map; in fact ALL four
named transports need a prior provider message reference to reply or
thread (§1 provider facts). The named common owner:

**The conversation reference store**, owned by the channel layer (not
adapters, not callers), at
`artifacts/agents/channel/<destination>/conversations.json`: for each
`ConversationID`, the first and latest `MessageRef` returned by
successful submissions, written by the channel layer after each
`ChunkOutcome`, read to build the `ConversationState` passed into
`AdapterSend`. Additionally every episode attempt retains its returned
`MessageRef` (§7), so the truth store carries the provider handles of
its own alerts independently of the reference store.

Degradation, stated without the round-2 euphemism: the store is
derived state whose LOSS is survivable but not free. A lost store
means the next message in a conversation starts a fresh thread or
sends unlinked — content is never lost, but the JOIN is: a late reply
arriving on an old provider thread can no longer be correlated to its
exchange by this design's data. For the ALERT channel that is
acceptable by construction (acknowledgment is the CLI verb, not a
thread reply). For the BRIDGE, exchange-join durability is one of the
§2b reserved obligations, and the bridge design decides whether the
reference store's durability must be raised or its ingress store owns
the join. Flat adapters (`command`, `desktop`) degrade to a bracketed
conversation tag in the text; adapters without a usable prior
reference send unthreaded. No caller changes in any of these cases.

### 3a. Thread-state sufficiency and email ancestry

(Folds AC3-THREAD-ANCESTRY-001.)

`MessageRef.ThreadID` is ADAPTER-OWNED OPAQUE THREADING STATE under
one contract invariant, stated so the two-value store is provably
enough for every adapter: **the pair (latest `MessageRef.ID`, latest
`MessageRef.ThreadID`) must alone let the same adapter compose the
conversation's next threaded or reply message.** An adapter that
would need more history is non-conforming; it must fold that history
into the `ThreadID` it returns.

Per shipped adapter:

- **Slack**: `ThreadID` is the thread's root `ts`, constant for the
  conversation; replies set `thread_ts` from it.
- **Telegram**: replies set `reply_to_message_id` from the latest
  `ID`; `ThreadID` is empty.
- **WhatsApp**: replies set `context.message_id` from the latest
  `ID`; `ThreadID` is empty.
- **Email**: `ThreadID` carries the References ancestry — the value
  the next reply's References header must contain. On each send the
  adapter composes In-Reply-To from the latest `ID` (the parent's
  Message-ID) and References from the latest `ThreadID` followed by
  the latest `ID`, per RFC 5322 §3.6.4, and returns the new
  message's `MessageRef` with `ThreadID` set to exactly that new
  References value — the chain is retained inductively in the single
  latest reference. The chain is BOUNDED by a design constant, not a
  standard: **`emailReferencesMaxBytes = 8192`** (8 KiB of the
  unfolded References value, roughly one hundred typical message
  identifiers). RFC 5322 imposes NO total limit on an unfolded
  References field (its §2.2.3 998-character limit is a
  physical-LINE limit, satisfied by folding) and specifies no
  trimming; the cap exists solely for interoperability with
  implementation-specific server header-section limits, and 8 KiB is
  a conservative constant this design fixes so every implementation
  emits identical ancestry. Behavior at the boundary, exactly: after
  appending the parent's Message-ID, while the value exceeds the
  cap, remove the entry at POSITION 2 (the oldest non-root entry)
  repeatedly — the root identifier (position 1) and the most recent
  contiguous suffix, ending in the parent's Message-ID, are always
  kept. In the pathological case where the root plus the parent
  identifier alone exceed the cap, keep only the parent identifier.
  The email adapter's provider-test slice must include a long-chain
  fixture: a chain long enough to cross the cap, asserting the root
  is preserved, the kept suffix is contiguous and most recent, and
  the parent's Message-ID is last.

## 4. Configuration key shape

Unchanged from revision 2 except the Slack settings (§2a):

```
channel.destination.wido-urgent.adapter=telegram
channel.destination.wido-urgent.telegram.chat-id=<id>
channel.destination.wido-urgent.telegram.bot-token=<SECRET — never committed, §10>
channel.destination.wido-quiet.adapter=telegram
channel.destination.seat-m2.adapter=slack
channel.destination.seat-m2.slack.channel-id=<id>
channel.destination.seat-m2.slack.bot-token=<SECRET>

channel.alert.destination=wido-urgent
channel.alert.fallback-destination=local-desktop
channel.digest.destination=wido-quiet
channel.digest.batch-minutes=240
channel.digest.batch-max-bytes=3500
```

Classes bind destinations; the bridge addresses `seat-<id>`
destinations by name; distinct destinations keep alerts out of
narrative. The legacy `metasystem.steward.notify-command` git-config
key remains honored as an implicit `command` destination
(`local-command`) when no alert destination is configured.

## 5. The single-flight sender

(Folds AC2-LOCK-001 — one implementation, chosen, with its laws.)

Revision 2's split was impossible both ways: keeping the arbitration
lock around delivery blocks lease and revival machinery on network;
dropping it lets two writers both apply the PENDING-reuse rule and
double-send under one attempt. The chosen implementation:

**Journaling and transport are separate phases with separate,
never-nested guards; transport has exactly one flight per repository,
enforced by the kernel.**

1. **Journal phase** (inside the tick, arbitration and alert locks as
   today): `UpdateAlertEpisodes` loses its transport call entirely. It
   opens/refreshes episodes and, where a send is due, ensures a
   PENDING attempt exists. It performs NO network work, so nothing
   new ever waits on the network under either lock. (`tick.go` line
   264's call becomes journal-only; the tick's contract is otherwise
   unchanged.) BOTH 2026-09-01 producers journal in this same tick
   phase as IDEMPOTENT DERIVATION SCANS over their durable source
   state — the terminal job records (11a.8) and the verified stop
   batches (11a.9); no transition writer
   dual-writes into the episode store (read sets and bounds in
   11a.8/11a.9; source retention in 11a.12). Every journal write takes the
   alert lock, and only TRANSPORT (phase 2) is single-flight.
2. **Transport phase** (`DeliverDueAlerts`, called AFTER `RunTick`
   returns — and with it the arbitration release, which `RunTick`
   holds only internally — by BOTH tick drivers: the resident steward
   runner's loop and the external `metasystem steward tick` command,
   each alongside its existing legacy `DeliverPending` call at the
   §1-traced call sites. The external command's printed report is
   byte-unchanged; this phase's outcomes live in the episode journal
   and surface through the 11a.5 floor): acquire a NEW dedicated sender flock,
   `artifacts/agents/steward/alerts-sender.flock`, with
   LOCK_EX|LOCK_NB. If it is held, return immediately — the live
   holder is the single flight, and the next tick retries. DUE,
   exactly: an attempt is due iff it is the episode's latest attempt,
   its `Result` is PENDING, and the episode is neither `Cleared` nor
   `Acknowledged` — the same exclusions as 11a.5's counting, so a
   scan-cleared or already-acknowledged episode is never sent late.
   Holding it: briefly take the alert lock to read due PENDING attempts and
   stamp each with this sender's identity; release the alert lock;
   perform sends (per-pass budget `channel.max-sends-per-tick`,
   default 3, each 15-second-bounded; for a `stop-awaiting-resume`
   attempt, 11a.9's pre-send source recheck runs between stamping
   and transport and may suppress the send by clearing the episode
   instead); re-take the alert lock briefly
   per completion and journal each `ChunkOutcome` through the
   COMPLETION MERGE of §5a — never by saving a pre-send snapshot. The
   alert lock is never held across network work; the arbitration lock
   is never held at all in this phase.

### 5a. The completion merge (folds AC3-SENDER-MERGE-001)

While a send is in flight, the alert lock is free, so acknowledgment
(`AcknowledgeAlert`) and healthy-tick resolution/clearing are LIVE
CONCURRENT WRITERS to the same episode file. A completion that saves
the episode as the sender loaded it before sending would overwrite
whatever they wrote — the lost update the shipped code cannot have
(its lock never releases) and the split would otherwise introduce.
The completion transition is therefore a RELOAD-AND-MERGE, stated as
law:

1. Under the re-taken alert lock, RELOAD the episode from disk. The
   pre-send in-memory snapshot is dead at this point and must not be
   saved, in whole or in part, except for the receipt values below.
2. Locate the stamped attempt in the RELOADED episode by sequence
   number and sender stamp (stamp fields and matching rule in
   §11a.1). If it is absent, no longer PENDING, or bears a foreign
   stamp, REFUSE the completion and journal the refusal to the
   refusal journal of §11a.1; never create a substitute attempt.
3. Merge ONLY these fields into the reloaded episode: the stamped
   attempt's `CompletedAt`, `Result`, sanitized `Problem`, and
   `MessageRef`; then derive `TransportResult` and `SubmittedVia`
   from the reloaded attempts per §7. Every other field —
   `Acknowledged`, `AcknowledgedAt`, `AcknowledgedBy`, `Resolved`,
   `ResolvedAt`, `Cleared`, `ClearedAt`, `Message`, `OpenedAt` — is
   saved exactly as reloaded. A mid-flight clearing is NOT undone by
   a completing receipt: the receipt lands as evidence on the cleared
   episode.
4. Save the merged episode under the same lock hold as the reload —
   reload, merge, and save are one critical section.

The testable invariant, verbatim for the implementation's fixture:
"for every episode field outside the stamped attempt's receipt
fields and the derived transport summary, the value saved by the
completion equals the value read at the same-lock-hold reload." The
named test: park a fixture send mid-flight, acknowledge (and, in a
second case, clear via a healthy verdict) the episode, release the
send; after completion the acknowledgment/clearing state must be
byte-identical to what the concurrent writer saved, and the stamped
attempt must carry its receipt.

Why each stated law now holds:

- **No machinery waits on a send.** Lease, revival, ticks, and
  acknowledgment contend only on arbitration and alert locks, and
  neither is ever held across transport. Worst added contention is
  the sender's brief journal reads/writes.
- **One try, one receipt; no duplicate concurrent sends.** Only a
  sender-flock holder performs transport or completes attempts, and
  LOCK_EX admits one holder per repository at a time. Two processes
  can no longer both act on one PENDING attempt, so the reuse rule
  has a single reader by construction. (In practice the resident
  steward runner is the only routine caller; the flock exists so a
  concurrent manual `health`-family invocation or a second runner
  during takeover cannot become a second flight.)
- **At-least-once across crashes, unchanged.** A flock dies with its
  process (kernel-released on exit), so a sender crash mid-send
  strands no lock and leaves the stamped PENDING attempt; the next
  sender reuses exactly that attempt (destination-matched, §7) — the
  same crash-gap law as today, duplicate-possible, never
  double-receipted.
- **Bounded, disclosed latency.** An alert journaled while a sender
  pass is in flight waits for the next pass (at most one tick
  interval plus the pass bound). The design says "immediate" means
  within one sender pass, not within the journaling write — that is
  the honest cost of not blocking machinery, and it is minutes-scale
  worst case against a specimen measured in hours.

## 6. Alert content and the launch gate

Alert content keeps revision 2's three fields (WHAT HAPPENED / WHAT
IS ASKED / the exact ANSWERING ACT — only verbs proven to exist,
e.g. `metasystem goal resume`, `metasystem mission-runner answer`;
composer refuses empty fields; `docs/seat-communication.md` binds
the register), now BYTE-EXACT (folds AC7-COMPOSER-BYTES-001). The
composed message is exactly four lines joined by single LF bytes
(`\n`), no trailing newline, labels ASCII verbatim:

```
WHAT HAPPENED: <Happened>
WHAT IS ASKED: <Asked>
ANSWER: <Answer>
Acknowledge: metasystem health acknowledge-alert --episode <episode-id> --repo <absolute repository path>
```

Line 4 IS the acknowledgment line — the shipped verb and flags
(`cmd/metasystem/steward_verbs.go` 77–80: `--episode`, `--repo`),
with the episode id and the repository's absolute path substituted.
THE PLACEHOLDER LAW (binding for every class's `Answer`, folds the
actionability half of AC8-ANSWER-BYTES-AND-ACTION-001): an `Answer`
is composed from literal ASCII bytes in which every angle-bracketed
token is either SUBSTITUTED — replaced whole, brackets included, by
a value the composer holds mechanically, each class enumerating
exactly which tokens those are — or VERBATIM: sent as its literal
bytes, brackets included, because only the human can supply the
value. No third treatment exists, and a class definition leaving a
token unclassified is a design defect, not an implementer choice
(11a.6, 11a.8, and 11a.9 each enumerate their tokens).
The NEVER-CUT set of §9 is thereby defined: every byte of the
composed message EXCEPT the `<Happened>` field value — the three
label prefixes, the three separator newlines, the Asked and Answer
values, and the whole acknowledgment line. A composer fixture derives
every byte before applying the cap without choosing anything. The launch gate
(folds AC3-SLICE-GATE-CUTOVER-001): the target state is `EnsureRunner`
and `arm` consulting `Ready` over the alert destination chain
(primary or fallback configured passes; no send, no output). But the
CUTOVER is lawful only when every producer still alive has a delivery
route the gate vouches for — and until the legacy
pending-notification queue is retired (§7), its producers (tick
notify verdicts, revival failures, reap notices) deliver ONLY through
`NotifyCommand`. A gate that accepted a Telegram-only configuration
while those producers still needed the legacy command would admit a
runner whose existing notifications silently cannot send. Therefore:
the gate is governed by `channel.gate` — default `legacy`, which
keeps the shipped `NotifyCommand` check byte-for-byte; `channel.gate=
channel` switches to `Ready`, and its OWN slice lands strictly after
queue retirement (§11), when the equivalence "gate passes ⇒ every
live producer can deliver" holds again. Readiness refusal exists ONLY
at the launch gate; everywhere else unconfigured is a typed,
non-blocking degradation.

## 7. One truth layer: episodes, receipts, dedup

Unchanged laws from revision 2: the pending-notification queue is
retired with every caller migrated (episodes or digest register); the
episode store is the only durable delivery state for the alert class;
resolution is class-scoped (the resolve-all-others law stays
health-only); the producer table (stop id / mission ask id / minted
ask id / enrolled-engine identity / claim-approval deferred to its
mechanism) stands as written in revision 2, EXTENDED by Wido's
2026-09-01 binding word (specimen
`records/misc/idle-loss-2026-09-01.md`) with two classes that are
SLICE-1 producers, not deferrals: `delegate-job-failed` — a delegate
job failed under a claimed goal; carries goal id, job id, and failure
reason; deduplicated per job; mechanics in 11a.8 — and
`stop-awaiting-resume` — a breach-stop closed a claimed revision's
fence and the goal waits for `metasystem goal resume`; mechanics in
11a.9.

Receipt law, completed for chunks and references (folds the receipt
halves of AC2-LOCK-001, AC2-THREAD-001, AC2-RECEIPT-001): one
`AlertAttempt` per `ChunkOutcome`, each carrying sequence, timestamps,
destination name (`Channel`), result, sanitized problem, AND the
returned `MessageRef`. Alerts are composed under one chunk by
construction (§9), so the normal case stays one attempt per try; a
fallback try is its own attempt; `SubmittedVia` records the successful
destination; PENDING reuse is destination-matched and, per §5,
exercised only ever by the single-flight sender.

## 8. Digests and the cursor law

Unchanged from revision 2: the external digest channel is a second
NAMED consumer of the register's existing byte-offset-plus-prefix-hash
cursor mechanism; per-consumer cursor records; the cursor is conceded
as named, bounded delivery state; a lost cursor re-sends at most the
retained register once, flagged in the batch header. Completed by §9:
the cursor advances using `ChunkOutcome.Span`.

## 9. Message size law and chunking — owned by the channel layer

(Folds AC2-RECEIPT-001's ownership half.)

- Alerts are bounded at composition (`channel.alert.max-bytes`,
  default 1500, under every named provider cap); an over-cap alert is
  truncated with a tail naming the episode id. One alert, one chunk.
  The truncation law, exact: the cap counts UTF-8 BYTES of the whole
  composed message, whose every byte §6 now fixes. Only the
  `Happened` field value may be shortened; the never-cut set —
  defined byte-exactly in §6 as everything else, labels, separators,
  `Asked`, `Answer`, and the acknowledgment line — carries the
  acting verb, which is the alert's point. The tail is the
  message's final line: a newline then `[truncated; episode
  <episode-id>]` — 22 bytes plus the episode id's bytes, ASCII apart
  from the id. Composition: if the full text fits the cap, send it
  tailless; otherwise give `Happened` the budget cap − bytes(all
  never-cut parts) − bytes(tail), and cut `Happened` at the largest
  length ≤ that budget ending on a UTF-8 code-point boundary (back up
  past continuation bytes; code points, not grapheme clusters, are
  the unit), possibly to zero bytes — §6's non-empty check runs on
  the pre-truncation field. Degenerate case, total by rule: when the
  budget is negative (never-cut parts plus tail alone exceed the
  cap), send the never-cut parts plus tail uncut — the compose cap is
  a courtesy bound, and the destination's DECLARED limit stays the
  hard refusal (caller misuse, 11a.3).
- A digest batch composes at most `channel.digest.batch-max-bytes`
  of entries and carries their `Spans`. When the destination's
  declared message-size limits (11a.2) are smaller, the CHANNEL LAYER (§2's
  concrete package — not the adapter, not the caller) splits Body on
  span boundaries into sequential submissions in the same
  conversation, calling `AdapterSend` per piece, and returns every
  outcome in `SendResult.Chunks` with its span.
- Cursor advance rule: the digest consumer's cursor advances to the
  end of the longest PREFIX of accepted chunks. A failed middle chunk
  stops advancement there; later accepted chunks will be re-sent next
  window — disclosed at-least-once duplication, matching the alert
  law, never a silent gap.

The configuration-only claim stays scoped as in revision 2: enabling
a shipped adapter is configuration-only at call sites; shipping one
is engine code with that provider's constraint tests (size cap,
redaction, reference mapping).

## 10. Credentials and failure honesty

Unchanged from revision 2: secrets resolve only from environment or
`metasystem.conf.local` (the channel layer skips the committed file
for secret-named settings and reports why); adapters must redact
their configured secrets from all error text and the channel layer
literal-scrubs every resolved secret from every problem string before
journaling, logging, or printing, with the known-bad fixture shipping
in slice 1; failed sends journal and retry next pass; the fallback
destination is its own journaled attempt.

The floor, now with the one-change plumbing §1 traced: the
undelivered-alert count joins the HEALTH VERDICT LINE itself ("N
alert(s) undelivered, oldest M minutes"). Because the tick already
narrates that line durably and the Stop hook already composes health
into its payload, ONE change lights both floors — the agent-free
terminal (`metasystem health`) and every seat's turn end — with no
new surface, which is what makes it fit slice 1 (§11). Undelivered
digest windows count separately, lower urgency. Delivery outcome
never gates machinery; only the launch gate refuses on readiness.

## 11. Slice plan

(Folds AC2-SLICE-001 and AC3-SLICE-GATE-CUTOVER-001: slice 1 lands
as remote-landed increments of at most 4 hours each and is
independently deployable because it CHANGES NO LEGACY BEHAVIOR — the
legacy queue keeps draining through `NotifyCommand` and the legacy
launch gate stands byte-for-byte; the new episode path is purely
additive, including the one additive `DeliverDueAlerts` call each
tick driver gains (§5). The gate cutover is its own
slice, behind `channel.gate`, strictly after queue retirement.)

1. **Alert path, Telegram, additive and live-token
   deployable.** The contract with single-chunk `Send` (SendResult
   carrying one outcome; the chunking path is dormant until slice 3),
   destination configuration for the alert class with the
   secret-layer skip, the UNTHREADED Telegram adapter (no
   conversation store yet — alerts do not need threads to reach a
   phone), the §5 journal/transport split with the sender flock and
   the §5a completion merge WITH its concurrent-writer fixture —
   including attempt `MessageRef` retention, which is SLICE-1 WORK:
   §5a step 3 merges the returned reference and the Telegram adapter
   already returns the message id (11a.7); only the conversation
   store that CONSUMES references is slice 4 — `DeliverDueAlerts`
   wired into BOTH tick drivers (the resident runner and the external
   `metasystem steward tick` command, §5), the 11a.12 retention
   handshake followed by the two 2026-09-01 producers
   (`delegate-job-failed`, 11a.8, and
   `stop-awaiting-resume`, 11a.9 — Wido's word pins both to this
   slice; the handshake's pin lands BEFORE the scans whose source it
   protects, 11a.12), the
   redaction invariant with its known-bad fixture, and the
   undelivered count in the health verdict line — which §1 shows
   reaches terminal and Stop hook through existing plumbing. The
   launch gate is NOT touched (§6): the legacy `NotifyCommand` check
   and the legacy queue's delivery keep working exactly as shipped,
   which is what makes this slice independently deployable under any
   configuration — a Telegram destination adds a route for episode
   alerts, removes none. Rough arithmetic, stated so it can be
   challenged: contract skeleton and config 1h, Telegram adapter and
   fixtures 1h, sender split plus completion-merge fixture 1.5h,
   health line plus redaction fixture 0.5h, the retention handshake
   with its interleaving fixtures 0.5h, the two producers plus
   the second driver call site 0.5h — ≈5 hours total, grown by the
   2026-09-01 producers and the handshake and disclosed rather than
   absorbed into the
   older estimates. The 4-hour law is a LANDING cadence, so the slice
   lands as two remote-landed increments — the alert path and adapter
   first, the handshake then the producers and floor second — each under 4 hours and
   each leaving the tree deployable. The round-1 floor
   (fallback, undelivered surfaces, redaction) ships inside the
   slice; enabling a live token at its end is lawful because nothing
   legacy stopped working.
2. **Digest class**: batch composition, the named second consumer
   cursor, Stop-hook cursor-record migration, noticings redirected.
3. **Chunking and spans**: multi-chunk SendResult live, digest span
   accounting, prefix cursor advance.
4. **Conversation reference store and the threaded Slack adapter**
   (Web API), plus reply mapping for Telegram/email/WhatsApp as each
   ships under the §3a invariant. Attempt `MessageRef` retention is
   NOT here: it lands in slice 1 with the completion merge (§5a);
   this slice adds only the store and threading that consume the
   retained references.
5. **Queue retirement**: every `QueueNotification` caller migrated;
   `DeliverPending` and the pending directory removed.
6. **Gate cutover**: `channel.gate=channel` becomes available and
   documented as the recommended setting; `EnsureRunner` and `arm`
   consult `Ready` when it is set. Lands only after slice 5, when
   every live producer delivers through the channel and the gate's
   guarantee is true again. Flipping the default to `channel` is a
   separate recorded decision once a deployment has run cut over.
7. **Blocked-on-human producers**: the class-scoped resolution law
   and the §7 producer table, producer by producer — MINUS the two
   producers Wido's 2026-09-01 word moved into slice 1 (11a.8,
   11a.9).
8. **Remaining adapters** (email, WhatsApp) with provider tests; the
   committed-secret validation rule with its governance record and
   marking-mode activation criterion.
9. **Bridge destinations**: `seat-<id>` outbound; the receive half
   proceeds under seat-mutual-awareness's design per §2b.

## 11a. Slice-1 mechanical specification (revision 6)

The Sol implementer gap-stopped slice 1 with seven gaps — places the
design referenced something without mechanically defining it. Each
subsection below closes one gap with exact fields, persisted
representations, and behavior; the numbering 11a.N answers gap N of
the gap-stop. Where slice 1 needs less than the full mechanism, the
slice-1 minimum is stated and the remainder carries a MARKED deferral
to its owning slice. Revision 7 adds 11a.8 and 11a.9 — the two
producers of Wido's 2026-09-01 binding word — and repairs in place
the four cross-section contradictions the second gap-stop proved.

### 11a.1 The sender stamp and the refusal journal

**Sender identity.** The transport phase's identity is the sender
PROCESS, probed from itself with the same kernel prober the
acknowledgment verb already uses (`identity.KernelProber{}.Probe` on
the sender's own process id): the pair (process id, process start
time in epoch seconds) that the census and `AlertInvoker` already
treat as a process identity.

**Persisted stamp and receipt.** FIVE additive fields on
`AlertAttempt` (episode schema stays 1; absent fields read as zero;
folds AC7-MESSAGEREF-PERSISTENCE-001 — the exact durable shape §7
assigns to slice 1):

```
senderPid          int64  // json "senderPid,omitempty"
senderPidStartedAt int64  // json "senderPidStartedAt,omitempty"
stampedAt          string // json "stampedAt,omitempty" — RFC 3339 UTC
channel            string // json "channel,omitempty" — destination name
messageRef         object // json "messageRef,omitempty" —
                          // {"id":"<string>","threadId":"<string>"},
                          // a *MessageRef in Go, the WHOLE field
                          // omitted when both members are empty
```

`channel` is BOUND AT STAMPING: the journal phase creates PENDING
attempts with `channel` absent (no destination is resolved under the
tick); the flock-holding sender writes the destination it will try
into `channel` in the same stamping write, and a restamp (dead
foreign sender) may rebind it. §7's destination-matched PENDING
reuse, mechanically: reuse the latest PENDING attempt whose `channel`
is empty or equals the destination being tried; a fallback try is its
own attempt with its own `channel`. `messageRef` is written only by
the §5a completion merge (step 3). Legacy attempts carry none of the
five fields and read as zero — no migration.

**Stamping rule.** Under the alert lock, the flock-holding sender
stamps every due PENDING attempt it is about to send by writing those
three fields and saving the episode. A PENDING attempt bearing a
FOREIGN stamp is a dead sender's (the sender flock is held
exclusively and released by the kernel at process death, so no other
stamper can be alive); the holder RESTAMPS it with its own identity
and proceeds — this is the crash-gap reuse, now observable in the
journal. Matching at completion means: same `senderPid` AND same
`senderPidStartedAt` as the completing process.

**The refusal journal.** A refused completion (§5a step 2: attempt
absent, not PENDING, or stamp mismatch) appends one line to
`artifacts/agents/steward/alerts-refusals.ndjson` — newline-delimited
JSON, one object per line, written under the alert lock with the
read-append-rewrite atomic pattern the narration file already uses,
capped at the newest 1000 lines (`alertRefusalCapLines = 1000`).
Line schema:

```
{"schema":1,"at":"<RFC3339 UTC>","episodeId":"...",
 "attemptSequence":N,
 "reason":"attempt-missing"|"attempt-not-pending"|"stamp-mismatch",
 "senderPid":N,"senderPidStartedAt":N,"details":"<sanitized>"}
```

After journaling a refusal the sender writes nothing else to that
episode in the same pass. The refusal file is evidence, not a queue:
nothing consumes it, and the undelivered-alert floor (11a.5) already
surfaces the stranded state because the episode remains
not-SUBMITTED.

### 11a.2 The five contract types, exactly

```
type MessageClass string
// Valid values exactly: "alert", "digest", "bridge".
// Slice 1 uses only "alert"; any other value is caller misuse
// (11a.3). "digest" activates in slice 2, "bridge" in slice 9.

type ContentSpan struct {
    Start int64 // inclusive byte offset into the digest register
    End   int64 // exclusive; 0 <= Start <= End
}
// Slice-1 minimum: alerts carry no spans; ChunkOutcome.Span is the
// zero value. Real spans are owned by slices 2–3 (MARKED deferral).

type AdapterCapabilities struct {
    Threads         bool
    Receive         bool
    MaxMessageChars int // Unicode code points; 0 = no character limit
    MaxMessageBytes int // UTF-8 bytes;         0 = no byte limit
}
// Two limit fields because providers count differently (Telegram
// counts characters, byte-oriented transports count bytes). The
// channel layer enforces every nonzero limit; characters are counted
// as Unicode code points of the message text after UTF-8 decoding.

type DestinationConfig struct {
    Name     DestinationName
    Adapter  string            // registry name
    Settings map[string]string // non-secret settings, committed keys allowed
    Secrets  map[string]string // resolved ONLY from env/.local (§10)
}
// A setting is a Secret iff its name is in the fixed list
// secretSettingNames = {"bot-token","webhook-url","smtp-password",
// "api-token"}; everything else is a Setting.

type ConversationState struct {
    First  MessageRef
    Latest MessageRef
}
// Slice-1 minimum: always the zero value (the conversation store is
// slice 4, MARKED deferral); every adapter must accept the zero
// value and send unthreaded/unlinked.
```

### 11a.3 The unconfigured-send outcome law

Two disjoint failure surfaces, so every alert-sender invocation has
exactly one journalable outcome per tried destination:

- **Caller misuse** — unknown destination name (no
  `channel.destination.<name>.*` key and no 11a.4 synthesis applies),
  invalid `MessageClass`, an empty required compose field, or a
  message exceeding the destination's declared limits (chunking is
  dormant in slice 1) — returns a TOP-LEVEL typed error
  (`ErrInvalidSend`) and an empty `SendResult`. The alert sender
  journals it as one `AlertAttempt` with `Result=TRANSPORT_FAILED`,
  `Channel=<the destination name it tried>`, and `Problem` prefixed
  `"send-refused: "`.
- **Unconfigured destination** — the name resolves but the adapter
  is unset, a required setting or secret is missing, or a secret was
  found only in the committed file (§10) — is NOT a top-level error:
  `Send` returns nil error and `SendResult{Chunks:[exactly one
  ChunkOutcome]}` whose `Err` is `ErrUnconfigured` and whose `Ref` is
  zero. The sender journals it as `Result=TRANSPORT_FAILED` with
  `Problem` prefixed `"unconfigured: "`.

No new persisted result value exists: the episode-store vocabulary
stays PENDING/TRANSPORT_SUBMITTED/TRANSPORT_FAILED, and the
unconfigured/refused distinction lives in the machine-checkable
problem prefix. The fallback destination is then tried as its own
attempt under the same law. Transport failures after submission
begins arrive per `ChunkOutcome` as already specified (§2).

### 11a.4 The implicit destination, without touching legacy bytes

The channel layer implements its OWN `command` and `desktop` adapters
inside `internal/channel` — it never calls, wraps, or receives an
injected `notify.go` transport, and `notify.go` plus the legacy queue
remain byte-for-byte as shipped, serving their existing callers and
the legacy launch gate. The channel's `command` adapter runs the
value of setting `exec` via `/bin/sh -c` with the composed text in
environment variable `STEWARD_MESSAGE` and the class in
`STEWARD_CLASS`; zero exit is submission (`MessageRef` zero). The
`desktop` adapter runs the `osascript` notification on darwin and
returns `ErrUnconfigured` elsewhere.

Implicit synthesis, exact: when `channel.alert.destination` is UNSET,
the channel layer synthesizes, in order:

1. If the git-config key `metasystem.steward.notify-command` is
   nonempty: destination `local-command`, adapter `command`, setting
   `exec` = that value (READ-only reuse of the legacy key; the legacy
   code path is not invoked).
2. Else, on darwin: destination `local-desktop`, adapter `desktop`.
3. Else: the alert destination is unconfigured (11a.3).

`channel.alert.fallback-destination` defaults to `local-desktop` and
resolves through the same synthesis. An EXPLICIT
`channel.destination.local-command.*` or `.local-desktop.*` block
overrides the synthesized one key-by-key.

### 11a.5 Undelivered counting, exactly

An episode is UNDELIVERED iff ALL hold:

- `Cleared` is false;
- `Acknowledged` is false (an acknowledged alert reached a human by
  stronger evidence than any transport receipt);
- `TransportResult` is not `TRANSPORT_SUBMITTED` (so PENDING and
  TRANSPORT_FAILED both count, including an in-flight stamped
  attempt);
- `len(Attempts) >= 1` (a held episode that never reached the alert
  boundary is silent by design and does not count).

Age runs from the FIRST attempt's `AttemptedAt` (the moment delivery
first became due). `M = floor(minutes since the earliest qualifying
first-attempt time)`, an integer. The health verdict line appends,
byte-exactly, `; N alert(s) undelivered, oldest M minute(s)` — the
literal strings `alert(s)` and `minute(s)`, no plural logic. When
N would be 0, NOTHING is appended: the line is byte-identical to
today's, so quiet narration diffs stay quiet. When the episode store
cannot be read, the line instead appends `; alert delivery state
unreadable: <sanitized error>` and never fabricates a zero.

### 11a.6 The health alert's three fields

The health producer persists only its precomposed verdict line; the
three-field alert is composed AT SEND TIME, so no persisted schema
changes:

- `Happened` = the episode's stored `Message` (the health verdict
  line), verbatim.
- `Asked` = the fixed string `Check this repository's health and
  repair or clear the failing component.`
- `Answer` = the fixed string `metasystem health --repo <repository
  path>` with the repository's absolute path substituted — a verb
  that exists today.

The acknowledgment line is appended by the composer as already
specified (§6). Richer per-verdict asks are lawful later precisely
because composition is send-time; any such enrichment is a design
change, not an implementer choice (MARKED deferral, unowned).

### 11a.7 The Telegram seam, exactly

- **Request**: HTTP POST to `<base-url>/bot<bot-token>/sendMessage`,
  `Content-Type: application/json`, body exactly
  `{"chat_id": <chat-id setting as a JSON string>, "text": <composed
  message as a JSON string>}`. No `parse_mode` (plain text; no
  entity escaping rules exist to get wrong).
- **Base URL**: non-secret setting `base-url`, default
  `https://api.telegram.org`. FAKE-ENDPOINT INJECTION is exactly
  this setting: tests point `base-url` at a local test server;
  no other injection seam exists.
- **Response validation**: submission succeeded iff HTTP status is
  200 AND the body is JSON with `"ok": true` AND
  `result.message_id` is present as an integer. `MessageRef.ID` is
  that integer's decimal string; `ThreadID` is empty. Anything else
  is `ErrSendFailed`; its sanitized problem carries the HTTP status
  and, when present, the response `description` truncated to 200
  bytes. The adapter NEVER places the request URL in any error — the
  URL embeds the token (§10's redaction is the second line of
  defense, not the first).
- **Timeout ownership**: the CHANNEL LAYER creates, per submission, a
  `context.Context` bounded by the existing 15-second notify timeout
  and passes it as `AdapterSend`'s FIRST PARAMETER — the §2a
  signature carries it, so the adapter honors a context it is
  actually handed (one contract). Adapters set no timeout of their
  own and return promptly with a typed sanitized error on
  cancellation.
- **Capability**: `AdapterCapabilities{Threads: false, Receive:
  false, MaxMessageChars: 4096, MaxMessageBytes: 0}` — Telegram's
  limit is character-based, which the 11a.2 two-field shape now
  expresses without ambiguity. Slice-1 guarantee: in the normal case
  the compose-side cap (§9, 1500 bytes) bounds the text at ≤ 1500
  code points, under 4096; in §9's disclosed degenerate overflow the
  hard law is unchanged — a message that exceeds a declared limit is
  caller misuse per 11a.3 (never silent truncation at the channel
  layer — truncation is compose-side only, §9).

### 11a.8 The delegate-job-failed class (slice 1; Wido's word, 2026-09-01, binding)

The alert class "delegate job failed under a claimed goal" — the
2026-09-01 six-hour idle's missing class — is a slice-1 producer.

- **Producer mechanism — an idempotent derivation scan, not a
  writer hook** (folds AC7-PRODUCER-ATOMICITY-001 and
  AC7-JOB-WRITER-001): the tick's journal phase (§5 phase 1) scans
  `artifacts/agents/jobs/*.json` and, for every record whose status
  is terminal `failed` or `timeout` (never `cancelled`: a deliberate
  cancellation is not a failure) with a nonempty `goalId`, ensures
  the episode keyed by 11a.10's digest exists with its class, facts,
  Message, and a PENDING attempt — all under the alert lock, no
  network. NO transition writer touches the episode store: the
  record writers (`RecordCAS`, `RecordProtocolError`, the reaper —
  §1's corrected two-writer trace) stay byte-unchanged, and the scan
  is writer-independent by construction, covering
  `RecordProtocolError` and any future terminal writer without
  enumeration. Crash-and-outage proof (folds
  AC8-JOB-SOURCE-RETENTION-001, with 11a.12): the terminal record is
  durable before any episode work begins; evidence GC DOES prune
  mirrored terminal records (§1's corrected trace), but 11a.12's
  retention handshake forbids collecting a failed/timeout record
  carrying a goal until the episode its digest names exists durably
  — so however long the runner outage, the first tick after it still
  finds the record and journals the episode, and only THEN may GC
  prune. The scan re-derives on every tick until the episode exists;
  a crash between record and episode write only delays one tick;
  once the PENDING attempt exists, the store's shipped
  at-least-once crash law owns the rest, and the episode carries the
  facts (11a.10), so pruning the record afterwards loses nothing.
  The interleaving table is 11a.12's. The only unalertable loss
  left is destruction of the record outside every shipped path —
  loss of the very truth the alert reports.
  This obeys §7's source-of-truth law: the episode store stays the
  only durable DELIVERY state; the job record stays the system of
  record for the failure facts. **Read set and bound** (folds
  AC8-SCAN-BOUNDEDNESS-001 for this scan): the job id IS the
  record's filename stem (`<job-id>.json`, the store's keying —
  11a.12 uses the same derivation), so the class digest is
  computable from the DIRECTORY LISTING alone. Per tick the scan
  performs exactly: one `ReadDir` of `artifacts/agents/jobs/`; a
  digest lookup per entry against the tick's episode index (built
  from one `ReadDir` of the alerts directory per tick, shared with
  11a.9 — the shipped full-episode load is acceptable in slice 1
  because 11a.12's collection rule bounds that directory); and one
  bounded record read (status, goalId, error) ONLY for entries
  whose digest has no episode. Episodes are WRITE-ONCE (11a.10):
  once the digest has an episode the entry is skipped without
  opening the file, so steady-state per-tick work is two directory
  listings and zero record reads. No durable cursor exists BY
  DECISION: record files are mutable in place (terminal state is
  CAS-patched), which makes an mtime or offset watermark unsound;
  the episode store itself is the scan's durable checkpoint —
  exactly the memory the digest skip consults. The directory's size
  is bounded by GC's retention contract (§1: grace window,
  current-revision spending facts, 11a.12 pins — pins drain on the
  first tick after an outage), not by history. An unreadable or
  unparseable record
  is skipped this pass (the reaper's own shipped treatment) and
  retried next tick. Latency, disclosed: journaled at most one tick
  interval after the record lands and delivered by that same tick's
  sender pass — no later than revision 7's writer-hook design, which
  also waited for a sender pass.
- **Carried facts** (persisted shape in 11a.10): goal id, job id,
  and failure reason — the record's `error` field when nonempty,
  else the terminal status word.
- **Dedup, per job**: the episode digest is 11a.10's encoding of the
  pair (`delegate-job-failed`, job id), riding the store's existing
  digest-keyed dedup (§1) — one episode per job ever (these episodes
  are never auto-cleared, 11a.10); once the episode exists the scan
  writes NOTHING for that digest — episodes are write-once, facts
  and Message set at creation from the record (a terminal record's
  alert-borne facts do not change: the traced writers set status,
  goalId, and error at terminalization; post-terminal CAS patches
  touch closure bookkeeping). One-episode-per-job-ever survives
  11a.12's collection because collection requires the source record
  to be ALREADY GONE, and minting requires the record.
- **Composition at send time (the 11a.6 pattern)**: `Happened` =
  `delegate job <job-id> failed under goal <goal-id>: <failure
  reason>`; `Asked` = the fixed string `Delegated work under this
  claimed goal stopped; decide whether to redispatch, follow up, or
  hand the work over.`; `Answer`, byte-exact under §6's placeholder
  law: the ASCII bytes `metasystem delegate --follow-up ` + the job
  id + ` --brief <corrective-brief-file>`, single spaces, no
  trailing space. SUBSTITUTED: `<job-id>` only. VERBATIM:
  `<corrective-brief-file>` — its literal 23 bytes, brackets
  included — because the corrective brief does not exist yet; the
  human authors it and replaces the token with its path. The
  recorded correction verb stays total even for a session that
  cannot be resumed (the typed fresh-context fallback). Richer
  per-failure asks follow 11a.6's enrichment law
  (design change, not implementer choice).

### 11a.9 The stop-awaiting-resume class (slice 1; Wido's word, 2026-09-01, binding)

The breach-stop's stop-awaiting-resume alert is an EXPLICITLY WIRED
slice-1 producer, not a later-slice deferral.

- **Producer mechanism — an idempotent derivation scan over the
  goal tree, not the custodian's reports** (folds
  AC7-PRODUCER-ATOMICITY-001 for this class): the tick's journal
  phase (§5 phase 1) projects the goal tree once and, for every LIVE
  CLAIMED goal whose `StopFence` is present AND for which resume's
  OWN precondition passes — `VerifyStopBatchComplete(root, goalID,
  capability, fence)` returns nil (`internal/goal/stop.go` 248–262):
  the batch reads cleanly, is COMPLETE, AND binds the exact stopped
  authority, goal id, goal revision, fence epoch, capability
  generation, machine, claim epoch, and reason all equal — ensures
  the episode keyed by 11a.10's digest (folds
  AC8-STOP-BATCH-BINDING-001: the scan demands exactly the evidence
  the prescribed verb will demand, so "alert" and "resume will
  accept" are one predicate). Read set and bound (the 11a.9 half of
  AC8-SCAN-BOUNDEDNESS-001): the tick's EXISTING single goal-tree
  projection plus one `ReadStopBatch` per live claimed goal carrying
  a fence — bounded by the live-goal count, no history walk; the
  episode side rides the same per-tick episode index as 11a.8. §1 traces why this condition
  is exact durable proof: `EnsureBreachStop` writes the fence only
  when closing launch, and `goal resume` verifies batch completeness
  then clears the fence into a fresh revision — so the condition
  holds on EVERY tick from fence closure until the human resumes,
  and a missed episode write self-heals next tick.
  `FindBreachStops`'s omit-completed behavior (§1) is why the
  custodian's one-shot COMPLETE report could not be the trigger.
  Crash-window proof: fence and batch are durable before any episode
  work; the scan re-derives until resume; after resume no alert is
  owed — the human has acted (a resume landing mid-pass is governed
  by the resume-race ordering rule below).
- **Stop-outcome verdict table, total** (folds
  AC7-STOP-OUTCOME-001 — the alert asserts a closed fence and
  prescribes `goal resume`, so it fires only on proof of both):
  - Closed `StopFence` + `VerifyStopBatchComplete` passes →
    **ALERT** (the sole alerting condition — the scan runs the SAME
    check resume runs, so the prescribed verb cannot refuse what the
    alert asserts).
  - Closed `StopFence` + batch reads COMPLETE but the binding
    comparison fails (contradictory goal id, revision, fence epoch,
    capability generation, machine, claim epoch, or reason) → **NO
    ALERT**: resume would refuse this batch; the contradiction is a
    defect for the health breaker, never an advertised action.
  - Closed `StopFence` + batch INDETERMINATE or unreadable → **NO
    ALERT** yet: the fence closed but `goal resume` would refuse an
    incomplete batch; the route stays visible to the custodian and
    the ordinary health breaker until the batch completes. (This
    row governs CREATION only; an already-journaled episode is
    governed by the lifecycle rule below, which HOLDS it.)
  - Custodian report FAILED (stop command failed) → **NO ALERT**: no
    proof any fence closed; the route reappears next tick and the
    custodian retries; persistent failure is the health breaker's,
    the tick's stated sole escalation owner.
  - Route INDETERMINATE (budget unknown, missing stop capability) →
    **NO ALERT**: no fence was closed.
  - Goalless FAILED (route-resolution failure) → **NO ALERT**:
    carries no goal; health breaker — unchanged revision-7 law.
- **Dedup**: 11a.10's encoding of (`stop-awaiting-resume`, goal id,
  revision) — one WRITE-ONCE episode per stopped revision (facts and
  Message set at creation; a fence's alert-borne facts do not
  change), skipped by digest on every later tick; its exit is owned
  by the lifecycle rule below, and a later stop lands on a fresh
  resumed revision, hence a fresh digest.
- **Episode lifecycle — one positive predicate each way** (folds
  AC8-STOP-INDETERMINATE-LIFECYCLE-001): CREATE only when
  `VerifyStopBatchComplete` passes — the verdict table's sole
  alerting row. CLEAR only on positive proof the fence is GONE: the
  goal's current state shows no `StopFence` bound to the episode's
  revision — the fence field absent, or the goal revision superseded
  (`goal resume` does both in one transaction, §1's trace).
  Everything else — a batch reading INDETERMINATE, an unreadable
  batch or goal file, a binding comparison failing on a previously
  verified fence — is **HOLD**: the episode and its PENDING attempt
  stay journaled byte-untouched and no send fires this pass, because
  a send requires a fresh verify-pass (the recheck below). A
  COMPLETE batch turning transiently unreadable therefore neither
  cancels delivery (revision 8's clear-when-condition-false rule did
  exactly that — the contradiction the finding proved) nor re-alerts
  nor asks the implementer to guess: the alert stays owed, and a
  permanently unreadable batch surfaces through 11a.5's undelivered
  floor rather than vanishing.
- **The resume-race ordering rule — a pre-send source recheck**
  (folds AC8-STOP-RESUME-RACE-001): `goal resume` serializes on the
  goal-revision lock and the tick on the arbitration lock — no
  shared lock (§1's race fact) — so a resume can land between the
  journal phase and the transport phase and leave a stale pending
  episode. The rule: in §5's transport phase, between stamping and
  transport, the sender re-runs the ONE predicate against current
  goal state for every `stop-awaiting-resume` attempt.
  Verify passes → send. Fence-gone proof (the clear predicate
  above) → CLEAR the episode instead of sending, through the §5a
  merge under the alert lock — §5's DUE definition and 11a.5's
  counting both exclude cleared episodes, so the suppressed alert
  leaves due and undelivered together. Anything else → HOLD: skip
  the send, leaving the attempt PENDING for a later pass (11a.1's
  restamp rule reclaims the dead stamp). Residual window, disclosed:
  a resume landing after the recheck's read and before the provider
  accepts the transport call can still produce one already-answered
  message; that window is one read-to-send span, and the design
  accepts it — the alternative, holding the goal-revision lock
  across network transport, would couple goal mutation to provider
  latency. The stale message asserts a state that was durably true
  at recheck time, and §2's outbound contract has no retraction.
- **Composition at send time** (persisted facts in 11a.10):
  `Happened` = `goal <goal-id> revision <revision> hit its budget
  fence: breach-stop <stop-id> is complete; the goal waits for
  resume.` — no state interpolation: only COMPLETE alerts, and a
  `StopFence` always carries its stop id (§1's fence validation);
  `Asked` = the fixed string `The budget fence closed this revision
  and nothing will move it without you; decide whether to resume the
  goal.`; `Answer`, byte-exact under §6's placeholder law and
  resume's REAL interface — `--id` and `--by` required, the complete
  budget tuple mandatory (§1's trace of `goalsync_mutations.go`
  104–180): the ASCII bytes `metasystem goal resume --id ` + the
  goal id + ` --by <name> --elapsed-limit <duration> --attempt-limit
  <count> --reserved-job-minutes-limit <minutes> --active-job-limit
  <count>`, single spaces, no trailing space. SUBSTITUTED:
  `<goal-id>` only (already substituted in the bytes above).
  VERBATIM: `<name>`, `<duration>`, `<minutes>`, and both `<count>`
  tokens — the resuming human's identity and the FRESH budget tuple
  are exactly the decision the alert asks for; the composer cannot
  hold them, and resume refuses without them, so a copy-paste
  without editing them fails loudly instead of resuming on stale
  budgets.

### 11a.10 Episode class, facts, digest encoding, class-scoped lifecycle

(Folds AC7-PRODUCER-STATE-001 and AC7-DEDUP-ENCODING-001.)

Two additive fields on `AlertEpisode` (schema stays 1; absent reads
as zero — every shipped health episode has neither and is class
health):

```
class string            // json "class,omitempty":
                        // "" (health) | "delegate-job-failed" |
                        // "stop-awaiting-resume"
facts map[string]string // json "facts,omitempty"; exact keys:
                        // delegate-job-failed:  goalId, jobId, reason
                        // stop-awaiting-resume: goalId, revision
                        //   (base-10, no leading zeros), stopId
```

**Message invariant**: the shipped loader refuses an empty `Message`
(`alert_episode.go` 119–121), so the scans set `Message` at journal
time to EXACTLY the class's rendered `Happened` line from `facts` —
deterministic denormalization, set ONCE at creation: producer
episodes are WRITE-ONCE (11a.8's skip law; both classes' alert-borne
facts are immutable at their source), so refresh-on-every-pass does
not exist; a fixture asserts `Message` equals the
composer's `Happened`. Send-time composition (11a.6 pattern)
switches on `class`: health composes per 11a.6 from `Message`; the
two new classes compose per 11a.8/11a.9 from `facts`.

**Digest encoding, exact**: the episode digest is the lowercase
64-hex SHA-256 of the UTF-8 bytes of the tuple's elements — class
literal first — joined by single LF (`\n`) bytes, no trailing
newline (LF is safe: no id or revision contains one). So
`delegate-job-failed` + LF + job id; `stop-awaiting-resume` + LF +
goal id + LF + revision base-10. This satisfies the store's
`validEvidenceDigest` (64 lowercase hex,
`component_evidence.go` 441–444). Pinned fixture vectors:
`delegate-job-failed\nimplementer-c002e6035a243bdbc1400067` →
`aba14958258cb095ae8f35356c939b238a90044435751584fd029304981f3b95`;
`stop-awaiting-resume\nalert-escalation-channel\n8` →
`8a6c1ffb2f72d5ae750e890e30c9a12a72bdceec6914029f73b3956e9f8e790d`.

**Class-scoped lifecycle, mechanical** (§7's law made real — the
shipped healthy verdict clears EVERY episode, `alert_episode.go`
246–268): `UpdateAlertEpisodes`' healthy-clear loop and its
resolve-all-others loop are restricted to episodes whose `class` is
`""` (health). `delegate-job-failed` episodes are NEVER auto-cleared
— acknowledgment is their terminal human step, and never clearing is
what makes one-episode-per-job-ever hold under the uncleared-digest
dedup match; the proof survives 11a.12's collection because an
episode may leave the store only after its source record is already
gone, and minting requires the record — a collected episode is
unre-mintable by construction. `stop-awaiting-resume` episodes clear
ONLY on 11a.9's positive fence-gone proof (its scan or the pre-send
recheck); anything unreadable HOLDS them (11a.9's lifecycle rule —
a clear is never inferred from a failed read).
`AcknowledgeAlert` is unchanged for all classes. The class-scoped
clearing fixture: a healthy health verdict must clear health
episodes and leave both producer classes' episodes byte-untouched.

### 11a.11 The tick drivers' delivery law, both branches

(Folds AC7-TICK-ERROR-PATH-001.) Each driver calls
`DeliverDueAlerts` on EVERY pass, immediately after its existing
`DeliverPending` call, on the failed-`RunTick` branch as well as the
successful one — four call sites total: the resident runner's loop
(one site, reached on success and failure alike, §1) and the
external command's error branch (`steward_verbs.go` 236–239) plus
its success path (line 270). Rationale: delivery reads only the
episode store, and pending episodes from earlier passes exist
regardless of this tick's outcome; a failed `RunTick` may also have
journaled before failing. Printed bytes unchanged on both branches
— the success report and the error branch's stderr are
byte-identical to shipped; `DeliverDueAlerts`' outcomes live in the
episode journal and surface through the 11a.5 floor (§5's law,
restated for the error branch).

### 11a.12 The retention handshake — job-record source retention

(Folds AC8-JOB-SOURCE-RETENTION-001, the round-2 critical: revision
8's crash-window proof leaned on the FALSE fact that terminal
records are never deleted; §1 now traces the real collector —
`pruneMirroredRecords` after the 5,400-second default grace window,
narrowed further by `keepsSpendingFact` to the current claimed
revision.)

**The pin, exact**: evidence GC's mirrored-record pruning
(`internal/evidence/gc.go` 375–449 — and, by the same rule, any
future collector of `artifacts/agents/jobs/*.json`) gains ONE
additional precondition, ANDed with every shipped condition
(mirror-hash equality, the grace window, `keepsSpendingFact`) and
never weakening them: a record whose status is terminal `failed` or
`timeout` and whose `goalId` is nonempty MUST NOT be collected until
the episode named by 11a.10's digest of (`delegate-job-failed`, job
id) exists durably under `artifacts/agents/steward/alerts/`. The
check needs no new state: the job id is the record's filename stem
(11a.8's derivation), the digest is computable from the listing
entry alone, and existence is one stat of the digest-named episode
file. `cancelled` and goalless records are untouched — the pin
covers exactly the records 11a.8 alerts on, nothing more.

**Why every interleaving is safe** (the crash-and-outage proof
11a.8's fold leans on):

| Interleaving | Outcome |
| --- | --- |
| Record lands terminal; the runner stays down PAST the grace window and past `keepsSpendingFact` retention (goal revision superseded or goal unclaimed); GC runs any number of times during the outage | Every GC pass finds no episode → the pin holds → the record survives the entire outage; the first tick after it journals the episode (11a.8's scan), and only then may GC collect. This is exactly the window the finding proved open. |
| The tick crashes after the episode write, before delivery | The episode is durable (§1: atomic, durability-verified save) → GC may collect the record; delivery is owed by the episode store's shipped at-least-once crash law, and the facts ride the episode (11a.10) — pruning the record loses nothing. |
| The tick crashes between its `ReadDir` and the episode write | No episode exists → the pin holds → the next tick re-derives (11a.8's idempotent scan). |
| GC's existence check races the tick's episode write | The pin errs only toward RETENTION: collection needs positive existence proof, a stale "no episode" read merely retains the record one extra pass, and a false "exists" read cannot occur — an episode file, once durably written, is removed only by the converse rule below, which itself requires the record to be already gone. |

Pins DRAIN: a pinned record is precisely one whose digest 11a.8's
scan has not yet journaled, so the first completed tick after any
outage journals every pinned digest and releases every pin — pinned
volume is bounded by the failures of one outage, never by history.

**The converse collection rule** (what bounds the episode store and
completes 11a.8's read-set bound): GC may collect an episode file
iff BOTH hold — (a) its delivery obligation is terminally closed:
`Acknowledged` for `delegate-job-failed` (the class's only terminal
step, 11a.10); `Cleared` or `Acknowledged` for
`stop-awaiting-resume`; and (b) its producer can never re-mint the
digest: for `delegate-job-failed` the source record is already gone
(minting requires the record, so one-episode-per-job-ever survives
collection); for `stop-awaiting-resume` the recorded clear already
proves fence-gone, and any later stop lands on the resumed
revision's fresh digest. An episode never leaves the store
undelivered: condition (a) is a human act or a fence-gone proof,
never a timeout. Honest residual: episodes the human never
acknowledges accumulate — bounded by unanswered failures, each one
owed attention, which is 11a.5's floor doing its job, not a leak.

**Slice placement** (§11): the handshake lands IN slice 1, in the
producers' increment and ordered BEFORE the producers within it — a
producer running against a collector without the pin is the reopened
window, while the pin without the producers only delays collection
until the same increment completes.

## 12. Finding dispositions — all rounds, honestly

Round-2 fold-fidelity correction: revision 2's table marked
AC-CONTRACT-001, AC-RECEIPT-001, and AC-SLICE-001 "folded" while the
receive half, per-chunk receipts, and the 4-hour deployable slice
were respectively missing, unrepresentable, and withdrawn. The
corrected record:

| Finding | Disposition |
| --- | --- |
| Round 1 AC-CONTRACT-001 | Partially folded in rev 2 (outbound fields, destinations); COMPLETED in rev 3 by §2's SendResult contract and §2b's explicit, obligation-enumerated receive boundary. The rev-2 table over-claimed. |
| Round 1 AC-BLOCK-001/002 | Rev-2 fold was unsound (missed the arbitration lock); REPLACED by §5's single-flight sender, completed in rev 4 by §5a's merge law. The Ready gate exists but its cutover moved behind `channel.gate` after queue retirement (rev 4, §6). |
| Round 1 AC-STATE-001, AC-DEDUP-001, AC-STATE-002, AC-CREDENTIAL-001 | Folded in rev 2; unchanged and standing (§7, §8, §10). |
| Round 1 AC-RECEIPT-001 | Rev-2 fold incomplete; completed by §2/§7/§9 (ChunkOutcome, per-outcome attempts, MessageRef retention). |
| Round 1 AC-SLICE-001 / AC-CONTRACT-002 | Rev-2 fold regressed the 4-hour requirement; repaired by §11's narrowed slice 1; size law completed by §9. |
| AC2-LOCK-001 (critical) | Folded, §5: one implementation, kernel-enforced single flight, laws argued individually; "immediate" honestly redefined as within one sender pass. |
| AC2-CONTRACT-001 (critical) | Folded, §2b: receive explicitly withdrawn from this contract and reserved with five enumerated obligations for seat-mutual-awareness; sender authentication disclaimed; the disposition dishonesty corrected in this table. |
| AC2-THREAD-001 | Folded, §3: channel-layer-owned conversation reference store for ALL adapters, ConversationState into AdapterSend, MessageRef on attempts, Slack moved to the Web API, join-loss on store loss disclosed. |
| AC2-RECEIPT-001 | Folded, §2/§9: channel layer defined concretely; SendResult with per-chunk outcomes and spans; split ownership fixed in the interface. |
| AC2-SLICE-001 | Folded, §11: slice 1 narrowed to a 4-hour alert path with floor and redaction inside it. Rev 3 kept the gate swap in the slice, which round 3 proved unsafe; rev 4 removed it (see AC3-SLICE-GATE-CUTOVER-001). |
| AC3-SENDER-MERGE-001 (critical) | Folded, §5a: completion is a reload-and-merge critical section touching only the stamped attempt's receipt fields and the derived transport summary; the invariant is stated verbatim for a fixture, with the concurrent-acknowledge and concurrent-clear test cases named. |
| AC3-THREAD-ANCESTRY-001 | Folded, §3a: ThreadID defined as adapter-owned threading state under a sufficiency invariant; for email the References chain is retained inductively in the latest reference per RFC 5322 §3.6.4. Rev 4's trimming clause was unimplementable (round 4's finding); rev 5 bounds it. |
| AC3-SLICE-GATE-CUTOVER-001 | Folded, §6/§11: slice 1 no longer touches the gate — the legacy check and legacy queue delivery stay byte-for-byte, making the slice purely additive; the cutover is its own slice behind `channel.gate` (default `legacy`), landing only after queue retirement restores the gate's guarantee. |
| Round 3: receive-half reservation; multipart receipts | Judged SOUND by the round-3 critic; unchanged. |
| AC4-EMAIL-TRIM-BOUNDARY-001 | Folded, §3a: the trimming boundary is the fixed design constant `emailReferencesMaxBytes = 8192` bytes of unfolded References value, honestly attributed to this design (RFC 5322 §2.2.3 limits physical lines only, handled by folding, and §3.6.4 specifies no trimming); boundary behavior is deterministic (drop position-2 entries until the value fits, keeping root and the most recent contiguous suffix; parent-only in the pathological case) and a long-chain fixture is required in the email adapter's provider tests. |
| Round 4: §5a merge law and every other line | Judged SOUND by the round-4 critic; unchanged. NO FINDING REMAINS OPEN across all four rounds. |
| Sol implementer gap-stop (seven gaps, slice 1) | Folded, §11a: gap 1 → 11a.1 (sender stamp fields and refusal journal), gap 2 → 11a.2 (the five contract types), gap 3 → 11a.3 (unconfigured-send outcome law), gap 4 → 11a.4 (own command/desktop adapters plus implicit-destination synthesis, legacy bytes untouched), gap 5 → 11a.5 (undelivered counting and the unreadable-store line), gap 6 → 11a.6 (health alert three-field mapping with a real answering verb), gap 7 → 11a.7 (Telegram request/validation/timeout/fake-endpoint/capability). A gap-stop is correct delegate behavior; these were design debts, not implementer questions. |
| Second implementer gap-stop (four cross-section contradictions, slice 1) | Folded, revision 7: contradiction 1 → `MessageRef` retention decided INTO slice 1 (§5a, §7, §11 slices 1 and 4 agree); contradiction 2 → `context.Context` added to `AdapterSend` (§2a) and 11a.7 restated over it, one contract; contradiction 3 → `DeliverDueAlerts` wired into BOTH tick drivers in slice 1 (§5, §11), external report bytes unchanged; contradiction 4 → exact truncation law in §9 (tail bytes, code-point boundary, Happened-only shortening). Root cause was skipping a self-consistency pass after §11a's one-pass addition; the pass is now performed and recorded in the status line. |
| Wido's 2026-09-01 binding addition (idle-loss specimen) | Folded: `delegate-job-failed` (11a.8) and `stop-awaiting-resume` (11a.9) enter the enumerated classes (§1 traced facts, §7 producer table) as slice-1 producers (§11). |
| AC7-PRODUCER-ATOMICITY-001 (critical) | Folded, 11a.8/11a.9: both producers become idempotent derivation scans over durable source state in the tick's journal phase — no dual-write exists to lose; crash-window proofs stated in each subsection; §1 traces the source durability (records retained, fence persists until resume). |
| AC7-PRODUCER-STATE-001 | Folded, 11a.10: additive `class` and `facts` episode fields with exact keys, Message-invariant denormalization, and the class-scoped clearing fixture; the shipped clear-everything healthy loop is restricted to class health. |
| AC7-STOP-OUTCOME-001 | Folded, 11a.9: total verdict table — only closed fence plus COMPLETE batch alerts (resume's own precondition); FAILED, INDETERMINATE, and goalless outcomes enumerated NO ALERT with their owners; the false `<state>` interpolation removed from `Happened`. |
| AC7-JOB-WRITER-001 | Folded, 11a.8 + §1: the scan is writer-independent, covering `RecordProtocolError` and future writers without enumeration; §1's only-through-the-CAS over-claim corrected to the two-writer trace. |
| AC7-MESSAGEREF-PERSISTENCE-001 | Folded, 11a.1: `channel` and `messageRef` join the persisted `AlertAttempt` with exact JSON names, omission and zero-read rules, stamping-time channel binding, and the mechanical destination-match rule. |
| AC7-SEND-OUTCOME-001 | Folded, §2: restated over 11a.3's law — top-level error is caller misuse only; unconfigured is nil error plus one ErrUnconfigured chunk; one contract, cross-referenced both ways. |
| AC7-COMPOSER-BYTES-001 | Folded, §6: the four-line composed message fixed byte-exactly (labels, LF separators, no trailing newline, the acknowledgment line's shipped verb and flags); §9's never-cut set defined as everything but the Happened value. |
| AC7-DEDUP-ENCODING-001 | Folded, 11a.10: class-literal-first LF-joined UTF-8 tuple, SHA-256, lowercase 64-hex, no trailing newline, revision base-10; two pinned fixture vectors. |
| AC7-TICK-ERROR-PATH-001 | Folded, 11a.11: `DeliverDueAlerts` on every pass of both drivers including the external command's failed-`RunTick` branch, printed bytes unchanged. |
| AC8-JOB-SOURCE-RETENTION-001 (critical) | Folded, 11a.12/11a.8/§1: revision 8's "terminal records are never deleted" was FALSE — §1 now traces evidence GC's real pruning; the retention handshake pins terminal failed/timeout goal-carrying records until their episode exists durably, the interleaving table proves the outage window closed, and the converse collection rule bounds the episode store. |
| AC8-STOP-BATCH-BINDING-001 | Folded, 11a.9: the scan's sole alerting condition IS resume's own precondition `VerifyStopBatchComplete`, full binding comparison included; a COMPLETE batch with contradictory coordinates is an enumerated NO-ALERT row for the health breaker. |
| AC8-STOP-RESUME-RACE-001 | Folded, 11a.9/§5: the pre-send source recheck between stamping and transport — send on verify-pass, clear on fence-gone proof, hold otherwise; the read-to-send residual window disclosed and accepted over coupling the goal-revision lock to provider latency. |
| AC8-SCAN-BOUNDEDNESS-001 | Folded, 11a.8/11a.9/11a.12: enumerated per-tick read sets (two directory listings steady-state; one `ReadStopBatch` per live fenced goal), write-once episodes with the episode store as the durable checkpoint, cursorlessness argued from in-place record mutability, and both directories bounded — jobs by GC's contract plus draining pins, alerts by the converse collection rule. |
| AC8-STOP-INDETERMINATE-LIFECYCLE-001 | Folded, 11a.9/11a.10: one positive predicate each way — create on verify-pass, clear only on fence-gone proof, HOLD on anything unreadable or indeterminate; revision 8's clear-when-condition-false rule, which silently cancelled delivery, is removed from both sections. |
| AC8-ANSWER-BYTES-AND-ACTION-001 | Folded, §6/11a.8/11a.9: the placeholder law makes every angle-bracketed token SUBSTITUTED or VERBATIM with each class enumerating its own; 11a.8's Answer fixes the corrective-brief token as literal bytes; 11a.9's Answer carries resume's real interface — `--id`, `--by`, and the mandatory complete budget tuple — with the fresh budget values verbatim because they are the decision being asked. |

## 13. Self-grade (R-24-m1, refreshed for revision 9)

- **Confidence:** 0.74. All six round-2 findings were checked
  against the shipped code they cite before folding (all six
  verified true; none refutable) — including the one that FALSIFIED
  a revision-8 traced fact ("terminal records are never deleted"),
  and that is why this grade drops from 0.76: the tracing discipline
  itself missed a live collector, and this document's crash proofs
  are only as strong as its traces. Priced against that: the two
  load-bearing folds REUSE shipped predicates instead of paralleling
  them — the stop scan alerts on resume's own
  `VerifyStopBatchComplete`, and the retention pin derives from the
  store's own filename keying — which is the direction that has
  historically survived this document's critique rounds; and
  revision 9 again adds a new subsection (11a.12) in one pass,
  mitigated by the recorded self-consistency pass over every changed
  pair.
- **Reject condition, stated plainly:** reject this revision if the
  implementer gap-stops on slice 1 a THIRD time. A third stop means
  revision-scale patching cannot make this document mechanical, and
  the design must then be split into a separate implementation
  specification rebuilt from the episode store's and channel layer's
  actual types, not grown further.
- **Weakest claim (new):** the 11a.12 retention handshake spans two
  components with no shared lock and no enforcement seam — evidence
  GC must honor an episode-store precondition that nothing in GC's
  own state compels, so a future GC edit that drops the pin reopens
  the outage window silently, and only 11a.12's interleaving
  fixtures would notice; the design mitigates by making the pin one
  ANDed predicate at one traced call site, but the coupling is
  discipline, not mechanism. Previously weakest, still standing: the §11a.1
  stamp-and-restamp rule — it derives
  "a foreign stamp on a PENDING attempt belongs to a dead sender"
  from the sender flock's exclusivity, which holds only while every
  sender honors the flock discipline; a code path that sends without
  the flock (a future manual verb, a test harness shortcut) would
  make restamping a live sender's attempt possible, and nothing in
  the persisted state can detect that violation. Second-weakest: the
  §5a refusal-branch enumeration, unchanged from revision 5 — its
  completeness is proven only by the implementation's fixtures.
- **Reject condition:** reject this revision if the implementer's
  next attempt still gap-stops on slice 1 — that would mean §11a's
  specification level is systematically short of mechanical, and the
  design should then be split into a separate implementation
  specification document rather than grown further; or if reading
  the legacy git-config notify key from the channel layer (11a.4)
  is ruled to violate the byte-for-byte legacy constraint even as a
  read-only reuse, which removes the implicit destination and makes
  an unset alert destination simply unconfigured.
- **Reject condition:** reject this revision if the completion-merge
  invariant cannot be held as one critical section on the actual
  episode file layout (for example if a future store shards attempts
  into separate files, reload-merge-save stops being atomic and the
  law needs a version check instead); or if operations requires the
  Ready gate in the FIRST deployable slice, which §6 now forbids
  until queue retirement — that demand reopens the cutover ordering,
  not slice 1; or if any platform in use does not release the sender
  flock on process death, which voids §5's crash law.
