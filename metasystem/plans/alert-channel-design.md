# Alert Channel Design — alert-escalation-channel (revision 3)

Status: revision 3 folds all five round-2 Sol findings
(design-critic-e17645332f616ee62bcc806f, two critical). Nothing is
left unfolded. Two folds change earlier claims rather than add to
them, and say so: the lock split is replaced by ONE implementation (a
kernel-enforced single-flight sender, §5) with its laws argued, and
the receive half of the bridge is EXPLICITLY withdrawn from this
contract and reserved, with its obligations enumerated, for the
seat-mutual-awareness design (§2b) — revision 2's disposition table
claimed that fold complete when it was not, and §12 now records both
rounds honestly.

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
adapter owning storage. Registry: `email`, `slack`, `telegram`,
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
   per completion to journal each `ChunkOutcome` as its attempt
   result (§7). The alert lock is never held across network work; the
   arbitration lock is never held at all in this phase.

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
`docs/seat-communication.md` binds the register). The launch gate:
`EnsureRunner` and `arm` replace the `NotifyCommand` check with
`Ready` over the alert destination chain (primary or fallback
configured passes; no send, no output). Readiness refusal exists ONLY
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

(Folds AC2-SLICE-001: slice 1 is again at most 4 hours AND
independently deployable with the floor, by NARROWING slice 1 —
threading, chunking, and digests move out; gate, floor, and redaction
move in. No live-token precondition outside the slice remains.)

1. **Alert path end to end, Telegram, ≤ 4 hours, live-token
   deployable.** The contract with single-chunk `Send` (SendResult
   carrying one outcome; the chunking path is dormant until slice 3),
   destination configuration for the alert class with the
   secret-layer skip, the UNTHREADED Telegram adapter (no
   conversation store yet — alerts do not need threads to reach a
   phone), the §5 journal/transport split with the sender flock, the
   redaction invariant with its known-bad fixture, the `Ready` launch
   gate swap, and the undelivered count in the health verdict line —
   which §1 shows reaches terminal and Stop hook through existing
   plumbing. Rough arithmetic, stated so it can be challenged:
   contract skeleton and config 1h, Telegram adapter and fixtures 1h,
   sender split 1h, gate + health line + redaction fixture 1h.
   Everything the round-1 floor finding demanded ships INSIDE this
   slice; enabling a live token at its end is lawful.
2. **Digest class**: batch composition, the named second consumer
   cursor, Stop-hook cursor-record migration, noticings redirected.
3. **Chunking and spans**: multi-chunk SendResult live, digest span
   accounting, prefix cursor advance.
4. **Conversation reference store and the threaded Slack adapter**
   (Web API), plus reply mapping for Telegram/email/WhatsApp as each
   ships; attempt `MessageRef` retention lands here with the store.
5. **Queue retirement**: every `QueueNotification` caller migrated;
   `DeliverPending` and the pending directory removed.
6. **Blocked-on-human producers**: the class-scoped resolution law
   and the §7 producer table, producer by producer.
7. **Remaining adapters** (email, WhatsApp) with provider tests; the
   committed-secret validation rule with its governance record and
   marking-mode activation criterion.
8. **Bridge destinations**: `seat-<id>` outbound; the receive half
   proceeds under seat-mutual-awareness's design per §2b.

## 12. Finding dispositions — both rounds, honestly

Round-2 fold-fidelity correction: revision 2's table marked
AC-CONTRACT-001, AC-RECEIPT-001, and AC-SLICE-001 "folded" while the
receive half, per-chunk receipts, and the 4-hour deployable slice
were respectively missing, unrepresentable, and withdrawn. The
corrected record:

| Finding | Disposition |
| --- | --- |
| Round 1 AC-CONTRACT-001 | Partially folded in rev 2 (outbound fields, destinations); COMPLETED in rev 3 by §2's SendResult contract and §2b's explicit, obligation-enumerated receive boundary. The rev-2 table over-claimed. |
| Round 1 AC-BLOCK-001/002 | Rev-2 fold was unsound (missed the arbitration lock); REPLACED by §5's single-flight sender. Gate fold (Ready) stands. |
| Round 1 AC-STATE-001, AC-DEDUP-001, AC-STATE-002, AC-CREDENTIAL-001 | Folded in rev 2; unchanged and standing (§7, §8, §10). |
| Round 1 AC-RECEIPT-001 | Rev-2 fold incomplete; completed by §2/§7/§9 (ChunkOutcome, per-outcome attempts, MessageRef retention). |
| Round 1 AC-SLICE-001 / AC-CONTRACT-002 | Rev-2 fold regressed the 4-hour requirement; repaired by §11's narrowed slice 1; size law completed by §9. |
| AC2-LOCK-001 (critical) | Folded, §5: one implementation, kernel-enforced single flight, laws argued individually; "immediate" honestly redefined as within one sender pass. |
| AC2-CONTRACT-001 (critical) | Folded, §2b: receive explicitly withdrawn from this contract and reserved with five enumerated obligations for seat-mutual-awareness; sender authentication disclaimed; the disposition dishonesty corrected in this table. |
| AC2-THREAD-001 | Folded, §3: channel-layer-owned conversation reference store for ALL adapters, ConversationState into AdapterSend, MessageRef on attempts, Slack moved to the Web API, join-loss on store loss disclosed. |
| AC2-RECEIPT-001 | Folded, §2/§9: channel layer defined concretely; SendResult with per-chunk outcomes and spans; split ownership fixed in the interface. |
| AC2-SLICE-001 | Folded, §11: slice 1 narrowed to a 4-hour, live-token-deployable alert path WITH gate, floor, and redaction inside it. |

## 13. Self-grade (R-24-m1, refreshed for revision 3)

- **Confidence:** 0.72. The two criticals now rest on mechanisms with
  arguable laws (a kernel-enforced single flight; an explicit scope
  boundary with enumerated reserved obligations) rather than on
  adjectives, and the dishonest table is corrected in place.
- **Weakest claim:** the slice-1 arithmetic — four one-hour estimates
  asserted, not measured; the slice is narrowed enough that each part
  is small, but a contract skeleton that later slices must extend
  without call-site changes is exactly where an hour estimate slips.
  Second-weakest: §5's claim that the resident runner plus the
  non-blocking sender flock yields prompt delivery assumes the runner
  reliably runs a transport pass every tick; a wedged runner delays
  alerts by exactly the mechanism meant to announce wedging, and the
  design's answer (the health-line floor still lights on the next
  human-side read) deserves adversarial critique.
- **Reject condition:** reject this revision if Wido intended the
  bridge's RECEIVE half to live inside this contract now — §2b is
  then the wrong boundary and the design reopens at the contract; or
  if slice 1 cannot in fact land inside 4 hours with the floor
  included, which refutes the narrowing and sends the slice plan back;
  or if any platform in use does not release the sender flock on
  process death, which voids §5's crash law.
