Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Critique brief, round 2: the channel gateway, design revision 2

FINDING IDS: chain-unique; continue the chain's numbering at FCG-C-16;
a re-opened round-1 finding keeps its own id. Never F-n.

Round budget: this is round 2 of the goal's three; R-60-m1's stop
rule: the loop closes the first round with no material finding, and a
material finding changes what gets built and names the artifact it
would change. Round 3 is the failsafe round: after it, only a
demonstrated requirement failure or a shape-level defect reopens
prose; a remaining finding that a fixture can express becomes an
obligation row and the build starts.

Subject: metasystem/plans/fleet-channel-gateway-design.md, revision 2,
a single-pass rewrite after round 1 (fifteen material findings, all
accepted; the dispositions table at the end of the design joins
against the round-1 return under the chain's round-1 directory, and
`validate critique-closed` confirms the join). Wido's decisions quoted
in the head are not up for review. Ground it against the tree at main
e526a54e (the round-1 base plus the legacy-question landing a7959217
and the design landing); a cite that no longer holds is a finding only
if the mechanism moved, not if the line number did.

What changed and where to attack it, in order: (1) the location
(INBOX-02): plans/channel/ is claimed invisible to every existing
engine because ReadCommitGoals lists only plans/goals/ and
records/goals/ — find any reader, validator, projection, transport,
conformance or landing path that walks plans/ and would refuse or
mis-handle a JSON directory there, and check the three step-1 fences
against the guard and path-classes as they are; (2) the shared opid
(INBOX-02): the deterministic ULID and the literal `inbox` machine
segment against validOpidShape, the journal (an entry per machine
under one opid, TakeOver of a dead owner's entry, PushedBlocking), the
replay branch of Publish (an entry already confirmed on this machine),
TrailerPresent on a long history, and the answer history row carrying
the shared opid — find a path where the second machine's Publish is
not idempotent confirmed, or where one machine's journal blocks its
own next message; (3) the receive contract (RECEIVE-03): Ack, Confirm
and the batch-prefix rule against Telegram's offset semantics
including the confirming call's own returned update, filtered updates,
and a batch whose first item fails validation forever (a poison
message that stops the prefix for every machine); (4) the schemas and
the refusal table (INBOX-02): a legal transition the validator
refuses, an illegal state it accepts, the answer-state row's decidability,
the secret row's false positives on ordinary text (a six-digit
number in a fact), and whether at-rest validation plus Mutate-side
transitions leave a gap the CAS does not close; (5) rule (b)
(MATCH-06): a message carrying a token for question X while X and Y
are open, the option-label match against short labels ("yes", "no"),
the `unbound` hint post as a channel for abuse or noise, and the
three-per-hour list ceiling; (6) the post protocol (POST-08): the
intent's stale takeover against a slow but alive poster, the
receipt path's phases, and whether the asker-with-token path (post
then Publish in one transaction) is consistent with the rest; (7)
ANSWER-11 against m2's landed code (poll.go advanceAnswer,
humanauthority.VerifiedChannelAnswerProof): does the proof build from
a ledger record on a machine that did not receive the message, and
does the deterministic approval opid survive goal.Approve's own
request shape; (8) POLL-04 lifecycle against runner.go and
steward_verbs.go as they are; (9) MIGRATE-10: the refusal until
`channel migrate`, a migrate that runs on two machines, and an old
engine still polling locally while a new one listens; (10) the fake
controls and the ten fixtures (EVIDENCE-12): each decidable, each
stageable with the named controls; (11) SECRET-15: any durable
surface still unnamed (the fake's journal, the runner log, the
question's `facts` written by an agent, the commit message).

Threat model, unchanged from round 1: two machines both winning; a
message lost; replay across machines; a stray message becoming the
human's word; the ledger as a queue under landings; the resident
listener's lifetime; the secret; WORD-07 against m2's tree. Out of
scope: whether the inbox should be on the ledger at all (Wido's word);
Slack push modes; email; the routing policy deferred by POST-08; taste;
the exact durations and jitter; rotation (answer-archive's).

# Mandate

1. Every mechanism the design leans on exists in the tree by that name
   or the design says it is new; a claimed-existing mechanism that does
   not exist is material.
2. Each design point leaves the implementer no guess at a contract,
   schema, or refusal shape; name the guess if one remains.
3. The ten fixtures and the unit tests are each buildable as one test
   with a decidable pass condition under the fake as revision 2
   extends it; an undecidable one is material.
4. The six build steps of FCG-BUILD-13 are separable and ordered so
   that steps 1 and 2 land alone without changing any live behaviour,
   and step 3 is the one cut-over.

# Constraints

Wall-clock budget: 30 minutes. Return per the design-critic schema
(version 3) with reviewedCommit. Read the tree; run nothing.

# Gap Rule

stop and report a gap; never fill it silently.
