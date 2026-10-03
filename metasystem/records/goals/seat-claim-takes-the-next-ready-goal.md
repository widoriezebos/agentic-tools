# seat-claim-takes-the-next-ready-goal

- State: done
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="A stalled seat wastes a start and idles (severity 2); a small change in an existing verb (novelty 1); every seat (exposure 2); every seat start (accumulation 2)"
- Tier: 2
- Intent: A seat never stalls on a goal another seat took first (Wido 2026-10-02: 'This should be machinery behaviour'): when a steward-started seat's goal claim names a goal that another machine has claimed in the meantime, the claim itself takes this machine's next ready goal and says which, and only answers 'nothing claimable' when the frontier is empty. The seat brief does not need to handle it.
- Origin: main
- Next step: Build: in the claim verb for the steward-seat lineage, fall back from a taken named goal to the machine's next ready goal; test with two machines and one goal; land
- Concluded: Landed 3681afe27: a steward seat whose named goal another machine took claims its machine's next ready goal and says which; persons keep the refusal; code review 0 material
- OpenedAt: 2026-10-02T21:06:51Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T21:06:57Z revision=2 opid=ECXBGNTD6MPJX3JJEPGN43FX8Z-m1e-9c612d71 authority=proven digest=676f2d401048b6a613285d53cd7b5b43e4b516244b1ec504efa5baccac15842c episode=2

History:
- 2026-10-02T21:06:51Z HKKY7NCV27P0HC74EDX26V571C-m1e-9c612d71 open actor=human:Wido targets=seat-claim-takes-the-next-ready-goal
- 2026-10-02T21:06:57Z ECXBGNTD6MPJX3JJEPGN43FX8Z-m1e-9c612d71 approve actor=human:Wido targets=seat-claim-takes-the-next-ready-goal
- 2026-10-02T21:07:03Z G9DB1SGW3QTW5NESM169XX6XPF-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-claim-takes-the-next-ready-goal,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,work-review-starts-its-critic reason=priority-order subject=seat-claim-takes-the-next-ready-goal from=unranked to=1:1 requested-sequence=1
- 2026-10-02T21:07:09Z ECK4XWDQ6V6QASBCH90V2VPW3K-m1e-9c612d71 set-pin actor=human:Wido targets=seat-claim-takes-the-next-ready-goal
- 2026-10-02T21:23:24Z 87SK1HNSS72691NDAZ1HW4NHXK-m1e-9c612d71 done actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-panel-ux,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-claim-takes-the-next-ready-goal,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent,work-review-starts-its-critic
Integrity: sha256=0b02fdd7135388aabafd7df1d7c9f62c67b7aeca46a14662edaeb91b44f6dbb2
