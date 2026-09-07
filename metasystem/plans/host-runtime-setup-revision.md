# Host setup: review dispositions and concurrent delegates

Goal: `host-runtime-setup`. Amendment to the frozen
`plans/host-runtime-setup-design.md`, 2026-09-07.

The user clarified that Claude, Codex, and Devin delegates run concurrently
under one MetaSystem, generally in their own worktrees. Registration prepares
all hosts; it must never select a single globally active runtime or promote a
delegate to coordinator. Writing delegates get worktrees automatically;
read-only critics may share the coordinator checkout
(`scripts/agents/dispatch.sh:1403-1418,1503-1522`). Job records stay at the
launcher's installation (`scripts/agents/adapters/runtime-common.sh:8-12`).

Round one returned an input-digest gap, not agreement. Round two found five
bounded defects; each is accepted below and becomes a named fixture. The
original failsafe was round two. The user's concurrent-delegate clarification
adds an isolation requirement, so the final permitted design round reviews
this concrete amendment and its new role boundary. There is no fresh chain or
larger review budget. Code critique remains mandatory.

## Accepted changes

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-D1-001 | accepted | `internal/census/ancestor_production.go:105-123` restricts an unrestricted lookup to the execution roster. An installed Devin host can therefore be missed. | Add an explicit all-host lookup using adoptable registry declarations, independent of `metasystem.runtimes`; preserve the old default lookup. Fixture `TestImportedClaudeHookSkipsDevinOutsideExecutionRoster`. |
| HOST-D1-002 | accepted | `supervision-hook.sh:34-64` starts the Stop budget only on entry, so a preceding wrapper adds unbudgeted time. | Keep the runtime and role guard inside the existing supervision script's Stop deadline, before any durable mutation. Fixture `TestRuntimeHookGuardDelayStillEmitsOneStopVerdictWithinTimeout`. |
| HOST-D1-003 | accepted | Codex's old SessionStart matcher lacks compact; a foreign sibling may share its group. | Remove only owned handlers, retain foreign siblings with their unchanged group fields, and insert one desired owned group. Fixture `TestMergeSplitsOwnedHandlerFromForeignSiblingWhenMatcherChanges`. |
| HOST-D1-004 | accepted | `internal/stateroot/stateroot.go:30-63` already scrubs Git steering variables. | Generated commands clear the same steering variables before locating the repository. Fixture `TestGeneratedHookCommandIgnoresGitSteeringEnvironment`. |
| HOST-D1-005 | accepted | Atomic rename does not preserve the original mode by itself. Preserving modes avoids an unnecessary operator-visible change. | Preserve existing instruction/configuration permission bits on content-changing writes; declare modes for new files. Fixture `TestSetupPreservesExistingInstructionAndConfigurationModes`. |

## One lifecycle entry owner

The existing `scripts/agents/supervision-hook.sh` remains the lifecycle entry
owner. Do not add the proposed `runtime-hook.sh` wrapper. Add the guarded
`receipt` event there, preserving the current receipt messages and cadence.
The script validates event/runtime shapes, enters its existing Stop deadline,
and proves context before hook-attempt, announcement, lease, digest,
stop-authorization or retirement writes.

A proven foreign runtime or proven rostered delegate is an intentional silent
skip for start, receipt, stop and end. Use one reserved internal worker result
that the Stop parent accepts as a silent skip; it must not treat an ordinary
empty worker response, query failure or timeout as that result. No durable
supervision log, digest cursor, refusal count or authorization is changed for
a skipped hook. Unknown identity retains existing failure semantics. The
missing-engine Stop launcher remains fail-closed.

The nearest runtime is found over the adoptable host registry, not the
execution roster. Add `proc find-ancestor --all-hosts` as an explicit selector,
mutually exclusive with `--runtime`; preserve existing invocation semantics.
The recorded-main fallback must compare the announcement runtime with the
hook runtime before permitting lifecycle mutation.

## Proving a delegate without breaking a new host

`lease classify` calls any unannounced runtime ancestor DELEGATE
(`internal/lease/classify.go:344-346`). That label alone is NOT evidence that a
fresh interactive or headless coordinator is a rostered delegate.

Reuse the existing job-custody owner in `internal/lease/classify.go:254-283`.
Add a narrow read-only `lease hook-delegate` query that accepts the canonical
state/installation roots and a caller PID, walks that process and its
ancestors, and matches authenticated live identities against job.pid plus
job.custodyProcesses, using platform-exact birth tokens where present and the
existing legacy start-time fallback where absent. It returns a positive delegate result only for an exact
job-owned ancestor. Check the adapter ancestor as well as the provider child:
that closes the fork-to-child-registration interval. Do not use mission host
or generic supervision custody to suppress hooks. Corrupt/unreadable evidence
is an error, never a positive skip. Reuse the existing identity probes and
job-record parsing; no second durable role registry, capability, or scheduler.

The shared adapter launch plumbing supplies the canonical installation root
and job identifier as environment hints to each Claude/Codex/Devin provider
child (dispatch and resume). These are evidence-location hints only: they do
not authorize skipping. The hook queries the launcher's installed engine and
job records to verify the current ancestry; a stale or forged hint without
that ancestry cannot skip. This works even when the delegate's worktree lacks
its own ignored bin/metasystem or artifacts tree. A successful proof exits
before using the worktree as a coordinator state root. If a supplied delegate
hint cannot be verified, report/refuse safely rather than announcing a new
coordinator. With no hint, inspect local job custody and otherwise retain
ordinary cold-start enrollment. Do not propagate the hint to a mission host
launcher, change provider permissions, or disable unrelated provider hooks.

The launcher root may be used only to query context until the proof succeeds;
ordinary lifecycle writes still use the local layout owner. This task does not
change the authority model: one checkout writer, delegated jobs supervised by
their dispatcher. It does not redesign independent coordinator stop tokens.

## Setup details retained from existing adoption

Adoption retains its Claude default, `--runtimes none`, and `--copy-skills`.
The shared setup command supports copy mode explicitly so adoption uses the
same owner for both copies and links. Existing identical copies are accepted;
changed copies conflict rather than being replaced. Setup takes a target
installation path even when its executing engine belongs to the source.
Extend the existing state-root owner with a small explicit layout resolver if
needed; do not infer the target from the current executable.

All original HOST-1 through HOST-5 obligations remain in force. These rows
add precise test obligations; their status records preparation, not completion.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| HOST-D1-001 | HIGH | Review round 2 | Imported hook skips a different host outside the execution roster | census and supervision entry | internal/census/ancestor_production.go and scripts/agents/supervision-hook.sh | scripts/agents/runtime-hook-fixtures.sh: TestImportedClaudeHookSkipsDevinOutsideExecutionRoster | isolated process fixture | MISSING | Implement and run |
| HOST-D1-002 | HIGH | Review round 2 | Role/runtime checks share Stop deadline and emit one safe result | supervision entry | scripts/agents/supervision-hook.sh | scripts/agents/runtime-hook-fixtures.sh: TestRuntimeHookGuardDelayStillEmitsOneStopVerdictWithinTimeout | delayed-lookup fixture | MISSING | Implement and run |
| HOST-D1-003 | HIGH | Review round 2 | Mixed group migration preserves foreign siblings and removes duplicate owned hooks | hooks merge | internal/hooks/setup.go | internal/hooks/setup_test.go: TestMergeSplitsOwnedHandlerFromForeignSiblingWhenMatcherChanges | emitted settings readback | MISSING | Implement and run |
| HOST-D1-004 | MEDIUM | Review round 2 | Hook root ignores inherited Git steering | hooks rendering and state-root | internal/hooks/setup.go | internal/hostsetup/setup_test.go: TestGeneratedHookCommandIgnoresGitSteeringEnvironment | execute generated command in both layouts | MISSING | Implement and run |
| HOST-D1-005 | MEDIUM | Review round 2 | Existing instruction/settings modes survive writes | hostsetup | internal/hostsetup/setup.go | internal/hostsetup/setup_test.go: TestSetupPreservesExistingInstructionAndConfigurationModes | content-changing merge and retry | MISSING | Implement and run |
| HOST-6 | HIGH | User clarification | Concurrent delegates cannot become coordinators or mutate coordinator/session state | lease job custody, adapter launch plumbing, supervision entry | internal/lease/hook_delegate.go and scripts/agents/supervision-hook.sh | internal/lease/hook_delegate_test.go and scripts/agents/runtime-hook-fixtures.sh | three concurrent runtime-shaped job children, distinct linked worktrees and shared checkout, equal raw session IDs, all hook events; compare coordinator state before/after | MISSING | Implement and run |
| HOST-7 | HIGH | Cold-start contract | Fresh interactive and headless mission hosts still enroll; forged context cannot suppress them | lease job custody and supervision entry | internal/lease/hook_delegate.go and scripts/agents/supervision-hook.sh | internal/lease/hook_delegate_test.go and scripts/agents/runtime-hook-fixtures.sh | unannounced standalone host and mission-owned host fixtures; stale PID/hint and unknown lookup fixtures | MISSING | Implement and run |
