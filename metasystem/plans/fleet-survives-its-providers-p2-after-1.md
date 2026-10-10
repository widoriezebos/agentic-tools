# Brief: fleet-survives-its-providers P2 (first part), correction 1

Working Mode: Implement
P2's first part (the shared provider mark) is uncommitted in this worktree. One Opus read found five material defects; fix exactly these, keeping the part within 250 production lines (deleting the old API frees room).

1. internal/outage/providers.go:27-32: Provider maps only claude/codex and strips only -headless, but seat and landing writers pass the launch adapter (landing_agent.go:189, steward_seat.go:165) so a Codex mark is stored as codex-exec while every reader uses the settings runtime (openai: seat_start.go:671, landing_agent.go:304); devin-print likewise. Map the adapter names to the same provider key; add a Codex landing case to TestFleetProviderMarkPublicLifecycle (mutation: the old map, red).
2. landing_agent.go:182-189 sends every ended launch to outage.Observe; any provider-state error then holds at internal/landing/lane/agent.go:405-406 (AgentHeld, ReapedAt unset) before held() applies the PersonAct exemption (agent.go:483), so a malformed providers-N.json holds a person's explicit `landing run` every time. A failed provider write during a reap is a logged hint, never a hold; a person's act is never held by it. Test: malformed provider state, a person's landing run proceeds.
3. seat_start.go:676-680 reports unreadable registration or state as {LastClass:"unknown"} and the person notices then say "the model provider is overloaded" (seat_start.go:457, revive.go:358, handoff.go:116). Give it a distinct reason naming the real remedy (`metasystem landing set PATH` when registration is unknown; the file when the state is unreadable).
4. A registration change (lane.go:300-304, epoch+1) reads a new providers-<epoch>.json that does not exist, so standing marks vanish and seats start into a known limit. When the new epoch's file is absent and the install is unchanged, carry the previous epoch's conditions forward under the lock. Test: a mark standing, landing set again, the mark still stands.
5. The old per-checkout API (outage.Record, Clear, StandingAt) has no production caller; delete it and migrate TestAdjudicateOverloadedCLIMarksTheOutage, TestAdjudicateCompletedClearsTheMark, TestTickHoldsRevivalDuringOutage and TestOutageHoldsRevival (which passes only because registration is unknown) to the shared mark.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
