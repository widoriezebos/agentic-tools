# seat-mutual-awareness

- State: claimed
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: the seat-to-seat path sits beside the human's channel, and a seat message that reached the human's authority surface would be an unauthorized word, though the design forbids it and nothing else unsafe is permitted; novelty 2: new record kinds, verbs and a health role on a ledger and validator that already exist; exposure 3: every machine in the fleet and the shared ledger; accumulation 2: in-flight records and questions grow on the ledger with the fleet and unanswered questions pile up silently unless surfaced"
- Tier: 3
- Intent: Wido's order 2026-08-31: seats must be aware of each other and ask each other questions directly, without the human as relay - the m3-to-m2 seam check of this day routed through Wido when it should have been seat-to-seat by default; DONE means a seat can discover what other seats have in flight and put a question to them as the normal, mechanized path
- Origin: main
- Next step: DESIGN LADDER, ROUND 2 IN REVIEW (m1d, 2026-09-06 evening). Revision 2 of the design landed (2e109816, 928 lines) answering all ten round-1 findings with a dispositions table; the second design review (Sol, job sma-crit2-20260906) is running. Two choices in revision 2 are for Wido's eyes because they change the rollout: (1) the first writer is fenced by an enable marker Wido writes ONCE at the terminal after every machine has rebuilt; every seat writer refuses while it is absent, and an upgraded validator refuses a tip an old checkout spoiled (first-commit-wins leaves no room for two engines reading one tip differently); if he wants no human act in the rollout the marker goes and finding SMA-C-02 reopens. (2) The build is four slices (records and validator with no writer; enable, membership, presence and seat fleet; ask, answer, close, wait, show; the health role and digest lines), estimated 315 to 430 actual job-minutes over two to three days with two closing code reviews; with the dispatcher reserving 120 minutes per job the accounting needs about 2000 reserved job-minutes and 20 attempts, and the elapsed limit must be three days: the raise to ask Wido for before slice 1 dispatches (over the tier norm, his word). After the review: fold or close, then the raise, then slice 1.
- OpenedAt: 2026-08-31T14:24:17Z
- Revision: 19
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T14:42:11Z revision=15 opid=2FSZ9E6JVBHXWDHG222WQY5GK5-m1-7cd0bd60 authority=proven digest=735ea5ba141b26ac0ffa48f224d2c878485edaa85d4989dc90faddad9ca85a1b
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=9 at=2026-09-06T10:14:44Z
- Claimed: machine=m1d lineage=main-1788683763-71870-f7f607 at=2026-09-06T15:35:03Z revision=18 accountingRevision=18
- StopCapability: generation=18 revision=18 machine=m1d claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T14:24:17Z PQVSVQQVASG56RB6DNG57JA3W9-m3-a5da21ff open actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:24:52Z 6XMZ029FWWWM1W7F3RQDP3KQ62-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:30:13Z QSR12G5EKAA1DEPBYVZXJN7E08-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:32:23Z AZZJY85CH39SZERMYHTZYKA2CE-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T17:24:03Z 809B7APA7QNP2S3843JEJ48T3H-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-09-01T14:41:43Z D4SYHJ782EQHP7BJ6EZ4MA0GB2-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=seat-mutual-awareness
- 2026-09-01T20:29:37Z A0JV3E94GEEWZ7HSSVQ27P9SXC-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=seat-mutual-awareness
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=seat-mutual-awareness reason=sweep
- 2026-09-06T10:14:23Z 2BEFGAAS7HAM93R30EKH7WHSXP-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T10:14:44Z AYJVTN1SD35837B2572QEE0V0Y-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T10:15:15Z ANZ69ZJPSJZBFFCHRC1HA913Y7-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness reason=Misclassified: from=0 to=3 evidence=root:sma-design1-20260906
- 2026-09-06T10:49:47Z 0DDEGS5XETFRVFA4T64FMVWFV5-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T10:49:50Z R1J2EQ5KGB9C7G9K1JGTA8W3KA-m1d-62183579 park actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness reason=design revision 1 landed and reviewed (ten accepted findings, fold brief landed); revision 2 cannot dispatch: the box's 240 job-minutes are spent by the design and its review; needs Wido's set-budget to the tier-3 norm at the enrolled terminal
- 2026-09-06T14:39:21Z 0PQKHVPYD7PT482YMTSHFBXN5B-m1-a4f8999f unpark actor=m1+main-1788594343-3833-fb64b9 targets=seat-mutual-awareness
- 2026-09-06T14:42:11Z 2FSZ9E6JVBHXWDHG222WQY5GK5-m1-7cd0bd60 approve actor=human:Wido targets=seat-mutual-awareness
- 2026-09-06T15:30:35Z XPRCJW6A5K4EVNYZFYQ746CTAM-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T15:32:01Z XKXGGNX5N1E6A10EH5ERFYK46V-m1d-62183579 release actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T15:35:03Z ZC7AEN6X8TQDKFCK774KCVZ941-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T15:42:22Z K05S03TQQM6BZ16AZPEBYKJYF6-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
Integrity: sha256=ab46e537e9d6afa0220c682228e7ebfa5f95af46989584bd9b7de1d72a661254
