# Correction after round 2: codex-jobs-run-unsandboxed-on-a-trusted-host, unit sandbox-mode-setting

Round 2 is green on its proof (five packages) and the read found one material defect; the rest conforms. Fold only this.

## The finding (round 2 read, high, material)

The unit launcher's sandbox does not follow the selected installation in two supported launch contexts. `newLaunchManager` (`cmd/metasystem/launch_verbs.go:33-42`) resolves settings from the executable's installation, and the real supervisor creates its own manager (`launch_verbs.go:97-103`), started with only `launch supervise --id` (`internal/launch/process.go:145-156`); `Manager.Supervise` then obtains the adapter and calls `Command` (`internal/launch/launch.go:299-332`) with `CodexExec.Sandbox` empty, so the argv carries `-s workspace-write` although the selected installation's local settings say `danger-full-access`. The reader reproduced it with an overlay test: bin-control PASS; pinned-engine FAIL (selected setting danger-full-access, adapter setting "", argv `-s workspace-write`); separate-selected-installation FAIL (adapter setting workspace-write). Consequence: the ordinary steward-pin path still starts Codex builders and readers in workspace-write after the operator enables full access, which is the one thing this goal removes.

## What to do

1. Resolve `launch.codex.sandbox` from the unit's SELECTED installation (the one the unit runs for), not from the executable's, and carry it to the adapter on every path that builds a Codex command: the manager the verb builds and the manager the supervisor process builds. The cleanest carrier is the launch record (write the resolved sandbox mode into the record at admission, next to the runtime and model, and have `CodexExec.Command` read it from the record when `adapter.Sandbox` is empty); if the record cannot carry it, resolve it in the supervisor from the record's installation root. Either way the supervisor process must not fall back to the executable's installation.
2. Add the regression test the reader wrote as a real test (its overlay source is at `/private/tmp/metasystem-sandbox-review-e351f30d/selected_sandbox_test.go`; read it, do not copy blindly): three cases, bin-control, pinned-engine, separate-selected-installation, each asserting the argv's `-s` value, with synthetic settings files in isolated fixtures and no real Git.
3. Re-run the proof (`go test -count=1 -timeout 30m ./internal/adapter/... ./internal/launch/ ./internal/config/ ./internal/dispatch/`) plus `go test -count=1 -timeout 30m ./cmd/metasystem/ -run 'TestAudit|Sandbox|Codex'`, and `go run ./cmd/devgate static`. Mutation: drop the record field (or the supervisor's resolution) and show the new test fails, then restore.
4. Report: what moved (file by file), the three cases green, the mutation result, the line count.

Nothing else changes; the round-2 change stands as it is.
