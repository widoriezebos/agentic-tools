Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Build step 3b of goal fleet-channel-gateway (tier 3, Wido's box): the
posting protocol, the open-work pass and the budget re-approval on the
ledger, as a library with NO caller. Step 3a (landed: the receive —
metasystem/internal/goal/channel_inbox.go and
metasystem/internal/channel/inbox.go) gave the inbox Publish and the
atomic answer; this step gives every question-side transaction that
follows an answer or precedes a post, so that step 3c (the cut-over:
the verbs, the new Poll, the steward tick's short poll, FCG-WORD-07's
append removal, migrate, skip, the fixtures) is wiring. Nothing here
is reachable from any verb, the steward, today's Poll or recovery;
the existing channel fixtures stay green, which is the proof.

The law is the design (metasystem/plans/fleet-channel-gateway-design.md):
FCG-INBOX-02's matrix and the one-opid rule (131-381), FCG-POST-08
(642-712), FCG-ANSWER-11 (814-884), the post texts of FCG-COMMIT-05
(559-566) and FCG-MATCH-06 (589-598), FCG-STATUS-09's silence post
(728-731, the text only; the listener that decides to post it is step
4), FCG-EVIDENCE-12's unit list (1073-1087: the duplicate-approval
predicate, FCG-C-21). Step 1 landed ChannelMatrix,
ClassifyChannelTransition, TupleAt and ChannelOpid
(metasystem/internal/goal/channel.go); step 3a landed ReadChannelTree.
Build on them; change neither the validator's rows nor the matrix.

# Workspace

A fresh worktree on main. Do not stage or commit; the seat lands the
chain.

- May write: metasystem/internal/goal/channel_posting.go
- May write: metasystem/internal/goal/channel_posting_test.go
- May write: metasystem/internal/goal/channel_approval.go
- May write: metasystem/internal/goal/channel_approval_test.go
- May write: metasystem/internal/channel/posting.go
- May write: metasystem/internal/channel/posting_test.go
- May write: metasystem/internal/channel/openwork.go
- May write: metasystem/internal/channel/openwork_test.go

No other file changes. No verb, flag or config key. Nothing under the
steward, recovery, poll.go, question.go, the adapters or the fake.

# What to build

## 1. Question transactions (internal/goal/channel_posting.go)

One constructor per matrix row that a post or an approval needs, each
returning a PublishRequest (Opid from the caller's ChannelOpid;
Machine, Lineage; Validate = ValidateCommit; Message `channel <row>
<qid>`; Intent Verb = the row's name with spaces replaced by `-`,
Targets [the question path], Args {question, kind} and the ref's
provider/id for ref rows — never text) whose Mutate re-reads the
question at the given tip on every attempt, computes TupleAt(now,
staleAfter), calls ClassifyChannelTransition with writer = the field
the row sets (posting.by for intent and take-over rows; the ref's
opid is not recorded, so for ref rows writer = "" — a TO-shaped
tuple without own trailer is then the generic rejection, the state
FCG-C-20 names), and on true applies exactly what the row's TO says
(INBOX-02 353-368):

- `ChannelIntentRequest(kind)`: posting {kind, me, now}; the Mutate
  refuses `channel-posting-busy` when the tip carries a posting
  younger than staleAfter by another machine (POST-08 652-654) — that
  is the FROM row's `posting null` failing with a named reason; kinds
  question, rejection, list, silence, receipt, approval; a rejection
  intent carries RejectionReason and is legal on a closed question
  only for reason `late`.
- `ChannelRefRequest(kind, ref, entry)`: posting null and the ref
  into its field — `thread` for question; for rejection a complete
  `rejected` entry {ref = the inbound message's ref, reason, at,
  postRef = ref, by = me} appended (the seat's reading of INBOX-02
  365 with COMMIT-05 563-565: the entry is written complete when its
  post exists, so the three-post ceiling counts entries with a
  postRef and no half-entry ever rests on the tip; record it); for
  list and silence an entry with reason `list`/`silence`, ref = the
  inbound ref that triggered it (list) or a zero ref (silence), postRef
  = ref, by = me; for receipt `answer.receiptRef` = ref, state closed,
  phase receipted, closedAt now, closedBy me, closedBecause
  `answered`.
- `ChannelTakeOverRequest()`: posting.by = me, at = now, nothing
  else; FROM requires a foreign posting older than staleAfter.
- `ChannelOrphanPostRequest(ref)`: orphanPosts + ref, nothing else;
  FROM any tuple with the file present.
- `ChannelApprovedRequest(receipt)`: phase approved, receipt set,
  posting null; FROM (answered, recorded, {approval, me}).

Every Mutate returns LostToCompetitor when the tuple equals TO and the
writer names another machine, AlreadyApplied when the tip carries this
opid (the classifier does both), and the named `channel-transition`
rejection otherwise.

## 2. The posting protocol (internal/channel/posting.go)

`type Poster struct { Endpoint goal.Endpoint; Machine, Lineage string;
Provider Provider; Destination DestinationConfig; StaleAfter
time.Duration; Now func() time.Time; FailAt func(phase string) error;
Log func(string) }` and

`(p Poster) Post(ctx, kind, qid, text string, thread *MessageRef,
entry RejectionEntry) (ref MessageRef, outcome string, err error)`:

(1) Publish the intent (ChannelIntentRequest under a fresh opid); on
`channel-posting-busy` or LostToCompetitor return outcome `busy`
without posting (another machine holds the intent); (2) FailAt
("before-post:<kind>"), post through Provider.Post, FailAt
("after-post:<kind>"); (3) Publish the ref (ChannelRefRequest under a
fresh opid); when its Mutate finds posting.by is another machine or
null (the take-over happened while we posted), publish the orphan-post
row instead, log `post orphaned <ref id>` and return outcome `orphan`
(POST-08 672-684); otherwise outcome `posted`. A Provider.Post error
leaves the intent standing (the take-over rule collects it after
StaleAfter) and is returned. The log carries kinds, question ids and
ref ids, never text.

`(p Poster) TakeOver(ctx, q *goal.ChannelQuestion) (outcome, err)`:
publish the take-over row, then finish that kind's steps (2) and (3)
with the text the kind needs (below); for kind `approval` there is no
post — run the approval step of section 3 instead.

Texts (`PostText(kind, q, args)`, one function, every string here
verbatim): question — renderQuestion's text as question.go:231-251
renders a local question today (build the same prompt from the ledger
question's fields; it already says "Reply in this thread with this
token verbatim"); rejection — "not recorded: <reason>; reply with your
answer and your code" (poll.go:207, today's text) for wrong-user,
no-code, bad-code, stale, replayed, and for `late` "already answered:
<feature name> was answered at <answer.at> with '<first forty
characters of answer.text>'; a new question needs a new ask"
(COMMIT-05 550-553); unbound hint (a rejection-kind post on the one
open question, reason `unbound`) — "not recorded: I have one open
question (<feature name>); reply in its thread, or with its token
<wants>"; list — one message listing the open questions by feature
name and token, ending "reply to the one you mean"; receipt — the
question's answer.receipt; silence — "the channel heard nothing for
<age>; answers sent in that time may need repeating". Feature name =
the goal id rendered as words (the way `channel status` names goals
today, report.go — reuse its function; if it has none, the goal id).

Ceilings, computed at the tip by the caller of Post through
`RejectionAllowed(q) bool` (fewer than three `rejected` entries with a
postRef whose reason is not `list` or `silence`) and
`ListAllowed(tree, destination, now) bool` (fewer than three `list`
entries across that destination's questions in the last hour, MATCH-06
596-598).

## 3. The budget re-approval (internal/goal/channel_approval.go, driven from internal/channel/openwork.go)

`ChannelApprovalDue(q) bool`: kind budget-above-norm, budget present,
answer present with phase recorded and text equal to wants.

`ChannelApproveLanded(t *TreeGoals, goalID, qid string) bool`: the
goal's History holds an event with Verb `approve`, AuthorityOutcome
VerifiedChannelAnswer and ChannelContext equal to qid (file.go:301,
604, 1368; FCG-C-21 — an approval through ANOTHER question's channel
answer must not match).

`DeterministicApprovalULID(at time.Time, qid string) string`: 48-bit
time = at in ms, 80 random bits = the first ten bytes of
SHA-256("approve:" + qid), Crockford base32 as the ULID library
renders it (ANSWER-11 822-827) — 3a already computes this for the
answer commit; move it here and have 3a's file call it (that call
site is the ONE line you may touch outside your files; name it in
whatWasDone), or, if 3a exported it under internal/goal, use it as
it is.

`(p Poster) Approve(ctx, q *goal.ChannelQuestion) (outcome, err)`
(openwork.go), item (ii) of the open-work pass: publish the approval
intent; if !ChannelApprovalDue(q) publish `approved` at once with the
receipt for the case ("recorded: <goal> has no proposed box on this
question; nothing raised" for a budget question without a tuple —
ANSWER-11 873-876 — else "recorded"); else read the goal at that tip:
if ChannelApproveLanded, publish `approved` with "recorded: <goal>
box raised to <box>" (renderProposedBox, question.go:252); else build
governance.RecordedChannelAuthority{Outcome: VerifiedChannelAnswer,
Provider: answer.ref.provider, UserID: answer.userId, MessageRef:
answer.ref.threadId + "/" + answer.ref.id, ContextID: qid, Step:
answer.step}, humanauthority.VerifiedChannelAnswerProof(root,
recorded, answer.at) (authority.go:179-190), then goal.Approve(
VerbRequest{Endpoint, Actor{Machine: me, Lineage: mine, Human:
answer.userId}, Ulid: answer.approvalUlid, Now: answer.at},
[q.Goal], q.Budget, &proof) (approval.go:406; the call today's Poll
makes at poll.go:367 with the ledger answer in place of the local
one); then publish `approved` with "recorded: <goal> box raised to
<box>" on OutcomeConfirmed, or the Approve error's text or Detail
otherwise (855-856). A retry after a terminal non-confirmed approve
entry uses a fresh ULID (857-863): when goal.Approve returns
Outcome abandoned/rejected/expired on the deterministic ULID,
publish `approved` with that outcome's text — the next pass's
ChannelApproveLanded read decides whether to call again — do NOT
loop inside one pass. FailAt("before-publish:approve") before the
Approve call and ("after-publish:approve") after it.

## 4. The open-work pass (internal/channel/openwork.go)

`OpenWork(ctx, p Poster, tree *goal.ChannelTree) (Report, error)`,
POST-08 703-712, over the questions on p.Destination.Name in openedAt
order: (i) open, thread null, posting null → Post(question); (ii)
answered, phase recorded, posting null → Approve; (iii) answered,
phase approved, posting null → Post(receipt); (iv) posting older than
StaleAfter by another machine → TakeOver. Each item is its own
transaction(s); a `busy` or lost outcome moves to the next question;
an error is returned after the report so far (the caller logs and
loops). Report counts posted, approved, takenOver, busy, errors by
question id — no text.

# Tests

internal/goal: each constructor's Mutate on a real repository (as
channel_test.go builds one): the happy row, the LostToCompetitor row
(tip at TO under another machine), AlreadyApplied (own trailer), the
named rejection (wrong tuple), `channel-posting-busy` against a fresh
foreign posting and success against a stale one; rejection intent on
a closed question refused unless reason late; the rejection ref
appends a complete entry and the receipt ref closes the question with
the three closed fields; ChannelApproveLanded against a goal approved
through this question's answer (true) and through another question's
(false, FCG-C-21) and with no approve row (false);
DeterministicApprovalULID's exact value for a fixed at and qid and
that two calls agree.

internal/channel, with a test Provider (channel_test.go has one) on a
real repository: Post's three steps and the ref on the tip; a Post
whose Provider.Post fails leaves the intent; a take-over during the
post makes the returning poster write orphanPosts and log `post
orphaned`; FailAt before-post leaves the intent with no journal post,
after-post leaves the post with no ref; OpenWork drives (i)-(iv) on a
tree with one question per case and a stale foreign posting; Approve
on a budget question with a tuple calls goal.Approve once (assert one
approve row with channelContext = qid, the box raised) and a second
pass skips the call (ChannelApproveLanded) and rests at approved;
Approve on a legacy-shaped budget question without a tuple rests at
approved with the "no proposed box" receipt and no approve row;
PostText's every case is asserted verbatim (the design fixes the
words).

Every test is deterministic: fixed times through Now, no sleeps.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/goal/...
./internal/channel/... ./cmd/...`, `go test ./internal/goal/...
./internal/channel/... ./cmd/... -count=1`, `go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/goal/...
./internal/channel/...`. The seat runs scripts/agents/channel-fixtures.sh
(report it as not run). Record every mechanical choice in
`whatWasDone`. Every path in your return is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 45 minutes. The file boundary above is exact (plus
the one call-site line named in section 3, if needed). No new verb,
flag or config key; no change to ValidateChannelTree's rows,
ChannelMatrix, ChannelInboxMutate, the provider contract or goal.Approve;
nothing today's Poll, the steward or recovery can reach.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief and
the design do not settle (a new refusal, a new authority, a schema
neither gives); a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
