# rearm-remedies-and-adoption-notes

- State: claimed
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: wording and two adoption edge cases, nothing unsafe; novelty 1: small edits in files that just landed; exposure 2: operators reading a refusal, and any re-adoption; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Four notes the third code critique (err-review3-20260906) left on the engine re-arm landing ea8c3ead7, none material: adoption refuses a detached target outright where it should continue and report that automatic re-arm stays off; adoption overwrites an existing metasystem.steward.landing-ref where the human verbs preserve one; the not-landed refusal remedy names only the terminal though a merely stale remote-tracking ref is repaired by one fetch of the configured remote, so the remedy names both; and a machine re-arm that fails before any stage transition with a non-drift error (missing notification channel, missing runner directory, lock error) gets the remedy 'repair the named enrollment publication failure' regardless of cause - the remedy names the actual error class.
- Origin: main
- Next step: Chain rra-build1 (DESIGN-BEARING for item five alone) builds from plans/rearm-remedies-and-adoption-notes-brief.md (bced1868): adopt.sh continues on a detached target and keeps a preset landing ref; up.go names the fetch in the not-landed remedy and the actual class in the before-mint remedy; a human arm with changed enrolled bytes beside a live runner takes the replace path and mints its own witnessed generation; a test per item. Then one Fable critique, close, records, receipt, land.sh --chain --test-receipt, and the all-three-green rearm receipt stays owed to the rearm-launch-fails scenario (still red on the honest bed).
- OpenedAt: 2026-09-06T11:17:34Z
- Revision: 10
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
Integrity: sha256=9428dac8ecc51cb42d54a71db60119dad79fc797acba02bdbf9aa3ee70f21218
