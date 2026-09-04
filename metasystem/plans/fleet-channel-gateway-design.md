# fleet-channel-gateway — design: one bot, one inbox, first commit wins (revision 2)

Goal: plans/goals/fleet-channel-gateway.md. Wido's decisions, verbatim
from the goal record (2026-09-03): "one bot, one git inbox, FIRST COME
FIRST SERVED - no leases"; "THE RULE, every provider: receive -> commit
to the shared git inbox -> confirm. First commit wins; others find it
committed (idempotent by provider message id) and skip; git's push race
is the arbiter"; "LONG-POLL - one open getUpdates per machine with a
30-50s timeout ... a bounded base interval with JITTER"; "NO adaptive
leader or back-off scheme - that would reinvent the lease"; "Identity
check (per-provider user id + shared TOTP) on the committing machine";
"FREEDOMS: inbox format/location". Priority R-76-m3: "a properly
working channel (telegram)" before the central brain. Approved by Wido
2026-09-04 at the terminal ("All approved, also said this on Telegram";
ledger 1dee9746), tier 3, box 1d/10/1200m/1/3 from the norm. Companion
goals: answer-archive (harvests the inbox; sequenced after, owns
rotation), channel-poll-not-automatic (tier 1, the steward runs the
poll), channel-budget-answer-binds-nothing (m2, done 2026-09-04: a
verified budget answer with the token re-approves the goal),
channel-code-verified-at-poll-time (done: the code is checked against
the message's own send time), channel-poll-refuses-legacy-budget-questions
(m3, in chain: a legacy budget question loads without a tuple).

Revision 2 answers critique round 1 (job fcg-design-cc1, fifteen
material findings FCG-C-01..15 at main 09678719); the dispositions are
at the end. Cites are re-read at main 83a2bcaf. What changed, in one
breath: the inbox moves out from under plans/goals/ so no engine that
exists today ever parses it; every message commit carries one opid
that every machine derives from the message itself, so the second
machine's transaction is its own idempotent success and not a loss;
the provider contract gains the per-update acknowledgement the receive
rule needs; both records get exact schemas and the validator an exact
refusal table; an unthreaded message binds only when it carries the
question's own token or option; every outbound post by a machine other
than the asker goes through one intent-then-ref protocol; m2's
two-step approval keeps its retry state on the ledger; the fake grows
a control file; compaction leaves this goal; the secret's every surface
is named.

The shape in one paragraph. Today every channel record lives under
artifacts/agents/channel/, which .gitignore:1 keeps out of git: the
question, its thread, the cursor, the replay register and the unmatched
file are all per checkout (internal/channel/question.go:64). So only the
machine that asked can match a reply, only that machine's poll can
verify the code, and two machines on one bot token fight over
getUpdates. This design moves the two records that matter, the question
and the inbound message, onto the goal ledger's branch, committed
through the ledger's own transaction engine (internal/goal/txn.go: fetch
the tip, rebuild on it, compare-and-swap push, retry under benign
advancement). The push race that already arbitrates every goal verb
arbitrates the inbox too: the first machine whose commit of a message
lands wins; every other machine rebuilds on the new tip, finds the
message committed under the very opid it is itself trying to publish,
and reports idempotent success. Nothing is confirmed to the provider
until the commit is durable, so a machine that dies mid-way confirmed
nothing and the message comes again.

### FCG-PRINCIPLE-01: the inbox is the ledger, the race is the arbiter, there is no leader

The shared inbox is a directory on the goal ledger branch
(goal.sync-branch, refs/heads/main by default, internal/goal/txn.go:49-61),
written only through goal.Publish (txn.go:508) with the same
Goal-Transaction trailer (txn.go:283), the same CAS publish
(txn.go:347) and the same classification by refetch. Every machine
that holds a provider token may receive; every machine that receives
commits; the commit is idempotent by a shared opid derived from the
provider message (FCG-INBOX-02). No machine is designated, elected or
leased. A machine without a provider token never receives and never
posts; it reads its answers from the inbox like everyone else
(FCG-READ-08).

What this rules out, by Wido's word: a gateway host, a lease and its
failover, an adaptive leader, a back-off that converges on one poller.
A conflict from the provider (Telegram's 409 "terminated by other
getUpdates request") is not an error and not a signal to stand down;
it is the ordinary sound of two machines polling one bot, handled by
the jittered retry of FCG-POLL-04 and nothing else.

### FCG-INBOX-02: two record kinds, one directory, written once each

Location: `plans/channel/` on the ledger branch, a sibling of
plans/goals/, never inside it. ReadCommitGoals (validate.go:405-419)
lists `plans/goals/` and `records/goals/` and nothing else, so every
engine built before this goal, and every engine after it, lists no
channel file when it validates a tip (txn.go:595-611, project.go:47-64)
and the parser at validate.go:69-105 never meets one (FCG-C-01,
FCG-C-02). The directory rides the transport
(scripts/agents/sync-transport.sh) and `goal fetch` unchanged because
they move the whole branch. Build step 1 adds three fences and no
writer: `install:plans/channel/ ledger` in
scripts/agents/path-classes.txt beside the four ledger rows
(path-classes.txt:34-37), so a landing that touches it refuses with
`ledger-path-not-goal-verb` (internal/landing/observe.go:586); the
pre-commit guard's regexp widens from `(^|/)plans/goals/` to
`(^|/)plans/(goals|channel)/` (scripts/agents/pre-commit-guard.sh:72);
and a new validator ValidateChannelTree(root, commit) in a new package
internal/goal/channel.go, called from ValidateCommit after the goal
validation, reads `plans/channel/` and applies the refusal table
below. An engine without step 1 ignores the directory; an engine with
it refuses a malformed one. A machine on the old engine keeps
committing goal verbs onto a tip that carries plans/channel/ because
its validator never reads it.

Question record: `plans/channel/questions/<qid>.json`, one JSON
object, canonical serialisation `json.MarshalIndent("", "  ")` with a
trailing newline, keys in the struct order below, no unknown keys
(the validator refuses one). `<qid>` is the 26-character Crockford
ULID the question already carries (question.go:39, `ID`).

| key | type | required | meaning |
|---|---|---|---|
| id | string | yes | equals the file's basename |
| goal | string | yes | goal id; the goal file must exist on the same tip |
| kind | string | yes | one of reserved-decision, stop, budget-above-norm (the vocabulary of `channel ask --kind`, cmd/metasystem/channel_verbs.go) |
| machine | string | yes | the asking machine (goal.ResolveMachine) |
| lineage | string | yes | the asking lineage |
| openedAt | RFC 3339 UTC, second precision | yes | |
| facts | []string | yes, may be empty | |
| options | []{label, consequence} | yes, may be empty | |
| recommendation | string | yes, may be "" | |
| wants | string | yes, may be "" | the token; required non-empty for kind stop and budget-above-norm (Ask's existing rule) |
| budget | object or absent | budget-above-norm only | the goal.Budget tuple; Ask's validateQuestionBudget rule unchanged |
| destination | string | yes | the destination name (channel.destination) |
| thread | {provider, id, threadId} or null | yes | the question post; null until posted |
| posting | {kind, by, at} or null | yes | an outbound post in flight (FCG-POST-08) |
| state | string | yes | open, answered, closed |
| answer | object or null | yes | below |
| rejected | []{ref, reason, at, postRef, by} | yes, may be empty | shared; at most three entries with postRef non-null |
| factsDigest | string | yes | as today |
| closedAt, closedBy, closedBecause | string or absent | closed only | |

`answer`: {text, userId, ref{provider,id,threadId}, at, step, inboxId,
phase, approvalUlid, approvalOpid, receipt, receiptRef}. `phase` is one
of recorded, approved, receipted; `approvalUlid`, `approvalOpid` are
set only for budget-above-norm questions whose text equals `wants`
and whose `budget` is present (FCG-ANSWER-11); `receipt` is the text
posted back; `receiptRef` is null until posted. The per-machine
`undelivered` counter leaves the record: delivery is FCG-POST-08's
business.

Inbound record: `plans/channel/inbox/<destination>/<provider>-<message
id>.json` (Telegram: `telegram-<message_id>`; Slack: `slack-<ts>` with
the dot kept).

| key | type | required | meaning |
|---|---|---|---|
| provider | string | yes | telegram, slack |
| destination | string | yes | |
| messageId | string | yes | equals the basename's id part |
| updateId | string | yes | Telegram update_id as decimal; Slack: the ts again |
| replyTo | string or null | yes | provider parent message id |
| userId | string | yes | provider user id of the sender |
| sentAt | RFC 3339 UTC | yes | the provider's send time |
| text | string | yes | the human's words with the code removed (FCG-SECRET-15); "" when the message was only a code |
| step | integer or null | yes | the verified TOTP step; null unless outcome is verified or replayed |
| outcome | string | yes | verified, wrong-user, no-code, bad-code, stale, replayed |
| question | string | yes | a question id, or `unmatched`, or `unbound` (FCG-MATCH-06) |
| receivedBy | string | yes | the committing machine |
| receivedAt | RFC 3339 UTC | yes | |

The shared opid. Both records are committed under
`Opid(ulid, "inbox", key)` (file.go:1494: `<ulid>-<machine>-<hash8>`,
shape checked by validOpidShape, file.go:1470) where the machine
segment is the literal `inbox`, `key` is
`<provider>:<destination>:<messageId>` and `ulid` is built
deterministically: its 48-bit time is `sentAt` in milliseconds and its
80 random bits are the first ten bytes of SHA-256(key). Two machines
receiving one message therefore mint one opid. The Mutate of the
Publish reads the tip: if the record path exists AND TrailerPresent
(txn.go:379) finds this opid in the tip's history, it returns
goal.AlreadyApplied — lawful, because it is this operation's own opid
on the rebuilt tip (txn.go:449-456) — and Publish reports
OutcomeConfirmed with Detail "idempotent" (txn.go:733-742). If the path
exists without the trailer the tree was hand-repaired: the Mutate
returns an error naming it (`inbox record present without its
transaction`) and the message is never confirmed until a human looks
(FCG-C-04). The receiving machine's own journal entry (journal.go)
carries the shared opid with its own machine name in the entry's
Machine field; the opid attributes the message, the entry attributes
the executor. LostToCompetitor is never returned by an inbox Mutate.

The secret never enters either record: the code is removed before the
record is built (FCG-SECRET-15), and the replay check uses the step.

Rotation is not this goal's (FCG-C-13): the inbox only grows here, at
about one kilobyte per message; answer-archive owns harvest and
removal, and until it lands nothing is removed. A `channel compact`
verb is not built.

ValidateChannelTree refuses, each by name, and a refusal abandons the
publishing transaction exactly as a goal refusal does:

| code | condition |
|---|---|
| channel-unknown-path | a file under plans/channel/ outside questions/ and inbox/<destination>/, or not `.json` |
| channel-json | not a JSON object, or an unknown key, or a key of the wrong type |
| channel-id-mismatch | basename does not equal id / provider-messageId |
| channel-goal-missing | question.goal has no goal file on the tip |
| channel-kind | kind, state, outcome, phase, posting.kind outside their vocabularies |
| channel-token-missing | kind stop or budget-above-norm with empty wants |
| channel-budget | budget present on a non-budget kind, or absent or incomplete on budget-above-norm |
| channel-answer-state | state answered or closed with answer null; state open with answer non-null; answer.question of an inbox record naming a question whose answer.inboxId is not this record while outcome is verified and the question is not open |
| channel-rejection-cap | more than three rejected entries with postRef non-null |
| channel-secret | any string field matching `(^|\s)[0-9]{6}(\s|$)` in text, facts, recommendation, answer.text, rejected[].reason, or receipt |

The validator is at rest: it reads one tree. Create-once and the
legal transitions are the Mutate's job on the fetched tip (a question
Mutate refuses `channel-transition` when the tip's state is not the
one it read; an inbox Mutate is idempotent as above), and the CAS
push makes the Mutate's reading of the tip the only reading that
lands. Deletion under plans/channel/ is refused by every Mutate in
this goal (answer-archive will own it).

### FCG-RECEIVE-03: receive, commit, then confirm, per provider

The provider contract (channel.go:18-43) changes in two places: Inbound
gains `Ack Cursor` — the cursor whose confirmation acknowledges this
item and everything the provider delivered before it — and Provider
gains `Confirm(ctx, DestinationConfig, Cursor) error`. Receive keeps
its second return, the batch cursor, meaning "everything in this
batch". Items the adapter filters out (another chat, the bot's own
messages, telegram.go:181-183) are not returned; they are acknowledged
by the Ack of the next returned item or, at the tail, by the batch
cursor (FCG-C-03).

Telegram. Receive calls getUpdates with NO offset (today the local
cursor is sent, telegram.go:203-208), limit 100, timeout T (FCG-POLL-04)
and returns each update as an Inbound with `Ack = update_id+1` and
`UpdateID`; Confirm(c) calls getUpdates with offset c, timeout 0,
limit 1 and discards the result — Telegram forgets every update below
c and the one it may return stays unconfirmed and comes again. The
listener handles a batch in update_id order: for each item, (1) build
the inbound record and verify (FCG-COMMIT-05), (2) Publish, (3) on
OutcomeConfirmed (fresh or idempotent) continue; on anything else stop
the batch. After the loop it calls Confirm once with the Ack of the
last confirmed item, or the batch cursor when every item confirmed, or
nothing when none did; then it writes the local cursor file for
`channel status` only. A machine that dies after (2) and before Confirm
has committed the message; the next poller on any machine receives the
same update, its Publish returns idempotent confirmed, and it confirms.
A machine that dies before (2) confirmed nothing.

Slack. Receive today pages conversations.replies per open thread with
a per-root cursor (internal/channel/slack/slack.go:96-146). It returns
each reply with `Ack` = its ts; Confirm is a no-op; the per-root
cursor stays local and advances only past committed replies. No
Socket Mode or Events work in this goal.

Email is out of scope here; the rule is recorded for it in the goal
(shared IMAP, first commit wins, mark-seen only after commit).

Loss window (FCG-C-09). The 150-second rule (poll.go:43,
`channelPollInterval`; poll.go:276-278; channel_test.go:533-570) goes:
it was the guard for a poll that ran every two minutes on the one
machine that could verify, and under the fleet rule it would turn a
three-minute gap in listening into a refused answer. The code is
verified at `sentAt` (unchanged, channel-code-verified-at-poll-time),
replay is refused by step on the ledger (FCG-COMMIT-05), and the only
age rule left is `channel.stale-sec` (default 86400): a message older
than that at receipt is committed with outcome `stale`. The honest
statement: an answer is lost only if no machine polls for a day, and
during a whole-fleet outage nothing can post a warning; visibility of
an outage is FCG-STATUS-09's.

### FCG-POLL-04: one long poll per machine, jittered, no leader

A resident listener, not the tick. Today channelphase.Run is a
15-second duty after the steward tick (internal/steward/runner.go:136-140,
cmd/metasystem/steward_verbs.go:272-278); a 30-50 second long poll
cannot live inside it. Lifecycle (FCG-C-10): RunLoop gains a
`ctx context.Context` parameter; the steward verb builds it with
signal.NotifyContext(SIGTERM, SIGINT) and cancels it when the stop
file appears, so disarm and re-arm (which restart the runner process)
end the listener with the loop. RunLoop starts
`go channel.Listen(ctx, repoRoot)` once, before its first tick, and
removes the channelphase.Run call at runner.go:136-140; the one-shot
`metasystem steward tick` keeps calling channelphase.Run
(steward_verbs.go:272-278), which now performs one short poll (timeout
0) under the same receive rule, so a machine without the resident
runner still receives when ticked. `channel poll` stays as the manual
single pass. Listen's loop: reload configuration from
metasystem.conf and metasystem.conf.local (phase.Load, phase.go:118-157,
called every iteration exactly as the phase does today, so a removed
token stops the listener at the next iteration and an added one
starts it); if no provider is configured, sleep base and loop; else
getUpdates with timeout T (default 40 s, `channel.long-poll-sec`);
handle the batch per FCG-RECEIVE-03; run the open-work pass
(FCG-POST-08, FCG-ANSWER-11); sleep base + jitter (base
`channel.poll-base-sec` default 30, jitter uniform in [0, base)). A
panic inside one iteration is recovered, logged with the update id
and the phase and never the text, and the loop continues after base.
Listen returns only when ctx is done; RunLoop waits for it before
returning.

A 409 conflict sleeps the same base + jitter and loops; it is logged
at debug level only and counted in the listener's status
(FCG-STATUS-09). The telegram adapter distinguishes the two 409 texts:
"terminated by other getUpdates request" becomes ErrBusy (a new
ErrorKind, channel.go:47-51), "webhook is active" stays
ErrReceiveFailed with today's message (telegram.go:83-85).

Rate: at base 30 s and T 40 s, an idle machine makes one request per
30-70 s; N machines make N such streams. Telegram publishes no
getUpdates limit; the 409 dance costs one refused request per
overlap. No machine ever changes its base in response to another.

### FCG-COMMIT-05: identity and code are checked by the committing machine, against the ledger

Before Publish, the receiving machine verifies, in this order, and the
first failure is the outcome: sender user id equals
`channel.human.<face>.user-id` (else wrong-user); the message carries
a code (FCG-SECRET-15; else no-code); the message is not older than
`channel.stale-sec` (else stale); the code verifies at `sentAt` with
one step of slack (VerifyTOTP, totp.go:35-50; else bad-code). The
replay register moves from artifacts/agents/channel/totp-consumed.json
(poll.go:365-388) into the Mutate: a step is consumed if any inbox
record at the tip carries the same step with a different messageId;
that check runs on the fetched tip inside the transaction, so two
machines committing two messages with one code cannot both win: the
second rebuilds on the first's commit, sees the step, and its record
is written with outcome `replayed`. Every outcome is committed (the
record exists whether verified or not); only `verified` records carry
a matched question forward. A rejection post for wrong-user, no-code,
bad-code, stale and replayed is made by the committing machine through
FCG-POST-08 with today's text (poll.go:200-209), only if the question
(threaded) or the destination's single open question (unthreaded) has
fewer than three posted rejections at the tip; the entry is appended to
that question's `rejected` with `by` = the committing machine, so the
ceiling and the post ids are shared (FCG-C-06). An unmatched rejection
(no question) is committed and not posted.

### FCG-MATCH-06: threaded first, then the one open question, never a stray

Matching runs on the committing machine against the questions at the
fetched tip. (a) A message whose replyTo is a question's thread id, or
one of that question's rejected[].postRef ids, or its receiptRef,
matches that question. (b) An unthreaded verified message from the
enrolled human matches the single open question on that destination
when exactly one is open AND the text, after the code is removed,
contains the question's `wants` token as a whole field, or equals one
of its option labels case-insensitively after trimming; a question with
neither wants nor options is matched by thread only. A verified
message that fails (b) is committed with question `unbound` and the
committing machine posts, through FCG-POST-08 and under the same
three-post ceiling, "not recorded: I have one open question
(<feature name>); reply in its thread, or with its token <wants>";
this is the answer to FCG-C-07: the code proves the sender, the token
or the label proves the question. (c) With several open, or none, the
record is committed `unmatched`; with several open the committing
machine posts one message listing the open questions by feature name
with "reply to the one you mean" (ceiling: once per inbound message,
never more than three such lists per hour per destination, counted
from the inbox records at the tip); with none open nothing is posted.
The three lost answers of 2026-09-04 (11:32Z, 12:02Z, 12:07Z;
poll.go:150-160 filed them to unmatched.jsonl unverified) all carried
the token and would match under (b).

A matched, verified record advances the question in the same commit:
state answered, answer{text, userId, ref, at, step, inboxId,
phase: recorded}. The goal `answer` history row (verbs.go:112-151) is
written in that same Publish under the shared opid, so the question
file, the inbox record and the ledger row are one atomic change set.
The approval, when one is due, is a second transaction (FCG-ANSWER-11).

### FCG-WORD-07: the token binds only when it is in the human's own words

goal.Answer today appends the asked token to the reason when the text
lacks it (verbs.go:120-123), and AuthenticatedChannelApproval
(verbs.go:67-93) then finds the token and binds: an authenticated "no"
approves. The append is removed. The reason is the human's text,
verbatim. AuthenticatedChannelApproval keeps its contiguous-fields
rule (verbs.go:96-108) over the text alone. A verified answer whose
text lacks the token can only reach the goal through rule (a)
(threaded); it is recorded as the human's word and binds nothing; the
question closes as answered; the asking machine, reading it
(FCG-READ-08), reports the words verbatim and asks again if it needs
the token. renderQuestion's prompt (question.go:209-225) already says
"Reply in this thread with this token verbatim".

m2's channel-budget-answer-binds-nothing compares the text to `wants`
by equality (poll.go:358) and is untouched by the removal of the
append; FCG-ANSWER-11 keeps its second step.

### FCG-POST-08: every post by a machine other than the asker is intent, post, ref

Telegram's Post creates a new message per call and has no idempotency
key (telegram.go:99-135), so a post made before its intent is durable
can be made twice by two machines and its id lost by a crash
(FCG-C-08). One protocol for the question post of a tokenless asker,
the rejection post, the list post and the receipt post: (1) Publish
`posting: {kind, by: <machine>, at}` on the question record — the
Mutate refuses `channel-posting-busy` when the tip already carries a
posting younger than `channel.posting-stale-sec` (default 300), and
takes over an older one; (2) post; (3) Publish the returned ref into
its field (thread, rejected[i].postRef, receiptRef) and clear
`posting`. Both Publishes use the receiving machine's own fresh opid
(they are not idempotent across machines; the intent is the fence).
A crash between (2) and (3) leaves an intent that another machine
takes over after five minutes and posts again: at most one duplicate
post per crash, recorded in the ledger by the surviving ref, never an
orphan whose existence the ledger does not know. A reply threaded to
the duplicate whose ref was lost is unthreaded to the matcher and is
caught by rule (b) when it carries the token. The asking machine's own
question post, when it has a token, stays as today: post, then write
the question record with `thread` set, in the `channel ask` Publish
(one transaction; a crash between post and Publish loses the post and
the ask, and the asker re-asks).

`channel wait` today loops on the local question file
(cmd/metasystem/channel_verbs.go:174-202). It now runs `goal fetch`
every interval (default 30 s, `--interval`) and reads the question at
the fetched tip; a machine with no provider token can ask (the ask is
a ledger commit with `thread` null) and wait; the first listener whose
open-work pass sees an open question with `thread` null and `posting`
null posts it under this protocol. Status posts (`channel status
--post`, report.go) stay per machine and direct; a machine without a
token cannot post status in this goal (the routing policy Wido named
is deferred to a follow-up goal).

The open-work pass, run by every listener after each batch: (i) open
questions with `thread` null → post; (ii) answered questions with
phase recorded and an approval due → FCG-ANSWER-11; (iii) answered
questions with a receipt and `receiptRef` null → post the receipt and
close; (iv) stale `posting` intents → take over. Each item is one
transaction and any listener may do it.

### FCG-STATUS-09: what the listener shows and what it refuses

Heartbeat: `plans/channel/listeners/<machine>.json` {lastReceiveAt,
lastConfirmAt, conflictsLastHour, updatedAt}, published by a listener
at most once per `channel.heartbeat-min` (default 60) and only when
its own getUpdates succeeded since the last heartbeat; a machine that
is down writes nothing. `channel status` on any machine reads the
listeners directory at the fetched tip and prints, per machine, the
age of its last receive, and one fleet line: "the fleet last heard
Telegram <age> ago" from the newest lastReceiveAt. The health line
(steward tick) carries the same fleet age and goes `unhealthy` past
`channel.silence-warn-hours` (default 12) while a question is open.
A listener that comes back after a silence longer than that posts
once "the channel heard nothing for <age>; answers sent in that time
may need repeating" through FCG-POST-08 on the oldest open question,
or nothing when none is open. The honest limit (FCG-C-09): while every
listener is down the channel says nothing; the signal in that state is
the health line on any machine and the absence of question posts.
Nothing here refuses the human (HCL-PRINCIPLE-01 applies: the channel
is the human's own path in).

### FCG-MIGRATE-10: questions asked before this lands

There is no dual period (FCG-C-14). A new engine's Poll, Listen and
`channel wait` read ledger questions only; `channel ask` writes ledger
questions only. A new verb `channel migrate` republishes every local
question in state open or answered (question.go:64-75 lists them) as a
ledger question with its id, thread, rejected entries and answer kept,
in one Publish per question, and then renames the local questions
directory to `questions.migrated`. A new engine whose local directory
still holds an open or answered question refuses every channel verb
with "N local questions predate the ledger inbox; run channel
migrate" — the same shape as today's refusal on a legacy budget
question, and the same fix path. The local files cursor.json,
unmatched.jsonl and totp-consumed.json are not read by the new engine
and are left in place. A machine still on the old engine keeps its
local poll until it rebuilds; its local open questions are invisible
to new listeners, and an unthreaded reply meant for one of them does
not carry a ledger question's token, so rule (b) does not bind it.

### FCG-ANSWER-11: the budget re-approval keeps its two steps on the ledger

m2's channel-budget-answer-binds-nothing records the answer, then runs
goal.Approve as a second transaction, and keeps ApprovalULID, Receipt
and Phase on the local question so an interrupted poll retries
(poll.go:341-380, question.go:29-39). Under this design the same state
lives on the ledger question's `answer`: the answer commit
(FCG-MATCH-06) writes phase recorded and, when kind is
budget-above-norm, text equals wants and budget is present,
`approvalUlid` (a deterministic ULID built as in FCG-INBOX-02 from the
key `approve:<qid>`) and `approvalOpid` = Opid(approvalUlid, "inbox",
"approve:<qid>"). The open-work pass of any listener then runs
goal.Approve with VerbRequest{Ulid: approvalUlid, Actor{Machine:
"inbox", Lineage: "approve:<qid>", Human: userId}} — the opid is
shared, so the second machine's Approve is AlreadyApplied and never a
duplicate approve row — and in the same pass publishes phase approved
with `receipt` = "recorded: <goal> box raised to <box>" (or the
Approve refusal's text). A budget question without a tuple (a legacy
record after channel-poll-refuses-legacy-budget-questions) gets phase
approved directly with the receipt that goal fixes. Non-budget
answers go from recorded to approved in the answer commit itself with
receipt "recorded: <goal> approved for execution" for a stop token, or
"recorded" otherwise. Phase receipted is set by FCG-POST-08 (iii) with
`receiptRef`, and the question closes in that commit. A crash anywhere
leaves a phase that the next pass on any machine resumes (FCG-C-11).

### FCG-SECRET-15: the code is removed before anything durable is written

Contract `StripCode(text) (clean, code string, present bool)` in
totp.go, replacing SplitTOTP (totp.go:52-66) on every path: `code` is
the last whitespace-delimited field when it is exactly six ASCII
digits; `clean` is the original bytes with that field and the
whitespace immediately before it removed and nothing else changed (no
field re-joining); a message that is only a code yields clean "" and
present true. Every durable surface is named: the inbox record's
`text` and the question's `answer.text` are `clean`; the Publish
Intent is `Intent{Verb: "inbox", Targets: [<record path>], Args:
{provider, destination, messageId, updateId, outcome}}` and carries
no text (journal.go:57-65 persists the Intent before the transaction);
the commit message is `channel inbox <provider>-<messageId>`; the
listener's log lines carry update ids, outcomes and phases and never
text; the fake provider's journal (fixtures only) lives under
artifacts and is git-ignored; the validator's `channel-secret` row
refuses any six-digit field in any text field on the tree, so a code
that slipped past StripCode cannot land. The TOTP secret and the bot
token are read from metasystem.conf.local at each iteration and are
held only in memory. `channel status` prints neither.

### FCG-EVIDENCE-12: what proves it

Fixtures extend scripts/agents/channel-fixtures.sh with a second export
clone on the same bare origin and the same fake bot (telegram face),
both listening as separate processes. The fake (internal/channel/fake)
grows what the fixtures need (FCG-C-12): it keeps a confirmed offset
per bot token in its journal, so getUpdates without offset returns
only unconfirmed updates and offset c forgets everything below c;
timeout T blocks up to T seconds until an update arrives; a control
file `<fake root>/control.json`, re-read per request, carries
`conflict: [{listener, remaining}]` (the next `remaining` getUpdates
from that listener get 409 with the configured description),
`deliverOnlyTo: {<updateId>: [listener]}` (routing), and
`pauseBefore: [{listener, phase, until: <path>}]` (the fake blocks the
listener's next request of that phase until the file exists). The
listener names itself by `channel.fake.listener` in its conf.local.
Failure points are injected by the environment
`METASYSTEM_CHANNEL_FAIL_AT=<phase>`, read by Listen into
PollConfig.FailurePoint; the phases are `before-publish`,
`after-publish` (= before-confirm), `before-post`, `after-post`.

- FCG-12-ONE-COMMIT: one human reply with the token, two listeners;
  exactly one inbox record, one answer row, one receipt post; the fake
  journal shows both listeners receiving the update and one Confirm
  after the commit.
- FCG-12-DIE-BEFORE-CONFIRM: A runs with FAIL_AT after-publish and
  exits; B receives the same update, its Publish is idempotent
  confirmed, it confirms; one record.
- FCG-12-DIE-BEFORE-COMMIT: A with FAIL_AT before-publish; B commits;
  A's restart receives nothing new.
- FCG-12-REPLAY-ACROSS-MACHINES: two messages, one code, routed one to
  each listener; the second record is `replayed`, one answer row.
- FCG-12-UNTHREADED: with one open question, an unthreaded reply
  carrying the token matches; one carrying only "ok" is `unbound` and
  the token hint is posted once; with two open, `unmatched` and the
  list is posted.
- FCG-12-NO-TOKEN-BINDS-NOTHING: a threaded verified "no" to a stop
  question; the answer row's reason is "no"; `goal resume
  --approved-ref` refuses.
- FCG-12-TOKENLESS-MACHINE: a third clone with no bot token asks and
  waits; one listener posts the question once (intent visible in the
  ledger before the post); the reply reaches the clone through
  `channel wait`.
- FCG-12-POST-CRASH: a listener with FAIL_AT after-post dies holding a
  posting intent; after posting-stale-sec (set to 2 s in the fixture)
  the other listener takes over and posts; the ledger holds one thread
  ref and the fake journal two posts.
- FCG-12-CONFLICT-IS-NOT-ERROR: the fake returns 409 "terminated by
  other getUpdates request" to one listener twice; it retries after
  jitter and its heartbeat counts two conflicts; a webhook 409 still
  fails.
- FCG-12-BUDGET-TWO-STEP: a budget question with a tuple, the token
  answered, listener A with FAIL_AT after-publish on the answer commit;
  B's open-work pass runs the approval; one approve row.
- Unit tests in internal/goal (ValidateChannelTree's refusal table row
  by row, the deterministic opid, the idempotent Mutate, the step check
  on the tip, the removed append) and internal/channel (StripCode,
  the batch-prefix Confirm rule, rule (b), the migrate refusal).

### FCG-BUILD-13: order and budget

Tier 3, Wido's box. Build order, each step landing alone: (1) the
three fences and ValidateChannelTree with the two schemas, plus the
deterministic opid helper — no writer, no live behaviour changes
(FCG-C-02); (2) the provider contract (Ack, Confirm, ErrBusy), the
telegram and slack adapters, the fake's controls — the local poll
unchanged in behaviour; (3) `channel ask` on the ledger, `channel
migrate`, the migrate refusal, the receive rule and idempotent commit,
matching and the atomic answer, FCG-ANSWER-11 — the cut-over, one
landing; (4) FCG-WORD-07; (5) FCG-POST-08 and the open-work pass, the
tokenless ask; (6) Listen, the RunLoop context, the heartbeat and
status; fixtures grow with each. One closing code review per two
build steps. Estimated: six build attempts of 25-45 minutes, three
review rounds (one design round spent).

## Dispositions (round 1, job fcg-design-cc1)

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| FCG-C-01 | accepted | ReadCommitGoals lists plans/goals/ recursively and the parser refuses nested files (validate.go:69-105, 405-419) | FCG-INBOX-02: inbox at plans/channel/, outside the goals prefix; ValidateChannelTree separate |
| FCG-C-02 | accepted | an old engine would refuse the tip | FCG-INBOX-02 location; FCG-BUILD-13 step 1 is fences and validator only |
| FCG-C-03 | accepted | Inbound had no update id, Receive one batch cursor | FCG-RECEIVE-03: Inbound.Ack, Provider.Confirm, the prefix rule stated on those |
| FCG-C-04 | accepted | AlreadyApplied is for the own opid only (txn.go:449-456) | FCG-INBOX-02: deterministic shared opid per message; idempotent confirmed is the ack authorisation; no new outcome word |
| FCG-C-05 | accepted | prose records, no refusal table | FCG-INBOX-02: two field tables, canonical serialisation, the ten-row refusal table, at-rest versus Mutate split |
| FCG-C-06 | accepted | rejected was per-machine but its post ids were needed fleet-wide | FCG-INBOX-02/FCG-COMMIT-05: rejected on the ledger question with by; ceiling on the shared list |
| FCG-C-07 | accepted | rule (b) proved the sender, not the question | FCG-MATCH-06: (b) binds only with the token or an option label; else `unbound` and a hint post |
| FCG-C-08 | accepted | Post has no idempotency key (telegram.go:99-135) | FCG-POST-08: intent, post, ref; stale takeover; fixture FCG-12-POST-CRASH |
| FCG-C-09 | accepted | the 150 s rule (poll.go:43, 276-278) contradicted the retention claim; no fleet signal | FCG-RECEIVE-03 loss window; FCG-STATUS-09 heartbeats and the honest limit |
| FCG-C-10 | accepted | RunLoop has no context and calls the phase after every tick | FCG-POLL-04 lifecycle: ctx, one goroutine, the after-tick call removed for the resident runner, config per iteration |
| FCG-C-11 | accepted | the atomic answer dropped m2's retry state | FCG-ANSWER-11: phases and approvalUlid/opid on the ledger record, any listener completes |
| FCG-C-12 | accepted | the fake could stage none of it | FCG-EVIDENCE-12: control file, confirmed offset, blocking timeout, env failure points |
| FCG-C-13 | accepted | the harvest marker was undefined | compact leaves this goal; answer-archive owns rotation |
| FCG-C-14 | accepted | two question worlds, one matcher | FCG-MIGRATE-10: no dual period, `channel migrate`, refusal until run |
| FCG-C-15 | accepted | SplitTOTP normalises and refuses code-only; the Intent was undefined | FCG-SECRET-15: StripCode contract, every durable surface named, the validator's secret row |
