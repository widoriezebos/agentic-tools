# tests-observe-only-what-they-inject

- State: done
- Priority: 1
- Sequence: 58
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="Lane test findings 368, 373, 375, 377, 382 (twice), 384 (2026-10-17): six load or leftover reds in one night; two of two hand gates failed on the legacy-worker test killing testenv's own witness child found in the host process table; every gate and proof is a coin toss until tests observe only injected facts. Step-back analysis of 08:05, Astra-critiqued."
- Tier: 3
- Intent: Tests observe only what they inject: a test or fixture never reads the host's process table, stores or leftovers, so host load and other packages' survivors cannot turn a green suite red; the known offenders are repaired first, then an audit with a shrink-only baseline holds the line.
- Origin: human
- Next step: U1 (D1): the legacy-worker launch test and two siblings take the injected process table (0 production lines); brief FINAL; Codex build in -inject, Opus read, gate, hand landing. U2 (D2): the supervise arming helpers register with the fixture custodian. U3 (D3): the audit with its baseline.
- Concluded: U1 (d4d5ea7c64, rebased by hand from be78739444d9f9b826f161096c2c914bcc4b2f8a over four records commits) landed by hand on 2026-10-19 after the sharded gate GATE OK 16:02 (code tree 34e658d5, evidence gates/tests-observe-u1-20261019) and a fresh full proof of main at 17:00; U2 (the binary-owned tag on arming helpers and the death witness) and U3 (the audit with its shrink-only baseline) continue in tests-observe-only-what-they-inject-2
- OpenedAt: 2026-10-17T08:10:36Z
- FirstClaimAt: 2026-10-19T12:37:18Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-17T08:11:14Z revision=3 opid=87931EGN3PHGYAW9EP4MKGAPJG-m1e-c6925449 authority=proven digest=5e7012c2dd3c88e81d101b55ecba55977a551562f6d9e1a7cd84ca51baef3792 episode=3

History:
- 2026-10-17T08:10:36Z HVE3V00E5T0CE8GAC8JQGHC6TR-m1e-c5a1e456 open actor=human:Wido targets=tests-observe-only-what-they-inject
- 2026-10-17T08:10:45Z 9HR77N83TYN5CTNQ66A3TW4QB0-m1e-c5a1e456 set-priority actor=human:Wido targets=tests-observe-only-what-they-inject reason=priority-order subject=tests-observe-only-what-they-inject from=unranked to=1:59 requested-sequence=append
- 2026-10-17T08:11:14Z 87931EGN3PHGYAW9EP4MKGAPJG-m1e-c6925449 approve actor=human:Wido targets=tests-observe-only-what-they-inject
- 2026-10-19T12:33:39Z F40E38EKT8XF89X65TNPJ5RYKR-m1e-f2f350ed done actor=human:Wido targets=a-persons-channel-carries-only-acts-and-deliveries,a-proof-unit-ends-on-time,a-proof-unit-ends-on-time-2,a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary,tests-observe-only-what-they-inject reason=priority-order from=1:59 to=1:58
- 2026-10-19T12:37:18Z QVA5V5793TYJ311RBDP4TQFAFG-m1e-f456f182 claim actor=human:Wido targets=tests-observe-only-what-they-inject authorityOutcome=HUMAN_AUTHORITY_PROVEN authorityGeneration=99 reason=areas unknown for goal tests-observe-only-what-they-inject in design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/tests-observe-only-what-they-inject.md; areas unknown for goal asks-clear-on-their-effect; areas unknown for goal lane-first-run-fixes; areas unknown for goal lane-recovers-and-pauses
- 2026-10-19T15:08:50Z 3WJG6JEYT11DJMMN233FE4SAY0-m1e-f2f350ed done actor=human:Wido targets=a-persons-channel-carries-only-acts-and-deliveries,a-proof-unit-ends-on-time-2,a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary,tests-observe-only-what-they-inject displaced=m1e+main-1790454088-93948-21671b@2026-10-19T12:37:18Z
Integrity: sha256=54dc5e9eb459f9a536f0013d22c221faec13de4545f909823f000532e7e569e0
