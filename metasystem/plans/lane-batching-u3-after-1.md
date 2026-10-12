# lane-batching U3 correction 1 (Astra LB-01, LB-03): reuse through the existing policy; --fresh bypasses every reuse

Working Mode: Implement. Correction from the design critique (read the amended D5 in plans/designs/lane-batching.md). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4.

## LB-01 (breaking): Go's cache does not see a child process's inputs
internal/proofrun/reuse_policy.go (~64) already forces execution for changed external inputs and for tests that spawn child processes whose file reads and environment Go's cache cannot record (test_go.go ~417 says why). Blanket-removing -count=1 would let a cached green hide a failure after a child's input changed. Fix: keep the reuse policy as THE decision; feed it the input facts the native proof path knows (the group's declared inputs from testing.json, the paths changed since the last green result of this group on this host, whether the group's tests spawn processes); drop -count=1 only for a group the policy clears, keep it everywhere else; the reporter records "cached (reuse: <policy reason>)" or "ran (reuse refused: <reason>)". Test: a changed-child-input fixture — a test that reads a file through a child process; the file changes; the group must re-run (mutation: ignore the policy → cached green → test fails); an unchanged pure package is cached.

## LB-03 (material): --fresh must bypass the lane's saved-proof reuse
prove.go ~472 returns the saved green for an already-green HEAD before launching anything, ~761 has another shortcut, and inherited proofs are reused. `landing prove --fresh` must carry freshness through admission and execution: a fresh proof on an already-green tree creates a new execution with -count=1 everywhere and records "fresh". Test: green tree → --fresh → a new attempt runs (mutation: drop the admission bypass → the saved result returns → test fails).

## Checks
`go vet ./internal/proofrun ./internal/repoproof ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/proofrun ./internal/repoproof ./proof ./internal/landing/plain ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingProve|TestARedProof|TestAudit'`; `go run ./cmd/devgate static`.
