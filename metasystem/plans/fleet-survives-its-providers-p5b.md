# Brief: fleet-survives-its-providers, unit P5b (current-load build admission)

Working Mode: Implement
P5 Part A (the restart bound) is committed on this branch. Build Part B of "P5" in plans/designs/fleet-survives-its-providers.md and ONLY that (handoff: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-fleet-survives-its-providers-p5.md): `host.builds=auto|N|person` and a positive declared `host.load-max` through the existing configuration/policy owners (internal/config/policy.go:59, internal/config/defaults.go:66), checked in Manager.Start (internal/launch/launch.go:105, :238) immediately before creating the Starting record, against current load from P1's hostcapacity reader; build launches only (not reads, not seats); a refusal writes no launch record (internal/launch/hostcapacity.go:78 would count it as Starting; runs-advance D1/D2 depend on this) and names the load, the limit and the setting; `person` asks through a question; a person's build is never refused (directPersonProof / StartSpec.Actor propagation, which Part A left unfinished); suite serialization stays a declaration over the existing admission cap (proof.admission.top-level-max), no new suite ratio.
Size: at most 250 production lines (the design estimates 140).
Public-verb test: through `work build`: over host.load-max an agent's build is refused with no launch record and the remedy names the setting; a person's build starts; host.builds=1 with one active build refuses the second; host.builds=person asks. Mutations: write a Starting record on refusal; refuse a person's build.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
