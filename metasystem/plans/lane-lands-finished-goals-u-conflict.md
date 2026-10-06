# Brief: lane-lands-finished-goals, unit conflict

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 3 ("every red and every return has a cause") — read the whole section; it is the spec, and this unit builds only the part named below. Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch. Earlier units of this goal are committed on the branch.

# What this unit builds (120 lines)

A conflict is classified before anything is returned, in landing resolve (internal/landing/plain/resolve.go): with main or with a batch member (git merge-tree --write-tree of the handed-in sha against origin/main alone); with main: abort and return with cause own, the conflict record's main is origin/main's commit, the seat's command is work rebase G; with a batch member: abort, return nothing, the line stays waiting and gains `after` (goals and shas merged since origin/main), held while one of them still waits, merged again in a later batch, and work land G says so with the design's text. Regeneration failures by class (the design's table: no exit status of its own -> environment lost-process, retried once at the agent's next turn, a second holds and asks; exited non-zero and passes on the tree before the merge -> own with the log; fails before the merge too -> unclassified, hold and ask); the run on the tree before the merge happens after the abort in the same lock and removes only its own outputs. The skill case 4 reads the outcome (resolved, returned, or held with its reason).

# A test through the public verb

At least one test runs the public verb (`landing return`, `landing prove`, `landing resolve`, `landing status` or `work land`, whichever this unit changes) end to end through the command layer's bed with Git stubbed through the existing seams (ProveSeams, ResolveSeams), and fails under the mutation that removes this unit's rule; name the mutation in the return.

# Readers

Grep every reader of each type, field and verb this unit changes (Result, Line, Entry and their JSON readers, including cmd/metasystem landing status and the UI board, results.jsonl and queue.jsonl readers) and list them in the return with file:line; every reader is updated or named as unchanged with the reason. Units that edit the skill or a message run the instruction and message audits.

# Not in this unit

Anything the design assigns to another unit of Decision 3 or to Decisions 4 to 7; plan, docs, receipts, memory or records files.

# Check

go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... ./internal/conflict/ && go test -count=1 -timeout 30m -run 'TestLanding|TestWorkLand|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Known red inherited from unit admission, not this unit's: TestIntentManualWorkLandsOnEndpoint. Do not fix it here.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches, and the readers list.
