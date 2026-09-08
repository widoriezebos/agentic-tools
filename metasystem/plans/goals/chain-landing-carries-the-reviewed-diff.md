# chain-landing-carries-the-reviewed-diff

- State: queued
- Priority: 2
- Sequence: 11
- Risk: severity=3 novelty=1 exposure=2 accumulation=1 basis="severity 3: a chain landing whose staged set does not contain the reviewed diff lands under the chain's message and register with a pass verdict, the landing law failing at its one job; novelty 1: one comparison of the staged tree against the chain's reviewed diff, both already on disk; exposure 2: every chain landing on every machine, but only when the seat stages the wrong set; accumulation 1: it does not compound"
- Tier: 3
- Intent: On 2026-09-07 00:25 CEST seat m1d landed commit 0b8d1bea with land.sh --chain hgvf-build1-20260906 --staged-only after its git apply of the chain's reviewed diff had FAILED on three files (main had moved under the chain); the staged set held only four dispositions records, yet the landing passed, took the chain's feature message and registered the chain as carried. The evaluator judged the staged tree on its own merits and never asked whether it contained the reviewed diff. DONE means: a chain landing compares the staged metasystem subtree against the chain's latest reviewed diff (artifacts/agents/<root>/rounds/<n>/diff.patch and review.json) and refuses with chain-diff-absent when any file the diff changes is unchanged in the staged set or absent from it; a landing fixture stages a records-only set against a real chain and proves the refusal; the error names the missing files and the round.
- Origin: main
- Next step: MECHANICAL, one chain: internal/landing (the chain stage of land.sh) reads the root's latest round diff, lists its changed paths, and refuses when the staged tree's blobs for those paths equal the base tree's; fixture in the landing fixture suite. Verify first on the 0b8d1bea case by replaying its staged set against the chain in a throwaway clone.
- OpenedAt: 2026-09-06T22:26:51Z
- Revision: 3
- Labels: headless-fleet, headless-process
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T22:26:51Z 8N6SFVRDAKFY800KSWNKE88FG5-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=chain-landing-carries-the-reviewed-diff
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 edit actor=human:Wido targets=chain-landing-carries-the-reviewed-diff
- 2026-09-08T15:57:47Z GFXRNHCJ8S1J8JZ5ARVZRGF5Z9-m1-7cd0bd60 set-priority actor=human:Wido targets=chain-landing-carries-the-reviewed-diff reason=priority-order subject=chain-landing-carries-the-reviewed-diff from=unranked to=2:11 requested-sequence=11
Integrity: sha256=ba4406cdbb29af82440a660b4c5fda209e5b5706beb51c9b9e43ae483f2db84a
