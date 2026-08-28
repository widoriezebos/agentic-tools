Verdict: P3 A+B does not converge. I found 10 material findings: five structural and five mechanical-grain. The flag being default-off does not make the lifecycle and evidence defects safe to ship.

## Findings

1. **CRITICAL — STRUCTURAL — ACP cannot satisfy the dispatcher’s handshake deadline.**

   `supervise_acp` runs the complete prompt synchronously at [devin.sh:301](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:301) and only records the session afterward at [devin.sh:319](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:319). Devin advertises a 30-second session-establishment budget at [devin.sh:80](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:80), while normal turns are explicitly measured in minutes at [devin.sh:26](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:26). At expiry, the dispatcher writes `handshake_timeout` and winds down the group through [dispatch.sh:576](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:576) and [dispatch.sh:1458](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1458). A healthy initial ACP turn lasting over 30 seconds is therefore killed before its already-established wire session reaches the record. This blocks the flag-on benchmarks themselves.

2. **HIGH — STRUCTURAL — The FIFO launch bypasses the shared supervision contract and has an unbounded missing-peer path.**

   The branch uses fixed, reused FIFO names, unlinks them, spawns only the server in the background, and registers only that server at [devin.sh:280](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:280)–[devin.sh:287](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:287). The client remains an unregistered foreground child at [devin.sh:301](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:301), bypassing `wait_for_cli`’s heartbeat and deadline enforcement at [runtime-common.sh:277](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:277).

   The ordinary `>out <in` pairing is complementary and works. The failure proof does not: if the server dies after registration but before opening `server_out`, the client blocks opening its read side. Follow-up records already contain a session at [build.go:300](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/build.go:300), so the handshake-timeout writer expressly stands down without killing at [dispatch.sh:1428](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1428). Reusing the round directory can also unlink FIFO names still associated with an earlier blocked generation. This is exactly what the required gates, bootstrap descriptors, per-attempt names, dual registration, and traps at [acp-transport-design.md:137](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:137) were meant to prevent.

3. **HIGH — STRUCTURAL — Transport selection fails open to legacy and is not pinned in the job.**

   Configuration-reader failure becomes `legacy` at [devin.sh:230](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:230), and every value other than exact `acp` also takes the legacy branch at [devin.sh:366](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:366). Missing configuration legitimately defaults to legacy before the flip; unreadable configuration and invalid values must refuse, not silently enter D61’s dangerous path. The selector is reread for each follow-up and no transport identity is stored in the record constructed at [build.go:273](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/build.go:273). That permits a chain to switch transports and is incompatible with D82’s fix-forward posture.

4. **HIGH — STRUCTURAL — The registry expectation and transport-specific admission contract do not reach launch or certification.**

   Devin declares protocol version 1 at [runtimes.go:181](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:181), but the launched arguments at [devin.sh:290](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:290) never query that declaration or pass `--expected-protocol`; the command independently defaults to 1 at [acp_verbs.go:92](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/acp_verbs.go:92). Changing the declaration to 2 leaves launch on 1 and the conformance test green.

   The capability snapshot also remains one plural legacy-shaped record at [devin.sh:75](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:75), with neither protocol version, schema digest, nor the required `(runtime, transport)` admission state. Selection filters only runtime/version/config hash at [select.go:55](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:55). The conformance join at [runtime_conformance_test.go:85](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_conformance_test.go:85) checks names and two copied grade strings, but not `ExpectedACP ⇒ HasAdapter`, the actual launch version, schema identity, or admission evidence. This is an unrecorded omission from the P3 contract.

5. **HIGH — MECHANICAL-GRAIN — Requested model is falsely persisted as observed effective model.**

   The recorded limitation requires effective model to remain `unobserved`, yet [devin.sh:320](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:320) supplies `$requested_model` to `record_handshake`, and [handshake.go:113](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/handshake.go:113) persists it as `effectiveModel`. The ACP path neither sets nor observes that model. Any opt-in job requesting a non-default model can run the wire default while its durable evidence claims otherwise.

6. **HIGH — MECHANICAL-GRAIN — Follow-ups fabricate session establishment on pre-session failures.**

   For every follow-up, [devin.sh:314](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:314) substitutes `requested_session` instead of reading `outcome.sessionId`. The ACP client only assigns that ID after successful `session/load` at [turn.go:157](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/acp/turn.go:157). Version mismatch, authentication refusal, unsupported load, load error, or malformed load response can therefore be recorded as a successful handshake even though no session was loaded.

7. **HIGH — MECHANICAL-GRAIN — Post-session failures use the pending-only terminal writer and can leave jobs running.**

   Once a session-bearing outcome is recorded, the job is running. Nevertheless, setup and protocol rows call `fail_pending` at [devin.sh:331](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:331). A mode-setting failure expressly carries a session at [turn.go:169](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/acp/turn.go:169), as do later protocol failures. `fail_pending` compares only against `pending` and treats lost-compare exit 3 as success at [runtime-common.sh:154](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:154), so the adapter returns while the record remains running. The same case also labels post-prompt protocol failures as phase `setup`.

8. **HIGH — STRUCTURAL — Delivery and native usage bypass the journal-completeness boundary.**

   `acp turn` exposes a thinned journal through `journalError` at [acp_verbs.go:192](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/acp_verbs.go:192). Both downstream projections omit that field: collection at [devincollect.go:164](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devincollect.go:164) and usage at [acp.go:18](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/acp.go:18). Consequently, an otherwise delivered response can be accepted and publish native totals at [acp.go:57](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/acp.go:57) even when the command says its raw evidence is incomplete. This contradicts the specification’s journal-owned completeness boundary.

9. **MEDIUM — MECHANICAL-GRAIN — Usage-owner failure is silently converted to null usage.**

   Every `acp-usage` error is discarded at [devin.sh:312](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:312). If parsing or writing fails, no usage artifact exists, and [patch.go:31](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/patch.go:31) records `usage:null` rather than the promised typed `unavailable` value or a mechanical terminal.

10. **LOW — MECHANICAL-GRAIN — The time-bomb repair does not verify that the clock pin succeeded.**

    Commit `81a8ce1` calls `os.Chtimes` without checking its error at [run_test.go:390](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/run_test.go:390). On a filesystem where that operation fails, the test silently retains the real modification time and the wall-clock time bomb remains. The fix’s intent is sound; the new syscall needs to be asserted.

## Requested attacks that did not produce findings

- Every global consumed from `prepare_supervision` is initialized on this path at [runtime-common.sh:43](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:43). The `fail_pending` and `finish_running` call arities are correct; their state selection is not.
- Passing `transcript=""` is valid. ACP collection has already normalized and fully validated the accepted candidate at [devincollect.go:225](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devincollect.go:225). Invalid candidates exit through collect code 3 before `complete_from_cli`, so the generic legacy repair function is dormant on the current ACP path.
- The exclusive collect channel and its outcome/session equality check are sound. The defect is the shell’s late or fabricated handshake evidence upstream.
- `row != delivered` does not by itself make usage partial. Usage is attached only after a matched, parseable `PromptResponse` at [turn.go:390](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/acp/turn.go:390); refusal, cancellation, or incomplete delivery may therefore have a complete completion-bounded total.
- `Adoptable=false` is not an ACP defect; adoption controls installation, while `HasAdapter` controls dispatchability. Devin mode strings remain adapter-owned at [devinacp.go:18](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devinacp.go:18), and protocol core transports an opaque mode ID.
- `setup`, `handshake`, and `delivery` are existing phases, and the lifecycle permits named adapter errors. The material terminal defect is the wrong pending/running writer and wrong phase selection, not the `acp_*` prefix itself.

Of the four recorded limitations, model handling and today’s FIFO/custody implementation are blockers now because they publish false evidence or admit an unbounded live failure. Repair-disabled-pre-claim is currently tolerable and remains unreachable through the qualified collector. The missing shell fixture may remain a recorded default-off landing exception, but it is a hard prerequisite before any flag-on benchmark is credited; D83 does not relax D82’s Mac-and-VM success gate.

Evidence: read all three commit diffs, the specification, wire probe, prior critiques, and downstream lifecycle owners. Ran `git show --check` for each commit, `git diff --check`, `bash -n` on the affected shell surfaces, and read-only CLI expectation/mode checks; all passed. Focused Go tests could not run because the read-only sandbox denied Go temporary/cache creation. Worktree remained clean; no files were modified. Reviewed tree: `0ec7f3f12ec2c57ee646647c514f7ec8a52fc313`.

REVISE — structural findings remain
ACPP3CC-DONE
