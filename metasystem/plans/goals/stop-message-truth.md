# stop-message-truth

- State: approved
- Priority: 3
- Sequence: 19
- Intent: The stop message reflects the actual state of the system: the live ledger projection (claims, real next steps) plus whether work is in flight right now - a stale snapshot that says nothing about activity must be impossible (Wido 2026-08-20). ABSORBS open-work-scanner-blindspots (parked, R-33 merge): KI-34 - the scanner equates work with this-checkout dispatch job records, so cross-checkout worktree jobs and non-job in-flight processes (background critiques, verification runs) are invisible; it cries wolf or gets silenced by wording. One stop-hook honesty seam, one goal
- Origin: main
- Next step: After cutover retargets the verdict to the new projection (that half is a backlog-git-sync cutover obligation), fold in the steward's status surface: the stop message names any live worker, in-flight continuation, or pending steward incident, so silence is never ambiguous; acceptance = the frozen-ledger staleness observed 2026-08-20 cannot recur. Proposal from m2 (2026-08-24, for the primary dispatch delegate's ratification): extend the idle-watchdog predicate to be ledger-aware — arm the watchdog on every enrolled checkout, and revive/notify when goal next names a tokened, unclaimed, unblocked item AND the machine holds no claim; that makes the distributed backlog itself the wake signal, so a machine with standing work never sits idle behind a stale stop message. SIGHTING 2026-09-06 (m1, first Stop after c1525b90a): the Stop verdict named 244 run records from mid-August ('looks hung', 'ended ended-unknown', 'finished green; the run record says: no continuation recorded') in one refusal beside the single actionable line. Runs that finished weeks ago with no recorded continuation are not open work: the scanner ages them out or reports them as one class-and-count line; the printed shape is stop-refusal-fits-on-one-screen's, the judgement of what is open work is this goal's. open-work-scanner-blindspots stays parked as merged here (Wido tried to resume it 2026-09-06; the verb refuses an unclaimed parked goal, so nothing to resume). JUDGEMENT DEFECT TRACED, m1, 2026-09-09: 27 of the 32 run-warning lines on every Stop are runs that finished GREEN in August and still print 'looks hung'. Cause: internal/report/scan.go line 288 sets RunFact.Hung from record.HungSince != nil with no status check, so a run that was once past its stale window keeps the flag after it ends; and decideRuns in internal/goal/turnverdict.go warns on Hung with no Acked check, so run ack does not silence it (proven today: acking the 5 terminal red/ended-unknown runs removed those 5 lines; the 27 green hung-flagged runs stayed). Both blockers named in this goal are done, and the printed SHAPE is landing under stop-refusal-fits-on-one-screen today, so what remains here is small and mechanical: a terminal run is never hung (clear or ignore HungSince once status is terminal), a hung warning respects ack, and run prune's 14-day rule applies to hung-flagged terminals like any other. Prune today dropped only 3 of 151 records; 91 unacked greens remain and print once per new session.
- OpenedAt: 2026-08-20T16:51:00Z
- Revision: 12
- BlockedBy: backlog-git-sync, idle-watchdog
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T14:41:18Z revision=9 opid=S7X3P2Y3TVZ95AMFPCA1V6JQJH-m1-7cd0bd60 authority=proven digest=7de8068c1f0781a1c549bd5a054db748d34ccd1a2e8551e6c34be5db0f5fabe2

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=stop-message-truth
- 2026-08-24T10:19:34Z W0J5R9MB25P2BN3GH0G6Z0QD6K-m2-bc1be9cb edit actor=m2+mac-coordinator targets=stop-message-truth
- 2026-08-24T10:20:23Z E0FHAVYVRQF8WQWHFB36W0RJVV-m2-bc1be9cb edit actor=m2+mac-coordinator targets=stop-message-truth
- 2026-08-31T06:40:15Z P5Y2E0MQB9GVYZ2HPVEWCQ4XVW-m2-bc1be9cb edit actor=m2+mac-coordinator targets=stop-message-truth
- 2026-08-31T19:10:00Z VK1T06JA27BG0RSCAHQ0YXX38Q-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=stop-message-truth
- 2026-09-01T20:29:47Z X1Q5JHGZKDW5FWHSBJTQRK94BM-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=stop-message-truth
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=stop-message-truth reason=sweep
- 2026-09-06T14:33:34Z TJJ3PRKXTFQ70HPTF4V0GE24NR-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=stop-message-truth
- 2026-09-06T14:41:18Z S7X3P2Y3TVZ95AMFPCA1V6JQJH-m1-7cd0bd60 approve actor=human:Wido targets=stop-message-truth
- 2026-09-08T16:00:13Z NNMRKPG3AJZKTYNMNE2X4MQ91G-m1-7cd0bd60 set-priority actor=human:Wido targets=stop-message-truth reason=priority-order subject=stop-message-truth from=unranked to=3:19 requested-sequence=19
- 2026-09-09T09:38:56Z 1X533VJC5EGSNY01MNR9CAS19H-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=stop-message-truth
- 2026-09-09T09:39:23Z 03VGMX7VHB8K9YNNR5908ZY0W4-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=stop-message-truth
Integrity: sha256=f249af0f76d5ea8dd9ef83c7a00544ef0b2740b7b2e1b991e10aace559aa0267
