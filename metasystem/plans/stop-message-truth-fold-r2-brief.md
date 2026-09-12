Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Fold round two: a held claim does not silence the steward

Follow-up round on chain stop-truth-build1-20260910. The critic
(stop-truth-crit1-20260910) found two material defects and three notes;
four fold here. The first corrects a rule the seat's own brief stated
wrongly.

## SMT-01 (high): the claim-holder escalation is gone

Round one returns from the idle-backlog check as soon as this machine
holds a claim, zeroing the refusal counter, so a claim-holding seat can
sit idle forever and the third-strike escalation (the steward continues
the seat's own goal, recording a real continuation intent) is
unreachable; two tests that proved it were inverted. The critic is
right and the seat's brief was wrong. The rule, decided: a held claim
never silences idle by itself. Only in-flight work does: a running job
of this checkout, a live proof attempt of this checkout, a held landing
lock. When the seat holds a claim and nothing is in flight, the verdict
prints "CLAIM HELD: <goal id>; nothing in flight" AND counts the refusal
exactly as before ("refusal N of 3 for this unchanged backlog"), and at
the third the escalation hands the seat's own held goal to the steward
as it did before this chain. Restore the two inverted tests to their
pre-chain assertions and add one: a held claim with a live proof
attempt counts no refusal; the same claim with the attempt ended counts
one.

## SMT-02 (medium): a proof attempt is live only for its own checkout

The new attempt reader in internal/report/scan.go treats every attempt
record under the scanned installation as this seat's work, but an
attempt names both its control root and its execution root, and a
proof launched by another checkout sharing the control root would reset
this seat's counter. Fix: an attempt counts as live only when its
control root and its execution root both name this checkout (the
scanned root), besides having no terminal block and a start within its
deadline. Test both a foreign attempt (ignored) and an own one (live).
Keep the file's header promise that another checkout's activity never
suppresses this checkout's goal, and say so in the comment.

## SMT-03 (low): call the canonical lock-custodian rule

The new landing-lock reader copies the owner-lock custodian rule
(internal/dispatch/ownerlock.go, ownerHolderProbe) and the edit deleted
the header sentence recording that argv matching is retired from this
path. Call the canonical rule instead and restore the sentence.

## SMT-04 (low): the seen shape, end to end

Add one display test walking an UNACKNOWLEDGED green hung-flagged
terminal record (the 2026-09-09 shape) to an empty display, including
the case where the record's process cannot be classified (no "liveness
unknown" line for a terminal record).

## SMT-05 (note, no change)

The CLAIM HELD line can become the headline when nothing else fills the
ladder, and names the first held claim; one claim per machine, so one
name. Recorded.

## Mandate

1. SMT-01 to SMT-04 as above.
2. Nothing else changes.

## Proof

internal/report, internal/goal (the idle, brain and render tests),
gofmt, vet. Report the round as your own.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
