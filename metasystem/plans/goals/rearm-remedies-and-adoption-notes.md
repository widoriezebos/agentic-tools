# rearm-remedies-and-adoption-notes

- State: claimed
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: wording and two adoption edge cases, nothing unsafe; novelty 1: small edits in files that just landed; exposure 2: operators reading a refusal, and any re-adoption; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Four notes the third code critique (err-review3-20260906) left on the engine re-arm landing ea8c3ead7, none material: adoption refuses a detached target outright where it should continue and report that automatic re-arm stays off; adoption overwrites an existing metasystem.steward.landing-ref where the human verbs preserve one; the not-landed refusal remedy names only the terminal though a merely stale remote-tracking ref is repaired by one fetch of the configured remote, so the remedy names both; and a machine re-arm that fails before any stage transition with a non-drift error (missing notification channel, missing runner directory, lock error) gets the remedy 'repair the named enrollment publication failure' regardless of cause - the remedy names the actual error class.
- Origin: main
- Next step: STATE 2026-09-06 18:21Z (m1c): chain rra-build1 round one built all five items (Sol, brief bced1868); Fable critique r1 (rra-critic1) found two material lows: the arm-lock remedy named a holder the blocking flock can never produce, and a detached target with a preset landing ref got the not-seeded note; both folded in round two (rra-build1-r2, fold brief 8c81f033) with a third adopt leg and a pinned fallback sentence; dispositions records/misc/rearm-remedies-and-adoption-notes-critique-r1-dispositions.md (0c475f54). Seat-side proof on the folded tree (eeb990d9): up and steward packages green, vet, gofmt, bash -n clean. Critique r2 (rra-critic2, the last review round) running on the whole chain diff. The adopt fixture bed's frozen gate (full race+coverage suite) failed once under load 7 on three unrelated timing tests (sightings recorded, 59b34696); rerunning the bed on the folded tree now. Then: r2 dispositions, register advance, close, land.sh --chain (area width), go-build, goal done. The all-three-green rearm receipt stays owed to the rearm-launch-fails scenario.
- OpenedAt: 2026-09-06T11:17:34Z
- Revision: 11
- Pinned: m1c
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:16:40Z revision=3 opid=STXTKSF0EBWQFXA3AEE2ARS7XJ-m1-7cd0bd60 authority=proven digest=faed688e447a25e760743da415acf6b80b523cde838b44ce8af37d9d52eb228e
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=8 at=2026-09-06T17:13:54Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T17:13:06Z revision=8 accountingRevision=8
- StopCapability: generation=8 revision=8 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T11:17:34Z QPGB1R343FQPHD5FHKC2G58XTB-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=rearm-remedies-and-adoption-notes
- 2026-09-06T11:42:28Z BTY2B0FT8G2RNNPTVS2A2C1NW7-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=rearm-remedies-and-adoption-notes
- 2026-09-06T12:16:40Z STXTKSF0EBWQFXA3AEE2ARS7XJ-m1-7cd0bd60 approve actor=human:Wido targets=rearm-remedies-and-adoption-notes
- 2026-09-06T12:45:49Z FK9682XKEEA1CTD10X5D3ZV9EN-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=rearm-remedies-and-adoption-notes
- 2026-09-06T12:45:59Z ZJQ7FJW5K6QT2KERKACQEVZ0TD-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=rearm-remedies-and-adoption-notes
- 2026-09-06T13:35:55Z PZRYP0257S6QQJZWMBMKC9FSNQ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=rearm-remedies-and-adoption-notes
- 2026-09-06T15:43:32Z AY3C6SMZYWRV6EQMP40T5XVDBQ-m1-7cd0bd60 set-pin actor=human:Wido targets=rearm-remedies-and-adoption-notes
- 2026-09-06T17:13:06Z Q0Y7Q24ARA42P25T0AN6CSSDJG-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=rearm-remedies-and-adoption-notes
- 2026-09-06T17:13:54Z Q0C37QD404SGMFFAN15XKAT0Q3-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=rearm-remedies-and-adoption-notes
- 2026-09-06T17:14:17Z QNQ3R3VKQRPT3410BDDNVC2S4T-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=rearm-remedies-and-adoption-notes
- 2026-09-06T18:21:05Z M063C7JZCJQMPGRDXPEAYAXVQ3-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=rearm-remedies-and-adoption-notes
Integrity: sha256=57bf12c76adffe32ad1b9f37ca8404263356750849bf73dc641a0f6f3557665a
