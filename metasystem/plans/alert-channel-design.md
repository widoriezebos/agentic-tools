# Alert Channel Design — alert-escalation-channel

Design for the promoted goal `plans/goals/alert-escalation-channel.md`:
escalations and blocked-on-human states reach Wido IMMEDIATELY over an
external channel, so he is notified the moment machinery lawfully needs
his judgment — instead of escalations terminating in a git-landed log he
must poll. Driving specimen: `records/misc/idle-loss-2026-08-31.md` —
three hours lost, nine stalled-idle escalations written into
`records/narrator-digest.log`, none delivered, the human himself was the
monitoring system.

Wido's design requirement, verbatim, binding:

> "it needs to have an abstraction/adapter. I want to be able to have
> email, slack, telegram, whatsapp etc underneath by simple
> configuration."

Design only. No code ships with this document.

## 1. What exists today (traced facts)

The machinery already has most of the truth layer; what is missing is
the transport.

- **The alert episode store is real and durable.**
  `internal/steward/alert_episode.go` keeps one JSON file per episode
  under `artifacts/agents/steward/alerts/`, flock-serialized, with a
  full lifecycle: `OpenedAt`, an `Attempts` journal (each attempt has a
  sequence, timestamps, a `TransportResult` of
  `PENDING`/`TRANSPORT_SUBMITTED`/`TRANSPORT_FAILED`, and a problem
  string), `Acknowledged`/`AcknowledgedBy`, `Resolved`, `Cleared`.
  Cleared episodes remain evidence; a recurrence opens a new id.
  `UpdateAlertEpisodes` already dedups on a finding digest (one open
  episode per finding), reuses a crash-interrupted pending attempt
  (at-least-once, never a second invented submission), and calls the
  transport only at the alert boundary.
- **The transport is one hardcoded seam.** `internal/steward/notify.go`
  `Deliver(repoRoot, message)` runs the command in the repository-local
  git configuration key `metasystem.steward.notify-command`, or on
  macOS falls back to an `osascript` desktop notification; 15-second
  timeout; zero exit means submitted. This is exactly the seam the
  adapter layer replaces from the inside — every caller of `Deliver`
  keeps its contract.
- **A durable retry queue exists.** `DeliverPending` retries queued
  notifications each steward tick; the first failure stops the pass
  ("the channel is down — one named failure beats a burst of them").
- **The narrator is a second, lower-urgency stream already.**
  `internal/steward/narrate.go`: the scrolling `narration.log` account,
  the durable `narratordigest.Append` register of highlights/lowlights,
  and `ReachTheHuman` queueing "noticing" lines (one pending message
  per building condition). Today the digest lands in git and reaches
  nobody — the specimen's exact failure.
- **Acknowledgment exists.** `metasystem health acknowledge-alert
  --episode <id>` (`cmd/metasystem/steward_verbs.go`) records the
  observed invoker identity against the episode without clearing it.
  Ruling G's agent-free-terminal enforcement is a named follow-up (L8)
  in that code; this design does not change it.
- **The configuration idiom is settled.** `internal/config/resolve.go`:
  one key resolves flag > mechanically derived environment variable >
  uncommitted `metasystem.conf.local` > committed `metasystem.conf` >
  explicit default. `.local` is the documented home for values that
  must not ship.

## 2. The transport abstraction

### 2.1 The adapter contract

One interface, deliberately minimal — send one message of a given
class, report submission or typed failure, nothing more:

```
type Channel interface {
    // Send submits one composed message. nil means the transport
    // accepted it (submission, not proof a person saw it — the
    // episode store already names this honestly as
    // TRANSPORT_SUBMITTED). A typed error distinguishes
    // UNCONFIGURED (no adapter or no credentials) from FAILED
    // (adapter tried and the transport said no).
    Send(class MessageClass, msg Message) error
}

type MessageClass string // "alert" | "digest"

type Message struct {
    Happened string // what happened, plain words, first
    Asked    string // what is asked of the human
    Answer   string // the exact command or act that answers it
    EpisodeID string // empty for digests
}
```

Adapters hold NO state, do NO retries, keep NO queue: retry cadence,
attempt journaling, and dedup stay where they already live (the episode
store and the steward tick). An adapter that cannot complete within the
existing 15-second notify timeout is a failed attempt, never a wedged
tick.

The adapters ship in the engine as a closed registry keyed by name:
`email`, `slack`, `telegram`, `whatsapp`, `command`, `desktop`, `none`.
`command` wraps an operator-supplied shell command (message in
`STEWARD_MESSAGE`, class in `STEWARD_CLASS`) so ANY transport can be
scripted with zero engine changes; `desktop` is today's macOS
`osascript` path, kept as the local floor. Adding a new NAMED adapter
is one registry entry (code, once, in the engine); ENABLING and
CONFIGURING any shipped adapter is configuration alone. Call sites call
`Send` and never name an adapter — zero code changes at call sites when
the configured channel changes, which is the requirement's letter.

### 2.2 Configuration key shape

Class-scoped keys, in the `metasystem.conf` family and idiom (lowercase
dotted, resolved flag > env > `.local` > committed):

```
channel.alert.adapter=telegram
channel.alert.telegram.chat-id=<id>
channel.alert.telegram.bot-token=<SECRET — .local or environment only>
channel.alert.fallback-adapter=desktop

channel.digest.adapter=telegram
channel.digest.telegram.chat-id=<a DIFFERENT id — see §3>
channel.digest.telegram.bot-token=<SECRET>
channel.digest.batch-minutes=240
```

The general shape: `channel.<class>.adapter` selects; every adapter
setting is `channel.<class>.<adapter>.<setting>`. Each class resolves
its own complete adapter configuration, so the same adapter under both
classes with different destinations is two distinct channel identities
— the goal text's "distinct channel instance or distinct high-urgency
identity" falls out of the key shape instead of needing a mechanism.
Per-adapter non-secret settings for the named targets: `email` takes
`to`, `from`, `smtp-host`, `smtp-port`; `slack` takes nothing beyond
its secret webhook URL; `telegram` takes `chat-id`; `whatsapp` takes
`to` and `phone-number-id` (Cloud API). Exact per-adapter setting lists
are frozen at each adapter's slice, not here.

The legacy git-config key `metasystem.steward.notify-command` remains
honored as the `command` adapter's source when
`channel.alert.adapter` is unset — existing installations keep working
untouched; the migration is one config edit, never a flag day.

### 2.3 Credentials

Secret-bearing keys (`bot-token`, `webhook-url`, `smtp-password`,
`api-token`) live ONLY in `metasystem.conf.local` or the environment —
the two layers the resolve order already places above the committed
file and the documentation already names for values that must not
ship. Nothing new is invented for secrecy; the idiom is reused.

Enforcement follows the law-becomes-software route
(`docs/paper/12-learning-systems.md`, `plans/goals-drafts/
law-becomes-software.md`): a validation rule refuses a committed
`metasystem.conf` (or any committed file) carrying a
`channel.*.*.<secret-name>` key with a non-placeholder value. The rule
lands as a feature with its governance record — owner, review date,
appeal route, a known-bad fixture that must keep failing — and runs in
marking mode before it refuses, per the chapter's discipline. It is a
slice below, not prose law.

When credentials are absent the adapter returns the typed
UNCONFIGURED error. The episode records the attempt as
`TRANSPORT_FAILED` with problem "unconfigured", the health surface
says so (§6), and machinery NEVER blocks on it: an unconfigured
channel degrades to the fallback and the floor, it does not stop a
tick, a dispatch, or a goal.

## 3. Two message classes, one channel design

**Alerts** are immediate, unmissable, one per actionable state. They
ride the episode store's existing law: one open episode per finding
digest, submission at the alert boundary, at-least-once across crash
gaps, delivery receipt on the attempt journal. An alert is sent the
moment its episode reaches the alert boundary — never batched, never
queued behind digests (the two classes have separate send paths by
construction; a digest batch in flight shares nothing with an alert
send but the adapter code).

**Digests** are batched narrative: every `channel.digest.batch-minutes`
(default 240) the steward tick composes one message from the narrator
digest entries appended since the last delivered batch and sends it on
the digest channel. The narrator digest register remains the truth; the
transport keeps only a delivery cursor (the timestamp of the last
entry included in a submitted batch) in
`artifacts/agents/steward/digest-cursor.json`. The cursor is
bookkeeping, not a second state: it is rebuildable, and losing it
costs at worst one repeated batch — at-least-once, matching the alert
law. An empty window sends nothing.

Alerts must never drown in narrative. The mechanism is identity, not
priority flags: the recommended configuration points the two classes
at different destinations (a Telegram chat that pings loudly for
alerts; a muted chat, or email, for digests). The design does not trust
configuration alone, though — even when both classes share one
destination, alerts are sent individually and immediately with a fixed
`ALERT:` lead, and digests are one message per window with a `digest:`
lead, so an alert is never inside, behind, or summarized into a batch.

## 4. Alert content

Every alert carries three parts, in this order, composed from the
`Message` fields: WHAT HAPPENED (first sentence, plain words, no
identifiers — `docs/seat-communication.md` Rule 3 governs every channel
that reaches the human, so it governs this one), WHAT IS ASKED, and THE
EXACT ANSWERING ACT — the command to run or the act to take, verbatim,
e.g.:

```
ALERT: a seat is stopped and waiting for your word — work is idle
until you answer.
Asked: approve or reject the claim of goal <plain goal title>.
Answer with: metasystem goal approve <id>   (or: ... reject <id>)
Acknowledge receipt: metasystem health acknowledge-alert --episode <id>
```

The acknowledgment line is appended to every alert automatically. The
composer refuses an alert whose `Happened`, `Asked`, or `Answer` field
is empty — a producer that cannot name the answering act is not ready
to interrupt a human, and the refusal surfaces at the producer, at
build time of that producer, not as a silent drop. Identifiers ride as
attachments to plain names (Rule 1); numbers wear units (Rule 4).
Digest lines are already written in the narrator's plain-English
register and pass through unchanged.

## 5. Source of truth and the producers

The alert episode store is authoritative (the goal's L4). The channel
is transport: it holds no message state, no dedup memory, no
acknowledgment record. Delivery receipts land where they already land —
`AlertAttempt` entries on the episode, extended with one additive field
`Channel string` naming the adapter that took the attempt (JSON schema
stays 1; the field is optional). Acknowledgment stays on the existing
`health acknowledge-alert` seam, unchanged.

### Alert classes carried

Two families, one store:

1. **Ruling L escalation classes** — ended auto-heal, no lawful
   remedy, flapping. These already flow: the steward tick's `ActNotify`
   decisions and health verdicts reach `UpdateAlertEpisodes`, which
   calls `Deliver` at the alert boundary. Swapping `Deliver`'s
   internals for the adapter layer carries this family with zero
   producer changes.
2. **Blocked-on-human states**, each a first-class alert: a claim
   awaiting approval, a stop awaiting resume, a decision-ask with no
   human at the terminal, an enrollment drift awaiting re-arm
   (`ENROLLMENT_DRIFT` from `internal/up` and
   `internal/steward/identity.go`). These get ONE new producer seam:

   ```
   RaiseHumanBlockedAlert(repoRoot, class, subjectID, msg Message)
   ```

   which opens or refreshes an episode keyed on a stable digest of
   `(class, subjectID)` — one actionable state, one episode, one
   alert; the state clearing (claim decided, stop resumed, ask
   answered, enrollment re-armed) resolves the episode through the
   same store law that health verdicts use today. The dedup key law is
   fixed here; the per-producer wiring (exactly which line in the
   claim path, the stop path, the up verb calls the seam) is traced at
   its slice, because those call sites live outside the files traced
   for this design and the gap rule forbids inventing them.

The idle-loss conduct rule rides this: a decision-ask to an absent
human is prose plus an alert through the seam, never a turn-blocking
dialog — the ask travels with its exact answering act, which is what
requirement 3's content contract exists for.

## 6. Failure honesty

What happens when the channel is down, stated in full:

- **A send fails**: the attempt is journaled `TRANSPORT_FAILED` with
  the adapter's problem string; the episode stays at the alert
  boundary and the next steward tick retries — the existing
  `DeliverPending` cadence and its first-failure-stops-the-pass rule
  are kept.
- **The fallback fires**: when the primary adapter fails or is
  unconfigured, the same send is attempted once on
  `channel.alert.fallback-adapter` (default `desktop` — the
  phone/desktop path the goal names; on a headless host the operator
  configures `command` or `none`). The fallback attempt is journaled
  with its own `Channel` name, so the receipt says WHICH transport
  claimed submission.
- **The floor**: an episode whose latest attempt is failed on both
  primary and fallback is an UNDELIVERED alert, and it must not become
  its own unread log. It surfaces on the surfaces a human or seat
  already touches: `metasystem health` output gains one line — "N
  alert(s) undelivered, oldest M minutes" — and the runtime Stop-hook
  surface (which already carries census and stale-supervisor lines)
  carries the same line, so ANY live seat's turn end shows it and the
  seat's standing duty is to say it to the human in its next message.
  This is the stop-message floor: the last hop is the seat's own
  mouth, which is exactly the hop that worked on 2026-08-31 at 17:15.
- **Digest sends fail** the same way minus the episode store: the
  cursor does not advance, the next window retries the whole span, and
  the health line counts undelivered digest windows separately (lower
  urgency, same honesty).

Delivery is never a gate on machinery: no tick, launch, dispatch, or
goal transition waits on a send. The one existing delivery-gated
behavior (notify.go's launch-gate comment and install-time refusal on
non-darwin hosts with no configured command) is preserved for its
current callers and NOT extended to any new alert class.

## 7. Slice plan

Independently deployable, in order; each lands through the normal
design-critique/implementation/critique loop.

1. **The abstraction plus the first adapter — Telegram (≤ 4 hours).**
   The `Channel` interface, the registry, the class-scoped key shape,
   the typed UNCONFIGURED/FAILED errors, `Deliver`'s internals rerouted
   through the alert-class channel with `desktop`/legacy-command as
   fallback, the `Channel` field on attempts, the acknowledgment line
   in the composer. Telegram ships first because its credential story
   is the simplest of the four named targets: one bot token from
   BotFather and one chat id — no OAuth flow, no workspace app
   approval, no business-API onboarding, one HTTPS POST to
   `api.telegram.org`. This slice alone kills the specimen: Ruling L
   escalations reach a phone.
2. **Digest class.** Batch composition from the narrator digest
   register, the delivery cursor, `channel.digest.*` keys, the
   `noticing` queue redirected through the digest channel (noticings
   are narrative, not actionable states).
3. **Blocked-on-human producers.** The `RaiseHumanBlockedAlert` seam
   and its dedup law, then per-producer wiring with the call-site
   tracing done in that slice's design: claim-awaiting-approval,
   stop-awaiting-resume, decision-ask, `ENROLLMENT_DRIFT`.
4. **Remaining adapters** — email (SMTP), Slack (webhook), WhatsApp
   (Cloud API) — each one registry entry behind the same contract;
   enabling any of them is configuration only, which is the verbatim
   requirement made checkable: the slice's acceptance test enables
   each by config against a fake endpoint with zero call-site diffs.
5. **The committed-secret refusal rule**, with its governance record
   and known-bad fixture, entering in marking mode per the
   learning-systems chapter.

## 8. Self-grade (R-24-m1)

- **Confidence:** 0.75. The truth layer, transport seam, retry law,
  acknowledgment seam, and configuration idiom are all traced to file
  and line and the design only rearranges the one hardcoded seam; the
  key shape and class split follow existing idiom rather than
  inventing one.
- **Weakest claim:** that the blocked-on-human states can each be
  reduced to a stable `(class, subjectID)` dedup key at their real
  call sites — those call sites were not traced for this design, and
  a state that lacks a stable subject id (a free-form decision-ask,
  say) would need an id minted at the ask, which slice 3's design
  must settle. Second-weakest: that Telegram is the simplest
  credential story on Wido's actual devices; if he does not use
  Telegram, slice 1's adapter choice should flip to whichever of the
  four he answers with — the abstraction is unchanged either way.
- **Reject condition:** reject this design if delivery must GATE
  machinery for any new alert class (this design makes delivery
  never-blocking by law, and gating would invert §6), or if Wido
  wants alerts and digests on structurally different channel designs
  rather than one contract with two class identities — the merged
  goal text says one design, and this document is built on that
  merge.
