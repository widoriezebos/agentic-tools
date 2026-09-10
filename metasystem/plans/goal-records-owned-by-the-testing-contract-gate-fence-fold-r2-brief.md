Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Fold round two: the gate-fence group stops providing the runtime-custody obligation

Follow-up round on chain gate-fence-cadence1-20260910. The critic
(gate-fence-cadencecrit1-20260910) proved round one changed nothing:
the group section/gate-fence-fixtures also declares the obligation
`runtime-custody`, which the runtime-custody surface lists as critical,
and the planner puts every provider of a critical obligation into the
standard tier at every delivery mode; the deep entry was redundant.
Clearing the group's obligations list flips every probed changed-path
set to "not selected" while the contract still validates, since
runtime-owner-standard, section/supervision-go-fixtures and
section/supervision-and-census-fixtures still provide the obligation.

## Mandate

1. In metasystem/testing.json set the `obligations` list of the group
   `section/gate-fence-fixtures` to empty (keep the group, its section
   and its cadence membership). Round one's removal from the
   runtime-custody deep list stands.
2. Prove with the contract check verb, `go test ./internal/testpolicy/`,
   and one probe of the real selection (a changed path under
   metasystem/internal/dispatch at mode auto) that the section is
   neither required nor selected for a landing, and is still in the
   cadence set.
3. No other file changes.

## Proof

The commands in 2 and `git diff --stat` showing only
metasystem/testing.json. Report the round as your own.

## Constraints

Wall-clock budget: 10 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
