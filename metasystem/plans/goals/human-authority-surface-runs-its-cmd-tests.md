# human-authority-surface-runs-its-cmd-tests

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a gate runs fewer tests than it could, nothing granted or destroyed; novelty 1: one more group in the shape the contract already uses; exposure 2: every landing that touches the three command files; accumulation 1: one group, no compounding"
- Tier: 2
- Intent: The human-authority surface in metasystem/testing.json (chain ha-surface1-20260910) owns cmd/metasystem/session_stop.go, goalsync_mutations.go and identity_probes.go together with their three test files, but its only delivery group, humanauthority-standard, runs internal/humanauthority; a change to those three command files is delivery-sufficient with their own tests unrun at the gate (critic note HAS-01, probe of the real Select: session_stop.go selects adapter-canary, command-interface-smoke, fast-static-build, goal-decision-standard, humanauthority-standard and the static sections; go test ./cmd/... runs only in the cadence go-engine-gate). Done means: the human-authority surface's standa || REWRITTEN 2026-09-11 (backlog consolidation; landed parts: The surface the record names never landed: main's testing.json has no 'human-authority' surface; ce59b313 added human-authority-and-channel owning internal/humanauthority, channel, governance, counselor and channel_verbs.go only; session_stop.go and identity_p): DONE: the surface that owns cmd/metasystem/session_stop.go, identity_probes.go and goalsync_mutations.go on main carries in its standard list a go unit group that runs the tests of session_stop_test.go, goalsync_mutations_test.go and identity_probes_test.go, and a Select probe for session_stop.go shows that group; contract-only landing.
- Origin: main
- Next step: One bounded correction to metasystem/testing.json: add a go unit group (cwd metasystem, run pattern naming the tests in session_stop_test.go, goalsync_mutations_test.go and identity_probes_test.go) to the human-authority surface's standard list; Sol builds, Opus critiques, contract-only landing.
- OpenedAt: 2026-09-10T08:53:23Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T08:53:23Z 8SPKHVV8JBHARP5NM156PN6M80-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=human-authority-surface-runs-its-cmd-tests
- 2026-09-11T22:08:05Z A8CG7X839JY8JNEF1PZFVDGM0F-m1-c6925449 edit actor=human:Wido targets=human-authority-surface-runs-its-cmd-tests
Integrity: sha256=05bf4f966969dd16e9af4da07bd7aab51a3c18c1db807edeac280d42638cbf7e
