Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Review brief: first independent critique of the workspace-candidate design

FINDING IDS: chain-unique, DCW-01, DCW-02, ... never F-n.

## What you are reading

`metasystem/plans/delivery-candidate-is-the-workspace-design.md`, revision
1, landed at commit 440081320, sha256
0977891a22c591090d75d69f0476826c632d579114717e5f950b1a45d30413ac, authored
on the Fable lane (job dcwl-design1-20260910). The "Declared Outputs"
digest line the dispatcher stamps into your prompt is the digest of the
outputs manifest, not of the design; the design's identity is the sha256
above. The goal record is
`metasystem/plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md`
and the brief the page answered is
`metasystem/plans/delivery-candidate-is-the-workspace-design-brief.md`.

Write your register as this new file: `metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r1.md`

Read-only design critique; implement nothing, run no bed. A material
finding is a place where two implementers would build different things, or
a claim the page rests on that the tree does not support, or a decision
that fails the goal's own sentence. This is fold-read cycle 0; a material
finding here is expected, not resented. Read the code at the revision the
page names (2fbd77535) and the two unlanded patches it cites under
`metasystem/artifacts/agents/dcwl-context/`.

## Settled, do not re-derive

- T (the exact tree the proof ran on) stays recorded everywhere; the
  index-versus-worktree drift check inside one checkout stays exact
  (section 2.1). Verified: no declared group input is a ledger path
  (`metasystem/testing.json`: only `fast-static-build` declares anything
  under `memory/`, and that is `memory/rulings.md`, which stays in W).
- The page's correction of the brief's site-2 claim is right:
  `componentDecisionLocked` (`metasystem/internal/proofrun/attempt.go:558`)
  keys on goal, accounting revision and per-group identities, never on the
  tree, so a repeat after a ledger-only move returns reusable success
  today; the new-attempt failure appears only under the engine-bed chain's
  whole-tree stamp.
- One file, two declarations (section 1.2) instead of one widened slice:
  the reason (the carry chain's drift and advance verbs give the register
  slice append-only semantics a goal file does not have) is verified in
  the carry patch. Not in dispute.
- The recertified path stays exact with a named follow-up (section 6).
  Test the reason if you like; the scoping is the orchestrator's.

## Mandate, in order of consequence

1. **Is W the right projection, or is the declared-input projection?**
   The goal's sentence defines W as the tree minus the ledger paths goal
   verbs write, and the page follows it. The contract's own design
   (`metasystem/plans/application-testing-contract-design.md:231`) says
   something wider: acceptance "compares its component input identities
   against the current candidate ... rather than requiring irrelevant
   whole-tree object-id equality". Measured on `origin/main` for the 24
   hours before this brief: 375 goal-verb commits and 45 other landings,
   and most of those 45 touch only `plans/` and `records/` (briefs,
   critique registers, design pages, merge commits). Under the page, a
   records-only landing by another seat between the receipt and the
   landing still voids the receipt: W differs, site 8 refuses, and site 9
   refuses with `workspace-changed-after-proof`, where today's
   `test verify` at site 9 would pass it because group identities are
   input-keyed (section 2.2). Decide: does W as defined satisfy "we should
   never have to wait until it gets quiet", or must the cross-tip
   comparison be the declared-input projection (every path that no
   selected group declares is irrelevant), with W as the goal's floor? If
   the input projection, say what proof is lost when a group's real reads
   exceed its declared inputs, and whether the contract already accepts
   that loss through component reuse. If W stands, say what the seat does
   on a records-only move and whether it is cheap: the page's recovery is
   a zero-execution re-receipt and "lands again", but the rebased commit
   is already on the local branch and `land.sh` has no path that pushes an
   existing commit (the page names that gap in one line and defers it).
2. **Section 4's stricter rule.** After the rebase the page refuses on any
   workspace difference, including an upstream change outside every
   declared input, where today `verify` passes. The brief asked for "a
   workspace change in A..B still refuses" and the page took it
   literally. Weigh it against the goal's intent and against mandate 1;
   if the refusal should be scoped to declared inputs, say so and name
   the fixture that changes.
3. **Section 3: dropping `BaseCommit` and `PolicyBaseCommit` from exact
   reuse.** The page argues `PlanDigest` over W plus the four digests
   cover what the two commits contributed. Find a case where the merge
   base or the policy base changes the plan or the base contract without
   changing W, the digests, or the selected groups. If none exists, say
   so in one line.
4. **Section 2.3 and the cutover.** `DisallowUnknownFields` makes an older
   engine refuse a receipt carrying `workspace`. The landing that lands
   this change runs under the enrolled (older) engine and takes its
   receipt with that engine, so the receipt has no `workspace` field and
   the exact rule applies to that landing. Confirm the cutover is
   self-consistent, and check the interaction with commit.sh's proof
   engine (`metasystem/scripts/agents/commit.sh:313`, built from the
   candidate) reading a receipt the enrolled engine wrote.
5. **Section 5, the two constraints on the engine-bed chain.** Constraint 1
   (the stamp is a function of the ENGINE projection, never T or W): check
   `enginePaths` at `metasystem/internal/behaviorsurface/policy.v2.json`
   is the right set and that two candidates with equal engine projections
   really yield byte-identical engines under `go-build.sh --trimpath`.
   Constraint 2 (`retainedCandidateEngineDigest` matches on W): read the
   patch lines the page cites and say whether W is the right key there or
   whether the engine digest should be keyed by the engine projection too.
6. **Section 1.1: the exclusion set.** `records/counselor` is out of W on
   the argument that only goal verbs append it and no group declares it.
   Check the counselor's other readers. Check whether any goal verb writes
   a path the set does not name (the page sampled forty commits).
7. **Section 8: the fixtures.** The shell legs take the first-transition
   policy path (no `testing.json` on the seed's `main`) because the bed
   cannot enroll an engine, so the enrolled-engine path is covered only by
   unit tests and by this chain's own landing. Say whether that is enough
   for a DESTRUCTIVE-REACH change to the landing gate, and whether the
   two-minute ceiling holds for a leg that runs a contract receipt inside
   the bed.
8. **Scope.** The build touches `gittree`, `landing`, `proofrun`,
   `cmd/metasystem` (`test verify --receipt`, `landing workspace`),
   `land.sh` at two sites, the refusal register, the docs rule, three
   shell legs and unit tests in four packages. Name any piece that can be
   a follow-up without weakening the goal's DONE, and any piece the page
   forgot (readers of `receipt.Tree` or `CandidateTree` it did not list).

## What is not in your mandate

Style, the length of the page, and section 6's scoping. Do not re-derive
the twelve-site inventory; check it for omissions instead.

## Return

One register at the path above: findings by id, each with the section,
the claim, the evidence (file and line at 2fbd77535 or patch line), and
what changes if the finding stands. End with a verdict line: the page is
buildable as revised, or it needs one fold naming which findings.
Wall-clock budget: 40 minutes.
