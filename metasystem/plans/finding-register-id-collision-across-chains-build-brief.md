Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal finding-register-id-collision-across-chains)
Date: 2026-09-04

# Goal

Goal finding-register-id-collision-across-chains (tier 2, approved by
Wido's word "the bugs you mentioned are approved to fix too"). Its
record, metasystem/plans/goals/finding-register-id-collision-across-chains.md,
is the contract. In short: the finding register
(metasystem/internal/dispatch/finding_register.go,
refuseCrossRootClassConflict) unions findings by their bare id across
EVERY chain root in the job records, so two unrelated reviews that
both number a finding F-1 collide ("conflicting rigor classes ...
waiting on the original critic or the human is the only remedy");
cancelling the stuck round does not unblock the implementer's
follow-up, and a critic follow-up to re-issue ids is refused because
the fold itself fails. Specimen: 2026-09-04, str-build3-cc1 against
str-build1c-cc1, which forced a fresh implementer chain from a
preserved branch.

# The change

1. Finding identity in the register is scoped by the reviewed
   SUBJECT: the union across roots applies only to critic roots that
   review the same implementer chain (the same `--reviews` target or
   the same reviewed tree), which is the lawful union of design point
   STR2-CRITIC-UNION-11 (metasystem/plans/severity-tiered-rigor-design.md);
   roots reviewing different subjects never conflict, whatever their
   ids. Implement in refuseCrossRootClassConflict (and the register
   state it reads): resolve each root's subject from its job record
   (the reviews reference and the reviewed tree) and compare classes
   only within one subject.
2. Diagnostics: the conflict message names both subjects and the
   remedy; a conflict within one subject keeps today's text.
3. Fixtures: two chains with F-1 of different classes reviewing
   DIFFERENT subjects both advance; two critic roots on the SAME
   subject with F-1 of different classes still refuse; a re-issued
   round on the same root with new ids advances. Unit tests in
   finding_register_test.go; a shell leg in
   metasystem/scripts/agents/dispatch-fixtures.sh only if the
   existing critique legs there run in the sandbox (say so if not).

# Gate

`cd metasystem && go build ./... && go vet ./internal/dispatch/ && gofmt -l internal/dispatch` (empty);
`go test ./internal/dispatch/ -count=1 -timeout 20m` green.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 2). Declare the boundary as
every file that differs from main. Gap rule: stop and report a gap
with your proposed contract written out.
