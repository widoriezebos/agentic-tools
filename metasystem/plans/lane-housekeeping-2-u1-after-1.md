# lane-housekeeping-2 unit 1 correction 1: --wait waits for the whole proof; the keeper's static step keeps its old bound

Working Mode: Implement. Correction after the Opus read (RETURN, 1 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json.

## F-1 (high): --wait gives up after 15 minutes
`cmd/metasystem/intent_landing_prove.go` ~344-349 sets `seams.WaitTimeout` from `proof.deadline` (15 min: "positive minutes allowed for each declared unit check command", a per-unit-check limit), and `internal/landing/plain/wait.go` ~24-49 turns the timeout into an error. Real proofs take 33-67 minutes (results.jsonl/gates.jsonl of the landing install), so a person's `landing prove --wait` and the landing agent's own `landing prove --gate --wait` would report a failure mid-proof, and a repeat --wait fails at once (remaining time 0). Fix: remove the proof.deadline-derived WaitTimeout; the wait is bounded by the lost-process recovery (a dead or killed detached child is detected and recorded red, wait.go ~54-61) and by the proof's own deadline inside the child. If a backstop is kept, it is a whole-proof bound (a new compiled setting, default well above the longest real proof, e.g. landing.wait-minutes 180) and reaching it reports "still running in the background" as a non-failure result, never exit 1. Keep the injected seam for tests. Test: a wait longer than proof.deadline on a running child returns the recorded result when the child ends (injected clock), and a repeat --wait attaches and returns the same (mutation: restore the 15-minute timeout → the test fails).

## Folds (non-material)
- N-2: `gate_impact.go` ~29,38 (the keeper's impact static step) is now killed after proof.deadline where it had no limit before: restore its previous behaviour (no new cap).
- N-4: LaunchDetached receives os.Environ() twice; pass the additions only.

## Checks
`go vet ./cmd/metasystem ./internal/landing/plain ./internal/gaterun`; `go test -count=1 -timeout 30m ./internal/landing/plain ./internal/gaterun ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestProve|TestClassify|TestAudit'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250), every exit, the mutation.
