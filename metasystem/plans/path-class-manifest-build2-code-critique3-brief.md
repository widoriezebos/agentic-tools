Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Review brief: closing code review of the path-class manifest's second part, re-issued on m2

Round budget: 1 focused round (the closing review R-55-m1 orders; the
tier-3 budget of R-54-m1 and the stop criterion of R-60-m1 apply).

Threat model: defects the change would ship into the landing evaluator,
the commit wrapper and their fixtures; the reviewed tree contradicting
its own design; a fixture that proves less than it claims. Out: taste,
naming, and anything the design already decided.

Scope: the diff of the implementer job under review (eleven files; the
computed diff.patch is the authority, not this list). It is the m1
chain's round-5 tree re-issued byte for byte on this machine, after one
code review (PCM-CC8) whose single material finding was folded per
metasystem/plans/path-class-manifest-build2-fix4-brief.md. The design is
metasystem/plans/path-class-manifest-design.md revision 2; the closing
design register is metasystem/records/misc/path-class-manifest-critique-r2.md.

# Mandate

1. PCM-CC8-001 closed: the evaluator resolves changed paths with mode and
   ownership; adopted application paths answer outside and follow
   section 3's outside row; the three adopted-mode legs exist and the
   exact-inverse comparison is reached by a passing leg; the vendored
   layout's answers are unchanged.
2. KNOWN RED, adjudicate it: on this host the leg added to
   metasystem/scripts/agents/static-reproof-fixtures.sh in
   TestRealCommitWrapperStampsParseableObservation (a register-carriage
   landing that changes an unlisted root file `README` must refuse
   path-unclassified with the base-manifest detail) does NOT refuse: the
   commit lands. The fixture repository is a root-layout installation
   with no shipped inventory row for README, so after the PCM-CC8-001
   fold the path answers `outside` and the outside row passes carriage.
   Round 5 changed only observe.go and its test and was never replayed
   against this fixture. Decide, from the design: is the leg's
   expectation wrong (then name the path a root-layout fixture must use
   to reach path-unclassified, or the layout it must use), or is the
   evaluator's root-layout answer wrong? Name the artifact that changes.
3. The gate replay on this host for the round-5 tree: five Go packages
   green under -race (internal/landing 76.3% coverage), land-fixtures
   green (4 legs); path-class-fixtures red only for the missing ripgrep
   command (backlog item path-class-fixture-ripgrep; the same check with
   grep finds no reader of the deleted tables). Do not run
   path-class-fixtures.sh here.

A finding is material only if it changes what gets built and names the
artifact. If nothing material remains beyond mandate 2, say so and give
mandate 2's disposition; that closes the chain.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema, with
the reviewedTree from validate conformance --stage review.

# Gap Rule

stop and report a gap; never fill it silently.
