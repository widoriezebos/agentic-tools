# Brief: lane-reads-its-policies, unit U2b-fixes (split from U2b under the stop rule)

Working Mode: Implement
U2b's work is uncommitted in this worktree (its two corrections done). Its final read found one material defect; fix exactly this by subtraction.

internal/landing/plain/prove.go:219 OwnProcess requires a non-empty Process; Start stores processRef(pid), which is "" when the identity probe fails right after launch, so the real child rejects itself: outside a pause it writes a false lost-process red for its own attempt and spends the tree's one repeat allowance; under a pause the person's admitted proof never runs. Fix: in Start (prove.go ~402), a just-launched child whose process identity cannot be read (or a launch that returns pid 0) is a launch failure: stop it, record nothing in running.json, and refuse with an environment cause naming the same command as the retry. No running record with an empty Process can then exist, and OwnProcess stays strict. Tests: keep TestRepeatDetachedOwnAttemptKeepsAllowance for the readable case; add a test where the launch returns an unreadable identity: start is refused with the environment cause, running.json is absent, the repeat allowance is untouched (mutation: store the record with an empty Process, red).

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 60m -run 'TestLanding|TestWork|TestIntent|TestKeeper|TestHelm|TestPolicy|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the change, the tests with their mutations.
