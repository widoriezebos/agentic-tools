# lane-batching U2 correction 2: a cancelled attempt may be proven again; a cancel never races a finishing proof; no live goal page in tests; the unit shrinks

Working Mode: Implement. Correction after the Opus read (RETURN, 4 material); the unit's last correction. Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load). Field names: U1 (another branch of this goal) defines the batch's `Open` (json "open") and `SealedAt` (json "sealed_at"); U2 must use exactly those names and tags and not add its own (the critic found `Open json:"open,omitempty"` here and a `sealed-at` elsewhere); a restart clears SealedAt (the batch is open again).

## F-1 (high): a cancelled attempt can never be proven again by the agent
restart.go ~344 writes the result "cancelled" without `Repeat: "allowed"`; prove.go ~394 checkBound returns NoRepeat for any non-green result lacking it — after a restart whose newcomer is then held or returned, the batch rebuilds the pre-restart tree and the agent's `landing prove` is refused ("this code failed its check and gets no other"). Fix: a cancelled result is never a red: exclude "cancelled" in checkBound (or set Repeat allowed on the cancel result). Test: restart → newcomer returned → prove of the old tree accepted (mutation: restore → refused → test fails).

## F-2 (medium): a cancel just after the proof command exits wedges the lane and loses a finished result
restartBatchLocked writes the cancel record (~343-353) BEFORE recordProofDescendants (~358), which refuses when the recorded proof command is already dead (~209); the wrapper then sees the cancel and discards its finished result, running.json stays, every later tick fails "descendants cannot be confirmed; the batch stays sealed"; endProof (~164) goes through SignalExact, which returns ErrGone for a dead group leader without signalling the group. Fix: before writing the cancel, if ProofProcess is recorded and NOT alive, record no decision and wait (the result is imminent); kill the process group directly (kill(-pgid)) rather than through SignalExact on the leader; REMOVE recordProofDescendants and the descendants half of endProof (~80 lines: the wrapper, the proof command and its process group are what the brief asks for). Test: a live wrapper with an exited command → the tick records nothing and the finished result lands; the lane never wedges (mutation: write the cancel first → wedge → test fails).

## F-3: a test pins the live goal page
restart_test.go ~403 TestRestartRealContractAndLedgerPlan reads plans/goals/lane-batching.md (goal done deletes goal pages). Drop the goal-page read and the test-plan half; keep the restartBed assertion.

## F-4: size 450 net against 250
The F-2 subtraction (~80) and the legacy three-state ProofProcess handling (~10) bring it to ~360; cut further where the critic's notes allow (N-3 %g formatting → one helper; N-7 stale Waiting reasons → compute once). Report the final count; if it stays above 250, say what a split would look like (coordinator decides).

## Folds (non-material)
- N-1: de-duplicate results.jsonl lines by attempt before the median.
- N-2: the "command's process identity is missing" message during a red's classification → "a red is being classified; the newcomer waits".
- N-3: minutes printed with one decimal.

## Checks
`go vet ./internal/landing/plain ./internal/gaterun ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/gaterun ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestKeeper|TestIncident|TestAudit'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count, every exit, each mutation.
