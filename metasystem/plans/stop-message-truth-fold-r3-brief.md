Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Fold round three: the idle rule exactly as before, plus one honest suppressor

Follow-up round on chain stop-truth-build1-20260910. The second critic
(stop-truth-crit2-20260910) found two material defects in round two;
both fold here, and nothing else.

## SMT-07 (high): the idle rule was widened

The pre-chain enforceIdleBacklog returned early, counter reset,
whenever the claimable list was empty (internal/goal/project.go fills
that list from the ready frontier only, never with this machine's
claims). Round two replaced that gate with "claimable is empty AND
claimed is empty" and added a block-source tweak, so a seat holding a
claim with nothing else claimable is refused twice and handed to the
steward on the third stop even while the display shows STILL WORKING
or WAITING ON THE HUMAN, and prints "IDLE WITH BACKLOG: 0 claimable
goals ... : ;". Five pre-existing tests were rewritten to assert the
new behaviour. Restore the pre-chain gate and block-source logic
exactly (empty claimable list: early return, counter reset, no idle
block), restore those five tests to their pre-chain assertions, and
drop the tweak. The only additions that remain from rounds one and two
are the suppressors: a running job of this checkout, a live proof
attempt of this checkout, a held landing lock, each counted as work in
flight (so the counter resets and STILL WORKING names it), and the
CLAIM HELD line when a claim is held and nothing is in flight. When
claimable goals exist and nothing is in flight, refusals count as
before, claim or no claim.

## SMT-06 (high): the own-attempt test can never be true

A proof attempt record names its control root (the metasystem
installation directory, where the record is filed and what the Stop
hook scans on this layout) and its execution root (the repository root
above it); round two requires both to equal the scanned root, which is
impossible, so the rule never fires and the new test passes only
because it writes one directory for both. Fix, in
internal/report/scan.go: an attempt is this checkout's when its
control root equals the scanned root AND its execution root equals
that root's repository root (the parent directory that holds the
installation, resolved the same way the hook resolves it). Rewrite the
test with the real two-directory layout and add the foreign case (a
different installation under the same repository root is not ours).

## Notes, no change

SMT-08: the landing lock covers only the landing's park step; with
SMT-06 fixed the live proof attempt covers the long part. SMT-09: the
scanner imports the dispatch package for the canonical custodian rule;
accepted.

## Mandate

1. SMT-07 and SMT-06 as above.
2. Nothing else changes.

## Proof

internal/report, internal/goal (idle, brain, render, world tests);
gofmt, vet. Report the round as your own.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
