# Brief: lane-lands-finished-goals, unit ladder

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 7 — read the whole decision; it is the spec, and this unit builds only: Decision 7: the two keys, committed-only, `settings set/show/check`, `system adopt`, `landing prove` reads `proof.full`. Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch. Earlier units of this goal are committed on the branch (admission, next-step, boundary, no-rebase, cause, and those before this one in the design's Units order); use their records and functions, do not duplicate them.

# What this unit builds (95 lines)

Decision 7: the two keys, committed-only, `settings set/show/check`, `system adopt`, `landing prove` reads `proof.full`, exactly as the design's decision states it.

# A test through the public verb (from the design's list)

`landing prove` runs the committed `proof.full` and ignores a local value, which `settings show` says; `settings check` reports a missing key; an adopted file has no `proof.*` line. Each test fails under the mutation that removes this unit's rule; name the mutation in the return.

# Readers

Grep every reader of each type, field, file, setting and verb this unit changes and list them in the return with file:line; every reader is updated or named as unchanged with the reason. A unit that edits the skill or a message runs the instruction and message audits.

# Not in this unit

Anything the design assigns to another unit; plan, docs, receipts, memory or records files (except the committed declarations the unit itself is about, for repo-proof).

# Check

go build ./... && go vet ./cmd/metasystem/ ./internal/config/ ./internal/landing/... ./internal/adopt/ && go test -count=1 -timeout 30m ./internal/config/ ./internal/landing/... ./internal/adopt/ && go test -count=1 -timeout 30m -run 'TestSettings|TestLanding|TestAdopt|TestSystemAdopt|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Known load flake, not this unit's: TestWorkRebaseGitAdapterHoldsAfterHistory (4 s ledger fetch). Never open any metasystem.conf.local; tests use synthetic configuration files.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches, and the readers list.
