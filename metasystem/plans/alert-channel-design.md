# Alert Channel Design — alert-escalation-channel (revision 2)

Status: revision 2 folds the full Sol critique
(design-critic-2ea477763ee73bd1dbc0ddec, eleven findings, one
critical) and Wido's three new words of 2026-08-31: Telegram is
CONFIRMED first, the contract bears the session bridge as a second
consumer, and thread identity is a contract concern. Every finding is
folded (accepted and designed in); none is refuted; the per-finding
disposition table is §12. No findings remain unfolded.

Design for the promoted goal `plans/goals/alert-escalation-channel.md`:
escalations and blocked-on-human states reach Wido IMMEDIATELY over an
external channel, so he is notified the moment machinery lawfully needs
his judgment — instead of escalations terminating in a git-landed log he
must poll. Driving specimen: `records/misc/idle-loss-2026-08-31.md` —
three hours lost, nine escalations written, none delivered.

Wido's design requirement, verbatim, binding:

> "it needs to have an abstraction/adapter. I want to be able to have
> email, slack, telegram, whatsapp etc underneath by simple
> configuration."

And his two additions of 2026-08-31 (verbatim): "We can use the same
mechanism for the session bridge too, so there is a bit of reuse
there" and "Another one would be slack, which has threaded messages.
that also needs to fit the design of the alert channel and session
bridge."

Design only. No code ships with this document.

## 1. What exists today (traced facts)

- **The alert episode store** (`internal/steward/alert_episode.go`):
  one JSON file per episode under `artifacts/agents/steward/alerts/`,
  flock-serialized, with an attempts journal
  (PENDING/TRANSPORT_SUBMITTED/TRANSPORT_FAILED), digest-keyed dedup,
  crash-safe pending recovery, `AcknowledgeAlert`. Two laws in it are
  health-specific and must NOT generalize (§7): a new finding resolves
  every non-matching open episode, and one Deliver call happens INSIDE
  the exclusive alert lock.
- **The transport seam** (`internal/steward/notify.go`): `Deliver`
  runs the git-config command `metasystem.steward.notify-command` or
  macOS `osascript`, synchronously, 15-second timeout, and embeds raw
  command output in its returned error.
- **A second durable delivery state exists**: the pending-notification
  queue (`internal/steward/intervene.go`,
  `artifacts/agents/steward/pending/`), fed by revival failures
  (`runner.go`), reap notices (`reap.go`), tick messages, and narrator
  noticings; `DeliverPending` retries it each tick and deletes entries
  on success — delivered messages leave no receipt.
- **The launch gate**: `EnsureRunner` and `arm`
  (`internal/steward/runner.go`) refuse when `NotifyCommand` reports
  no channel — an AVAILABILITY check, not a send.
- **The narrator digest register** (`internal/narratordigest/`)
  already has a byte-offset plus prefix-hash cursor, and the runtime
  Stop hook (`scripts/agents/supervision-hook.sh`) is its existing
  consumer: it reads the register from the cursor and advances it
  after showing the human the payload.
- **The tick** (`internal/steward/tick.go`) calls
  `UpdateAlertEpisodes` synchronously before completing.
- **Acknowledgment**: `metasystem health acknowledge-alert --episode
  <id>` exists and is kept unchanged.
- **Real human answering verbs** (verified in
  `cmd/metasystem/main.go`): `metasystem goal resume` (human-only
  restart after a stop batch) and `metasystem mission-runner answer`
  (record a human answer to an open ask). No `goal approve` or `goal
  reject` verb exists; revision 1's example was wrong and is replaced
  (§6).
- **The configuration idiom** (`internal/config/resolve.go`): flag >
  derived environment variable > uncommitted `metasystem.conf.local` >
  committed `metasystem.conf` > default.

## 2. The channel contract — two consumers, one abstraction

(Folds AC-CONTRACT-001, critical, together with Wido's words 2–3.)

The contract serves TWO consumers from day one:

1. **The human alert channel** (this goal): alerts and digests to
   Wido.
2. **The session bridge** (goal seat-mutual-awareness): addressed,
   bidirectional, runtime-agnostic seat-to-seat messages.

Revision 1's `Send(class, msg)` with one fixed destination per class
could not carry the second consumer without leaking adapters into call
sites; the contract itself changes:

```
// A Destination is a NAMED, configuration-resolved place messages go
// (a Telegram chat, a Slack channel, a mailbox, a seat's inbox).
// Callers name destinations, never adapters.
type DestinationName string

type Message struct {
    Class          MessageClass // "alert" | "digest" | "bridge"
    Sender         string       // asserted origin identity, e.g. "steward@mac-m3", "seat:m2"
    ConversationID string       // stable correlation key; threads live here (§3)
    InReplyTo      string       // prior MessageRef.ID this answers, or empty
    Deadline       time.Time    // zero, or when an unanswered ask escalates
    // Human-alert content (empty for bridge messages, which carry Body):
    Happened, Asked, Answer string
    Body           string       // pre-composed text (digest batches, bridge payloads)
    EpisodeID      string       // empty unless an alert episode backs it
}

type MessageRef struct {
    ID       string // transport-assigned handle (Telegram message_id, Slack ts)
    ThreadID string // transport thread identity when the adapter threads
}

type Channel interface {
    // Send submits one message to one destination. A returned
    // MessageRef enables replies and threading. Errors are TYPED:
    // ErrUnconfigured (no adapter/credentials) vs ErrSendFailed
    // (transport said no), and are SANITIZED (§10) before return.
    Send(dest DestinationName, msg Message) (MessageRef, error)

    // Ready reports, WITHOUT network side effects, whether a
    // destination resolves to a fully configured adapter (adapter
    // named, required settings and credentials present). This is the
    // launch gate's operation (§5); it never sends.
    Ready(dest DestinationName) (bool, string)

    // Capabilities reports the resolved adapter's declared facts for
    // a destination: threads (bool), receive (bool),
    // maxMessageBytes (int). Callers adapt composition, never
    // adapter choice.
    Capabilities(dest DestinationName) AdapterCapabilities
}
```

Adapters implement `Send`/`Ready`/`Capabilities` plus, when they
declare `receive`, a `Receive` poll operation returning inbound
`Message`s with their `MessageRef` and `ConversationID` mapped back
from the transport's thread identity. The RECEIVE/REPLY LOOP — who
polls, how a seat answers, response commitments, deadlines-as-conduct
— is the session bridge's own design under seat-mutual-awareness;
what THIS design fixes is the shared contract those loops ride:
addressed destinations, sender identity, conversation identity,
message references, deadlines, typed sanitized errors, and declared
capabilities. The bridge is a caller, not a fork of the mechanism.

Adapters still hold NO state, do NO retries, keep NO queue; a send
that cannot complete within the 15-second timeout is a failed attempt.
The registry ships `email`, `slack`, `telegram`, `whatsapp`,
`command`, `desktop`, `none`. Adding a NAMED adapter is engine code,
once, in the registry; enabling and configuring any SHIPPED adapter is
configuration alone, and call sites never name adapters — the
requirement's letter, now scoped honestly (see AC-CONTRACT-002 fold,
§9 and §12).

## 3. Threading law

(Folds Wido's word 3.)

`ConversationID` is the caller's stable correlation key: for an alert
episode it IS the episode id — the alert, its updates, and its
acknowledgment echo are one conversation; for a bridge exchange it is
the exchange id minted by the asking seat. The mapping to transport
threads is the ADAPTER's job, invisible to callers:

- A THREADED adapter (Slack) keeps a small conversation map
  (ConversationID → thread ts) in its per-destination scratch under
  `artifacts/agents/channel/<destination>/threads.json` — a derived
  CACHE, rebuildable by starting a fresh thread, never a truth store:
  losing it degrades threading, never content.
- A REPLY-CAPABLE flat adapter (Telegram) uses reply-to-message-id
  against the conversation's first MessageRef where it has one.
- A FLAT adapter (email subject tagging aside, SMS-like transports,
  `command`, `desktop`) degrades HONESTLY: the composed text carries a
  short bracketed conversation tag (`[re: alert-ab12…-1]`) so a human
  can correlate; nothing else changes.

No per-adapter thread behavior appears at any call site: callers set
`ConversationID` and nothing else.

## 4. Configuration key shape

Destinations are first-class; classes and the bridge point at them:

```
channel.destination.wido-urgent.adapter=telegram
channel.destination.wido-urgent.telegram.chat-id=<id>
channel.destination.wido-urgent.telegram.bot-token=<SECRET — never committed, §10>
channel.destination.wido-quiet.adapter=telegram
channel.destination.wido-quiet.telegram.chat-id=<a DIFFERENT chat>
channel.destination.seat-m2.adapter=slack
channel.destination.seat-m2.slack.webhook-url=<SECRET>

channel.alert.destination=wido-urgent
channel.alert.fallback-destination=local-desktop
channel.digest.destination=wido-quiet
channel.digest.batch-minutes=240
channel.digest.batch-max-bytes=3500
```

General shape: `channel.destination.<name>.adapter` selects;
`channel.destination.<name>.<adapter>.<setting>` configures;
`channel.<class>.destination` binds a message class. Distinct
destinations give alerts and digests distinct identities (alerts never
drown in narrative); the bridge addresses `seat-<id>` destinations by
name. The legacy git-config key `metasystem.steward.notify-command`
remains honored as an implicit `command`-adapter destination named
`local-command` when no alert destination is configured — existing
installations keep working; migration is one config edit.

Per-adapter non-secret settings are frozen AT EACH ADAPTER'S SLICE
with that slice's provider-constraint tests (§11); this document fixes
only the shape and the Telegram set (`chat-id`; secret `bot-token`).

## 5. Sending discipline and the launch gate

(Folds AC-BLOCK-001 and AC-BLOCK-002.)

Revision 1's "never-blocking law" was false as stated: the shipped
path sends INSIDE the exclusive alert lock, and the tick waits on it.
The honest, redesigned law is BOUNDED BLOCKING, OUTSIDE THE LOCK:

- `UpdateAlertEpisodes` splits into journal and transport phases. The
  lock covers journaling only: the pending attempt is written, the
  lock is RELEASED, the send runs with no lock held, the lock is
  re-acquired to journal completion. `AcknowledgeAlert` and other lock
  contenders therefore never wait on the network; the crash-gap law is
  unchanged (a pending attempt found at recovery is the same
  at-least-once reuse as today).
- The tick's delivery phase runs after its decision work, bounded by a
  per-tick send budget (`channel.max-sends-per-tick`, default 3, each
  bounded by the 15-second timeout); remaining pending sends wait for
  the next tick. A tick can be DELAYED by at most budget × timeout of
  transport time; it is never gated on delivery OUTCOME, and no goal
  transition, dispatch, or decision waits on a send. The design says
  "bounded", not "never", and means it.

The LAUNCH GATE is representable now: `EnsureRunner` and `arm` replace
their `NotifyCommand` availability check with `Ready` on the
alert-class destination chain — the gate passes when the primary OR
the fallback destination is fully configured (any adapter: Telegram on
Linux passes without the legacy command; macOS passes on `desktop`).
`Ready` performs no send and emits nothing. The gate's meaning is
preserved exactly — "an unreachable watchdog guards nothing" — and the
policy split is now explicit: ONLY the launch gate consults readiness
as a refusal; every other consumer treats unconfigured as a typed,
non-blocking degradation (§8). No new alert class ever gates.

## 6. Alert content

Every alert carries WHAT HAPPENED (first sentence, plain words —
`docs/seat-communication.md` binds every human-facing channel), WHAT
IS ASKED, and THE EXACT ANSWERING ACT, with the acknowledgment line
appended automatically:

```
ALERT: work is stopped and waiting for your word.
Asked: restart the stopped goal (its budget stop completed).
Answer with: metasystem goal resume --goal <id> ...
Acknowledge receipt: metasystem health acknowledge-alert --episode <id>
```

(Folds part of AC-DEDUP-001.) Revision 1's example named a
non-existent `goal approve` verb. Corrected law: the `Answer` field
names a REAL act — the traced verbs are `metasystem goal resume` (a
stop awaiting resume) and `metasystem mission-runner answer` (an open
ask); each producer slice supplies its own verb and its acceptance
test proves the named verb exists in `metasystem help` output. The
composer refuses an alert with an empty `Happened`, `Asked`, or
`Answer` — refusal surfaces at the producer, never as a silent drop.

## 7. One truth layer: episodes, receipts, dedup

**The episode store is the ONLY durable delivery state for the alert
class.** (Folds AC-STATE-001.) The pending-notification queue is
RETIRED, not preserved: every current `QueueNotification` caller
(revival failures, reap notices, tick messages, narrator noticings)
migrates — actionable ones become episodes; narrative noticings become
digest-register entries and ride the digest class. `DeliverPending`
and the pending directory are removed at the migration slice, with the
same one-time compatibility drain the store already performs for
legacy health notifications. After that slice there is exactly one
owner of "was this delivered": episode attempt journals (alerts) and
per-consumer digest cursors (digests, §8).

**Receipts represent every transport try.** (Folds AC-RECEIPT-001.)
Each try — primary or fallback — is its OWN `AlertAttempt` with its
own `Channel` (destination name), result, problem, and monotonically
increasing sequence. Specified semantics revision 1 left open: a
primary failure followed by fallback success is TWO attempts (failed
+ submitted); episode-level `TransportResult` is SUBMITTED when any
attempt submitted, and the episode records `SubmittedVia` (the
successful destination). Pending-recovery narrows: a PENDING attempt
found at recovery is reused only when its `Channel` matches the
destination about to be tried; otherwise it is completed as FAILED
("interrupted") and a new attempt opens. Retries stop at SUBMITTED.

**Dedup and resolution are class-scoped.** (Folds AC-DEDUP-001.) The
current update law — a new finding resolves every non-matching open
episode — is HEALTH-CLASS ONLY (correct there: one health verdict at a
time) and does not apply to blocked-on-human episodes, which coexist
and resolve each by its OWN clear event. The producer table, with
traced identities:

| Blocked state | Subject identity (traced) | Clear event |
| --- | --- | --- |
| Stop awaiting resume | stop id, `internal/goal/stop.go` | the human `goal resume` for that stop |
| Open mission ask | ask id, `internal/missionrunner/answer.go` | the recorded `mission-runner answer` |
| Decision-ask (free-form) | ask id MINTED AT COMPOSITION; the composer refuses an unidentified ask | the producer's recorded answer against that id |
| Enrollment drift awaiting re-arm | the enrolled-engine identity (`ENROLLMENT_DRIFT` itself is an ephemeral result) | the next successful enrollment verification, observed by the steward tick |
| Claim awaiting approval | its approval object's id, traced AT ITS SLICE — no verb exists today (§1), so this producer lands only with the mechanism it alerts on | that mechanism's recorded decision |

Episode key: stable digest of (class, subject identity). One
actionable state, one episode, one alert; recurrence after clearing
opens a new episode id, per the store's existing evidence law.

## 8. Two message classes, digests, and the cursor law

Alerts send individually and immediately (within §5's bounds), with a
fixed `ALERT:` lead; digests are one batched message per window with a
`digest:` lead; the two classes share nothing but the contract, so an
alert is never inside or behind a batch.

(Folds AC-STATE-002.) Revision 1's new timestamp cursor is DROPPED. The
narrator digest register already owns a byte-offset plus prefix-hash
cursor with the Stop hook as its consumer; the external digest channel
becomes a SECOND NAMED CONSUMER of the SAME mechanism: the register
grows per-consumer cursor records (`stop-hook`, `external-digest`),
each with the existing offset+hash law, and no consumer deletes or
consumes entries destructively — each reads forward from its own
cursor. "Delivered" has one definition per consumer: that consumer's
cursor covers the entry. The external cursor advances only after the
transport accepted the batch; a crash between send and advance repeats
one bounded batch. Conceded honestly: this cursor is irreducible
delivery state — the register cannot say what an external transport
accepted — so it is named AS state, owned by the register (not the
adapter), and bounded: composition takes at most
`channel.digest.batch-max-bytes` per window (§9), and a LOST cursor
re-sends at most the retained register once, flagged in the batch
header ("cursor was rebuilt; entries may repeat"). The Stop hook's
existing behavior is untouched except for the cursor file gaining a
consumer name (its migration is part of the digest slice).

## 9. Message size law

(Folds AC-CONTRACT-002.) Telegram caps a message at 4096 characters;
the digest window was unbounded. The law: COMPOSITION is bounded, and
the channel layer chunks.

- Alerts are bounded at composition (`channel.alert.max-bytes`,
  default 1500); an over-cap alert is truncated with a tail naming the
  episode id — the full text lives in the episode, which is the truth.
- A digest batch composes at most `channel.digest.batch-max-bytes`
  (default 3500, under the smallest known provider cap) of entries;
  remaining entries simply wait — the consumer cursor advances only
  over entries actually included in an ACCEPTED send, so partial
  windows are ordinary, not partial failures.
- When a composed message still exceeds the destination's declared
  `maxMessageBytes`, the channel layer splits it into sequential
  chunks in the same conversation; each chunk is its own send with its
  own receipt, and for digests the cursor advances per accepted
  chunk's entry span. Multipart partial success is therefore
  representable: accepted chunks are delivered and covered, the failed
  chunk retries next window.

The "configuration-only" claim is scoped honestly: enabling a SHIPPED
adapter is configuration-only at call sites; SHIPPING an adapter is
engine code, and each adapter slice carries provider-constraint tests
(size cap, secret redaction, thread mapping) against recorded provider
behavior — a fake endpoint proves call-site neutrality, not provider
correctness, and the slice plan says which test proves which (§11).

## 10. Credentials and failure honesty

Secrets (`bot-token`, `webhook-url`, `smtp-password`, `api-token`)
resolve ONLY from the environment or `metasystem.conf.local`:
(mechanical guard, folds part of AC-CREDENTIAL-001) the channel
layer's resolution of a secret-named setting SKIPS the committed file
— a secret committed anyway is ignored and the destination reports
unconfigured with the reason "secret found in committed configuration".
The committed-secret validation rule (§11 slice 1b) adds repository
hygiene on top, with its law-becomes-software governance record.

**Sanitized errors are a contract invariant.** (Folds
AC-CREDENTIAL-001.) Telegram puts the bot token in the request URL and
Slack's webhook URL is itself the secret, and the current notifier
already embeds raw output in errors. Therefore: an adapter must
redact every secret it was configured with from any error text it
returns (`<redacted:bot-token>`), and the channel layer additionally
literal-scrubs all resolved secret values from every problem string as
defense in depth, BEFORE anything is journaled, logged, or printed.
The known-bad fixture — an error string carrying a live-shaped token
must come out scrubbed — ships with slice 1 and must keep failing
unscrubbed.

**Failure floor** (unchanged in substance, now with an owner, §11): a
failed send journals TRANSPORT_FAILED and retries next tick; the
fallback destination is tried as its own journaled attempt; an episode
failed on both is an UNDELIVERED alert surfaced on surfaces already
touched — one `metasystem health` line ("N alert(s) undelivered,
oldest M minutes") and the same line in the Stop-hook payload, making
the seat's own next message the last hop. Undelivered digest windows
count separately, lower urgency. Delivery outcome never gates
machinery; the ONLY readiness refusal is the launch gate (§5).

## 11. Slice plan

(Folds AC-SLICE-001 and the slice half of AC-CONTRACT-002.) Each slice
independently deployable; Telegram first is CONFIRMED by Wido.

1. **Contract, Telegram, gate, floor, redaction (the specimen-killer;
   ≤ 4 hours is no longer claimed for all of it — 1a is the 4-hour
   cut).**
   - 1a: the `Channel` contract (Send/Ready/Capabilities, typed
     sanitized errors), destination configuration, the Telegram
     adapter with its size cap and redaction fixtures, alert sends
     rerouted outside the lock (§5 journal/transport split),
     per-try attempt receipts, `desktop`/legacy-command fallback.
   - 1b (same slice, required before 1a is ENABLED on a live token):
     the launch gate moved to `Ready`, the health and Stop-hook
     undelivered lines with their acceptance tests (health output
     asserts the count line; the hook fixture asserts the payload
     line), the committed-layer secret skip, and the committed-secret
     validation rule entering in MARKING mode with its governance
     record — owner: this goal's owner seat; activation criterion:
     refusal power after 14 days of marking with zero false marks, by
     the owner's recorded decision.
2. **Digest class**: batch composition with the size law, the named
   second consumer cursor on the existing register mechanism, Stop
   hook cursor migration, noticings redirected into the register.
3. **Queue retirement**: every `QueueNotification` caller migrated to
   episodes or register entries; `DeliverPending` and the pending
   directory removed; compatibility drain.
4. **Blocked-on-human producers**: the class-scoped resolution law in
   the store, then the §7 producer table wired producer by producer,
   each with its real answering verb proven to exist; the
   claim-approval producer waits for its mechanism.
5. **Remaining adapters** — Slack (threaded; the thread map), email,
   WhatsApp — each one registry entry plus that provider's constraint
   tests; enabling each is configuration-only at call sites.
6. **Bridge destinations**: `seat-<id>` destinations and the
   `receive` capability, consumed by seat-mutual-awareness's own
   design for the ask/answer loop.

## 12. Finding dispositions (Sol round 1)

| Finding | Disposition |
| --- | --- |
| AC-CONTRACT-001 (critical) | Folded, §2–§4: addressed destinations, sender/conversation/reply/deadline fields, MessageRef, capabilities, receive; the bridge is a contract consumer. |
| AC-BLOCK-001 | Folded, §5: never-blocking restated as bounded-blocking; send moves outside the alert lock; per-tick send budget. |
| AC-BLOCK-002 | Folded, §5: `Ready` (non-side-effecting) replaces the NotifyCommand check; gate passes on primary-or-fallback configured; only the gate refuses. |
| AC-STATE-001 | Folded, §7/§11-3: pending queue retired, all callers migrated, one delivery-state owner. |
| AC-RECEIPT-001 | Folded, §7: one attempt per try, `SubmittedVia`, narrowed pending-recovery, episode-level result law specified. |
| AC-DEDUP-001 | Folded, §6/§7: class-scoped resolution, traced producer identities, minted ask ids, real verbs only (bogus example removed). |
| AC-STATE-002 | Folded, §8: new cursor dropped; per-consumer cursors on the existing register mechanism; cursor conceded as named, bounded delivery state. |
| AC-CREDENTIAL-001 | Folded, §10: adapter redaction invariant plus channel-layer scrub, known-bad fixture in slice 1; committed-layer secret skip. |
| AC-SLICE-001 | Folded, §11: gate, floor surfaces, and redaction own slice 1b with acceptance tests; secret rule has owner and activation criterion. |
| AC-CONTRACT-002 | Folded, §9: size law, chunking with per-chunk receipts, digest bound, configuration-only claim rescoped, per-adapter provider tests. |
| Telegram-first (undisputed) | Confirmed by Wido; kept. |

## 13. Self-grade (R-24-m1, refreshed for revision 2)

- **Confidence:** 0.7. The contract now carries both confirmed
  consumers and the control-flow, state, and receipt laws are stated
  against traced code rather than asserted; the drop from 0.75
  reflects that revision 2 widens scope (bridge fields, receive
  capability, queue retirement) on one uncritiqued pass.
- **Weakest claim:** that the journal/transport split of §5 preserves
  the store's crash-gap at-least-once law under concurrent writers —
  releasing the lock between the pending write and the completion
  write introduces an interleaving revision 1 did not have, and the
  narrowed pending-recovery rule (§7) is designed but not yet
  adversarially traced. Second-weakest: the Slack thread map as a
  rebuildable cache assumes losing a thread mid-conversation is
  acceptable degradation for both consumers; the bridge's design may
  find it is not.
- **Reject condition:** reject this revision if the session bridge's
  own design needs SYNCHRONOUS receive or delivery guarantees stronger
  than at-least-once-with-receipts — that would make the shared
  contract the wrong home and the reuse Wido named would need to live
  at the adapter registry instead; or if the queue retirement (§7)
  turns out to break a `DeliverPending` caller whose message is
  neither actionable (episode) nor narrative (register), a third kind
  this design says does not exist.
