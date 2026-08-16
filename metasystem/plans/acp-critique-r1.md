Round 1 does not converge. P1 can settle Devin’s wire spellings and observed message shapes, but it cannot repair the permission, custody, delivery, ownership, and fallback contracts currently asserted by the draft.

## Findings

1. **CRITICAL — STRUCTURAL — Permission admission is mistaken for containment.**

   Evidence: the draft treats answering ACP permission requests as enforcement sufficient to retire D61 ([acp-transport-design.md:19](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:19)), while P1 only observes traffic ([acp-transport-design.md:75](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:75)). Existing behavioral evidence shows Devin’s controls gate tool availability but do not constrain an allowed command to read/write roots ([devin-support.md:159](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/devin-support.md:159)); Devin therefore remains `notEnforced` for every boundary ([runtimes.go:157](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:157)).

   The design should say instead: ACP permission answering is admission control, not a filesystem or network sandbox. A field becomes `mapped` only after the real ACP launch path behaviorally proves both allowed and forbidden effects, including attempts by an allowed shell to escape roots. Until then, residuals and D61 remain.

2. **CRITICAL — STRUCTURAL — ACP and legacy transports cannot safely share today’s enforcement snapshot.**

   Evidence: ACP is feature-flagged and reversible ([acp-transport-design.md:65](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:65), [acp-transport-design.md:82](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:82)), but a snapshot has one transport list and one undifferentiated `envelopeEnforcement` map ([snapshot.go:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/snapshot.go:67)). Selection keys only on runtime, CLI version, and configuration hash before trusting that singleton map ([select.go:55](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:55), [select.go:97](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:97)).

   The design should say instead: transport and negotiated ACP version are part of capability/enforcement identity. Each job pins one selected transport and its snapshot before launch; changing transport requires fresh selection and handshake. Legacy `dangerous` evidence must never inherit ACP enforcement claims.

3. **CRITICAL — STRUCTURAL — Negotiated client capabilities can create an ungoverned second side-effect path.**

   Evidence: initialization leaves client capabilities unspecified ([acp-transport-design.md:44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:44)), while permission requests are treated as the sole policy boundary ([acp-transport-design.md:51](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:51)). ACP clients may advertise filesystem and terminal methods that let the server ask the client to perform effects directly ([official ACP v1 schema, lines 143–212](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L143-L212)). The current probe deliberately advertises minimal capabilities ([acp-wire-probe.md:4](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-wire-probe.md:4)), but the production design does not preserve that restriction.

   The design should say instead: Devin v1 advertises no client filesystem or terminal capabilities; unsolicited calls fail closed and are recorded. Enabling either later requires a separate containment and custody design.

4. **HIGH — STRUCTURAL — `allow|deny|escalate` does not preserve today’s independent effect and interaction semantics.**

   Evidence: the proposed function supplies no truth table or durable escalation lifecycle ([acp-transport-design.md:51](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:51)). Today `tools` and `approvals` are independently ordered ([permissions.go:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/permissions.go:40), [permissions.go:98](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/permissions.go:98)); both standard presets deny approvals while still permitting different tool grades ([none.json:2](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/none.json:2), [workspace.json:2](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/workspace.json:2)). There is no current same-job approval surface matching “exactly like today.”

   The design should say instead: define a total decision table separating effect admissibility from permission to interrupt a human. State whether approval may ever widen the immutable envelope, and define request ownership, timeout, cancellation, restart, and authorization. If no safe approval lifecycle exists in v1, allow only classified in-envelope effects, deny everything else, and provide no escalation.

5. **MEDIUM — MECHANICAL-GRAIN — The decision function omits `tools`.**

   Evidence: the draft names only roots, network, and approvals ([acp-transport-design.md:51](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:51)); the exact envelope contract contains five fields, including `tools` ([envelope.go:19](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/envelope.go:19)).

   The design should say instead: `Decide` consumes all five fields. Fixtures must prove `read-only` denies state-changing tool categories even within an allowed root, and `runtime-default` never overrides roots or network.

6. **CRITICAL — STRUCTURAL — ACP provides a typed completion signal, not the typed return object the draft claims.**

   Evidence: the draft makes the final response the return candidate ([acp-transport-design.md:26](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:26), [acp-transport-design.md:60](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:60)). ACP v1’s `PromptResponse` contains a stop reason and metadata; response text arrives through message-chunk updates ([official schema, lines 3362–3383](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L3362-L3383), [lines 3591–3624](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L3591-L3624)). The shipped validator requires actual candidate bytes ([return.go:37](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/return.go:37)), while D62 additionally normalizes, validates, snapshots, and rejects torn candidates ([delegate-delivery-design.md:69](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/delegate-delivery-design.md:69), [delegate-delivery-design.md:112](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/delegate-delivery-design.md:112)).

   The design should say instead: notifications are durably journaled and deterministically assembled; a matched successful prompt response is completion evidence. Only the assembled bytes enter D62’s existing qualification, provenance, immutable snapshot, and adjudication owners. Partial streams remain evidence, never delivery.

7. **HIGH — STRUCTURAL — Protocol death, drift, authentication, and partial streams have fixture names but no required outcomes.**

   Evidence: P2 merely lists mid-turn death ([acp-transport-design.md:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:79)), and the draft admits the failover contract is open ([acp-transport-design.md:100](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:100)). The checked-in probe currently covers only `initialize` ([acp-wire-probe.md:3](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-wire-probe.md:3)) and already exposes nonempty authentication methods and unversioned extensions ([acp-wire-probe.md:44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-wire-probe.md:44), [acp-wire-probe.md:63](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-wire-probe.md:63)).

   The design should say instead: provide an outcome table for version mismatch, authentication required, unknown required variants, malformed/oversized frames, blocked reads/writes, JSON-RPC errors, every stop reason, EOF before completion, and teardown timeout. Unknown permission dialects fail closed. Once a prompt may have executed, server death never causes automatic restart or replay because side effects and spend are uncertain.

8. **HIGH — STRUCTURAL — D61 cannot be retired while the legacy `dangerous` path remains callable.**

   Evidence: the design claims D61 retirement while retaining the old reversible path ([acp-transport-design.md:19](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:19), [acp-transport-design.md:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:67)). D61 authorizes `dangerous` for every readable legacy dispatch ([2026-08-13-delegated-decisions.md:1743](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:1743)), and that remains shipped behavior ([devin.go:195](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devin.go:195)).

   The design should say instead: ACP-selected jobs do not invoke D61 after acceptance; the separately selected legacy transport retains its waiver. D61 retires only when the legacy flag and path are removed. An ACP failure must never switch the same job to `dangerous`.

9. **HIGH — STRUCTURAL — D62 retirement is incorrectly coupled to permission acceptance, and rollback assumes an unprobed session bridge.**

   Evidence: P3 retains D62 only “until” graded permission proof ([acp-transport-design.md:82](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:82)), although delivery and permission enforcement are independent: three dangerous-mode rounds still produced no result and D62 correctly refused fabrication ([2026-08-13-delegated-decisions.md:2067](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:2067)). P1 omits `session/load`, while existing follow-up and repair resume through `devin -p -r` ([devin.sh:199](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:199), [devin.sh:296](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:296)).

   The design should say instead: D61 and D62 have independent gates. Rollback is selected before launch, never after partial execution. P1 must separately prove ACP→ACP load, legacy→ACP load, and ACP→legacy resume; a failed direction closes that fallback. D62 qualification and one-repair behavior remain until ACP delivery earns retirement independently.

10. **HIGH — STRUCTURAL — Host recollection is incorrectly disabled by runtime identity rather than integration surface.**

    Evidence: the draft keeps B1 only for non-ACP runtimes ([acp-transport-design.md:31](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:31)), but ACP is introduced only through the delegate adapter. Devin mission hosts still use `devin -p` and `host devin-collect` ([hosts/devin.sh:59](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/hosts/devin.sh:59), [hosts/devin.sh:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/hosts/devin.sh:79)); the recollector remains registered ([internal/host/devin.go:93](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/host/devin.go:93)).

    The design should say instead: “Devin delegate transport = ACP” does not change “Devin host delivery = legacy collector.” Host recollection survives until a separately scoped host-ACP design proves its turn-contract and retry behavior.

11. **CRITICAL — STRUCTURAL — Usage has no single authoritative source and conflates live reporting with dead-round recovery.**

    Evidence: the draft assumes ACP usage events and a combined recoverer/reporter ([acp-transport-design.md:57](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:57)), despite the mandate recording that ACP has no standard usage reporting ([backlog-notes.md:68](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/backlog-notes.md:68)). Devin totals are cumulative and require predecessor differencing ([devin.go:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/devin.go:24), [devin.go:98](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/devin.go:98)); dead-round recovery is a distinct operation and Devin currently declares it unsupported ([recover.go:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/recover.go:40), [devin.go:10](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/devin.go:10)).

    The design should say instead: define one live source per transport, its counter semantics, predecessor identity, resumed-turn delta, and repair replacement rule. Transcript and ACP metrics are alternatives, never summed. Dead-round recovery remains separate and unavailable unless complete wire evidence proves an honest delta can be reconstructed.

12. **CRITICAL — STRUCTURAL — “Long-lived” is not bounded to the human-mandated per-turn lifetime.**

    Evidence: the draft describes a long-lived client with plural prompt turns ([acp-transport-design.md:41](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:41)). The ratified mandate explicitly preserves one process per turn and runs `devin acp` per turn, not as a persistent server ([backlog-notes.md:59](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/backlog-notes.md:59), [backlog-notes.md:63](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/backlog-notes.md:63)).

    The design should say instead: one ACP client/server belongs to exactly one job/turn and exits before that record becomes terminal. Follow-ups launch fresh custody and use proven `session/load`; no process crosses job records. Chain persistence would require a separate custody-transfer design.

13. **CRITICAL — STRUCTURAL — “Same custody machinery” does not cover the proposed nested server or preserve the kill path.**

    Evidence: current custody registration records one exact direct child ([runtime-common.sh:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:81)); a shell→Go client→ACP server topology makes the server a grandchild. The normal supervisor waits only on that direct child ([runtime-common.sh:277](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:277)), while the reaper proves only the recorded custodian dead before writing `groupDeathProvenAt` ([reaper.go:116](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper.go:116), [reaper.go:133](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper.go:133)). Today cancellation instead TERM/KILLs and proves the whole group dead before terminal status ([dispatch.sh:285](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:285), [dispatch.sh:1346](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1346)).

    The design should say instead: specify the exact process tree, PGID/tag inheritance, and registration order before any protocol I/O. All client/server identities and descendants must be proven gone before terminal CAS. `session/cancel` is only a bounded courtesy followed by the existing group sweep. The lifecycle loop must continue heartbeats and enforce handshake/cap deadlines during blocked ACP I/O. Add survivor, daemonization, blocked-frame, TERM/KILL, and client-death probes; the current P1 wire probe cannot establish custody.

14. **HIGH — STRUCTURAL — Behavioral implementation is assigned to the pure-data runtime registry, and the single-runtime probe cannot establish generic interoperability.**

    Evidence: runtime declarations would contain server argv, protocol pin, dialect quirks, and usage shape ([acp-transport-design.md:65](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:65)). Doctrine permits only expected capabilities in `internal/runtimes`; behavior registers in adapter, host, or usage owners ([architecture.md:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:40)). The draft simultaneously claims cross-runtime neutrality ([acp-transport-design.md:12](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:12)) while P1 probes only Devin.

    The design should say instead: the registry declares only expected ACP capability. Adapter-owned tables own launching and protocol translation; `internal/usage` owns decoding; conformance joins declarations both ways. Scope the first supported increment to Devin. Every additional runtime requires its own wire, lifecycle, and conformance evidence.

15. **HIGH — STRUCTURAL — ACP events are described as replacement state, violating the accelerator ruling.**

    Evidence: the draft says runs and jobs obtain state from protocol events “instead of log polling” ([acp-transport-design.md:33](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:33)) and puts every raw exchange into round events ([acp-transport-design.md:60](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:60)). The architecture requires correctness to remain record-based when an accelerator is absent ([architecture.md:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:67)).

    The design should say instead: ACP notifications are advisory progress and wakeup signals. Existing owners commit authoritative job, run, usage, delivery, and turn outcomes to records; recovery remains correct with missing, duplicated, reordered, or truncated notifications. Keep the bounded raw wire journal separate from the typed flight-recorder catalog.

Evidence level: repository and official protocol schema read; no live ACP session or tests run. No files were modified.

Proposed receipt: `type=review | outcome=reworked | skills=design-critique | verify=caught | corrections=0 | stop_loss=no | note=ACP transport DRAFT r1 has structural permission, custody, delivery, usage, registry, and fallback defects.`

REVISE — structural findings remain
