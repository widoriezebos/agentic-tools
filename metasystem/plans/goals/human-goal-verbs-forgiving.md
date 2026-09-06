# human-goal-verbs-forgiving

- State: claimed
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a wrong bind of a human budget act would misrecord authority, but every path stays human-only at the enrolled terminal and every refusal stays a refusal; novelty 1: a grammar, a routing table and a default on verbs that exist; exposure 2: the human's verbs only, on every machine; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Wido's word 2026-09-06, after three refused attempts to raise one goal's budget at his terminal: 'this command is too complex, not intuitive and maybe not forgiving enough'. The evidence of the day: set-budget on a parked goal refuses with 'set through goal approve with --budget'; approve --budget box together with the five limits refuses 'mutually exclusive'; the working form needs five long flags, all-or-nothing, that mirror an internal tuple; --budget takes the literal word box as its value; every human verb needs --by even though the enrolled terminal already knows who enrolled it; the same morning goal edit --risk refused for a missing --evidence whose only hint was a format string. Every refusal names flags, none prints the command that would have worked. The human should not have to know whether a goal is queued, parked or claimed to give it a bigger box. DONE means, at the enrolled terminal: one verb gives a live goal (queued, parked or claimed) a box, taking the compact form everyone already writes in prose (1d/10/720m/1/3) or a preset (norm for the tier's ceiling, keep for the standing box), and does the approval or revision rebind itself; the five explicit limits stay as the long form; --by defaults to the enrolled human's recorded name; every refusal of a human verb prints one complete command that would have succeeded with the values it saw; and a fixture drives each refusal and asserts the printed command runs. Authority does not widen: every act stays human-only at the enrolled terminal and every history line stays as it is.
- Origin: main
- Next step: RE-REVIEW RUNNING (m1d, 2026-09-06 22:50 CEST). Fold round two on chain hgvf-build1-20260906 returned (job hgvf-build1-20260906-r2, reviewed tree 7b6b47ce21ecebeab18562b484c5c55b9f9f15c9, same 19 files as round one; the sandbox's only red was the unchanged process-ownership test). Round-two critique brief landed 9e63106d; critic hgvf-cc2c-20260906 (Fable) running, the last attempt in the box (6 of 6, 720 of 720 minutes). Seat replays of go test internal/goal internal/goalbudget cmd/metasystem and goal-cli-fixtures.sh on the fold tree run outside the sandbox in parallel. NEXT: if the critic returns zero material and both replays are green: job critique-register-advance --repo . --root-job hgvf-cc2c-20260906 --round-job hgvf-cc2c-20260906; dispatch.sh close --job hgvf-build1-20260906; dispositions r1 (drafted) and r2 records under plans/dispositions/; git pull; from the repo root git apply --index --directory=metasystem metasystem/artifacts/agents/hgvf-build1-20260906/rounds/2/diff.patch; git add both records; git checkout -- records/narrator-digest.log; land.sh -m <msg> --chain hgvf-build1-20260906 --direct-fix register-carriage --goal human-goal-verbs-forgiving --staged-only --allow-new-plan --skip-transport; go-build, metasystem up; goal done. If material remains, or a replay is red, or elapsed (closes 00:05 CEST) breaches before landing: park and ask Wido for a bigger box, the act this goal fixes.
- OpenedAt: 2026-09-06T12:45:35Z
- Revision: 10
- Arc: verbs-match-intent
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:52:20Z revision=3 opid=Q0HMY137H80MMNZMS7PMA8FPSE-m1-7cd0bd60 authority=proven digest=10a4e68634e75c7789a6f26c6c7cd0feb1a6d1a9ef16d357fefbbc5afbe35498
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=4 at=2026-09-06T18:09:00Z
- Claimed: machine=m1d lineage=main-1788683763-71870-f7f607 at=2026-09-06T18:05:55Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1d claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T12:45:35Z 80R77DYNPPC53PEPJ5AHWB9Y4N-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T12:49:45Z 82VNKJKQ2ZZZJ5Q83NZX6SMEWW-m1d-62183579 set-arc actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T12:52:20Z Q0HMY137H80MMNZMS7PMA8FPSE-m1-7cd0bd60 approve actor=human:Wido targets=human-goal-verbs-forgiving,repo-root-paths-ride-agent-commits-unjudged,stop-deadline-parent-trusts-ps,stop-hook-open-work-refusal-repeats,verbs-match-intent
- 2026-09-06T18:05:55Z A4R4T4WF7DCSPD1WSK3HT743PB-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T18:09:00Z 1N25PE13SPVTN5DS53GKRN4B7V-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T18:09:22Z T07BVWFSTEMH5VBS1JH3S678RW-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T18:23:18Z NCWDDERWXP06P5CEZXJX5Q0P1M-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T19:47:45Z 1MQ9HZE3G1D7EK1CDP05QKV4HQ-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T20:04:39Z ZAZPPEV91NYX816ZQ4RJV5YKYY-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T20:35:40Z 2VTD2283WFWVBKQ43MJ04DBTN5-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
Integrity: sha256=e4581a34b81423ea2b70e5e78d58cd9980be07d65ae0355238fdf91ff9a58b52
