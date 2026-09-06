# seat-mutual-awareness

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: the seat-to-seat path sits beside the human's channel, and a seat message that reached the human's authority surface would be an unauthorized word, though the design forbids it and nothing else unsafe is permitted; novelty 2: new record kinds, verbs and a health role on a ledger and validator that already exist; exposure 3: every machine in the fleet and the shared ledger; accumulation 2: in-flight records and questions grow on the ledger with the fleet and unanswered questions pile up silently unless surfaced"
- Tier: 3
- Intent: Wido's order 2026-08-31: seats must be aware of each other and ask each other questions directly, without the human as relay - the m3-to-m2 seam check of this day routed through Wido when it should have been seat-to-seat by default; DONE means a seat can discover what other seats have in flight and put a question to them as the normal, mechanized path
- Origin: main
- Next step: DESIGN LADDER STOPPED AND RAISED (m1d, 2026-09-06 evening). Three revisions, three reviews (registers records/misc/seat-mutual-awareness-critique-r1.md, -r2.md, -r3.md: ten, ten, five material findings). Revision 3 (278d8b9d) is the design on main. What converged: presence records and the fleet view, asks and answers as ledger transactions under the machines' own identities with durable deadlines and one outcome per transition, recovery from journal entries for every writer, immutable retirement, the lineage transport, a total validator, no seat-authored words on any human surface, the proof matrix, an honest box. What did not: the rollout fence. The design fences the first writer with an enable marker Wido writes once at the terminal plus a human-only repair verb for a spoiled tip; the third review shows the marker is forgeable from ledger history, the repair exemption cannot travel through the transaction hook and its fallback bypasses the engine's safety, and repairing another machine's record contradicts the writer identity rules (SMA-C-21, 22, 23), plus a deadline-second boundary (C-24) and a slice order that lets a seat ask before targets can hear (C-25). THE QUESTION FOR WIDO: (A) rule the rollout operational: every machine pulls, rebuilds and re-arms before any seat writer runs (the engine-rearm law makes a pull a rebuild), the validator refuses a spoiled tip, and a spoiled tip is repaired by an existing human act at the terminal (a hand commit with the guard acknowledged, or goal repair), so the marker and the repair verb are removed; revision 4 is small and one closing review likely closes; residual accepted by Wido: an old checkout can only spoil the directory by a deliberate hand landing. (B) keep an engine fence: revision 4 must design a commit-bound, terminal-proven marker and a lawful repair transaction path inside the goal engine (a new transaction kind), several more rounds and engine work before slice 1. (C) implementation-first per D81: build slices 1 and 2 behind fixtures with the marker as designed and let the code reviews judge it; the reviewer already showed the marker forgeable, so this ships a fence that is not one. The seat recommends A. After the ruling: revision 4, one review, then the build box (the design counts seventeen reservations, 2400 job-minutes, three days, twenty attempts, three review rounds for the four slices; smaller under A). Box used so far since re-approval: three reservations, 360 of 720 job-minutes.
- OpenedAt: 2026-08-31T14:24:17Z
- Revision: 22
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T14:42:11Z revision=15 opid=2FSZ9E6JVBHXWDHG222WQY5GK5-m1-7cd0bd60 authority=proven digest=735ea5ba141b26ac0ffa48f224d2c878485edaa85d4989dc90faddad9ca85a1b
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=9 at=2026-09-06T10:14:44Z

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
- 2026-09-06T16:17:28Z V79PMJYQCBR6GPFHPGXY3CDKPT-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
- 2026-09-06T16:17:31Z KH9HNYP3VZ55E83FFXVSDYNXZ4-m1d-62183579 park actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness reason=design ladder stopped after three revisions and three reviews; the rollout fence (a human enable marker plus a repair verb) does not converge and the choice between an operational rollout rule and an engine fence is Wido's; the seat recommends the operational rule
- 2026-09-06T21:20:55Z 31VB5VYX31N6PAWK15G318HEPD-m1d-62183579 unpark actor=m1d+main-1788683763-71870-f7f607 targets=seat-mutual-awareness
Integrity: sha256=01759c575e61781dd47cf56de4cd44d06fa0e1a3835090b2c9d87159e7e3d41e
