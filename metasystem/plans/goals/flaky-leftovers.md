# flaky-leftovers

- State: claimed
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Production lock hand-off in diskstore and fixture custody: a wrong release can drop a writer's scratch or leave a child unowned (severity 2); a new lock hand-off mechanism (novelty 2); engine-internal, no adopter surface (exposure 1); a one-time cleanup (accumulation 1)"
- Tier: 2
- Intent: Retire the three flaky-test leftovers: WriterDrain's wall-clock re-probe of fork-inherited writer locks, the eight fixture-custody exit bounds, and the single-pid seams still swapped in sequential tests
- Origin: main
- Next step: Design page, Astra critique under the materiality stop rule, then build and code critique
- OpenedAt: 2026-10-02T10:06:58Z
- Revision: 8
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 1
- NormApproval: approvedRef=ANGQZ8JA50DM808KSFM7MSTG7P-m1e-f456f182 minutes=720 reviewRounds=20 goalRevision=7
- Approved: by=human:Wido at=2026-10-02T11:43:48Z revision=8 opid=ANGQZ8JA50DM808KSFM7MSTG7P-m1e-f456f182 authority=proven digest=d6c3c7bad571b3b81e3b30e022954dd761b39daf04d82a77cb1675f39dd6bf3a episode=8
- Sliced: machine=m1e lineage=main-1790454088-93948-21671b revision=5 at=2026-10-02T10:31:11Z
- Claimed: machine=m1e lineage=main-1790454088-93948-21671b at=2026-10-02T11:43:48Z revision=8 accountingRevision=8 episodeAt=2026-10-02T10:16:27Z episodeRevision=5
- StopCapability: generation=8 revision=8 machine=m1e claimEpoch=9 fenceEpoch=0

History:
- 2026-10-02T10:06:58Z 540D7DS4S8P8ASGCXWMA4YF0H5-m1e-9c612d71 open actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:07:05Z YC9HTGKNZT616F94APVV8Y3EF1-m1e-9c612d71 approve actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:07:17Z 579SWA1TCRBSKPCFAG9YKCXH72-m1e-9c612d71 set-priority actor=human:Wido targets=flaky-leftovers reason=priority-order subject=flaky-leftovers from=unranked to=1:26 requested-sequence=append
- 2026-10-02T10:07:22Z JKP2211RSXVGR48HBWQADSZ8SJ-m1e-9c612d71 set-pin actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:16:27Z D2VVM51K6YGVSFX2CXP5K6KMG6-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=flaky-leftovers
- 2026-10-02T10:31:11Z DC1D6XXEK6S7T8J6TQ7MVNSQKM-m1e-f456f182 slice-start actor=m1e+main-1790454088-93948-21671b targets=flaky-leftovers
- 2026-10-02T11:18:55Z 12MX4NHKB4BQ0HJPWDFNCMDCPR-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,flaky-leftovers,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-deploys-the-engine,landing-lane-runtime-redesign,old-lane-plumbing-is-deleted,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-successor-continues-a-handoff-without-a-human,seat-works-without-a-person,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,work-review-starts-its-critic reason=priority-order subject=flaky-leftovers from=1:26 to=1:1 requested-sequence=1
- 2026-10-02T11:43:48Z ANGQZ8JA50DM808KSFM7MSTG7P-m1e-f456f182 set-budget actor=human:Wido targets=flaky-leftovers
Integrity: sha256=8c9f5c2992827a12aabb1762b69c55d974b9efcce64fe4375b542b23e98a9e13
