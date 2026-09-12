Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Review brief: the Stop message's judgement of open work (chain stop-truth-build1-20260910, reviewing round stop-truth-build1-20260910-r3)

FINDING IDS: chain-unique, continue the chain's sequence at SMT-10,
never F-n. You are the third critic on this chain. Report `round` as 1
in your return: it is this job's own round. One focused round.

Why: the goal record metasystem/plans/goals/stop-message-truth.md
traces the defects; the build brief (in the plans directory under this
goal's name) binds three: a terminal run is never hung and a hung
warning respects acknowledgement (internal/report/scan.go,
internal/goal/turnverdict.go); a held claim or a live proof attempt is
not idle, so the three-strike IDLE WITH BACKLOG counter stops firing
during a seat's own landings (turnverdict.go); the brain summary lines
are capped in width so the display stays within its 4,000-rune bound
(internal/goal/verdictrender.go).

Round three folds the second critic's two material findings
(stop-truth-crit2-20260910) and nothing else. SMT-07 (high): round two
had widened the idle rule, refusing a seat that holds a claim with
nothing claimable and rewriting five pre-chain tests; round three
restores the pre-chain gate exactly (an empty claimable list returns
early and resets the counter, the block source is unconditional again,
the five tests are byte-equivalent to their pre-chain versions) and
keeps only the in-flight suppressors (a running job of this checkout, a
live proof attempt of this checkout, a held landing lock) and the CLAIM
HELD line. SMT-06 (high): the own-attempt test required an attempt's
control root and execution root to both equal the scanned root, which
this layout never satisfies; round three matches the control root to
the scanned installation and the execution root to that installation's
containing repository, with a real two-directory fixture and a foreign
case (another installation under the same repository is not ours).

Attack first: the restored gate byte for byte against the pre-chain
code, with the suppressors layered on it and nothing else (diff the
five tests against their pre-chain versions yourself); the two-root
match on this layout (control root = the installation directory the
hook scans, execution root = the repository root above it) and its
foreign case; a proof attempt of this checkout still counted after its
deadline or terminal block; the escalation path (a claim held with
claimable work and nothing in flight still counts refusals and hands
the seat to the steward on the third). Closing review: unless something
material remains, say so plainly so the chain lands.

Threat model: a run still hung after its status went terminal by any
path; an acknowledged hung run still printing; the idle rule reading a
proof attempt as live after its deadline; a held claim silencing a real
idle forever; the counter no longer resetting on a backlog change; the
brain cap dropping names from the verdict FILE rather than the display;
any change to the printed shape beyond the cap and the CLAIM HELD line;
anything outside the boundary.

Scope: the computed diff of the implementer job (its diffBoundary
names internal/dispatch/ownerlock.go, internal/goal/order_test.go,
turnverdict.go, turnverdict_brain_test.go, turnverdict_idle_test.go,
verdictrender.go, verdictrender_test.go, internal/report/scan.go,
scan_test.go, internal/run/run_test.go). Contract: the goal record and
the build brief.

# Mandate

1. SMT-07 and SMT-06 are closed as described, with tests that would
   fail on the round-two shapes.
2. The three original rules still hold; nothing outside the boundary
   changed; the printed shape is unchanged except the brain cap and the
   CLAIM HELD line.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so). The orchestrator
runs the packages, the goal-cli bed and the hook suite on the seat.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
