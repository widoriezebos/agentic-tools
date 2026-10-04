# Brief: codex-jobs-run-unsandboxed-on-a-trusted-host

Goal state: approved, tier 2, box 1d/8/720m/1/20; this unit has at most six review rounds (fleet rule of 2026-10-04); at the sixth it lands with its non-breaking findings as follow-ups.

# Goal

What: on a host the person declares trusted, every Codex job the machinery launches (build, revise, read) runs with `--sandbox danger-full-access`, so builders and readers can run the repository's tests; the default keeps today's read-only/workspace-write mapping. Wido, 2026-10-04 09:05: "Fix the Codex issue in the metasystem: run with danger-full-access; we will switch to running in a VM soon. I accept the risk until then."

# Workspace

Branch goal/codex-jobs-run-unsandboxed-on-a-trusted-host. Its workspace does not exist yet; `metasystem work build` prepares it. Leave the change there, uncommitted.

# Inputs

- Design: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/codex-jobs-run-unsandboxed-on-a-trusted-host.md (accepted 09:58 CEST after Astra rounds 1/0; the specification this brief builds; its Decisions 1 to 4 and Tests section are binding).
- Code sites: `internal/adapter/codex.go` (`BuildCodexCommand` lines 51-110, `CodexPermissionSettings` lines 135-166), `internal/launch/codex.go` (line 89, the literal `-s workspace-write`), `internal/config` (the launch settings' declarations; follow the pattern of the existing `launch.*` keys so `settings keys`, `settings set` validation and `settings show` pick the key up).
- Measured sandbox table (2026-10-04, codex exec 0.160): workspace-write denies process listing, Go cache outside the workspace and network; `network_access=true` lifts only the network; `danger-full-access` lifts all three.

# Units

| Unit | Lines |
| --- | ---: |
| sandbox-mode-setting | 420 |

## What this unit builds

1. **The setting** `launch.codex.sandbox`, values `workspace-write` (default) and `danger-full-access`; any other value refused by `settings set` with the two values named. Declared with the other launch keys; read from the host's local settings the same way the launch runtime and model are.
2. **The adapter mapping** (`CodexPermissionSettings`): when the setting says `danger-full-access`, the sandbox is `danger-full-access` whatever the envelope says, read-only envelopes included; the network value is irrelevant then. Give the function the setting's value as a parameter (or a small options struct) rather than reading global state inside it, so its tests stay pure.
3. **The command** (`BuildCodexCommand`): under full access a fresh thread gets `--sandbox danger-full-access` and no `sandbox_workspace_write.network_access` or `writable_roots` flags; a resume gets `-c sandbox_mode="danger-full-access"` and no network flag. Under the default every argv is byte-for-byte today's.
4. **The unit launcher** (`internal/launch/codex.go`): reads the same setting and replaces the literal `-s workspace-write` with the mode; default unchanged.
5. **The truthful envelope** (design Decision 5, R-134-m1e): where the dispatcher writes a Codex job's `permissions.requested`, under full access it widens the request to what the host enforces: `network` `allow`, `writeRoots` and `readRoots` `["/"]`, `approvals` and `tools` unchanged, plus a field `widenedBy` with the value `launch.codex.sandbox=danger-full-access`. The effective file is materialized from that request as today (`internal/adapter/permissions.go` `MaterializeEffective`); `RewriteWriteScope` leaves the roots alone when the request carries `widenedBy` (the workspace is no longer the boundary) and pins the workspace otherwise; the handshake comparison (`internal/dispatch/handshake.go`, `ComparePermissions`) is not changed: requested and effective agree, so it passes, and every other widening is still refused. Under the default nothing here runs.
6. **The settings reference** (the docs page that lists the settings keys): the key, its two values, that a job admitted under full access records `widenedBy`, and Wido's sentence and date for the risk.

## Not in this unit

The VM; per-job envelopes finer than one host-wide mode; the Claude launcher; GOCACHE placement; applying the setting on the seats (an operator act after landing).

# Constraints

The accepted design is the specification. Build in Go; no new dependencies. Tests use synthetic settings files in isolated fixtures and never read the host's `metasystem.conf.local` (it holds secrets). No test sets process environment or uses real Git. Keep the change to the files named plus their tests and the settings reference page; the gap rule covers anything else.

Maximum reader tool calls: 60

# Expected Return

The change, uncommitted in the goal worktree, on top of the branch's current tip, with a report: what moved (file by file), the argv for both values on fresh thread and resume, the test list with each test's mutation check, and the line count.

# Acceptance Criteria

- `go test -count=1 -timeout 30m ./internal/adapter/ ./internal/launch/ ./internal/config/` passes, and `go run ./cmd/devgate static` passes.
- `go test -count=1 -timeout 40m ./cmd/metasystem/ -run 'TestAudit'` passes (the static audit tests the Codex sandbox cannot run).
- Tests exist and pass for: the mapping for both values and both envelopes (four cases); the fresh-thread and resume argv for both values, with golden argv for the default; the launcher's argv for both values; the setting read from a synthetic local settings file; `settings set launch.codex.sandbox other` refused naming the two values; the request widened with `widenedBy` under full access and untouched under the default; `RewriteWriteScope` leaving widened roots alone and pinning the workspace otherwise; `ComparePermissions` passing the widened pair and still refusing an effective `network` `allow` against a requested `deny` under the default. Each test fails under its mutation (flip the mapping, drop the flag, drop `widenedBy`): check, restore, report.
- `go test -count=1 -timeout 30m ./internal/dispatch/ ./internal/adapter/supervisor/` passes as well (the handshake and the supervisor read the envelope).
- Reverse dependents of the changed packages compile: `go build ./...` and `go vet ./internal/adapter/... ./internal/launch/...`.

# Gap Rule

If the design or this brief does not say what to do, choose the smallest thing that keeps the default path byte-for-byte unchanged, and say so in the report; do not widen the change to make the sandbox mode per job or per role.
