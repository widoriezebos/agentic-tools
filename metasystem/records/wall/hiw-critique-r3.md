R3 has not converged. Four structural decisions still change what an implementer would build; the remaining matrix/schema gaps are mechanical-grain.

## Fold fidelity

| R2 finding | R3 fold | Assessment |
|---|---|---|
| HIW-R2-01, replay-safe authorization | Partial | Issue-time bindings and the consumption ledger are present, but later-turn applicability and the acceptance/consumption commit point remain undefined. |
| HIW-R2-02, total tree composition | Partial | Projection, ordering, overlap, exact apply, baseline, reachability, and shared ownership are stated. The consuming turn’s expected-tree transition is not. |
| HIW-R2-03, machine-metadata replacement | Partial | The declared host-artifact delta is carried, but tracked runner metadata has no equation term or owner. |
| HIW-R2-04, taint state machine | Full | Writer, ordering, recovery inspection, park precedence, `RESTORE`, `ADOPT_DISPUTED_TREE`, and generic-answer refusal are carried. |
| HIW-R2-05, prompt authorities | Full | Both authorities, the verbatim interim rule, narrowed host duties, doctrine, and four benchmark manifests are named. |
| HIW-R2-06, delegation floor | Full | Nonempty, unsuperseded, actually consumed authorization and every exclusion are explicit. |
| HIW-R2-07, implementation artifacts | Partial | Conceptual records are named, but schemas, locations, transaction boundaries, and one migration outcome remain open. |
| HIW-R2-08, goal handoff | Full | [goals.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/goals.md:5) now says “No exemptions — D100” and routes through r3 critique. |

## Material findings

### HIW-R3-01 — STRUCTURAL: delayed authorization applicability is undefined

R2 required an earlier authorization to be “current, unsuperseded, and still applies.” R3 weakens this to:

> “an unconsumed, current, unsuperseded authorization is usable in a later turn”

([design](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:71)).

R3 later says a patch must apply cleanly “to the expected tree,” but never defines that tree or its relationship to the authorization’s bound base ([design](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:100)).

For authorization `A`, issued in turn N with `apply(P, B) = R`, after turn N+1 changes the accepted tree to `C`, turn N+2 admits at least three plausible implementations:

- Require `C == B`, rejecting any delayed authorization after tree movement.
- Apply `P` contextually to `C`, potentially producing an output never reviewed at issue time.
- Permit transport only when intervening changes are disjoint from `A`’s changed paths.

Those produce different acceptance outcomes. Mission-incarnation verification proves provenance; it does not select a staleness rule. R3 must define `E0 = openTurn.preTree`, the compatibility predicate between each authorization’s bound base and current `Ei`, and how `Ei+1` is derived.

### HIW-R3-02 — STRUCTURAL: consumption is not atomically joined to accepted-tree commitment

No sentence defines the recoverable commit joining:

- successful wall comparison;
- pre/expected/post-tree evidence;
- accepted turn-log entry; and
- `authorizationDigest → consumedByTurnId`.

Ledger-first burns an authorization when the turn later taints and is refused. Acceptance-first leaves a replay window if the runner crashes before ledgering. The design must specify one authoritative transaction or write-ahead/reconciliation protocol, including crash behavior. HIW-O3, O4, and O10 can currently pass independently while this hole remains.

The returned certification also needs either an authorization-digest reference or a unique runner-owned lookup rule; current evidence is job-ID based.

### HIW-R3-03 — STRUCTURAL: tracked runner metadata has no equation category

R3 says:

> “an adopting repository that TRACKS runner metadata gets those bytes represented as exact deltas like everything else”

([design](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:40)).

But the equation contains only consumed delegate patches and host-authored declared artifacts. Tracked machine output fits neither. An implementer must invent whether to refuse such repositories, misclassify the files as host artifacts, or add a separately owned deterministic transform. That sentence only promises representation; it does not specify it.

### HIW-R3-04 — STRUCTURAL: legacy-state behavior is still delegated to the implementer

HIW-O11 says legacy state “migrates safely or refuses explicitly” ([design](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:229)). Migration and refusal are different observable control flows. R3 must choose one or state a deterministic selection rule.

## Obligation matrix

The shared single-owner tree primitive is covered by HIW-O4. The matrix is otherwise incomplete:

- `core.fileMode` pinning has no required-behavior or focused-proof entry.
- The clean-or-human-sealed initial baseline has no row, owner, or seal proof.
- Durable order-before-apply has only a generic “order” fixture, not persistence/crash proof.
- The acceptance/consumption transaction above has no row.
- HIW-O8 does not preserve the body’s explicit “verbatim in both authorities” and four-manifest proof requirements.
- HIW-O11 contains an undecided outcome rather than an obligation.

Mechanically, the five-column table is not a valid gate matrix. I ran:

`scripts/assert-design-obligation-gate.sh --file plans/host-implementer-wall-design.md`

It exited 1 with `no design-obligation rows found`; the gate requires the canonical ten-column header and proof/status fields.

R2-07 also remains materially incomplete: authorization and ledger schemas/storage, certification-reference shape, wall-evidence schema, mission-contract declaration key and protected-path table, tree-ref namespace/cleanup, typed-resolution interface, and event payloads are still choices rather than readable contracts.

Evidence: checked by reading r1, r2, r3, the goal ledger, relevant landed/conformance/missionrunner code, and the obligation-gate implementation; ran only the obligation-matrix check. No tests ran and no files were modified.

Proposed receipt: host-implementer-wall round 3 — revise; delayed-tree applicability, atomic consumption, tracked runner metadata, legacy handling, and the executable obligation matrix remain open.

REVISE — structural findings remain
