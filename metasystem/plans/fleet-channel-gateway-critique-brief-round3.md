Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Critique brief, round 3 (failsafe): the channel gateway, design revision 3

FINDING IDS: chain-unique; continue the chain's numbering at FCG-C-19;
a re-opened finding from round 1 or 2 keeps its own id. Never F-n.

This is round 3 of the goal's three, the failsafe round under
R-60-m1's rule: after it, only a demonstrated requirement failure or a
shape-level defect reopens prose; a remaining finding that a fixture
can express becomes an obligation row and the build starts. Mark
each material finding with which of the two it is — `reopens-prose`
(the design cannot be built as written, or building it as written
would violate one of Wido's quoted decisions or an existing law of
the ledger) or `fixture-expressible` (the design is buildable and the
finding names a case a fixture or unit test must pin) — in the
claim's first words. A finding that is neither is not material.

Subject: metasystem/plans/fleet-channel-gateway-design.md, revision 3,
and its round-2 dispositions in
metasystem/plans/fleet-channel-gateway-dispositions-round2.md (twelve
findings, all accepted; `validate critique-closed` confirms both
joins). Wido's decisions quoted in the head are not up for review.
Ground it against the tree at main as it is when the job starts; the
round-2 cites were re-read at e526a54e and the mechanisms they name
have not moved.

What changed and where to attack it, in order: (1) INBOX-02's own
opid per commit and the inbox Mutate's three branches — confirm
against txn.go (Publish 508-544, terminalFromMutate 725-760,
LostToCompetitor 443) and recover.go (56-140, 209-260) that a loser's
OutcomeLost carries the winner's opid in Detail, that no path leaves a
clone unable to commit or confirm a message it receives again, and
that the new inbox recovery rule sits where the slice-start rule sits;
(2) the transition matrix and its predicate — walk every row's FROM
tuple against every other row's TO and find a tuple two transitions
share or a state the matrix reaches that the at-rest table refuses;
check the heartbeat's transition and the `close` row against `channel
close` as it exists; (3) MATCH-06 (b) as rewritten — a message with
one token and two open questions; a token that is a substring of
another question's token; a message whose token field carries trailing
punctuation; the `late` outcome's step consumption against the replay
check; (4) POST-08 with the asker inside the protocol — `channel ask`
writing question and intent in one commit, a crash at every step for
the asker and for a listener, the orphanPosts rule, and whether the
HTTP deadlines named actually bound the Telegram adapter's Post as it
is (telegram.go:99-135, New at 20-25); (5) ANSWER-11 — the exact
Approve call against approval.go:406-425 and authority.go:179-190,
the goal read that prevents a duplicate row against opidLanded
(approval.go:459-461) and the ApprovalRecord (file.go:151-164), the
new requestForEntry case; (6) MIGRATE-10 — the fleet sequence as a
landing precondition (is anything in it enforceable by the engine
that the design leaves to people?), the field mapping against the
legacy Question struct (question.go:29-58) and the legacy phases in
poll.go:340-410, the `matched` case through goal.Answer, and the
`unverified-migrated` record against the validator's rows; (7)
SECRET-15's StripCode and the narrowed secret row — a case where a
code survives into a durable surface, or a committable message the
validator refuses; (8) EVIDENCE-12 — each fixture's pass condition
decidable with the named controls; the token-as-identity rule against
the fake's ServeHTTP path parsing (fake.go:127-170); FAIL_AT's
`<phase>[:<kind>]` against the phases the design actually has; (9)
BUILD-13's five steps — step 2's Confirm-at-cursor-write claim against
poll.go:252-260 and the existing channel fixtures, step 3 alone, and
the step-4 landing carrying the fleet sequence; (10) POLL-04's stop —
the bounds named versus the code paths that hold a Publish
(txn.go:566-611), and `steward restart` at steward_verbs.go:543-566 as
the rollout action.

Threat model, unchanged: two machines both winning; a message lost; a
message never confirmable (the poison case); replay across machines; a
stray message becoming the human's word; the ledger as a queue under
landings; the resident listener's lifetime; the secret; WORD-07
against m2's tree. Out of scope: whether the inbox should be on the
ledger at all (Wido's word); Slack push modes; email; the routing
policy deferred by POST-08; taste; the exact durations and jitter;
rotation (answer-archive's); the wording of posts.

# Mandate

1. Every mechanism the design leans on exists in the tree by that name
   or the design says it is new; a claimed-existing mechanism that does
   not exist is material and reopens prose.
2. Each design point leaves the implementer no guess at a contract,
   schema, refusal shape or transition; name the guess if one remains,
   and say whether a fixture can pin it.
3. Every fixture in FCG-EVIDENCE-12 is buildable as one test with the
   pass condition it states, under the fake as revision 3 extends it;
   an undecidable one is material.
4. The five build steps of FCG-BUILD-13 are separable and ordered so
   that steps 1-3 land alone without changing what Telegram is told or
   what binds, and step 4 is the one cut-over.

# Constraints

Wall-clock budget: 30 minutes. Return per the design-critic schema
(version 3) with reviewedCommit. Read the tree; run nothing.

# Gap Rule

stop and report a gap; never fill it silently.
