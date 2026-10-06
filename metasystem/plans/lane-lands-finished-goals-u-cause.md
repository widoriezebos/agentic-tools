# Brief: lane-lands-finished-goals, unit cause

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 3 ("every red and every return has a cause") — read the whole section; it is the spec, and this unit builds only the part named below. Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch. Earlier units of this goal are committed on the branch.

# What this unit builds (110 lines)

The record and the return: the `Cause` type in internal/landing/plain (kind own|main|other|flake|environment|unclassified, goal, sha, name, tests[], evidence); `Result` gains `cause` on every red and `goals` (goal and sha of each merge in the checked tree, as `proving` derives them, internal/landing/plain/status.go); `Line` and `Entry` gain `cause` on a return. `landing return GOAL --cause CAUSE [--reason TEXT]`: --cause required; without --reason the reason is the failing tests and evidence from the result; for the landing agent the verb refuses anything but a demonstrated own (the newest result line naming this goal carries cause own for this goal at its waiting line's sha; the gate's gates.jsonl is read too once it exists, so read it when present and ignore it when absent); a person's return proven by --by takes any cause and always takes effect; the refusal text as the design gives it. The card: a return writes board.StageReturned on the goal's live card; writeJoinedCard (cmd/metasystem/landing_plain.go) also takes a card whose stage is returned. The skill skills/landing-agent/SKILL.md: case 3 reads last_proof.cause (own: landing return GOAL --cause own, merge the rest, prove; repeat allowed: prove once more; else end the turn); case 7 holds instead of returning after a check that stopped twice. The classifier itself is unit replay: in this unit the cause of a red is written by whoever already decides it today, and is `unclassified` where nothing decides it yet.

# A test through the public verb

At least one test runs the public verb (`landing return`, `landing prove`, `landing resolve`, `landing status` or `work land`, whichever this unit changes) end to end through the command layer's bed with Git stubbed through the existing seams (ProveSeams, ResolveSeams), and fails under the mutation that removes this unit's rule; name the mutation in the return.

# Readers

Grep every reader of each type, field and verb this unit changes (Result, Line, Entry and their JSON readers, including cmd/metasystem landing status and the UI board, results.jsonl and queue.jsonl readers) and list them in the return with file:line; every reader is updated or named as unchanged with the reason. Units that edit the skill or a message run the instruction and message audits.

# Not in this unit

Anything the design assigns to another unit of Decision 3 or to Decisions 4 to 7; plan, docs, receipts, memory or records files.

# Check

go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... ./internal/board/ && go test -count=1 -timeout 30m -run 'TestLanding|TestWorkLand|TestHandIn|TestLanded|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Known red inherited from unit admission, not this unit's: TestIntentManualWorkLandsOnEndpoint. Do not fix it here.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches, and the readers list.
