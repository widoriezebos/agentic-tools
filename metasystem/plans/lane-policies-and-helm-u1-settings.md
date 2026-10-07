# Brief: lane-policies-and-helm, unit U1 policy settings

Working Mode: Implement
Goal lane-policies-and-helm. The spec is the accepted design plans/designs/lane-policies-and-helm.md, Decision 1 (read it whole; it is binding) and its U1 test and round-4 acceptance item under "Public-verb tests". Build only U1; Decisions 4 and 5 are other units.

Sites, read on this tree (3a0dbd6b7): config.Get internal/config/resolve.go:303 and getLayered :319; Setting internal/config/defaults.go:18; the value validator SettingValueProblem internal/config/codexsandbox.go:46; runIntentSettingsSet cmd/metasystem/intent_work.go:2277, its authority check :2306, authoritySettings :2429, directPersonProof's proof test :2472; the roster caller cmd/metasystem/intent_roster.go:65 (its contract does not change); enrollment per checkout internal/humanauthority/authority.go:659; brain Undeclared vs Corrupt cmd/metasystem/intent_operations.go:494-511.

What U1 builds, in short: the nine keys with their one-token grammars (settings.apply read-only, committed); config.ResolvePolicy as the single reader (helm override, the checkout's value, the coordinator's default, built-in auto; registry reader injected from cmd; Undeclared coordinator skips its layer); per-checkout policy metadata (name, checkout, value, set-by, at, previous) written under a settings lock, set-by from the proof, "author unknown" when unnamed; the eight decision keys in authoritySettings; proof at the calling checkout then the destination (outside every checkout: the destination); an agent's write of a lane key refuses naming `metasystem settings set KEY VALUE --repo <lane>`; question.route with no coordinator names the declare command; settings show KEY shows value, checkout, source, setter, time, and helm holder when overridden; settings check validates grammar and scope without a live lane.

The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
