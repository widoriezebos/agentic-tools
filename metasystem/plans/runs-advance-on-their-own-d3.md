# Brief: runs-advance-on-their-own, unit D3 (bounded automatic step advancement and public waiting)

Working Mode: Implement
Branch goal/runs-advance-on-their-own, stacked on goal 2's branch (ee8ba2f5c); it lands after goal 2. Build unit D3 of plans/designs/runs-advance-on-their-own.md (accepted; read it in full, especially "Decided by m1e for Wido" 10:05 and its D3 acceptance item) and ONLY D3: the steward advances a run's recorded next step on its own (DriveWork, placed before the helm `continue` at internal/steward/runner.go:361-366 so collection continues under the helm) and `work wait` observes it; under seat.driver=person or the helm, an agent's invocation only observes and renders the next act, enforced at the one shared entry that starts a step (an option on runner.Continue or one command-layer guard) covering all three agent routes: work wait (cmd/metasystem/intent_work.go:1730), work build run:RUN (:503-504) and work revise run:RUN (:1413-1414); only an invocation with directPersonProof (:2455) drives; the person probe writes no attorney refusal log (:2497). D1's capacity wait, D2's recovery, D4's worker revision and D5's boundary are other units.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (the shared-entry guard first) and report the rest.
Public-verb test: the design's D3 test, plus: under the helm an agent's `work build run:RUN` starts no step and prints the next act; a person's starts it. Mutations: an agent's wait starts a step under the helm; `work build run:RUN` from an agent under the helm starts a step.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
