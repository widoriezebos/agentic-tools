# delegate-follow-up-cannot-merge-main

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: every correction round on a busy fleet day risks a stall or a hand-done merge; novelty 2: a new lawful path through the commit guard or the dispatcher; exposure 2: every chain whose files another seat lands on; accumulation 2: the hand-done stash-and-reapply recipe will be repeated and will drift"
- Tier: 2
- Intent: A delegate's follow-up round cannot bring its worktree up to date when main moved on the chain's own files: the dispatcher warns WORKTREE-BEHIND and asks the orchestrator to merge main into the worktree, but the worktree holds the round's uncommitted changes, git refuses a merge over them, and no lawful checkpoint commit exists - the pre-commit guard admits only wrapper-token commits and the wrapper is the landing lane; a delegate in its sandbox cannot even reach the lease test the wrapper runs. Seen 2026-09-06 on chain idle-escalate-build1 after landing c1525b90 touched the same four files: the follow-up round stopped on the gap and the orchestrator resorted to a tagged stash, a fast-forward and a re-apply by hand. DONE means a follow-up round whose base is behind main on the chain's files gets a lawful rebase or merge of its worktree, either by the dispatcher before the round starts (stash, fast-forward, re-apply, conflicts left for the builder) or by a checkpoint commit verb the guard admits on agent branches, and a fixture proves it
- Origin: main
- Next step: PARKED by m1b 2026-09-06 19:50Z, budget spent (attempts 6 of 6, elapsed 4h). Chain followup-rebase-build1 stands at round 4 (reviewed tree e962f76a, diff applies to main); the first critic's four material items are folded; the fresh closing critic found three more (FRF-01 to FRF-03: a failed restore still drops the stash, brief authority runs on the dispatcher's prefixed message and refuses a modify/delete conflict forever, the paragraph never asks to stage resolved paths), accepted in plans/dispositions/follow-up-rebase-final-critique-r1.md with the round-5 fold brief plans/follow-up-rebase-fold3-brief.md ready. Needs a human budget raise (elapsed 8h, attempts 10, review rounds 3), then: claim, follow-up round 5 with that brief, conformance review, a fresh critic on round 5, close, land with --chain, receipt, done. Two orchestrator errors cost rounds 2 and 3 (an empty brief, a boundary gap) and are backlogged as empty-brief-admitted.
- OpenedAt: 2026-09-06T13:23:13Z
- Revision: 8
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:human:Wido at=2026-09-06T20:02:39Z revision=8 opid=V4YHX29QYSNQ4ABM3DSSNJGHAF-m1-7cd0bd60 authority=proven digest=ea272c834ce4af50b5950d76d474fdd3266a2f60183bc412001a2bf05c18f5f9
- Sliced: machine=m1b lineage=main-1788680071-18713-e76d5d revision=3 at=2026-09-06T16:06:37Z

History:
- 2026-09-06T13:23:13Z AGAHF86TZVX4XSR5AJQSN9NYSD-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=delegate-follow-up-cannot-merge-main
- 2026-09-06T14:49:24Z WCPJTSBBGBRS6M6F7GFEWRBWEM-m1-c6925449 approve actor=human:Wido targets=delegate-follow-up-cannot-merge-main,tracked-orig-files-litter
- 2026-09-06T16:04:56Z HTCJY4X475SZYARVZCGFYR81FK-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=delegate-follow-up-cannot-merge-main
- 2026-09-06T16:06:37Z Z2ZR9E75X5WSDVR93KPADFRG7Y-m1b-c6925449 slice-start actor=m1b+main-1788680071-18713-e76d5d targets=delegate-follow-up-cannot-merge-main
- 2026-09-06T19:48:38Z S72N105PRY4STGSP1A62A13CYY-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=delegate-follow-up-cannot-merge-main
- 2026-09-06T19:48:41Z J7DJTDSMF92JGJAP9NJK3PYHA2-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=delegate-follow-up-cannot-merge-main
- 2026-09-06T20:02:36Z EJKF266ZW8TM0WNJZY2359EFCC-m1-7cd0bd60 unapprove actor=human:human:Wido targets=delegate-follow-up-cannot-merge-main reason=re-tier to 3 so round 5 can carry a third critic round
- 2026-09-06T20:02:39Z V4YHX29QYSNQ4ABM3DSSNJGHAF-m1-7cd0bd60 approve actor=human:human:Wido targets=delegate-follow-up-cannot-merge-main
Integrity: sha256=fd31cb7eb57914a116cc417fd726e2bfede7e9122a7066722509bf45dead43c3
