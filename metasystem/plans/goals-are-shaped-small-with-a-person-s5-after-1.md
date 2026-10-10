# Brief: goals-are-shaped-small S5, correction 1

Working Mode: Implement
S5 (the approval message) is uncommitted in this worktree. Its read found two material defects; fix exactly these.

1. internal/channel/split.go:88-89 with internal/channel/landed.go:101-105: if a person runs `goal unblock child-two --on source` before approving it, every retry fails "approval request cannot confirm the child and source hold"; a failed refresh puts the notice back in Pending instead of Failed, so it retries forever and phase.Run (phase.go:206) errors on every tick, while child-one never gets its request. A child no longer held by the source is left out of the text (like an approved child), never a failure; any other refresh error counts against the bounded retry and ends in Failed. Test the critic's "unblocked" scenario: child-one's request is delivered once, the tick succeeds. Mutation: fail on an unheld child -> red.
2. internal/ui/act/recover.go:52 (UI endpoint from internal/ui/act/act.go:95) recovers without SplitConfirmed set (only the CLI endpoints are wired: goalsync_verbs.go:212, goalsync_mutations.go:638), so splitAfterConfirmed (split.go:430-434) returns nil and the entry is confirmed with no notice sent or queued. Set the hook where the endpoint is resolved so every recovery caller gets it (or wire phase.NotifySplitApproval into the UI endpoint). Test: recovery through the UI act queues and sends the owed notice once. Mutation: no hook on the UI endpoint -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
