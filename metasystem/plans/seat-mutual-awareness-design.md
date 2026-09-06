# seat-mutual-awareness — design: seats see each other and ask each other (revision 1)

Goal: plans/goals/seat-mutual-awareness.md. Wido's order (2026-08-31,
verbatim on the goal record): seats must be aware of each other and
ask each other questions directly, without the human as relay; DONE
means a seat can discover what other seats have in flight and put a
question to them as the normal, mechanized path. His binding word for
the inbound loop (2026-09-01, on the same record) is not touched here:
anything inbound on the external channel carries a single-use TOTP
code, verified against a secret provisioned once at his agent-free
terminal, and a code authorizes only the message it accompanies.
Author m1d (design delegate, job sma-design1-20260906), 2026-09-06,
every cite read at branch agent/sma-design1-20260906 (main 8c86a874).

The shape in one paragraph. Every machine already shares one git
branch that the goal verbs commit to and push, with a transaction
engine that fetches the tip, rebuilds, pushes by compare-and-swap and
lets the push race decide (internal/goal/txn.go:466-516). Every
steward tick already fetches that branch, validates the new tip and
advances a local "accepted" ref (internal/steward/ledgerattention.go:518-544).
This design adds one directory on that branch, `plans/seats/`, holding
two record kinds: a presence record per machine, written by the
steward tick, saying what that machine has in flight; and an ask
record per seat-to-seat question, written by the asking seat and
answered by the addressed seat, each under its own machine's operation
id. Nothing here posts to a provider, nothing here reads the TOTP
secret, nothing here writes a goal file. The seat learns of a question
it owes, and of an answer it waited for, from the health line and the
narrator digest the Stop hook already prints at every turn end.

## 1. What exists and binds (traced)

1. The channel ledger directory `plans/channel/` and its validator are
   landed: record types, the refusal table, the transition matrix and
   the opid helper (internal/goal/channel.go:13-108, 139-215, 775-880,
   979-985); the tree reader and the inbound matcher
   (internal/goal/channel_inbox.go:20-70, 74-121); inbound verification
   and the atomic answer publish (internal/channel/inbox.go:20-73).
   ValidateCommit calls the channel validator on every commit
   (internal/goal/validate.go:485). NOT landed: the posting protocol and
   open-work pass (gateway design step 3b), the verb cut-over (3c) and
   the resident listener with its heartbeat (step 4). `channel ask`
   still writes under the machine's own artifacts directory
   (internal/channel/question.go:78-81, 192), invisible to other
   machines, and `channel wait` loops on that local file
   (cmd/metasystem/channel_verbs.go:179-223).
2. Two landed facts decide where the new records go. The channel
   validator lists `plans/channel/` and refuses any path it does not
   know with `channel-unknown-path` (channel.go:140, 168-171), and
   every record decoder refuses unknown keys
   (channel.go:370-374, DisallowUnknownFields). The channel tree reader
   errors on an unknown path too (channel_inbox.go:44-47). So a new
   record kind or a new key UNDER `plans/channel/` makes every engine
   on the fleet that predates it refuse the tip: its ledger-attention
   pass fails at ValidateCommit (ledgerattention.go:541) and its own
   goal verbs, which validate the commit they build on that tip,
   refuse too. The gateway design avoided exactly this for
   `plans/channel/` by putting it where no old engine looks
   (FCG-INBOX-02). This design does the same: `plans/seats/` is a
   sibling that ReadCommitGoals (validate.go:405-419, per FCG-INBOX-02)
   and ValidateChannelTree (channel.go:140) never list. An engine
   without this goal ignores the directory; an engine with it refuses
   a malformed one; no fleet-wide rebuild is needed before the first
   writer lands.
3. The human answer object cannot honestly carry a seat's answer: its
   `step`, `userId` and provider `ref` are required and non-null
   (channel.go:439-455), and phase `approved` demands a receipt
   (channel.go:543-546). A seat answer written into that shape would
   have to fake a verified TOTP step. That is the confusion Wido's rule
   forbids, so seat asks get their own record and never touch the
   question schema (section 3).
4. An operation id is `<26-char ULID>-<machine>-<8 hex of the lineage
   hash>` (internal/goal/file.go:1494-1497; the goal record's own
   Claimed line shows one: `2BEFGAAS7HAM93R30EKH7WHSXP-m1d-62183579`).
   The machine segment is parseable at rest, which is what lets the
   validator check that a seat record's writer is the machine it
   claims to be (section 5). A seat's identity for a ledger verb is the
   enrolled machine plus the lineage in METASYSTEM_OWNER_LINEAGE
   (channel_verbs.go:21-31).
5. Health: the tick evaluates a fixed role list in a fixed order and
   prints every role with its reason on one line
   (internal/steward/health.go:44-79, 176-193, 267-287); a role is
   alive, dead or unknown (health.go:35-39), a dead role makes the
   aggregate `unhealthy` (health.go:252-255), and an unhealthy
   aggregate opens an alert episode that reaches the human's alert
   channel (internal/steward/alert_episode.go:241-281 keys on the
   aggregate). The narrator digest appends entries deduplicated by
   source type and id (internal/narratordigest/digest.go:105-129), and
   the tick writes them in NarrateDigest (internal/steward/narrate.go:47-95,
   called at tick.go:230). The Stop hook reads the pending digest
   through `steward digest-pending` (cmd/metasystem/steward_verbs.go:180-198).
6. The tick already reads this machine's running delegate jobs and its
   landing receipts for the delivery role
   (internal/steward/delivery.go:85-88), and the job record carries
   `role`, `round`, `parentJob`, `goalId`, `startedAt`, `status`
   (internal/dispatch/record.go:60-77). The engine digest is
   steward.installDigest (internal/steward/identity.go:96).
7. The status post is composed from open channel questions and the
   goal projection (internal/channel/report.go:36-112). Conduct for
   anything a human reads is docs/seat-communication.md; rule 2 (every
   choice carries its consequence, silence has a stated cost) is
   applied between seats here. The core never names a runtime
   (AGENTS.md); every decision below is Go, the verbs are thin.
8. Fences that keep hands off ledger directories: the landing path
   class `install:plans/channel/ ledger`
   (scripts/agents/path-classes.txt:35) and the pre-commit guard's
   regexp over `plans/(goals|channel)/` (per FCG-INBOX-02;
   scripts/agents/pre-commit-guard-fixtures.sh:87-89 proves it on
   `plans/channel/`). Both gain the `seats` alternative in slice 1.
9. Coverage floors: internal/goal 80.0 and internal/steward 74.0
   (scripts/agents/coverage-ratchet.json), enforced by
   scripts/agents/coverage-delta.sh. New code lands in those two
   packages and in cmd/metasystem so it stands under a floor.

## 2. Discovery: the presence record and `seat fleet`

Decision: (a), a durable per-machine record, because goal claims
cannot see a chain, a landing or an engine, and (b) would leave "what
is m2 doing right now" exactly where it is today (ask Wido). One
deviation from the brief's list: the record does NOT carry the
claimed goals. Claims are already on the same tip in every goal file's
`Claimed:` line (delivery.go:63 reads them) and a copy written at tick
time would only ever be equal or stale. The read verb joins claims
from the goal files at the tip it reads.

Record: `plans/seats/presence/<machine>.json`, `<machine>` the value
goal.ResolveMachine returns (internal/goal/actor.go:21), canonical
serialisation MarshalChannel (channel.go:111-117), keys in this order,
no unknown keys.

| key | type | required | meaning |
|---|---|---|---|
| machine | string | yes | equals the basename |
| lineage | string | yes | the lineage of the session that armed this checkout's steward; "" when none is readable (see the seam note below) |
| engine | string | yes | the engine digest (steward.installDigest) the tick runs |
| chain | object or null | yes | the newest non-terminal delegate job: `{root, job, role, round, goal, startedAt}`; `root` is reached by following `parentJob` until absent; null when no job is running |
| lastLanding | object or null | yes | `{commit, goal, at}` from the newest landing receipt in memory/receipts.log (delivery.go:85 reads it); null when none |
| tickAt | RFC 3339 UTC, second precision | yes | the tick that composed this version |
| updatedAt | RFC 3339 UTC | yes | equals tickAt at write; kept separate so a later rule may refresh without recomposing |
| opid | string | yes | the opid of the commit that wrote this version; its machine segment equals `machine` |

Writer and cadence. The steward tick composes the record after its
ledger-attention pass (tick.go:178-186, which has just fetched and
validated the tip) and publishes it through goal.Publish under a fresh
opid from ChannelOpid (channel.go:979-985), Intent verb `seat-presence`,
Targets the record path, Args {machine, engine}, message
`seat presence <machine>`, Validate ValidateCommit. Its Mutate reads
the machine's own record at the rebuilt tip and: writes when the path
is absent; writes when the body digest (every key but tickAt,
updatedAt, opid) differs from the tip's; writes when the tip's
updatedAt is older than `seat.presence-min` (default 30 minutes); else
returns NothingToDo (txn.go:462) and the tick logs nothing. It refuses
`seat-transition` when the tip's updatedAt is younger than the one it
read (a second tick of the same machine landed first, the heartbeat
rule of FCG-INBOX-02). Idle cost at the default: one commit per
machine per 30 minutes. The publish is best-effort by contract: its
outcome is recorded as a component attempt named `seat-presence`
(beginComponentAttempt, tick.go:126) and a failure never degrades the
tick; a pushed-blocking refusal (txn.go:512-516) is logged as
`presence blocked by <opid>; run goal recover` and shows in the
health role's reason (section 5). No seat conduct is involved: a
machine whose steward is armed is visible; one whose steward is not
armed writes nothing and is reported silent, which is the truth.

Staleness. A record whose updatedAt is older than twice
`seat.presence-min` (60 minutes by default) is `silent since
<updatedAt>`; a machine with no record is `never seen`. A machine's
own record is read like any other's.

Validator rows (ValidateSeatTree, section 5): `seat-unknown-path`,
`seat-json`, `seat-id-mismatch` (basename versus machine),
`seat-writer` (opid machine segment versus machine), `seat-json` for
every time field not RFC 3339 UTC at second precision
(parseChannelTime, channel.go:650-656 is reused).

The read verb: `metasystem seat fleet [--root <checkout>] [--no-fetch]`.
It captures the tip with the same bounded fetch the projection uses
(internal/goal/project.go:44, 117-123; `--no-fetch` reads the accepted
ref), reads the seat tree and the goal tree at that tip, and prints one
block per machine, machines sorted by name, every machine that has a
presence record OR a claim OR an open ask in either direction:

```
m2   alive (presence 4 min ago, engine 9a1c0f2e)   session main-1788683763-71870-f7f607
     claimed: channel-poll-refuses-legacy-budget-questions — next: <first line of the goal's next step>
     chain: implementer round 2 under root fcg-build3 for fleet-channel-gateway, started 41 min ago
     last landing: 8c86a874 for seat-mutual-awareness, 2 h ago
     owes 1 seat question (from m1d about seat-mutual-awareness, 12 min); waiting on 0
m3   silent since 2026-09-06T08:10:00Z (presence 2 h old, engine 7b2d11aa)
     claimed: none
paper never seen (no presence record: steward not armed or engine predates seat awareness)
```

"owes" counts open asks addressed to the machine, "waiting on" open
asks it sent. The next-step first line is the goal file's `Next step:`
text cut at its first sentence, 120 characters. This verb is what a
seat runs before it asks; it is read-only and needs no lineage.

Seam note (not traced): which lineage the resident steward runner
holds. The lease announcement carries `--owner-lineage`
(scripts/agents/channel-fixtures.sh:52-54 shows the flag), but I did
not trace the runner reading it back. The implementer reads the
checkout's live lease record for the lineage; if the seam does not
exist, `lineage` is "" and the verb prints `session unknown`. This is
a gap to stop on if the record has no such field.

## 3. Asking a seat: the ask record and `seat ask`

A seat-to-seat question is its own record kind, not a channel question
with a machine destination (section 1, items 2 and 3). Record:
`plans/seats/asks/<qid>.json`, `<qid>` a fresh 26-character ULID from
goal.NewOperationULID (question.go:184 mints one the same way).

| key | type | required | meaning |
|---|---|---|---|
| id | string | yes | equals the basename |
| from | string | yes | the asking machine; equals the opid's machine segment |
| fromLineage | string | yes | the asking lineage |
| to | string | yes | the addressed machine; never equals `from` |
| goal | string | yes | the goal the question concerns; a goal file must exist on the tip (channelGoalIDs, channel.go:221-241, is reused) |
| facts | []string | yes, one to four entries | what the asker knows, one sentence each |
| question | string | yes, non-empty | the question in plain words |
| ifSilent | string | yes, non-empty | what the asker will do if nobody answers, and when (seat-communication rule 2) |
| askedAt | RFC 3339 UTC | yes | |
| opid | string | yes | the opid of the ask commit |
| state | string | yes | open, answered, closed |
| answer | object or null | yes | `{text, by, lineage, at, opid}`; non-null exactly when state is answered; `by` equals `to`; the answer opid's machine segment equals `by` |
| closedAt, closedBy, closedBecause, closedReason | strings, closed only | | closedBecause is `not-mine` (closed by `to`) or `withdrawn` (closed by `from`); closedBy is that machine; closedReason is the `--because` text |

There is no `kind`, `wants`, `budget`, `options`, `destination`,
`thread`, `step`, `userId` or `ref` key, and an unknown key is refused
(`seat-json`). A seat therefore cannot express a stop, a budget or any
token-bearing question to another seat, because there is nothing a
seat could approve. The ask touches no goal file: the goal's next step
is not marked (goal.Asked, verbs.go:43-65, is for human questions
only), so no narrator line about the goal changes and no other
machine's claimed goal is edited from outside.

Verb: `metasystem seat ask --to <machine> --goal <id> --fact <text>
[--fact ...] --question <text> --if-silent <text> [--root <checkout>]`,
printing the qid. Identity is channelIdentity (channel_verbs.go:21-31).
One ledger transaction under the asker's own opid: Intent verb
`seat-ask`, Targets the record path, Args {to, goal}, message
`seat ask <to> about <goal>`, Validate ValidateCommit. Mutate refuses,
by name and before any write: `seat-self` when `--to` is this machine;
`seat-unknown-machine` when `--to` has neither a presence record nor a
claim at the tip ("seat fleet lists the machines the ledger knows");
`seat-goal-missing`; `seat-duplicate` when an open ask from this
machine to that machine on that goal with the same facts digest is at
the tip (the dedup rule of Ask, question.go:174-183, printing the
existing qid). A path already present is the ordinary AlreadyApplied
or LostToCompetitor classification (channel.go:905-927 is the model).
No provider is posted; a machine without a bot token asks exactly as
one with.

## 4. Answering: `seat answer`, `seat close`, `seat wait`

`metasystem seat answer --question <qid> --text <text>` or
`--not-mine --because <text>`. One transaction under the answering
machine's own opid, Intent verb `seat-answer`, Targets the record
path, Args {qid, outcome}. Mutate at every rebuilt tip:

| transition (owner) | FROM | TO |
|---|---|---|
| answer (the machine named in `to`) | state open | state answered; answer {text, by: me, lineage: mine, at: now, opid: this commit} |
| not-mine (the machine named in `to`) | state open | state closed; closedBecause not-mine; closedBy me; closedReason the `--because` text |
| withdraw (`seat close`, the machine named in `from`) | state open | state closed; closedBecause withdrawn; closedReason the `--because` text |

Refusals by name: `seat-not-addressee` when `to` is not this machine
(for answer and not-mine); `seat-not-asker` when `from` is not this
machine (for close); `seat-late` when the state at the tip is not
open, printing the standing answer or the close reason and exiting 3.
`seat-late` writes nothing: a late answer binds nothing and leaves no
record, which is the pattern of FCG-MATCH-06 minus the inbox row a
human message needs for its own audit. A second answer from the same
machine is the same case: the first answer stands, and an amendment is
a new ask in the other direction. `seat-not-a-seat-question` when the
id names no ask record; when the id names a channel question at the
tip (plans/channel/questions/<id>.json) the message says "<id> is a
question to the human; seats do not answer those". AlreadyApplied and
LostToCompetitor are classified as in ClassifyChannelTransition
(channel.go:905-927): the tuple is `(state, answer.opid or null)`.

The one sentence the validator enforces: a seat answer carries no
human authority, because it is written only into `plans/seats/` and
into no goal file, and every authority consumer reads goal history
(RecordedNormApproval for `goal claim --approved-ref`;
AuthenticatedChannelApproval, verbs.go:67-93, for resume and
set-obligation), where a seat opid never appears. `goal approve`,
`set-budget`, `resume` and enrollment stay at the enrolled terminal or
behind a verified channel answer, untouched. Authentication of a seat
answer is the committing machine's identity in the opid, exactly as a
goal claim is authenticated today. The residual, plainly: any process
that can push to the ledger branch as machine X can answer as X, and
can forge an opid naming X; the ledger already trusts that for claims,
and nothing here widens what such a process could do, since a seat
answer authorizes nothing.

`metasystem seat wait --question <qid> [--timeout <minutes>]
[--interval <seconds>, default 30]` fetches the tip every interval
(the bounded capture of project.go:117-123), reads the ask, and
returns: exit 0 printing `answer.text` when answered; exit 2 printing
`not-mine: <reason>` when closed not-mine; exit 2 printing
`withdrawn: <reason>` when withdrawn; exit 1 `seat wait timed out`
past the timeout. It blocks a script, never a turn, and the asker's
`ifSilent` text is the turn's own plan for the timeout case.
`channel wait --question <id>` (channel_verbs.go:179-223) gains one
lookup: when no local question file exists and the id names a seat
ask at the accepted ref, it runs `seat wait` with the same flags, so a
script that waits on any question id keeps working.

## 5. Surfacing without polling: the tick, the health role, the digest

The tick gains one read after its ledger-attention pass: RunSeatAttention
(new, internal/steward/seats.go) reads the seat tree at the accepted
ref (goal.AcceptedLedgerTip, internal/goal/attention.go:32-45) and
returns {owed: open asks addressed to me, oldest first; resolved: asks
from me whose state is answered or closed; presence: the publish
outcome of section 2}. TickResult carries it, NarrateDigest consumes
it, and the health role reads the same tree.

Health role `seat-questions` (RoleSeatQuestions), placed after
`ledger-attention` in healthRoleOrder (health.go:63-79), evaluated in
evaluateHealthRoles (health.go:267-287) by reading the accepted ref
itself, so PreviewHealthAt from the Stop hook (steward_verbs.go:60)
sees the last tick's view without a fetch:

- alive, reason `owes no seat question` when no open ask is addressed
  to this machine, or every such ask is younger than `seat.answer-min`
  (default 30 minutes);
- alive, reason `owes <n> seat question(s): <from> asks about <goal>,
  <age> min; remedy: metasystem seat answer --question <qid>` (the
  oldest named, the rest counted) while the oldest is between
  `seat.answer-min` and `seat.answer-dead-min` (default 180 minutes);
- dead, NoAutomaticRemedy true (the retro-debt pattern,
  health.go:359-374), same reason text prefixed `UNANSWERED SEAT
  QUESTION`, once the oldest passes `seat.answer-dead-min`;
- unknown when the accepted ref or the seat tree is unreadable, or
  when the last presence publish was refused (reason names the opid
  and `goal recover`).

Why two ages and not the brief's one: a dead role makes the aggregate
unhealthy and an unhealthy aggregate opens an alert episode to the
human (alert_episode.go:256; section 1 item 5), which would make the
human the relay for every slow seat, the thing the goal exists to end.
The 30-minute key therefore governs what the seat sees (the line names
the debt at every turn end from that age), and the 180-minute key
governs when the fault becomes the human's business as a fault report,
not as a question. Wido may collapse them to one by setting both keys
equal. Keys are read through internal/config like the steward's other
keys (delivery.go:90 shows the idiom), from metasystem.conf with
metasystem.conf.local overriding.

Narrator digest lines (narrate.go:47-95 pattern, deduplicated by
source marker so each fires once, digest.go:105-129):

- new question owed: kind lowlight, source type `seat-ask`, source id
  the qid: `Seat <from> asks this machine about <goal in words>:
  "<question, 160 chars>". If unanswered it will <ifSilent, 120 chars>.
  Answer with: metasystem seat answer --question <qid>.`
- answer received: kind highlight, source type `seat-answer`, source
  id the qid: `Seat <to> answered this machine's question about <goal
  in words>: "<text, 160 chars>".`
- closed not-mine or withdrawn: kind lowlight, source type
  `seat-answer`, source id the qid: `Seat <by> closed this machine's
  question about <goal in words> as <not theirs|withdrawn>: "<reason>".`

Both directions therefore arrive at the next turn end on each side
with no hand polling: the target's line and digest name the question,
the asker's digest names the answer, and `seat wait` is for a script
that wants to block. The human sees none of this through the tick.
The status post (report.go:36-112) gains one line, only when the count
is non-zero, after the "Needs you" lines: `Owes <n> question(s) from
other seats, oldest <m> min`. No question text reaches the post.

## 6. The human channel stays what it is

Stated, then enforced:

1. A seat record is never a channel record. ValidateChannelTree lists
   only `plans/channel/` (channel.go:140) and ReadChannelTree likewise
   (channel_inbox.go:26); `plans/seats/` is never decoded as a
   ChannelQuestion, ChannelInbound or ChannelListener. The inbound
   matcher iterates tree.Questions only (channel_inbox.go:75-77), so no
   inbound record can name a seat ask, and rule (b) needs a posted
   thread (channel_inbox.go:104), which a seat ask has no key for.
2. No provider ever posts a seat record. Today's poster reads local
   channel questions (question.go:195-209); the future open-work pass
   reads `plans/channel/questions/` (FCG-POST-08). Neither has a code
   path into `plans/seats/`, and the seat verbs call no Provider
   method.
3. The provider poll never confirms or matches anything a seat wrote.
   Verify (inbox.go:20-43) is reached only through InboundRecord
   (inbox.go:45) on a provider Inbound; no seat verb constructs an
   Inbound, reads `channel.human.totp-secret`, or calls VerifyTOTP.
   Wido's inbound rule (code checked at the message's own send time,
   single use by step on the ledger, code bound to its message) is
   the gateway design's FCG-COMMIT-05 and is not edited here.
4. A seat's words never carry a code. `seat-secret` refuses ANY field
   of six ASCII digits in facts, question, ifSilent, answer.text or
   closedReason (channelCodeField, channel.go:671-682, reused), a
   stricter rule than the channel's last-field rule, because seats
   never legitimately carry one.

ValidateSeatTree (new, internal/goal/seats.go), called from
ValidateCommit next to the channel validator (validate.go:485),
refuses by name; a refusal abandons the publishing transaction as a
goal refusal does:

| code | condition |
|---|---|
| seat-unknown-path | a file under plans/seats/ that is not `presence/<machine>.json` or `asks/<ulid>.json` |
| seat-json | not a JSON object, an unknown key, a missing required key, a wrong type, null where non-null is required, a time not RFC 3339 UTC at second precision |
| seat-id-mismatch | basename does not equal `machine` or `id` |
| seat-writer | `opid`'s machine segment does not equal `machine` (presence) or `from` (ask); `answer.opid`'s machine segment does not equal `answer.by`; `answer.by` does not equal `to`; closedBy is not `to` for not-mine or `from` for withdrawn |
| seat-self | `from` equals `to` |
| seat-goal-missing | `goal` has no goal file on the tip |
| seat-state | state answered with answer null; state open or closed with answer non-null; state closed without closedAt, closedBy, closedBecause, closedReason; closedBecause outside {not-mine, withdrawn}; facts empty or more than four; question or ifSilent empty |
| seat-secret | a six-digit field anywhere in facts, question, ifSilent, answer.text or closedReason |

Fences: `install:plans/seats/ ledger` beside path-classes.txt:35, so a
landing that touches it refuses with `ledger-path-not-goal-verb`; the
pre-commit guard's regexp widens to `plans/(goals|channel|seats)/`.

Paths I can see and close: a seat answer reaching an authority consumer
(closed: no goal history row, section 4); a seat ask being posted or
matched as a human question (closed: items 1 to 3); a code smuggled in
seat text (closed: item 4); a record forged under another machine's
name by a well-formed commit (closed at the writer, and at rest by
`seat-writer` for the opid field; NOT closed against a process that
forges the opid itself, which is the claims residual of section 4).
Paths I cannot close: a seat quoting a peer's answer into a human
question's facts or into a chat turn, which then stands as that seat's
own words under docs/seat-communication.md, a conduct matter; a seat
acting on a peer's answer by running a goal verb it was already
entitled to run, which is the seat's own act; and a hand-written
malformed seat record committed without a trailer, which poisons the
accepted ref on every machine exactly as a malformed goal file does
today (ledgerattention.go:537-543), with the same fallback: a hand
commit at the terminal removing it. No skip verb is built.

## 7. Fixtures and tests

Script `scripts/agents/seat-fixtures.sh`, on the channel fixture bed
(bare origin, export clone, fake provider serving;
channel-fixtures.sh:21-54) plus a second clone `export-b` enrolled as
`fixture-b` with its own lineage; machine A is `fixture-machine`. Every
assertion reads origin/main, a verb's stdout and exit code, the health
line, or `steward digest-pending`.

- SMA-F-PRESENCE: A runs `steward tick`; origin/main holds
  `plans/seats/presence/fixture-machine.json` with A's engine digest
  and the tick's opid; B's `seat fleet` prints A `alive`, A's claimed
  goal with its next step's first line, and B `never seen`. A second
  tick within seat.presence-min with nothing changed adds no commit.
- SMA-F-ASK-ANSWER: A `seat ask --to fixture-b`; B's tick with
  `seat.answer-min=0` prints `seat-questions=alive (owes 1 seat
  question: fixture-machine asks about <goal>, ...)`, and B's pending
  digest names the ask once across two ticks; B `seat answer`; A's
  `seat wait` prints the text and exits 0; A's tick digest names the
  answer once.
- SMA-F-LATE: B answers again; exit 3 with `late`; the record's answer
  opid is unchanged on origin/main.
- SMA-F-NOT-MINE: A asks; B `--not-mine`; A's wait prints `not-mine:`
  and exits 2; A's digest names the close.
- SMA-F-WRONG-MACHINE: A `seat answer` on an ask addressed to B is
  refused `seat-not-addressee` and origin/main is unchanged; then a
  plain git commit on the ledger branch (no trailer, the
  FCG-12-POISON-SKIP staging) writes a presence record for `fixture-b`
  under an opid naming `fixture-machine`; B's next ledger-attention
  reports failed naming `seat-writer` and B's accepted ref did not
  advance.
- SMA-F-HUMAN-QUESTION-REFUSED: a valid channel question record is
  hand-committed on the ledger (the shape channel_test.go:151 stages);
  `seat answer --question <that id>` is refused with "is a question to
  the human"; a seat ask hand-committed with a `wants` key is refused
  `seat-json` by B's validator.
- SMA-F-QUIET: two presence commits by A that change no goal file;
  B's ledger-attention report has no pending event and B's digest
  gains no "shared goal ledger moved" entry. This is the design's
  weakest claim, so it is a fixture and not a sentence.
- SMA-F-STATUS-CLAUSE: B's `channel status` shows `Owes 1 question(s)
  from other seats` only while the ask is open.

Go unit tests, respecting the floors of section 1 item 9:
internal/goal/seats_test.go: TestValidateSeatTreeRefusesEachRow (a
table with one hand-built tree per code above),
TestSeatPresenceMutateWritesOnChangeOrAge,
TestSeatPresenceRefusesYoungerTip, TestSeatAskMutateRefusals (self,
unknown machine, goal missing, duplicate), TestSeatAnswerTransitions
(answer, not-mine, withdraw, late, not-addressee, not-asker,
already-applied, lost-to-competitor), TestSeatOpidMachineSegment,
TestSeatSecretRefusesSixDigitsAnywhere.
internal/steward/seats_test.go: TestSeatQuestionsRoleThreeAges,
TestSeatDigestEntriesFireOnce, TestPresenceComposesChainFromJobRecords
(root through parentJob, null when none), TestPresencePublishFailureNeverDegradesTick.
internal/channel/report_test.go: TestStatusReportOwedClauseOnlyWhenOwed.
cmd/metasystem: TestChannelWaitDelegatesToSeatWait.

## 8. Build order and the box

Three slices, each landing alone with its own gate.

1. Records, validator, fences, reader, transactions as a library, no
   writer and no verb: internal/goal/seats.go (types, ValidateSeatTree,
   ReadSeatTree, the three request builders), the ValidateCommit call,
   the path-classes row, the guard regexp. Gate: `go test ./internal/goal`
   under the coverage delta, path-class and pre-commit-guard fixtures
   green. One attempt, 35 to 45 minutes. Old engines are unaffected
   (section 1 item 2), so no fleet action precedes slice 2.
2. The presence writer in the tick, `seat fleet`, `seat ask`, `seat
   answer`, `seat close`, `seat wait`, the `channel wait` delegation,
   and the fixtures SMA-F-PRESENCE through SMA-F-QUIET. Gate: the
   fixture script and the steward unit tests green. Two attempts, 60
   to 90 minutes.
3. The health role, the digest lines, the status clause,
   SMA-F-STATUS-CLAUSE and the role fixture inside SMA-F-ASK-ANSWER.
   Gate: health fixtures and the seat fixtures green. One to two
   attempts, 40 to 60 minutes.

One closing code review after slice 2 and one after slice 3, each its
own critique chain with the box's three rounds. Estimate: four to five
build attempts, 135 to 195 build minutes, plus two reviews at 30 to 45
minutes each, 195 to 285 job-minutes in all, inside one day only if
nothing is retried. The box (one day, six attempts, 240 job-minutes,
three review rounds) holds slices 1 and 2 with their review. Slice 3
with its review will most likely need a raise of one day, two
attempts and 120 job-minutes; the seat asks Wido for it when slice 2
has landed and the remaining minutes are known, not before.

## 9. Non-goals

No new provider and no second bot. No change to the TOTP rule, to
FCG-COMMIT-05 or to any channel record. No authority for seats: a seat
answer approves, budgets, resumes and enrolls nothing. No archive or
rotation of seat records (answer-archive owns harvest and rotation;
`plans/seats/asks/` only grows here, at under one kilobyte per ask).
No central brain: every machine reads the same branch and decides for
itself. No skip verb for a poisoned seat record. No mark on the goal
file for a seat ask.

## 10. Self-grade

Confidence: medium-high that the shape is right and buildable in the
existing engine, because every write is a goal.Publish with a Mutate
of the kind already landed for the channel, and every read is a tip
the tick already fetches. Weakest claim: that a presence commit which
changes no goal file produces no ledger-attention event on other
machines (snapshotLedger reads Ready, Pinned and Queue from the goal
projection, ledgerattention.go:488-490, so I expect none), proven or
disproven by SMA-F-QUIET; if disproven, presence moves from the tick
to the Stop hook's turn end, keeping the same record. Second weakest:
the lineage the runner holds (section 2 seam note). Reject condition:
Wido wants the records under `plans/channel/`, which turns slice 1
into a fleet-wide rebuild-before-writer action; or the tick must never
push, which moves the writer as above; or the human must be alerted at
30 minutes, which collapses the two ages to one.
