# Brief: fleet-survives-its-providers, unit P3b (stale-mark expiry, its alert, the person's clear)

Working Mode: Implement
P1-P5b are committed and merged on this branch (8d645c3e9). P3's first part (the pause) is in; its read named the remainder, which is this unit and ONLY this (plans/designs/fleet-survives-its-providers.md "P3", and P2's person clear): expiry in the outage owner at reset plus one probe interval (two minutes, internal/steward/runner.go:454) or outage.Horizon (30 minutes) from the last genuine observation when no reset exists; expiry closes the interval at that bound and clears the mark; a once-per-mark stale alert retained for the registered steward to deliver through its existing notification owner (delivered once, also after a late tick or a steward restart; a replay does not duplicate it); stale expiry is not provider success (fleet-provider-and-session-recovery R3 will read firstSuccessAt: record the first genuine success after the mark's close once, never overwritten, and leave it unset on stale expiry); a person's clear (`machine provider clear PROVIDER` or the verb the design names; read the design) closes only that provider's interval at the clear's time; an agent's clear is refused naming the person's command.
Size: at most 250 production lines.
Public-verb test: extend TestFleetProviderPausePublicStatus: a reset-plus-grace stale mark clears with exactly one alert, also after a late tick and after a steward restart; a person's clear ends only its provider's interval; firstSuccessAt is set by the first success after close and not by stale expiry. Mutations: keep a stale mark silently; alert twice; set firstSuccessAt on stale expiry.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
