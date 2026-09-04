Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Critique brief: the channel gateway, design revision 1

FINDING IDS: chain-unique, FCG-C-01, FCG-C-02, ... never F-n.

Round budget: 1 focused round (R-60-m1's stop rule: the loop closes
the first round with no material finding; a material finding changes
what gets built and names the artifact it would change). The goal's
box: one day, ten attempts, 1200 minutes, three review rounds; this is
the design's only planned critique before build step 1, so attack the
whole design now.

Subject: metasystem/plans/fleet-channel-gateway-design.md (revision 1),
under goal fleet-channel-gateway. Wido's decisions are quoted in the
design's head and are not up for review: one bot, one git inbox, first
commit wins, no lease and no leader, long poll with jitter, identity
checked on the committing machine. Ground the design against the tree
at main 28b5e051: the goal transaction engine (internal/goal/txn.go:
ResolveEndpoint, Publish, AlreadyApplied, the CAS push and its
classification by refetch), the ledger validators
(internal/goal/validate.go: goalsPrefix, ReadCommitGoals, ValidateTree
and where the Validate hook runs on a landing and on a goal verb), the
channel poll pass (internal/channel/poll.go: byThread, the reject
ladder, advanceAnswer, the replay register, the cursor advance), the
Telegram adapter (internal/channel/telegram/telegram.go: updates(),
the offset and timeout it sends, the 409 mapping), the answer verb and
its token rule (internal/goal/verbs.go: answerRequest,
AuthenticatedChannelApproval) as m2's goal
channel-budget-answer-binds-nothing left them, the steward run loop
(internal/steward/runner.go RunLoop and the channelphase call after the
tick), the fixture bed (scripts/agents/channel-fixtures.sh,
internal/channel/fake). The design's cites were taken at main dbe1b41e;
a cite that no longer holds at 28b5e051 is a finding only if the
mechanism moved, not if the line number did. A design claim about
shipped behavior that the tree contradicts is a finding; cite
file:line.

Threat model, in order: (1) two machines both winning: a path through
Publish where the message-id check does not run on the fetched tip, or
where AlreadyApplied is reported as a failure and the update is never
confirmed, or where two answer rows for one message can land; (2) a
message lost: anything confirmed to Telegram before its commit is
durable, including the batch-prefix rule of FCG-RECEIVE-03 against
Telegram's real offset semantics, and the 24-hour expiry with every
listener down; (3) the replay check across machines: a code step
consumed on one machine's tip that another machine does not see before
its own commit; (4) a stray message becoming the human's word: rule (b)
of FCG-MATCH-06 binds an unthreaded verified message to the single
open question, so a message the human meant for something else (a
status remark, a question of his own) is recorded as his answer on
that goal; judge whether a `reserved-decision` answered this way needs
more than "the asking machine reports it verbatim and asks again"; (5)
the ledger as a queue: channel commits on refs/heads/main interleaved
with landings, what the commit guard, land.sh's rebase loop and
conformance make of a tree containing plans/goals/channel/, and a
validator refusal that stalls the whole fleet's channel; (6) the
resident listener inside the steward: its goroutine's lifetime across
re-arm, enrollment drift and `steward tick`, and what happens on a
machine whose steward is not armed; (7) the secret: any path by which
the six digits, the TOTP secret or the bot token reach a committed
record or a log; (8) FCG-WORD-07 against the tree m2 left: whether
removing the append breaks the budget re-approval m2 built, and
whether a verified answer without the token can still be mistaken for
a binding one anywhere (`--approved-ref` readers). Out of scope:
whether the inbox should be on the ledger at all (Wido's word: "inbox
format/location" is the seat's freedom, and the ledger is the seat's
choice; a finding here must show a concrete failure, not a preference);
Slack push modes; email; the routing policy deferred by FCG-READ-08;
taste; the exact durations and jitter.

# Mandate

1. Every mechanism the design leans on exists in the tree by that name
   or the design says it is new; a claimed-existing mechanism that does
   not exist is material.
2. Each of the twelve design points leaves the implementer no guess at a
   contract, schema, or refusal shape; name the guess if one remains.
   In particular: the exact fields and JSON shape of the two records,
   the outcome vocabulary, and what ValidateChannelCommit refuses.
3. The eight fixtures and the unit tests are each buildable as one test
   with a decidable pass condition under the fake provider; an
   undecidable one, or one the fake cannot stage (FailurePoint
   "before-confirm", the 409 text), is material.
4. The seven build steps of FCG-BUILD-12 are separable and ordered so
   that step 1 lands alone without changing any live behavior, and each
   later step leaves the channel working for a machine on the old
   engine until FCG-MIGRATE-10 says otherwise.

# Constraints

Wall-clock budget: 30 minutes. Return per the design-critic schema
(version 3) with reviewedCommit. Read the tree; run nothing.

# Gap Rule

stop and report a gap; never fill it silently.
