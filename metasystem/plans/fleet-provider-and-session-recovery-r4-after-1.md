# Brief: fleet-provider-and-session-recovery, unit R4 (with R3b), correction 1 — fix by subtraction

Working Mode: Implement
The uncommitted R4 + R3b build is in this worktree; keep R3b and the views. The read found:
F-1 BREAKING: internal/launch/hostcapacity.go:75 createAdmitted calls hostcapacity.Read under build-admission.lock; Read now type-asserts CapacityUsage (hostcapacity.go:82-91), and *Manager has it, so every agent build admission computes usage: CapacityUsage (host_usage.go) lists every launch record and walks all of ~/.claude/projects or ~/.codex/sessions per record. Measured on this host: machine list --json 6.4s -> 25.8s; admission holds the lock ~20s longer, so concurrent builds get LAUNCH_BUILD_CAPACITY. Admission never uses usage. Remove usage from the admission read entirely (admission reads only what it decides on); compute usage only in the two display callers (machine list, /api/board), and only for active sessions or sessions with calls in the design's window; read each transcript once per call, not once per record. Test: admission does not invoke the usage reader (counting fake); mutation -> red.
F-2: default `machine list` text grew from 15 to 16,543 lines (intent_machine.go:1279-1297, no --verbose gate; 898 plain-exec runs are not provider sessions). Summary by default (Wido's rule): one usage summary line per seat/provider by default; the per-session detail only under --verbose; plain-exec runs are not listed as sessions at all. Update the goldens; add a test with many sessions that the default output stays short.
F-3: production lines are 282 against the 250 cap: the subtractions above should bring it under; report the final count.
Run the changed tests by name plus TestFleetUsagePublicViews and TestFleetRecoveryDuePublicMachineList; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
