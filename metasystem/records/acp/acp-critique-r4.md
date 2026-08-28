R4 has 12 structural defects and one mechanical-grain defect. The custody lock can support the proposed atomic protocol, but that protocol and several terminal, admission, repair, and wire contracts remain unspecified or contradictory.

1. **CRITICAL — STRUCTURAL — Sealed custody generations lack an atomic state machine.**

   R4 names generation advancement, opening, sealing, and status-plus-generation comparison but never defines their order or owners ([design:149](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:149)). The implementation can support this: custody registration and record compare-and-swap share the same exclusive record lock ([custody.go:8](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/custody.go:8), [record.go:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:102)). But the shipped compare-and-swap compares only status ([record.go:250](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:250)), and records have neither generation nor seal state ([build.go:185](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/build.go:185)).

   The design should specify: open/reserve a generation before spawning; register identities against that generation while advancing a mutation revision; seal after no more processes can be spawned; refuse proof while open; and atomically compare status, generation, revision, and sealed state at terminalization. It must define abandoned-open recovery, absent-generation semantics for both empty and populated legacy custody lists, and prohibit generic record patches from changing custody-control fields.

2. **CRITICAL — STRUCTURAL — Proof mode is wrongly selected by outcome instead of committer liveness.**

   R4 assigns every failure and deadline to `full-set`, while reserving `except-live-custodian` for normal completion ([design:165](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:165)). Shipped setup, running-failure, and protocol-error writers execute inside the still-live adapter ([runtime-common.sh:154](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:154)). Its deadline handler likewise kills descendants, commits the terminal record, and only then exits ([runtime-common.sh:238](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:238)). Handshake rejection is also committed by the live adapter ([handshake.go:119](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/handshake.go:119), [dispatch.sh:1335](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1335)).

   The design should select mode from who commits: every adapter-self-finalized outcome uses authenticated `except-live-custodian`, or a named external finalizer waits for adapter death and then uses `full-set`. Every failure-matrix row needs an explicit committer and proof mode.

3. **CRITICAL — STRUCTURAL — The claimed universal proof owner still misses shipped terminal writers.**

   Lease takeover accepts stale `pending-setup`, `pending`, and `running` records, sends at most one group `SIGTERM`, then directly rewrites the record to `failed` without custody, generation, or group-death proof ([sweep.go:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/sweep.go:24), [sweep.go:121](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/sweep.go:121), [sweep.go:143](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/sweep.go:143)). `internal/lease` is absent from r4’s blast radius ([design:474](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:474)).

   Dispatch also starts a detached adapter before publishing its exact identity; failures while discovering or recording that identity become `pending → failed` without cleanup or proof ([dispatch.sh:536](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:536), [dispatch.sh:1007](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1007)).

   The design should route lease sweeping and every post-spawn launch failure through the shared owner. Only a positively proven never-launched reservation husk may use a zero-process terminal exception; once launch returns a PID, that identity must be retained and proved dead before terminalization.

4. **CRITICAL — STRUCTURAL — The r3 group-emptiness fold is not real.**

   R4 requires group emptiness only on kill-capable paths, then claims the remaining risk is limited to descendants outside both registration and the process group ([design:165](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:165), [design:187](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:187)). A no-kill reaper or normal completion can therefore terminalize while an unregistered same-group descendant survives. The standing reaper currently proves only the top-level identity ([reaper.go:111](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper.go:111)); terminal records then disappear from live custody census ([run.go:481](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:481)).

   Both modes should always observe the group: `full-set` requires it empty; `except-live-custodian` permits only the exact adapter. Kill authority controls signaling, not the strength of death proof. Non-kill paths must defer on survivors or indeterminate enumeration.

5. **CRITICAL — STRUCTURAL — The admission surface is not an implementable contract.**

   R4 introduces a per-`(runtime, transport)` five-field admission surface and promises a new comparison without normatively defining its exact keys, value/proof-state enum, absence semantics, actual-side owner, or evidence binding ([design:111](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:111), [design:400](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:400)). It cannot reuse containment: that type is deliberately exactly three fields with `mapped|notEnforced` values ([runtimes.go:22](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:22), [runtimes.go:401](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:401)), and the suite enforces exactly three keys ([validate-metasystem.sh:523](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:523)). Current self-test evidence explicitly calls approvals, tools, and read-root completeness constructed-only ([selftest.go:124](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftest.go:124)).

   The design should preserve containment unchanged and define a separate exact admission type: fields, closed states, duplicate/unknown handling, legacy and non-ACP representation, adapter-owned registration/list interface, both-way conformance join, and how proof is attributed to runtime, transport, protocol version, schema digest, and snapshot.

6. **HIGH — STRUCTURAL — A schema-valid permission request can bypass session and prompt correlation.**

   `Decide` receives only normalized effects and the envelope; unlike delivery, permission handling has no active-session or prompt-window gate ([design:218](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:218), [design:279](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:279)). ACP v1 requires `sessionId`, but only as a string, while the embedded tool update requires only `toolCallId` ([official ACP v1 schema:346–458](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L346-L458)).

   A valid request carrying another session ID, an in-root write, and one-shot options reaches an allow row. The design should require the active session and an open prompt window before normalization; wrong-session, pre-prompt, and post-response requests are named protocol violations and never reach `Decide`.

7. **HIGH — STRUCTURAL — Single-transport snapshots have no transport-keyed producer or test interface.**

   R4 says contract, fake, probe, and self-test consumers move together ([design:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:102)), but the suite invokes one zero-argument contract per runtime ([validate-metasystem.sh:506](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:506)). The common emitter hard-codes plural `transports: []` ([runtime-common.sh:466](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:466)); Devin rejects contract arguments ([devin.sh:510](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:510)); fake rejects arguments and emits `["stdin","file"]` ([fake.sh:271](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/fake.sh:271), [fake.go:236](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/fake.go:236)). Self-test parameters contain no transport, protocol version, or schema digest ([adapter_selftest_verbs.go:148](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/adapter_selftest_verbs.go:148), [selftestrun.go:29](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:29)).

   The design should define registry-derived transport enumeration and explicit transport-keyed probe, contract, fake, and self-test interfaces. The suite must certify every declared pair and its scalar transport/version/schema identity; containment remains once per runtime, admission runs per pair.

8. **HIGH — STRUCTURAL — Go-owned process launch conflicts with standing architecture doctrine.**

   R4 makes `internal/acp` spawn `devin acp` ([design:126](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:126)). Repository doctrine assigns launch, wait, signaling, and environment wiring to scripts ([architecture.md:23](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:23)) and permits only two explicit Go-launch exceptions for caller-supplied programs ([architecture.md:99](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:99)). R4 simultaneously says launch argv is adapter-owned ([design:398](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:398)).

   The design should either leave launch/signaling with scripts and wire the pipes to the Go client, or request a human-ratified third exception where Go receives a fully built command and contains no Devin-specific launch knowledge. It must add `docs/architecture.md` and `cmd/metasystem` to the blast radius and specify the shell-callable client/proof verb contracts.

9. **HIGH — STRUCTURAL — Repair load failure has two contradictory outcomes.**

   The client section says a failed `session/load` makes the job run without repair ([design:133](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:133)); the repair section and matrix say the same live repair-load failure consumes the durable claim and follows repair-failure precedence ([design:319](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:319), [design:360](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:360)). The claim is durably written before the paid call ([record.go:450](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:450)).

   The design should distinguish “P1 did not prove load, therefore repair is disabled before claiming” from “supported repair was claimed and its live load failed, therefore repair is consumed and repair-failure precedence applies.”

10. **HIGH — STRUCTURAL — ACP repair settlement has no replacement contract.**

    R4 names settlement as changed and permits operation without an ATIF transcript, but defines no new owner or evidence ([design:310](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:310), [design:328](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:328)). Today successful repair must settle ([runtime-common.sh:379](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:379)); Devin’s hook requires a repair transcript ([devin.sh:187](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:187)), and `DevinSettle` refuses when that transcript is absent ([devin.go:226](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devin.go:226)).

    The design should define ACP settlement’s authoritative session identity, effective-model evidence, artifacts, owner, and exact failure outcome. It should either declare `DevinSettle` legacy-only or specify its generalized ACP replacement and blast radius.

11. **HIGH — STRUCTURAL — Failed-path usage has no completeness boundary or probe evidence.**

    R4 requires failed repairs and setup/load failures to account spend ([design:319](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:319), [design:346](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:346)), but P1 asks only about initial, loaded, and repair-shaped prompts ([design:435](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:435)). ACP `usage_update` has no final/complete marker ([official ACP v1 schema:4149–4184](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L4149-L4184)). A baseline update before a load error therefore cannot prove complete spend. The shipped path deliberately recomputes usage before judging repair success ([runtime-common.sh:368](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:368)).

    P1 should induce claimed repair-load error, prompt error, cancellation, and early EOF, observing usage and ATIF behavior through teardown. The design should define the completion boundary and publish `unavailable` otherwise, including for initial setup/load failures.

12. **HIGH — STRUCTURAL — P1 never captures the read-permission dialect required by the normalizer.**

    The normalizer must construct `read(paths)` ([design:218](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:218)), but P1 provokes only write, shell, and network requests ([design:430](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:430)). The official request’s tool update is generic and does not standardize a Devin read-path discriminator ([official ACP v1 schema:346–458](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L346-L458)).

    P1 should provoke inside-root and outside-root reads and capture the complete tool call and options. If Devin emits no read permission request, read admission remains unproven; separately shaped delete, move, or search calls remain `unknown` until captured.

13. **MEDIUM — MECHANICAL-GRAIN — Option mapping is undefined for multiple matching one-shot options.**

    R4 says “select `allow_once`” or `reject_once` ([design:251](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:251)), but ACP allows an unrestricted options array, each element carrying its own `optionId`, and the response must return one exact ID ([options array](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L366-L371), [option identity](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L1093-L1124), [response](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L5360-L5379)). Two distinct `allow_once` entries therefore leave no defined response.

    The design should accept exactly one required one-shot kind; zero, multiple, or duplicate-ID matches return `cancelled`. Add fixtures for each cardinality.

The preflight narrowing itself is not defective against current dispatches: shipped presets use `approvals=deny`, real dispatches use `network=allow|deny`, and follow-ups inherit that envelope. The per-effect matrix is otherwise total over validated ordinal values.

Evidence level: repository and official schema read statically; no live Devin probe or test suite run. No files modified. Proposed review receipt: `type=review|outcome=reworked|skills=design-critique|verify=caught|corrections=0|stop_loss=no|note=ACP transport r4 retains structural defects`.

REVISE — structural findings remain
ACP4-DONE
