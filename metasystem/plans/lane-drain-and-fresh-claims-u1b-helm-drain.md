# Brief: lane-drain-and-fresh-claims, unit U1b helm drain

Working Mode: Implement
Goal lane-drain-and-fresh-claims. U1a (the drain) is committed on this branch. The spec is the accepted design plans/designs/lane-drain-and-fresh-claims.md, Decision 1's "Helm integration (U1b)" paragraph and its U1b test; mechanism 1's sentence "a lane helm take also drains" (plans/designs/machinery-mechanisms.md). 1b's helm (cmd/metasystem/intent_helm.go, internal/helm/helm.go, the policy override) is on main. Build only U1b.

What U1b builds, in short: 1b's lane helm take calls SetDrain after its policy signature write, recording that signature's identity as the drain's source; a failed drain write reports a partial take, never a completed one; helm return clears only a drain whose source is that signature (a person's own drain stays); the take is still person-only; status shows both.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
