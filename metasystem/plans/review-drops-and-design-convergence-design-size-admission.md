# Brief: review-drops-and-design-convergence, unit design-size-admission

Working Mode: Implement
Branch goal/review-drops-and-design-convergence-dsa from the goal branch at 8cfaa48d2; it merges back into goal/review-drops-and-design-convergence (the drop units build there in parallel; stay out of the drop code). Build unit design-size-admission (Decision 3) of plans/designs/review-drops-and-design-convergence.md and ONLY that: its own declarations and settings validation, the production-line parser of a design's Units table (the reader briefs-carry-their-rules B1 and goals-are-shaped-small S3 depend on), the goal-wide count, the build and land gate, the person route, the present remedy (the person's `goal open NEW-GOAL ...` for the over-five-units case, accepted design lines 104 and 117), and the advisory author/critic result.
Size: at most 250 production lines (the design estimates 245). If it will not fit, build the largest usable first part within 250 (parser and gate first) and report the rest.
Public-verb test: the design's design-size-admission test; mutation: the design's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
