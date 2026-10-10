# Brief: fleet-survives-its-providers, unit P2 (one provider mark in the registered owner's state)

Working Mode: Implement
P1 is committed on this branch. Spec, and ONLY this text: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/fleet-survives-its-providers.md, section "P2 — One provider mark in the registered owner's state", the P2 rows of the Units, Estimates and public-verb tables, the five-questions rows for P2, round-2 changes 1-2 (the landing agent's writer and reader cmd/metasystem/landing_agent.go:183, :307; a person's clear; the 2-minute probe internal/steward/runner.go:454, :497 when the reset is unknown or beyond outage.Horizon) and acceptance item R3-1 (the probe takes a provider and uses the runtime and model the mark recorded; the registered steward loops over every standing mark of either class). Move every listed outage writer and reader (internal/steward/seat_start.go:300, internal/missionrunner/loop.go:2104, :2163, internal/steward/runner.go:497, internal/steward/revive.go:169, landing_agent.go:183, :307) to the one shared mark; fill P1's provider section from it. A mark never pauses a person's act.
Size: at most 250 production lines. The design expects P2 to split: build the largest usable first part within 250 (the shared mark with all writers and readers moved first; the multi-provider probe and the person's clear next) and report the rest.
Public-verb test: the design's P2 test (including a landing-agent limit result), with its mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
