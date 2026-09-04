Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Build step 3a of goal fleet-channel-gateway (tier 3, Wido's box): the
ledger receive as a library — verify, match, commit — with NO caller.
FCG-BUILD-13 step (3) (metasystem/plans/fleet-channel-gateway-design.md
lines 1089-1120) is the cut-over and lands under the fleet sequence of
FCG-MIGRATE-10 (737-812), which needs every machine's steward
disarmed; the seat therefore splits step 3 into library landings that
change no live behaviour (3a this brief, 3b the posting protocol and
the open-work pass) and one cut-over landing (3c: the verbs, the new
Poll, the steward tick's short poll, FCG-WORD-07's append removal, the
fixtures). Nothing in 3a is reachable from any verb, the steward, the
old Poll or recovery: today's Poll (metasystem/internal/channel/poll.go)
keeps writing local questions, the totp-consumed register and
unmatched.jsonl exactly as it does; `goal.Answer` keeps its append
(verbs.go:119-123) until 3c. The existing channel fixtures stay green
against the fake; that is the proof that nothing moved.

The law is the design: FCG-INBOX-02 (131-381: the three records, the
one-opid rule, the inbox Mutate's three branches, the transition
matrix), FCG-RECEIVE-03's per-item rule (382-440, the listener's steps
(1)-(3) — you build (1) and (2); (3), Confirm, is the caller's in 3c),
FCG-COMMIT-05 (532-566), FCG-MATCH-06 (568-621), FCG-WORD-07 (623-640,
read for the atomic answer's reason: the human's text verbatim, no
append), FCG-SECRET-15 (886-932) and FCG-EVIDENCE-12's unit list
(1073-1087). Step 1 landed the fences, ValidateChannelTree, the
matrix, ClassifyChannelTransition, ChannelInboxMutate and ChannelOpid
(metasystem/internal/goal/channel.go); step 2 landed the provider
contract (Inbound.Ack, Inbound.UpdateID, Provider.Confirm), StripCode
and MaskCodes (metasystem/internal/channel/channel.go,
metasystem/internal/channel/totp.go). Build on them; change neither
the validator's rows nor the matrix.

# Workspace

A fresh worktree on main. Do not stage or commit; the seat lands the
chain.

- May write: metasystem/internal/goal/channel_inbox.go
- May write: metasystem/internal/goal/channel_inbox_test.go
- May write: metasystem/internal/goal/verbs.go (only the factoring
  named under "The goal answer row"; goal.Answer's behaviour is
  byte-for-byte what it is)
- May write: metasystem/internal/channel/inbox.go
- May write: metasystem/internal/channel/inbox_test.go

No other file changes. No verb, flag or config key. Nothing under the
steward, recovery, poll.go, question.go, the adapters or the fake.

# What to build

## 1. Reading the channel tree at a tip (internal/goal)

`ReadChannelTree(e Endpoint, tip string) (*ChannelTree, error)`:
lists `plans/channel/` at `tip` (the way ValidateChannelTree does,
channel.go:139-160) and decodes every question, inbox record and
listener record with the existing decoders (channel.go:282-340),
returning them keyed by id (questions), by `<destination>/<provider>-
<messageId>` (inbox) and by machine (listeners). A tip without the
directory is an empty tree, not an error; a file the decoders refuse
is an error naming the path (the validator already refused it on
landing, so this is a corrupt-ledger report, not a rule).

## 2. Verification on the receiving machine (internal/channel)

`type ReceiveRule struct { HumanUserID, TOTPSecret string; StaleAfter
time.Duration; Now time.Time }` and

`Verify(rule ReceiveRule, in Inbound) (outcome string, step *int64,
text string)`

in this order, the first failure is the outcome (FCG-COMMIT-05
534-539): sender user id equals rule.HumanUserID, else `wrong-user`;
`StripCode(in.Text)` finds a code, else `no-code`; `rule.Now.Sub(
in.SentAt) <= rule.StaleAfter`, else `stale`; `VerifyTOTP(secret,
code, in.SentAt)` accepts, else `bad-code`; otherwise `verified` with
the step. An empty HumanUserID or TOTPSecret is `wrong-user` /
`bad-code` respectively (there is no `unconfigured` outcome in the
record's vocabulary; the caller in 3c refuses to receive at all when
unconfigured, as today's Poll does at poll.go:267-269). `text` is
ALWAYS `MaskCodes(clean, secret, in.SentAt)` of StripCode's `clean`,
whatever the outcome (FCG-SECRET-15 903-916: masking runs before the
record is built, on the machine that holds the secret); a message
that was only a code yields "". A zero SentAt is treated as rule.Now
for the stale and TOTP checks (record it in whatWasDone).

`InboundRecord(rule ReceiveRule, provider, destination, machine
string, in Inbound) goal.ChannelInbound` builds the record of
FCG-INBOX-02 (213-232) from Verify: provider, destination, messageId
= in.Ref.ID, updateId = decimal of in.UpdateID (Slack: the ts again,
so when UpdateID is 0 use in.Ref.ID), replyTo = in.Ref.ThreadID or
null when "", userId, sentAt at second precision (channelTimeLayout),
text, step, outcome, question "" (the Mutate fills it), opid "" (the
Publish fills it), receivedBy machine, receivedAt rule.Now.

## 3. Matching against the tip (internal/goal)

`MatchChannelInbound(tree *ChannelTree, record ChannelInbound, wants
func(q *ChannelQuestion) bool) (question string, bound bool)` over the
questions whose `destination` equals the record's:

- rule (a), any outcome: `replyTo` non-null and equal to a question's
  `thread.id`, one of its `rejected[].postRef.id`, one of its
  `orphanPosts[].id` or its `answer.receiptRef.id` names that
  question. `bound` is true when that question's state is `open` and
  the record's outcome is `verified`. (A threaded verified reply to a
  question that is not open is the outcome `late`, decided in the
  Mutate below.)
- rule (b), only when the record's outcome is `verified` and replyTo
  is null: take the questions open on that destination whose
  `thread` is non-null (a question that has not been posted has put
  no token in the human's hands — the seat's mechanical reading of
  "open on that destination"; record it) and whose `wants` is
  non-empty and appears in the record's text as a contiguous run of
  whitespace-delimited fields equal to strings.Fields(wants), exact
  bytes, case-sensitive, once and only once — containsContiguousFields
  (verbs.go:96-108) is the rule and the function, extended ONLY by
  dropping trailing `.,;:!?` from the text field that matches the
  token's last field before the comparison (FCG-C-22: "goal=x resume
  elapsed=1d attempts=10 minutes=1200 active=1." matches the
  five-field token; "goal=x resume" alone matches nothing). Exactly
  one such question → its id, bound. None with exactly one open
  question on the destination → `unbound`. None with several open, or
  two or more tokens present → `unmatched`. None open → `unmatched`.
- everything else (an unverified unthreaded message) → `unmatched`.

Option labels never bind unthreaded (599-602). A question with empty
`wants` is matched by thread only.

## 4. The inbox Publish (internal/goal, wrapped in internal/channel)

`ChannelInboundRequest(machine, lineage, opid string, record
ChannelInbound, now time.Time, decided *ChannelInbound)
PublishRequest` — the caller mints the opid with ChannelOpid
(channel.go:979) — with Intent `{Verb: "inbox", Targets: [<record
path>], Args: {provider, destination, messageId, updateId, outcome,
question}}` (no text, 917-921; the Args carry the outcome and
question as first computed; the committed record is the truth),
Message `channel inbox <provider>-<messageId>`, Validate =
ValidateCommit, and a Mutate that on EVERY attempt, against the given
tip:

1. runs ChannelInboxMutate (channel.go:950) for the record path — a
   present record with its trailer returns LostToCompetitor, present
   without it the named error, absent continues;
2. reads the tree (ReadChannelTree) and copies the record;
3. replay by step (540-546): when the record's step is non-null and
   any inbox record at the tip carries the same step with a different
   messageId, outcome becomes `replayed` (step kept);
4. matching (MatchChannelInbound); `question` is set. When rule (a)
   named a question whose state is not `open` and the outcome is
   `verified`, outcome becomes `late` (547-554): step kept, the
   question file is NOT touched. When the outcome is `replayed`, the
   record is not bound (question stays what the matcher named, nothing
   advances);
5. when `bound` and the outcome is still `verified`: the atomic answer
   (606-621). Select the matrix row: `answer budget` when kind is
   budget-above-norm, the text equals wants and budget is present,
   else `answer`; ClassifyChannelTransition (channel.go:905) with the
   question's Tuple, me = machine, writer = the question's answer.opid
   when an answer is present; on true, write the question with state
   `answered` and answer {text, userId, ref {provider, id =
   messageId, threadId = replyTo}, at = sentAt, step, inboxId =
   `<provider>-<messageId>`, opid = this opid, phase and approvalUlid
   and receipt per the row: `recorded` with approvalUlid = the
   deterministic ULID of FCG-ANSWER-11 (822-827: 48-bit time =
   answer.at in ms, 80 random bits = the first ten bytes of
   SHA-256("approve:" + qid)) and receipt null for the budget row;
   `approved` with approvalUlid null and receipt "recorded: <goal>
   approved for execution" when kind is stop and the text carries
   wants, "recorded: <goal> box not raised; the reply did not carry
   the token" for budget-above-norm with a tuple whose text does not
   equal wants, "recorded: <goal> has no proposed box on this
   question; nothing raised" for budget-above-norm without a tuple,
   "recorded" otherwise (873-881)}, receiptRef null, posting
   unchanged; and the goal answer row of item 5 below in the same
   change set. On LostToCompetitor or AlreadyApplied from the
   classifier, return it (the record path was absent, so this is the
   race the design names at 617-620 and the outer Publish classifies
   it);
6. writes `decided` = the record as committed on this attempt (outcome
   and question after 3-4, opid set) so the caller learns what landed;
7. returns the changes: the record (MarshalChannel), plus the question
   and the goal file when 5 applied.

## 5. The goal answer row (internal/goal/verbs.go)

Factor the body of answerRequest's Mutate (verbs.go:125-148) into a
helper that takes the loaded tree, the goal id, qid, text, reason and
the proof fields and returns the goal-file Change (the NextStep
marker, Revision++, the HistoryLine with Verb `answer`, Actor
"human:wido", AuthorityOutcome AuthenticatedChannelWord, the channel
provider/user/ref/step, Reason). answerRequest keeps computing
`reason` with its append (119-123) and calls the helper — goal.Answer
is unchanged in behaviour and its tests pass unmodified. The inbox
Mutate calls the helper with reason = the record's text verbatim
(FCG-WORD-07: no append on this path, ever), Ref = replyTo + "/" +
messageId (the shape today's Poll passes, poll.go:351), Step = the
record's step. The HistoryLine's Opid is the inbox commit's opid, so
the record, the question and the row carry one opid (289-291).

## 6. The channel wrapper (internal/channel/inbox.go)

`PublishInbound(e goal.Endpoint, machine, lineage string, record
goal.ChannelInbound, now time.Time) (goal.PublishResult,
goal.ChannelInbound, error)`: mints the opid (ChannelOpid), builds the
request, calls goal.Publish (txn.go:508) and returns the result and
the decided record. Nothing else: no Confirm, no post, no cursor, no
log — the 3c caller owns steps (3) of FCG-RECEIVE-03 and the posts.

# Tests

internal/goal (channel_inbox_test.go), on a real repository the way
channel_test.go's commitChannelFiles and txn tests build one:

- ReadChannelTree: empty on a tip without the directory; the three
  record kinds keyed as named; a refused file names its path.
- MatchChannelInbound rule (a): thread id, a rejected postRef id, an
  orphanPosts id, a receiptRef id each name the question; open +
  verified binds, open + wrong-user names but does not bind, answered
  + verified names and does not bind.
- Rule (b) (FCG-C-22): zero tokens with one open → unbound; zero
  tokens with two open → unmatched; one token → bound; two tokens →
  unmatched; the five-field resume token followed by a full stop
  matches; "goal=x resume" alone does not; a token in a question whose
  thread is null does not bind; case differs → no match; a non-
  verified unthreaded message → unmatched; no open question →
  unmatched.
- The Publish, end to end on two clones of one bare origin (as the
  txn fixtures do): (i) a verified bound message writes the record,
  the question (state answered, phase per row, answer.opid = the
  commit's opid, inboxId) and the goal row (Reason = the text
  verbatim, no appended token; ChannelStep = the step) under ONE
  opid, and ValidateCommit accepts the commit; (ii) the same message
  published from the second clone after fetching returns OutcomeLost
  with Detail naming the first record's opid and touches nothing
  (FCG-12-ABANDONED-THEN-RETRIED's second half; and its first half:
  a Publish whose capture is made to fail — BeforePush or the
  fixture seam txn tests use — leaves an abandoned entry, the retry
  under a fresh opid returns lost, the journal holds two entries
  under two opids); (iii) a second verified message with the same
  step and a different messageId commits as `replayed` and advances
  nothing; (iv) a verified threaded reply to an answered question
  commits as `late` with the question's id, the question unchanged;
  (v) the budget row: text equals wants with a tuple → phase
  recorded, approvalUlid = the deterministic ULID (assert the exact
  value for a fixed at and qid), receipt null; text differs → phase
  approved with the "box not raised" receipt; a stop question with
  the token → "approved for execution"; (vi) the record present
  without its trailer (committed by hand on the ledger branch) →
  the named error, nothing written; (vii) goal.Answer's existing
  tests pass unmodified and its append still happens.

internal/channel (inbox_test.go): Verify's order — wrong-user before
no-code before stale before bad-code, verified with the step; the
text is masked whatever the outcome (a mid-sentence code masked, a
six-digit fact kept, an only-code message → ""); InboundRecord's
field mapping including replyTo null, updateId from UpdateID and the
Slack fallback, second-precision sentAt.

Every test is deterministic: fixed times, a fixed secret, no sleeps.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/goal/...
./internal/channel/... ./cmd/...`, `go test ./internal/goal/...
./internal/channel/... ./cmd/... -count=1` (the cmd package is slow
and is part of the proof that no caller moved), `go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/goal/...
./internal/channel/...`. The seat runs scripts/agents/channel-fixtures.sh
on the worktree (report it as not run). Record every mechanical
choice in `whatWasDone`. Every path in your return is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 45 minutes. The five-file boundary is exact. No
new verb, flag or config key; no change to ValidateChannelTree's rows,
ChannelMatrix, ChannelInboxMutate or the provider contract; goal.Answer
unchanged in behaviour; nothing that today's Poll, the steward or
recovery can reach.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief and
the design do not settle (a new refusal, a new authority, a schema
neither gives); a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
