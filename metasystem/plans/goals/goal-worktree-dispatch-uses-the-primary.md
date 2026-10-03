# goal-worktree-dispatch-uses-the-primary

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="changes how every critic and builder is dispatched from a goal worktree, a path every seat uses many times a day; a wrong rule blocks reviews fleet-wide, a right one removes four of tonight's six person-needing refusals"
- Tier: 2
- Intent: Every dispatch made from a goal worktree (a critic read, a commit read, a builder round, a review close) runs with the seat's armed primary engine and the primary's runtime registry and settings; the worktree is only the tree under review, never the tool. A brief the build admitted is never refused at review, and a review of a frozen unit can take a corrected brief without a builder round. Generic: holds for any adopter checkout layout; nothing names this repository.
- Origin: main
- Next step: Opened 2026-10-03 23:25 by m1e under Wido's eight-hour watch, splitting the goal-worktree family off m1j's machinery list so a second seat can take it (m1j keeps Q, R, U, V). Evidence tonight, all at goal worktrees: (1) m1g's critics ran the worktree's bin/metasystem built from the goal branch, so item P's fix on main never reached them until m1g copied the primary engine in by hand (item T); (2) m1k's critic was refused 'runtime adapter is not installed: codex' because the worktree installation has no built engine and no registry ('system check': this installation has no built engine yet); (3) m1k and m1l were refused 'engine older than this checkout, engine scripts changed' because the skew check compares the engine with the unit's own HEAD (item 55), fixed only by rebuilding the primary from main and re-arming; (4) m1l's partner-mode review is refused BRIEF_AUTHORITY_REFUSED on a frozen build brief the build itself admitted (plans/handoff-project-partner.md cited without 'new file:'), both --work and --commit forms (item S). Smallest thing that works: one resolution rule in internal/dispatch for engine, registry and settings at dispatch time (primary wins), the skew check against main, and the brief check at build time only with a --brief correction path at review. Design 1,000 words, one Astra round, then build with Codex Sol and a Claude Opus critic; estimate 1 h design, 4 h build, 2 lane cycles. First free seat claims it (m1k or m1l when their goals land; m1h between design rounds if nothing else frees).
- OpenedAt: 2026-10-03T21:24:10Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T21:24:22Z revision=2 opid=BHKEE52E8RSX311FW7XXAV9S9F-m1e-718ba0eb authority=proven digest=f2db5452b2ea675776cd7ad917b9eec4a7970f3c80c78d77a9bd471e5fd9162b episode=2

History:
- 2026-10-03T21:24:10Z 2S0NGTRKDFA87BXSDEEMXHYNZ6-m1e-718ba0eb open actor=human:Wido targets=goal-worktree-dispatch-uses-the-primary
- 2026-10-03T21:24:22Z BHKEE52E8RSX311FW7XXAV9S9F-m1e-718ba0eb approve actor=human:Wido targets=goal-worktree-dispatch-uses-the-primary
Integrity: sha256=3434f994ee8a75143eda0c9f5f105638f3b4b3563a33877c7c31988fe56565e3
