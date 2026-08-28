R6 has not converged. D79’s scope pivot is sound, but the design does not yet enforce its marker boundary atomically.

The ancestry mechanism itself is implementable: both [Darwin](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/enumerate_darwin.go:96) and [Linux](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/enumerate_linux.go:52) expose parent traversal, while the record owner already holds one exclusive lock across read–decide–write ([record.go:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:102)). The admission enum and P1’s two-sided allow/refuse experiment are also coherent.

1. **CRITICAL — STRUCTURAL — `custodyProtocol` is neither immutable nor guaranteed to be checked under the terminal lock.**

   Evidence: sealed rules depend entirely on the marker ([design:33](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:33)), but the immutability list omits it and then applies unrelated tightening to every record ([design:195](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:195), [design:200](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:200)). Shipped `RecordCAS` rejects only its fixed immutable-field set and otherwise copies arbitrary patch fields ([record.go:55](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:55), [record.go:278](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:278), [record.go:290](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:290)). A marked job can therefore be downgraded to legacy semantics unless every path happens to behave.

   The design should say that trusted full-record construction is the sole writer of `custodyProtocol`; the field is immutable for every record; absent means legacy, exact `sealed-v1` means sealed, and any other present value is corruption. `RecordCAS`, `RecordProtocolError`, and proof-commit must re-read that discriminator under the same record lock. Marked terminal transitions must be refused through generic CAS. All other proof-field tightening must be marker-gated so unmarked records retain their shipped behavior.

2. **HIGH — STRUCTURAL — The lease sweep cannot call the lock-owning proof-commit verb using its shipped lock structure.**

   Evidence: proof and commit are specified as one lock-owning invocation ([design:170](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:170)), with lease sweep named as a caller ([design:186](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:186)). But `sweepOne` already acquires the same record lock before reading, signaling, and rewriting ([sweep.go:80](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/sweep.go:80), [sweep.go:124](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/sweep.go:124), [sweep.go:127](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/sweep.go:127)). Re-entering the new verb would hit the bounded second acquisition rather than perform one operation ([record.go:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:129)).

   The design should assign the marked branch’s entire locked decision—marker, status, stale `claimEpoch`, generation/revision/seal, signaling, proof, and commit—to the proof-commit owner. The existing lease-owned direct path remains only for unmarked records. It must not prescribe “marker check” followed by a nested lock acquisition.

3. **HIGH — STRUCTURAL — Cancellation still has two incompatible unique committers.**

   Evidence: cancellation is declared an external/full-set writer ([design:186](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:186)), while the failure matrix assigns the cancellation race to the live adapter using `except-live-custodian` ([design:335](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:335)). The shipped public path invokes a separate adapter cancellation command ([dispatch.sh:1219](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1219)), which forwards to dispatcher-owned cancellation ([devin.sh:533](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:533), [dispatch.sh:1346](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1346)).

   The design should distinguish protocol cancellation from external cancellation: a live adapter may send `session/cancel` and commit the resulting PromptResponse race through `except-live-custodian`; operator/lease cancellation is an external backstop that signals, proves the full set, and commits only if the adapter did not already win. Each trigger needs one owner and an atomic precedence check.

4. **HIGH — STRUCTURAL — The settlement join still does not map its refusals to lifecycle outcomes.**

   Evidence: r6 says missing evidence “disables repair or fails settlement” and claims every refusal lists an outcome ([design:281](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:281), [design:283](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:283)), but the failure matrix contains no settlement row ([design:318](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:318)). Shipped settlement separately refuses an absent transcript, a missing exported session, and a mismatched session ([devin.go:238](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devin.go:238), [devin.go:274](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devin.go:274), [devin.go:280](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devin.go:280)); ordinary disagreement becomes `failed/session_identity_disagreement/delivery` ([adjudicate.go:157](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/adjudicate.go:157)), while oversize evidence is a distinct `transcript_oversize` outcome ([devin.sh:365](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:365)). Also, `LoadSessionResponse` does not echo a session ID, but it does not “echo nothing”: it can contain modes and configuration options ([official schema, `schema/types.gen.ts:3009`](https://agentclientprotocol.github.io/typescript-sdk/types/LoadSessionResponse.html)).

   The design should provide a refusal table. Evidence unavailable during certification disables repair before a claim. Once repair is claimed, missing or contradictory request/replay/window/session evidence consumes the claim and maps to the exact settlement failure tuple; oversize retains its separate outcome; absent model remains `unobserved`; mismatch refuses. Usage, custody proof, and artifact handling must be stated for every post-claim row.

5. **HIGH — STRUCTURAL — Usage source selection omits a source in the current official ACP v1 schema.**

   Evidence: r6 considers only `usage_update` and the ATIF export ([design:291](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:291), [design:381](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:381)). ACP v1.3 also defines optional experimental `PromptResponse.usage` at [`schema/types.gen.ts:3232–3246`](https://agentclientprotocol.github.io/typescript-sdk/types/PromptResponse.html). Its `Usage` fields are documented as totals across turns at [`schema/types.gen.ts:3276`](https://agentclientprotocol.github.io/typescript-sdk/types/Usage.html), making it a potentially cumulative source rather than merely another update.

   The design should add `PromptResponse.usage` to P1 and the exactly-one-source decision. P1 must capture it on initial and loaded repair sessions, determine its cumulative boundary, test its absence on failed prompts, and explicitly accept or reject reliance on this unstable field for the pinned release.

6. **HIGH — MECHANICAL-GRAIN — The spawn gate does not require identity preservation across release.**

   Evidence: r6 says the blocked children are registered before release ([design:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:117)), but the shipped helper’s failure is precisely that the registered child can exit before identity capture ([runtime-common.sh:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:81), [runtime-common.sh:92](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:92)). A gate wrapper that forks or launches a new process after release merely registers the wrapper.

   The design should require each registered gate process to `exec` its real protocol peer without changing PID, prohibit daemonization, and add a P2 assertion that PID and birth token remain identical across gate release.

7. **MEDIUM — MECHANICAL-GRAIN — The schema digest cannot be verified through official initialization as written.**

   Evidence: r6 says the digest is verified at initialize ([design:61](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:61)), but official `InitializeResponse` contains protocol version, capabilities, authentication methods, agent information, and opaque `_meta`—no schema digest ([official schema, `schema/types.gen.ts:1563`](https://agentclientprotocol.github.io/typescript-sdk/types/InitializeResponse.html)). The existing Devin capture’s `_meta` also provides no digest ([wire probe:56](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-wire-probe.md:56)).

   The design should say the digest authenticates the local schema artifact before launch, while initialize verifies the negotiated version and capabilities. If peer digest attestation is required, P1 must first prove a versioned Devin extension rather than assuming one.

8. **MEDIUM — MECHANICAL-GRAIN — P1 question 9 cannot establish the framing contract r6 says it supplies.**

   Evidence: r6 intends to derive delimiter, encoding, stream purity, maximum frame size, and partial-frame behavior from P1 ([design:22](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:22)), but the only question is “Raw byte framing trace” ([design:391](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:391)). An ordinary trace cannot establish a maximum or force fragmentation behavior.

   The design should require deliberate fragmented and coalesced writes, below/above-limit frames, UTF-8 boundaries, stdout contamination, and stderr separation. If the wire supplies no maximum, state a client-owned ceiling instead of treating one observed trace as the server contract.

Evidence level: read-only inspection of the design, prior critiques, shipped code, D79, wire capture, and official ACP v1.3 schema. No files were modified and no tests were run.

Proposed receipt: `acp-critique-r6 — D79 scope accepted; eight material findings, five structural; escalate before implementation.`

REVISE — structural findings remain
ACP6-DONE
