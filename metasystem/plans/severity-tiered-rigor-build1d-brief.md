Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Goal

Part one of the tiering machinery passed its final code review
(chain str-build1c, reviewed tree f00f88f1, records in
metasystem/records/misc/severity-tiered-rigor-build1-critique-cc1.md,
-cc2.md and -cc3.md) and was refused at the landing gate on one
number: metasystem/internal/goalbudget measures 93.3% test coverage
against its ratchet floor of 95.5% (scripts/agents/coverage-ratchet.json).
That chain is closed, so this fresh chain takes its exact tree and adds
the missing tests. Nothing else changes.

# Workspace

The delegate worktree the dispatcher created for this job. First
command, from the repository root:

    git cherry-pick --no-commit preserve/str-build1c-r4

(the reviewed tree on top of its own base; main has moved by records
only since, so no conflict is expected; a conflict is a gap). Then,
in metasystem/internal/goalbudget only, add tests until
`cd metasystem && go test ./internal/goalbudget/ -cover -count=1`
reports at least 95.5%: the review-round member's validation
(negative, above the ceiling, zero for tier 1), the four-member legacy
parse and its render as five, the conf notation with five members and
its malformed forms, the intent-args round trip. No production change.

# Gate

`cd metasystem && go build ./... && go vet ./internal/goalbudget/ && gofmt -l internal/goalbudget` (empty);
`bash scripts/agents/coverage-delta.sh ./internal/goalbudget` green.
Declare the boundary as every file that differs from main.

# Constraints

Wall-clock budget: 30 minutes. DESIGN-BEARING reach (the chain law
for this goal). Gap rule: stop and report a gap.
