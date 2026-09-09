Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Review brief: first independent critique of the receipt-survives-drift design

FINDING IDS: chain-unique, LRD-01, LRD-02, ... never F-n.

## What you are reading

`metasystem/plans/landing-receipt-survives-records-drift-design.md`,
revision 1, landed at commit 5a316c26, sha256
d030a289924a02f330eb6ec4474c66e0a60ee39f69e27d1311536d030796ccb2, authored on
the Fable lane (job lrsrd-design3). The goal record is
`metasystem/plans/goals/landing-receipt-survives-records-drift.md` and the
brief it answered is
`metasystem/plans/landing-receipt-survives-records-drift-design-brief.md`.

Write your register as this new file: `metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md`

Read-only design critique; implement nothing, run no bed. A material
finding is a place where two implementers would build different things, or
a claim the page rests on that the tree does not support. This is fold-read
cycle 0; a material finding here is expected, not resented.

## Settled, do not re-derive

The page corrected the orchestrator's brief twice and both corrections are
verified in the tree: `CreateTestReceipt` reads both postures from the
isolated candidate (`metasystem/internal/landing/receipt.go` around 103 and
130), so a real-workspace append during the battery breaks only the
landing-time read; and `metasystem/internal/behaviorsurface/policy.v2.json`
lines 29-35 list `memory/receipts.log` in `coordinationPaths` and not
`records/narrator-digest.log`, which makes commit.sh's LANDING comparison a
sixth refusal site. Decision 2 (filter the projection only; index, candidate
and commit trees exact) is the orchestrator's own reading and the page
adopted it; test it, but it is not in dispute.

## Mandate, in order of consequence

1. **The autostash rides a shared stack (Decision 3b, the page's own
   riskiest part).** `git rebase --autostash` pushes the register appends
   onto the stash stack and pops them after the rebase. On this machine the
   stash stack is shared by every worktree of the repository and by other
   agent sessions that push and pop concurrently. The page's guard covers
   only git's re-apply-conflict line, and its recovery is "run git stash pop
   in this checkout", which on a shared stack may pop someone else's entry.
   Decide whether a landing may put bytes on that stack at all. If not, name
   the alternative the page's own 3d argument leaves open: the drift verb
   already knows the register paths, so the landing can copy their working
   bytes aside privately, rebase, and restore them, with no stash entry and
   no dependence on git's wording. If the autostash stands, say what makes
   the shared-stack race acceptable.
2. **The union premise.** 3b says "with merge=union their re-apply cannot
   conflict on content". Today `git check-attr merge` on the two registers
   answers: memory/receipts.log: merge: union;records/narrator-digest.log: merge: union;. Decision 1 says it pins
   `.gitattributes`; check the pin actually asserts the attribute the
   argument needs, for both paths, and what happens when it is absent.
3. **Schema 2 and the fleet (Decision 4).** A schema-1 receipt is refused,
   not translated, and `DisallowUnknownFields` makes a pre-change reader
   refuse a schema-2 receipt. Receipts are keyed by tree and short-lived,
   and the seat-boot rule refuses dispatch when the engine is older than the
   checkout, which the orchestrator hit twice today. Is that enough, or is
   there a path where a receipt is created by one engine and read by
   another on the same machine (a worktree's own binary, a steward
   continuation, m1c and m1d sharing the transport mirror)?
4. **The drift verb's rule table (3a).** Rule 3 tolerates `Y == 'M'` on a
   register "whatever X is", including `D`. Rule 4 treats `AA`, `UU`,
   `DD` as drift. Walk the porcelain v1 XY table and say whether any state
   is misclassified; the untracked class `??` counts as drift even though
   the receipt battery writes under gitignored `artifacts/`, confirm that
   holds for everything a landing legitimately leaves untracked.
5. **Decision 2's identity.** The read compares
   `FilterTree(T, excludes)` recomputed now against the recorded projection.
   Check `gittree.FilterTree` (`metasystem/internal/gittree/gittree.go`
   around 282, `update-index --force-remove`) is deterministic and does not
   error when an excluded path is absent from the tree, and that the
   filtered identity cannot collide two different candidates.
6. **The policy edit (3c).** Adding the digest to `coordinationPaths`
   changes its behaviour-surface class. The page cannot say what other
   consumers of `Classify` change; judge whether that is a gap the build
   can close by running the tests, or a decision that belongs on this page.
7. **The three things the code could not answer** (the page's last section,
   from line 537). Say which, if any, is load-bearing for the mechanism.
8. **Fixtures.** For each canary the page names, check the fail-before
   text is producible from the current tree and that the refusal canary
   (an unstaged edit to a non-register path must still refuse) cannot pass
   for the wrong reason.

## Constraints

Wall-clock budget: 45 minutes. Return per the design-critic schema with the
page digest above as the reviewed identity. Gap rule: stop and report a gap;
never fill it silently. Your sandbox cannot run the fixture beds; do not
treat that as evidence about the design.
