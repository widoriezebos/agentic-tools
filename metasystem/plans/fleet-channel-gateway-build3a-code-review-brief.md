Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

The closing code review of build step 3a of goal fleet-channel-gateway
(tier 3, box 3d/24/1200m/1/3): the ledger receive as a library —
verify, match, commit — with no caller. Implementer chain fcg-build3a
(gpt-5.6-sol), two rounds: the step
(metasystem/artifacts/agents/fcg-build3a/rounds/1/prompt.md, which is
metasystem/plans/fleet-channel-gateway-build3a-brief.md as delivered;
return in rounds/1/return.json with its recorded mechanical choices)
and a one-file fixture fix (rounds/2/prompt.md: four test ULIDs
carried the letter I, outside the Crockford alphabet, and the ledger
validator refused the opid; return in rounds/2/return.json). The
delegate's sandbox could not run the git-backed tests; the seat ran
them (Evidence below).

You are a fresh root: review the terminal round's computed diff
(metasystem/artifacts/agents/fcg-build3a/rounds/2/diff.patch,
reviewedTree in rounds/2/review.json —
04560087ca8c104fbc05e4404bf2d2917ebd2b8d; five files, base-to-tree)
against the round-1 brief as delivered and the design points it names
in metasystem/plans/fleet-channel-gateway-design.md: FCG-INBOX-02
(lines 131-381), FCG-RECEIVE-03 (382-440), FCG-COMMIT-05 (532-566),
FCG-MATCH-06 (568-621), FCG-WORD-07 (623-640), FCG-ANSWER-11
(814-884), FCG-SECRET-15 (886-932) and FCG-EVIDENCE-12's unit list
(1073-1087). Review the diff, never the delegate's summaries.

# Review brief

Threat model and scope, declared before round one: the library is
unreachable from any verb, the steward, the old Poll or recovery; the
threat is a wrong ledger truth or a leaked code once step 3c calls it,
and a factoring of goal.Answer that changes what it writes today.
Out of scope: the caller's Confirm and posting (3b/3c), the fake, the
adapters, the fixtures.

LAYER 1, conformance, in this order:
(a) Boundary: exactly the five declared files; verbs.go changed only
by the factoring of answerRequest's Mutate body into a helper, and
goal.Answer's committed bytes (the token append, the history line's
verb, actor, authority outcome, channel context) are what they were —
name the test or the diff hunk that proves it.
(b) ReadChannelTree: lists plans/channel at the tip the way
ValidateChannelTree does; questions, inbox records and listeners keyed
by path; a corrupt record names its path; an absent directory is an
empty tree, not an error.
(c) Verify (RECEIVE-03, COMMIT-05): outcomes in the order wrong-user →
no-code → stale → bad-code, first refusal wins; the text carried
forward is always MaskCodes of StripCode's clean (SECRET-15: no raw
code reaches a record, a journal intent, an error string or a test
fixture's expected value); staleness measured against ReceiveRule.Now
when SentAt is zero; the TOTP step recorded for every verified item.
(d) InboundRecord: second-precision UTC; replyTo nullable; Telegram
update ids decimal; Slack falls back to the message id; provider,
destination and machine recorded as given.
(e) MatchChannelInbound (MATCH-06): rule (a) threaded, via thread,
postRef, orphanPosts and receiptRef ids, before rule (b); rule (b)
only on open questions whose thread is non-null (the seat's reading,
recorded in the brief), contiguous token fields, exact case, trailing
`.,;:!?` dropped only from the candidate for the token's last field
(FCG-C-22); zero tokens with one open question binds, with two open
is unbound; a repeated or multiple token is unmatched; an unposted
question counts for unbound-versus-unmatched but never binds.
(f) ChannelInboundRequest's Mutate (INBOX-02, COMMIT-05, ANSWER-11):
inbox mutate first; the tree read at the transaction's tip, never a
cached one; replay by TOTP step at the tip (equal step, different
message id, is a replay; a replay never advances a question); a
threaded verified answer to a non-open question is `late`; the
atomic answer selects the existing answer-matrix row, keeps the
posting, writes the question's answer and the goal history line
under ONE opid; the human's text verbatim as the goal reason (no
append, WORD-07); the budget approval's ULID is deterministic
(48 time bits from the answer time, the first ten SHA-256 bytes of
"approve:<qid>"); receipt texts for stop, budget mismatch, missing
tuple and the ordinary case are the design's, byte for byte; the
inbox commit never writes `rejected` (the seat's reading: rejection
entries are completed by 3b's rejection-ref transition).
(g) The Endpoint-first parameter the delegate added to
ChannelInboundRequest (recorded as mechanical: the retry callback
reads the tip): is it the smallest change that lets Mutate read the
transaction's tip, and does PublishInbound still only mint the opid,
build the request, call Publish and return the decided record?
(h) The tests in EVIDENCE-12's unit list are present, discriminating
(name the assertion that fails if the rule is removed), on fixed
clocks and secrets, and none carries a raw TOTP code as an expected
string.

LAYER 2, adversarial, on the whole diff: a verified item whose
question closed between the pre-tip decision and the transaction
(is the recomputed truth what commits, and does the journal intent
say the pre-tip outcome honestly); two items for the same question
in one poll (second is late or replay, never a double answer); a
token that is a prefix of another question's token; a message that
carries the token and a thread reference to a DIFFERENT question
(which rule wins, and is the answer atomic with that question only);
MaskCodes when the text contains the code twice or split by
punctuation; ReadChannelTree on a tree where plans/channel holds an
unknown file kind; the deterministic approval ULID colliding across
two questions answered in the same second (is the qid hash in the
random bits); a lost race (LostToCompetitor) leaving an inbox record
half-written; the goal history line's opid rule (`<ulid>-<machine>-
<hash8>`) for every opid the library mints; the factored helper called
from goal.Answer with the append still applied by the caller (is the
append applied exactly once).

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. A finding that indicts the design rather
than the code is reported as such (design point named) and counts.
A true finding outside the threat model above closes as out-of-scope
citing it.

Evidence (seat, round-2 tree): `go test ./internal/goal -run
'ChannelInbound|ReadChannelTree|MatchChannelInbound|ChannelAnswerDisposition'
-count=1` ok (25 s); `go test -race ./internal/channel/...` all five
packages ok on the round-1 tree (round 2 changed only
channel_inbox_test.go); conformance review of round 2 wrote
rounds/2/diff.patch (5 files) at reviewedTree 04560087. Full goal
package on the round-2 tree, run alone: `go test ./internal/goal
-count=1 -v -timeout 5400s` ok (352 tests, 957.9 s; the package is
slow because its fixtures run real git transactions, not because
anything hangs). Run what your sandbox allows (`go build ./...`,
`go vet ./internal/goal/... ./internal/channel/...`); report what
could not run.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
reviewedTree 04560087ca8c104fbc05e4404bf2d2917ebd2b8d.

# Gap Rule

stop and report a gap; never fill it silently.
