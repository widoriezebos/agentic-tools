# Brief: lane-lands-finished-goals, unit replay

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 3 ("every red and every return has a cause") — read the whole section; it is the spec, and this unit builds only the part named below. Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch. Earlier units of this goal are committed on the branch.

# What this unit builds (125 lines)

The classifier and its table, in the check's own process after a red and before the result line (proveInWorktree, internal/landing/plain/prove.go): the six rows of the design's first-match table, the trees per caller (batch proof: origin/main then each batch prefix in merge order; merge gate and trunk check use it in later units), each run alone through the existing runCheck with LANDING_ONLY in a worktree at that tree's commit with its own log under the lane's proofs/ folder; one repeat per tree through the existing repeat field (allowed, started; checkBound refuses after) read before anything is granted; the two counted full proofs budget per batch, refused under the lane lock beside checkBound, the open-loop rule as the design states it; until the stop record exists the refusal names `metasystem landing run`. An own is only ever 'passes without this goal's merge, fails with it'.

# A test through the public verb

At least one test runs the public verb (`landing return`, `landing prove`, `landing resolve`, `landing status` or `work land`, whichever this unit changes) end to end through the command layer's bed with Git stubbed through the existing seams (ProveSeams, ResolveSeams), and fails under the mutation that removes this unit's rule; name the mutation in the return.

# Readers

Grep every reader of each type, field and verb this unit changes (Result, Line, Entry and their JSON readers, including cmd/metasystem landing status and the UI board, results.jsonl and queue.jsonl readers) and list them in the return with file:line; every reader is updated or named as unchanged with the reason. Units that edit the skill or a message run the instruction and message audits.

# Not in this unit

Anything the design assigns to another unit of Decision 3 or to Decisions 4 to 7; plan, docs, receipts, memory or records files.

# Check

go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 30m -run 'TestLanding|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/ && go run ./cmd/devgate static

Known red inherited from unit admission, not this unit's: TestIntentManualWorkLandsOnEndpoint. Do not fix it here.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches, and the readers list.
