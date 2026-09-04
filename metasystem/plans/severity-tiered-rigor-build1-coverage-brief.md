Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Coverage round on chain str-build1c

The landing gate refused part one on one number: the package
metasystem/internal/goalbudget measures 93.3% test coverage against
its ratchet floor of 95.5% (scripts/agents/coverage-ratchet.json). The
five-member tuple added branches its tests do not reach. Add tests in
that package only (no production change) until
`cd metasystem && go test ./internal/goalbudget/ -cover -count=1`
reports at least 95.5%: the review-round member's validation (negative,
above the ceiling, zero for tier 1), the four-member legacy parse and
its render as five, the conf notation with five members and its
malformed forms, and the intent-args round trip. Then
`bash scripts/agents/coverage-delta.sh ./internal/goalbudget` green.
Declare the boundary as every file that differs from main.

Wall-clock budget: 25 minutes. Gap rule: stop and report a gap.
