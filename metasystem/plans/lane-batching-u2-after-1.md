# lane-batching U2 correction 1 (Astra LB-02, LB-04): the cancel stops the whole proof tree; only an incident's fix preempts

Working Mode: Implement. Correction from the design critique (read the amended D3 and D4 in plans/designs/lane-batching.md). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4.

## LB-02 (breaking): killing the recorded wrapper leaves the proof running
internal/landing/plain/prove.go ~1129 launches the proof command through gaterun.LaunchDetached (launchdetached.go ~62 gives it its own session) and discards its pid; a cancel that ends only the recorded `landing prove --wait --attempt` wrapper leaves `sh proof/full.sh` and the reporter running against a worktree about to be removed. Fix: record the proof command's process identity (pid + start time, the identity package) on the running record at launch; the cancel ends the WHOLE tree (the wrapper, the proof command and its process group) and waits until every recorded process is gone (bounded, injected clock) before the batch is reopened or the worktree touched; a cancel that cannot confirm termination says so and leaves the batch sealed (never a silent partial cancel). Test: a fixture proof with a long child → cancel → both the wrapper and the child are gone before the reopen (mutation: end only the wrapper → the child survives → test fails).

## LB-04 (material): an unrelated newcomer must not interrupt the proof of main's fix
Only a newcomer that IS an open incident's fix (the goal the incident names; reuse hold.go ~161's identification) overrides the elapsed-time threshold and the one-restart limit; any other newcomer follows D2 — and while the running batch contains an incident's fix, no newcomer restarts it at all. Test: main red, batch proving the fix, an unrelated hand-in at 10 min → waits ("the fix of main is proving"); the fix itself arriving at 40 min → restart.

## Checks
`go vet ./internal/landing/plain ./internal/gaterun ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/gaterun ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestKeeper|TestIncident|TestAudit'`; `go run ./cmd/devgate static`.
