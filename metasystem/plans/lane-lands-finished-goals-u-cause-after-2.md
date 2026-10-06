# Brief: lane-lands-finished-goals, unit cause, correction 2 (the last)

Working Mode: Implement
The unit's change is uncommitted in this worktree. The second Opus read found one material defect; fix exactly this, keep everything else.

F-1: after a design refusal the returned goal's commit stays in HEAD with its green proof; `checkLaneDesigns` (`cmd/metasystem/intent_landing_push.go:66`) only looks at waiting entries and the refusal names `metasystem landing push` as the next step (`:89-102`), so the next push lands the refused goal (reproduced: a second push with HEAD unchanged confirms and pushes). Fix: `landing push` refuses a HEAD that contains the commit of a `returned` hand-in which `old` (origin/main) does not contain, naming a rebuild of the batch as its next step (check out origin/main, merge the waiting shas, prove: name the commands the skill uses); the design refusal names the same rebuild, not `landing push`; `skills/landing-agent/SKILL.md` gains the design-refusal case in those words. Test through `landing push`: refuse for a design, then push again with HEAD unchanged is refused (mutation: drop the returned-in-HEAD check, red). The instruction audit must stay green.

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... ./internal/board/ && go test -count=1 -timeout 30m -run 'TestLanding|TestWorkLand|TestHandIn|TestLanded|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Known load flake, not this unit's: TestWorkRebaseGitAdapterHoldsAfterHistory. Every new test calls t.Parallel(); -timeout 30m. Leave uncommitted. Return the exits, git diff --stat of the correction, and the tests with their mutations.
