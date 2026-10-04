# Correction after round 1: codex-jobs-run-unsandboxed-on-a-trusted-host, unit sandbox-mode-setting

Round 1 built the setting, the mapping, the command, the launcher, the request widening (`admittedRequest` in `internal/dispatch/build.go`) and `RewriteWriteScope`'s widened-envelope exemption. The proof went red on three tests, all one cause.

## The cause

`internal/adapter/permissions.go` removed `{"network", map[string]int{"deny": 0, "ask": 1, "allow": 2}}` from `ordinalFields`. That weakens `ComparePermissions` for every launch on every host: an effective `network: allow` against a requested `deny` is no longer a widening. The design (Decision 5) says the opposite: the comparison is not changed; the request is widened at admission so requested and effective agree, and every other widening is still refused.

Failures it explains:

- `TestComparePermissionsRefuse` (adapter_test.go:124): widening set lost `network`.
- `TestComparePermissionsStillRefusesAnUnwidenedNetwork` (codexsandbox_test.go:186), the round's own test: network allow against deny returned no mismatch.
- `TestFakeEffectiveWider` (supervisor/fake_test.go:251): the fake's wider effective envelope launched (exit 0) instead of failing the handshake (exit 1).

## What to do

1. Restore the `network` ordinal in `ordinalFields` exactly as it was. Do not touch `ComparePermissions` otherwise.
2. Keep the full-access path correct by the request alone: under `launch.codex.sandbox=danger-full-access`, `admittedRequest` widens `permissions.requested` (network allow, readRoots and writeRoots `["/"]`, `widenedBy` set); `MaterializeEffective` copies it; `RewriteWriteScope` leaves a `widenedBy` envelope alone; so `ComparePermissions` sees requested == effective and passes with no special case. Confirm this with the test that exercises the admitted full-access pair (it must pass without any change to the comparison).
3. Re-run the full check: `go test -count=1 -timeout 30m ./internal/adapter/... ./internal/launch/ ./internal/config/ ./internal/dispatch/`, then `go run ./cmd/devgate static`, then the mutation checks the brief lists (flip the mapping, drop the flag, drop `widenedBy`): each named test must fail under its mutation, then restore.
4. Report: the three tests green, the mutation table, the final argv for both values on fresh thread and resume, the line count.

Nothing else changes. If a test outside the three fails after step 1, report it with its cause before changing production code.
