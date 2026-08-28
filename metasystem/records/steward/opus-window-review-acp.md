Verdict: D90 does not justify the default flip as currently recorded. D82’s dual-host requirement was weakened, ACP is not proven by the named command, and transport-specific snapshot admission remains a prerequisite.

### 1. STRUCTURAL — D88/D90 improperly replace D82’s dual-host gate

D82 explicitly requires a successful ACP-enabled benchmark “on the VM and on the Mac.” D83 then says the D82 gate remains benchmarks one and two; only the additional Devin-host benchmarks are VM-only. [D82/D83 decision record](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:2381)

The existing `bm-2` configuration supplies the coherent interpretation: Claude is the host and Devin is the delegate. [bm-2 manifest](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/specs/bm-2/manifest.json:281) D83 does not say a Devin delegate is impossible on the Mac; it constrains the newly added Devin-host benchmarks.

Therefore, the minimal honest reading of D82 is:

- Run one complete `bm-2` repetition on the Mac with `dispatch.transport.devin=acp` explicitly proven.
- Run the same benchmark in the VM with ACP explicitly proven.
- Require both to satisfy a named success contract.
- Treat `bm-2d` and `bm-2dc` as additional VM-only Devin-host coverage, not substitutes for the D82 pair.

If “do not trust Devin on the Mac” was intended to prohibit even Devin delegates, D82 and D83 conflict and need human clarification. A Mac run containing no Devin role would prove only that the configuration key is inert for unrelated paths; it would not honestly satisfy “with ACP on.”

The plan is internally contradictory: its new top section names one VM-only `bm-2d` gate, while its retained acceptance section still says Mac and VM runs gate the flip. [Reclassified gate](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:41) [Dual-host acceptance section](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:460)

There is an additional decisive problem: the exact `bm-2d` command in D88/D90 does not itself enable or prove ACP. The benchmark provisioning does not set `dispatch.transport.devin`, and the Devin adapter defaults an absent key to `legacy`. [Devin transport selection](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:226) Thus a fresh `bm-2d` run can be mechanically successful entirely through the legacy path.

### 2. STRUCTURAL — Snapshot identity and admission cannot safely follow the flip

The reclassification is unsound. Current snapshot selection matches runtime, CLI version, configuration hash, and freshness—but not requested transport, ACP protocol version, schema digest, or `(runtime, transport)` admission evidence. [Snapshot representation](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/snapshot.go:17) [Capability selection](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:44)

A concrete failure chain is possible:

1. A fresh snapshot is created while the legacy transport is active.
2. The default changes to ACP, but that transport key is not part of snapshot identity.
3. Selection reuses the legacy/runtime-wide snapshot.
4. Permissions and containment evidence from that snapshot admit the dispatch.
5. ACP protocol compatibility is discovered only during launch; schema identity is never bound to admission.
6. The dispatch may then fail after being admitted—or run without evidence proving that ACP was the certified transport.

That directly violates the design’s stated invariant that evidence must not cross transport boundaries. [Required transport identity](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:113) [Required four-state admission](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/acp-transport-design.md:133)

The successful `e1ce759` delegate turn proves one happy ACP execution. It cannot disprove stale or mismatched admission, which is specifically a selection-state problem. These surfaces remain flip prerequisites.

### 3. MECHANICAL-GRAIN — The missing `supervise_acp` fixture remains flip-blocking

This is bounded test work rather than a new architectural decision, so it is mechanical-grain—but it should precede making ACP the default.

The existing ACP fixture tests the Go preflight/turn machinery. It does not exercise the adapter’s `supervise_acp` path: transport selection, FIFO lifecycle, child custody, handshake, job-record patching, cancellation, terminal recording, usage collection, or failure cleanup. [Existing ACP fixture](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/acp-fixtures.sh:1) [ACP supervisor](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:261)

Once ACP becomes the default, that supervisor is no longer parallel or optional machinery. The fixture need not prove every product outcome, but it should exercise success, handshake failure, cancellation, and cleanup before the flip.

### 4. STRUCTURAL — “Mechanically successful” is not an executable acceptance criterion

The present benchmark machinery does not supply D90’s implied definition:

- The cohort runner accepts both completed and parked missions and proceeds to grading.
- “Every job terminal” includes failed, timed-out, and cancelled jobs.
- The seven validity checks cover terminality, chain closure, untracked processes, fences, delegation floor, roster pinning, and evidence completeness—but do not prove ACP was used.
- The benchmark job-record evidence schema does not contain transport or capability-snapshot identity.

[Run-validity checks](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/extractor.py:813)

Before the flip, the decision should read these named checks:

1. The provisioned target resolves `dispatch.transport.devin=acp`, captured before the seal.
2. The candidate commit/configuration is the sealed candidate.
3. The cohort reaches its complete phase and all seven existing validity gates are true.
4. Mission and host-turn terminal states satisfy a predeclared policy; “parked” cannot silently mean mechanically successful.
5. Every required Devin dispatch records `transport=acp`, a wire-derived session identifier, ACP outcome/journal artifacts, and closed runner/chain state.
6. No legacy fallback or ACP handshake, launch, protocol, custody, collection, timeout, or untracked-process failure occurred.
7. Every admitted Devin job points to an ACP-only snapshot binding runtime, CLI/configuration identity, expected and negotiated protocol, schema digest, and `(runtime, acp)` admission evidence.
8. Any non-completed delegate state considered acceptable is enumerated beforehand as a product-quality failure, not decided after observing the run.

A poor benchmark score can be acceptable. Ambiguous transport use or transport-level failure cannot.

### Seal interpretation

The D88/D90 distinction is sound. Before explicit human approval, refusing to manufacture a human-reserved seal was correct. Once the human explicitly approves the exact contract and run, an agent may execute the deterministic sealing operation as clerical implementation of that approval.

The human approval must remain attributable to the human and tied to the exact sealed hash. It does not authorize silently changing D82’s dual-host gate or inventing the meaning of “mechanically successful.”

Evidence level: read the decision record, plan, manifests, benchmark evaluator, adapter, and snapshot-selection code. I did not run the VM/Mac benchmarks. No files were modified; unrelated existing worktree changes were left untouched.

Proposed review receipt: “Re-reviewed D82, D83, D88, D90 and the ACP transport plan; found that the VM-only gate does not satisfy D82, does not itself prove ACP, and defers transport-specific admission prerequisites.”

REVISE — structural findings remain
