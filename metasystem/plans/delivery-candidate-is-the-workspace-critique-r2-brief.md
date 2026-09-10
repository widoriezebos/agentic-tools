Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Review brief: second read of the workspace-candidate design, revision 2

FINDING IDS: chain-unique, continue the register: DCW-08, DCW-09, ... never F-n.

## What you are reading

`metasystem/plans/delivery-candidate-is-the-workspace-design.md`, revision
2, at commit bc164d658 in your worktree (a local scaffold commit: the page
lands on `main` together with this read), sha256
dca76908f7538f987844abdec61d699c58fb13d552381395ce2d0d27c75601ae, folded
by job dcwl-design1-20260910-r2 on the Fable lane from your first read
(`metasystem/artifacts/agents/dcwl-context/critique-r1.md`, with the
coordinator's dispositions at its end). The "Declared Outputs" digest line
the dispatcher stamps into your prompt is the digest of the outputs
manifest, not of the design. The goal record is
`metasystem/plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md`;
the fold brief is
`metasystem/artifacts/agents/dcwl-context/design-fold-r1-brief.md` (unlanded; it lands with the chain).

Write your register as this new file: `metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r2.md`
If your runtime is read-only, return it in the job return as you did in
read 1; the coordinator projects it.

Read-only design critique; implement nothing, run no bed. A material
finding is a place where two implementers would build different things, or
a claim the page rests on that the tree does not support, or a decision
that fails the goal's own sentence. This is fold-read cycle 1: the page
should now be buildable; say so plainly if it is, and name the fold if it
is not. Read the code at 2fbd77535 and the two patches under
`metasystem/artifacts/agents/dcwl-context/` as before.

## Settled, do not re-derive

The dispositions in the read-1 register are binding: the key is the
selected component execution identities, W is the recorded floor and fast
path; site 9 is today's `test verify` call; the two engine-bed constraints
are constraints on that chain; recertification stays exact; the two legacy
ledger paths are in; the counselor rationale is corrected. Decisions 1.2,
1.3, 2.1 and the twelve-site inventory stand.

## Mandate, in order of consequence

1. **Your reopening triggers.** Read 1 named a reopening trigger per
   finding. Go through DCW-01 to DCW-06 and say, one line each, whether
   revision 2 meets the trigger. Where it does not, that is a finding.
2. **Section 4.4: the observer computes the verify core.** `landing
   observe` now runs `verifyRetainedTesting` at commit time when the
   receipt carries `workspace`, adding a detached worktree and the input
   hashing to that verb. The designer flagged the alternative it did not
   take (commit.sh passes its own `test verify` result through a new flag)
   and why (a second cutover in commit.sh's call shape). Decide whether
   4.4 is right, and check the two computations agree: commit.sh's `test
   verify` at `:272` and the observer's core run on the same index a
   moment apart; name what can differ between them and whether the
   observer must refuse when its result is not sufficient while
   commit.sh's was.
3. **Section 3, site 5 and site 6.** At the end of a battery the rule is
   "exact, then W": a non-ledger index move refuses today's message and
   the attempt "finalizes as a success without a committed receipt", so the
   next `landing test-receipt` composes through `CreateTestingReceipt`
   with zero execution. Check that chain of claims against
   `metasystem/cmd/metasystem/test.go:648` and the attempt finalization
   around it, and against `metasystem/internal/proofrun/test_result.go:263`
   (exact reuse requires a committed receipt). Site 6:
   `PublishCommittedReceipt` gains the accepted index tree from `landing
   test-receipt`'s `--tree`; check that no other caller of
   `PublishCommittedReceipt` (`metasystem/cmd/metasystem/landing_verbs.go:154`
   and `:191`) is left without an accepted tree.
4. **Section 4.3, step 5 in the shell.** Site 8 runs `test verify` on the
   staged index before commit.sh runs it again; check the cost bound the
   page cites (`suite.section-cap-min` through `limits.sectionCap`) is the
   bound that applies to a read-only verify, and that a verify refusal
   there keeps land-fixtures' grep at `:834` true.
5. **Section 8.2, the receipt-cutover leg.** The leg rewrites a receipt
   without `workspace` and shims `bin/metasystem` so `landing workspace`
   is "unknown verb". Check the leg proves the older-engine plus
   new-land.sh pair and not only the shim; say whether the shim must also
   refuse a receipt WITH the field (an old engine's `DisallowUnknownFields`
   behaviour) for the leg to be honest.
6. **Section 8.3, the shared fixture scripts.**
   `metasystem/scripts/agents/fixture-bed-scenarios.sh` and
   `metasystem/scripts/agents/fixture-budget.sh` are sourced by other beds.
   Check the per-scenario ceiling (`bed-scenario`, base 120 s, scaled 8 to
   48) cannot cut a healthy leg of another bed, and that the
   poll-and-kill loop it reuses (`fixture-budget.sh:359-369`) reaps the
   child's process group, not only the child.
7. **Section 11 completeness.** Name any reader of `receipt.Tree`,
   `TestResult.CandidateTree`, `ExactReusableTestResult` or
   `PublishCommittedReceipt` the files list misses, and any test the page
   names that does not exist at 2fbd77535.

## What is not in your mandate

Style, length, the scoping of section 6, and re-deriving read 1.

## Return

One register at the path above: findings by id (DCW-08 onward), each with
the section, the claim, the evidence (file and line at 2fbd77535 or patch
line), and what changes if the finding stands; then the six one-line
trigger verdicts of mandate 1. End with a verdict line: buildable as is,
or one fold naming which findings. Wall-clock budget: 35 minutes.
