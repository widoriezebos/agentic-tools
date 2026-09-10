# human-authority-surface-runs-its-cmd-tests

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a gate runs fewer tests than it could, nothing granted or destroyed; novelty 1: one more group in the shape the contract already uses; exposure 2: every landing that touches the three command files; accumulation 1: one group, no compounding"
- Tier: 2
- Intent: The human-authority surface in metasystem/testing.json (chain ha-surface1-20260910) owns cmd/metasystem/session_stop.go, goalsync_mutations.go and identity_probes.go together with their three test files, but its only delivery group, humanauthority-standard, runs internal/humanauthority; a change to those three command files is delivery-sufficient with their own tests unrun at the gate (critic note HAS-01, probe of the real Select: session_stop.go selects adapter-canary, command-interface-smoke, fast-static-build, goal-decision-standard, humanauthority-standard and the static sections; go test ./cmd/... runs only in the cadence go-engine-gate). Done means: the human-authority surface's standard list includes a unit group that runs the named tests from those three files, and a Select probe for session_stop.go shows it.
- Origin: main
- Next step: One bounded correction to metasystem/testing.json: add a go unit group (cwd metasystem, run pattern naming the tests in session_stop_test.go, goalsync_mutations_test.go and identity_probes_test.go) to the human-authority surface's standard list; Sol builds, Opus critiques, contract-only landing.
- OpenedAt: 2026-09-10T08:53:23Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T08:53:23Z 8SPKHVV8JBHARP5NM156PN6M80-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=human-authority-surface-runs-its-cmd-tests
Integrity: sha256=c232e924ced4992d8ab90e30701e0873dc7ecdc0d0789beea93bec4d71caac63
