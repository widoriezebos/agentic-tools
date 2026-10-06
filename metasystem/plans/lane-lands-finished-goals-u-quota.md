# Brief: lane-lands-finished-goals, unit quota

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 5 — read the whole decision; it is the spec, and this unit builds only: Decision 5. Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch. Earlier units of this goal are committed on the branch (admission, next-step, boundary, no-rebase, cause, and those before this one in the design's Units order); use their records and functions, do not duplicate them.

# What this unit builds (80 lines)

Decision 5, exactly as the design's decision states it.

# A test through the public verb (from the design's list)

`goal claim` of a second goal succeeds while the first waits in the lane, in one commit that also enters the first in the land-ready slot; after `landing return` of the first, a third claim is refused and names it. Each test fails under the mutation that removes this unit's rule; name the mutation in the return.

# Readers

Grep every reader of each type, field, file, setting and verb this unit changes and list them in the return with file:line; every reader is updated or named as unchanged with the reason. A unit that edits the skill or a message runs the instruction and message audits.

# Not in this unit

Anything the design assigns to another unit; plan, docs, receipts, memory or records files (except the committed declarations the unit itself is about, for repo-proof).

# Check

go build ./... && go vet ./cmd/metasystem/ ./internal/goal/... && go test -count=1 -timeout 30m ./internal/goal/... && go test -count=1 -timeout 30m -run 'TestGoalClaim|TestClaim|TestWorkLand|TestLanding|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Known load flake, not this unit's: TestWorkRebaseGitAdapterHoldsAfterHistory (4 s ledger fetch). Never open any metasystem.conf.local; tests use synthetic configuration files.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches, and the readers list.
