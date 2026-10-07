# Brief: lane-policies-and-helm, unit U4 helm

Working Mode: Implement
Goal lane-policies-and-helm. The spec is the accepted design plans/designs/lane-policies-and-helm.md, Decision 4 (read it whole; binding) and its U4 test. U1 is committed on this branch (79e0352bd): config.ResolvePolicy, the policy keys and their provenance, and the person-proof roots in cmd/metasystem/intent_policy.go; build on them, do not duplicate them. Build only U4.

Sites, read on this branch: helm.Record internal/helm/helm.go:22, Write :143, Remove :168; the verbs runIntentHelmStatus cmd/metasystem/intent_helm.go:144, runIntentHelmTake :172, runIntentHelmReturn :388; host discovery of machine stop --all cmd/metasystem/intent_machine.go:870; coordinator declaration runIntentSettingsCoordinator cmd/metasystem/intent_operations.go:457; the permissive fallback helmAdmitter.admit cmd/metasystem/helm_admits.go:97 (NOT the person boundary); the hook message internal/hooks/runtime_hook_stop.go:145 (changes with person-only return).

What U4 builds, in short: helm take at a seat sets seat.driver, review.stop, goal.raise to person, at the lane also the four landing.* policies, at the coordinator also question.route, in one atomic signature write carrying each policy's underlying value and provenance (configuration never overwritten); return removes the override, exposing configured values; settings set while held changes the underlying value and says it applies on return; repeated take keeps the original identity, repeated return is a no-op; take and return are person-only (terminal proof, including removing a corrupt signature); --all enumerates this computer's primary checkouts, the lane and the coordinator, proves the person once and carries the proof, runs sequentially, reports each target, and on an unreadable registry gives a partial result plus the direct `--repo` command per missing target; helm status (and --all) shows who holds what and the overridden policies; help text and the hook message direct return to the person's enrolled terminal. No drain, no keeper or start-guard change (deferred).

The five questions are answered in Decision 4; implement those answers and say in the return where each lives in code.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
