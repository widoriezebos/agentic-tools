# return-path-form-not-stated

- State: queued
- Tier: 1
- Intent: Implementer returns are rejected with DIFF_BOUNDARY_INVALID when a diffBoundary entry does not start with metasystem/, and nothing in the delegate prompt says that return paths are repository-root relative while the implementer's working directory and the brief both speak in metasystem-relative paths. Two chains were lost this way on 2026-09-04 (pcr-build1 and dss-build2, both Codex): each cost a preserve branch and a carry chain. DONE means the dispatcher's prompt states the path form in one sentence next to the return schema, and the return validator, when every offending entry resolves to an existing file under metasystem/, normalizes the entries by prefixing metasystem/ instead of failing the round; a genuinely unknown path still fails.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a prompt sentence and one normalization in the return validator with its test): build, go test ./internal/returnschema/... ./internal/dispatch/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T10:09:41Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T10:09:41Z HCPAT9EXDY5D0WGNG0BF4JB31K-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=return-path-form-not-stated
Integrity: sha256=032803b8238b119218dc5dea6137821450cafd3fbdf1a49828b8960ece603641
