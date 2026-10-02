# fleet-forgets-an-unreachable-seat

- State: approved
- Priority: 1
- Sequence: 9
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Releasing claims of a seat that is in fact alive would let two seats work one goal (severity 2); existing presence, claim and pin records (novelty 1); every fleet (exposure 2); rare act (accumulation 1)"
- Tier: 2
- Intent: A person removes an unreachable seat from the fleet in the UI (Wido 2026-10-02), e.g. a machine that is gone or whose checkout is not on this computer: one button on its fleet card, and the same act as a verb (UI parity), e.g. 'metasystem machine forget NAME'. It drops the seat's presence from the fleet view, releases its claims and clears pins that name it (each released goal returns to the ready list), keeps its pushed branches and records untouched, and says plainly what it did. Only for a seat that is unreachable; a reachable seat on this computer is stopped and removed through machine-remove instead. A person's act; idempotent.
- Origin: main
- Next step: Smallest design: what makes a seat unreachable (no presence for a set time, or no checkout on this computer), the verb and the fleet-card button; build; land
- OpenedAt: 2026-10-02T21:14:43Z
- Revision: 5
- Pinned: ui
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T21:14:52Z revision=2 opid=XRQ07ZVDSNKTWKD998Y6CMM25G-m1e-9c612d71 authority=proven digest=1bf31cc6246bef2c814691fb3c0607b4d70ed066d0ae6d209dad12cc82c710f0 episode=2

History:
- 2026-10-02T21:14:43Z FGMBYJ6Y46YB2AYB1F7CMHRJHZ-m1e-9c612d71 open actor=human:Wido targets=fleet-forgets-an-unreachable-seat
- 2026-10-02T21:14:52Z XRQ07ZVDSNKTWKD998Y6CMM25G-m1e-9c612d71 approve actor=human:Wido targets=fleet-forgets-an-unreachable-seat
- 2026-10-02T21:15:00Z 7ZYVDXNF8TJKA1JEKHK4W937RQ-m1e-9c612d71 set-pin actor=human:Wido targets=fleet-forgets-an-unreachable-seat
- 2026-10-02T21:15:09Z VBHF2TJR7TH6MTG8W6CDZ3ZJXW-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror reason=priority-order subject=fleet-forgets-an-unreachable-seat from=unranked to=1:8 requested-sequence=8
- 2026-10-02T21:18:43Z MGWN9QEKSDG62Q0ZBFCY6EKEHZ-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-panel-ux,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent,work-review-starts-its-critic reason=priority-order subject=fleet-panel-ux from=1:8 to=1:9 requested-sequence=2
Integrity: sha256=2ca1fedb5b0e130c0c4e7eeea9c9c598a32a6a37824036c594033c4ff17713d4
