The design is not implementation-ready. Its central assumption—that all five invariants can be evaluated from existing records—is false, and the proposed remediation path crosses the repository’s authority boundary.

Evidence level: read the proposal, contracts, implementation, and records; cross-checked through two independent read-only explorations. No files changed and no tests run because this was a design review.

### PSD-01 — STRUCTURAL: Only one invariant is fully observable without new records

| Invariant | Checkability | Missing evidence or policy |
|---|---|---|
| Dead supervision | **Checkable, with a semantic conflict** | Active sessions, supervision state, component heartbeats, and `runs-pass.json` are readable. But “fresh within the interval” conflicts with existing predicates: census uses one interval, the attestation reader permits two, and heartbeat freshness permits two plus two seconds. The steward must reuse one canonical armed predicate, not introduce another. See [verifyarmed.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/verifyarmed.go:36) and [scan.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/report/scan.go:242). |
| No process outlives a finished run’s temporary namespace | **Not checkable** | Run records contain no namespace root or ownership relation. Census deliberately sees only agent-shaped processes. Fixture scripts can search their own private `mktemp` path, but that is not a production record. Required: namespace root, run/generation identity, authoritative process ownership, enumeration beyond runtime signatures, and durable orphan observations for the claimed trend. See [run.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/run.go:87), [census/run.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:145), and [supervision-fixtures.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-fixtures.sh:90). |
| Every in-flight plan promise has a leash | **Partly checkable** | The present checker substring-matches free-text plan content against job identifiers and a gate special case. There is no exact plan-to-run, waiter, monitor, or background-task relation. Required: typed work kind and identifier, expected lifecycle, plan/goal linkage on every work record, exposure of the run’s existing `GoalId`, and a runtime-neutral record for other background work. See [openwork.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/report/openwork.go:124) and [plans/README.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/README.md:13). |
| Terminal runs are concluded and acknowledged; unacknowledged work trends down | **Current state checkable; trend not checkable** | Status, wind-down, and acknowledgment are recorded. History is not: acknowledgment overwrites a Boolean and has neither `ackedAt` nor an authoritative transition record. Required: historical steward samples or an authoritative acknowledgment timestamp/event, plus a trend window and comparison rule. See [conclude.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/conclude.go:132) and [verbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/verbs.go:301). |
| Every shipped state joins to critique and a green gate | **Not checkable** | There is no machine definition of “ship,” universal shipped commit/tree identity, durable green gate result bound to that identity, or critique/waiver bound to direct commits. Gate markers represent processes in flight, not outcomes; receipts lack tree and gate identifiers. Required: an immutable ship certification joining the shipped tree, gate result, critique or waiver, and artifact location. See [gaterun.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/gaterun/gaterun.go:1), [receipt.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/receipt/receipt.go:128), and [commit.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/commit.sh:1). |

The last invariant may belong at commit/gate ownership rather than in a later standing scanner. Retrospective detection cannot reconstruct evidence that was never preserved.

### PSD-02 — STRUCTURAL: The proposed safe-act examples are neither safe nor authority-neutral

The design says the checker is non-destructive, then permits it to acknowledge runs or clean leaked processes.

- `run ack` is holder-only, one-way, suppresses warnings, and makes old records eligible for pruning. That is an authority-bearing semantic decision, not harmless bookkeeping.
- Terminating a process is destructive. The existing standing reaper is intentionally denied kill authority; fixture cleanup is safe only because it owns a unique private namespace.
- Automatically dispatching a model-based coach also touches authority and cost. A supervision caller does not have general holder authority to dispatch work.

The initial allowlist should therefore be empty: observe, persist a verdict, and signal. Any future remediation needs a separately reviewed verb owned by the affected subsystem, with exact identity proof, authority mode, idempotence, recovery behavior, and an unreadable-evidence refusal path. See [authority.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/authority/authority.go:50) and [reaper.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reaper.go:17).

### PSD-03 — STRUCTURAL: “Supervision watches the steward” does not close the liveness loop

Supervision currently has exactly two components: watcher and reaper. That assumption is enforced by ownership, armed-state verification, and census validation. A third component is therefore a schema and protocol change, not an incidental addition.

Embedding the steward pass inside the watcher is insufficient: the watcher heartbeat occurs before component work, and pass failures are logged while the loop continues. A fresh watcher heartbeat would not prove a successful steward check. If the supervision owner and both components die, no member of that same set can detect the failure; only an independent boundary such as the Stop hook or subsequent arming can do so.

The design needs an independently readable steward-pass attestation and must name the outside observer responsible for its freshness. The existing supervision watcher cannot be presented as a non-circular solution. See [supervise_component.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/supervise_component.go:120), [owner.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/owner.go:425), and [supervision-hook.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:114).

### PSD-04 — STRUCTURAL: The authoritative verdict and delivery protocol are undefined

The flight recorder cannot own the steward verdict: it is explicitly a best-effort witness and may never become machinery authority. The run ledger is a lifecycle ledger for processes, not a generic alert inbox.

The design needs a durable, atomic, typed steward record containing at least scan identity, completion time, per-invariant outcome, evidence references, unreadable/unknown state, and acknowledgment or resolution state. A flight event may mirror that record, but neither the coach nor Stop delivery may decide from the event stream.

The Stop hook already combines watchdog and turn-verdict state with digest-based exactly-once delivery. Adding another independent warning source without precedence, deduplication, clearing, and retry rules creates conflicting turn outcomes. It is also only an accelerator: the repository’s no-hook fallback must still expose the durable verdict. See [flight-recorder.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/design/flight-recorder.md:278), [emit.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/events/emit.go:1), and [turn-verdict-delivery-contract.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/design/turn-verdict-delivery-contract.md:12).

### PSD-05 — STRUCTURAL: “Drift” and “breach” are labels without decision contracts

The proposal supplies no windows, denominators, hysteresis, baseline, reset rules, or treatment of unreadable evidence. Its closed `ok | drift | breach` result is unsafe because missing or malformed authority records must not silently become either success or permission to remediate.

Each invariant needs a decision table defining:

- Required evidence and an explicit `unknown` or `degraded` outcome.
- The precise `ok`, `drift`, and `breach` predicates.
- Observation window, cadence, hysteresis, and recovery/reset rule.
- Whether the outcome is immediate or trend-based.
- Which outcomes may signal, page, or merely record.

For example, a missing critique on a certified ship is an immediate binary failure; a rising raw count of unacknowledged runs is not meaningful without accounting for newly completed runs. Supervision freshness should reuse its existing predicates rather than invent a fourth timing rule.

### PSD-06 — STRUCTURAL: The steward duplicates owners and is too broad for a first slice

The proposed scanner would become a second policy owner over at least five existing domains:

- Supervision liveness: `internal/supervise` and watchdog reporting.
- Run lifecycle: `internal/run`.
- Turn warnings and unacknowledged outcomes: turn-verdict reporting.
- Plan continuity: open-work reporting.
- Critique and gate conformance: validation, dispatch, and commit/gate paths.

That risks threshold disagreement and contradictory verdicts. The steward should aggregate typed verdicts produced by those owners; it should not reinterpret their raw records independently.

The retro is also not a downstream automation target. Retro inputs are receipts, Git history, and the instruction ledger, and applying changes is human-gated. Steward incidents may become one-way evidence for a later retro, but the steward must not edit policy or turn breaches directly into process rules. See [retro/SKILL.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/skills/retro/SKILL.md:10).

A credible first slice is a read-only aggregator over already-owned predicates, with one durable steward incident record. The missing namespace, plan-work correlation, trend-history, and ship-certification contracts should be designed at their owning boundaries before promising all five checks.

### PSD-07 — MECHANICAL-GRAIN: The runtime-neutral split is achievable, but the coach is placed at the wrong seam

The mechanical Go checker can remain runtime-neutral. The coach, however, is a generic rostered role dispatched through the existing orchestration path; it is not itself an adapter. Only the selected runtime/model binding belongs in adapter configuration. The narrator is presently a backlog precedent, not a shipped implementation seam.

The role should have no repository permissions, and its output must remain untrusted advice adjudicated by the orchestrator. Automatic invocation additionally needs an explicit caller, cost budget, cadence, and deduplication rule; otherwise it inherits the structural authority problem above. See [architecture.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:34) and [orchestration.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:50).

No receipt was written because the checkout is read-only. Proposed receipt:

`RECEIPT|type=review|outcome=reworked|skills=design-critique|verify=skipped|corrections=0|stop_loss=no|delegate=read-only-exploration|note=Round 1 found incomplete observability, authority violations, circular liveness, undefined thresholds, and overlapping ownership`

REVISE — structural findings remain
PS1-DONE
