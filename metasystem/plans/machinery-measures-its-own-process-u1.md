# Brief: machinery-measures-its-own-process, unit U1 (cost per step)

Working Mode: Implement
Goal machinery-measures-its-own-process. The spec is the accepted design at /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-measures-its-own-process.md (read it there; it is not yet on this branch): Decision 1 "Extend the recorded steps; one reader owns all arithmetic (U1)", the U1 rows of the Units, Estimates and public-verb test tables, the five-questions rows for U1, and the round-2 acceptance items R2-M1 (record every unit's check argv as an observation; only an argv equal to the committed proof.full, split on whitespace, is a full-suite check), R2-M2 (no estimate correction in this goal: leave it out of U1 and its test) and R2-M3 (freeze the estimate only from the accepted design revision's Estimates row with its digest; never from the brief; a missing row or a changed page at first admission gives "estimate unavailable" and holds an agent's build, never a person's). This worktree is goal/machinery-measures-its-own-process from main 752cfbe9e. Confirm each site the design cites on this tree before changing it.

Build U1 only: the frozen first-admission estimate and step timing; the measures (including the full-suite argv identity and the unloaded comparison); `work status G --work U` (and goal status) showing where the time went. One reader owns all arithmetic; every measure is derived from records, never typed. At most 250 production lines; if it will not fit, stop and report what you would split.
Public-verb test: the design's `TestProcessStepCostPublicStatus`, with its mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
