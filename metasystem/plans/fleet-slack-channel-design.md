# Fleet Conversation Channel — design (goal fleet-slack-channel, revision 1)

Author m0b (Fable lane), 2026-09-03. Tier 3 under R-54-m1: this design, one
Sol review, one fold, one closing review, build, one code review. R-60-m1
binds the reviews: a finding is material only if it changes what gets built
and names the artifact. Under 300 lines by order.

## 0. Wido's words this design serves

Status per machine: "work done, under way and planned, per machine".
Questions: "a way to raise questions of a machine to me ... threaded
conversations about questions from machines". Providers: "Slack NEXT TO
telegram; ... implement the slack adapter first. It needs to be switchable
between providers (Slack, Telegram, Whatsapp)." Authentication of his reply:
his seat-mutual-awareness word, a TOTP code or equivalent (binding design
word for the external inbound channel). Done: a machine posts status and
opens a question; he answers from his phone; the answer lands on the ledger
as his word; the machine continues; proven against a fake endpoint.

## 1. What exists and is reused (traced)

1. plans/alert-channel-design.md (revision 13) DECIDED the outbound contract
   and this design adopts those sections as law, not as input to redo:
   §2 `Message`/`MessageRef`/`ChunkOutcome`, §2a the stateless adapter and
   the Slack facts (Web API `chat.postMessage` with a bot token, never a
   webhook, because only `ts` can thread), §3/§3a the conversation reference
   store at artifacts/agents/channel/<destination>/conversations.json and
   the thread-state sufficiency invariant (Slack `ThreadID` = root `ts`), §4
   the `channel.destination.<name>.*` key idiom, §10 credentials (secrets
   only from environment or metasystem.conf.local; literal scrubbing of
   every resolved secret from every problem string). §2b reserved the
   INBOUND half explicitly; this design is that reserved work for one
   provider at a time.
2. No `internal/channel` package exists on main today; the alert design's
   slice 1 has not been built. This feature builds the package the alert
   design named, with the outbound surface it needs and nothing it does
   not (§9 chunking and §5 single-flight are deferred; status posts are
   bounded by composition, §3 below).
3. Human authority: internal/humanauthority proves an enrolled agent-free
   terminal (`HUMAN_AUTHORITY_PROVEN`) or, under R-32-m1 until 2026-09-06,
   a relayed verbatim word for exactly `goal resume` and `goal
   set-obligation` (`TEMPORARY_HUMAN_WORD`). internal/goal/norm.go
   `RecordedNormApproval` already accepts, as an approval, a goal-history
   operation whose actor is `human:` and whose line carries the strict token
   `goal=<id> minutes=<n> goalRevision=<r>`. The ledger's history lines
   carry `actor=human:<name>` for human acts.
4. Sources for the report, all durable: the goal ledger (plans/goals/*.md:
   state, claim, intent, next step, history); the landing history (git log
   of origin/main, `Goal-Item:` trailers, commit subjects in plain words);
   job records (artifacts/agents/<job>/status and rounds; internal/report
   scans running and open work); spend (internal/usage per job record, the
   cost lines adapters already write into events.jsonl).
5. The steward tick (internal/steward, `metasystem steward tick`) is the
   fleet's one scheduled observation per machine; it already narrates
   health and will carry the channel's poll and cadence.

## 2. The provider contract, fixed once

Package `internal/channel`. One interface, every provider behind it,
selected by configuration (§6). Operations, exactly five:

```
type Provider interface {
    // outbound (alert design §2a shape: one submission, one ref, typed error)
    Post(ctx, dest DestinationConfig, text string, thread *MessageRef) (MessageRef, error)
    // inbound (the §2b reserved half, polled, never a listener)
    Replies(ctx, dest DestinationConfig, thread MessageRef, after Cursor) ([]Inbound, Cursor, error)
    // identity of the configured human on this provider, for §5
    Whoami(ctx, dest DestinationConfig) (ProviderIdentity, error)
}
type Inbound struct { Ref MessageRef; UserID string; Text string; At time.Time }
type Cursor string // provider-opaque; Slack: the last seen reply ts
```

`Post` with `thread == nil` starts a thread and returns its root ref;
with a thread it posts into it. Close is not a provider operation: a closed
question is a state in the question record (§4) plus one final `Post`
into the thread saying so; providers keep no state (alert §2a). The
15-second per-call transport bound and the typed sanitized errors
(`ErrUnconfigured`, `ErrSendFailed`, `ErrReceiveFailed`) are the alert
design's. Registry for this goal: `slack`, `fake`. `telegram` and
`whatsapp` are later goals behind this same interface; the registry
refuses an unknown provider name with the name in the message.

**Slack adapter** (`internal/channel/slack`): `Post` = `chat.postMessage`
(`channel`, `text`, `thread_ts` when threaded); `Replies` =
`conversations.replies` (`channel`, `ts` = thread root, `oldest` = cursor,
`limit` 200, paged by `response_metadata.next_cursor` until exhausted),
returning only messages with `ts > cursor` and skipping the bot's own
user; `Whoami` = `auth.test`. Base URL is `slack.api-base` (default
https://slack.com/api); the fixture points it at the fake.

**Fake provider** (`internal/channel/fake`, plus the fixture server): an
in-process HTTP server speaking exactly the three Slack methods above with
Slack's JSON shapes (`ok`, `ts`, `messages[]`, `error`), a scripted reply
queue the fixture appends to, and a request journal the tests read. The
Slack adapter under test talks to it through `slack.api-base`, so the fake
proves the adapter's bytes, not a mock of the adapter. `fake` as a
configured provider name is the same server started in-process by the
`metasystem channel` verbs for the fixture script (§8).

## 3. The per-machine status report

Composed by `internal/channel/report.go` from §1.4 sources only; never from
a session's memory. One report per machine, one Slack message (no thread),
plain words, feature names not slice or job ids (R-61-m1 reporting rule).
Shape, in this order, each section omitted when empty:

```
<machine> status <YYYY-MM-DD HH:MM>Z
Landed since <window start>: <feature> — <commit subject> (<n> landings)
Under way: <feature> — <goal next-step first sentence>; job <role> running <m> min
Planned: <feature> (queued, ready|needs budget|blocked by <feature>)
Spend today: $<usd> across <n> jobs (<runtime>: $<usd>, ...)
Undelivered: <n> channel messages, oldest <m> min   ← only when non-zero
```

`<feature>` is the goal id rendered as words (hyphens to spaces) followed by
the intent's first sentence truncated to 120 characters. "Landed since"
reads origin/main commits with a `Goal-Item:` trailer naming a goal this
machine claimed or claims, since the previous status post (state file
artifacts/agents/channel/status.json: last post time, last content
digest, last ref). "Under way" = goals claimed by this machine, plus any
running delegate job under them (job id, role, elapsed). "Planned" = queued
goals pinned to or last claimed by this machine, with readiness from the
ledger (a human budget stored or not; blocked-by). "Spend today" sums
internal/usage cost for jobs whose records started today UTC, by runtime.

Cadence: the steward tick posts when `channel.status.interval-minutes`
(default 240) have passed since the last post AND the content digest
changed; `metasystem channel status` prints it; `--post` posts now
regardless of cadence (the on-demand path). Size: the composer caps each
section at 12 lines and the whole at 3500 bytes, truncating with
"(+n more)"; no chunking.

## 4. The question thread contract

A question is a durable record `artifacts/agents/channel/questions/<qid>.json`
(qid = ULID), written before any network:

```
{ "id", "goal", "kind": "budget-above-norm|fork|reserved-decision|stop|other",
  "machine", "openedAt", "facts": [..], "options": [{"label","consequence"}],
  "recommendation", "wants": "<strict token the answer must carry, or empty>",
  "thread": MessageRef|null, "cursor", "state": "open|answered|closed",
  "answer": {"text","userID","ref","at","authority":"AUTHENTICATED_CHANNEL_WORD",
             "opid"}|null, "undelivered": n }
```

Opened by `metasystem channel ask --goal <id> --kind <k> --fact ... --option
"label: consequence" ... --recommend <label> [--wants "<token>"]`. The verb
writes the record, posts the thread root (question text: goal in words,
kind, facts, options with consequences, recommendation, and the exact reply
form: "reply in this thread with your answer followed by your code"), then
appends to the goal's next step one line: `ASKED <qid> (<kind>): <first
fact>`. One thread per question; a second `ask` with the same (goal, kind,
facts digest) while one is open returns the existing qid and posts nothing.
Undelivered (provider unconfigured or failed): the record exists with
`thread: null`, the tick retries the root post each pass, and the count
rides the health line (alert §10 floor). Asks never block a turn: the seat
records the qid on the goal and carries on with unblocked work.

`--wants` fixes the ledger token an answer must produce for machinery to
consume it: for `budget-above-norm`, `goal=<id> minutes=<n> goalRevision=<r>`
(norm.go's strict form); for `stop`, the resume tuple; otherwise empty.

## 5. The reply path: authentication and the recording as his word

The tick calls `channel.Poll`: for every open question with a thread, it
fetches `Replies` after the record's cursor, advances the cursor, and
judges each inbound message in order. A reply is Wido's word iff BOTH:

1. `Inbound.UserID` equals `channel.human.<provider>.user-id` (configured,
   not secret; his Slack member id), and
2. the text ends with a valid TOTP code: 6 digits, RFC 6238 with the
   secret `channel.human.totp-secret` (secret, conf.local/env), SHA-1, 30 s
   step, window ±1 step, and the (secret, step) pair not yet consumed
   (replay store artifacts/agents/channel/totp-consumed.json, pruned past
   the window). The code is stripped; the rest, trimmed, is the answer.

Any other reply (wrong user, no code, bad code, replayed code) is posted
back ONCE per inbound ref: "not recorded: <reason>; reply with your answer
and your code" and journaled in the record's `rejected` list; nothing
touches the ledger. Three rejections in one question stop further
rejection posts (the record still journals) so a stranger cannot make the
bot chatter.

An authenticated reply is recorded in this order, each step durable before
the next: (a) the question record gets `answer` and `state: answered`; (b)
the goal's history gets one operation `answer` with
`actor=human:wido`, the verbatim answer text, the qid, the provider ref,
and the authority outcome `AUTHENTICATED_CHANNEL_WORD` (new outcome in
internal/humanauthority, alongside PROVEN and TEMPORARY; its proof carries
provider, user id, message ref and the TOTP step, never the secret or the
code); when the question `wants` a token and the answer contains it
verbatim, the line carries it so `RecordedNormApproval` and the resume
path find it; (c) the thread gets "recorded as your word on <goal in
words>, ledger operation <opid>"; (d) `state: closed`. The goal's next
step gets `ANSWERED <qid>: <answer text>`.

What the recorded word may DRIVE, exactly: nothing runs by itself. The
waiting machine consumes it as it consumes any human word on the ledger
today: `--approved-ref <opid>` for a norm claim, the R-32-m1 relayed-word
path for `resume`/`set-obligation` with the recorded text as the word (the
op id is its provenance, stronger than a seat's relay), and the seat's
own judgment for a fork or a reserved decision, quoting the op id. No new
human-only act is opened; the record is the word (R-32-m1's scope stands).
`metasystem channel wait --question <qid> [--timeout <m>]` blocks a
script, not a turn, until the record is answered, printing the answer.

## 6. Configuration

```
channel.provider=slack|fake                      # telegram, whatsapp: later goals
channel.destination.fleet.adapter=<provider>     # alert §4 idiom, one destination
channel.destination.fleet.slack.channel-id=<C…>
channel.destination.fleet.slack.bot-token=<SECRET, conf.local or env only>
channel.destination.fleet.slack.api-base=https://slack.com/api
channel.human.slack.user-id=<U…>
channel.human.totp-secret=<SECRET, base32, conf.local or env only>
channel.status.interval-minutes=240
```

Unconfigured provider: `status` prints locally, `ask` records and reports
"undelivered: channel unconfigured", the tick does nothing on the network.
Secrets never in the committed file; a committed secret-named key is
reported and ignored (alert §10). The TOTP secret is Wido's to generate and
place on each machine that asks (each machine polls its own questions);
the bot token likewise. Both are his acts, after the build.

## 7. Command surface and the tick

`metasystem channel status [--post]`, `channel ask ...`, `channel show
--question <qid>`, `channel wait --question <qid>`, `channel poll` (what
the tick runs; callable by hand), `channel close --question <qid> --because
<text>` (machine-side withdrawal, posts the reason). The steward tick,
after its health narration, runs `channel poll` then the cadence check for
status, each under the 15 s per-call bound, never under the arbitration
lock, never gating any machinery: a channel failure is an undelivered
count, not a stopped fleet.

## 8. Proof plan (tests by name; fixture script)

internal/channel: `TestReportComposesFromLedgerJobsAndLandings` (fixture
ledger, two jobs, three commits with trailers → exact text);
`TestReportOmitsEmptySectionsAndCaps`; `TestStatusCadenceAndDigestGate`;
`TestAskWritesRecordBeforePosting`; `TestAskDedupsOpenQuestion`;
`TestTOTPVerifiesRFC6238Vectors` (RFC 6238 appendix B, SHA-1);
`TestTOTPWindowAndReplay`; `TestPollRejectsWrongUserNoCodeBadCodeReplay`
(one rejection post each, cap at three); `TestPollRecordsAuthenticatedReply`
(record, history op with `actor=human:wido` and outcome, thread close,
next-step line, order durable on injected failure between steps);
`TestAnswerCarryingStrictTokenSatisfiesNormApproval` (a claim with
`--approved-ref <opid>` succeeds); `TestSecretsScrubbedFromErrors`.
internal/channel/slack against the fake server: `TestPostRootAndThreaded`,
`TestRepliesPagesAndFiltersByCursor`, `TestWhoami`, `TestUnconfiguredIsTyped`.
Fixture `scripts/agents/channel-fixtures.sh`: start the fake, configure a
clone with `channel.provider=fake`, post status (assert the text), ask a
budget-above-norm question, script a reply without a code (assert
rejection post), script a reply with the right code (assert the goal's
history shows `answer actor=human:wido`, the thread's close post, and a
claim with `--approved-ref` of that op succeeds), then `channel wait`
returns the answer. Live: one `channel status --post` by hand when the
token arrives; not in any suite. No benchmarks (R-31).

## 9. Slices

Slice 1 (this goal, one build): §2 contract + fake + Slack adapter, §3
status, §4 ask, §5 poll/answer/record, §6 keys, §7 verbs and tick hook, §8
proof. Later goals, not this one: telegram and whatsapp adapters behind §2;
alert-channel slice 1 (episodes, single-flight, chunking) riding the same
package; a fleet-wide roll-up report if Wido asks for one message instead
of one per machine.

## 10. Decisions Wido may still change (recorded, not blocking)

D1 TOTP as the reply authentication (his prior word); alternative: Slack
user id alone (weaker: anyone at his unlocked phone). D2 one status message
per machine, on a 240-minute cadence plus on demand. D3 polling on the tick
rather than Slack Events (no listener, no public endpoint, fits the
stateless adapter). D4 the recorded answer drives nothing by itself; the
machine consumes it by the existing paths.

## 11. Self-grade

The five-operation contract is the smallest that carries the three message
kinds and the inbound half; the authentication is one secret and one
configured id; every source of the report is durable. Risk: the tick's poll
adds network to the fleet's heartbeat, bounded at 15 s per call and never
under a lock. Weakest part: `Planned` readiness is derived from the ledger's
budget presence, which the breach-clock work is still changing.
