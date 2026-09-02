# breach-clock-and-budget-honesty

- State: claimed
- Intent: Wido's highest-priority order (2026-09-01, verbatim: 'This needs to be fixed immediately... Only resume work after these problems are fixed, proven with tests'): two of the three proven breach-machinery breakdowns from the night of 2026-08-31, sharing the goal-machinery seam. (1) THE RAISE-RESET CLOCK: SetBudget re-binds the claim record on every raise and the elapsed breach clock anchors on the current revision's claim timestamp - every budget raise restarts the breach clock (the night reset it five times lawfully; internal/goal/verbs.go SetBudget comment + internal/dispatch/budget.go anchor are the proof). (2) DISHONEST DURATIONS: budget elapsed limits parse through a working-hours grammar (d = 8 hours) and New() normalizes inputs into it - a human's 24h displays as 3d, and a human's 9d is enforced at 72 clock hours, one third of intent, silently, across every live budget.
- Origin: main
- Next step: CHAIN STATE 2026-09-02 18:25Z (m0b): design revision 3 landed (job breach-design-r3) folding Sol's round-2 register records/misc/breach-design-critique-r2.md (3 material: hold today's set-obligation governance, complete day-token inventory, complete old-writer rollout table). Sol round 3 RUNNING as job breach-design-crit3 on plans/breach-clock-critique-r3-brief.md. Next: land register records/misc/breach-design-critique-r3.md; if material, fold (Fable, cap 20) and re-critique; on zero material write plans/breach-clock-build-brief.md, Sol build (cap 120) DESIGN-BEARING, validate conformance --stage review, Fable code critique --reviews, apply the diff, land --chain. OPEN FOR WIDO (not built): should a later human set-obligation inherit a discharge consumed inside the same claim episode. Handshake goal codex-handshake-budget-load-fragile takes precedence the moment its budget is raised (Wido granted; the verb needs a human hand).
- OpenedAt: 2026-09-01T06:54:30Z
- Revision: 20
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=840 activeJobLimit=1
- Sliced: machine=m2 lineage=mac-coordinator revision=4 at=2026-09-01T07:31:13Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-02T17:29:46Z revision=18
- StopCapability: generation=18 revision=18 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T06:54:30Z XQ8RYAX5R7JBZ9DH0TX694ENCA-m2-bc1be9cb open actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T06:55:38Z 92DH70PXTT5QZTPCC72369W4M4-m2-bc1be9cb set-budget actor=human:wido targets=breach-clock-and-budget-honesty
- 2026-09-01T06:57:39Z RS2MPQXRHNER939N9HTSME2QKR-m2-bc1be9cb edit actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T06:59:59Z RD3ATKREV1PA65JBPRM7W2FYZ5-m2-bc1be9cb claim actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T07:31:13Z 877KHEAK53KDK68V19GRKK1ZJ0-m2-bc1be9cb slice-start actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T08:32:50Z F9NFJ1AEAXA7QZTPJH0AZ9WPC1-m2-bc1be9cb edit actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T08:33:08Z HDS3CM9X4WZM9DG9JP811Y5Z6H-m2-bc1be9cb release actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T08:35:35Z 3DYAHM5BQEYKR0M2Q9SABH0N4J-m2-bc1be9cb edit actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T08:38:14Z 47QGKWFWT9KM6GEBFAG9RV4PMC-m2-bc1be9cb edit actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T08:51:13Z VV4SQV2A4063R7BQH618WCV874-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-clock-and-budget-honesty
- 2026-09-01T08:53:49Z G93KRPYDX1KAHAQ9SDT224KQ7P-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-clock-and-budget-honesty
- 2026-09-01T12:44:45Z Z89Q3FCW938SWFANRBSJ4B82C4-m1-bf243850 edit actor=m1+coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T13:51:40Z BP5TR0WSVMA8YSVP9A1XMZCEGW-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-clock-and-budget-honesty
- 2026-09-02T11:37:08Z VA520J017P6874PZJ08CWEEF2V-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=breach-clock-and-budget-honesty
- 2026-09-02T12:06:41Z ETF8T5BGN6D03HM7FHSQP45W9G-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=breach-clock-and-budget-honesty
- 2026-09-02T12:15:25Z DKX4DBK9MV7GX44HMD3WQK2J7K-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=breach-clock-and-budget-honesty
- 2026-09-02T13:31:16Z K7GCW40NM2JJDZFNCMCYW07STC-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=breach-clock-and-budget-honesty
- 2026-09-02T17:29:46Z 4CTSBKH0KB4MN5GQ5BFWH2SPTM-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-clock-and-budget-honesty
- 2026-09-02T17:31:11Z AY336ZD0PYBPDS2ZMW05B7ZKJS-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-clock-and-budget-honesty
- 2026-09-02T17:48:09Z S95HRKWH2S0KC382Y1AFF9ZYT6-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-clock-and-budget-honesty
Integrity: sha256=bf00307dfccba5e3b2d7864cf7611234b907159bf15002cecce595ae734131be
