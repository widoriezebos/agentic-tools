# Brief: machinery-measures-its-own-process, unit U1a (estimate retention and step evidence)

Working Mode: Implement
U1 split at the size stop (read plans/handoff-machinery-measures-its-own-process-u1.md in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem: its evidence of the owners on this tree is your map). Spec: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-measures-its-own-process.md, Decision 1 and "Decided by m1e for Wido" 2026-10-07 23:15 (the recut), with R2-M1 (record every selected check argv as an observation) and R2-M3 (freeze the estimate only from the accepted design revision's Estimates row with its digest, reusing designGateFacts/designBodyDigest in cmd/metasystem/intent_design_gate.go; a missing row or changed page at first admission is "estimate unavailable" and holds an agent's build, never a person's). R2-M2: no estimate correction.

U1a builds only the evidence: the frozen estimate and its digest retained on the unit plan (internal/launch/unit_plan.go and its strict decoder) across the unit's runs; each step's start and end (build, attest, read, correction) and each executed command's identity (argv, display name separate) retained without overwriting the collection time (internal/launch/read_sequence.go endStep keeps FinishedAt as collection; add the execution interval). No arithmetic and no status here: U1b builds the one reader over this evidence.
Size: at most 250 production lines; if it will not fit, stop and report.
Public-verb test: `work build` of a unit with an accepted Estimates row retains the estimate and digest; a second run of the same unit keeps the first estimate; an agent's build with no row (or a page changed since acceptance) is held as "estimate unavailable" and a person's is not; each step records its interval and the check's argv. Mutation: freeze from the brief's size instead of the design row.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
