# fleet-channel-gateway — design: one bot, one inbox, first commit wins (revision 4)

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

Revision 3 answers critique round 2 (job fcg-design-cc1-r2, twelve
material findings at main e526a54e; the dispositions are in
plans/fleet-channel-gateway-dispositions-round2.md, one table per
file as `validate critique-closed` requires). What changed: the
shared opid goes — every inbox commit runs under the committing
machine's own opid and the second machine's Mutate classifies the tip
as lost to the winner it names, so no clone's journal can ever block
its own next message and recovery has one rule (abandon; the provider
re-delivers); the listeners directory gets its schema; a verified
reply to a question no longer open is the outcome `late`, recorded
and never binding; the answer object, the posting vocabulary, the
transition matrix and every operation's opid are written out; an
unthreaded reply binds by its token alone, with any number of
questions open; the asker posts through the same intent protocol as
everyone else and a taken-over poster records its orphan; the
approval step names its proof, its arguments, its fence and its
recovery; the cut-over is one ordered fleet action with the old
engines stopped first and the replies of the window resurfaced by
`channel migrate`; the code is recognised with trailing punctuation
and a six-digit fact in the middle of a sentence is a fact; the fake
tells listeners apart by token; the build has five steps and the
resident listener lands with a steward restart and a bounded stop.

Revision 4 answers critique round 3, the failsafe round (job
fcg-design-cc1-r3, thirteen material findings at main 7bcee9fb; the
dispositions are in plans/fleet-channel-gateway-dispositions-round3.md).
Ten findings reopened prose and are answered here; three are
fixture-expressible and became obligation rows in that file. What
changed: the listener runs the ledger's own recovery call itself, so
a pushed entry left by a failed confirming refetch unblocks within
one push deadline; the approve recovery case goes (recovery cannot
rebuild it; the standing approval intent is the re-run); the
duplicate-approval predicate binds the goal's approve event to this
question by its ChannelContext; a migrated `matched` answer is joined
to the goal by the legacy record's saved Opid, never re-derived; a
repeated or mid-sentence code is masked by the listener before the
record is built; the old Poll keeps sending its offset so step 2
tells Telegram nothing new, and WORD-07 folds into the cut-over,
which makes four steps; a verified record may name `unbound` or
`unmatched`; the stop makes no graceful-join promise — restart
force-kills after two seconds and the listener's first act is
recovery; closed/null with a reason of its own is legal, so `channel
close` and a legacy question closed unanswered both fit; the matrix
gains its orphan-post row, splits the answer row so a non-budget
answer rests in approved, and admits a `late` rejection on a closed
question; the token match is a contiguous field sequence, as the
goal's own rule already is; `channel skip` is written out step by
step and `channel ask` gets its failure points. Every cite in this
revision was re-read at main 7bcee9fb.

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
message's record already there under the winner's opid, and its own
transaction classifies itself lost to that winner — a classification
the ledger already has (LostToCompetitor, txn.go:443), and the one
that authorises the loser to confirm. Nothing is confirmed to the provider
until the commit is durable, so a machine that dies mid-way confirmed
nothing and the message comes again.

### FCG-PRINCIPLE-01: the inbox is the ledger, the race is the arbiter, there is no leader

The shared inbox is a directory on the goal ledger branch
(goal.sync-branch, refs/heads/main by default, internal/goal/txn.go:49-61),
written only through goal.Publish (txn.go:508) with the same
Goal-Transaction trailer (txn.go:283), the same CAS publish
(txn.go:347) and the same classification by refetch. Every machine
that holds a provider token may receive; every machine that receives
commits; the commit is idempotent by the provider message id, which
is the record's path (FCG-INBOX-02). No machine is designated, elected
or leased. A machine without a provider token never receives and never
posts; it reads its answers from the inbox like everyone else
(`channel wait`, FCG-POST-08).

What this rules out, by Wido's word: a gateway host, a lease and its
failover, an adaptive leader, a back-off that converges on one poller.
A conflict from the provider (Telegram's 409 "terminated by other
getUpdates request") is not an error and not a signal to stand down;
it is the ordinary sound of two machines polling one bot, handled by
the jittered retry of FCG-POLL-04 and nothing else.

### FCG-INBOX-02: three record kinds, one directory, written once each

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
| kind | string | yes | one of budget-above-norm, fork, reserved-decision, stop, other — Ask's own vocabulary (question.go:164), so every legacy kind migrates |
| machine | string | yes | the asking machine (goal.ResolveMachine) |
| lineage | string | yes | the asking lineage; `migrated` for a record `channel migrate` converted |
| opid | string | yes | the opid of the commit that created the record (the ask or the migrate) |
| openedAt | RFC 3339 UTC, second precision | yes | |
| facts | []string | yes, may be empty | |
| options | []{label, consequence} | yes, may be empty | |
| recommendation | string | yes, may be "" | |
| wants | string | yes, may be "" | the token; required non-empty for kind stop and budget-above-norm (Ask's existing rule) |
| budget | object or absent | budget-above-norm only | the goal.Budget tuple; Ask's validateQuestionBudget rule unchanged |
| destination | string | yes | the destination name (channel.destination) |
| thread | ref or null | yes | the question post; null until posted |
| orphanPosts | []ref | yes, may be empty | posts whose intent was taken over before their ref landed (FCG-POST-08) |
| posting | {kind, by, at} or null | yes | an outbound step in flight (FCG-POST-08); `kind` one of question, rejection, list, receipt, silence, approval; `by` a machine; `at` RFC 3339 UTC |
| state | string | yes | open, answered, closed |
| answer | object or null | yes | below |
| rejected | []{ref, reason, at, postRef, by} | yes, may be empty | shared; at most three entries with postRef non-null |
| factsDigest | string | yes | as today |
| closedAt | RFC 3339 UTC or absent | closed only | |
| closedBy | string or absent | closed only | a machine |
| closedBecause | string or absent | closed only | answered, or the `channel close` reason text |

A `ref` everywhere in this design is `{provider: string, id: string,
threadId: string}`, all three required, `threadId` "" when the post
is a thread root. A `rejected` entry: `ref` the inbound message's
ref, `reason` string, `at` RFC 3339 UTC, `postRef` ref or null, `by`
machine.

`answer`, every key required, null where noted:

| key | type | meaning |
|---|---|---|
| text | string | the human's words with the code removed |
| userId | string | |
| ref | ref | the inbound message |
| at | RFC 3339 UTC | the message's sentAt |
| step | integer | the verified TOTP step |
| inboxId | string | the inbox record's basename id part (`<provider>-<messageId>`) |
| opid | string | the opid of the answer commit (FCG-MATCH-06) |
| phase | string | recorded, approved, receipted |
| approvalUlid | string or null | non-null only when kind is budget-above-norm, text equals wants and budget is present (FCG-ANSWER-11) |
| receipt | string or null | the text posted back; null while phase is recorded |
| receiptRef | ref or null | null until posted |

The per-machine `undelivered` counter leaves the record: delivery is
FCG-POST-08's business.

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
| step | integer or null | yes | the verified TOTP step; non-null exactly when outcome is verified, late or replayed, whatever `question` says |
| outcome | string | yes | verified, late, wrong-user, no-code, bad-code, stale, replayed, unverified-migrated, skipped — what FCG-COMMIT-05 found about the sender and the code |
| question | string | yes | what FCG-MATCH-06 found about the question, independent of outcome (FCG-C-05): a question id when rule (a) threaded the message to one (any outcome) or rule (b) bound it (verified only); `unbound` when verified, unthreaded, without a token and exactly one question open; `unmatched` otherwise. A verified record whose question is `unbound` or `unmatched` is legal and advances nothing |
| opid | string | yes | the opid of the commit that wrote this record |
| receivedBy | string | yes | the committing machine |
| receivedAt | RFC 3339 UTC | yes | |

One opid per commit, the committing machine's own (FCG-C-16). Every
Publish in this design — inbox, ask, migrate, posting intent, ref,
approval, receipt, heartbeat — runs under an ordinary opid:
`Opid(<fresh ULID>, <this machine>, <this lineage>)` (file.go:1494,
verbs.go:182), so requestForEntry's derivation (recover.go:209-229)
holds for every entry and no two attempts on one clone ever share an
opid; the journal (journal.go) keeps its one-entry-per-opid law
untouched. The opid of the commit that created a record is written
into the record (`opid`) so that any reader can join the record to the
history line that made it. Idempotency across machines is the
record's path, exactly Wido's "idempotent by provider message id":
the inbox Mutate on a rebuilt tip does one of three things — the path
is absent: write the record (and the question advance of
FCG-MATCH-06) and return the changes; the path is present and
TrailerPresent(tip, record.opid) (txn.go:379) holds: return
`goal.LostToCompetitor{Winner: record.opid}` (txn.go:443), which
Publish classifies as OutcomeLost with Detail `winner: <opid>`
(txn.go:750-755) — for an inbox commit this is the ordinary second
place, not a failure, and it is the classification that authorises
the loser to confirm the update to the provider (FCG-RECEIVE-03); the
path is present without its trailer: return the error `inbox record
present without its transaction`, a rejection by name, and the update
is never confirmed until a human looks (FCG-C-04). AlreadyApplied is
returned only when the tip carries THIS operation's own opid (a
resumed push), which is the existing law (txn.go:449-456). A clone
whose earlier attempt on a message ended abandoned, rejected or
expired holds a terminal entry under an opid nobody will ever use
again; its next receipt of the same update is a new opid and a new
transaction that finds the winner and confirms. Recovery
(recover.go:56-140) needs no new rule: a dead or expired entry whose
commit landed is confirmed by the existing PostconditionPresent rule
(recover.go:64-92); one whose commit did not land reaches
completeFromIntent, where requestForEntry's default for a verb it
does not rebuild (`inbox`, and every other verb of this design)
terminalises the entry rejected with "re-runs from its own entry
point" (recover.go:184-194, 403) — the provider re-delivers what was
never confirmed, and the next listener commits or loses as above.
The one wedge that is not a dead owner's (FCG-C-16): a confirming
refetch that fails leaves this clone's entry PUSHED with its outcome
unknown (txn.go:654-675), and PushedBlocking then refuses every other
opid on this clone (txn.go:508-516, journal.go:423-437) — a live
listener that only logged the error would never commit again. So the
listener runs recovery itself: once before its first iteration, when
a Publish returns the pushed-blocking refusal, and at the start of
every iteration while its last Publish did, Listen (and `channel
poll`) calls goal.RecoverWithPolicy(endpoint,
dispatchcore.GoalRecoveryPolicy{Now: now}) — exactly the call `goal
recover` makes (goalsync_verbs.go:498; the steward tick makes no goal
recovery call of its own, runner.go:95-140) — which classifies the
clone's own pushed entry by refetch: confirmed when the commit is on
the tip, expired once the entry's own deadline (the push deadline,
60 s) has passed, and left retrying until then (journal.go:475-503).
The wedge therefore lasts at most one push deadline plus one
iteration, is logged as `journal blocked by <opid>; recovery ran:
<action>`, and never needs a restart. The answer
history row (FCG-MATCH-06) is written in the same commit as the
inbox record and therefore carries the same opid as the record's
`opid`.

The secret never enters either record: the code is removed before the
record is built (FCG-SECRET-15), and the replay check uses the step.

Listener record: `plans/channel/listeners/<machine>.json`
(FCG-STATUS-09; FCG-C-17), `<machine>` the value goal.ResolveMachine
returns, which is already a path-safe token.

| key | type | required | meaning |
|---|---|---|---|
| machine | string | yes | equals the basename |
| engine | string | yes | the engine digest (steward.installDigest) the listener runs |
| lastReceiveAt | RFC 3339 UTC | yes | the last getUpdates that returned without error |
| lastConfirmAt | RFC 3339 UTC or null | yes | |
| conflictsLastHour | integer | yes | |
| updatedAt | RFC 3339 UTC | yes | |
| opid | string | yes | the opid of the commit that wrote this version |

A heartbeat Publish overwrites the machine's own file (create or
replace, never another machine's); its Mutate refuses
`channel-transition` when the tip's `updatedAt` is younger than the
one it read (a second heartbeat of the same machine landed first).

Rotation is not this goal's (FCG-C-13): the inbox only grows here, at
about one kilobyte per message; answer-archive owns harvest and
removal, and until it lands nothing is removed. A `channel compact`
verb is not built.

ValidateChannelTree refuses, each by name, and a refusal abandons the
publishing transaction exactly as a goal refusal does:

| code | condition |
|---|---|
| channel-unknown-path | a file under plans/channel/ that is not `questions/<qid>.json`, `inbox/<destination>/<provider>-<messageId>.json` or `listeners/<machine>.json` |
| channel-json | not a JSON object, an unknown key, a missing required key, a key of the wrong type, a nullable key null where the table says non-null |
| channel-id-mismatch | basename does not equal id / provider-messageId / machine |
| channel-goal-missing | question.goal has no goal file on the tip |
| channel-kind | kind, state, outcome, phase, posting.kind outside their vocabularies |
| channel-token-missing | kind stop or budget-above-norm with empty wants |
| channel-budget | budget present on a non-budget kind, or absent or incomplete on budget-above-norm, unless lineage is `migrated` (a legacy record loads without a tuple, channel-poll-refuses-legacy-budget-questions) |
| channel-answer-state | state answered with answer null; state closed with closedBecause `answered` and answer null; state closed with closedBecause other than `answered` and answer non-null (FCG-C-19: `channel close` and a legacy question closed unanswered are closed/null with their own reason); state open with answer non-null; answer non-null with phase recorded and receipt non-null, or phase approved and receipt null, or phase receipted and receiptRef null; state closed with closedAt, closedBy or closedBecause absent; an inbox record with outcome verified whose `question` is a question id (not `unbound` or `unmatched`) and whose question's answer is null or has an inboxId other than this record's — a verified record bound to a question is that question's answer, and a second verified reply is outcome `late`, not verified (FCG-C-05) |
| channel-rejection-cap | more than three rejected entries with postRef non-null |
| channel-secret | the LAST whitespace-delimited field of text or answer.text, after trailing `.,;:!?` are dropped, is six ASCII digits (the field StripCode would have removed, FCG-SECRET-15); any field of six ASCII digits anywhere in facts, recommendation, rejected[].reason or receipt (machine-written text never carries a code) |

The validator is at rest: it reads one tree. Create-once and the
legal transitions are the Mutate's job on the fetched tip, and the CAS
push makes the Mutate's reading of the tip the only reading that
lands. Every question Mutate in this design names its transition in
the matrix below; on every attempt it re-reads the question at the
rebuilt tip and computes the tuple `(state, answer.phase or null,
posting, thread is null, receiptRef is null)`. If the tuple equals
the transition's FROM row it applies the change; if the tip carries
this operation's own opid in history it returns AlreadyApplied
(txn.go:449-456); if the tuple equals the TO row and the field the
transition sets names another opid (answer.opid, posting.by,
thread/orphanPosts ref written under another intent) it returns
LostToCompetitor with that opid; any other tuple is the rejection
`channel-transition: <qid> is <tuple>, expected <FROM>` (FCG-C-05).
An answered-to-answered rewrite is therefore never legal: no
transition has an answered FROM row that writes `answer`.

| transition (owner) | FROM (state, phase, posting, thread null, receiptRef null) | TO |
|---|---|---|
| ask (asker) | no file | (open, null, {question,me,now}, true, true) |
| migrate (any) | no file | the legacy record's mapped state (FCG-MIGRATE-10), posting null |
| post-ref question (the poster named in posting) | (open, null, {question,by=me}, true, true) | (open, null, null, false, true) |
| answer, budget with tuple (receiver) | (open, null, any, false, true) | (answered, recorded, unchanged posting, false, true); answer.opid = this commit, approvalUlid set, receipt null — only when kind is budget-above-norm, text equals wants and budget is present (FCG-ANSWER-11) |
| answer, every other (receiver) | (open, null, any, false, true) | (answered, approved, unchanged posting, false, true); answer.opid = this commit, approvalUlid null, receipt = the text FCG-ANSWER-11 names for the case (FCG-C-20: a non-budget answer never rests in recorded) |
| approve-intent (any listener) | (answered, recorded, null, false, true) | (answered, recorded, {approval,me,now}, false, true) |
| approved (the machine named in posting) | (answered, recorded, {approval,by=me}, false, true) | (answered, approved, null, false, true); receipt set |
| receipt-intent (any listener) | (answered, approved, null, false, true) | (answered, approved, {receipt,me,now}, false, true) |
| receipted (the poster) | (answered, approved, {receipt,by=me}, false, true) | (closed, receipted, null, false, false); closedAt, closedBy, closedBecause=answered |
| rejection/list/silence intent (any listener) | (any state, any, null, false, any) — closed is legal only for a rejection with reason `late` (FCG-C-20) | posting {kind,me,now}, nothing else |
| rejection/list/silence ref (the poster) | posting {kind,by=me} | posting null; rejected[i].postRef set for a rejection; a list or silence post writes only its ref into `rejected` as an entry with reason `list` or `silence` and by — a list post hangs on the newest open question at the tip (the first the list names), a silence post on the oldest (FCG-STATUS-09); neither counts toward that question's three-rejection ceiling |
| take-over (any listener) | posting {kind,by=other,at older than posting-stale-sec} | posting {kind,me,now}; the intent's owner changes, nothing else |
| orphan-post (a poster whose intent was taken over, FCG-POST-08) | any tuple; the file present | orphanPosts + this ref, nothing else (FCG-C-20) |
| close (asker, `channel close`) | (open, null, null, any, true) | (closed, null, null, unchanged, true); closedAt, closedBy, closedBecause = the reason, never `answered` |

`state` and `answer.phase` together are the state machine:
open/null → answered/recorded → answered/approved → closed/receipted
for a budget question with a tuple, open/null → answered/approved →
closed/receipted for every other, plus open/null → closed/null by
`channel close` and by a legacy question migrated closed and
unanswered (FCG-MIGRATE-10). Every row's TO is a tuple some row's FROM
admits or a terminal one (closed/receipted, closed/null), and no two
rows share a FROM tuple with the same posting kind: the intent rows
are told apart by the kind they write, the ref rows by the kind they
find (FCG-C-20). Nothing under plans/channel/ is deleted by any
Mutate in this goal (answer-archive will own it).

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

Telegram. Receive keeps its cursor parameter and keeps sending it as
the offset when it is non-empty (telegram.go:203-218, unchanged), so
the old Poll, which passes the saved cursor, tells Telegram exactly
what it tells it today (FCG-C-02); the new listener passes "" and so
calls getUpdates with NO offset, limit 100, timeout T (FCG-POLL-04).
Receive returns each update as an Inbound with `Ack = update_id+1`
and `UpdateID`; Confirm(c) calls getUpdates with offset c, timeout 0,
limit 1 and discards the result — Telegram forgets every update below
c and the one it may return stays unconfirmed and comes again. The
listener handles a batch in update_id order: for each item, (1) build
the inbound record and verify (FCG-COMMIT-05), (2) Publish, (3) on
OutcomeConfirmed, or on OutcomeLost whose Detail names the winner
(the record is on the tip under that opid, FCG-INBOX-02), the update
is committed — continue; on anything else (rejected, abandoned,
expired, an error) stop the batch. After the loop it calls Confirm
once with the Ack of the last committed item, or the batch cursor when
every item committed, or nothing when none did; then it writes the
local cursor file for `channel status` only. A machine that dies after
(2) and before Confirm has committed the message; the next poller on
any machine receives the same update, its Publish is lost to the
committed record, and it confirms. A machine that dies before (2)
confirmed nothing. A poison update — one whose Publish is rejected by
name on every machine — stops every listener's prefix at that update;
the listener logs `inbox refused <updateId>: <detail>` on every pass,
`channel status` shows that line (FCG-STATUS-09) and the operator's
path is `channel skip --update <id>`, a new human verb at the
terminal (FCG-C-23), which does, in order: Receive with an empty
cursor and timeout 0 and find the update whose UpdateID is `<id>`
(refuse "update <id> is not pending at the provider" when absent);
build the record exactly as the listener would (provider,
destination, messageId, updateId, replyTo, userId, sentAt from the
update; receivedBy me, receivedAt now; opid this commit) but with
outcome `skipped`, text "", step null, question `unmatched`; Publish
it under the inbox Mutate's three branches — a record already on the
tip under another opid is the ordinary lost-to-winner and the skip
proceeds; a record present without its trailer (the poison case of
FCG-INBOX-02) is REPLACED, the one write in this design that
overwrites an inbox record, because the operator has looked and the
verb is the human's word — then Confirm(update_id+1) and print `skipped
<id>: <outcome of the Publish>`. The validator cannot refuse a record
the listener built by the rules of FCG-SECRET-15 and FCG-COMMIT-05 —
the rules are written so every outcome has a committable record — so
a poison update is a bug in this design, and the skip verb is the
honest fallback, not the mechanism; the fixture that proves the verb
stages the poison by committing a hand-written record without its
trailer directly on the ledger branch (FCG-EVIDENCE-12,
FCG-12-POISON-SKIP).

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
`go channel.Listen(ctx, repoRoot)` once, after its first tick has
returned, and removes the channelphase.Run call at runner.go:136-140;
Listen's first act is the recovery call of FCG-INBOX-02, so an entry
a killed listener left behind is classified before the new one
publishes anything; the one-shot
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
Listen returns when ctx is done and its current step has ended: a
getUpdates in flight is cancelled through ctx (the adapter passes it
to the request); a Publish in flight cannot be — PublishRequest
carries no context (txn.go:466-483) and its git subprocesses are not
cancellable; DefaultPublishDeadline (txn.go:44) bounds only the
CAS-refused retries — so no graceful join is promised (FCG-C-10).
The honest statement: `steward restart` (steward_verbs.go:543-566,
stopRunnerForReplacement at runner.go:478-492) sends SIGTERM and
force-kills the runner two seconds later, and the listener may die
inside a Publish or between a commit and its Confirm. Both are the
cases the design already handles: an update committed but not
confirmed comes again and is lost to its own record; an entry left in
the journal is classified by the recovery call Listen makes before
its first iteration (FCG-INBOX-02) — confirmed when the commit
landed, terminal rejected when it did not, and the provider
re-delivers. There is no `channel.stop-grace-sec`; RunLoop cancels
ctx and returns without waiting. Rollout: an armed runner is not
replaced by a landing — `steward arm` returns "already armed"
(runner.go:409-416) — so step 4 lands with `steward restart` on every
enrolled machine, under R-37's standing word for re-arming after an
engine rebuild; until a machine restarts it runs the old RunLoop and
its after-tick poll, which is the step-3 short poll and still
correct, only slower. `channel status` shows each listener's engine
digest from its heartbeat, so a machine that has not restarted is
visible from any machine.

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
is written with outcome `replayed`. A verified message that rule (a)
of FCG-MATCH-06 threads to a question whose state at the tip is not
open is the outcome `late` (FCG-C-18): the record is committed with
`question` = that question's id and its step consumed, the question
file is not touched, nothing binds, and the committing machine posts
"already answered: <feature name> was answered at <answer.at> with
'<the first forty characters of answer.text>'; a new question needs a
new ask" through FCG-POST-08 as a rejection entry (reason `late`),
under the same ceiling as every rejection. Every outcome is committed
(the record exists whether verified or not) and every record names
what the matcher found in `question` — a question id, `unbound` or
`unmatched` — independently of the outcome; only a `verified` record
whose `question` is a question id advances that question (FCG-C-05).
A rejection post for wrong-user,
no-code, bad-code, stale, replayed and late is made by the committing
machine through FCG-POST-08 with today's text (poll.go:200-209), only
if the question rule (a) or (b) names has fewer than three posted
rejections at the tip; the entry is appended to that question's
`rejected` with `by` = the committing machine, so the ceiling and the
post ids are shared (FCG-C-06). An unmatched rejection (no question)
is committed and not posted.

### FCG-MATCH-06: threaded first, then the one open question, never a stray

Matching runs on the committing machine against the questions at the
fetched tip. (a) A message whose replyTo is a question's thread id, or
one of that question's rejected[].postRef ids, its orphanPosts, or its
receiptRef, matches that question; if that question is open the
message is its answer (option labels count here: a threaded "yes" is
an answer to the question the human replied to), else the outcome is
`late` (FCG-COMMIT-05). (b) An unthreaded verified message binds by
the token alone (FCG-C-07): take the questions open on that
destination at the tip whose `wants` is non-empty and appears in the
text, after the code is removed, as a contiguous run of
whitespace-delimited fields equal to strings.Fields(wants) — a token
is several fields (ResumeApprovalToken is five, verbs.go:34-37;
FCG-C-22) — exact bytes, case-sensitive, once and only once
(containsContiguousFields, verbs.go:96-108, is the rule and the
function), with trailing `.,;:!?` dropped from the text's field that
matches the token's last field before the comparison, as for the
code. "goal=x resume elapsed=1d attempts=10 minutes=1200 active=1."
matches the five-field token; "goal=x resume" alone matches nothing.
Exactly one such question: it is
the answer. None, with exactly one question open: the record is
`unbound` and the committing machine posts, through FCG-POST-08 and
under the same three-post ceiling, "not recorded: I have one open
question (<feature name>); reply in its thread, or with its token
<wants>". None, with several open, or two or more tokens present: the
record is `unmatched` and the committing machine posts one message
listing the open questions by feature name and token with "reply to
the one you mean" (ceiling: once per inbound message, never more than
three such lists per hour per destination, counted from the `list`
entries in `rejected` at the tip). None open: `unmatched`, nothing
posted. Option labels never bind unthreaded: "yes" proves nothing
about which question, so the code proves the sender and only the
token proves the question. A question with empty `wants`
(reserved-decision, fork, other may have none) is matched by thread
only. The three lost answers of 2026-09-04 (11:32Z, 12:02Z, 12:07Z;
poll.go:150-160 filed them to unmatched.jsonl unverified) all carried
the token and would match under (b) whatever else was open.

A matched, verified record advances the question in the same commit
(the two `answer` rows of the matrix): state answered, answer{text,
userId, ref, at, step, inboxId, opid: this commit, phase: recorded
with approvalUlid set and receipt null for a budget question with a
tuple whose text equals wants, else approved with approvalUlid null
and the receipt text of FCG-ANSWER-11; receiptRef: null}. The goal
`answer` history row
(verbs.go:112-151) is written in that same Publish, so the question
file, the inbox record and the ledger row are one atomic change set
under one opid and the one Intent of FCG-SECRET-15 (verb `inbox`,
never rebuilt by recovery, so it carries no text). Two machines
committing the same message race on the inbox path, not on the
question: the loser's Mutate finds the record and returns lost before
it reads the question. The approval, when one is due, is a second
transaction (FCG-ANSWER-11).

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
(`channel wait`, FCG-POST-08), reports the words verbatim and asks again if it needs
the token. renderQuestion's prompt (question.go:209-225) already says
"Reply in this thread with this token verbatim".

m2's channel-budget-answer-binds-nothing compares the text to `wants`
by equality (poll.go:358) and is untouched by the removal of the
append; FCG-ANSWER-11 keeps its second step.

### FCG-POST-08: every post is intent, post, ref — the asker's too

Telegram's Post creates a new message per call and has no idempotency
key (telegram.go:99-135), so a post made before its intent is durable
can be made twice by two machines and its id lost by a crash
(FCG-C-08). One protocol for every post — the question post (by the
asker when it holds a token, by the first listener otherwise), the
rejection, list and silence posts and the receipt: (1) Publish the
intent `posting: {kind, by: <machine>, at}` on the question record
(the intent transitions of the matrix; `channel ask` writes the
question and its intent in one commit) — the Mutate refuses
`channel-posting-busy` when the tip already carries a posting younger
than `channel.posting-stale-sec` (default 300); (2) post; (3) Publish
the returned ref into its field (thread, rejected[i].postRef,
receiptRef) and clear `posting`, a transition whose FROM row requires
`posting.by` to be this machine. Every Publish here runs under the
machine's own opid; the intent is the fence. Take-over: a listener's
open-work pass finds a posting older than posting-stale-sec and
publishes the take-over transition (posting.by becomes itself, at
now), then posts. The window is safe against a slow but alive poster
because the poster's step (2) is bounded by the adapter's HTTP
timeout — today telegram.New(nil) falls back to http.DefaultClient
(telegram.go:20-25), which has none; build step 2 gives Post, Confirm
and the short poll a per-request context deadline of
`channel.http-timeout-sec` (default 30) and the long poll T + 15 s —
and its step (3) by DefaultPublishDeadline (60 s, txn.go:44) for
its CAS retries — a hung git subprocess is not bounded by anything
(FCG-C-10), which is why the orphan rule below exists and why the
window is a heuristic, not a fence; 300 s is longer than both bounds
in sequence, and the fixture sets 2 s only after the poster is dead.
Should a poster nevertheless return after a take-over, its step (3)
Mutate finds `posting.by` is another machine (or null) and, instead
of the ref field, appends its ref to `orphanPosts` under the matrix's
`orphan-post` row (FROM: any tuple; TO: orphanPosts + ref) and logs
`post orphaned <ref>`; the ledger then knows every post that exists, the human sees
one duplicate, and a reply threaded to the orphan matches by rule (a)
through orphanPosts. There is no exemption for the asker: a crash
between post and step (3) leaves an intent that is taken over and
posted again, the earlier post becoming an orphan when its poster
returns, or a message with no ref anywhere when it does not — the one
case the ledger does not know, and a reply threaded to it is
unthreaded to the matcher and binds by rule (b) when it carries the
token.

`channel wait` today loops on the local question file
(cmd/metasystem/channel_verbs.go:174-202). It now runs `goal fetch`
every interval (default 30 s, `--interval`) and reads the question at
the fetched tip; a machine with no provider token can ask (the ask is
a ledger commit with `thread` null) and wait; the first listener whose
open-work pass sees an open question with `thread` null and `posting`
null posts it under this protocol. `channel ask` on a machine with a
token runs the same three steps as one process — ask commit (question
plus intent), post, ref commit — and reads
METASYSTEM_CHANNEL_FAIL_AT like the listener does (FCG-EVIDENCE-12):
`before-post:question` exits 3 after the ask commit, leaving an
intent a listener takes over; `after-post:question` exits 3 after the
post, leaving a post the ledger never learns of (FCG-C-12). Status posts (`channel status
--post`, report.go) stay per machine and direct; a machine without a
token cannot post status in this goal (the routing policy Wido named
is deferred to a follow-up goal).

The open-work pass, run by every listener after each batch and by
`channel poll` once: (i) open questions with `thread` null and
`posting` null → question intent, post, ref; (ii) answered questions
with phase recorded and `posting` null → approval intent, then
FCG-ANSWER-11; (iii) answered questions with phase approved and
`posting` null → receipt intent, post, receipted (the question
closes); (iv) `posting` older than posting-stale-sec → take over,
then finish that kind's step (2) and (3). Each item is one or more
transactions of the matrix and any listener may do it; the intent
fence means at most one machine works one question at a time.

### FCG-STATUS-09: what the listener shows and what it refuses

Heartbeat: `plans/channel/listeners/<machine>.json`, schema in
FCG-INBOX-02, published by a listener at most once per
`channel.heartbeat-min` (default 60) and only when its own getUpdates
succeeded since the last heartbeat; a machine that is down writes
nothing. `channel status` on any machine reads the listeners
directory at the fetched tip and prints, per machine, the age of its
last receive and its engine digest, and one fleet line: "the fleet
last heard Telegram <age> ago" from the newest lastReceiveAt; when
the local listener's last pass logged a refusal, the line `inbox
refused <updateId>: <detail>` (FCG-RECEIVE-03) follows. The health line
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

A new engine's Poll, Listen and `channel wait` read ledger questions
only; `channel ask` writes ledger questions only. The old engine
cannot be fenced from the ledger — it never reads plans/channel/ (that
is the point of FCG-INBOX-02) and its steward polls after every tick
(runner.go:136-140), acknowledging by offset whatever it received —
so the dual period is closed by order, not by code, and what the
order cannot close is resurfaced (FCG-C-14). The cut-over of build
step 4 is one fleet action, in this sequence and written into that
step's landing message: (1) every enrolled machine's steward is
disarmed (`steward disarm`, R-37's standing word covers the re-arm);
from this moment no old engine polls unless someone runs `channel
poll` by hand, which the seats are told not to; (2) the step lands;
(3) each machine pulls, rebuilds, runs `channel migrate`, re-arms;
(4) the first new listener starts. A reply sent between (1) and (4)
sits at Telegram unacknowledged and is received by the first new
listener. A reply the old engine did receive before (1) for a
question it did not know went to its local unmatched.jsonl unverified
(poll.go:146-176); `channel migrate` republishes every unmatched.jsonl
entry younger than `channel.stale-sec` as an inbox record with
outcome `unverified-migrated`, `question` unmatched, step null,
sentAt and userId from the filed Inbound, text = StripCode's `clean`
of the filed text (the old engine appended the whole Inbound, code
included, poll.go:170; the code is long expired by then, and the
rule is the rule), and the open-work pass posts the list message once for such
records so the human knows to repeat. Nothing sent to the channel is
lost silently; a reply from the window is at worst repeated.

`channel migrate` converts every local question (question.go:64-75)
in one Publish per question, own opid, Intent verb `channel-migrate`,
Targets the ledger path, Args {id, state}; a local question in state
closed is migrated too (answer-archive harvests from the ledger, so
the history is kept). The mapping, field by field: id, goal, kind
(all five kinds are legal, FCG-INBOX-02), machine, openedAt, facts,
options, recommendation, wants, budget (absent stays absent; the
validator's `channel-budget` row exempts lineage `migrated`),
thread, rejected (each entry's `by` = the migrating machine),
factsDigest as they are; lineage `migrated`; opid the migrate
commit's; destination the configured `channel.destination`;
orphanPosts []; posting null; state as it is. A legacy question in
state closed with a null answer (closed by hand) migrates as
closed/null with closedAt = the migrate's now, closedBy = the
migrating machine and closedBecause "closed before the ledger inbox"
(FCG-C-19). The answer: legacy phase `matched` (the answer row may or
may not be on the goal — advanceAnswer's Publish may have landed
before the phase was written): the legacy Answer carries the `Opid`
its goal.Answer ran under (question.go:36; the Opid is
Opid(ULID, machine, lineage) and a legacy record has no lineage
field, so the opid cannot be re-derived, FCG-C-14). Migrate reads it
back: TrailerPresent(tip, legacy.Opid) (txn.go:379) → the row landed;
the record migrates as phase recorded with receipt null and no
Answer call. Absent → the row never landed; migrate runs goal.Answer
under a FRESH own ULID and opid with the legacy text, ref, step and
user (the reason the human gave, verbatim, so FCG-WORD-07 holds), in
its own Publish before the migrate Publish, and the record's
answer.opid is that fresh opid; a migrate that dies between the two
finds the trailer on its re-run and does not call Answer again.
Legacy `recorded` (the goal row and any approval already done,
receipt text set) → phase approved; legacy `receipted` → phase
receipted, state closed, closedBecause answered; legacy `closed` with
an answer → the same. `approvalUlid` is the legacy ApprovalULID or
null; `inboxId` is `<provider>-<ref.id>` (the message exists at the
provider, its inbox record does not; the validator's answer-state
row is not applied when lineage is `migrated`). A migrate Mutate that
finds the question path present
returns LostToCompetitor with the record's opid (two machines never
hold one local question, so this is a re-run of migrate on one
machine after a crash); the migrating machine then renames its local
questions directory to `questions.migrated` and its unmatched.jsonl
to `unmatched.migrated.jsonl`. A new engine whose local directory
still holds any question refuses every channel verb but migrate and
status with "N local questions predate the ledger inbox; run channel
migrate" — the same shape as today's refusal on a legacy budget
question. cursor.json and totp-consumed.json are not read by the new
engine and are left in place.

### FCG-ANSWER-11: the budget re-approval keeps its two steps on the ledger

m2's channel-budget-answer-binds-nothing records the answer, then runs
goal.Approve as a second transaction, and keeps ApprovalULID, Receipt
and Phase on the local question so an interrupted poll retries
(poll.go:341-380, question.go:29-39). Under this design the same state
lives on the ledger question's `answer`, and the step is fenced like
a post (FCG-C-11). The answer commit (FCG-MATCH-06) writes phase
recorded and, when kind is budget-above-norm, text equals wants and
budget is present, `approvalUlid`: a ULID whose 48-bit time is
answer.at in milliseconds and whose 80 random bits are the first ten
bytes of SHA-256("approve:" + qid) — deterministic so that every
machine derives the same ULID, but the opid it yields is the
approving machine's own (Opid(approvalUlid, me, mine)), never shared.
Open-work pass item (ii), on any listener: publish the approval
intent (posting {approval, me, now}; the fence); if the question is
not a budget question with a tuple whose answer.text equals wants (a
migrated `matched` record of another kind, FCG-MIGRATE-10), publish
`approved` at once with the receipt text of the case below and no
Approve call; else read the goal at that tip — if its History holds
an event with Verb `approve`, AuthorityOutcome
VerifiedChannelAnswer and ChannelContext equal to this question's
id (file.go:301; the approve event carries the proof's ContextID,
validated at file.go:604 and parsed at 1368; ApprovalRecord itself
holds no question, file.go:151-164, so Approved.At and Authority
cannot tell this question's approval from another's, FCG-C-21) the
approval already landed under a machine that died before writing
the phase: skip to the receipt with "recorded: <goal> box raised to
<box>"; else build
governance.RecordedChannelAuthority{Outcome:
AuthorityOutcomeVerifiedChannelAnswer, Provider: answer.ref.provider,
UserID: answer.userId, MessageRef: answer.ref.threadId + "/" +
answer.ref.id, ContextID: qid, Step: answer.step} and
humanauthority.VerifiedChannelAnswerProof(root, recorded, answer.at)
(authority.go:179-190; the constructor takes the tuple and a time,
nothing local, so any machine builds it from the ledger answer), then
goal.Approve(VerbRequest{Endpoint, Actor{Machine: me, Lineage: mine,
Human: answer.userId}, Ulid: approvalUlid, Now: answer.at},
[question.goal], question.budget, &proof) — the call m2 makes at
poll.go:358-368 with the ledger answer in place of the local one;
then publish transition `approved` with `receipt` = "recorded: <goal>
box raised to <box>" on OutcomeConfirmed, or the Approve error's text
or Detail otherwise. The Approve Publish is an ordinary goal
transaction: on this machine a retry replays its own confirmed entry
(txn.go:517-544) and an abandoned one is followed by a fresh attempt
under a fresh ULID — the deterministic ULID is a convenience for
reading the history, not a correctness device, so a second attempt
after a terminal non-confirmed entry uses Opid(<fresh ULID>, me,
mine); a duplicate approve row is prevented by the goal read above,
not by the ULID. Recovery of a dead approve entry needs no new case
(FCG-C-11): VerbRequest has no source or question field
(verbs.go:168-180) and Approve writes its own Intent.Args, so
requestForEntry cannot rebuild a channel approve; its default
terminalises the entry rejected ("re-runs from its own entry point",
recover.go:403) when the recovery call of FCG-INBOX-02 runs, and the
approval intent still standing on the question is the re-run: the
next open-work pass takes it over, reads the goal (the predicate
above answers whether the dead attempt's commit landed) and calls
Approve again under a fresh opid or skips to the receipt. A budget
question without a tuple (a legacy record) gets phase approved
directly with the receipt channel-poll-refuses-legacy-budget-questions
fixed ("recorded: <goal> has no proposed box on this question;
nothing raised"). Every other answer goes from open to approved in
the answer commit itself (the matrix's second `answer` row) with
receipt "recorded: <goal> approved for execution" when the text
carries a stop token, "recorded: <goal> box not raised; the reply did
not carry the token" for a budget question with a tuple whose text
does not equal wants, or "recorded" otherwise. Phase receipted
is set by FCG-POST-08 (iii) with `receiptRef`, and the question
closes in that commit. A crash anywhere leaves a phase and at most
one intent that the next pass on any machine resumes or takes over.

### FCG-SECRET-15: the code is removed before anything durable is written

Contract `StripCode(text) (clean, code string, present bool)` in
totp.go, replacing SplitTOTP (totp.go:52-66) on every path: take the
last whitespace-delimited field of `text`; drop any trailing run of
the characters `.,;:!?` from it; if what remains is exactly six ASCII
digits it is the `code`, and `clean` is the original bytes with the
whole last field (punctuation included) and the whitespace
immediately before it removed and nothing else changed (no field
re-joining); otherwise present is false and clean is the text. A
message that is only a code yields clean "" and present true.
"approve 123456." yields ("approve", "123456", true); "order 123456
now" yields ("order 123456 now", "", false) and is the outcome
no-code, a fact with six digits in it, committed as it is
(FCG-C-15). The one ambiguity is a six-digit fact as the LAST field:
it is read as a code, fails to verify, and the human gets today's
bad-code rejection; renderQuestion's prompt already asks for the
code last, and the limit is named in the goal's conclusion. A code
that is not last, or is repeated ("approve 123456 note 123456",
round 3's FCG-C-15), is caught by the listener, not the validator:
after StripCode, and whatever the outcome, the committing machine
walks every remaining whitespace-delimited field of `clean`, trims
the same trailing `.,;:!?`, and replaces the field with the literal
`[code]` when the trimmed field is six ASCII digits and
VerifyTOTP(secret, field, sentAt) (totp.go:35-50, the three steps
around sentAt) accepts it; a six-digit field the secret does not
produce is a fact and stays. `clean` after masking is what the
record and the answer carry, and the code's bytes are never durable
on any path — the masking runs before the record is built, on the
one machine that holds the secret, so the validator's row below stays
what it is. Every durable surface is named: the inbox record's
`text` and the question's `answer.text` are the masked `clean`; the
Publish Intent is
`Intent{Verb: "inbox", Targets: [<record path>], Args: {provider,
destination, messageId, updateId, outcome, question}}` and carries no
text (journal.go:57-65 persists the Intent before the transaction;
recovery never rebuilds it, FCG-INBOX-02); the commit message is
`channel inbox <provider>-<messageId>`; the listener's log lines
carry update ids, outcomes and phases and never text; the fake
provider's journal (fixtures only) lives under artifacts and is
git-ignored; the validator's `channel-secret` row refuses the field
StripCode would have removed when it is still there (a hand-written
record) and any six-digit field in machine-written text, and never a
six-digit fact inside the human's sentence, so no committable message
is ever refused for its digits. The TOTP secret and the bot
token are read from metasystem.conf.local at each iteration and are
held only in memory. `channel status` prints neither.

### FCG-EVIDENCE-12: what proves it

Fixtures extend scripts/agents/channel-fixtures.sh with a second export
clone on the same bare origin and the same fake bot (telegram face),
both listening as separate processes. The fake (internal/channel/fake)
grows what the fixtures need (FCG-C-12). Listener identity rides the
bot token: the fake accepts any token of the form
`fake-telegram-token-<listener>` on the `/bot<token>/` path
(fake.go:36-40 hard-codes one token today) and takes `<listener>` as
the caller's name; the real bot has one token, so the fake keeps ONE
confirmed offset and ONE update stream across all its tokens — that
is what models one bot polled by several machines. Each fixture
listener's conf.local sets `channel.fake.listener=<name>` and the
fake provider (fake.go:31-43) mints its token from it. The journal
(journal.jsonl, fake.go:188) gains the listener name and a monotonic
sequence number on every row. getUpdates without offset returns only
unconfirmed updates and offset c forgets everything below c; timeout
T blocks up to T seconds until an update arrives. A control file
`<fake root>/control.json`, re-read per request, carries `conflict:
[{listener, remaining, description}]` (the next `remaining`
getUpdates from that listener get 409 with `description`),
`deliverOnlyTo: {<updateId>: [listener]}` (other listeners never see
that update), and `pauseBefore: [{listener, method, until: <path>}]`
(the fake holds that listener's next request of that Telegram method
— getUpdates, sendMessage, or the confirming getUpdates, named
`confirm` when it carries an offset — until the file exists; the
fixture asserts the ledger in the meantime, which is how a fixture
proves "commit before Confirm" without observing git from the fake).
Failure points: `METASYSTEM_CHANNEL_FAIL_AT=<phase>[:<kind>]`, read
by Listen, `channel poll` and `channel ask` (FCG-C-12) into
PollConfig.FailurePoint; at the
named point the process writes `failed at <phase>` to stderr and
exits 3 — once, because it is dead, and the fixture restarts it
without the variable. Phases: `before-publish:inbox`,
`after-publish:inbox` (the commit landed, Confirm not yet called),
`before-publish:approve`, `after-publish:approve`, `before-post:<posting
kind>`, `after-post:<posting kind>` (the post was made, its ref not
published). An unqualified phase applies to every kind.

Each fixture names its pass condition; every assertion reads the
ledger at origin/main, the fake journal, or a listener's exit status.

- FCG-12-ONE-COMMIT: one human reply with the token, two listeners A
  and B, control pauseBefore {A, confirm, <file>} and {B, confirm,
  <file>}. Pass: while both are paused, origin/main holds exactly one
  inbox record for the update and one `answer` history row; after the
  file is created the journal holds one Confirm row per listener, both
  after the paused getUpdates rows by sequence; exactly one receipt
  post; both listeners' logs show the update committed (one
  `confirmed`, one `lost to <opid>`).
- FCG-12-DIE-BEFORE-CONFIRM: A runs with FAIL_AT after-publish:inbox
  and exits 3; B receives the same update, its log says `lost to
  <A's opid>`, it confirms. Pass: one record whose `opid` is A's, one
  Confirm row, by B.
- FCG-12-DIE-BEFORE-COMMIT: A with FAIL_AT before-publish:inbox exits
  3; B commits and confirms; A restarts. Pass: one record with B's
  opid; A's journal rows after restart show a getUpdates returning
  zero updates; A's goal journal holds no entry for the message.
- FCG-12-ABANDONED-THEN-RETRIED (unit, internal/goal): on one clone an
  inbox Publish whose capture fails leaves an abandoned entry; a
  second Publish of the same message on a tip that now carries the
  record (written by another clone in the test) returns OutcomeLost
  naming the record's opid, and the clone's journal holds two
  entries under two opids.
- FCG-12-REPLAY-ACROSS-MACHINES: two messages with one code,
  deliverOnlyTo routes one to each listener. Pass: two records, one
  `verified` and one `replayed` with the same step, one answer row.
- FCG-12-UNTHREADED: with one open question, an unthreaded reply
  carrying the token is the answer; one carrying only "ok" is
  `unbound` and the token hint is posted once; with two open, a reply
  carrying question X's token answers X and one carrying neither is
  `unmatched` with the list posted once.
- FCG-12-LATE: a second threaded verified reply to an answered
  question. Pass: a record with outcome `late` and the question's id,
  the question's answer unchanged, one `late` rejection post.
- FCG-12-NO-TOKEN-BINDS-NOTHING: a threaded verified "no" to a stop
  question; the answer row's reason is "no"; `goal resume
  --approved-ref` refuses.
- FCG-12-TOKENLESS-MACHINE: a third clone with no bot token asks and
  waits; one listener posts the question once. Pass: the ledger shows
  the question intent (posting {question, by}) in a commit that
  precedes the sendMessage journal row by the pause protocol
  (pauseBefore sendMessage); the reply reaches the clone through
  `channel wait`.
- FCG-12-POST-CRASH: a listener with FAIL_AT after-post:question exits
  holding a posting intent; posting-stale-sec is 2 s in the fixture;
  the other listener takes over and posts. Pass: one `thread` ref on
  the question, two sendMessage rows in the journal, and after the
  dead listener restarts an `orphanPosts` entry is NOT present (it
  never got its ref: the ledger does not know that post, as the
  design says).
- FCG-12-POST-TAKEN-OVER-ALIVE: control pauseBefore {A, sendMessage,
  <file>} with posting-stale-sec 2 s; B takes over and posts; the file
  is created; A's post goes out and A's step (3) records the orphan.
  Pass: `thread` holds B's ref, `orphanPosts` holds A's, two
  sendMessage rows.
- FCG-12-CONFLICT-IS-NOT-ERROR: control conflict {A, 2, "Conflict:
  terminated by other getUpdates request"}; A retries after jitter and
  its heartbeat's conflictsLastHour is 2; a conflict with description
  "Conflict: can't use getUpdates method while webhook is active"
  makes A log ErrReceiveFailed and keep looping.
- FCG-12-BUDGET-TWO-STEP: a budget question with a tuple, the token
  answered, A with FAIL_AT after-publish:approve exits after the
  approve landed and before the phase; B's open-work pass takes over
  the approval intent, reads the goal, skips Approve. Pass: exactly
  one approve row on the goal, phase approved with the box receipt,
  one receipt post.
- FCG-12-MIGRATE: a clone with two legacy local questions (one
  answered in phase recorded, one open with no tuple as budget kind)
  and an unmatched.jsonl of one row runs `channel migrate`. Pass:
  two ledger questions with lineage `migrated`, the mapped phases,
  one `unverified-migrated` inbox record, the local directory
  renamed, and `channel poll` refusing before the migrate and not
  after; a third legacy question in phase `matched` whose saved Opid
  is on the tip migrates without a second answer row, and one whose
  Opid is not migrates with exactly one fresh answer row (FCG-C-14).
- FCG-12-POISON-SKIP (FCG-C-23): the fixture commits, directly on
  the ledger branch with a plain git commit and no trailer, a valid
  inbox record for an update the fake is about to deliver; both
  listeners log `inbox refused <id>: inbox record present without its
  transaction` and neither confirms; `channel skip --update <id>` on
  A replaces the record with outcome `skipped` under A's opid and
  confirms. Pass: the journal shows no Confirm before the skip and one
  after; the record's opid is A's; the next update is committed
  normally.
- FCG-12-ASK-CRASH (FCG-C-12): `channel ask` with FAIL_AT
  before-post:question exits 3 leaving the intent; a listener takes
  it over after posting-stale-sec (2 s) and posts. Pass: one `thread`
  ref written by the listener, one sendMessage row. Then `channel
  ask` with after-post:question exits 3; a listener takes over and
  posts again. Pass: `thread` holds the listener's ref, two
  sendMessage rows, `orphanPosts` empty.
- FCG-12-BLOCKED-JOURNAL (unit, internal/channel, FCG-C-16): a
  listener whose clone holds a PUSHED inbox entry (staged by a Publish
  whose confirming refetch is made to fail) receives its next update;
  its Publish is refused pushed-blocking; the iteration runs the
  recovery call, which confirms the entry from the tip, and the next
  Publish commits. Pass: the log line `journal blocked by <opid>;
  recovery ran: confirmed`, two confirmed entries in the journal.
- Unit tests in internal/goal (ValidateChannelTree's refusal table row
  by row including the listener schema, the secret row's three
  cases — last-field code, mid-sentence digits accepted, digits in
  facts refused — and the answer-state row's closed/null cases; the
  transition matrix row by row through the tuple predicate, including
  the orphan-post row and both answer rows; the inbox Mutate's three
  branches; the step check on the tip; the removed append; the
  duplicate-approval predicate against a goal approved through
  another question's channel answer, which must NOT match, FCG-C-21)
  and internal/channel (StripCode's four cases; the masking of a
  repeated and of a mid-sentence code and the non-masking of a
  six-digit fact; the batch-prefix Confirm rule; rule (b) with zero,
  one, two tokens and with the five-field resume token followed by a
  full stop, FCG-C-22; the migrate mapping including the two `matched`
  cases; the migrate refusal; `channel skip`'s record shape).

### FCG-BUILD-13: order and budget

Tier 3, Wido's box. Four steps, each landing alone (FCG-C-02, round
3): (1) the three fences and ValidateChannelTree with the three
schemas, the transition predicate as a library function, the opid
helper, and the inbox Mutate's three branches as a library function
with no caller — no writer, no live behaviour changes, no recovery
change (revision 4 needs none); (2) the provider contract (Ack,
Confirm, ErrBusy, the HTTP deadlines), the telegram and slack
adapters, the fake's controls and tokens, StripCode and the masking
as functions; the old Poll keeps passing its saved cursor and never
calls Confirm, so what Telegram is told is byte-for-byte what it was
told before — the existing channel fixtures stay green against the
fake, which is the proof that nothing moved; (3) the cut-over, one
landing: FCG-WORD-07's append removal (folded in here so that no
release removes the append while the old Poll still calls
goal.Answer), `channel ask` on the ledger with its failure points,
`channel migrate`, the migrate refusal, the receive rule and the
lost-to-winner commit, the listener's recovery call, matching, the
atomic answer, FCG-POST-08 for every post and the open-work pass (run
by `channel poll` and the steward tick), FCG-ANSWER-11, `channel
skip`; landed under the fleet sequence of FCG-MIGRATE-10; (4) Listen,
the RunLoop context, the heartbeat and status, landed with `steward
restart` on each machine (FCG-POLL-04). Fixtures grow with each step;
FCG-12-MIGRATE lands with step 3 before the fleet cut-over is
performed. One closing code review per two build steps. Estimated:
four build attempts of 25-45 minutes; the box's three review rounds
are spent on the design, so the two closing code reviews need a
review-round raise that only Wido can give — the build steps land
without them only if he says so.

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
