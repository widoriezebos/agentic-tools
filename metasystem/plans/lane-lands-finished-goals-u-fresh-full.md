# Brief: lane-lands-finished-goals, unit fresh-full

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decisions 4 and 7 — read the whole decision; it is the spec, and this unit builds only: Decisions 4 and 7: `landing prove --trunk`, `proof.trunk-every` and the clock, skill case 8, `decideScope` proves whole when an executable input changed. Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch. Earlier units of this goal are committed on the branch (admission, next-step, boundary, no-rebase, cause, and those before this one in the design's Units order); use their records and functions, do not duplicate them.

# What this unit builds (90 lines)

Decisions 4 and 7: `landing prove --trunk`, `proof.trunk-every` and the clock, skill case 8, `decideScope` proves whole when an executable input changed, exactly as the design's decision states it.

# A test through the public verb (from the design's list)

with a held line waiting and `proof.trunk-every` passed, `landing status` reports `full-due`; `landing prove --trunk --wait` runs the whole command on main's unchanged tree when its last result was green, and again when it was red, where today the first is answered from the record and the second refused; the green one clears the incident and the line is held no more. A batch whose tree moved by a script after its full green is proven whole; one that moved by a record runs only the covering groups. Each test fails under the mutation that removes this unit's rule; name the mutation in the return.

# Readers

Grep every reader of each type, field, file, setting and verb this unit changes and list them in the return with file:line; every reader is updated or named as unchanged with the reason. A unit that edits the skill or a message runs the instruction and message audits.

# Not in this unit

Anything the design assigns to another unit; plan, docs, receipts, memory or records files (except the committed declarations the unit itself is about, for repo-proof).

# Check

go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 30m -run 'TestLanding|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Known load flake, not this unit's: TestWorkRebaseGitAdapterHoldsAfterHistory (4 s ledger fetch). Never open any metasystem.conf.local; tests use synthetic configuration files.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches, and the readers list.

# Defect classes the reads of this goal keep finding (avoid each; the read checks them)

1. A refusal's remedy that cannot succeed, or that, followed, undoes the gate (e.g. a refusal naming `settings set` for a committed-only key; a next step of `landing push` while a refused goal is still in HEAD).
2. An agent given a person's power (the landing agent reopening its own budget), or a person treated as an agent (a person refused because a lane or session cannot be read). A person's act is never refused except to prevent damage.
3. An older or records entry hiding a goal's current state (read a goal's latest code hand-in through latestLaneGoalEntry, never plain.Latest).
4. A test seam hiding production behavior: a function only tests call (wire every new seam into its production caller and test through the public verb); a stub returning an error shape production does not (plain.Git wraps exit errors with %w).
