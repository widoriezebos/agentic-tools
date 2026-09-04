Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal human-carried-landing)
Date: 2026-09-04

# Critique brief, round 2: the human-carried landing, design revision 2

FINDING IDS: chain-unique; continue the chain's numbering at HCL-C-18;
a re-opened round-1 finding keeps its own id. Never F-n.

Round budget: this is round 2 of the goal's three; R-60-m1's stop
rule: the loop closes the first round with no material finding, and a
material finding changes what gets built and names the artifact.

Subject: metasystem/plans/human-carried-landing-design.md, revision 2,
a single-pass rewrite after round 1 (sixteen material findings, all
accepted; the dispositions table at the end of the design joins
against the round-1 return.json under the chain's round-1 directory,
and `validate critique-closed` confirms the join). Ground it against
the tree at main, the same base as round 1 plus the design landing.

What changed and where to attack it, in order: (1) the word is now a
goal-history row under the enrolled-terminal proof or the authenticated
channel answer (02, 04), the temporary word excluded — is the identity
gate now exactly as strong as `goal set-budget`'s, and is the channel
path really the existing question/answer grammar with no new envelope?
(2) the transaction (06): four ordered writes, consumption defined as
the `carried` row or the `Carry:` trailer on origin/main — find a
crash point or a race that lands twice, lands nothing while consuming,
or leaves the obligation unwritten with no recovery; (3) the advisory
gates in commit.sh (05): is every judgement gate named, and are the
two remaining refusals truly record failures and not judgement? (4)
the rebase rule (05): does "no push with a tree the row did not name"
hold on the retry loop too? (5) the obligation and its two discharges
(07) against the existing schema and verbs, and the `commit:` critic
subject (09) at all three seams; (6) the audit grammar (03): is the
regex pair plus exclusions decidable and complete against the tree as
it is, and does the pending-row rule let slice 1 land alone?

Out of scope: whether the feature should exist (Wido's ruling); taste;
durations; wording.

# Mandate

1. Every mechanism the design leans on exists by that name or is
   declared new in the build list; a claimed-existing mechanism that
   does not exist is material.
2. Each design point leaves the implementer no guess at a contract,
   schema, refusal shape or write order; name the guess if one remains.
3. Each of the thirty-eight fixtures has a decidable pass condition.
4. A round-1 finding whose amendment does not actually answer it is
   re-opened under its own id with the reason.

# Constraints

Wall-clock budget: 30 minutes. Return per the design-critic schema
(version 3) with reviewedCommit. Read the tree; run nothing.

# Gap Rule

stop and report a gap; never fill it silently.
