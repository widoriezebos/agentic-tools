# Fleet Conversation Channel — design (goal fleet-slack-channel, revision 3)

Author m0b (Fable lane), 2026-09-03. Tier 3 under R-54-m1; revision 2
folded Sol round 1 (FSC-R1-001..008), revision 3 carries the closing
review's five obligations (FSC-R2-001..005) as build law; §10. Wido's
words (verbatim on the goal record): status "per machine", "threaded
conversations about questions from machines", "switchable between
providers (Slack, Telegram, Whatsapp)" with Slack first, his reply
authenticated by a TOTP code or equivalent; DONE = status posted,
question answered from his phone, answer on the ledger as his word,
machine continues, proven against a fake endpoint.

## 1. What exists and is reused (traced)

1. plans/alert-channel-design.md (revision 13) DECIDED the outbound contract
   and this design adopts those sections as law: §2 `Message`/`MessageRef`,
   §2a the stateless adapter and the Slack facts (Web API `chat.postMessage`
   with a bot token, never a webhook: only `ts` threads), §3/§3a the
   conversation store artifacts/agents/channel/<destination>/conversations.json
   and Slack `ThreadID` = root `ts`, §4 the `channel.destination.<name>.*`
   idiom, §10 credentials (secrets only from environment or
   metasystem.conf.local; literal scrubbing from every problem string); §2b
   reserved the INBOUND half, which this design builds for one provider.
2. No `internal/channel` package exists on main; this feature builds it
   (alert §9 chunking and §5 single-flight are deferred).
3. internal/humanauthority proves an enrolled agent-free terminal
   (`HUMAN_AUTHORITY_PROVEN`) or, under R-32-m1 until 2026-09-06, a relayed
   word for exactly `goal resume` and `goal set-obligation`
   (`TEMPORARY_HUMAN_WORD`). `RecordedNormApproval` already accepts a
   `human:` history operation whose reason carries the strict token.
4. Report sources, all durable on main: the goal ledger (plans/goals),
   the landing history (origin/main, `Goal-Item:` trailers, subjects), job
   records (artifacts/agents/jobs; internal/report scanners), usage facts
   (internal/usage per job; cost is null on main).
5. The steward tick is the fleet's one scheduled observation per machine;
   both its drivers will carry the channel phase (§7).

## 2. The provider contract, fixed once

Package `internal/channel`. One interface, every provider behind it,
selected by configuration (§6). Operations, exactly three:

```
type Provider interface {
    // outbound (alert design §2a shape: one submission, one ref, typed error)
    Post(ctx, dest DestinationConfig, text string, thread *MessageRef) (MessageRef, error)
    // inbound (the §2b reserved half): destination-wide, polled, never a listener.
    // threads = the caller's open thread roots (Slack needs them; Telegram ignores them).
    Receive(ctx, dest DestinationConfig, threads []MessageRef, after Cursor) ([]Inbound, Cursor, error)
    // credential readiness: the token's own identity, never the human's (FSC-R1-008)
    Credential(ctx, dest DestinationConfig) (CredentialIdentity, error)
}
type Inbound struct { Ref MessageRef; ThreadID string; UserID string; Text string; At time.Time }
type Cursor string // provider-opaque checkpoint; ONE per destination, persisted by the caller
```

`Post` with `thread == nil` starts a thread and returns its root ref;
with a thread it posts into it. Close is not a provider operation: it is
a question-record state (§4) plus one final `Post`; providers keep no
state (alert §2a). Inbound checkpoint law (alert §2b, FSC-R1-004):
`Receive` returns envelopes in provider order with thread correlation and
a new cursor; the caller persists that cursor ONLY after every envelope's
disposition is durable (§5), and that persisted cursor IS the
acknowledgment — no separate acknowledge operation, because re-delivery
after a crash is absorbed by §5's per-ref idempotence. Typed sanitized
errors (`ErrUnconfigured`, `ErrSendFailed`, `ErrReceiveFailed`) are the
alert design's. Registry for this goal: `slack`, `fake`; `telegram`
(getUpdates, one owned offset as cursor) and `whatsapp` are later goals.

**Slack adapter** (`internal/channel/slack`): `Post` = `chat.postMessage`
(`channel`, `text`, `thread_ts` when threaded); `Receive` = for each
given thread root, `conversations.replies` (`channel`, `ts` = root,
`oldest` = that thread's last ts from the cursor, `limit` 200, paged by
`response_metadata.next_cursor` until exhausted), keeping only `ts >`
the thread's last ts and skipping the bot's own user (Slack's `oldest` is
inclusive and returns the root first); the cursor encodes the map root →
last ts. `Credential` = `auth.test`. Base URL is `slack.api-base`
(default https://slack.com/api); the fixture points it at the fake.

**Fake provider** (`internal/channel/fake`): an in-process HTTP server
speaking exactly the three Slack methods above with Slack's JSON shapes
(`ok`, `ts`, `messages[]`, `error`), a scripted reply queue the fixture
appends to, and a request journal the tests read. The Slack adapter talks
to it through `slack.api-base`, so the fake proves the adapter's bytes;
the `fake` adapter name starts that server in-process for the fixture (§8).

## 3. The per-machine status report

Composed by `internal/channel/report.go` from §1.4 sources only, never
from a session's memory; one message per machine, no thread, plain words,
feature names not job ids (R-61-m1). Sections omitted when empty:

```
<machine> status <YYYY-MM-DD HH:MM>Z
Landed since <window start>: <feature> — <commit subject> (<n> landings)
Under way: <feature> — <goal next-step first sentence>; job <role> running <m> min
Planned: <feature> (queued, ready|needs budget|blocked by <feature>)
Spend today: <n> jobs; <runtime>: <units> ... ← usage units, never dollars
Undelivered: <n> channel messages, oldest <m> min   ← only when non-zero
```

`<feature>` = goal id as words (hyphens to spaces) + the intent's first
sentence cut at 120 characters. "Landed since" = origin/main commits with
a `Goal-Item:` trailer naming a goal this machine claims or claimed, since
the previous post (state artifacts/agents/channel/status.json: last post
time, content digest, ref). "Under way" = goals claimed by this machine
plus any running delegate job under them. "Planned" = queued goals pinned
to or last claimed by this machine, readiness from the ledger (human
budget stored or not; blocked-by). "Spend today" = internal/usage facts
(tokens or provider units) for jobs started today UTC, by runtime; NO
dollars — every cost field internal/usage writes on main is null
(FSC-R1-005); dollars come with a priced source in a later goal.
Cadence: the tick posts when `channel.status.interval-minutes` (default
240) have passed AND the digest changed; `--post` posts now. Size: 12
lines per section, 3500 bytes total, "(+n more)"; no chunking.

## 4. The question thread contract

A question is a durable record `artifacts/agents/channel/questions/<qid>.json`
(qid = ULID), written before any network:

```
{ "id", "goal", "kind": "budget-above-norm|fork|reserved-decision|stop|other",
  "machine", "openedAt", "facts": [..], "options": [{"label","consequence"}],
  "recommendation", "wants": "<strict token the answer must carry, or empty>",
  "thread": MessageRef|null, "state": "open|answered|closed", "undelivered": n,
  "answer": {"text","userID","ref","at","step","opid","phase"}|null, "rejected": [..] }
```
(The destination cursor lives in artifacts/agents/channel/<destination>/cursor.json.)

Opened by `metasystem channel ask --goal <id> --kind <k> --fact ... --option
"label: consequence" ... --recommend <label> [--wants "<token>"]`: writes
the record, posts the thread root (goal in words, kind, facts, options
with consequences, recommendation, and the reply form "reply in this
thread with your answer followed by your code"), then appends `ASKED <qid>
(<kind>): <first fact>` to the goal's next step. One thread per question:
a second `ask` with the same (goal, kind, facts digest) while one is open
returns the existing qid and posts nothing. Undelivered: `thread: null`,
the tick retries the root post each pass, the count rides the health line
(alert §10 floor). Asks never block a turn. `--wants` fixes the ledger
token the answer must carry: for `budget-above-norm` the strict
`goal=<id> minutes=<n> goalRevision=<r>`; for `stop` the resume tuple.

## 5. The reply path: authentication and the recording as his word

`channel.Poll` runs under ONE channel lock (flock on
artifacts/agents/channel/lock; a second poll, manual or tick, returns
"busy" without touching anything — FSC-R1-001). It calls `Receive` once
for the destination with every open thread root and the persisted
destination cursor, then judges each inbound envelope in provider order.
A reply is Wido's word iff BOTH:

1. `Inbound.UserID` equals `channel.human.<provider>.user-id` (configured,
   not secret; his Slack member id), and
2. the text ends with a valid TOTP code: 6 digits, RFC 6238 with the
   secret `channel.human.totp-secret` (secret, conf.local/env), SHA-1, 30 s
   step, window ±1 step, and the matched step not yet consumed. Consumption
   is durable BEFORE attribution: under the lock, a row `{step,
   destination, provider, threadID, ref, qid}` is appended to
   artifacts/agents/channel/totp-consumed.json (fsync, rename) and only
   then does the envelope proceed; a step already present is a replay
   unless its row equals this envelope on ALL of destination, provider,
   threadID, ref and qid (a resumed phase, below; FSC-R2-001). Rows older
   than the window are pruned. The code is stripped; the rest is the answer.

Any other reply (wrong user, no code, bad code, replay) is answered ONCE
per inbound ref, "not recorded: <reason>; reply with your answer and your
code", journaled in `rejected`; the ledger is untouched. After three
rejections in one question the posts stop (journaling continues) so a
stranger cannot make the bot chatter.

An authenticated reply is a resumable phase machine on the question
record (FSC-R1-002), `answer.phase` ∈ matched → recorded → receipted →
closed, each phase durable (write, fsync, rename) before the next acts:
(a) MATCHED: the record gets `answer` {text, userID, ref, at, step, ulid}
with a caller ULID ALLOCATED NOW; the op id is `goal.Opid(ulid, machine,
lineage)`, the ledger's only valid shape (FSC-R2-002); `state: answered`;
(b) RECORDED: ONE goal transaction with that op id writes the history
operation `answer` (`actor=human:wido`, the verbatim text with the
`wants` token when present in the reason field, where
`RecordedNormApproval` scans) AND appends `ANSWERED <qid>: <text>` to the
next step, so the two land together or not at all; a repeated op id is a
no-op, so a crash between (a) and (b) re-runs (b) exactly once and one
ANSWERED line exists (FSC-R2-003); (c) RECEIPTED: the thread gets
"recorded as your word on <goal in words>, ledger operation <opid>"; (d)
CLOSED: `state: closed` by one rename. Poll visits every question whose
state is open OR whose answer phase is not closed, resuming at the first
undone phase. The history grammar (FSC-R1-003, FSC-R2-004): the line
carries `authorityOutcome=AUTHENTICATED_CHANNEL_WORD` and exactly four
new keys `channelProvider=<registry name>`, `channelUser=<provider user
id>`, `channelRef=<threadID>/<message id>` (both provider tokens, URL-safe
already for Slack ts; otherwise percent-encoded), `channelStep=<decimal
TOTP step>`; the renderer emits them in that order and the parser rejects
any other key, as today. Dispositions: an envelope that correlates to no
open or resuming question (a Telegram destination update, a stray Slack
reply) is journaled by ref in
artifacts/agents/channel/<destination>/unmatched.jsonl (FSC-R2-005);
the destination cursor is persisted only after every envelope of the pass
is durably rejected, unmatched, or at phase ≥ matched.

What the recorded word may DRIVE, exactly: nothing runs by itself. The
consumers, named (FSC-R1-007): (1) `goal claim --approved-ref <opid>` —
`RecordedNormApproval` finds the strict token in the op's reason (no
change); (2) `goal resume --approved-ref <opid>` and `goal set-obligation
--approved-ref <opid>` — the humanauthority consumers for exactly these
two acts (the R-32-m1 set, no wider) gain a branch that validates an
`AUTHENTICATED_CHANNEL_WORD` operation on the same goal whose text answers
the question, independent of the R-32-m1 horizon (2026-09-06); set-budget
and enroll-terminal stay enrolled-terminal only; (3) a fork or reserved
decision: the seat's judgment, quoting the op id. Decision D5 in §9.
`channel wait --question <qid> [--timeout <m>]` blocks a script, not a
turn, until the record is answered, and prints the answer.

## 6. Configuration

```
channel.destination.fleet.adapter=slack|fake     # alert §4 idiom; telegram, whatsapp later
channel.destination.fleet.slack.channel-id=<C…>
channel.destination.fleet.slack.bot-token=<SECRET, conf.local or env only>
channel.destination.fleet.slack.api-base=https://slack.com/api
channel.human.slack.user-id=<U…>
channel.human.totp-secret=<SECRET, base32, conf.local or env only>
channel.status.interval-minutes=240
```

Unconfigured: `status` prints locally, `ask` records as undelivered, the
tick touches no network; a committed secret-named key is reported and
ignored (alert §10). Secret and token are Wido's acts after the build.

## 7. Command surface and the tick

`metasystem channel status [--post]`, `channel ask ...`, `channel show
--question <qid>`, `channel wait --question <qid>`, `channel poll` (what
the tick runs; callable by hand, "busy" if the tick holds the lock),
`channel close --question <qid> --because <text>` (withdrawal, posts the
reason). The channel phase is the LAST duty of both tick drivers (the
resident runner and `steward tick`), after RunTick released the
arbitration lock and after revival and pending delivery, under ONE
15-second context for the whole phase and a work budget per pass (one
`Receive`, five dispositions, one status post; the rest carries to the
next tick, FSC-R1-006). A channel failure is an undelivered count.

## 8. Proof plan (tests by name; fixture script)

internal/channel: `TestReportComposesFromLedgerJobsAndLandings` (fixture
ledger, two jobs, three trailered commits → exact text);
`TestReportOmitsEmptySectionsAndCaps`; `TestStatusCadenceAndDigestGate`;
`TestAskWritesRecordBeforePosting`; `TestAskDedupsOpenQuestion`;
`TestTOTPVerifiesRFC6238Vectors` (appendix B, SHA-1); `TestTOTPWindowAndReplay`;
`TestPollRejectsWrongUserNoCodeBadCodeReplay` (one post each, cap three);
`TestPollRecordsAuthenticatedReply` (record, history op, thread close,
next-step line); `TestAnswerCarryingStrictTokenSatisfiesNormApproval`
(`goal claim --approved-ref <opid>` succeeds);
`TestSecretsScrubbedFromErrors`; the reviews' obligations by name: `TestPollAtomicallyConsumesTOTP` (two polls,
one step, one attribution), `TestTOTPResumeExceptionIsEnvelopeScoped`
(equal ref from another provider in the same step is a replay),
`TestPollCrashRecoveryExactlyOnce` (crash injected after each phase,
re-poll: one history op, ONE ANSWERED line, the ledger parses, thread
closed, cursor advanced last), `TestAuthenticatedChannelHistoryRoundTrip`
(the four keys, exact spellings), `TestInboundCheckpointSurvivesCrashAndDeduplicates`
(an unmatched update before a valid reply; cursor still acknowledges),
`TestReportHasDurableSpendSource`, `TestTickChannelPassBound`,
`TestAuthenticatedChannelAuthorityAfterTemporaryHorizon` (resume and
set-obligation with `--approved-ref` on 2026-09-07),
`TestCredentialIsTokenIdentity`. internal/channel/slack against the fake:
`TestPostRootAndThreaded`, `TestReceivePagesAndFiltersByCursor`,
`TestCredential`, `TestUnconfiguredIsTyped`. Fixture
`scripts/agents/channel-fixtures.sh`: start the fake, configure a clone
with the fake adapter, post status (assert text), ask a budget-above-norm
question, script a reply without a code (assert the rejection post), then
one with the right code (assert `answer actor=human:wido` in the goal's
history, the close post, and a claim with `--approved-ref` of that op
succeeding), then `channel wait` returns the answer. Live: one `channel
status --post` by hand when the token arrives; never in a suite. R-31.

## 9. Slices and decisions Wido may still change

Slice 1 (this goal, one build): §2–§8. Later goals: telegram and whatsapp
adapters behind §2; alert-channel slice 1 (episodes, single-flight,
chunking) on the same package; a priced spend source; a fleet-wide
roll-up if Wido wants one message instead of one per machine.
D1 TOTP as the reply authentication (his prior word; user id alone is
weaker: anyone at his unlocked phone). D2 one status message per machine,
240-minute cadence plus on demand. D3 polling on the tick, not Slack
Events (no listener, no public endpoint). D4 the recorded answer drives
nothing by itself. D5 it may drive `resume` and `set-obligation` (the
R-32-m1 set) beyond 2026-09-06; Wido may narrow this to recording only.

## 10. Self-grade and review dispositions

Three operations, one secret, one id, one lock, one durable consumption
row; every report source is on main. Weakest: `Planned` readiness reads
the ledger's budget presence, which breach-clock work still changes. Round 1 (fsc-design-crit1): R1-001/002/003/007 folded §5 (+D5), 005 §3,
006 §7, 008 §2; 004 AMENDED — receive destination-wide, the persisted
cursor is the acknowledgment, the named test stands. Closing review
(fsc-design-crit2), all folded as build law in §5/§8: R2-001 consumption
row scoped to the whole envelope; R2-002 op id from a caller ULID via
goal.Opid; R2-003 history op and ANSWERED line in one transaction;
R2-004 the four key spellings; R2-005 the unmatched disposition.
