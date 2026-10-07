# Brief: lane-drain-and-fresh-claims, unit U3 engine inputs

Working Mode: Implement
Goal lane-drain-and-fresh-claims. The spec is the accepted design plans/designs/lane-drain-and-fresh-claims.md, Decision 3 (read it whole; binding) with its U3 test. Build only U3.

Sites, read on main: the whole-commit deferral internal/steward/runner.go:724; verifyEnrollmentLandedSourceWithDeps internal/steward/rearm_resolver.go:519 (ancestry and the narrow internal,cmd list); compareEngineProjectionWithDeps :632 (already compares ENGINE projections); verifySourceAtDestinationWithDeps :686; dispatch's preflight engineSkewPreflight internal/delegation/infra.go:50 (fail-open for no merge base or a git failure at :57-64 stays); the ENGINE inputs internal/behaviorsurface/policy.v2.json:3, embedded at policy.go:107; the engine build cmd/devgate/build.go:112.

What U3 builds, in short: steward.EngineInputsEqual over the existing projection (equal/different/error plus the first changed path), used by re-arm, enrolled-source verification and dispatch preflight in place of their path-list and whole-commit shortcuts; equal inputs reuse the enrolled bytes and clear the stale deferred note (ledger, records and settings commits never defer session start); a real difference names `go run ./cmd/devgate build` then `metasystem session start`, and the public test follows both; an error is never equality; preflight keeps fail-open where a rebuild cannot help.

The five questions are answered in Decision 3; implement those answers and say in the return where each lives in code.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
