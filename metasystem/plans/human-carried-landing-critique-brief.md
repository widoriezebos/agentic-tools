Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal human-carried-landing)
Date: 2026-09-04

# Critique brief: the human-carried landing, design revision 1

FINDING IDS: chain-unique, HCL-C-01, HCL-C-02, ... never F-n.

Round budget: 1 focused round (R-60-m1's stop rule: the loop closes
the first round with no material finding; a material finding changes
what gets built and names the artifact it would change). The goal's
box: one day, ten attempts, 360 minutes, three review rounds; this
round is the design's only planned critique, so attack the whole
design now.

Subject: metasystem/plans/human-carried-landing-design.md (revision 1),
under goal human-carried-landing (Wido's ruling R-75-m3, quoted in the
design's head). Ground it against the tree at main 36649427, where
part two of the tiering machinery is law: the four risk answers and
the derived tier (internal/goal/file.go RiskRecord, DerivedTier,
GateWidth), the review obligations and `goal accept-risk` and `goal
discharge-review-obligation` (internal/goal), the BudgetExceptions
counter and the appetite line (internal/goal/verbs.go, the health
summary), the landing classifications and the observe verdicts
(internal/landing/observe.go, scripts/agents/land.sh,
scripts/agents/path-classes.txt), the enrolled-terminal and
temporary-word proofs (internal/humanauthority), the channel's
authenticated word and its `--approved-ref` path (internal/channel),
the critic subject reader (internal/dispatch/finding_register.go). A
design claim about shipped behavior that the tree contradicts is a
finding; cite file:line.

Threat model, in order: (1) the identity gate weaker than the design
says — an agent, a replayed word, or a stale record landing an
unreviewed tree; (2) the "never refuse a verified human" principle
turned into a refusal by construction anywhere on the path from word
to push (a check that is judgement, not identity or record); (3) the
obligation, counter and trailer not actually binding: a carried
landing whose review can be forgotten, a `goal done` that passes with
the obligation open, an exception that is not counted; (4) the carry
record and the landing disagreeing on which tree (digest binding); (5)
the audit of HCL-AUDIT-02 unbuildable as specified (the register test
that walks source for refusal constants: name what makes it
deterministic or why it cannot be); (6) the channel word form colliding
with the existing channel grammar. Out of scope: whether the feature
should exist (Wido's ruling); taste; the exact durations.

# Mandate

1. Every mechanism the design leans on exists in the tree by that name
   or the design says it is new; a claimed-existing mechanism that does
   not exist is material.
2. Each of the eight design points leaves the implementer no guess at a
   contract, schema, or refusal shape; name the guess if one remains.
3. The seventeen fixtures are each buildable as one test with a
   decidable pass condition; an undecidable one is material.
4. The build list's three slices are separable and ordered so that
   slice 1 (the audit) lands alone.

# Constraints

Wall-clock budget: 30 minutes. Return per the design-critic schema
(version 3) with reviewedCommit. Read the tree; run nothing.

# Gap Rule

stop and report a gap; never fill it silently.
