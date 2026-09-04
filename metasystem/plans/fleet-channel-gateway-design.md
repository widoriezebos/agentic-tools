# fleet-channel-gateway — design: one bot, one inbox, first commit wins (revision 1)

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
ledger 1dee9746), tier 3, box 1d/10/1200m/1/3 from the norm. Every cite below
is re-read at main dbe1b41e. Companion goals: answer-archive (harvests
the inbox; sequenced after), channel-poll-not-automatic (tier 1, the
steward runs the poll), channel-budget-answer-binds-nothing (m2,
building: a budget answer re-approves the goal),
channel-code-verified-at-poll-time (done: the code is checked against
the message's own send time).

The shape in one paragraph. Today every channel record lives under
artifacts/agents/channel/, which .gitignore:1 keeps out of git: the
question, its thread, the cursor, the replay register and the unmatched
file are all per checkout (internal/channel/question.go:68). So only the
machine that asked can match a reply, only that machine's poll can
verify the code, and two machines on one bot token fight over
getUpdates. This design moves the two records that matter, the question
and the inbound message, onto the goal ledger's branch, committed
through the ledger's own transaction engine (internal/goal/txn.go: fetch
the tip, rebuild on it, compare-and-swap push, retry under benign
advancement). The push race that already arbitrates every goal verb
arbitrates the inbox too: the first machine whose commit of a message
lands wins; every other machine rebuilds on the new tip, finds the
message committed, and does nothing. Nothing is confirmed to the
provider until the commit is durable, so a machine that dies mid-way
confirmed nothing and the message comes again.

### FCG-PRINCIPLE-01: the inbox is the ledger, the race is the arbiter, there is no leader

The shared inbox is a directory on the goal ledger branch
(goal.sync-branch, refs/heads/main by default, internal/goal/txn.go:49-61),
written only through goal.Publish (txn.go:508) with the same
Goal-Transaction trailer, the same CAS publish (txn.go:347) and the
same classification by refetch. Every machine that holds a provider
token may receive; every machine that receives commits; the commit is
idempotent by provider message id (FCG-INBOX-02). No machine is
designated, elected or leased. A machine without a provider token
never receives and never posts; it reads its answers from the inbox
like everyone else (FCG-READ-08).

What this rules out, by Wido's word: a gateway host, a lease and its
failover, an adaptive leader, a back-off that converges on one poller.
A conflict from the provider (Telegram's 409 "terminated by other
getUpdates request") is not an error and not a signal to stand down;
it is the ordinary sound of two machines polling one bot, handled by
the jittered retry of FCG-POLL-04 and nothing else.

### FCG-INBOX-02: two record kinds, one directory, written once each

Location: `plans/goals/channel/` beside the live goals (goalsPrefix,
internal/goal/validate.go:39), so the records ride the ledger branch,
the transport (scripts/agents/sync-transport.sh:27-31) and `goal fetch`
unchanged. ReadCommitGoals (validate.go:405-419) lists only goal files
and records/goals; ValidateTree never sees the channel directory, and
the parse of a goal file is untouched. A new validator,
ValidateChannelCommit, runs in the same Validate hook and checks the
two shapes below.

Question record: `plans/goals/channel/questions/<qid>.json`, the
Question of internal/channel/question.go:39-55 minus Answer, Rejected
and Undelivered (those are per-machine working state and stay in
artifacts), plus `AskedBy` (machine) and `Destination`. Written by
`channel ask` in one Publish together with the goal's `ask` history row
(verbs.go:43-64 today writes the row alone); the file and the row land
in one commit or neither. State moves open -> answered -> closed by
later Publish calls from the committing machine (FCG-COMMIT-05) and
from `channel close`.

Inbound record: `plans/goals/channel/inbox/<destination>/<provider
message id>.json`: provider, destination, message id, reply-to id,
sender user id, sent-at, text with the code REMOVED and the verified
step number in its place (`Step`), the matched question id or the
string `unmatched`, the verification outcome (verified, wrong-user,
no-code, bad-code, stale, replayed), the receiving machine and the
receive time. The provider message id is the whole idempotence key:
the Mutate of the Publish reads the tip's tree first, and if the path
exists it returns goal.AlreadyApplied (txn.go:457), which Publish
reports as confirmed-by-another. The secret never enters the record:
the code digits are stripped before the record is built (SplitTOTP,
internal/channel/totp.go:52-66), and the register's replay check uses
the step, not the digits.

Rotation: the inbox is a working queue. Records whose question is
closed and older than 30 days are removed by `channel compact`, one
Publish per run, after answer-archive has harvested them (that goal's
own rule; until it lands, nothing is removed).

### FCG-RECEIVE-03: receive, commit, then confirm, per provider

Telegram. getUpdates carries no offset until a commit is durable
(today the cursor is sent as offset from the local file,
internal/channel/telegram/telegram.go:203-208, and advanced after a
local write, poll.go:247-257). The new order for one received update:
(1) build the inbound record, verify identity and code (FCG-COMMIT-05),
(2) Publish it, (3) on confirmed OR already-applied, call getUpdates
with offset = update_id+1, timeout 0, limit 1, which is Telegram's
acknowledgement, (4) then write the local cursor. A machine that dies
after (2) and before (3) has committed the message; the next poller on
any machine receives the same update, finds it committed at step (2)
and performs (3). A machine that dies before (2) confirmed nothing.
Unconfirmed updates expire on Telegram's side after about 24 hours,
which is harmless once committed and a loss only if no machine polled
for a day, which FCG-POLL-04 makes visible.

Slack. Receive today pages conversations.replies per open thread
(internal/channel/slack/slack.go:96-146) with a per-root cursor. Under
the rule it commits each reply as an inbound record and advances the
per-root cursor only past committed replies. No Socket Mode or Events
work in this goal; Slack stays a pull face.

Email is out of scope here; the rule is recorded for it in the goal
(shared IMAP, first commit wins, mark-seen only after commit).

### FCG-POLL-04: one long poll per machine, jittered, no leader

A resident listener, not the tick. Today channelphase.Run is a 15-second
duty after the steward tick (internal/steward/runner.go:136-140,
cmd/metasystem/steward_verbs.go:273-277); a 30-50 second long poll
cannot live inside it. The listener is a goroutine started once by
RunLoop (runner.go:70) with its own context, running
`channel.Listen(ctx, cfg)`: loop { getUpdates with timeout T (default
40 s, config `channel.long-poll-sec`), no offset; for each update,
FCG-RECEIVE-03; sleep base + jitter (base `channel.poll-base-sec`
default 30, jitter uniform in [0, base)) }. A 409 conflict sleeps the
same base + jitter and loops; it is logged at debug level only and
counted in the listener's status (FCG-STATUS-09). The one-shot
`metasystem steward tick` keeps calling channelphase.Run, which now
performs a single short poll (timeout 0) under the same receive rule,
so a machine without the resident steward still receives when ticked.
`channel poll` stays as the manual single pass.

The telegram adapter distinguishes the two 409 texts: "terminated by
other getUpdates request" becomes ErrBusy (a new ErrorKind,
channel.go:47-51), "webhook is active" stays ErrReceiveFailed with
today's message (telegram.go:83-85).

Rate: at base 30 s and T 40 s, an idle machine makes one request per
30-70 s; N machines make N such streams. Telegram publishes no
getUpdates limit; the 409 dance costs one refused request per
overlap. No machine ever changes its base in response to another.

### FCG-COMMIT-05: identity and code are checked by the committing machine, against the ledger

Before Publish, the receiving machine verifies: sender user id equals
`channel.human.<face>.user-id`; the last field is a six-digit code
(SplitTOTP); the code verifies at the message's own sent-at with one
step of slack (VerifyTOTP, totp.go:35-50, on SentAt per
channel-code-verified-at-poll-time); the step is unused. The replay
register moves from artifacts/agents/channel/totp-consumed.json
(poll.go:365-388) into the Mutate: a code step is consumed if any inbox
record at the tip carries the same Step with a different message id;
that check runs on the fetched tip inside the transaction, so two
machines committing two messages with one code cannot both win: the
second rebuilds on the first's commit, sees the step, and its record
is written with outcome `replayed`. Every outcome is committed
(the record exists whether verified or not); only `verified` records
carry a matched question forward. Rejections are posted by the
committing machine, at most three per question, with today's text
(poll.go:200-209).

### FCG-MATCH-06: threaded first, then the one open question, never lost

Matching runs on the committing machine against the questions at the
fetched tip, not against a local file. (a) A reply whose reply-to id
is a question's thread root, or one of that question's posted
rejection ids, matches that question. (b) An unthreaded message from
the enrolled human matches the single open question on that
destination when exactly one is open. (c) With several open, the
record is committed `unmatched` and the committing machine posts one
message listing the open questions by feature name with "reply to the
one you mean"; with none open, the record is committed `unmatched` and
nothing is posted. Rule (b) is the fix for the three lost answers of
2026-09-04 (11:32Z, 12:02Z, 12:07Z; poll.go:150-160 filed them to
unmatched.jsonl unverified).

A matched, verified record advances the question in the same commit:
State answered, Answer{Text, UserID, Ref, At, Step}. The goal `answer`
history row (verbs.go:112-151) is written in that same Publish, so the
question file, the inbox record and the ledger row are one atomic
change set; the ordering problem of the human-carried-landing critique
(a word recorded in one place and not the other) does not arise.

### FCG-WORD-07: the token binds only when it is in the human's own words

goal.Answer today appends the asked token to the reason when the text
lacks it (verbs.go:120-123), and AuthenticatedChannelApproval
(verbs.go:67-93) then finds the token and binds: an authenticated "no"
approves. The append is removed. The reason is the human's text,
verbatim. AuthenticatedChannelApproval keeps its contiguous-fields
rule (verbs.go:96-108) over the text alone. A verified answer whose
text lacks the token is recorded as the human's word and binds
nothing; the question closes as answered; the asking machine, reading
it (FCG-READ-08), reports the words verbatim and asks again if it
needs the token. renderQuestion's prompt (question.go:209-225) already
says "Reply in this thread with this token verbatim".

This point and channel-budget-answer-binds-nothing (m2) touch the same
verb from two sides: m2 makes a verified budget answer WITH the token
re-approve; this point makes an answer WITHOUT the token bind nothing.
The build here lands after m2's chain and rebases on it.

### FCG-READ-08: every machine reads answers from the inbox; posting is direct

`channel wait` today loops on the local question file
(cmd/metasystem/channel_verbs.go:174-202). It now runs `goal fetch`
every interval (default 30 s, `--interval`) and reads the question at
the fetched tip; a machine with no provider token can ask (the ask is
a ledger commit plus, when the machine has a token, a post) and wait.
When the asking machine has no token, the post is made by the first
listener that sees an open question with no Thread at its tip: it
posts, then commits the Thread into the question record (a second
Publish; a race here is benign, the loser's rebuild finds the Thread
set and skips). Status posts (`channel status --post`, report.go)
stay per machine and direct; a machine without a token cannot post
status in this goal (the routing policy Wido named is deferred to a
follow-up goal; this one delivers the inbox and the read path).

### FCG-STATUS-09: what the listener shows and what it refuses

`channel status` gains a Listener block: last successful getUpdates,
last commit, conflicts in the last hour, updates received, oldest
unconfirmed update age. If no machine has confirmed anything for 12
hours while a question is open, the status post carries "the channel
has heard nothing for 12 h", so a whole-fleet outage is visible on the
channel itself. Nothing here refuses the human (HCL-PRINCIPLE-01
applies: the channel is the human's own path in).

### FCG-MIGRATE-10: questions asked before this lands

Questions open in artifacts at landing time keep their old life on the
machine that asked (their poll path stays for one release); `channel
ask` after landing writes ledger questions only. The local files
cursor.json, unmatched.jsonl and totp-consumed.json are read no more
after the first listener pass on each machine and are deleted by
`channel compact`.

### FCG-EVIDENCE-11: what proves it

Fixtures extend scripts/agents/channel-fixtures.sh with a second export
clone on the same bare origin and the same fake bot
(internal/channel/fake, telegram face), both listening.

- FCG-11-ONE-COMMIT: one human reply, two listeners; exactly one inbox
  record, one answer row, one receipt post; the fake journal shows two
  getUpdates deliveries of the update and one offset confirmation
  after the commit.
- FCG-11-DIE-BEFORE-CONFIRM: listener A is killed between Publish and
  the offset call (FailurePoint "before-confirm"); listener B receives
  the same update, finds it committed, confirms; no duplicate record.
- FCG-11-DIE-BEFORE-COMMIT: A is killed before Publish; B commits; A's
  restart receives nothing new.
- FCG-11-REPLAY-ACROSS-MACHINES: two messages, one code, received by
  different machines; the second record is `replayed`, no answer row.
- FCG-11-UNTHREADED-ONE-OPEN: an unthreaded reply with one open
  question matches and is verified; with two open, the record is
  unmatched and the list is posted.
- FCG-11-NO-TOKEN-BINDS-NOTHING: a verified "no" to a stop question;
  the answer row's reason is "no"; `goal resume --approved-ref` refuses.
- FCG-11-TOKENLESS-MACHINE: a third clone with no bot token asks and
  waits; a listener posts the question; the reply reaches the clone
  through `channel wait`.
- FCG-11-CONFLICT-IS-NOT-ERROR: the fake returns 409 "terminated by
  other getUpdates request" to one listener; it retries after jitter
  and the status counts one conflict; a webhook 409 still fails.
- Unit tests in internal/channel and internal/goal for the validator,
  the idempotent Mutate, the step check on the tip and the removed
  append (a test asserting the old append is gone).

### FCG-BUILD-12: order and budget

Tier 3, Wido's box. Build order: (1) ledger records and validator with
`channel ask` writing both (goal + channel packages); (2) receive rule
and idempotent commit in Poll, the 409 split, the offset-after-commit;
(3) matching and the atomic answer; (4) FCG-WORD-07, after m2's
channel-budget-answer-binds-nothing lands; (5) the listener and the
runner goroutine; (6) `channel wait` on the fetched tip and the
tokenless-machine post; (7) status block and compact; fixtures grow
with each. One design critique round before (1); one closing code
review per two build steps. Estimated: seven build attempts of 25-45
minutes, three review rounds.
