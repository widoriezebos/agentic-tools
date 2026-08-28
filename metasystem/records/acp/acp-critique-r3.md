R3 does not converge. Several earlier folds are substantive, but core custody, permissions, snapshots, repair accounting, and probe completeness remain structurally under-specified or contradicted by shipped owners.

## Findings

1. **CRITICAL — STRUCTURAL — Custody proof is not atomic with custody registration.**

   R3 permits a second repair tree while the record remains running and requires all registered identities dead before terminal compare-and-swap ([acp-transport-design.md:109](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:109), [acp-transport-design.md:139](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:139)). `CustodyAdd` can append while status is pending/running ([custody.go:8](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/custody.go:8)), but the reaper reads and proves outside the record lock ([reaper.go:78](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper.go:78)) and its final compare checks status only ([reaper.go:151](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper.go:151), [record.go:250](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:250)). A live repair identity can therefore be appended after proof but before terminalization.

   The design should require a sealed custody generation or exact-set digest. Registration advances it; death-proof compare-and-swap checks status plus that generation/set atomically; launching another attempt atomically opens a new generation; registration after sealing refuses and tears down the new process. Add the registration-versus-reap/cancel race fixture.

2. **CRITICAL — STRUCTURAL — “Every registered identity” still does not define a valid group-death proof.**

   R3 makes only the kill-capable path verify the process group ([acp-transport-design.md:132](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:132)), so the no-kill reaper could prove registered processes dead and stamp group death while an unregistered same-group descendant remains. It also does not explicitly include the top-level adapter in the proof set or define malformed/unknown custody entries and legacy empty lists. Current fixtures terminalize records with no custody list after testing only the top-level PID ([reaper_test.go:93](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper_test.go:93)).

   The identity predicate is also unspecified. Custody stores the job tag ([custody.go:35](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/custody.go:35)), while the existing custodian proof treats a live process whose argv lacks that tag as dead-to-us ([custodian.go:44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/custodian.go:44)). R3 does not guarantee `devin acp` carries the tag in argv.

   The design should define the proof set as the top-level identity plus every valid custody entry; use PID/start identity for children unless tag carriage is separately guaranteed; make malformed or unknown evidence defer terminalization; and require the group to be empty before stamping group death. It must state the compatibility outcome for absent or empty legacy custody lists.

3. **CRITICAL — STRUCTURAL — The terminal/death-proof change has no coherent owner and misses a second reaper.**

   Mission drain independently reaps records by proving only the top-level custodian, then writes `groupDeathProvenAt` ([drain.go:200](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/drain.go:200), [drain.go:217](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/drain.go:217)); `internal/missionrunner` is absent from R3’s blast radius ([acp-transport-design.md:396](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:396)).

   Other terminal paths conflict too: normal completion compare-and-swaps without any death proof ([runtime-common.sh:165](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:165)); its sweep excludes the still-live adapter itself ([runtime-common.sh:209](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:209)); handshake timeout terminalizes before best-effort cleanup ([dispatch.sh:1458](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1458)); cancellation signals the group, not escaped registered identities ([dispatch.sh:1346](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1346)). This contradicts R3’s universal pre-terminal proof promise and its claim that downstream terminal owners remain unchanged ([acp-transport-design.md:139](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:139), [acp-transport-design.md:414](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:414)).

   The design should name one shared proof owner used by standing reap, mission drain, cancel, deadlines, protocol failure, normal completion, and repair failure. Either an external finalizer commits after the adapter exits and proves the whole group dead, or the contract must explicitly define a weaker “all descendants dead except live custodian” proof. Exact escaped identities need a PID-reuse-safe signal-and-proof rule.

4. **CRITICAL — STRUCTURAL — Strict refusal is not demonstrated to be a usable increment, and its option rule can persist denial.**

   R3 calls universal refusal “safe-but-strict” ([acp-transport-design.md:186](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:186)), but the shipped Devin seam records that a denied tool ends the turn with no reply or report ([devin.sh:31](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:31)); dangerous mode exists because graded refusals ended without delivery ([devin.go:195](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devin.go:195)). P1 captures request shapes but never sends denial/cancellation and observes the resulting turn ([acp-transport-design.md:369](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:369)).

   “Most restrictive denying option” may also select `reject_always`; the official schema says that choice is remembered, so it can poison the loaded repair session. R3 forbids only `allow_always`. [Official ACP v1 `PermissionOptionKind`](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L1130-L1153)

   The design should classify strict refusal as a defensive failure mode, not supported transport behavior, unless P1 proves representative denied tasks still produce a useful `end_turn` or eligibility is limited to workloads requiring no permission calls. Strict mode must select only `reject_once`, otherwise return cancelled.

5. **CRITICAL — STRUCTURAL — R2’s required total permission table is still only folded in words.**

   R3 lists dimensions that a future “real table” will cover but supplies no decisions for `network=ask`, `approvals=ask|allow`, mixed effects, multiple paths, missing facts, or option priority ([acp-transport-design.md:193](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:193)). The real envelope has five fields and independent ordinal scales ([envelope.go:19](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/envelope.go:19), [permissions.go:98](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/permissions.go:98)). Calling the decision pure is also contradicted by symlink normalization, which reads live filesystem state ([fspath.go:18](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/fspath.go:18)).

   The design should define a dialect-neutral normalized effect request and the complete decision matrix now. P1 should establish only the versioned Devin-wire-to-effect mapping and its stable discriminator. A separate named normalization owner should resolve filesystem facts before the pure decision step.

6. **CRITICAL — STRUCTURAL — Snapshot identity omits the protocol and schema identities it is supposed to authorize.**

   R3 pins expected protocol version and a schema digest ([acp-transport-design.md:20](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:20), [acp-transport-design.md:73](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:73)), but defines snapshot/garbage-collection identity as only runtime, CLI version, configuration hash, and transport ([acp-transport-design.md:80](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:80)). The selector result presently records none of transport, expected version, or schema digest ([select.go:29](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:29)).

   The design should make `transport`, `expectedProtocolVersion`, and `schemaArtifactDigest` explicit snapshot identity and selection fields, with not-applicable values for legacy. Job provenance must record requested transport, expected version, actual negotiated version, digest, and exact snapshot path.

7. **CRITICAL — STRUCTURAL — The per-transport producer/consumer migration is not implementable as written.**

   The current Devin probe writes one plural snapshot containing legacy channels and ACP ([devin.sh:65](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:65)); selector self-healing invokes an unparameterized probe ([dispatch.sh:485](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:485)); the contract producer emits one plural snapshot ([runtime-common.sh:466](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:466)). The registry and validation surface exposes one runtime-wide, exactly three-field enforcement map ([runtimes.go:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:79), [validate-metasystem.sh:523](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:523)), so ACP-only mapped values cannot coexist with legacy values and `approvals`/`tools` cannot become mapped there.

   R3 also omits the self-test reader, which selects the newest runtime snapshot without transport/version/schema filtering ([selftest.go:61](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftest.go:61)) and then uses it to certify live permission behavior ([selftestrun.go:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:293)).

   The design should specify transport-keyed probe, contract, enforcement-map, conformance, fake, self-heal, and self-test interfaces. It should either key enforcement declarations by transport or separate the existing three-field containment declaration from a five-field permission-admission proof surface. The old plural-snapshot-to-legacy rule itself is safely fail-closed.

8. **HIGH — STRUCTURAL — Garbage collection can delete a snapshot pinned by a live job.**

   R3 retains only the newest snapshot per identity while promising each job pins an exact snapshot ([acp-transport-design.md:82](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:82), [acp-transport-design.md:95](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:95)). Current garbage collection deletes every older member without consulting job references ([gc.go:553](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:553), [gc.go:583](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:583)); mirroring silently omits a missing snapshot ([mirror.go:132](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mirror.go:132)).

   The design should retain every referenced snapshot until a durable mirror manifest proves that exact file was copied. Only unreferenced or durably mirrored superseded snapshots may be deleted.

9. **HIGH — STRUCTURAL — The second-attempt repair story contradicts the shipped repair owner and failure matrix.**

   R3 says one-repair is reused unchanged ([acp-transport-design.md:235](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:235)), but the shipped owner invokes legacy `devin -p -r` ([devin.sh:199](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:199)). R3 instead requires a fresh ACP tree and `session/load` ([acp-transport-design.md:107](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:107)). Its matrix then classifies every load error as preflight with no prompt sent ([acp-transport-design.md:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:293)), although repair load occurs after initial work and the durable repair claim ([record.go:450](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:450)).

   The design should say only validation, adjudication rules, repair claim, and precedence are reused. Repair execution, collection, usage, settlement, custody, and terminal sequencing change. Split initial setup failure from repair setup failure: the latter consumes the claimed repair, accounts for both attempts, proves second-tree custody, skips settlement, and follows repair-failure precedence—not preflight.

10. **HIGH — STRUCTURAL — Usage still names two incompatible algorithms instead of choosing one.**

    R3 says repair usage “replaces or extends” initial usage ([acp-transport-design.md:269](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:269)). The existing owner has one concrete cumulative-session algorithm: repair metrics replace the provisional initial calculation and are differenced against predecessor totals ([devin.sh:164](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:164), [devin.go:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/devin.go:24)).

    The design should prescribe both possible P1-selected branches: cumulative-session totals replace the provisional value and subtract the exact predecessor; genuine per-attempt deltas are combined exactly once, including load spend. Any launched repair lacking complete usage makes the entire round unavailable, and failed repairs still account for spend.

11. **HIGH — STRUCTURAL — P1 is incomplete on launch identity and normal process shutdown.**

    The job pins requested/effective model ([build.go:190](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/build.go:190)), while the existing launch pins workspace, model, and job-derived configuration ([devin.sh:287](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:287)). R3’s six questions ask none of this ([acp-transport-design.md:358](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:358)).

    It also requires teardown after successful `PromptResponse`, but asks only what `session/cancel` does ([acp-transport-design.md:109](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:109), [acp-transport-design.md:376](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:376)). ACP’s cancel message cancels an active prompt; it is not a server-shutdown contract. [Official ACP v1 `session/cancel`](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L5666-L5730)

    P1 should additionally capture:

    - Production-shaped argv and `session/new`/`session/load` mapping for cwd, requested model, and job configuration, plus effective-model/session certification evidence.
    - Normal success and setup-failure wind-down: stdin/client close, late frames, exit code, grace deadline, session durability before repair load, and TERM/KILL escalation.
    - PID/start/PGID/descendant state throughout initial and loaded repair attempts.

12. **MEDIUM — MECHANICAL-GRAIN — The stdio framing contract is neither pinned nor captured.**

    The checked-in wire probe is normalized, pretty-printed JSON rather than raw bytes ([acp-wire-probe.md:4](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-wire-probe.md:4)). R3 requires malformed, oversized, and half-frame behavior ([acp-transport-design.md:160](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:160), [acp-transport-design.md:296](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:296)) while pinning only the JSON schema.

    The design should pin the stdio transport/framing specification and add a byte-for-byte P1 trace covering delimiter, encoding, stdout versus stderr purity, maximum frame size, and partial-frame behavior.

13. **MEDIUM — MECHANICAL-GRAIN — ACP candidate selection and deduplication are not deterministic.**

    The current collector has fixed stdout → named-file → transcript precedence ([devincollect.go:104](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devincollect.go:104)); R3 adds `acp` without placing it in that order or saying whether invalid ACP falls through ([acp-transport-design.md:235](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:235)).

    R3 also drops duplicates by “content-and-position” ([acp-transport-design.md:226](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:226)), but ACP chunks have content and optional message ID, not a chunk position or stable deduplication identity. [Official ACP v1 `ContentChunk`](https://github.com/agentclientprotocol/agent-client-protocol/blob/main/schema/v1/schema.json#L3779-L3810)

    The design should define ACP’s exact candidate-channel precedence and fallback rule. Preserve chunks in arrival order unless P1 proves a stable chunk identity; never content-deduplicate potentially legitimate repeated text.

Substantive folds verified: admission is no longer presented as containment; filesystem/terminal callbacks remain disabled; `PromptResponse` is completion evidence rather than return content; load replay is watermarked; ACP→legacy repair is prohibited; events remain advisory; blocked writes have a fixture; legacy plural snapshots fail closed for ACP selection; hosts and dead-round usage recovery remain honestly out of scope.

Evidence level: repository and official ACP v1 schema read statically; no live Devin probe or tests run. No files modified.

Proposed receipt: `review acp-transport r3: 13 material findings (11 structural, 2 mechanical-grain); no files changed`

REVISE — structural findings remain
ACP3-DONE
