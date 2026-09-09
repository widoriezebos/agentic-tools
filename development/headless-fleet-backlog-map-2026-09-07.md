# Headless MetaSystem backlog map — 7 September 2026

**Historical snapshot.** Statements below describe the ledger at the cited 7 September revision. Source links are pinned to that revision; consult the live ledger for current status.

The five requirements now map to the existing headless-fleet effort, with bounded implementation and acceptance owners. They do not become five additional broad build programs. The shared ledger has accepted these changes through `goal open` and `goal reconcile`, ending at revision `d9ec0fa95827e8a17e45856639edfddb710f79c3`.

This is a planning result. Reliable unattended operation still needs the implementation and operational evidence below. No features were implemented or acceptance runs launched during this pass.

| Requirement | Implementation owners | Evidence required before declaring readiness |
| --- | --- | --- |
| Mandatory design, design critique, build and independent build critique | `design-gate-at-dispatch`, `manifest-floor-at-dispatch`, `critique-always`, new `landing-design-provenance`, and the existing two chain-landing defect items | Missing or stale evidence refuses at admission and landing. A valid reviewed candidate passes. Attempts at unauthorized direct host, delegate or unannounced-caller integration refuse. |
| Continuous execution from the approved backlog | Restored `fleet-pull`, supported by `one-approval-gate` | Consecutive approved goals finish without a supervising chat; blocked, unapproved and foreign-pinned goals are skipped; an empty queue wakes when new approved work arrives. |
| Independent oversight, recovery and durable human stop | `watch-verb`, `idle-every-runtime-enforcement`, `turn-verdict-hardening`, `recovery-to-good-state`; consume the active `host-runtime-setup` and `metasystem-stop-verb` work | `recovery-rehearsal` interrupts delegate, host/runner and supervision owners independently; proves recovery or an explicit bounded refusal, preserved evidence, no duplicated accepted effect, and no resurrection after human stop. |
| Nodes and central brain work together | `fleet-channel-gateway`, `seat-mutual-awareness`, `fleet-join-bootstrap`; reuse delivered `fleet-coordinator-brain` work | Two nodes cannot execute the same claim; independent goals progress concurrently; the brain sees remote progress and questions, helps shape the backlog and receives its standing context under its actual runtime. Human approval remains authoritative. Nodes continue approved work during brain absence, and replayed channel delivery applies an answer once. |
| Demonstrated operation of the integrated system | Existing diagnostic `first-headless-run`, new `headless-continuous-delivery-proof`, new `headless-fleet-coordination-proof`, and `recovery-rehearsal` | Retained evidence identifies the exact integrated revisions, approvals, limits, roles and outcomes. One successful feature or a count of completed components cannot establish readiness. |

The canonical acceptance map is in [never-idle-ironclad](https://github.com/widoriezebos/agentic-tools/blob/d9ec0fa95827e8a17e45856639edfddb710f79c3/metasystem/plans/goals/never-idle-ironclad.md). This document is a local review companion; scheduling dependencies live in the ledger.

## Three new bounded items

1. [Landing design provenance](https://github.com/widoriezebos/agentic-tools/blob/d9ec0fa95827e8a17e45856639edfddb710f79c3/metasystem/plans/goals/landing-design-provenance.md) owns the deferred landing check from `two-bars-for-changes`: bind the landed implementation to the certified design revision and independent design critique admitted before its build. It reuses the existing landing evaluator and existing promotion rules.
2. [Continuous single-node delivery proof](https://github.com/widoriezebos/agentic-tools/blob/d9ec0fa95827e8a17e45856639edfddb710f79c3/metasystem/plans/goals/headless-continuous-delivery-proof.md) owns one bounded acceptance run in a disposable adopted repository. It covers consecutive features, required-stage refusal and empty-queue wakeup.
3. [Fleet and brain coordination proof](https://github.com/widoriezebos/agentic-tools/blob/d9ec0fa95827e8a17e45856639edfddb710f79c3/metasystem/plans/goals/headless-fleet-coordination-proof.md) owns one bounded two-node integration scenario: competing claims, parallel independent work, brain interaction, brain absence and channel replay.

The proof items expose defects and return them to existing owners or separately scoped goals. They do not absorb unlimited implementation work. All three remain queued, unapproved and unclaimed. The engine assigned each its standing Tier 3 budget proposal: one day, ten attempts, 1,200 reserved job minutes, one active job and three review rounds. Intake did not authorize execution.

## Reconciled existing work

- Restored `fleet-pull` to the queue. Delivered ledger notifications and the narrow repeated-Stop recovery path do not satisfy its ordinary autonomous selection/claim/run/next-item loop.
- Concluded `design-critique-chain-binding` as a duplicate outcome delivered by `design-chain-has-no-lawful-close`. This does not claim that pre-build design admission is implemented.
- Concluded `channel-telegram-shared-update-pointer` as a withdrawn duplicate of `fleet-channel-gateway`. This does not claim that the shared channel is finished.
- Preserved the gateway's later human decision: receive, shared Git commit with one winner, then confirm. Its next step identifies that decision as superseding the historical leased-gateway wording; its approved intent was not rewritten.
- Reused the completed `path-class-manifest` prerequisite. Unrelated remaining witness, growth-fuse and audit work in `two-bars-for-changes` is excluded from the readiness prerequisites. Its caller classification and guard failures still have to pass the targeted refusal proof.
- Preserved all four active claimed goal records, including the other Codex agents' host setup and stop work, byte for byte.

## Recorded dependencies

Thirty prerequisite links are recorded across eight goals. Completed prerequisites remain useful evidence links and do not represent new implementation work.

| Goal | Requires |
| --- | --- |
| `design-gate-at-dispatch` | `design-chain-has-no-lawful-close` |
| `manifest-floor-at-dispatch` | `path-class-manifest` |
| `landing-design-provenance` | `design-gate-at-dispatch`, `path-class-manifest` |
| `fleet-pull` | `one-approval-gate` |
| `idle-every-runtime-enforcement` | `watch-verb` |
| `headless-continuous-delivery-proof` | `first-headless-run`, `fleet-pull`, `one-approval-gate`, `design-gate-at-dispatch`, `manifest-floor-at-dispatch`, `critique-always`, `landing-design-provenance`, `chain-landing-carries-the-reviewed-diff`, `chain-landing-after-base-move-recertifies`, `host-runtime-setup`, `metasystem-stop-verb` |
| `headless-fleet-coordination-proof` | `headless-continuous-delivery-proof`, `fleet-channel-gateway`, `seat-mutual-awareness`, `fleet-join-bootstrap`, `fleet-coordinator-brain` |
| `never-idle-ironclad` | `headless-continuous-delivery-proof`, `headless-fleet-coordination-proof`, `recovery-rehearsal`, `recovery-to-good-state`, `idle-every-runtime-enforcement`, `turn-verdict-hardening`, `host-runtime-setup`, `metasystem-stop-verb` |

Implementation can proceed along the independent branches. `first-headless-run` and early recovery probes can expose defects before every repair finishes; they have no blanket blocker on all of those repairs. Final proof must use the integrated final revisions. The sequence for acceptance is the limited first run, consecutive delivery, fleet/brain integration, and then the umbrella readiness decision with recovery and supervision evidence included.

## Verification and limits

Read back the accepted ledger and compared the published files with their materialized copies. All dependency targets exist and the complete dependency graph is acyclic. The publication changes only goal records: eighteen existing items, three new items and two duplicate conclusions.

Existing intents, approvals, pins, claims and effective budgets were preserved. The engine expanded `watch-verb`'s legacy four-field budget to spell out its already-inferred three-review-round limit; the parser confirms that this is a representation change. All four actively claimed records are byte-identical to the baseline.

No product test suite or live headless/fleet acceptance run was executed: this pass changed planning records only. Readiness is bounded to governed execution and demonstrated failure cases. Protection against forged human-authority history remains with `ledger-authentication`; these acceptance goals do not claim to solve it.

The original development checkout was not pulled or reset. Ledger publication used an isolated clone so other agents' working files and claims were preserved.
