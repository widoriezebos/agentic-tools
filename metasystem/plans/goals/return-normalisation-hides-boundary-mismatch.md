# return-normalisation-hides-boundary-mismatch

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A return validator change may let an undeclared changed path through or misreport it; every chain landing relies on that check; the fix is one ordering or one message."
- Tier: 2
- Intent: Since the return path-form landing (b7d119b3), the dispatch fixture's diff-boundary-mismatch leg fails with 'agent fixture diff-boundary-mismatch did not report: changed paths fall outside the cumulative implementation boundary': the leg writes an untracked source.txt at the conformance job's workspace root, runs validate conformance --stage review, and expects that refusal; it then declares diffBoundary ["source.txt"] and expects the review to pass. The new normalisation in internal/validate/returncomplete.go (checkDiffBoundary) either refuses earlier with DIFF_BOUNDARY_INVALID or rewrites the entry so the cumulative-boundary check in internal/validate/conformance.go (line 474) no longer sees the mismatch. Seen seat-side on m2 2026-09-04 18:47Z. DONE means an undeclared changed path is still reported with that exact text, a declared workspace-root path the fixture uses still passes, the normalisation only touches entries that resolve under metasystem/, and the fixture leg passes.
- Origin: main
- Next step: One validator ordering or message fix with a unit test, then the fixture leg: build, go test ./internal/validate/..., run dispatch-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T18:48:51Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-04T18:48:51Z X13ZT6QSZ7RCNHVTZA05ZWWEYS-m2-5fcf08ab open actor=human:Wido targets=return-normalisation-hides-boundary-mismatch
Integrity: sha256=c46910a7a016ea7314aca809ab4f10948205e8ca4ca841270b0703bc6f8a064b
