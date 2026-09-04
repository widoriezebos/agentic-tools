# return-normalisation-hides-boundary-mismatch

- State: claimed
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A return validator change may let an undeclared changed path through or misreport it; every chain landing relies on that check; the fix is one ordering or one message."
- Tier: 2
- Intent: Since the return path-form landing (b7d119b3), the dispatch fixture's diff-boundary-mismatch leg fails with 'agent fixture diff-boundary-mismatch did not report: changed paths fall outside the cumulative implementation boundary': the leg writes an untracked source.txt at the conformance job's workspace root, runs validate conformance --stage review, and expects that refusal; it then declares diffBoundary ["source.txt"] and expects the review to pass. The new normalisation in internal/validate/returncomplete.go (checkDiffBoundary) either refuses earlier with DIFF_BOUNDARY_INVALID or rewrites the entry so the cumulative-boundary check in internal/validate/conformance.go (line 474) no longer sees the mismatch. Seen seat-side on m2 2026-09-04 18:47Z. DONE means an undeclared changed path is still reported with that exact text, a declared workspace-root path the fixture uses still passes, the normalisation only touches entries that resolve under metasystem/, and the fixture leg passes.
- Origin: main
- Next step: One validator ordering or message fix with a unit test, then the fixture leg: build, go test ./internal/validate/..., run dispatch-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T18:48:51Z
- Revision: 4
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T18:48:57Z revision=2 opid=2F3DV6H7TK4T2A8Y1DE395XARW-m2-5fcf08ab authority=relayed digest=058bd454f57c352ccae984a071becc05ec983edc0d3b95bdab7ebb65a6038c39 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T22:10:34Z
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T22:10:18Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T18:48:51Z X13ZT6QSZ7RCNHVTZA05ZWWEYS-m2-5fcf08ab open actor=human:Wido targets=return-normalisation-hides-boundary-mismatch
- 2026-09-04T18:48:57Z 2F3DV6H7TK4T2A8Y1DE395XARW-m2-5fcf08ab approve actor=human:Wido targets=return-normalisation-hides-boundary-mismatch authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T22:10:18Z HBYECFZ4JQB4G458S2Y2H7SYWC-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=return-normalisation-hides-boundary-mismatch
- 2026-09-04T22:10:34Z B4ZT35WJ45DVX2G2ZXQ47VFPRH-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=return-normalisation-hides-boundary-mismatch
Integrity: sha256=684be39bfbaffe10772ee8aeecd606645e80a0a8a09ec541177b1ccb6ec1bd27
