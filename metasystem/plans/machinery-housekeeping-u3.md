# Brief: machinery-housekeeping, unit U3 (one repeat and the failing test's output)

Working Mode: Implement
U1 and U2 are committed on this branch. Spec: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-housekeeping.md: Decision 3 "Record one repeat and the failing test's actual output", the U3 rows, round-1 change 4 (extend the existing tree-keyed Result.Repeat record: Repeat="started" under lock by recordProofStop / continueRed, internal/landing/plain/replay.go:76-87; a second proof refused unless Repeat=="allowed", prove.go:320, proof_admission.go:112; no separate repeat-claim key) and R2-M2 (output bytes, digests and their assertions are this unit's: TestEvidence). Goal 2 (not on main yet) routes its unit-check repeats through this unit's TestEvidence/RecordFlake later; keep the API small and callable from launch.

Build: a known flake (FlakeFacts, U2) gets exactly one repeat per tree, recorded on the existing Result.Repeat record; TestEvidence retains the failing test's own output (bytes and digest, both attempts) as the sighting's evidence; RecordFlake records the sighting with that evidence once; a test that fails again on the repeat is not a flake for that tree and is reported red with both outputs. No other retry, no wider timeout.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's U3 test (a known-flake red repeats once and records one sighting with both outputs; a second red stays red; a second repeat on the same tree is refused). Mutations: allow a second repeat; drop the output evidence.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
