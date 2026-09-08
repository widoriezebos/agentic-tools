# follow-up-brief-cannot-cite-fresh-trunk-files

- State: queued
- Priority: 3
- Sequence: 35
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a round either burns its whole attempt reading guidance it cannot see, as round 13 did, or builds six cross-cutting behaviours without the amendment that decides them; novelty 2: admission reads the worktree base while the rebase trigger reads the chain's own paths, and the fix has to join them; exposure 2: every chain whose guidance lands mid-chain, which is every chain that folds a critique; accumulation 2: three sightings this week and it recurs every time a disposition lands between rounds"
- Tier: 2
- Intent: A follow-up brief cannot point a round at a design or record file that landed after its worktree was cut. Two rules combine: brief authority checks a cited metasystem/ path against the DELEGATE BASE commit and refuses anything newer, and the follow-up rebase fires only when trunk commits touch the CHAIN's own files, so plan and record files never bring the worktree forward. Dropping the metasystem/ prefix passes admission but does not help, because the file still is not in the worktree. Seen 2026-09-07 on chain stopverb-build1: round 13 was told to read section 14 of the stop design and records/misc/metasystem-stop-critique-r4.md, both landed on main at 984bf7c5 while the worktree sat at 8e704050; it correctly refused to implement six accepted findings without them and the attempt was spent for nothing. The orchestrator then did the stash, fast-forward and re-apply by hand. DONE means a follow-up round can cite a file that exists on the trunk tip and actually receive it, with a fixture proving both the admission and the delivery
- Origin: main
- Next step: the two seams are the admission check (cited paths judged against the base commit) and the rebase decision (internal/dispatch/followup_rebase.go plus the wrapper in scripts/agents/dispatch.sh, landed 7211d700 by goal delegate-follow-up-cannot-merge-main). Candidate shape: admission accepts a path present on the trunk tip, and the rebase plan treats a cited-but-absent path as a reason to bring the worktree forward, which is the same stash, fast-forward and re-apply the wrapper already performs. Sibling goals: follow-up-brief-cannot-cite-round-created-files covers files a ROUND created; this one covers files the TRUNK gained
- OpenedAt: 2026-09-07T14:29:13Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T14:29:13Z Z8Y2FPSCCES041S4CQ9ETC414Y-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=follow-up-brief-cannot-cite-fresh-trunk-files
- 2026-09-08T16:01:10Z NRW277N1CZKNB0Q9FC6X1YHW3M-m1-7cd0bd60 set-priority actor=human:Wido targets=follow-up-brief-cannot-cite-fresh-trunk-files reason=priority-order subject=follow-up-brief-cannot-cite-fresh-trunk-files from=unranked to=3:35 requested-sequence=35
Integrity: sha256=e91baea6263e0e70abe4317a81373301deae99c179a7113b6492a0e696c0ca89
