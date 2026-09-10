Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Bounded policy correction: two more packages get owning test surfaces

The landing receipt refuses two reviewed chains for unowned paths
("delivery impact is unresolved: no surface owns changed path"):
hp-terminal-grade-for-stopping-acts touches
metasystem/internal/humanauthority/** and the command files
cmd/metasystem/goalsync_mutations.go, goalsync_mutations_test.go,
identity.go, identity_probes_test.go, session_stop.go,
session_stop_test.go; stop-message-truth touches
metasystem/internal/report/**. No surface in metasystem/testing.json
owns any of them. Rule 6 of the contract design names the remedy.

## Mandate

1. In metasystem/testing.json add a surface `human-authority` owning
   `metasystem/internal/humanauthority/**`,
   `metasystem/cmd/metasystem/goalsync_mutations.go`,
   `metasystem/cmd/metasystem/goalsync_mutations_test.go`,
   `metasystem/cmd/metasystem/goalsync_verbs.go`,
   `metasystem/cmd/metasystem/identity.go`,
   `metasystem/cmd/metasystem/identity_probes.go`,
   `metasystem/cmd/metasystem/identity_probes_test.go`,
   `metasystem/cmd/metasystem/session_stop.go` and
   `metasystem/cmd/metasystem/session_stop_test.go` (skip any of the
   listed files that does not exist); dependsOn `dispatch-goal-mission`;
   standard groups: a new unit group `humanauthority-standard`
   (packages ["internal/humanauthority"], shaped like
   goal-decision-standard), `goal-decision-standard` and
   `command-interface-smoke`; deep and critical empty.
2. Add a surface `report-scan` owning `metasystem/internal/report/**`;
   dependsOn `dispatch-goal-mission`; standard groups: a new unit group
   `report-standard` (packages ["internal/report"], same shape) and
   `goal-decision-standard`; deep and critical empty.
3. Nothing else changes: metasystem/testing.json only. Prove with the
   contract check verb and `go test ./internal/testpolicy/`, and a
   probe of the real selection that a change to
   internal/humanauthority/authority.go and one to
   internal/report/scan.go each resolve as delivery with no unresolved
   impact.

## Proof

The commands in 3 and `git diff --stat`. Report the round as your own.

## Constraints

Wall-clock budget: 10 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
