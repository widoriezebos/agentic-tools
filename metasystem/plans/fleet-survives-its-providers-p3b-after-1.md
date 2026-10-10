# Brief: fleet-survives-its-providers P3b, correction 1

Working Mode: Implement
P3b is uncommitted in this worktree. Its read found two material defects; fix exactly these. The critic's probe: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/f77e7ebb-e1ab-4137-9935-206f37035290/scratchpad/p3b/metasystem/internal/outage/zz_critic_test.go.

1. internal/outage/expiry.go:139-157 Clear never runs s.expire(at) first: with no tick in between (steward stopped, or under the helm) a person's clear closes the interval at the clear time instead of the bound and does not mark it stale (probe: a 429 mark at 10:00, bound 10:30, cleared at 15:00 stored Until 15:00, Stale false). Run s.expire(at) under the lock first; if the mark is then gone, the clear is a no-op success. Test: the probe's case stores Until at the bound, Stale true, with one alert. Mutation: no expire before clear -> red.
2. internal/steward/tick.go:294-299 makes RunTick fail whenever the expiry or alert step fails; it runs before budget healing, health and decisions, so on a host without a notify command (notify.go:103-104 notifyUnavailable) every tick fails once any mark goes stale, and a corrupt provider file now kills the tick (tick.go:556 used to treat it as unknown). Record the expiry/alert error and keep the tick going, leaving AlertDelivered false so the next tick retries (as alert_episode.go:560-575 does); a corrupt provider file stays unknown. Change the "delivery failure" subtest to expect the tick to succeed and the alert to retry. Mutation: return the error -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
