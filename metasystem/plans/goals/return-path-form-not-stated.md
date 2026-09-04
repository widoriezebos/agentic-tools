# return-path-form-not-stated

- State: approved
- Tier: 1
- Intent: Implementer returns are rejected with DIFF_BOUNDARY_INVALID when a diffBoundary entry does not start with metasystem/, and nothing in the delegate prompt says that return paths are repository-root relative while the implementer's working directory and the brief both speak in metasystem-relative paths. Two chains were lost this way on 2026-09-04 (pcr-build1 and dss-build2, both Codex): each cost a preserve branch and a carry chain. DONE means the dispatcher's prompt states the path form in one sentence next to the return schema, and the return validator, when every offending entry resolves to an existing file under metasystem/, normalizes the entries by prefixing metasystem/ instead of failing the round; a genuinely unknown path still fails.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a prompt sentence and one normalization in the return validator with its test): build, go test ./internal/returnschema/... ./internal/dispatch/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T10:09:41Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T16:28:01Z revision=2 opid=YB558NZ1FPCN00GF93FTJR6PEK-m2-5fcf08ab authority=relayed digest=62ffc627b05630720edcf332178ff866484a2c5b0a3a5ab222ccbfde413c5b29 reviewBy=2026-09-06

History:
- 2026-09-04T10:09:41Z HCPAT9EXDY5D0WGNG0BF4JB31K-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=return-path-form-not-stated
- 2026-09-04T16:28:01Z YB558NZ1FPCN00GF93FTJR6PEK-m2-5fcf08ab approve actor=human:Wido targets=return-path-form-not-stated authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
Integrity: sha256=5b48473b80a23381d040cfb6f6a23f9db0d515509c52ec0b70883a30397d0eae
