# Design for codex-jobs-run-unsandboxed-on-a-trusted-host

- Kind: design
- Id: 01M42XK7CDXSANDB0XTRSTD001
- Status: accepted
- Goals: codex-jobs-run-unsandboxed-on-a-trusted-host

Revision 2, 2026-10-04 11:20 CEST by m1e (Claude Fable 5.1) for Wido under his power of attorney (grant EJSNF4E0Q4GRGW0R3YHWM7RX5X-m1e-718ba0eb): folds the one material finding of Codex on Astra round 1 (RULING-R-134-m1e: the record must tell the truth about the envelope) as Decision 5 and its tests; Decision 4 is folded into it. Revision 1 was written at 10:00. Paths relative to `metasystem/`.

Accepted 2026-10-04 09:58 CEST by m1e for Wido under his power of attorney (grant EJSNF4E0Q4GRGW0R3YHWM7RX5X-m1e-718ba0eb), after Codex on Astra rounds 1/0 (design-critic-8087247692c1d204dc0ace7b: one material finding, folded as Decision 5; round 2 agreed with no findings). His order and risk acceptance of 09:05 are quoted above.

## Wido's words (2026-10-04 09:05, binding)

> "Fix the Codex issue in the metasystem: run with danger-full-access; we will switch to running in a VM soon. I accept the risk until then."

## The defect

Every Codex job the machinery launches runs with `--sandbox workspace-write` and no network: the adapter maps a job's permission envelope to `read-only` or `workspace-write` only (`internal/adapter/codex.go:142-166`, `CodexPermissionSettings`), and the unit launcher hardcodes `-s workspace-write` (`internal/launch/codex.go:89`). Measured on 2026-10-04 with codex exec 0.160: that sandbox denies the process listing the test fixtures need (`sysctl kern.proc.all`), the Go build cache and temp dir outside the workspace, and the network the declared static tools fetch from; `workspace-write` with `network_access=true` lifts only the network; `danger-full-access` lifts all three. So a Codex builder or reader cannot run this repository's tests in `internal/launch`, `internal/delegation` or `cmd/metasystem`; it hands its change over untested and the seat's proof is the first test run. The round analysis of 2026-10-03/04 attributes 9 of 27 review rounds to this.

## Scope

One unit. Not in scope: the VM the fleet will move to, per-job envelopes finer than one host-wide mode, any change to the Claude launcher, GOCACHE placement (moot once the sandbox allows the default cache).

## Decision 1: one per-host setting

A new settings key `launch.codex.sandbox`, declared where the launch settings are declared (`internal/config`, the same registry `settings keys` lists), values `workspace-write` (default) and `danger-full-access`, any other value refused by `settings set` with the two values named. It lives in the host's local settings (`metasystem.conf.local`, set with `settings set`, never hand-edited) because it is a property of the host, not of the repository.

## Decision 2: the mode replaces the mapping, for every Codex launch

When the setting says `danger-full-access`: `CodexPermissionSettings` returns sandbox `danger-full-access` whatever the envelope says (read-only envelopes too, because a reader that verifies by running tests needs the same access), `BuildCodexCommand` passes `--sandbox danger-full-access` on a fresh thread and `-c sandbox_mode="danger-full-access"` on a resume, and does not pass `sandbox_workspace_write.network_access` or `writable_roots` (both are meaningless outside workspace-write and Codex warns on them). The unit launcher (`internal/launch/codex.go`) reads the same setting and replaces its literal `-s workspace-write`. When the setting is absent or `workspace-write`, every argv is byte-for-byte what it is today.

## Decision 3: what still protects the repository

The sandbox was never the commit guard. What stands with full access: the pre-commit guard that refuses an agent commit outside its declared outputs, the worktree per unit, the lane's proof on the merged tree, the independent read, and the host registry's process census. The accepted risk is that a Codex job can write outside its worktree on this host until the fleet runs in a VM; recorded in the settings reference with Wido's sentence and date.

## Decision 4: the operator sees the mode

`settings show` lists the key with its value like any other; a Codex job's record carries the envelope it actually ran under (Decision 5), so a later read of a job knows whether its builder could run the tests.

## Decision 5: the record tells the truth about the envelope (R-134-m1e)

R-134-m1e: an unenforced runtime is declared uncontained in its envelope, never assumed contained. Today a job's `permissions.requested` is materialized verbatim as the effective envelope (`internal/adapter/permissions.go:14`), the workspace is pinned as the write boundary (`RewriteWriteScope`, `permissions.go:28`), and the handshake refuses a launch whose effective envelope is wider than the request (`internal/dispatch/handshake.go:45-110`, `internal/adapter/supervisor/lifecycle.go:341`). Under full access the OS enforces none of that, so leaving the envelope as it is would record a read-only, no-network job as contained: false evidence.

The widening happens at admission, in the request, and is named. When the setting says `danger-full-access`, the dispatcher that writes `permissions.requested` for a Codex job widens it to what the host enforces: `network` `allow`, `writeRoots` and `readRoots` the host's root `/`, `approvals` and `tools` unchanged, and one more field `widenedBy` with the value `launch.codex.sandbox=danger-full-access`. The effective file is materialized from that request as today, `RewriteWriteScope` leaves it alone under full access (the workspace is no longer the boundary), and the handshake's comparison passes because requested and effective agree; no exception path is added to the comparison, so every other widening is still refused. The job record therefore reads "uncontained by launch.codex.sandbox" instead of "read-only", and `settings show` names the setting on the host. When the setting is absent or `workspace-write`, nothing in this decision runs and the records are byte-for-byte today's.

What a reader of such a job knows: the builder or critic could write anywhere on this host and reach the network; the containment is the host, which Wido supplies (a VM soon), per R-134-m1e.

## Tests (the unit's proof)

- `CodexPermissionSettings` for both values and both envelopes (read-only, workspace-write): four cases.
- `BuildCodexCommand` argv for both values on a fresh thread and on a resume: no network or writable-roots flags under full access; today's argv under the default (golden).
- The unit launcher's argv for both values.
- The setting read from a synthetic local settings file in an isolated fixture (never the host's).
- `settings set launch.codex.sandbox other` refused with the two values named.
- Decision 5: a Codex job admitted under full access has `permissions.requested` widened with `widenedBy` set; its effective file equals the request; the handshake passes it; the same handshake still refuses an effective envelope wider than its request when the setting is `workspace-write` (network allow against a deny request); `RewriteWriteScope` leaves the roots alone under full access and pins the workspace under the default.
- Static gate (`go run ./cmd/devgate static`) and the `cmd/metasystem` audit tests (`TestAudit*`).

## Rollout

Land through the lane; then on each seat `settings set launch.codex.sandbox danger-full-access` from its own checkout (nine seats and the landing checkout), at a unit boundary (a settings change strands a running unit, item O). Verify with `settings show` and the next Codex job's record.

## Estimate

About 180 lines of Go and 220 of tests, one build round, one read; two and a half hours from acceptance to hand-in.

## Dispositions (critique design-critic-8087247692c1d204dc0ace7b)

Written by metasystem design review when critique design-critic-8087247692c1d204dc0ace7b closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | RULING-R-134-m1e | The full-access override leaves permission reporting and admission undefined. Ruling R-134-m1e, which requires honest containment claims, says an unenforced runtime is "declared uncontained in its envelope rather than assumed contained". Decisions 2 and 4 change launch arguments and add a sandbox label without addressing the effective envelope. Following that specification leaves the first unsandboxed read-only job recorded as unable to write or use the network. Reporting its actual access instead makes the existing widening checks reject it. The design must specify truthful permission reporting and how the accepted override passes both checks. Materiality test 1 fails because this changes the record and admission contract. Test 2 fails because first use produces false permission evidence, or fails to launch if the envelope is corrected alone. | accepted | Correct: under full access the materialized envelope (permissions.go:14, RewriteWriteScope at :28) would record a read-only, no-network job as contained, and reporting the truth at launch alone would fail the handshake (handshake.go:45-110, lifecycle.go:341). Folded as Decision 5 of revision 2: the widening happens at admission in the request, named by a widenedBy field (launch.codex.sandbox=danger-full-access), with network allow and roots the host root; the effective file equals the request, RewriteWriteScope leaves it alone under full access, the comparison passes with no exception path and still refuses every other widening; tests added for both the pass and the continued refusal. Decision 4 folded into it. | Revision 2 of the page, Decision 5 and its tests |
