# Brief: machinery-measures-its-own-process, unit U5b (report own process cost on every face)

Working Mode: Implement
U1a, U1b, U2, U3, U3b and U5a are committed on this branch. Build the rest of Decision 5 of plans/designs/machinery-measures-its-own-process.md and ONLY that (U5a built the goal-level status route; handoffs in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-machinery-measures-its-own-process-*.md): `processmeasure.Report` renders U1 Measures plus applied acts and process stops, consumed by all three faces: goal/unit status (cmd/metasystem/intent_selection.go:172, intent_process.go:1210), the fleet payload and visible fleet text (internal/ui/httpd/board.go:38, :65; internal/ui/web/_app/src/fleet/FleetPane.tsx:80), and the seat status message (internal/channel/report.go:21, :70, :358; cmd/metasystem/channel_verbs.go:129); order: a current drift stop first, then changes and their measured cost, then external delays and unknown coverage; "own process cost" = measured work under a matching applied act with act id and actor (a helm/attorney-backed act is the agent's own cost with its grant provenance; direct-person acts labelled separately); never a claim of avoidable minutes; a widened check shows observed cost and an unknown counterfactual. The report watermark (included operation ids and observation revisions) is stored with the existing successful channel report state and advances only after successful delivery; status and fleet reads do not advance it; no readable watermark includes all retained acts and says the boundary is unknown; cost accrued since the last report for an older active act is included. The board stays data-only.
Size: at most 250 production lines. If it will not fit, build status and the channel report first and report the rest.
Public-verb test: the design's U5 test through goal status, the fleet payload and the channel report: a drift stop first; an agent's helm-backed act shown as own cost with provenance; a failed delivery retries with the same watermark; a status read does not advance it. Mutations: advance the watermark on read; label a helm-backed act a person's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
