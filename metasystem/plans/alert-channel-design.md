# Alert Channel Design — alert-escalation-channel (revision 4)

Status: revision 4 folds the three round-3 Sol findings
(one critical). Nothing is left unfolded. The critical is answered
with a testable completion-merge law (§5a): the sender's completion
write reloads the current episode under the alert lock and merges
ONLY receipt fields, so an acknowledgment or clearing recorded while
transport was in flight survives by law, not by luck. Email reply
ancestry is fixed by defining `MessageRef.ThreadID` as
adapter-owned threading state with a stated sufficiency invariant —
for email it carries the complete References chain (§3a). The launch
gate cutover leaves slice 1 entirely: the legacy gate stands
untouched until every legacy-queue producer has a channel route, and
the cutover is its own slice behind a configuration default (§6,
§11). Round 3 also closed two lines as SOUND: the receive-half
reservation (§2b) and multipart receipt outcomes (§2/§9); they are
unchanged.

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
  currently runs INSIDE the alert lock (lines 324–356).
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
  mission-runner answer`. No approve/reject verb exists.
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
    Capabilities(dest DestinationName) AdapterCapabilities // threads, maxMessageBytes
}
```

`Send`'s top-level error covers only pre-transport failure (unknown
destination, unconfigured); once submissions begin, every outcome —
success or failure, per submission — is a `ChunkOutcome`. This is the
per-chunk receipt AC2-RECEIPT-001 found unrepresentable: the episode
owner journals one attempt per outcome (§7), and the digest owner
advances its cursor from the outcomes' spans (§9).

### 2a. The adapter interface

```
AdapterSend(resolved DestinationConfig, text string,
            conv ConversationState) (MessageRef, error)
```

One submission, one reference, one typed sanitized error. No
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
- **Email**: `ThreadID` carries the COMPLETE References ancestry —
  the value the next reply's References header must contain. On each
  send the adapter composes In-Reply-To from the latest `ID`
  (the parent's Message-ID) and References from the latest
  `ThreadID` followed by the latest `ID`, per RFC 5322 §3.6.4, and
  returns the new message's `MessageRef` with `ThreadID` set to
  exactly that new References value. The full chain is thereby
  retained INDUCTIVELY in the single latest reference, for chains of
  any length. When the accumulated chain would exceed header limits,
  the adapter trims middle entries, always keeping the first and the
  most recent — RFC 5322 permits trimming, and first-plus-recent
  preserves the join for both common threaders.

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
   unchanged.)
2. **Transport phase** (`DeliverDueAlerts`, called by the steward
   runner AFTER `RunTick` returns and its deferred arbitration
   release has run): acquire a NEW dedicated sender flock,
   `artifacts/agents/steward/alerts-sender.flock`, with
   LOCK_EX|LOCK_NB. If it is held, return immediately — the live
   holder is the single flight, and the next tick retries. Holding
   it: briefly take the alert lock to read due PENDING attempts and
   stamp each with this sender's identity; release the alert lock;
   perform sends (per-pass budget `channel.max-sends-per-tick`,
   default 3, each 15-second-bounded); re-take the alert lock briefly
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
   number and sender stamp. If it is absent or no longer PENDING,
   REFUSE the completion (journal the refusal as a transport-phase
   defect, matching the shipped "attempt changed before completion"
   error); never create a substitute attempt.
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

Alert content is unchanged from revision 2 (WHAT HAPPENED / WHAT IS
ASKED / the exact ANSWERING ACT — only verbs proven to exist, e.g.
`metasystem goal resume`, `metasystem mission-runner answer`; the
acknowledgment line appended; composer refuses empty fields;
`docs/seat-communication.md` binds the register). The launch gate
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
mechanism) stands as written in revision 2.

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
- A digest batch composes at most `channel.digest.batch-max-bytes`
  of entries and carries their `Spans`. When the destination's
  declared `maxMessageBytes` is smaller, the CHANNEL LAYER (§2's
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

(Folds AC2-SLICE-001 and AC3-SLICE-GATE-CUTOVER-001: slice 1 is at
most 4 hours and independently deployable because it CHANGES NO
LEGACY BEHAVIOR — the legacy queue keeps draining through
`NotifyCommand` and the legacy launch gate stands byte-for-byte; the
new episode path is purely additive. The gate cutover is its own
slice, behind `channel.gate`, strictly after queue retirement.)

1. **Alert path, Telegram, ≤ 4 hours, additive and live-token
   deployable.** The contract with single-chunk `Send` (SendResult
   carrying one outcome; the chunking path is dormant until slice 3),
   destination configuration for the alert class with the
   secret-layer skip, the UNTHREADED Telegram adapter (no
   conversation store yet — alerts do not need threads to reach a
   phone), the §5 journal/transport split with the sender flock and
   the §5a completion merge WITH its concurrent-writer fixture, the
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
   health line plus redaction fixture 0.5h. The round-1 floor
   (fallback, undelivered surfaces, redaction) ships inside the
   slice; enabling a live token at its end is lawful because nothing
   legacy stopped working.
2. **Digest class**: batch composition, the named second consumer
   cursor, Stop-hook cursor-record migration, noticings redirected.
3. **Chunking and spans**: multi-chunk SendResult live, digest span
   accounting, prefix cursor advance.
4. **Conversation reference store and the threaded Slack adapter**
   (Web API), plus reply mapping for Telegram/email/WhatsApp as each
   ships under the §3a invariant; attempt `MessageRef` retention
   lands here with the store.
5. **Queue retirement**: every `QueueNotification` caller migrated;
   `DeliverPending` and the pending directory removed.
6. **Gate cutover**: `channel.gate=channel` becomes available and
   documented as the recommended setting; `EnsureRunner` and `arm`
   consult `Ready` when it is set. Lands only after slice 5, when
   every live producer delivers through the channel and the gate's
   guarantee is true again. Flipping the default to `channel` is a
   separate recorded decision once a deployment has run cut over.
7. **Blocked-on-human producers**: the class-scoped resolution law
   and the §7 producer table, producer by producer.
8. **Remaining adapters** (email, WhatsApp) with provider tests; the
   committed-secret validation rule with its governance record and
   marking-mode activation criterion.
9. **Bridge destinations**: `seat-<id>` outbound; the receive half
   proceeds under seat-mutual-awareness's design per §2b.

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
| AC3-THREAD-ANCESTRY-001 | Folded, §3a: ThreadID defined as adapter-owned threading state under a sufficiency invariant; for email it carries the complete References chain, retained inductively in the latest reference per RFC 5322 §3.6.4, with bounded trimming that keeps first and most recent. |
| AC3-SLICE-GATE-CUTOVER-001 | Folded, §6/§11: slice 1 no longer touches the gate — the legacy check and legacy queue delivery stay byte-for-byte, making the slice purely additive; the cutover is its own slice behind `channel.gate` (default `legacy`), landing only after queue retirement restores the gate's guarantee. |
| Round 3: receive-half reservation; multipart receipts | Judged SOUND by the round-3 critic; unchanged. |

## 13. Self-grade (R-24-m1, refreshed for revision 4)

- **Confidence:** 0.78. The three rounds have converged: eleven
  findings became five, then three, and the round-3 criticals land on
  laws stated precisely enough to test (the §5a merge invariant, the
  §3a sufficiency invariant) rather than on new mechanism; two lines
  are independently judged sound and untouched.
- **Weakest claim:** the §5a refusal branch — when the reloaded
  attempt is absent or no longer PENDING the completion is refused
  and journaled as a defect, but the design does not enumerate every
  path that could legitimately produce that state (a concurrent
  operator repair, a future migration); a refusal that fires on a
  lawful write would strand a real receipt, and only the
  implementation's fixtures will show whether the enumeration is
  complete. Second-weakest: the email trimming rule (keep first and
  most recent) is asserted from common threader behavior, not from a
  normative requirement — RFC 5322 permits trimming but does not
  bless any particular strategy.
- **Reject condition:** reject this revision if the completion-merge
  invariant cannot be held as one critical section on the actual
  episode file layout (for example if a future store shards attempts
  into separate files, reload-merge-save stops being atomic and the
  law needs a version check instead); or if operations requires the
  Ready gate in the FIRST deployable slice, which §6 now forbids
  until queue retirement — that demand reopens the cutover ordering,
  not slice 1; or if any platform in use does not release the sender
  flock on process death, which voids §5's crash law.
