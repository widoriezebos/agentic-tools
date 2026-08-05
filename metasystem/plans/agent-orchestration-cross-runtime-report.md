# Cross-Runtime Agent Orchestration Research

Date: 2026-08-03

Purpose: implementation input for `agent-orchestration-design.md`. This report does not amend or replace that design.

## Executive conclusion

Copy the operational discipline from `openai/codex-plugin-cc`, but not its Claude-to-Codex/App Server architecture.

The portable foundation should be:

1. A deterministic mission controller.
2. A provider-neutral, durable file/job protocol.
3. Basic CLI adapters that work with Codex, Claude, and Devin.
4. Capability-gated enhanced transports:
   - Codex SDK or App Server.
   - Claude stream-JSON or Agent SDK.
   - Devin ACP.
5. Polling and owned OS-process control as the universal fallback.

Devin is a full peer for headless execution, exact-session resume, model selection, permissions, hooks, and workspace-local operation. It is not yet demonstrably a full peer for native structured output and machine-readable usage through ordinary `devin -p` execution.

## Recommended architecture

```text
                     deterministic mission-runner
                                |
                 +--------------+--------------+
                 |                             |
          host adapter                    durable job store
     resume orchestrator turns        requests, rounds, results
                 |                             |
       Claude / Codex / Devin          delegate runtime adapter
                                               |
                           +-------------------+-------------------+
                           |                   |                   |
                       Codex CLI           Claude CLI          Devin CLI
                       + SDK/server         + Agent SDK          + ACP
```

The important new boundary is between:

- The **host adapter**, which keeps the orchestrator and mission progressing.
- The **delegate adapter**, which executes one assigned job.

Shell scripts make delegate launching portable; they do not ensure that a Claude, Codex, or Devin host will wake up, adjudicate a completed job, and begin the next unsupervised cycle. The design's claim in §3.2 that the mechanism is symmetric by construction should therefore be narrowed.

For mission mode, a deterministic `mission-runner` should own waiting, timeouts, state transitions, and starting the next orchestrator turn. Hooks may notify it, but correctness must not depend on hooks firing.

## Verified cross-runtime capability

| Capability | Codex | Claude | Devin local |
| --- | --- | --- | --- |
| Non-interactive invocation | `codex exec` | `claude -p` | `devin -p` or `--prompt-file` |
| Resume exact session | `codex exec resume <id>` | `claude -r <id>` | `devin -r <id>` |
| Native structured result | `--output-schema` | `--json-schema` | Not documented for ordinary `-p` |
| Native event stream | `--json` JSONL | `stream-json` | ACP path; not ordinary `-p` |
| Machine-readable usage | Token usage in JSONL | Usage and cost in JSON result | Not documented for ordinary `-p`; ACP usage is not yet stable |
| Enhanced protocol | SDK or App Server | Agent SDK | ACP JSON-RPC over stdio |
| Lifecycle hooks | Yes | Yes | Yes; can import Claude hook configuration |
| Native/provider budget | No universal CLI budget | USD and turn caps | Cloud ACU cap; no equivalent local CLI flag documented |

Codex's non-interactive interface supplies JSONL events, token usage, JSON Schema output, and exact-session resume. OpenAI positions App Server for deep interactive integrations and the SDK for automated jobs.

Claude supplies validated JSON Schema output, stream-JSON events, exact-session resume, usage and cost metadata, `--max-budget-usd`, `--max-turns`, and documented SIGTERM process-tree cancellation.

Devin supplies non-interactive execution, prompt files, exact-session resume, model selection, permissions, sandboxing, ATIF export, session listing as JSON, and an ACP server. Its hooks expose stable `session_id` and per-turn `prompt_id`, and it reads compatible Claude hook configuration.

ACP gives Devin a serious enhanced path: sessions, streamed updates, cancellation, resume, list, and close. ACP usage reporting remains a draft facility, so it cannot underpin the universal mission budget.

## Material recommendations from `codex-plugin-cc`

### 1. Keep host integration thin

The plugin's Claude commands and rescue agent mostly forward work to a standalone control program. Registration, hooks, slash commands, and host-specific prompting should likewise remain thin facades around a provider-neutral job engine.

Use one shared implementation with small Codex, Claude, and Devin registration layers.

### 2. Separate terminal status from progress phase

The plugin distinguishes job completion from human-readable phases such as investigating, editing, verifying, and finalizing. Use the same separation:

```json
{
  "status": "running",
  "phase": "verifying"
}
```

Recommended terminal vocabulary:

```text
queued | running | completed | failed | cancelled | timed_out
```

The design currently uses competing vocabularies across §3.3, §3.4, and §3.9: `completed`/`timeout` versus `done`/`capped`/`lost`. Define one state machine before implementation. Process loss and budget cap should be error/reason fields or explicit terminal states, not alternate vocabularies used by different components.

### 3. Add cancellation as a first-class adapter verb

The adapter contract in §3.3 lacks `cancel`.

Use this universal sequence:

1. Send a provider-native interrupt when advertised.
2. Wait a bounded grace period.
3. Signal the exact owned process.
4. Kill the owned process tree if necessary.
5. Atomically record `cancelled`.

For Devin ACP, use `session/cancel`; for Codex App Server, use `turn/interrupt`; for baseline CLI execution, use the owned process. Claude explicitly documents SIGTERM cleanup.

Also enforce one active turn per conversation or session. Concurrently resuming one session can interleave state, and `codex-plugin-cc` serializes active streaming requests for the same reason.

### 4. Make every follow-up an immutable round

The phrase "same outputs" for follow-ups risks overwriting the original return. Store:

```text
job.json
request.json
prompt.md
rounds/0001/raw.out
rounds/0001/events.jsonl
rounds/0001/return.json
rounds/0001/return.md
rounds/0002/...
```

Record `conversationId`, `turnId`, `round`, and `parentRound`. State writes should be atomic and locked.

### 5. Make JSON canonical and Markdown derived

Reverse the current §3.5 ownership:

- `return.json`: canonical and locally validated.
- `return.md`: derived human-readable projection.
- `raw.out`: retained for diagnosis.
- `events.jsonl`: normalized when available.

Codex and Claude should use native schema enforcement. Devin's CLI fallback should request JSON-only output and validate it locally. Invalid output becomes `failed` with `errorCode=protocol_error`, not a successful job. Do not silently accept or regex-scrape malformed output.

### 6. Introduce a machine-readable capability handshake

The prose capability table in §3.8 is insufficient because capabilities vary by CLI version, account, enterprise policy, and host configuration.

`probe` should produce and persist something like:

```json
{
  "runtime": "devin",
  "version": "...",
  "transports": ["cli", "acp"],
  "capabilities": {
    "resume": true,
    "nativeStructuredOutput": false,
    "nativeEvents": true,
    "nativeUsage": false,
    "gracefulCancel": true,
    "hooks": true
  }
}
```

Every role should declare required and optional capabilities. Missing required capabilities fail closed; missing optional capabilities invoke an explicit fallback.

Suggested required portable baseline:

- Non-interactive invocation.
- Exact-session resume, or a declared fresh-context fallback.
- Owned process and exit status.
- Captured raw final output.
- Enforceable workspace and permission mapping.
- Local result-schema validation.

Suggested optional capabilities:

- Native structured output.
- Native event stream.
- Native usage telemetry.
- Provider-native cancellation.
- Lifecycle hooks.
- Protocol server.
- Provider-native spending cap.

### 7. Normalize hooks, but never require them

All three runtimes share a useful lifecycle-event intersection:

- `SessionStart`
- `UserPromptSubmit`
- `PreToolUse`
- `PermissionRequest`
- `PostToolUse`
- `Stop`
- `SessionEnd`

Build one hook receiver and three small payload translators. Use it for notification, telemetry, and waking the controller.

Retain process exit, polling, and durable files as the fallback because hooks can be disabled, skipped as untrusted, or unavailable in older versions.

Do not make mission lifetime equal host-session lifetime. A `SessionEnd` hook may clean up a protocol connection, but must not automatically kill mission jobs that are explicitly allowed to survive a host restart.

### 8. Cap context and make input transport argv-safe

The plugin's history contains several lessons:

- Avoid embedding large diffs.
- Limit inline and untracked content.
- Prefer file references for large inputs.
- Never pass repository-derived strings through shell expansion.
- Test malicious branch names and other provider arguments.

The job record should capture input byte counts, hashes, delivery mode, and truncation. Prompts should travel through stdin or files, never interpolated shell commands.

### 9. Replace `costTokens`

The mission token proxy in §6.3 cannot honestly sum Codex tokens, Claude USD, and Devin ACUs or unavailable local usage.

Use typed telemetry:

```json
{
  "usage": {
    "availability": "native",
    "inputTokens": 1000,
    "cachedInputTokens": 800,
    "outputTokens": 200,
    "reasoningTokens": 50,
    "cost": {"amount": 0.12, "currency": "USD"},
    "providerUnits": {"name": "acu", "value": null}
  },
  "requestedModel": "...",
  "effectiveModel": "..."
}
```

Never sum heterogeneous provider units. Universal hard fences should be:

- Wall clock.
- Cycle count.
- Job count.
- Concurrency.
- Per-job timeout.

Provider-specific budgets are additional fences only when natively enforceable. Estimated or unavailable telemetry must not drive a hard spending claim.

### 10. Model permissions as an envelope, not one write flag

`write-access=workspace|none` is too narrow to express the effective security boundary. Normalize:

```json
{
  "permissions": {
    "readRoots": ["..."],
    "writeRoots": [],
    "network": "deny",
    "approvals": "deny",
    "tools": ["read", "search"]
  }
}
```

Each adapter maps this envelope onto provider controls and records the effective result. A self-test should attempt a permitted read and a forbidden write in a scratch workspace.

This matters especially for Devin: its OS sandbox applies to exec-tool processes, while direct edit/write tools are governed by permission rules. `--sandbox` alone is not evidence of a read-only agent.

### 11. Expand the fake adapter into a protocol simulator

The plugin's test suite covers cancellation, nested subagent events, session scoping, compatibility fallbacks, context limits, and malicious arguments. The metasystem fake should script:

- Malformed structured output.
- Missing session ID.
- Resume collision.
- Two concurrent turns on one session.
- Cancellation/completion races.
- Process loss and timeout.
- Missing native event stream.
- Nested-agent completion events.
- Old CLI capability sets.
- Oversized context.
- Hook unavailable.
- Atomic-write interruption.

The single proposed Claude-host/Codex-delegate end-to-end run does not prove symmetry. Require at least one host-cycle smoke test for Codex, Claude, and Devin, plus delegate self-tests for every configured runtime.

## What not to copy

- Do not make Codex App Server the core protocol. It is powerful but provider-specific.
- Do not put the plugin's shared broker or daemon into the portable layer.
- Do not import whole host transcripts as the normal handoff mechanism.
- Do not tie mission lifetime to host `SessionEnd`.
- Do not implement mission mode as a blocking `Stop`-hook loop. The plugin warns that its review gate can create long-running loops and rapidly consume usage.
- Do not weaken the design's existing worktree isolation, untrusted-return rule, or orchestrator-only certification. Those are stronger than the plugin.

## Recommended design changes by section

| Design section | Recommended change |
| --- | --- |
| §3.2 | Narrow the symmetry claim; introduce host adapters and an explicit mission controller. |
| §3.3 | Add `cancel`; define capability discovery and CLI-versus-enhanced transport selection. |
| §3.4 | Use one terminal state machine, a separate phase, immutable turn IDs, typed usage, requested/effective model, capability snapshot, and atomic state updates. |
| §3.5 | Make `return.json` canonical and schema-validated; derive Markdown; store immutable rounds. |
| §3.6 | Replace the write-only abstraction with a normalized permissions envelope including network and approvals. |
| §3.8 | Make the capability table generated from `probe`; specify required capabilities and fallback behavior. |
| §3.9 | Turn the fake into a scripted protocol simulator and prove all three possible hosts. |
| §6.2 | Specify the executable controller that advances the mission between orchestrator turns. |
| §6.3 | Replace cross-provider `costTokens` summation with universal lifecycle fences and typed provider-specific telemetry. |

## Evidence and sources

### Directly observed

- Inspected `openai/codex-plugin-cc` at commit `db52e28f4d9ded852ab3942cea316258ae4ef346`.
- Ran its current test suite locally: 91 tests passed, zero failed.
- Locally installed and inspected: Codex CLI `0.146.0` and Claude Code `2.1.220`.
- Devin CLI is not installed locally, so its real adapter self-test remains mandatory before support is declared shipped.

### Primary sources

- [`openai/codex-plugin-cc`](https://github.com/openai/codex-plugin-cc)
- [Plugin Codex runtime](https://github.com/openai/codex-plugin-cc/blob/db52e28f4d9ded852ab3942cea316258ae4ef346/plugins/codex/scripts/lib/codex.mjs)
- [Plugin structured review schema](https://github.com/openai/codex-plugin-cc/blob/db52e28f4d9ded852ab3942cea316258ae4ef346/plugins/codex/schemas/review-output.schema.json)
- [Plugin runtime tests](https://github.com/openai/codex-plugin-cc/blob/db52e28f4d9ded852ab3942cea316258ae4ef346/tests/runtime.test.mjs)
- [Plugin shell-expansion security fix](https://github.com/openai/codex-plugin-cc/commit/db52e28f4d9ded852ab3942cea316258ae4ef346)
- [Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode)
- [Codex App Server](https://developers.openai.com/codex/app-server)
- [Codex SDK](https://developers.openai.com/codex/sdk)
- [Codex hooks](https://developers.openai.com/codex/hooks)
- [Claude programmatic usage](https://code.claude.com/docs/en/headless)
- [Claude CLI reference](https://code.claude.com/docs/en/cli-usage)
- [Claude session semantics](https://code.claude.com/docs/en/sessions)
- [Claude hooks](https://code.claude.com/docs/en/hooks)
- [Devin CLI reference](https://docs.devin.ai/cli/reference/commands)
- [Devin hooks](https://docs.devin.ai/cli/extensibility/hooks/overview)
- [Devin lifecycle events](https://docs.devin.ai/cli/extensibility/hooks/lifecycle-hooks)
- [Devin sandbox](https://docs.devin.ai/cli/sandbox)
- [Devin permissions](https://docs.devin.ai/cli/reference/permissions)
- [ACP protocol updates](https://agentclientprotocol.com/updates)
- [ACP session usage draft](https://agentclientprotocol.com/rfds/session-usage)

## Verification status

- Evidence level **ran**: plugin test suite; local Codex and Claude CLI help/version inspection.
- Evidence level **read**: current official Codex, Claude, Devin, and ACP documentation; plugin source and history.
- Evidence level **inferred**: the portable architecture and fallback recommendations derived from the documented capability mismatch.

