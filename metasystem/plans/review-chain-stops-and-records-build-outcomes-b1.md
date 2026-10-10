# Brief: review-chain-stops-and-records, unit build-outcomes-b1 (failed-step execution and holds)

Working Mode: Implement
build-outcomes-b stopped at the size cap (about 320 lines). Read its handoff at /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/review-chain-stops-and-records-build-outcomes-b-handoff.md: its evidence of the owners on this tree (stepDriver in internal/launch/read_sequence.go, advanceRunning/roundCause/countedRounds in internal/launch/unit_run.go, syncBuildHolds in cmd/metasystem/intent_unit_questions.go) is your map. This unit is its proposed part 1 (execution state 80 + holds 50, about 130 lines). Spec: plans/designs/review-chain-stops-and-records.md Decision 3, paragraph 2.

Build: a failed step converts to the shared cause vocabulary (internal/landing/plain/cause.go); physical launch ids retained per step; `environment` (a command that cannot run, a lost process, a moved tree) gets one retry of that step with identical retained inputs and a fresh output path, never another build; a second environment failure stops with its cause; a deadline stops with no automatic retry; a test exit alone is `unclassified`, which holds and asks with a retained person remedy (never spends a correction, never `own`). countedRounds counts only demonstrated `own`. Baseline comparison, flake repeats and reservation accounting are part b2, not this unit: leave a clear seam.
Size: at most 250 production lines.
Public-verb test: a denied command retries exactly once with the same inputs and a fresh output path, a third execution starts nothing, no rebuild happens; an unattributed red holds and asks, and an agent cannot bypass the hold. Mutations: rebuild on an environment retry; count an unclassified round.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
