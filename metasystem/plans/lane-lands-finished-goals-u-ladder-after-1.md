# Brief: lane-lands-finished-goals, unit ladder, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. One Opus read found one material defect; fix exactly this.

M-1: `cmd/metasystem/intent_landing_prove.go:123` reads `proof.full` with config.CommittedLookup on the lane checkout's working metasystem.conf, not from the commit being proven; the missing-key refusal (`:128-130`) names `settings set proof.full COMMAND`, which (`intent_work.go:2307-2310`) writes an uncommitted line into the lane checkout's conf, and the next prove runs a command the proven tree never declared. Fix:
1. Read `proof.full` (and `proof.cheap` wherever this unit reads it) from the proven commit's tree: `git show <commit>:metasystem/metasystem.conf` (resolve the conf path the way the repository layout already does), parsed with the existing committed-only parser; refuse naming the key when that tree has no value. An uncommitted line in the lane checkout never runs.
2. The refusal's next step tells the person to declare the key in metasystem.conf through a goal and land it on main; it never names `settings set` for a `proof.*` key. `settings set proof.full|proof.cheap` refuses with the same words (the keys are committed-only), so no remedy writes an uncommitted value.
3. Tests: restore `TestPlainLaneProveIgnoresTheLaneCheckoutsChanges` to its intent (an uncommitted edit of the lane checkout's conf, including a proof.full line, is ignored; the proof runs the committed command), and make the beds (`landing_plain_verbs_test.go` setCommand and the beds that use it) commit the declaration instead of writing it uncommitted. Add a public-verb test: a lane whose HEAD declares no proof.full and whose working conf has one is refused naming the key (mutation: read the working file again, red).

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/config/ ./internal/landing/... ./internal/adopt/ && go test -count=1 -timeout 30m ./internal/config/ ./internal/landing/... ./internal/adopt/ && go test -count=1 -timeout 30m -run 'TestSettings|TestLanding|TestPlainLane|TestAdopt|TestSystemAdopt|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout|TestInstruction|TestNewHomes' ./cmd/metasystem/ && go run ./cmd/devgate static

Never open any metasystem.conf.local; tests use synthetic files. Every new test calls t.Parallel(); -timeout 30m. Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, the tests with their mutations.
